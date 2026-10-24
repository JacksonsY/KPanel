package panel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/ai"
)

type liveSnapshotClient struct {
	ready  chan struct{}
	finish chan struct{}
}

func (c liveSnapshotClient) Models(context.Context, ai.Provider, string) ([]ai.Model, error) {
	return nil, nil
}
func (c liveSnapshotClient) Stream(ctx context.Context, _ ai.Provider, _ string, _ ai.CompletionRequest, emit func(ai.CompletionEvent) error) error {
	for _, delta := range []string{strings.Repeat("x", 1024), "tail-test-key-visible"} {
		if err := emit(ai.CompletionEvent{Delta: delta}); err != nil {
			return err
		}
	}
	close(c.ready)
	select {
	case <-c.finish:
		return emit(ai.CompletionEvent{Done: true})
	case <-ctx.Done():
		return ctx.Err()
	}
}

type liveSnapshotTools struct{}

func (liveSnapshotTools) Definitions() []ai.ToolDefinition     { return nil }
func (liveSnapshotTools) DryRun(string, json.RawMessage) error { return nil }
func (liveSnapshotTools) RequiresApproval(context.Context, string, json.RawMessage) bool {
	return false
}
func (liveSnapshotTools) Execute(context.Context, ai.ToolExecutionContext, string, json.RawMessage) (string, error) {
	return "", nil
}

func TestAIStreamSnapshotIncludesPublicLiveDraft(t *testing.T) {
	f := newAIStreamFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := "test-key"
	p, err := f.server.ai.Providers.Save(ctx, "", ai.ProviderInput{Name: "mock", Protocol: ai.ProtocolOpenAICompatible, BaseURL: "https://example.com/v1", EndpointScope: ai.EndpointPublic, APIKey: &key, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.server.ai.Store.SaveModels(ctx, p.ID, []ai.Model{{ModelID: "mock", DisplayName: "Mock", Enabled: true, ContextWindow: 8000}}); err != nil {
		t.Fatal(err)
	}
	models, err := f.server.ai.Store.ListModels(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	session, err := f.server.ai.Store.CreateSession(ctx, ai.Session{UserID: f.session.User.ID})
	if err != nil {
		t.Fatal(err)
	}
	f.run, err = f.server.ai.Store.CreateRun(ctx, ai.Run{UserID: f.session.User.ID, SessionID: session.ID, ProviderID: p.ID, ProviderName: p.Name, ModelID: models[0].ID, ModelName: models[0].DisplayName})
	if err != nil {
		t.Fatal(err)
	}
	client := liveSnapshotClient{ready: make(chan struct{}), finish: make(chan struct{})}
	runtime, err := ai.NewNativeRuntime(f.server.ai.Store, f.server.ai.Providers, client, liveSnapshotTools{}, f.server.ai.Events)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	done := make(chan error, 1)
	go func() { done <- runtime.Run(ctx, f.run.ID) }()
	select {
	case <-client.ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	streamCtx, stop := context.WithCancel(ctx)
	defer stop()
	w := &aiSnapshotRecorder{ResponseRecorder: httptest.NewRecorder(), cancel: stop}
	request := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(streamCtx)
	request.AddCookie(f.cookie)
	f.server.aiRunEvents(w, request, f.session, f.run.ID)
	var snapshot struct {
		Messages []ai.Message `json:"messages"`
		ID       string       `json:"streamingMessageId"`
	}
	for _, line := range strings.Split(w.Body.String(), "\n") {
		if strings.HasPrefix(line, "data: ") {
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &snapshot); err != nil {
				t.Fatal(err)
			}
		}
	}
	if snapshot.ID == "" || len(snapshot.Messages) != 1 || snapshot.Messages[0].ID != snapshot.ID || len(snapshot.Messages[0].Content) <= 1024 || strings.Contains(w.Body.String(), key) || !strings.Contains(w.Body.String(), "[REDACTED]") {
		t.Fatalf("unsafe or stale SSE snapshot: id=%s messages=%d", snapshot.ID, len(snapshot.Messages))
	}
	close(client.finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	request.AddCookie(f.cookie)
	f.server.aiRunEvents(response, request, f.session, f.run.ID)
	if !strings.Contains(response.Body.String(), `"status":"completed"`) || !strings.Contains(response.Body.String(), snapshot.ID) || strings.Contains(response.Body.String(), `"streamingMessageId"`) {
		t.Fatal("completed reconnect did not use durable draft identity")
	}
}

type snapshotBoundaryRecorder struct {
	*httptest.ResponseRecorder
	first func()
	stop  context.CancelFunc
}

func (w *snapshotBoundaryRecorder) Flush() {
	w.ResponseRecorder.Flush()
	if w.first != nil {
		first := w.first
		w.first = nil
		first()
	} else {
		w.stop()
	}
}

func TestAIStreamSnapshotSubscribeBoundary(t *testing.T) {
	f := newAIStreamFixture(t)
	ctx, stop := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer stop()
	w := &snapshotBoundaryRecorder{ResponseRecorder: httptest.NewRecorder(), stop: stop}
	w.first = func() {
		f.run.Status = ai.RunCompleted
		if err := f.server.ai.Store.UpdateRun(ctx, f.run); err != nil {
			t.Fatal(err)
		}
		f.server.ai.Events.Publish(ai.RunEvent{RunID: f.run.ID, Type: "run.completed", Data: f.run})
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	r.AddCookie(f.cookie)
	f.server.aiRunEvents(w, r, f.session, f.run.ID)
	if !strings.Contains(w.Body.String(), "event: run.completed") {
		t.Fatalf("completion between snapshot and subscription disappeared: %s", w.Body.String())
	}
}

func TestAIStreamOverflowRecoversTerminalOnReconnect(t *testing.T) {
	f := newAIStreamFixture(t)
	ctx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	w := &snapshotBoundaryRecorder{ResponseRecorder: httptest.NewRecorder(), stop: func() {}}
	w.first = func() {
		f.run.Status = ai.RunCompleted
		if err := f.server.ai.Store.UpdateRun(ctx, f.run); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 33; i++ {
			f.server.ai.Events.Publish(ai.RunEvent{RunID: f.run.ID, Type: "message.delta", Data: map[string]string{"delta": "x"}})
		}
		f.server.ai.Events.Publish(ai.RunEvent{RunID: f.run.ID, Type: "run.completed", Data: f.run})
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	r.AddCookie(f.cookie)
	f.server.aiRunEvents(w, r, f.session, f.run.ID)
	if ctx.Err() != nil {
		t.Fatal("overflow stream did not close for reconnect")
	}
	response := httptest.NewRecorder()
	f.server.aiRunEvents(response, r, f.session, f.run.ID)
	if !strings.Contains(response.Body.String(), `"status":"completed"`) || strings.Contains(response.Body.String(), `"streamingMessageId"`) {
		t.Fatalf("reconnect lost terminal snapshot: %s", response.Body.String())
	}
}
