package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type finalProjectionClient struct {
	calls  int
	leaked bool
}

func (c *finalProjectionClient) Models(context.Context, Provider, string) ([]Model, error) {
	return nil, nil
}
func (c *finalProjectionClient) Stream(_ context.Context, _ Provider, _ string, r CompletionRequest, emit func(CompletionEvent) error) error {
	for _, m := range r.Messages {
		if strings.Contains(m.Content, "CLOSURE_PRIVATE_VALUE") {
			c.leaked = true
		}
	}
	c.calls++
	if c.calls == 1 {
		return emit(CompletionEvent{Done: true, ToolCalls: []ToolCall{{ID: "closure-call", Name: "host_action", Arguments: json.RawMessage(`{"resourceVersion":"sha256:test"}`)}}})
	}
	return emit(CompletionEvent{Delta: "ordinary-final", Done: true})
}

type finalProjectionTools struct {
	fakeTools
	body string
}

func (t *finalProjectionTools) Execute(context.Context, ToolExecutionContext, string, json.RawMessage) (string, error) {
	return t.body, nil
}
func TestFinalProjectionConsumerBoundaries(t *testing.T) {
	cases := map[string]string{
		"shell-inline-bracket":         "API_TOKEN=prefix[\"CLOSURE_PRIVATE_VALUE\"]suffix",
		"shell-logical-identifier-eof": "API_TO\\\nKEN=CLOSURE_PRIVATE_VALUE\\",
		"shell-ansi-escaped-quote": `API_TOKEN=$'first\'
CLOSURE_PRIVATE_VALUE
last'`,
		"shell-command-substitution": "API_TOKEN=\"$(printf \"%s\" \"first\nCLOSURE_PRIVATE_VALUE\nlast\")\"",
		"shell-logical-identifier":   "API_TO\\\nKEN=CLOSURE_PRIVATE_VALUE",
		"mysql-repeated":             "mysql -pdecoy -pCLOSURE_PRIVATE_VALUE",
		"object-key":                 `{"password=CLOSURE_PRIVATE_VALUE":"safe"}`,
		"log-prefixed-json":          `INFO {"passw\u006frd":"CLOSURE_PRIVATE_VALUE"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			store, providers, provider, model := runtimeFixture(t)
			defer store.Close()
			ctx := context.Background()
			session, err := store.CreateSession(ctx, Session{UserID: "admin"})
			if err != nil {
				t.Fatal(err)
			}
			run, err := store.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID, ProviderID: provider.ID, ModelID: model.ID})
			if err != nil {
				t.Fatal(err)
			}
			client := &finalProjectionClient{}
			hub := NewEventHub()
			events, unsubscribe := hub.Subscribe(run.ID)
			defer unsubscribe()
			runtime, err := NewNativeRuntime(store, providers, client, &finalProjectionTools{fakeTools{readOnly: true}, body}, hub)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			if err := runtime.Run(ctx, run.ID); err != nil {
				t.Fatal(err)
			}
			if client.leaked {
				t.Error("private value reached next provider request")
			}
			for len(events) > 0 {
				raw, _ := json.Marshal(<-events)
				if strings.Contains(string(raw), "CLOSURE_PRIVATE_VALUE") {
					t.Error("private value reached public events")
				}
			}
			messages, err := store.Messages(ctx, session.ID, 50)
			if err != nil {
				t.Fatal(err)
			}
			tool := false
			for _, m := range messages {
				if m.Role == RoleTool {
					tool = true
				}
				if strings.Contains(m.Content, "CLOSURE_PRIVATE_VALUE") {
					t.Error("private value retained in safe history")
				}
			}
			loaded, loadErr := store.RunByID(ctx, run.ID)
			if loadErr != nil || loaded.Status != RunCompleted {
				t.Fatalf("final state=%s error=%v", loaded.Status, loadErr)
			}
			final := false
			for _, m := range messages {
				final = final || (m.Role == RoleAssistant && m.Content == "ordinary-final")
			}
			if !final {
				t.Fatal("ordinary final answer missing")
			}
			if !tool || client.calls != 2 {
				t.Fatal("expected actual two-step runtime with a tool result")
			}
		})
	}
}
