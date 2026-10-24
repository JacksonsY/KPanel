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
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSSECumulativeBudgets(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"event", strings.Repeat("data: "+strings.Repeat("x", 64<<10)+"\n", 48) + "\n"},
		{"wire", strings.Repeat(":"+strings.Repeat("x", 64<<10)+"\n\n", 130)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := scanSSE(strings.NewReader(tc.input), func(string, string) error { return nil }); err == nil {
				t.Fatal("unbounded input accepted")
			}
		})
	}
	want := errors.New("cancelled consumer")
	if err := scanSSE(strings.NewReader("data: first\n\ndata: second\n\n"), func(string, string) error { return want }); !errors.Is(err, want) {
		t.Fatal(err)
	}
}

func TestEveryProviderRejectsToolBudgetBeforeCompletion(t *testing.T) {
	for _, protocol := range []string{"chat", "responses", "anthropic", "gemini"} {
		for _, kind := range []string{"arguments", "name", "count"} {
			t.Run(protocol+"/"+kind, func(t *testing.T) {
				var stream strings.Builder
				write := func(value any) { raw, _ := json.Marshal(value); fmt.Fprintf(&stream, "data: %s\n\n", raw) }
				count := 1
				if kind == "count" {
					count = MaxToolCalls + 1
				}
				for index := 0; index < count; index++ {
					name, args := "host_read", "{}"
					if kind == "name" {
						name = strings.Repeat("n", maxToolNameBytes+1)
					}
					if kind == "arguments" {
						args = `{"value":"` + strings.Repeat("a", MaxToolResultBytes) + `"}`
					}
					switch protocol {
					case "chat":
						write(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{map[string]any{"index": index, "id": fmt.Sprint(index), "function": map[string]any{"name": name, "arguments": args[:len(args)/2]}}}}}}})
						write(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{map[string]any{"index": index, "function": map[string]any{"arguments": args[len(args)/2:]}}}}}}})
					case "responses":
						write(map[string]any{"type": "response.output_item.added", "output_index": index, "item": map[string]any{"type": "function_call", "call_id": fmt.Sprint(index), "name": name, "arguments": ""}})
						for _, part := range []string{args[:len(args)/2], args[len(args)/2:]} {
							write(map[string]any{"type": "response.function_call_arguments.delta", "output_index": index, "delta": part})
						}
					case "anthropic":
						write(map[string]any{"type": "content_block_start", "index": index, "content_block": map[string]any{"type": "tool_use", "id": fmt.Sprint(index), "name": name}})
						for _, part := range []string{args[:len(args)/2], args[len(args)/2:]} {
							write(map[string]any{"type": "content_block_delta", "index": index, "delta": map[string]any{"type": "input_json_delta", "partial_json": part}})
						}
					case "gemini":
						write(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"functionCall": map[string]any{"name": name, "args": json.RawMessage(args)}}}}}}})
					}
				}
				switch protocol {
				case "chat", "responses":
					stream.WriteString("data: [DONE]\n\n")
				case "anthropic":
					write(map[string]string{"type": "message_stop"})
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, stream.String())
				}))
				defer server.Close()
				provider := Provider{Protocol: ProtocolOpenAICompatible, EndpointScope: EndpointPrivate, BaseURL: server.URL}
				if protocol == "responses" {
					provider.APIMode = OpenAIResponses
				}
				if protocol == "anthropic" {
					provider.Protocol = ProtocolAnthropic
				}
				if protocol == "gemini" {
					provider.Protocol = ProtocolGemini
				}
				done := false
				err := NewHTTPModelClient().Stream(context.Background(), provider, "", CompletionRequest{Model: "test"}, func(e CompletionEvent) error { done = done || e.Done; return nil })
				if err == nil || done {
					t.Fatalf("over-budget response reached completion: done=%v err=%v", done, err)
				}
			})
		}
	}
}

func TestCompletionBudgetIncludesBackgroundCollectors(t *testing.T) {
	var output strings.Builder
	emit := boundedCompletion(func(e CompletionEvent) error { output.WriteString(e.Delta); return nil })
	if err := emit(CompletionEvent{Delta: strings.Repeat("a", MaxAssistantBytes)}); err != nil {
		t.Fatal(err)
	}
	if err := emit(CompletionEvent{Delta: "b"}); err == nil || output.Len() != MaxAssistantBytes {
		t.Fatal("collector grew beyond its budget")
	}
	parts := map[int]*ToolCall{}
	if err := updateTool(parts, 0, "id", strings.Repeat("n", 128), "", true, true); err != nil {
		t.Fatal(err)
	}
	if err := updateTool(parts, 0, "", strings.Repeat("n", 129), "", true, true); err == nil || len(parts[0].Name) != 128 {
		t.Fatal("oversized name fragment retained")
	}
}

func TestPrivateToolStateMigrationPreservesApprovalAndScrubsPlaintext(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "ai.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.CreateSession(ctx, Session{UserID: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.CreateRun(ctx, Run{SessionID: session.ID, UserID: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	marker := "migration-synthetic-private-credential"
	arguments := json.RawMessage(`{"path":"/tmp/test","password":"` + marker + `","content":"password=` + marker + `"}`)
	native := json.RawMessage(`{"opaque":"` + marker + `"}`)
	legacyResult := `{"content":"{\"auth\":\"` + marker + `\",\"note\":\"ordinary result\"}"}`
	for _, status := range []ToolCallStatus{ToolPendingApproval, ToolCompleted} {
		call, err := s.SaveToolCall(ctx, ToolCall{ID: string(status), RunID: run.ID, SessionID: session.ID, Name: "host_file_write", Arguments: arguments, ProviderData: native, Status: status, RequiresApproval: true})
		if err != nil {
			t.Fatal(err)
		}
		// Emulate the old binary after a previous schema-11 startup. The scrub
		// marker must be invalidated again before committing the conversion.
		if _, err := s.db.ExecContext(ctx, `UPDATE tool_calls SET arguments=?,provider_data=?,private_state=X'',arguments_preview=?,result_preview=? WHERE id=?`, []byte(arguments), []byte(native), string(arguments), legacyResult, call.ID); err != nil {
			t.Fatal(err)
		}
	}
	toolMessage, err := s.AddMessage(ctx, Message{SessionID: session.ID, RunID: run.ID, Role: RoleTool, ToolCallID: string(ToolCompleted), Content: legacyResult})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE messages SET content=? WHERE id=?`, legacyResult, toolMessage.ID); err != nil {
		t.Fatal(err)
	}
	userMessage, err := s.AddMessage(ctx, Message{SessionID: session.ID, Role: RoleUser, Content: "user-authored-content-retained"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, status := range []ToolCallStatus{ToolPendingApproval, ToolCompleted} {
		call, err := s.ToolCall(ctx, run.ID, string(status))
		if err != nil {
			t.Fatal(err)
		}
		if call.Status != status || !call.RequiresApproval || !bytes.Equal(call.Arguments, arguments) || !bytes.Equal(call.ProviderData, native) {
			t.Fatalf("private state changed: %#v", call)
		}
		if strings.Contains(call.ResultPreview, marker) || !strings.Contains(call.ResultPreview, "ordinary result") {
			t.Fatalf("historical tool result was not projected: %s", call.ResultPreview)
		}
		public, _ := json.Marshal(call)
		if bytes.Contains(public, []byte(marker)) || bytes.Contains(public, []byte(`"arguments":`)) {
			t.Fatalf("private data exposed: %s", public)
		}
	}
	var restoredUser, restoredTool string
	if err := s.db.QueryRowContext(ctx, `SELECT content FROM messages WHERE id=?`, userMessage.ID).Scan(&restoredUser); err != nil || restoredUser != userMessage.Content {
		t.Fatalf("user message changed: %s %v", restoredUser, err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT content FROM messages WHERE id=?`, toolMessage.ID).Scan(&restoredTool); err != nil || strings.Contains(restoredTool, marker) || !strings.Contains(restoredTool, "ordinary result") {
		t.Fatalf("historical provider tool context leaked: %s %v", restoredTool, err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		data, err := os.ReadFile(path + suffix)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(marker)) {
			t.Fatalf("legacy plaintext remains in ai.db%s", suffix)
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE tool_calls SET private_state=(SELECT private_state FROM tool_calls WHERE id=?) WHERE id=?`, string(ToolCompleted), string(ToolPendingApproval)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ToolCall(ctx, run.ID, string(ToolPendingApproval)); err == nil {
		t.Fatal("ciphertext substitution accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "ai-secrets.key")
	if err := os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}
	if opened, err := OpenStore(path); err == nil {
		opened.Close()
		t.Fatal("missing private-state key did not fail closed")
	}
	if _, err := os.Stat(keyPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("missing key was replaced")
	}
}

func TestPrivateMigrationFailureRollsBackAndCanResume(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ai.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	session, _ := s.CreateSession(ctx, Session{UserID: "admin"})
	run, _ := s.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID})
	args := []byte(`{"path":"/tmp/migration","content":"retained input"}`)
	for _, id := range []string{"a", "b"} {
		_, err := s.SaveToolCall(ctx, ToolCall{ID: id, RunID: run.ID, SessionID: session.ID, Name: "host_file_write", Arguments: args, Status: ToolPendingApproval, RequiresApproval: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE tool_calls SET arguments=?,private_state=X'' WHERE id=?`, args, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER fail_private_migration BEFORE UPDATE OF private_state ON tool_calls WHEN OLD.id='b' BEGIN SELECT RAISE(ABORT,'injected migration failure'); END`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if opened, err := OpenStore(path); err == nil {
		opened.Close()
		t.Fatal("migration failure exposed a partial service")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	var retained int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM tool_calls WHERE length(private_state)=0 AND arguments=? AND status='pending_approval'`, args).Scan(&retained); err != nil || retained != 2 {
		t.Fatalf("migration did not roll back: %d %v", retained, err)
	}
	if _, err := db.ExecContext(ctx, `DROP TRIGGER fail_private_migration`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"a", "b"} {
		call, err := s.ToolCall(ctx, run.ID, id)
		if err != nil || !bytes.Equal(call.Arguments, args) || call.Status != ToolPendingApproval {
			t.Fatalf("migration retry lost pending state: %#v %v", call, err)
		}
	}
}

func TestToolCallIdentityAndApprovalArgumentsAreImmutable(t *testing.T) {
	s, _, _, _ := runtimeFixture(t)
	defer s.Close()
	ctx := context.Background()
	session, _ := s.CreateSession(ctx, Session{UserID: "admin"})
	run, _ := s.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID})
	call, err := s.SaveToolCall(ctx, ToolCall{ID: "fixed-call", RunID: run.ID, SessionID: session.ID, Name: "host_file_write", Arguments: json.RawMessage(`{"path":"/tmp/a","content":"nginx configuration"}`), Status: ToolPendingApproval, RequiresApproval: true})
	if err != nil {
		t.Fatal(err)
	}
	public, _ := json.Marshal(call)
	if !bytes.Contains(public, []byte("nginx configuration")) || !bytes.Contains(public, []byte(`"approvalPreview"`)) {
		t.Fatalf("approval cannot inspect concrete write: %s", public)
	}
	var persistedPreview string
	if err := s.db.QueryRowContext(ctx, `SELECT arguments_preview FROM tool_calls WHERE id=?`, call.ID).Scan(&persistedPreview); err != nil || strings.Contains(persistedPreview, "nginx configuration") {
		t.Fatalf("approval details persisted in clear: %s %v", persistedPreview, err)
	}
	changed := call
	changed.Arguments = json.RawMessage(`{"path":"/tmp/b","content":"different"}`)
	if _, err := s.SaveToolCall(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("approval arguments replaced: %v", err)
	}
	changed = call
	changed.SessionID = "other-session"
	if _, err := s.SaveToolCall(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("approval identity replaced: %v", err)
	}
	call.Status = ToolCompleted
	if _, err := s.SaveToolCall(ctx, call); err != nil {
		t.Fatal(err)
	}
	public, _ = json.Marshal(call)
	if bytes.Contains(public, []byte("nginx configuration")) || bytes.Contains(public, []byte(`"approvalPreview"`)) {
		t.Fatalf("completed call exposes private details: %s", public)
	}
	call.Status = ToolPendingApproval
	if _, err := s.SaveToolCall(ctx, call); !errors.Is(err, ErrConflict) {
		t.Fatalf("completed tool replayed into approval: %v", err)
	}
}

func TestPendingApprovalCanReviewCommandsAndInteractiveInput(t *testing.T) {
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"command":["mysql","-p","command-password"],"containerId":"container"}`),
		json.RawMessage(`{"data":"print current status\npassword=input-password"}`),
	} {
		call := ToolCall{Status: ToolPendingApproval, Arguments: raw, ArgumentsPreview: safeToolArgumentsPreview(raw)}
		public, err := json.Marshal(call)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(public, []byte("command-password")) || bytes.Contains(public, []byte("input-password")) {
			t.Fatalf("known credentials leaked in approval: %s", public)
		}
		if !bytes.Contains(public, []byte("mysql")) && !bytes.Contains(public, []byte("print current status")) {
			t.Fatalf("approval cannot inspect the requested input: %s", public)
		}
		if strings.Contains(call.ArgumentsPreview, "mysql") || strings.Contains(call.ArgumentsPreview, "print current status") {
			t.Fatal("private execution content persisted in the preview")
		}
	}
}

func TestPrivateToolBackupRequiresMatchingKey(t *testing.T) {
	ctx := context.Background()
	source := t.TempDir()
	s, err := OpenStore(filepath.Join(source, "ai.db"))
	if err != nil {
		t.Fatal(err)
	}
	session, _ := s.CreateSession(ctx, Session{UserID: "admin"})
	run, _ := s.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID})
	call, err := s.SaveToolCall(ctx, ToolCall{ID: "backup", RunID: run.ID, SessionID: session.ID, Name: "host_read", Arguments: json.RawMessage(`{"path":"/root/.docker/config.json"}`), ProviderData: json.RawMessage(`{"opaque":"saved-native-state"}`), Status: ToolPendingApproval, RequiresApproval: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	for _, name := range []string{"ai.db", "ai-secrets.key"} {
		data, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destination, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := OpenStore(filepath.Join(destination, "ai.db"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := restored.ToolCall(ctx, run.ID, call.ID)
	if err != nil || !bytes.Equal(loaded.Arguments, call.Arguments) || !bytes.Equal(loaded.ProviderData, call.ProviderData) || loaded.Status != ToolPendingApproval {
		t.Fatalf("backup state mismatch: %#v %v", loaded, err)
	}
	restored.Close()
	if err := os.WriteFile(filepath.Join(destination, "ai-secrets.key"), bytes.Repeat([]byte{1}, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	if opened, err := OpenStore(filepath.Join(destination, "ai.db")); err == nil {
		opened.Close()
		t.Fatal("wrong backup key accepted")
	}
}

func TestBackgroundLearningRejectsOversizedProviderOutput(t *testing.T) {
	s, providers, provider, model := runtimeFixture(t)
	defer s.Close()
	ctx := context.Background()
	session, _ := s.CreateSession(ctx, Session{UserID: "admin"})
	run, _ := s.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID})
	runtime, err := NewNativeRuntime(s, providers, &learningClient{output: strings.Repeat("x", MaxAssistantBytes+1)}, &fakeTools{}, NewEventHub())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	err = runtime.generateProposal(ctx, run, provider, "", model, []Message{{Role: RoleUser, Content: "remember this preference"}}, false)
	if err == nil || !strings.Contains(err.Error(), "response budget") {
		t.Fatalf("background collector did not reject oversized output: %v", err)
	}
	memories, _ := s.Memories(ctx, "admin")
	if len(memories) != 0 {
		t.Fatal("oversized learning output was saved")
	}
}

type approvalContextTools struct {
	fakeTools
	contexts []ToolExecutionContext
}

func (tools *approvalContextTools) Execute(ctx context.Context, execution ToolExecutionContext, name string, args json.RawMessage) (string, error) {
	tools.contexts = append(tools.contexts, execution)
	return tools.fakeTools.Execute(ctx, execution, name, args)
}

func TestReadDataGrantStillRequiresHumanApprovalInAutoMode(t *testing.T) {
	s, providers, provider, model := runtimeFixture(t)
	defer s.Close()
	ctx := context.Background()
	session, _ := s.CreateSession(ctx, Session{UserID: "admin", ProviderID: provider.ID, ModelID: model.ID})
	run, _ := s.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID, ProviderID: provider.ID, ModelID: model.ID, ApprovalMode: ApprovalAuto})
	required := true
	tools := &approvalContextTools{fakeTools: fakeTools{readOnly: true, approval: &required}}
	runtime, err := NewNativeRuntime(s, providers, &scriptedClient{tool: "host_action"}, tools, NewEventHub())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if err := runtime.Run(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	loaded, _ := s.RunByID(ctx, run.ID)
	if loaded.Status != RunPendingApproval || len(tools.contexts) != 0 {
		t.Fatal("auto mode read before the human decision")
	}
	calls, err := s.ToolCalls(ctx, run.ID)
	if err != nil || len(calls) != 1 {
		t.Fatalf("pending calls: %#v %v", calls, err)
	}
	if err := runtime.Resume(ctx, run.ID, Decision{ToolCallID: calls[0].ID, Approve: true}); err != nil {
		t.Fatal(err)
	}
	if len(tools.contexts) != 1 || !tools.contexts[0].Approved || tools.contexts[0].ToolCallID != calls[0].ID || tools.contexts[0].SessionID != session.ID {
		t.Fatalf("decision not bound to the approved call: %#v", tools.contexts)
	}
	if err := runtime.Resume(ctx, run.ID, Decision{ToolCallID: calls[0].ID, Approve: true}); !errors.Is(err, ErrConflict) || len(tools.contexts) != 1 {
		t.Fatal("completed approval was replayed")
	}
}

func TestPrivateToolWritesSupportTwoConcurrentRuns(t *testing.T) {
	s, _, _, _ := runtimeFixture(t)
	defer s.Close()
	ctx := context.Background()
	var runs []Run
	for i := 0; i < 2; i++ {
		session, err := s.CreateSession(ctx, Session{UserID: "admin"})
		if err != nil {
			t.Fatal(err)
		}
		run, err := s.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID})
		if err != nil {
			t.Fatal(err)
		}
		runs = append(runs, run)
	}
	start := make(chan struct{})
	failures := make(chan error, 2)
	var workers sync.WaitGroup
	for i, run := range runs {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for n := 0; n < 10; n++ {
				call, err := s.SaveToolCall(ctx, ToolCall{ID: fmt.Sprintf("concurrent-%d-%d", i, n), RunID: run.ID, SessionID: run.SessionID, Name: "host_read", Arguments: json.RawMessage(`{}`), Status: ToolRunning})
				if err != nil {
					failures <- err
					return
				}
				call.Status = ToolCompleted
				if _, err := s.SaveToolCall(ctx, call); err != nil {
					failures <- err
					return
				}
			}
		}()
	}
	close(start)
	workers.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	for _, run := range runs {
		calls, err := s.ToolCalls(ctx, run.ID)
		if err != nil || len(calls) != 10 {
			t.Fatalf("concurrent records missing: %d %v", len(calls), err)
		}
	}
}
