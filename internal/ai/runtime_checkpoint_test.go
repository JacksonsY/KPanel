package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type checkpointClient struct {
	stream func(context.Context, CompletionRequest, func(CompletionEvent) error) error
}

func (c checkpointClient) Stream(ctx context.Context, _ Provider, _ string, req CompletionRequest, emit func(CompletionEvent) error) error {
	return c.stream(ctx, req, emit)
}
func (checkpointClient) Models(context.Context, Provider, string) ([]Model, error) { return nil, nil }

// Observe real SQLite mutations, including the upsert UPDATE path, without a
// production hook or extra WAL writes. One connection owns the TEMP triggers.
func observeCheckpoints(t *testing.T, s *Store) func() (int, int64) {
	t.Helper()
	s.db.SetMaxOpenConns(1)
	for _, sql := range []string{
		`CREATE TEMP TABLE checkpoint_writes(bytes INTEGER NOT NULL)`,
		`CREATE TEMP TRIGGER checkpoint_insert AFTER INSERT ON main.messages WHEN new.role='assistant' BEGIN INSERT INTO checkpoint_writes VALUES(length(CAST(new.content AS BLOB))); END`,
		`CREATE TEMP TRIGGER checkpoint_update AFTER UPDATE OF content ON main.messages WHEN new.role='assistant' BEGIN INSERT INTO checkpoint_writes VALUES(length(CAST(new.content AS BLOB))); END`,
	} {
		if _, err := s.db.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	return func() (int, int64) {
		var count int
		var bytes int64
		if err := s.db.QueryRow(`SELECT count(*),coalesce(sum(bytes),0) FROM checkpoint_writes`).Scan(&count, &bytes); err != nil {
			t.Fatal(err)
		}
		return count, bytes
	}
}

func checkpointRun(t *testing.T, s *Store, p Provider, m Model) (Session, Run) {
	t.Helper()
	ctx := context.Background()
	session, err := s.CreateSession(ctx, Session{UserID: "admin", ProviderID: p.ID, ProviderName: p.Name, ModelID: m.ID, ModelName: m.DisplayName})
	if err != nil {
		t.Fatal(err)
	}
	// Context history orders millisecond timestamps by ID on ties. Keep this
	// fixture's input explicitly before a fast first checkpoint.
	if _, err := s.AddMessage(ctx, Message{SessionID: session.ID, Role: RoleUser, Content: "inspect", CreatedAt: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	run, err := s.CreateRun(ctx, Run{SessionID: session.ID, UserID: "admin", ProviderID: p.ID, ProviderName: p.Name, ModelID: m.ID, ModelName: m.DisplayName})
	if err != nil {
		t.Fatal(err)
	}
	return session, run
}

// Finite integration samples use the same fake provider, runtime, public event
// boundary and SQLite mutations on the baseline and candidate. Payload is SQL
// content bytes, not a claim about physical IO or denial of service.
func TestRuntimeCheckpointMeasurement(t *testing.T) {
	for _, sample := range []struct {
		name        string
		size, delta int
		delay       time.Duration
		stop        string
	}{
		{"burst_1KiB", MaxAssistantBytes, 1024, 0, ""},
		{"burst_1byte", 64 << 10, 1, 0, ""},
		{"burst_large", MaxAssistantBytes, MaxAssistantBytes, 0, ""},
		{"slow", 3 << 10, 1024, 2100 * time.Millisecond, ""},
		{"cancel", 128 << 10, 1024, 0, "cancel"},
		{"provider_error", 128 << 10, 1024, 0, "error"},
		{"slow_exhausted", 9 << 10, 1024, 2050 * time.Millisecond, ""},
		{"slow_exhausted_cancel", 9 << 10, 1024, 2050 * time.Millisecond, "cancel"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			s, providers, p, m := runtimeFixture(t)
			defer s.Close()
			session, run := checkpointRun(t, s, p, m)
			measure := observeCheckpoints(t, s)
			hub := NewEventHub()
			events, unsubscribe := hub.Subscribe(run.ID)
			defer unsubscribe()
			var live strings.Builder
			completed := false
			drain := func() {
				for {
					select {
					case e := <-events:
						if e.Type == "message.delta" {
							live.WriteString(e.Data.(map[string]string)["delta"])
						}
						completed = completed || e.Type == "run.completed"
					default:
						return
					}
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := checkpointClient{stream: func(ctx context.Context, _ CompletionRequest, emit func(CompletionEvent) error) error {
				for sent := 0; sent < sample.size; sent += sample.delta {
					if sent > 0 && sample.delay > 0 {
						time.Sleep(sample.delay)
					}
					if err := emit(CompletionEvent{Delta: strings.Repeat("x", sample.delta)}); err != nil {
						return err
					}
					drain()
				}
				if strings.HasPrefix(sample.name, "slow_exhausted") {
					_, stop, err := hub.SnapshotSubscribe(run.ID, func(live *Message) error {
						if live == nil || len(live.Content) != sample.size {
							t.Errorf("exhausted checkpoint lost live prefix: %+v", live)
						}
						return nil
					})
					if err != nil {
						return err
					}
					stop()
				}
				if sample.stop == "cancel" {
					cancel()
					return ctx.Err()
				}
				if sample.stop == "error" {
					return errors.New("injected provider error")
				}
				return emit(CompletionEvent{Done: true})
			}}
			r, err := NewNativeRuntime(s, providers, client, &fakeTools{readOnly: true}, hub)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			err = r.Run(ctx, run.ID)
			elapsed := time.Since(start)
			runtime.ReadMemStats(&after)
			drain()
			count, bytes := measure()
			loaded, loadErr := s.RunByID(context.Background(), run.ID)
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			messages, loadErr := s.Messages(context.Background(), session.ID, 50)
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			stored := 0
			for _, message := range messages {
				if message.Role == RoleAssistant {
					stored += len(message.Content)
				}
			}
			var dbPath string
			var seq int
			var dbName string
			if scanErr := s.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &dbName, &dbPath); scanErr != nil {
				t.Fatal(scanErr)
			}
			walBytes := int64(0)
			if info, statErr := os.Stat(filepath.Clean(dbPath) + "-wal"); statErr == nil {
				walBytes = info.Size()
			}
			t.Logf("sample=%s output_bytes=%d delta_bytes=%d mutations=%d sql_content_bytes=%d wal_file_bytes=%d elapsed_ms=%d total_alloc_bytes=%d stored_bytes=%d live_bytes=%d status=%s run_error=%v", sample.name, sample.size, sample.delta, count, bytes, walBytes, elapsed.Milliseconds(), after.TotalAlloc-before.TotalAlloc, stored, live.Len(), loaded.Status, err)
			if live.Len() != sample.size {
				t.Fatalf("live stream lost output: %d", live.Len())
			}
			if sample.stop == "" && (err != nil || loaded.Status != RunCompleted || stored != sample.size || !completed) {
				t.Fatalf("final history/event mismatch: %s %d %v", loaded.Status, stored, err)
			}
			if sample.stop != "" && (err == nil || completed) {
				t.Fatal("stopped stream reported success")
			}
			if count > maxDraftCheckpoints+1 || bytes > maxDraftPayloadBytes+MaxAssistantBytes {
				t.Fatalf("checkpoint budget exceeded: %d mutations / %d bytes", count, bytes)
			}
			if stored != sample.size || (sample.stop == "cancel" && loaded.Status != RunCancelled) {
				t.Fatalf("stopped stream lost accepted output or cancellation: %s / %d", loaded.Status, stored)
			}
			if strings.HasPrefix(sample.name, "slow_exhausted") && count != maxDraftCheckpoints+1 {
				t.Fatalf("slow stream did not reach draft cap and final flush: %d", count)
			}
		})
	}
}

func TestDraftCheckpointAdmissionBounds(t *testing.T) {
	start := time.Unix(100, 0)
	for _, size := range []int{1, 1024, 256 << 10, MaxAssistantBytes} {
		d := draftCheckpoints{last: start}
		count, bytes := 0, 0
		for i := 1; i <= 1000; i++ {
			if d.admit(size, start.Add(time.Duration(i)*draftCheckpointInterval)) {
				count++
				bytes += size
			}
		}
		if count > 8 || bytes > 2<<20 || count == 0 {
			t.Fatalf("unbounded admission for size %d: %d / %d", size, count, bytes)
		}
	}
	d := draftCheckpoints{last: start}
	if d.admit(1, start) || !d.admit(1024, start) || d.admit(MaxAssistantBytes, start.Add(time.Second)) || !d.admit(MaxAssistantBytes, start.Add(2*time.Second)) {
		t.Fatal("first draft or subsequent time throttle changed")
	}
}

func TestRuntimeCheckpointPersistenceErrors(t *testing.T) {
	for _, sample := range []struct {
		name, trigger, code string
		stored              int
		cancel              bool
	}{
		{"draft", `BEFORE INSERT ON main.messages WHEN new.role='assistant'`, "message_store_failed", 0, false},
		{"status_unavailable", `BEFORE INSERT ON main.messages WHEN new.role='assistant'`, "message_store_failed", 0, false},
		{"final", `BEFORE UPDATE OF content ON main.messages WHEN new.role='assistant'`, "message_store_failed", 1024, false},
		{"cancel_final", `BEFORE UPDATE OF content ON main.messages WHEN new.role='assistant'`, "message_store_failed", 1024, true},
		{"session_transaction", `BEFORE UPDATE OF last_message_at ON main.sessions`, "message_store_failed", 0, false},
		{"run_completion", `BEFORE UPDATE OF status ON main.runs WHEN new.status='completed'`, "run_store_failed", 1028, false},
	} {
		t.Run(sample.name, func(t *testing.T) {
			s, providers, p, m := runtimeFixture(t)
			defer s.Close()
			session, run := checkpointRun(t, s, p, m)
			observeCheckpoints(t, s)
			if _, err := s.db.Exec(`CREATE TEMP TRIGGER reject_checkpoint ` + sample.trigger + ` BEGIN SELECT RAISE(ABORT,'injected checkpoint persistence error'); END`); err != nil {
				t.Fatal(err)
			}
			if sample.name == "status_unavailable" {
				if _, err := s.db.Exec(`CREATE TEMP TRIGGER reject_failure BEFORE UPDATE ON main.runs WHEN new.status='failed' BEGIN SELECT RAISE(ABORT,'injected run persistence error'); END`); err != nil {
					t.Fatal(err)
				}
			}
			hub := NewEventHub()
			events, unsubscribe := hub.Subscribe(run.ID)
			defer unsubscribe()
			requests, emits := 0, 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := checkpointClient{stream: func(_ context.Context, _ CompletionRequest, emit func(CompletionEvent) error) error {
				requests++
				for _, delta := range []string{strings.Repeat("x", 1024), "tail"} {
					emits++
					if err := emit(CompletionEvent{Delta: delta}); err != nil {
						return err
					}
				}
				if sample.cancel {
					cancel()
					return ctx.Err()
				}
				return emit(CompletionEvent{Done: true})
			}}
			r, _ := NewNativeRuntime(s, providers, client, &fakeTools{readOnly: true}, hub)
			defer r.Close()
			runErr := r.Run(ctx, run.ID)
			if runErr == nil || !strings.Contains(runErr.Error(), "injected checkpoint persistence error") {
				t.Fatalf("persistence failure disappeared: %v", runErr)
			}
			loaded, err := s.RunByID(context.Background(), run.ID)
			wantStatus, wantCode := RunFailed, sample.code
			if sample.name == "status_unavailable" {
				wantStatus, wantCode = RunRunning, ""
				if !strings.Contains(runErr.Error(), "injected run persistence error") {
					t.Fatal("failure status SQL error was ignored")
				}
			}
			if err != nil || loaded.Status != wantStatus || loaded.ErrorCode != wantCode || requests != 1 || (sample.name == "draft" && emits != 1) {
				t.Fatalf("failure was retried or reported success: %#v requests=%d emits=%d err=%v", loaded, requests, emits, err)
			}
			messages, err := s.Messages(context.Background(), session.ID, 50)
			if err != nil {
				t.Fatal(err)
			}
			stored := 0
			for _, message := range messages {
				if message.Role == RoleAssistant {
					stored += len(message.Content)
				}
			}
			if stored != sample.stored {
				t.Fatalf("failed transaction changed history: %d", stored)
			}
			for len(events) > 0 {
				event := <-events
				if event.Type == "run.completed" || (sample.code == "message_store_failed" && event.Type == "message.completed") {
					t.Fatalf("false completion on SQL failure: %s", event.Type)
				}
			}
		})
	}
}

func TestRuntimeCheckpointApprovalResume(t *testing.T) {
	s, providers, p, m := runtimeFixture(t)
	defer s.Close()
	session, run := checkpointRun(t, s, p, m)
	measure := observeCheckpoints(t, s)
	requests := 0
	client := checkpointClient{stream: func(_ context.Context, _ CompletionRequest, emit func(CompletionEvent) error) error {
		requests++
		if requests == 1 {
			for _, delta := range []string{strings.Repeat("x", 1024), "tail"} {
				if err := emit(CompletionEvent{Delta: delta}); err != nil {
					return err
				}
			}
			return emit(CompletionEvent{Done: true, ToolCalls: []ToolCall{{ID: "approval-call", Name: "host_action", Arguments: json.RawMessage(`{}`)}}})
		}
		return emit(CompletionEvent{Delta: "finished", Done: true})
	}}
	approve := true
	tools := &fakeTools{readOnly: true, approval: &approve}
	r, _ := NewNativeRuntime(s, providers, client, tools, NewEventHub())
	defer r.Close()
	if err := r.Run(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.RunByID(context.Background(), run.ID)
	if err != nil || loaded.Status != RunPendingApproval || tools.executed != 0 {
		t.Fatalf("approval bypassed: %#v %v", loaded, err)
	}
	if err := r.Resume(context.Background(), run.ID, Decision{ToolCallID: "approval-call", Approve: true}); err != nil {
		t.Fatal(err)
	}
	loaded, err = s.RunByID(context.Background(), run.ID)
	if err != nil || loaded.Status != RunCompleted || loaded.Step != 2 || requests != 2 || tools.executed != 1 {
		t.Fatalf("resume=%#v requests=%d tools=%d %v", loaded, requests, tools.executed, err)
	}
	messages, err := s.Messages(context.Background(), session.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	assistant := []string{}
	for _, message := range messages {
		if message.Role == RoleAssistant {
			assistant = append(assistant, message.Content)
		}
	}
	if len(assistant) != 2 || assistant[0] != strings.Repeat("x", 1024)+"tail" || assistant[1] != "finished" {
		t.Fatalf("resume history changed: %v", assistant)
	}
	if count, _ := measure(); count != 3 {
		t.Fatalf("approval/resume created redundant drafts: %d", count)
	}
}

func TestRuntimeCheckpointRestartAndRetry(t *testing.T) {
	s, providers, p, m := runtimeFixture(t)
	defer s.Close()
	session, run := checkpointRun(t, s, p, m)
	backup := filepath.Join(t.TempDir(), "ai.db")
	if err := os.WriteFile(filepath.Join(filepath.Dir(backup), "ai-secrets.key"), s.secrets.key, 0600); err != nil {
		t.Fatal(err)
	}
	client := checkpointClient{stream: func(_ context.Context, _ CompletionRequest, emit func(CompletionEvent) error) error {
		for _, delta := range []string{strings.Repeat("x", 1024), "uncheckpointed-tail"} {
			if err := emit(CompletionEvent{Delta: delta}); err != nil {
				return err
			}
		}
		// Capture committed state while the real runtime is inside the stream.
		// Reopening this consistent snapshot models an abrupt Panel restart.
		if _, err := s.db.Exec(`VACUUM INTO ?`, backup); err != nil {
			return err
		}
		return emit(CompletionEvent{Done: true})
	}}
	r, _ := NewNativeRuntime(s, providers, client, &fakeTools{readOnly: true}, NewEventHub())
	defer r.Close()
	if err := r.Run(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	restored, err := OpenStore(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	old, err := restored.RunByID(context.Background(), run.ID)
	if err != nil || old.Status != RunInterrupted {
		t.Fatalf("restart=%#v %v", old, err)
	}
	messages, err := restored.Messages(context.Background(), session.ID, 50)
	if err != nil || len(messages) != 2 || messages[1].Content != strings.Repeat("x", 1024) {
		t.Fatalf("restart lost committed draft or invented tail: %v %v", messages, err)
	}
	retry, err := restored.CreateRun(context.Background(), Run{SessionID: session.ID, UserID: "admin", ProviderID: p.ID, ModelID: m.ID, RetryOf: &run.ID})
	if err != nil {
		t.Fatal(err)
	}
	retryClient := checkpointClient{stream: func(_ context.Context, req CompletionRequest, emit func(CompletionEvent) error) error {
		if len(req.Messages) != 2 || req.Messages[0].Content != "inspect" || req.Messages[1].Content != strings.Repeat("x", 1024) {
			return fmt.Errorf("retry context changed: %v", req.Messages)
		}
		return emit(CompletionEvent{Delta: "retry-complete", Done: true})
	}}
	retryRuntime, _ := NewNativeRuntime(restored, providers, retryClient, &fakeTools{readOnly: true}, NewEventHub())
	defer retryRuntime.Close()
	if err := retryRuntime.Run(context.Background(), retry.ID); err != nil {
		t.Fatal(err)
	}
	messages, err = restored.Messages(context.Background(), session.ID, 50)
	if err != nil || len(messages) != 3 || messages[1].ID == messages[2].ID || messages[2].Content != "retry-complete" {
		t.Fatalf("retry overwrote committed draft: %v %v", messages, err)
	}
}

func TestRuntimeCheckpointTwoWorkersAndSteps(t *testing.T) {
	s, providers, p, m := runtimeFixture(t)
	defer s.Close()
	var sessions [2]Session
	var runs [2]Run
	for i := range runs {
		sessions[i], runs[i] = checkpointRun(t, s, p, m)
		if _, err := s.db.Exec(`UPDATE messages SET content=? WHERE session_id=? AND role='user'`, fmt.Sprint("inspect-", i), sessions[i].ID); err != nil {
			t.Fatal(err)
		}
	}
	measure := observeCheckpoints(t, s)
	ready, release := make(chan struct{}, 2), make(chan struct{})
	var mu sync.Mutex
	steps := map[string]int{}
	client := checkpointClient{stream: func(ctx context.Context, req CompletionRequest, emit func(CompletionEvent) error) error {
		if strings.Contains(req.System, "你负责 KPanel 后台学习") {
			return emit(CompletionEvent{Delta: `{"decision":"skip"}`, Done: true})
		}
		key := req.Messages[0].Content
		mu.Lock()
		steps[key]++
		step := steps[key]
		mu.Unlock()
		if step == 1 {
			ready <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		for _, delta := range []string{strings.Repeat("x", 1024), "tail"} {
			if err := emit(CompletionEvent{Delta: delta}); err != nil {
				return err
			}
		}
		event := CompletionEvent{Done: true}
		if step < MaxModelSteps {
			event.ToolCalls = []ToolCall{{ID: fmt.Sprintf("%s-call-%d", key, step), Name: "host_action", Arguments: json.RawMessage(`{}`)}}
		}
		return emit(event)
	}}
	r, _ := NewNativeRuntime(s, providers, client, &fakeTools{readOnly: true}, NewEventHub())
	defer r.Close()
	errorsOut := make(chan error, 2)
	for _, run := range runs {
		go func(run Run) { errorsOut <- r.Run(context.Background(), run.ID) }(run)
	}
	for range runs {
		select {
		case <-ready:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("both runtime workers did not enter the provider")
		}
	}
	close(release)
	for range runs {
		if err := <-errorsOut; err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	for i, run := range runs {
		loaded, err := s.RunByID(context.Background(), run.ID)
		if err != nil || loaded.Status != RunCompleted || loaded.Step != MaxModelSteps {
			t.Fatalf("worker %d = %#v %v", i, loaded, err)
		}
		messages, err := s.Messages(context.Background(), sessions[i].ID, 100)
		if err != nil {
			t.Fatal(err)
		}
		assistant := 0
		for _, message := range messages {
			if message.Role == RoleAssistant {
				assistant++
				if message.Content != strings.Repeat("x", 1024)+"tail" {
					t.Fatal("step final history lost its tail")
				}
			}
		}
		if assistant != MaxModelSteps {
			t.Fatalf("worker %d assistant messages=%d", i, assistant)
		}
	}
	count, bytes := measure()
	if count > 2*MaxModelSteps*(maxDraftCheckpoints+1) || bytes > 2*MaxModelSteps*(maxDraftPayloadBytes+MaxAssistantBytes) {
		t.Fatalf("run budgets exceeded: %d / %d", count, bytes)
	}
	t.Logf("workers=2 model_steps_each=%d mutations=%d sql_content_bytes=%d", MaxModelSteps, count, bytes)
}
