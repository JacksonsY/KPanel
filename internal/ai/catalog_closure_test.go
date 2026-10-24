package ai

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func seedCatalogModels(t *testing.T, s *Store, provider string, count int) {
	t.Helper()
	_, err := s.db.ExecContext(context.Background(), `WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n<?)
	 INSERT INTO models(id,provider_id,model_id,display_name,context_window,tool_calling,vision,reasoning,enabled,is_default,created_at,updated_at)
	 SELECT 'seed-'||n,?,'seed-'||n,'Seed '||n,8192,1,1,0,1,0,1,1 FROM seq`, count, provider)
	if err != nil {
		t.Fatal(err)
	}
}

func catalogProvider(t *testing.T, s *Store) Provider {
	t.Helper()
	p, err := s.SaveProvider(context.Background(), Provider{Name: "Catalog", Protocol: ProtocolOpenAICompatible, BaseURL: "https://example.com", EndpointScope: EndpointPublic}, 0)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCatalogQuotaIsCumulativeAndAtomic(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(filepath.Join(t.TempDir(), "ai.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := catalogProvider(t, s)
	seedCatalogModels(t, s, p.ID, maxDiscoveredModels-1)
	if err := s.SaveModels(ctx, p.ID, []Model{{ModelID: "last"}, {ModelID: "last", DisplayName: "Final"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveModels(ctx, p.ID, []Model{{ModelID: "seed-1", DisplayName: "Changed"}, {ModelID: "overflow"}}); err == nil {
		t.Fatal("cumulative quota accepted another batch")
	}
	items, err := s.ListModels(ctx, "")
	if err != nil || len(items) != maxDiscoveredModels {
		t.Fatalf("catalog size=%d err=%v", len(items), err)
	}
	for _, m := range items {
		if m.ModelID == "seed-1" && m.DisplayName != "Seed 1" {
			t.Fatal("rejected batch partially updated existing configuration")
		}
	}
	if err := s.SaveModels(ctx, p.ID, []Model{{ModelID: "seed-1", DisplayName: "Legal update"}}); err != nil {
		t.Fatalf("existing configuration update at capacity rejected: %v", err)
	}
}

func TestCatalogQuotaSerializesConcurrentProviders(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(filepath.Join(t.TempDir(), "ai.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, q := catalogProvider(t, s), catalogProvider(t, s)
	seedCatalogModels(t, s, p.ID, maxDiscoveredModels-1)
	start, results := make(chan struct{}), make(chan error, 2)
	for _, provider := range []Provider{p, q} {
		go func() {
			<-start
			results <- s.SaveModels(ctx, provider.ID, []Model{{ModelID: "new"}})
		}()
	}
	close(start)
	success := 0
	for range 2 {
		if <-results == nil {
			success++
		}
	}
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM models`).Scan(&count); err != nil || count != maxDiscoveredModels || success != 1 {
		t.Fatalf("concurrent catalog count=%d successes=%d err=%v", count, success, err)
	}
}

func TestProviderQuotaIsCumulativeAndConcurrent(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(filepath.Join(t.TempDir(), "ai.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := catalogProvider(t, s)
	for range 126 {
		catalogProvider(t, s)
	}
	start, results := make(chan struct{}), make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := s.SaveProvider(ctx, Provider{Name: "Concurrent", Protocol: ProtocolOpenAICompatible}, 0)
			results <- err
		}()
	}
	close(start)
	success := 0
	for range 2 {
		if <-results == nil {
			success++
		}
	}
	providers, err := s.ListProviders(ctx)
	if err != nil || success != 1 || len(providers) != 128 {
		t.Fatalf("providers=%d successes=%d err=%v", len(providers), success, err)
	}
	p.Name = "Updated"
	if _, err := s.SaveProvider(ctx, p, p.Version); err != nil {
		t.Fatalf("existing provider update rejected: %v", err)
	}
}

func TestLegacyCatalogReadsFailBeforeMaterialization(t *testing.T) {
	for _, kind := range []string{"count", "long-model", "long-provider"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			s, err := OpenStore(filepath.Join(t.TempDir(), "ai.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			p := catalogProvider(t, s)
			count := 1
			if kind == "count" {
				count = maxDiscoveredModels + 1
			}
			seedCatalogModels(t, s, p.ID, count)
			if kind == "long-model" {
				_, err = s.db.Exec(`UPDATE models SET display_name=?`, strings.Repeat("x", 1<<20))
			}
			if kind == "long-provider" {
				_, err = s.db.Exec(`UPDATE providers SET name=?`, strings.Repeat("x", 1<<20))
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, provider := range []string{"", p.ID} {
				if items, err := s.ListModels(ctx, provider); err == nil || items != nil {
					t.Fatal("legacy oversized model catalog materialized")
				}
			}
			// Missing client and ProviderService deliberately prove that legacy
			// rejection happens before provider decryption or any network call.
			if items, err := (&Service{Store: s}).SyncModels(ctx, p.ID); err == nil || items != nil {
				t.Fatal("legacy oversized catalog reached remote discovery")
			}
			providers, _ := NewProviderService(s, s.secrets)
			if backup, err := providers.ExportAccess(ctx); err == nil || len(backup.Providers) != 0 || len(backup.Models) != 0 {
				t.Fatal("legacy oversized catalog materialized into backup")
			}
			var retained int
			if err := s.db.QueryRow(`SELECT count(*) FROM models`).Scan(&retained); err != nil || retained != count {
				t.Fatal("catalog read silently deleted existing models")
			}
			if items, err := s.ListProviders(ctx); kind == "long-provider" && (err == nil || items != nil) {
				t.Fatal("legacy oversized provider catalog materialized")
			} else if kind != "long-provider" && (err != nil || len(items) != 1) {
				t.Fatal("provider list unavailable for explicit model catalog repair")
			}
			if err := s.DeleteProvider(ctx, p.ID); err != nil {
				t.Fatalf("legacy catalog cannot be repaired by explicit provider deletion: %v", err)
			}
		})
	}
}

func TestSyncModelsPreservesExistingConfiguration(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(filepath.Join(t.TempDir(), "ai.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := catalogProvider(t, s)
	if err := s.SaveModels(ctx, p.ID, []Model{{ModelID: "configured", DisplayName: "My name", ContextWindow: 65536, ToolCalling: false, Enabled: false, IsDefault: true}}); err != nil {
		t.Fatal(err)
	}
	before, _ := s.ListModels(ctx, p.ID)
	providers, _ := NewProviderService(s, s.secrets)
	service := &Service{Store: s, Providers: providers, client: &syncModelsClient{models: []Model{{ModelID: " configured ", DisplayName: "Remote rename", ContextWindow: 8192, ToolCalling: true, Enabled: true}}}}
	items, err := service.SyncModels(ctx, p.ID)
	if err != nil || len(items) != 1 {
		t.Fatal(err)
	}
	m := items[0]
	if m.ID != before[0].ID || m.DisplayName != "My name" || m.ContextWindow != 65536 || m.ToolCalling || m.Enabled || !m.IsDefault || !m.Vision {
		t.Fatal("discovery erased administrator configuration")
	}
}

func TestShortKeyHintsAreFixedAndLegacyCopiesScrubbed(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path, keyPath := filepath.Join(dir, "ai.db"), filepath.Join(dir, "ai-secrets.key")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	providers, _ := NewProviderService(s, s.secrets)
	key := "Q7z!"
	p, err := providers.Save(ctx, "", ProviderInput{Name: "Short key", Protocol: ProtocolOpenAICompatible, BaseURL: "https://example.com", EndpointScope: EndpointPublic, APIKey: &key})
	if err != nil {
		t.Fatal(err)
	}
	public, _ := json.Marshal(p)
	if bytes.Contains(public, []byte(key)) || p.APIKeyHint != "[REDACTED]" || keyHint("a") != keyHint("四字符钥") || keyHint("") != "" {
		t.Error("short key public hint reveals credential")
	}
	// Emulate a downgrade rewriting hints after the scrub marker exists.
	if _, err := s.db.Exec(`UPDATE providers SET api_key_hint=? WHERE id=?`, key, p.ID); err != nil {
		t.Fatal(err)
	}
	s.Close()
	originalKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"missing", "wrong"} {
		if mode == "missing" {
			if err := os.Rename(keyPath, keyPath+".saved"); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(keyPath, bytes.Repeat([]byte{23}, 32), 0600); err != nil {
			t.Fatal(err)
		}
		if opened, err := OpenStore(path); err == nil {
			opened.Close()
			t.Fatal("unpaired key restore accepted")
		}
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		var hint string
		err = db.QueryRow(`SELECT api_key_hint FROM providers WHERE id=?`, p.ID).Scan(&hint)
		db.Close()
		if err != nil || hint != key {
			t.Fatal("failed startup changed legacy credential metadata")
		}
		if mode == "missing" {
			if err := os.Rename(keyPath+".saved", keyPath); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(keyPath, originalKey, 0600); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	restored, err := s.Provider(ctx, p.ID)
	plain, openErr := s.secrets.Open(p.ID, restored.EncryptedKey)
	if err != nil || openErr != nil || plain != key || restored.APIKeyHint != "[REDACTED]" || restored.Version != p.Version || millis(restored.UpdatedAt) != millis(p.UpdatedAt) {
		t.Fatal("key hint repair changed provider configuration or key")
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		raw, err := os.ReadFile(path + suffix)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte(key)) {
			t.Fatalf("legacy hint remains in database artifact %s", suffix)
		}
	}
	session, err := s.CreateSession(ctx, Session{UserID: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	message, err := s.AddMessage(ctx, Message{SessionID: session.ID, Role: RoleUser, Content: key})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var content string
	if err := s.db.QueryRow(`SELECT content FROM messages WHERE id=?`, message.ID).Scan(&content); err != nil || content != key {
		t.Fatal("hint repair changed user-authored chat")
	}
}

func TestTransportRedactionPreservesTimeoutAndCancellation(t *testing.T) {
	key := "opaque /+credential"
	var percentEncoded strings.Builder
	for _, b := range []byte(key) {
		fmt.Fprintf(&percentEncoded, "%%%02x", b)
	}
	for _, encoded := range []string{key, url.PathEscape(key), url.QueryEscape(key), percentEncoded.String()} {
		err := safeProviderTransportError(&url.Error{Op: "Get", URL: "https://example.com/" + encoded, Err: context.DeadlineExceeded}, key)
		var u *url.Error
		var n net.Error
		if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &u) || !errors.As(err, &n) || !n.Timeout() || strings.Contains(u.Error(), encoded) {
			t.Fatal("credential redaction lost timeout/cancellation semantics or exposed encoded credential")
		}
	}
}

func TestTransportErrorsHideRedirectedAPIKeys(t *testing.T) {
	const key = "catalogSyntheticOpaqueKey123"
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		http.Redirect(w, r, "/opaque/"+key, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	ctx := context.Background()
	client := NewHTTPModelClient()
	for _, mode := range []string{"models", "chat", "responses", "anthropic", "gemini", "test-provider"} {
		t.Run(mode, func(t *testing.T) {
			p := Provider{Protocol: ProtocolOpenAICompatible, BaseURL: server.URL, EndpointScope: EndpointPrivate}
			var err error
			switch mode {
			case "models":
				_, err = client.Models(ctx, p, key)
			case "test-provider":
				s, openErr := OpenStore(filepath.Join(t.TempDir(), "ai.db"))
				if openErr != nil {
					t.Fatal(openErr)
				}
				defer s.Close()
				providers, _ := NewProviderService(s, s.secrets)
				secret := key
				p, err = providers.Save(ctx, "", ProviderInput{Name: "Redirect", Protocol: p.Protocol, BaseURL: p.BaseURL, EndpointScope: EndpointPrivate, PrivateConfirmed: true, APIKey: &secret})
				if err != nil {
					t.Fatal(err)
				}
				err = (&Service{Store: s, Providers: providers, client: client}).TestProvider(ctx, p.ID)
			default:
				if mode == "responses" {
					p.APIMode = OpenAIResponses
				}
				if mode == "anthropic" {
					p.Protocol = ProtocolAnthropic
				}
				if mode == "gemini" {
					p.Protocol = ProtocolGemini
				}
				err = client.Stream(ctx, p, key, CompletionRequest{Model: "test"}, func(CompletionEvent) error { return nil })
			}
			var urlErr *url.Error
			var netErr net.Error
			if err == nil || strings.Contains(err.Error(), key) || !errors.As(err, &urlErr) || !errors.As(err, &netErr) {
				t.Fatal("transport error exposes redirected credential or lost error semantics")
			}
		})
	}
	mu.Lock()
	defer mu.Unlock()
	if requests > 24 {
		t.Fatal(fmt.Sprintf("redirect mock exceeded finite request budget: %d", requests))
	}
}
