package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type publicOutputClient struct {
	pieces []string
	calls  []ToolCall
}

func TestPublicOutputFourProtocols(t *testing.T) {
	for _, protocol := range []string{"chat", "responses", "anthropic", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				for _, piece := range []string{"opaque-", "credential"} {
					switch protocol {
					case "chat":
						fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", piece)
					case "responses":
						fmt.Fprintf(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":%q}\n\n", piece)
					case "anthropic":
						fmt.Fprintf(w, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\n", piece)
					case "gemini":
						fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":%q}]}}]}\n\n", piece)
					}
				}
				switch protocol {
				case "chat":
					fmt.Fprint(w, "data: [DONE]\n\n")
				case "responses":
					fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{}}\n\n")
				case "anthropic":
					fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n")
				}
			}))
			defer server.Close()
			p := Provider{Protocol: ProtocolOpenAICompatible, BaseURL: server.URL, EndpointScope: EndpointPrivate}
			switch protocol {
			case "responses":
				p.APIMode = OpenAIResponses
			case "anthropic":
				p.Protocol = ProtocolAnthropic
			case "gemini":
				p.Protocol = ProtocolGemini
			}
			var out strings.Builder
			err := streamPublicCompletion(context.Background(), NewHTTPModelClient(), p, "opaque-credential", CompletionRequest{Model: "test"}, func(e CompletionEvent) error { out.WriteString(e.Delta); return nil }, false)
			if err != nil || out.String() != "[REDACTED]" {
				t.Fatalf("output=%q err=%v", out.String(), err)
			}
		})
	}
}

func TestKnownCredentialMigrationPreservesUserChat(t *testing.T) {
	store, _, provider, _ := runtimeFixture(t)
	ctx := context.Background()
	path := storePathForTest(t, store)
	session, err := store.CreateSession(ctx, Session{UserID: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	user := "user keeps test-key and ordinary-password"
	assistant := "provider echoed test-key but ordinary-password stays"
	for _, m := range []Message{{ID: "user-kept", SessionID: session.ID, Role: RoleUser, Content: user}, {ID: "assistant-fixed", SessionID: session.ID, Role: RoleAssistant, ProviderID: provider.ID, Content: assistant}} {
		if _, err := store.AddMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.protectToolState(ctx, path); err != nil {
		t.Fatal(err)
	}
	rows, err := store.Messages(ctx, session.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range rows {
		if m.Role == RoleUser && m.Content != user {
			t.Fatal("user chat changed")
		}
		if m.Role == RoleAssistant && m.Content != "provider echoed [REDACTED] but ordinary-password stays" {
			t.Fatalf("assistant=%q", m.Content)
		}
	}
	store.Close()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), assistant) {
		t.Fatal("obsolete assistant plaintext page retained")
	}
}

func storePathForTest(t *testing.T, s *Store) string {
	t.Helper()
	var seq int
	var name, path string
	if err := s.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	return path
}

func (*publicOutputClient) Models(context.Context, Provider, string) ([]Model, error) {
	return nil, nil
}
func (c *publicOutputClient) Stream(_ context.Context, _ Provider, _ string, _ CompletionRequest, emit func(CompletionEvent) error) error {
	for _, p := range c.pieces {
		if err := emit(CompletionEvent{Delta: p}); err != nil {
			return err
		}
	}
	return emit(CompletionEvent{Done: true, ToolCalls: c.calls})
}

func TestPublicOutputKnownCredentialAcrossChunks(t *testing.T) {
	key := "opaque-credential-12345"
	for split := 0; split <= len(key); split++ {
		r := &NativeRuntime{client: &publicOutputClient{pieces: []string{"ordinary password new-value " + key[:split], key[split:] + " suffix"}}}
		emitted := false
		var out strings.Builder
		err := r.streamWithRetry(context.Background(), Provider{}, key, CompletionRequest{}, func(e CompletionEvent) error {
			out.WriteString(e.Delta)
			emitted = emitted || e.Delta != ""
			return nil
		}, &emitted)
		if err != nil || out.String() != "ordinary password new-value [REDACTED] suffix" {
			t.Fatalf("split %d: %q %v", split, out.String(), err)
		}
	}
}

func TestCredentialFilterRepeatedPrefixes(t *testing.T) {
	for _, key := range []string{"aaaaab", "ababab", "aba", "界界a", ""} {
		text := strings.Repeat("aaaaaabababab界界b", 1000) + key + key + "tail"
		filter := newCredentialFilter(key)
		var output strings.Builder
		for i := range len(text) {
			output.WriteString(filter.write(text[i : i+1]))
		}
		output.WriteString(filter.finish())
		want := text
		if key != "" {
			want = strings.ReplaceAll(text, key, "[REDACTED]")
		}
		if output.String() != want {
			t.Fatalf("filter changed ordinary bytes for %q", key)
		}
	}
}

func TestPublicOutputRejectsCredentialToolArguments(t *testing.T) {
	key := "opaque-credential-12345"
	raw := json.RawMessage(`{"path":"opaque-credential-12345"}`)
	r := &NativeRuntime{client: &publicOutputClient{calls: []ToolCall{{Name: "host_action", Arguments: raw}}}}
	emitted := false
	forwarded := false
	err := r.streamWithRetry(context.Background(), Provider{}, key, CompletionRequest{}, func(e CompletionEvent) error { forwarded = forwarded || len(e.ToolCalls) > 0; return nil }, &emitted)
	if err == nil || forwarded || string(raw) != `{"path":"opaque-credential-12345"}` {
		t.Fatalf("tool credential forwarded=%v err=%v", forwarded, err)
	}
}

func TestContentPageBudgetCursorCompleteness(t *testing.T) {
	store, _, _, _ := runtimeFixture(t)
	defer store.Close()
	ctx := context.Background()
	session, err := store.CreateSession(ctx, Session{UserID: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Repeat("\x01", 96<<10)
	for i := 0; i < 50; i++ {
		if _, err := store.AddMessage(ctx, Message{ID: fmt.Sprintf("page-%02d", i), SessionID: session.ID, Role: RoleAssistant, Content: content}); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	cursor := ""
	for pages := 0; pages < 51; pages++ {
		page, err := store.ConversationMessageMetadataPage(ctx, session.ID, 50, cursor)
		if err != nil {
			t.Fatal(err)
		}
		total := 0
		for _, m := range page.Items {
			total += len(m.Content)
			if seen[m.ID] {
				t.Fatal("duplicate cursor row")
			}
			seen[m.ID] = true
		}
		if total > 2<<20 {
			t.Fatalf("unbounded content page: %d", total)
		}
		if page.NextCursor == "" {
			break
		}
		if page.NextCursor == cursor || len(page.Items) == 0 {
			t.Fatal("cursor stalled")
		}
		cursor = page.NextCursor
	}
	if len(seen) != 50 {
		t.Fatalf("lost rows: %d", len(seen))
	}
}

func TestPublicOutputLearningRejectsCredentialBeforeDryRun(t *testing.T) {
	for _, value := range []string{"opaque-credential", `opaque-\u0063redential`} {
		t.Run(value, func(t *testing.T) {
			store, providers, provider, model := runtimeFixture(t)
			defer store.Close()
			ctx := context.Background()
			session, _ := store.CreateSession(ctx, Session{UserID: "admin"})
			run, _ := store.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID})
			_, err := store.SaveToolCall(ctx, ToolCall{ID: "learning", RunID: run.ID, SessionID: session.ID, Name: "host_action", Arguments: json.RawMessage(`{}`), Status: ToolCompleted})
			if err != nil {
				t.Fatal(err)
			}
			tools := &learningArgumentsTools{}
			output := `{"decision":"procedure","title":"safe","steps":[{"tool":"host_action","arguments":{"path":"` + value + `"}}]}`
			runtime, err := NewNativeRuntime(store, providers, &learningClient{output: output}, tools, NewEventHub())
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if err := runtime.generateProposal(ctx, run, provider, "opaque-credential", model, nil, true); err == nil {
				t.Fatal("credential accepted in learning arguments")
			}
			if len(tools.checked) > 0 {
				t.Fatal("rewritten learning arguments reached dry-run")
			}
			proposals, err := store.Proposals(ctx, "admin", EvolutionPending)
			if err != nil || len(proposals) > 0 {
				t.Fatal("credential proposal retained")
			}
		})
	}
}

func TestKnownCredentialLegacyToolPreservesArgsAndBlocksExecution(t *testing.T) {
	store, providers, provider, model := runtimeFixture(t)
	defer store.Close()
	ctx := context.Background()
	session, _ := store.CreateSession(ctx, Session{UserID: "admin"})
	run, _ := store.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID, ProviderID: provider.ID, ModelID: model.ID})
	args := json.RawMessage(`{"path":"test-key"}`)
	_, err := store.SaveToolCall(ctx, ToolCall{ID: "legacy-known", RunID: run.ID, SessionID: session.ID, Name: "host_action", Arguments: args, Status: ToolPendingApproval, RequiresApproval: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.protectToolState(ctx, storePathForTest(t, store)); err != nil {
		t.Fatal(err)
	}
	call, err := store.ToolCall(ctx, run.ID, "legacy-known")
	if err != nil {
		t.Fatal(err)
	}
	if string(call.Arguments) != string(args) || call.Status != ToolPendingApproval {
		t.Fatal("migration changed private execution state")
	}
	raw, err := json.Marshal(call)
	if err != nil || strings.Contains(string(raw), "test-key") {
		t.Fatalf("public legacy preview leaked: %s %v", raw, err)
	}
	tools := &fakeTools{}
	r, err := NewNativeRuntime(store, providers, &publicOutputClient{}, tools, NewEventHub())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.executeTool(ctx, &run, &call); err == nil || tools.executed != 0 {
		t.Fatal("legacy credential executed")
	}
}

func TestPublicOutputRuntimeSinks(t *testing.T) {
	store, providers, provider, model := runtimeFixture(t)
	defer store.Close()
	ctx := context.Background()
	session, _ := store.CreateSession(ctx, Session{UserID: "admin"})
	_, _ = store.AddMessage(ctx, Message{SessionID: session.ID, Role: RoleUser, Content: "ordinary user password unchanged"})
	run, _ := store.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID, ProviderID: provider.ID, ModelID: model.ID})
	hub := NewEventHub()
	events, unsubscribe := hub.Subscribe(run.ID)
	defer unsubscribe()
	r, err := NewNativeRuntime(store, providers, &publicOutputClient{pieces: []string{"reply test-", "key ordinary new-password"}}, &fakeTools{}, hub)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Run(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	for len(events) > 0 {
		event := <-events
		raw, _ := json.Marshal(event)
		if strings.Contains(string(raw), "test-key") {
			t.Fatal("credential event leaked")
		}
	}
	messages, _, err := store.ContextMessages(ctx, session.ID, model.ContextWindow)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range messages {
		if strings.Contains(m.Content, "test-key") {
			t.Fatal("credential persisted/context leaked")
		}
		if m.Role == RoleAssistant && m.Content != "reply [REDACTED] ordinary new-password" {
			t.Fatalf("reply changed %q", m.Content)
		}
	}
}

func TestContentPageRejectsSingleOversizedContent(t *testing.T) {
	store, _, _, _ := runtimeFixture(t)
	defer store.Close()
	ctx := context.Background()
	session, _ := store.CreateSession(ctx, Session{UserID: "admin"})
	_, err := store.AddMessage(ctx, Message{SessionID: session.ID, Role: RoleAssistant, Content: strings.Repeat("界", (2<<20)/3+1)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConversationMessageMetadataPage(ctx, session.ID, 50, ""); err == nil {
		t.Fatal("oversized message silently truncated")
	}
}
