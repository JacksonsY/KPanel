package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoveryRejectsCredentialMetadataBeforeStorage(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"data":[{"id":"opaque-credential"}]}`) }))
	defer server.Close()
	if items, err := NewHTTPModelClient().Models(ctx, Provider{Protocol: ProtocolOpenAICompatible, BaseURL: server.URL, EndpointScope: EndpointPrivate}, "opaque-credential"); err == nil || len(items) != 0 || strings.Contains(err.Error(), "opaque-credential") {
		t.Fatalf("discovery credential admitted: models=%d err=%v", len(items), err)
	}
	store, providers, provider, _ := runtimeFixture(t)
	defer store.Close()
	before, _ := store.ListModels(ctx, provider.ID)
	service := &Service{Store: store, Providers: providers, client: &syncModelsClient{models: []Model{{ModelID: "safe", DisplayName: "test-key"}}}}
	if _, err := service.SyncModels(ctx, provider.ID); err == nil || strings.Contains(err.Error(), "test-key") {
		t.Fatal("custom client credential metadata admitted")
	}
	after, _ := store.ListModels(ctx, provider.ID)
	if len(after) != len(before) {
		t.Fatal("rejected metadata changed the catalog")
	}
}

type credentialResultTools struct{ fakeTools }

func (t *credentialResultTools) Execute(context.Context, ToolExecutionContext, string, json.RawMessage) (string, error) {
	return `{"note":"test\u002dkey","safe":"visible"}`, nil
}

func TestToolResultsUseTheSameProviderCredentialBoundary(t *testing.T) {
	store, providers, provider, model := runtimeFixture(t)
	defer store.Close()
	ctx := context.Background()
	session, _ := store.CreateSession(ctx, Session{UserID: "admin"})
	run, _ := store.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID, ProviderID: provider.ID, ModelID: model.ID})
	hub := NewEventHub()
	events, unsubscribe := hub.Subscribe(run.ID)
	defer unsubscribe()
	r, err := NewNativeRuntime(store, providers, &scriptedClient{tool: "host_action"}, &credentialResultTools{fakeTools{readOnly: true}}, hub)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Run(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	for len(events) > 0 {
		raw, _ := json.Marshal(<-events)
		if strings.Contains(string(raw), "test-key") || strings.Contains(string(raw), `test\\u002dkey`) {
			t.Fatal("tool event retained credential")
		}
	}
	messages, err := store.Messages(ctx, session.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, message := range messages {
		if message.Role == RoleTool {
			found = true
			if strings.Contains(message.Content, "test-key") || strings.Contains(message.Content, `test\u002dkey`) || !strings.Contains(message.Content, "visible") {
				t.Fatal("tool history retained credential or lost ordinary data")
			}
		}
	}
	if !found {
		t.Fatal("tool was not executed")
	}
}
