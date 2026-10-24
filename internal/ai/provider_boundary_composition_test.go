package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type boundaryCompositionClient struct {
	models []Model
	err    error
	before func()
}

func (c *boundaryCompositionClient) Models(context.Context, Provider, string) ([]Model, error) {
	if c.before != nil {
		c.before()
	}
	return c.models, c.err
}
func (c *boundaryCompositionClient) Stream(context.Context, Provider, string, CompletionRequest, func(CompletionEvent) error) error {
	return c.err
}

func TestProviderMetadataAndTestErrorsShareEncodedCredentialPolicy(t *testing.T) {
	store, providers, provider, _ := runtimeFixture(t)
	defer store.Close()
	ctx := context.Background()
	const encoded = `{"note":"test\u002dkey"}`
	service := &Service{Store: store, Providers: providers, client: &boundaryCompositionClient{models: []Model{{ModelID: encoded, DisplayName: "ordinary"}}}}
	if _, err := service.SyncModels(ctx, provider.ID); err == nil {
		t.Fatal("encoded credential metadata admitted")
	}
	models, err := store.ListModels(ctx, provider.ID)
	if err != nil || len(models) != 1 || models[0].ModelID != "mock-model" {
		t.Fatalf("rejected catalog changed: %v %v", models, err)
	}
}

func TestProviderTestAndSyncErrorsShareEncodedCredentialPolicy(t *testing.T) {
	store, providers, provider, _ := runtimeFixture(t)
	defer store.Close()
	ctx := context.Background()
	const encoded = `{"note":"test\u002dkey"}`
	cause := errors.Join(context.DeadlineExceeded, &ProviderError{Status: 429, Message: encoded, Retryable: true})
	service := &Service{Store: store, Providers: providers, client: &boundaryCompositionClient{err: cause}}
	for _, test := range []func() error{func() error { return service.TestProvider(ctx, provider.ID) }, func() error { _, err := service.SyncModels(ctx, provider.ID); return err }} {
		err := test()
		var status *ProviderError
		if err == nil || strings.Contains(err.Error(), `test\u002dkey`) || !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &status) || !status.Retryable {
			t.Fatalf("error boundary: %v", err)
		}
	}
	body, _ := json.Marshal(map[string]any{"error": map[string]string{"message": encoded}})
	if err := providerHTTPError(429, body, "test-key"); strings.Contains(err.Error(), `test\u002dkey`) {
		t.Fatalf("HTTP error retained credential: %v", err)
	}
}

func TestModelDiscoveryMergesCurrentConfigurationInsideWriterTransaction(t *testing.T) {
	for _, reasoning := range []bool{false, true} {
		t.Run(fmt.Sprint(reasoning), func(t *testing.T) {
			store, providers, provider, model := runtimeFixture(t)
			defer store.Close()
			ctx := context.Background()
			model.Reasoning = !reasoning
			if err := store.SaveModels(ctx, provider.ID, []Model{model}); err != nil {
				t.Fatal(err)
			}
			service := &Service{Store: store, Providers: providers, client: &boundaryCompositionClient{models: []Model{{ModelID: model.ModelID}}, before: func() {
				model.Reasoning = reasoning
				model.DisplayName = "edited while discovery waits"
				model.ContextWindow = 16000
				model.Enabled = false
				model.IsDefault = true
				if err := store.SaveModels(ctx, provider.ID, []Model{model}); err != nil {
					t.Fatal(err)
				}
			}}}
			items, err := service.SyncModels(ctx, provider.ID)
			if err != nil || len(items) != 1 {
				t.Fatalf("sync: %v %v", items, err)
			}
			got := items[0]
			if got.Reasoning != reasoning || got.ID != model.ID || got.DisplayName != model.DisplayName || got.ContextWindow != model.ContextWindow || got.Enabled || !got.IsDefault {
				t.Fatalf("administrator configuration overwritten: %+v", got)
			}
		})
	}
}

func TestProviderComplexityRejectsBeforeSSEHandlerOrNativeExpansion(t *testing.T) {
	dense := `{"type":"reasoning","extra":[` + strings.Repeat(`{"a":0},`, 10000) + `{}]}`
	called := false
	err := scanSSE(strings.NewReader("data: "+dense+"\n\n"), func(string, string) error { called = true; return nil })
	if err == nil || called {
		t.Fatal("dense event reached typed decoding")
	}
	raw := json.RawMessage(`{"type":"openai_responses_output","providerId":"p","model":"m","complete":true,"items":[` + dense + `]}`)
	request := CompletionRequest{Model: "m", Messages: []ChatMessage{{Role: "assistant", ToolCalls: []ToolCall{{Name: "read", Arguments: json.RawMessage(`{}`), ProviderData: raw}}}}}
	if err := NewHTTPModelClient().Stream(context.Background(), Provider{ID: "p", Protocol: ProtocolOpenAICompatible, APIMode: OpenAIResponses, BaseURL: "http://127.0.0.1:1"}, "key", request, func(CompletionEvent) error { return nil }); !errors.Is(err, errProviderJSONBudget) {
		t.Fatalf("legacy native tree reached network/expansion: %v", err)
	}
	if _, err := marshalProviderJSON(raw, maxProviderNativeBytes); !errors.Is(err, errProviderJSONBudget) {
		t.Fatalf("dense new native tree admitted: %v", err)
	}
}
