package terminalbatch

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func settledBatch(t *testing.T, m *Manager, id string) Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		active := m.active[id] != nil || m.cleaning[id] != nil
		m.mu.Unlock()
		job, err := m.Get("user", id)
		if err != nil {
			t.Fatal(err)
		}
		if !active && job.State != "running" {
			return job
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("batch or cleanup did not settle within the bounded deadline")
	return Job{}
}

func TestFailedCleanupHasIndependentDeadlineAndRetainsReservation(t *testing.T) {
	d := newFake()
	d.closeFailures = 1 << 20
	m, err := Open(t.TempDir(), d, Options{Timeout: time.Hour, CleanupTimeout: 40 * time.Millisecond, RetryDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	job, err := m.Create("user", strings.Repeat("a", 32), "true", targets(1))
	if err != nil {
		t.Fatal(err)
	}
	job = settledBatch(t, m, job.ID)
	if job.State != "completed" || job.Results[0].State != "succeeded" || job.Results[0].Error != "" || !strings.Contains(job.Results[0].Output, "private-output") {
		t.Fatalf("wrong terminal result: %+v", job)
	}
	m.mu.Lock()
	pending := len(m.cleanup[job.ID])
	m.mu.Unlock()
	d.mu.Lock()
	live, sends := len(d.live), d.sends
	d.mu.Unlock()
	if pending != 1 || live != 1 || sends != 1 {
		t.Fatalf("pending=%d live=%d sends=%d", pending, live, sends)
	}
	if !job.CleanupPending || job.CleanupRunning {
		t.Fatalf("missing live cleanup state: %+v", job)
	}
	if err := m.Remove("user", job.ID); !errors.Is(err, ErrBusy) {
		t.Fatal("unconfirmed reservation was discarded")
	}
}

type failingHostCleanupDriver struct {
	*fakeDriver
	hostID string
}

func (d *failingHostCleanupDriver) Close(ctx context.Context, session Session) error {
	if session.ID == d.hostID {
		return errors.New("lost close acknowledgement")
	}
	return d.fakeDriver.Close(ctx, session)
}

func TestStopWakesCleanupBackoffAndPreservesSuccessfulHosts(t *testing.T) {
	hosts := targets(14)
	d := &failingHostCleanupDriver{fakeDriver: newFake(), hostID: hosts[13].ID}
	m, err := Open(t.TempDir(), d, Options{Timeout: time.Hour, CleanupTimeout: time.Hour, RetryDelay: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	job, err := m.Create("user", strings.Repeat("c", 32), "true", hosts)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	ready := false
	for time.Now().Before(deadline) {
		job, _ = m.Get("user", job.ID)
		succeeded := 0
		for _, result := range job.Results {
			if result.State == "succeeded" {
				succeeded++
			}
		}
		ready = succeeded == 13 && job.Results[13].State == "closing"
		if ready {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !ready {
		t.Fatal("13/14 cleanup stall was not reached")
	}
	if err = m.Stop("user", job.ID); err != nil {
		t.Fatal(err)
	}
	job = settledBatch(t, m, job.ID)
	if job.State != "stopped" || job.Results[13].State != "succeeded" {
		t.Fatalf("stop did not settle: %+v", job)
	}
	for i, result := range job.Results {
		if result.Host != hosts[i] || result.Output == "" {
			t.Fatal("result/order/output was lost")
		}
		if i < 13 && result.State != "succeeded" {
			t.Fatal("successful host changed state")
		}
	}
}

type blockingCleanupDriver struct {
	*fakeDriver
	entered chan context.Context
	release chan struct{}
}

func (d *blockingCleanupDriver) Close(ctx context.Context, session Session) error {
	select {
	case d.entered <- ctx:
	default:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-d.release:
		return d.fakeDriver.Close(ctx, session)
	}
}

func TestRetryCleanupDoesNotResurrectBatchOrCancelAnExistingRetry(t *testing.T) {
	d := &blockingCleanupDriver{fakeDriver: newFake(), entered: make(chan context.Context, 1), release: make(chan struct{})}
	m, err := Open(t.TempDir(), d, Options{CleanupTimeout: time.Second, RetryDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	id := strings.Repeat("d", 32)
	now := time.Now().UTC()
	m.mu.Lock()
	m.jobs = []Job{{ID: id, UserID: "user", Command: "true", State: "stopped", CreatedAt: now, UpdatedAt: now, Results: []Result{{Host: targets(1)[0], State: "failed", Error: "terminal_close_failed", Output: "preserved output", OutputAvailable: true}}}}
	m.cleanup[id] = map[int]Session{0: {ID: "a"}}
	m.mu.Unlock()
	d.mu.Lock()
	d.live["a"] = true
	d.mu.Unlock()
	if err = m.Stop("user", id); err != nil {
		t.Fatal(err)
	}
	var retry context.Context
	select {
	case retry = <-d.entered:
	case <-time.After(time.Second):
		t.Fatal("retry did not start")
	}
	for range 5 {
		if err = m.Stop("user", id); err != nil {
			t.Fatal(err)
		}
	}
	job, _ := m.Get("user", id)
	if !job.CleanupPending || !job.CleanupRunning {
		t.Fatalf("retry progress missing: %+v", job)
	}
	listed, _, _ := m.List("user")
	if len(listed) != 1 || !listed[0].CleanupRunning {
		t.Fatalf("list lost cleanup state: %+v", listed)
	}
	duplicate, err := m.Create("user", id, "true", targets(1))
	if err != nil || !duplicate.CleanupRunning {
		t.Fatalf("idempotent create lost cleanup state: %+v %v", duplicate, err)
	}
	if job.State != "stopped" || job.Results[0].State != "failed" || retry.Err() != nil {
		t.Fatalf("retry reset or cancelled: job=%+v error=%v", job, retry.Err())
	}
	close(d.release)
	job = settledBatch(t, m, id)
	if job.CleanupPending || job.CleanupRunning || job.Results[0].Error != "" || job.Results[0].Output != "preserved output" {
		t.Fatal(job)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sends != 0 || len(d.live) != 0 {
		t.Fatal("cleanup replayed commands or failed to release")
	}
}

func TestHistoricalCleanupFailureDoesNotRestoreLiveCleanupOrBlockNewBatch(t *testing.T) {
	root := t.TempDir()
	d := newFake()
	d.closeFailures = 1 << 20
	m, err := Open(root, d, Options{CleanupTimeout: 20 * time.Millisecond, RetryDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	job, err := m.Create("user", strings.Repeat("f", 32), "true", targets(1))
	if err != nil {
		t.Fatal(err)
	}
	job = settledBatch(t, m, job.ID)
	if !job.CleanupPending {
		t.Fatal("fixture did not retain cleanup")
	}
	// Simulate an index saved by the earlier RC, which recorded closure failure
	// as an execution error. History must never become a live reservation.
	m.update(job.ID, 0, "failed", "terminal_close_failed", "", false)
	m.Close()
	data, err := os.ReadFile(m.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "cleanupPending") || strings.Contains(string(data), "cleanupRunning") {
		t.Fatal("runtime cleanup state was persisted")
	}
	restartedDriver := newFake()
	restarted, err := Open(root, restartedDriver, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	history, err := restarted.Get("user", job.ID)
	if err != nil || history.CleanupPending || history.CleanupRunning || history.Results[0].Error != "terminal_close_failed" {
		t.Fatalf("history and live cleanup were conflated: %+v %v", history, err)
	}
	if err = restarted.Stop("user", job.ID); err != nil {
		t.Fatal(err)
	}
	listed, _, _ := restarted.List("user")
	if len(listed) != 1 || listed[0].CleanupPending || listed[0].CleanupRunning {
		t.Fatal(listed)
	}
	restartedDriver.mu.Lock()
	sends := restartedDriver.sends
	restartedDriver.mu.Unlock()
	if sends != 0 {
		t.Fatal("history replayed a command")
	}
	next, err := restarted.Create("user", strings.Repeat("1", 32), "true", targets(1))
	if err != nil {
		t.Fatal(err)
	}
	if settledBatch(t, restarted, next.ID).Results[0].State != "succeeded" {
		t.Fatal("next batch did not execute")
	}
}

func TestPendingCleanupKeepsItsReservationWithoutBlockingOtherHosts(t *testing.T) {
	hosts := targets(2)
	d := &failingHostCleanupDriver{fakeDriver: newFake(), hostID: hosts[0].ID}
	m, err := Open(t.TempDir(), d, Options{CleanupTimeout: 20 * time.Millisecond, RetryDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	first, err := m.Create("user", strings.Repeat("2", 32), "true", hosts[:1])
	if err != nil {
		t.Fatal(err)
	}
	if !settledBatch(t, m, first.ID).CleanupPending {
		t.Fatal("missing cleanup fixture")
	}
	next, err := m.Create("user", strings.Repeat("3", 32), "true", hosts[1:])
	if err != nil {
		t.Fatal(err)
	}
	if settledBatch(t, m, next.ID).Results[0].State != "succeeded" {
		t.Fatal("healthy host was blocked")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.live[hosts[0].ID] || len(d.live) != 1 || d.sends != 2 {
		t.Fatal("unconfirmed reservation was discarded or command replayed")
	}
}

func TestCleanupDeadlineCancelsAStalledClose(t *testing.T) {
	d := &blockingCleanupDriver{fakeDriver: newFake(), entered: make(chan context.Context, 1), release: make(chan struct{})}
	m, err := Open(t.TempDir(), d, Options{Timeout: time.Hour, CleanupTimeout: 40 * time.Millisecond, RetryDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	job, err := m.Create("user", strings.Repeat("e", 32), "true", targets(1))
	if err != nil {
		t.Fatal(err)
	}
	job = settledBatch(t, m, job.ID)
	if job.Results[0].Error != "" || job.Results[0].State != "succeeded" {
		t.Fatal(job)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.live) != 1 {
		t.Fatal("unconfirmed close lost its reservation")
	}
}
