package terminalbatch

import (
	"context"
	"strings"
	"testing"
	"time"
)

func eventually(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition did not become true")
}

func TestAutomaticCleanupRecoversCapacityWhileBatchIsStillRunning(t *testing.T) {
	d := newFake()
	d.closeFailures = 1 << 20
	m, err := Open(t.TempDir(), d, Options{Timeout: 3 * time.Second, CleanupTimeout: 10 * time.Millisecond, RetryDelay: time.Millisecond, CleanupInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	job, err := m.Create("user", strings.Repeat("a", 32), "true", targets(8))
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return len(m.cleanup[job.ID]) == 4
	})
	current, _ := m.Get("user", job.ID)
	if current.State != "running" {
		t.Fatal("fixture should still be waiting for capacity")
	}
	d.mu.Lock()
	d.closeFailures = 0
	d.mu.Unlock()
	// No Stop, browser poll, or second submission is needed to recover.
	eventually(t, func() bool {
		current, _ = m.Get("user", job.ID)
		return current.State == "completed" && !current.CleanupPending && !current.CleanupRunning
	})
	for _, result := range current.Results {
		if result.State != "succeeded" || result.Error != "" || result.Output == "" {
			t.Fatal(result)
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sends != 8 || len(d.live) != 0 || d.max > 4 {
		t.Fatalf("sends=%d live=%d max=%d", d.sends, len(d.live), d.max)
	}
}

func TestCleanupInProgressDoesNotBlockNewAdmissionAndShutdownCancelsIt(t *testing.T) {
	d := &blockingCleanupDriver{fakeDriver: newFake(), entered: make(chan context.Context, 8), release: make(chan struct{})}
	m, err := Open(t.TempDir(), d, Options{CleanupTimeout: time.Minute, CleanupInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	id := strings.Repeat("b", 32)
	m.mu.Lock()
	m.jobs = []Job{{ID: id, UserID: "user", State: "completed", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Results: []Result{{Host: targets(1)[0], State: "succeeded", Output: "original"}}}}
	m.cleanup[id] = map[int]Session{0: {ID: "old"}}
	m.mu.Unlock()
	if err := m.Stop("user", id); err != nil {
		t.Fatal(err)
	}
	var retry context.Context
	select {
	case retry = <-d.entered:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not start")
	}
	// The new job can be admitted even while an older Close is blocked.
	d.mu.Lock()
	d.block = true
	d.mu.Unlock()
	if _, err := m.Create("user", strings.Repeat("c", 32), "true", targets(1)); err != nil {
		t.Fatal(err)
	}
	close(d.release)
	m.Close()
	if retry.Err() == nil {
		t.Fatal("shutdown left cleanup alive")
	}
	current, _ := m.Get("user", id)
	if current.State != "completed" || current.Results[0].State != "succeeded" || current.Results[0].Output != "original" {
		t.Fatal(current)
	}
}

func TestStopRetriesAllOwnedPendingJobsWithoutTouchingOtherUsers(t *testing.T) {
	d := newFake()
	m, err := Open(t.TempDir(), d, Options{CleanupInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	ids := []string{strings.Repeat("d", 32), strings.Repeat("e", 32), strings.Repeat("f", 32)}
	m.mu.Lock()
	for i, id := range ids {
		owner := "user"
		if i == 2 {
			owner = "other"
		}
		m.jobs = append(m.jobs, Job{ID: id, UserID: owner, State: "stopped", Results: []Result{{Host: targets(1)[0], State: "failed", Error: "terminal_stop_failed"}}})
		m.cleanup[id] = map[int]Session{0: {ID: id}}
		d.live[id] = true
	}
	m.mu.Unlock()
	if err := m.Stop("other", ids[0]); err != ErrNotFound {
		t.Fatal("cross-owner stop accepted")
	}
	if err := m.Stop("user", ids[0]); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return len(m.cleanup[ids[0]]) == 0 && len(m.cleanup[ids[1]]) == 0
	})
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.live) != 1 || !d.live[ids[2]] || d.sends != 0 {
		t.Fatal("stop missed owned sessions, replayed commands, or crossed ownership")
	}
}
