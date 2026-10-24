package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
)

// These finite regressions also run unchanged against the pre-fix snapshot.
// They use only a few MiB, rather than attempting a process OOM.
func TestContextEnvelopeCompactsHiddenStateByWholeRun(t *testing.T) {
	s, session, run := contextFixture(t)
	ctx := context.Background()
	native := json.RawMessage(`{"text":"` + strings.Repeat("x", 900<<10) + `"}`)
	for i := 0; i < 6; i++ {
		if i > 0 {
			previous := run
			previous.Status = RunCompleted
			if err := s.UpdateRun(ctx, previous); err != nil {
				t.Fatal(err)
			}
			var err error
			run, err = s.CreateRun(ctx, Run{SessionID: session.ID, UserID: "admin", ProviderID: "provider", ModelID: "model"})
			if err != nil {
				t.Fatal(err)
			}
		}
		call, err := s.SaveToolCall(ctx, ToolCall{ID: fmt.Sprintf("call_%d", i), RunID: run.ID, SessionID: session.ID, Name: "host_read", Arguments: json.RawMessage(`{}`), ProviderData: native, Status: ToolCompleted})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddMessage(ctx, Message{SessionID: session.ID, RunID: run.ID, Role: RoleTool, ToolCallID: call.ID, Content: "ok"}); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := s.ContextForRun(ctx, run, 10_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Messages) >= 6 || snapshot.Summary == "" {
		t.Fatalf("hidden state was not compacted: messages=%d summary=%q", len(snapshot.Messages), snapshot.Summary)
	}
	if snapshot.Messages[len(snapshot.Messages)-1].RunID != run.ID {
		t.Fatal("current run was lost")
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM tool_calls WHERE session_id=?`, session.ID).Scan(&count); err != nil || count != 6 {
		t.Fatalf("original state changed: count=%d err=%v", count, err)
	}
}

func TestContextEnvelopeRejectsCumulativeCurrentRunBeforeLoading(t *testing.T) {
	s, session, run := contextFixture(t)
	ctx := context.Background()
	native := json.RawMessage(`{"text":"` + strings.Repeat("x", 900<<10) + `"}`)
	for i := 0; i < 6; i++ {
		call, err := s.SaveToolCall(ctx, ToolCall{ID: fmt.Sprintf("call_%d", i), RunID: run.ID, SessionID: session.ID, Name: "host_read", Arguments: json.RawMessage(`{}`), ProviderData: native, Status: ToolCompleted})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddMessage(ctx, Message{SessionID: session.ID, RunID: run.ID, Role: RoleTool, ToolCallID: call.ID, Content: "ok"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ContextForRun(ctx, run, 10_000_000); err == nil {
		t.Error("current run exceeded the private context budget without rejection")
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := s.ToolCalls(ctx, run.ID)
	runtime.ReadMemStats(&after)
	t.Logf("six 900 KiB private records rejected with totalAlloc=%d", after.TotalAlloc-before.TotalAlloc)
	if err == nil {
		t.Fatal("cumulative tool-state read was allowed")
	}
	if after.TotalAlloc-before.TotalAlloc > 512<<10 {
		t.Fatal("private bodies were allocated before aggregate rejection")
	}
	var cursor string
	if err := s.db.QueryRowContext(ctx, `SELECT summary_cursor FROM sessions WHERE id=?`, session.ID).Scan(&cursor); err != nil || cursor != "" {
		t.Fatalf("failed preflight advanced cursor: %q %v", cursor, err)
	}
}

func TestContextEnvelopeLegacySingleRowFailsBeforeCopy(t *testing.T) {
	s, session, run := contextFixture(t)
	ctx := context.Background()
	call, err := s.SaveToolCall(ctx, ToolCall{ID: "legacy", RunID: run.ID, SessionID: session.ID, Name: "host_read", Arguments: json.RawMessage(`{}`), Status: ToolCompleted})
	if err != nil {
		t.Fatal(err)
	}
	call.ProviderData = json.RawMessage(`{"text":"` + strings.Repeat("x", 2<<20) + `"}`)
	encrypted, err := s.sealToolState(call) // preserve an authenticated legacy record
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE tool_calls SET private_state=? WHERE id=?`, encrypted, call.ID); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err = s.ToolCall(ctx, run.ID, call.ID)
	runtime.ReadMemStats(&after)
	t.Logf("legacy 2 MiB record rejected with totalAlloc=%d", after.TotalAlloc-before.TotalAlloc)
	if err == nil {
		t.Fatal("oversized legacy record was silently loaded")
	}
	if after.TotalAlloc-before.TotalAlloc > 512<<10 {
		t.Fatal("legacy body copied before rejection")
	}
}

func TestContextEnvelopeOutboundRejectsBeforeJSONAllocation(t *testing.T) {
	payload := map[string]any{"prompt": strings.Repeat("\x00", 3<<20)}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := NewHTTPModelClient().do(context.Background(), Provider{BaseURL: "://invalid"}, "synthetic", "POST", "/never", payload, nil)
	runtime.ReadMemStats(&after)
	t.Logf("18 MiB encoded request rejected with totalAlloc=%d", after.TotalAlloc-before.TotalAlloc)
	if err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatalf("large escaped request did not fail at budget: %v", err)
	}
	if after.TotalAlloc-before.TotalAlloc > 512<<10 {
		t.Fatal("large JSON allocated before wire-budget rejection")
	}
}

func TestContextEnvelopeCountCompactionKeepsWholeToolRun(t *testing.T) {
	s, session, oldRun := contextFixture(t)
	ctx := context.Background()
	oldTime := time.Now().UTC().Add(-time.Minute)
	for i := 0; i < 3; i++ {
		call, err := s.SaveToolCall(ctx, ToolCall{ID: fmt.Sprintf("old_%d", i), RunID: oldRun.ID, SessionID: session.ID, Name: "host_read", Arguments: json.RawMessage(`{}`), Status: ToolCompleted})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddMessage(ctx, Message{SessionID: session.ID, RunID: oldRun.ID, Role: RoleTool, ToolCallID: call.ID, Content: "old result", CreatedAt: oldTime}); err != nil {
			t.Fatal(err)
		}
	}
	oldRun.Status = RunCompleted
	if err := s.UpdateRun(ctx, oldRun); err != nil {
		t.Fatal(err)
	}
	current, err := s.CreateRun(ctx, Run{SessionID: session.ID, UserID: "admin", ProviderID: "provider", ModelID: "model"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 198; i++ {
		if _, err := s.AddMessage(ctx, Message{SessionID: session.ID, RunID: current.ID, Role: RoleUser, Content: "new input"}); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := s.ContextForRun(ctx, current, 131072)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Messages) != 198 {
		t.Fatalf("count boundary split old tool run: %d messages", len(snapshot.Messages))
	}
	for _, message := range snapshot.Messages {
		if message.RunID == oldRun.ID {
			t.Fatal("orphan old tool result retained")
		}
	}
}

func TestContextEnvelopeInterleavedCurrentRunCannotBeSummarized(t *testing.T) {
	s, session, oldRun := contextFixture(t)
	ctx := context.Background()
	created := time.Now().UTC().Truncate(time.Millisecond)
	oldRun.Status = RunCompleted
	if err := s.UpdateRun(ctx, oldRun); err != nil {
		t.Fatal(err)
	}
	current, err := s.CreateRun(ctx, Run{SessionID: session.ID, UserID: "admin", ProviderID: "provider", ModelID: "model"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "c", "d"} {
		call, err := s.SaveToolCall(ctx, ToolCall{ID: "call_" + id, RunID: oldRun.ID, SessionID: session.ID, Name: "host_read", Arguments: json.RawMessage(`{}`), Status: ToolCompleted})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddMessage(ctx, Message{ID: id, SessionID: session.ID, RunID: oldRun.ID, Role: RoleTool, ToolCallID: call.ID, Content: "old result", CreatedAt: created}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 198; i++ {
		id := fmt.Sprintf("z%03d", i)
		if i == 0 {
			id = "b"
		}
		if _, err := s.AddMessage(ctx, Message{ID: id, SessionID: session.ID, RunID: current.ID, Role: RoleUser, Content: "new input", CreatedAt: created}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ContextForRun(ctx, current, 131072); err == nil {
		t.Fatal("count compaction accepted an overlapping current-run span")
	}
	var cursor string
	if err := s.db.QueryRowContext(ctx, `SELECT summary_cursor FROM sessions WHERE id=?`, session.ID).Scan(&cursor); err != nil || cursor != "" {
		t.Fatalf("failed overlapping compaction advanced cursor: %q %v", cursor, err)
	}
}
