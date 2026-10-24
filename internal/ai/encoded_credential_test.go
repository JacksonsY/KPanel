package ai

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestEncodedCredentialStreamProtectsEveryChunkBoundary(t *testing.T) {
	const key = "test-key"
	for _, body := range []string{
		`{"note":"test\u002dkey","password":"new-password"}`,
		"```json\n{\"note\":\"test\\u002dkey\"}\n```",
		`{"note":"test\u002dkey`,
	} {
		for split := 0; split <= len(body); split++ {
			var output strings.Builder
			client := &publicOutputClient{pieces: []string{body[:split], body[split:]}}
			err := streamPublicCompletion(context.Background(), client, Provider{}, key, CompletionRequest{}, func(e CompletionEvent) error { output.WriteString(e.Delta); return nil }, false)
			if err != nil || strings.Contains(output.String(), `test\u002dkey`) || strings.Contains(output.String(), key) || !strings.Contains(output.String(), "[REDACTED]") {
				t.Fatalf("split=%d body=%q output=%q err=%v", split, body, output.String(), err)
			}
			if strings.Contains(body, "new-password") && !strings.Contains(output.String(), "new-password") {
				t.Fatal("unrelated generated password was changed")
			}
		}
	}
}

func TestEncodedCredentialAllProviderProtocols(t *testing.T) {
	for _, protocol := range []string{"chat", "responses", "anthropic", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for _, piece := range []string{`{"note":"test\u00`, `2dkey","password":"ordinary-password"}`} {
					quoted, _ := json.Marshal(piece)
					switch protocol {
					case "chat":
						fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", quoted)
					case "responses":
						fmt.Fprintf(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":%s}\n\n", quoted)
					case "anthropic":
						fmt.Fprintf(w, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":%s}}\n\n", quoted)
					case "gemini":
						fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":%s}]}}]}\n\n", quoted)
					}
				}
				if protocol == "anthropic" {
					fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n")
				} else if protocol != "gemini" {
					fmt.Fprint(w, "data: [DONE]\n\n")
				}
			}))
			defer server.Close()
			provider := Provider{Protocol: ProtocolOpenAICompatible, BaseURL: server.URL, EndpointScope: EndpointPrivate}
			switch protocol {
			case "responses":
				provider.APIMode = OpenAIResponses
			case "anthropic":
				provider.Protocol = ProtocolAnthropic
			case "gemini":
				provider.Protocol = ProtocolGemini
			}
			var output strings.Builder
			if err := streamPublicCompletion(context.Background(), NewHTTPModelClient(), provider, "test-key", CompletionRequest{Model: "model"}, func(e CompletionEvent) error { output.WriteString(e.Delta); return nil }, false); err != nil {
				t.Fatal(err)
			}
			if output.String() != `{"note":"[REDACTED]","password":"ordinary-password"}` {
				t.Fatalf("unsafe or changed output: %q", output.String())
			}
		})
	}
}

func TestEncodedCredentialUnicodeAndJSONSyntaxInKey(t *testing.T) {
	for _, key := range []string{`a"b\c`, "界😀key", "12345678", "a\nb\tc"} {
		var escaped strings.Builder
		escaped.WriteByte('"')
		for _, code := range utf16.Encode([]rune(key)) {
			fmt.Fprintf(&escaped, `\u%04x`, code)
		}
		escaped.WriteByte('"')
		body := `{"note":` + escaped.String() + `}`
		pieces := make([]string, len(body))
		for i := range body {
			pieces[i] = body[i : i+1]
		}
		var output strings.Builder
		if err := streamPublicCompletion(context.Background(), &publicOutputClient{pieces: pieces}, Provider{}, key, CompletionRequest{}, func(e CompletionEvent) error { output.WriteString(e.Delta); return nil }, false); err != nil {
			t.Fatal(err)
		}
		if output.String() != `{"note":"[REDACTED]"}` {
			t.Fatalf("key encoding not protected: %q", output.String())
		}
	}
	ordinary := `{"password":"ordinary-new-password","encoded":"\u003c","number":1234567890123456789} and unmatched "quoted text`
	var output strings.Builder
	if err := streamPublicCompletion(context.Background(), &publicOutputClient{pieces: []string{ordinary}}, Provider{}, "unrelated-key", CompletionRequest{}, func(e CompletionEvent) error { output.WriteString(e.Delta); return nil }, false); err != nil || output.String() != ordinary {
		t.Fatalf("ordinary source changed: %q %v", output.String(), err)
	}
}

type observedCredentialClient struct {
	publicOutputClient
	after func(int)
}

func (c *observedCredentialClient) Stream(_ context.Context, _ Provider, _ string, _ CompletionRequest, emit func(CompletionEvent) error) error {
	for index, piece := range c.pieces {
		if err := emit(CompletionEvent{Delta: piece}); err != nil {
			return err
		}
		c.after(index)
	}
	return emit(CompletionEvent{Done: true})
}

func TestEncodedCredentialStreamsProseBeforeQuotedTokenCloses(t *testing.T) {
	var output strings.Builder
	client := &observedCredentialClient{publicOutputClient: publicOutputClient{pieces: []string{"ordinary prose ", `{"note":"test\u002`, `dkey"} suffix`}}, after: func(index int) {
		if index == 0 && output.String() != "ordinary prose " {
			t.Fatal("ordinary prose was held until completion")
		}
		if index == 1 && strings.Contains(output.String(), "test") {
			t.Fatal("unverified quoted token escaped before closure")
		}
	}}
	if err := streamPublicCompletion(context.Background(), client, Provider{}, "test-key", CompletionRequest{}, func(e CompletionEvent) error { output.WriteString(e.Delta); return nil }, false); err != nil || output.String() != `ordinary prose {"note":"[REDACTED]"} suffix` {
		t.Fatalf("stream output=%q err=%v", output.String(), err)
	}
}

func TestEncodedCredentialProjectionHasCumulativeWorkBudget(t *testing.T) {
	body := strings.Repeat(`""`, 150000)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := streamPublicCompletion(context.Background(), &publicOutputClient{pieces: []string{body}}, Provider{}, "synthetic-key", CompletionRequest{}, func(CompletionEvent) error { return nil }, false)
	runtime.ReadMemStats(&after)
	t.Logf("wire=%d totalAlloc=%d", len(body), after.TotalAlloc-before.TotalAlloc)
	if !errors.Is(err, errCredentialProjectionBudget) {
		t.Fatalf("excessive token work accepted: %v", err)
	}
	if after.TotalAlloc-before.TotalAlloc > 64<<20 {
		t.Fatal("projection work used unbounded allocation")
	}
}

func TestEncodedCredentialRuntimeEventsHistoryAndContext(t *testing.T) {
	store, providers, provider, model := runtimeFixture(t)
	defer store.Close()
	ctx := context.Background()
	session, _ := store.CreateSession(ctx, Session{UserID: "admin"})
	run, _ := store.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID, ProviderID: provider.ID, ModelID: model.ID})
	_, _ = store.AddMessage(ctx, Message{SessionID: session.ID, RunID: run.ID, Role: RoleUser, Content: "ordinary user password unchanged"})
	hub := NewEventHub()
	events, unsubscribe := hub.Subscribe(run.ID)
	defer unsubscribe()
	r, err := NewNativeRuntime(store, providers, &publicOutputClient{pieces: []string{`{"note":"test\u002`, `dkey","password":"ordinary-new-password"}`}}, &fakeTools{}, hub)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Run(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	const want = `{"note":"[REDACTED]","password":"ordinary-new-password"}`
	var deltas strings.Builder
	for len(events) > 0 {
		event := <-events
		if event.Type == "message.delta" {
			deltas.WriteString(event.Data.(map[string]string)["delta"])
		}
	}
	if deltas.String() != want {
		t.Fatalf("public delta leaked or changed: %q", deltas.String())
	}
	messages, _, err := store.ContextMessages(ctx, session.ID, model.ContextWindow)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, message := range messages {
		if message.Role == RoleAssistant {
			found = true
			if message.Content != want {
				t.Fatalf("history/context retained encoded credential: %q", message.Content)
			}
		}
	}
	if !found {
		t.Fatal("assistant context missing")
	}
}

func TestEncodedCredentialMigrationProtectsAttributedCopiesOnly(t *testing.T) {
	store, _, provider, _ := runtimeFixture(t)
	ctx := context.Background()
	path := storePathForTest(t, store)
	session, _ := store.CreateSession(ctx, Session{UserID: "admin", ProviderID: provider.ID})
	run, _ := store.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID, ProviderID: provider.ID})
	const encoded = `{"note":"test\u002dkey","password":"ordinary-new-password"}`
	user := "user original " + encoded
	for _, message := range []Message{
		{ID: "encoded-user", SessionID: session.ID, Role: RoleUser, Content: user},
		{ID: "encoded-assistant", SessionID: session.ID, Role: RoleAssistant, ProviderID: provider.ID, Content: "legacy-assistant-only " + encoded},
		{ID: "encoded-run-assistant", SessionID: session.ID, RunID: run.ID, Role: RoleAssistant, Content: encoded},
		{ID: "unattributed", SessionID: session.ID, Role: RoleAssistant, ProviderID: "unrelated-provider", Content: "unattributed " + encoded},
		{ID: "unrelated-large-prose", SessionID: session.ID, Role: RoleAssistant, ProviderID: provider.ID, Content: strings.Repeat("z", 3<<20)},
	} {
		if _, err := store.AddMessage(ctx, message); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.Exec(`UPDATE sessions SET summary=? WHERE id=?`, "derived "+encoded, session.ID); err != nil {
		t.Fatal(err)
	}
	store.Close()
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for id, want := range map[string]string{
		"encoded-user":          user,
		"encoded-assistant":     `legacy-assistant-only {"note":"[REDACTED]","password":"ordinary-new-password"}`,
		"encoded-run-assistant": `{"note":"[REDACTED]","password":"ordinary-new-password"}`,
		"unattributed":          "unattributed " + encoded,
	} {
		var got string
		if err := reopened.db.QueryRow(`SELECT content FROM messages WHERE id=?`, id).Scan(&got); err != nil || got != want {
			t.Fatalf("%s got=%q err=%v", id, got, err)
		}
	}
	var summary string
	if err := reopened.db.QueryRow(`SELECT summary FROM sessions WHERE id=?`, session.ID).Scan(&summary); err != nil || summary != `derived {"note":"[REDACTED]","password":"ordinary-new-password"}` {
		t.Fatalf("summary=%q err=%v", summary, err)
	}
	var largeBytes int
	if err := reopened.db.QueryRow(`SELECT length(content) FROM messages WHERE id='unrelated-large-prose'`).Scan(&largeBytes); err != nil || largeBytes != 3<<20 {
		t.Fatalf("unrelated large prose was loaded or changed: %d %v", largeBytes, err)
	}
	reopened.Close()
	for _, suffix := range []string{"", "-wal", "-shm"} {
		data, err := os.ReadFile(path + suffix)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte("legacy-assistant-only "+encoded)) {
			t.Fatal("old encoded assistant copy remained after migration scrub")
		}
	}
}

func TestEncodedCredentialStreamHandlesNestedQuotedJSON(t *testing.T) {
	body := `{"note":"test\u002dkey","password":"ordinary-new-password"}`
	for depth := 0; depth < 6; depth++ {
		var output strings.Builder
		pieces := make([]string, len(body))
		for i := range body {
			pieces[i] = body[i : i+1]
		}
		err := streamPublicCompletion(context.Background(), &publicOutputClient{pieces: pieces}, Provider{}, "test-key", CompletionRequest{}, func(e CompletionEvent) error { output.WriteString(e.Delta); return nil }, false)
		if err != nil || strings.Contains(output.String(), "test") || !strings.Contains(output.String(), "ordinary-new-password") {
			t.Fatalf("depth=%d output=%q err=%v", depth, output.String(), err)
		}
		raw, _ := json.Marshal(body)
		body = string(raw)
	}
}

func TestEncodedCredentialLearningRejectsBeforeCompletion(t *testing.T) {
	done := false
	err := streamPublicCompletion(context.Background(), &publicOutputClient{pieces: []string{`{"note":"test\u002dkey"}`}}, Provider{}, "test-key", CompletionRequest{}, func(e CompletionEvent) error { done = done || e.Done; return nil }, true)
	if err == nil || done {
		t.Fatal("encoded credential reached completed learning output")
	}
}

func TestEncodedCredentialMigrationFailureRollsBackPublicCopies(t *testing.T) {
	store, _, provider, _ := runtimeFixture(t)
	ctx := context.Background()
	path := storePathForTest(t, store)
	session, _ := store.CreateSession(ctx, Session{UserID: "admin", ProviderID: provider.ID})
	const source = `{"note":"test\u002dkey"}`
	for _, id := range []string{"encoded-a", "encoded-b"} {
		if _, err := store.AddMessage(ctx, Message{ID: id, SessionID: session.ID, ProviderID: provider.ID, Role: RoleAssistant, Content: source}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.Exec(`CREATE TRIGGER fail_encoded_projection BEFORE UPDATE OF content ON messages WHEN OLD.id='encoded-b' BEGIN SELECT RAISE(ABORT,'injected projection failure'); END`); err != nil {
		t.Fatal(err)
	}
	store.Close()
	if opened, err := OpenStore(path); err == nil {
		opened.Close()
		t.Fatal("failed projection exposed a partial service")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM messages WHERE content=?`, source).Scan(&count); err != nil || count != 2 {
		t.Fatalf("failed projection partially modified history: %d %v", count, err)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_encoded_projection`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.db.QueryRow(`SELECT count(*) FROM messages WHERE content=?`, `{"note":"[REDACTED]"}`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("projection retry lost original history: %d %v", count, err)
	}
}

type encodedCredentialErrorClient struct {
	publicOutputClient
	err error
}

func (c *encodedCredentialErrorClient) Stream(context.Context, Provider, string, CompletionRequest, func(CompletionEvent) error) error {
	return c.err
}

func TestEncodedCredentialFailureKeepsRetryAndCancellationIdentity(t *testing.T) {
	providerErr := &ProviderError{Status: 429, Message: `{"note":"test\u002dkey"}`, Retryable: true}
	cause := errors.Join(context.Canceled, providerErr)
	err := streamPublicCompletion(context.Background(), &encodedCredentialErrorClient{err: cause}, Provider{}, "test-key", CompletionRequest{}, func(CompletionEvent) error { return nil }, false)
	var found *ProviderError
	if !errors.Is(err, context.Canceled) || !errors.As(err, &found) || found.Status != 429 || !found.Retryable || strings.Contains(err.Error(), `test\u002dkey`) {
		t.Fatalf("failure credential projection lost identity or leaked: %v", err)
	}
}

func TestEncodedCredentialNestedToolArgumentsRejectWithoutMutation(t *testing.T) {
	raw := json.RawMessage(`{"note":"{\"payload\":\"test\\u002dkey\"}"}`)
	before := append([]byte(nil), raw...)
	forwarded := false
	err := streamPublicCompletion(context.Background(), &publicOutputClient{calls: []ToolCall{{ID: "call", Name: "host_action", Arguments: raw}}}, Provider{}, "test-key", CompletionRequest{}, func(e CompletionEvent) error { forwarded = forwarded || len(e.ToolCalls) > 0; return nil }, false)
	if err == nil || forwarded || !bytes.Equal(before, raw) {
		t.Fatalf("encoded tool credential executed or rewritten: %v", err)
	}
}

func TestEncodedCredentialMarkerCannotCauseRepeatedProjectionGrowth(t *testing.T) {
	for _, key := range []string{"A", "RED", "[", "]", "REDACTED", "normal-key", "hidden", "界"} {
		raw, _ := json.Marshal(map[string]string{"note": key})
		first, found, err := knownCredentialText(string(raw), key)
		if err != nil || !found {
			t.Fatalf("missing first projection: %q %v", first, err)
		}
		for range 4 {
			next, changed, err := knownCredentialText(first, key)
			if err != nil || changed || first != next {
				t.Fatalf("key=%q repeated projection grew: %q -> %q err=%v", key, first, next, err)
			}
		}
		for _, body := range []string{string(raw), "password=" + key, "-----BEGIN PRIVATE KEY-----\nbody\n-----END PRIVATE KEY-----"} {
			projected := publicProviderText(body, key)
			if next := publicProviderText(projected, key); projected != next {
				t.Fatalf("key=%q tool projection grew: %q -> %q", key, projected, next)
			}
		}
	}
}

func TestEncodedCredentialShortKeyMigrationRemainsStableAcrossRestarts(t *testing.T) {
	store, _, provider, _ := runtimeFixture(t)
	ctx := context.Background()
	path := storePathForTest(t, store)
	encrypted, err := store.secrets.Seal(provider.ID, "A")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE providers SET encrypted_key=?,api_key_hint='[REDACTED]' WHERE id=?`, encrypted, provider.ID); err != nil {
		t.Fatal(err)
	}
	session, _ := store.CreateSession(ctx, Session{UserID: "admin", ProviderID: provider.ID})
	run, _ := store.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID, ProviderID: provider.ID})
	const original = `{"note":"A","safe":"visible"}`
	if _, err := store.AddMessage(ctx, Message{ID: "short-key-assistant", SessionID: session.ID, ProviderID: provider.ID, Role: RoleAssistant, Content: original}); err != nil {
		t.Fatal(err)
	}
	call, err := store.SaveToolCall(ctx, ToolCall{ID: "short-key-tool", RunID: run.ID, SessionID: session.ID, Name: "host_read", Arguments: json.RawMessage(`{}`), ResultPreview: original, Status: ToolCompleted})
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	for range 3 {
		store, err = OpenStore(path)
		if err != nil {
			t.Fatal(err)
		}
		var assistant string
		if err := store.db.QueryRow(`SELECT content FROM messages WHERE id='short-key-assistant'`).Scan(&assistant); err != nil || assistant != `{"note":"(hidden)","safe":"visible"}` {
			t.Fatalf("assistant migration grew: %q %v", assistant, err)
		}
		tool, err := store.ToolCall(ctx, run.ID, call.ID)
		if err != nil || tool.ResultPreview != `{"note":"(hidden)","safe":"visible"}` {
			t.Fatalf("tool migration grew or lost ordinary data: %q %v", tool.ResultPreview, err)
		}
		store.Close()
	}
}
