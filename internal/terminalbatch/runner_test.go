package terminalbatch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/terminal"
)

type fakeDriver struct {
	mu            sync.Mutex
	inputs        map[string]string
	live          map[string]bool
	opens         int
	sends         int
	max           int
	limits        int
	closeFailures int
	ambiguous     bool
	block         bool
}

func (d *fakeDriver) Open(_ context.Context, _ string, h Host) (Session, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.limits > 0 {
		d.limits--
		return Session{}, terminal.ErrLimit
	}
	if d.ambiguous {
		d.opens++
		return Session{}, errors.New("lost open acknowledgement")
	}
	if len(d.live) >= 4 {
		return Session{}, terminal.ErrLimit
	}
	d.opens++
	d.live[h.ID] = true
	if len(d.live) > d.max {
		d.max = len(d.live)
	}
	return Session{ID: h.ID}, nil
}
func (d *fakeDriver) Input(_ context.Context, s Session, text string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sends++
	d.inputs[s.ID] = text
	return nil
}
func (d *fakeDriver) Output(ctx context.Context, s Session, _ int64) (terminal.Output, error) {
	d.mu.Lock()
	block := d.block
	text := d.inputs[s.ID]
	d.mu.Unlock()
	if block {
		<-ctx.Done()
		return terminal.Output{}, ctx.Err()
	}
	marker := regexp.MustCompile(`__KPANEL_DONE_[a-f0-9]+_`).FindString(text)
	return terminal.Output{Data: []byte("private-output\n" + marker + "0__\n"), NextOffset: 100}, nil
}
func (d *fakeDriver) Close(_ context.Context, s Session) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closeFailures > 0 {
		d.closeFailures--
		return errors.New("cleanup pending")
	}
	delete(d.live, s.ID)
	return nil
}
func newFake() *fakeDriver { return &fakeDriver{inputs: map[string]string{}, live: map[string]bool{}} }
func openTest(t *testing.T, d Driver) (*Manager, string) {
	t.Helper()
	root := t.TempDir()
	m, err := Open(root, d, Options{Timeout: time.Second, RetryDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m, root
}
func targets(n int) []Host {
	hosts := make([]Host, n)
	for i := range hosts {
		hosts[i] = Host{ID: string(rune('a' + i)), Name: "node", Shell: "posix"}
	}
	return hosts
}
func finished(t *testing.T, m *Manager) Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		jobs, _, _ := m.List("user")
		if len(jobs) > 0 && jobs[0].State != "running" {
			return jobs[0]
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("batch did not finish")
	return Job{}
}

func TestQueueWaitsForCapacityAndConfirmedCleanupWithoutReplaying(t *testing.T) {
	d := newFake()
	d.limits = 6
	d.closeFailures = 2
	m, root := openTest(t, d)
	command := "echo token=supersecret"
	j, err := m.Create("user", strings.Repeat("a", 32), command, targets(14))
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := m.Create("user", j.ID, command, targets(14))
	if err != nil || duplicate.ID != j.ID {
		t.Fatal(err)
	}
	done := finished(t, m)
	for _, r := range done.Results {
		if r.State != "succeeded" {
			t.Fatalf("result=%+v", r)
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.max > 4 || d.opens != 14 || d.sends != 14 || len(d.live) != 0 {
		t.Fatalf("max=%d opens=%d sends=%d live=%d", d.max, d.opens, d.sends, len(d.live))
	}
	data, err := os.ReadFile(filepath.Join(root, "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "supersecret") || strings.Contains(string(data), "private-output") {
		t.Fatal("secret command/output persisted")
	}
	if !strings.Contains(done.Results[0].Output, "private-output") {
		t.Fatal("memory output missing")
	}
}

func TestUnknownOpenOutcomeIsNeverRetried(t *testing.T) {
	d := newFake()
	d.ambiguous = true
	m, _ := openTest(t, d)
	_, err := m.Create("user", strings.Repeat("a", 32), "true", targets(1))
	if err != nil {
		t.Fatal(err)
	}
	job := finished(t, m)
	if job.Results[0].State != "failed" {
		t.Fatal(job)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.opens != 1 || d.sends != 0 {
		t.Fatal("uncertain open was replayed")
	}
}

func TestStopCancelsQueuedWorkAndClosesActiveSessions(t *testing.T) {
	d := newFake()
	d.block = true
	m, _ := openTest(t, d)
	j, err := m.Create("user", strings.Repeat("a", 32), "sleep 100", targets(14))
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Stop("other", j.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-user stop allowed")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		sends := d.sends
		d.mu.Unlock()
		if sends == 4 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err = m.Stop("user", j.ID); err != nil {
		t.Fatal(err)
	}
	j = finished(t, m)
	for _, r := range j.Results {
		if r.State != "stopped" {
			t.Fatalf("result=%+v", r)
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.opens != 4 || len(d.live) != 0 {
		t.Fatalf("opens=%d live=%d", d.opens, len(d.live))
	}
}

func TestRestartRetainsMetadataAndInterruptsWithoutReplay(t *testing.T) {
	d := newFake()
	m, root := openTest(t, d)
	saved := diskState{Version: 1, Jobs: []persistedJob{{Job: Job{ID: strings.Repeat("a", 32), Command: "sleep 100", State: "running", CreatedAt: time.Now(), UpdatedAt: time.Now(), Results: []Result{{Host: targets(1)[0], State: "running"}}}, Owner: "user"}}}
	data, _ := json.Marshal(saved)
	if err := os.WriteFile(filepath.Join(root, "jobs.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(root, d, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	jobs, _, available := restored.List("user")
	if !available || len(jobs) != 1 || jobs[0].State != "interrupted" || jobs[0].Results[0].OutputAvailable {
		t.Fatal(jobs)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.opens != 0 {
		t.Fatal("restart replayed command")
	}
	_ = m
}

func TestCorruptHistoryAndPersistenceFailureRejectNewExecution(t *testing.T) {
	d := newFake()
	root := t.TempDir()
	path := filepath.Join(root, "jobs.json")
	original := []byte("broken history")
	_ = os.WriteFile(path, original, 0600)
	m, err := Open(root, d, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if _, err = m.Create("user", strings.Repeat("a", 32), "true", targets(1)); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(original) {
		t.Fatal("corrupt history overwritten")
	}
	m2, _ := openTest(t, d)
	m2.write = func(string, []byte) error { return errors.New("disk full") }
	if _, err = m2.Create("user", strings.Repeat("b", 32), "true", targets(1)); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if d.opens != 0 {
		t.Fatal("executed without durable admission")
	}
}

func TestBoundsOwnershipAndDeletion(t *testing.T) {
	d := newFake()
	m, _ := openTest(t, d)
	if _, err := m.Create("user", strings.Repeat("a", 32), "true", targets(65)); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	j, err := m.Create("user", strings.Repeat("a", 32), " true", targets(1))
	if err != nil {
		t.Fatal(err)
	}
	finished(t, m)
	if j.Command != "[excluded from command history]" {
		t.Fatal("leading space command persisted")
	}
	jobs, _, _ := m.List("other")
	if len(jobs) != 0 {
		t.Fatal("cross-user list")
	}
	if err := m.Remove("other", j.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-user delete")
	}
	if err := m.Remove("user", j.ID); err != nil {
		t.Fatal(err)
	}
	jobs, _, _ = m.List("user")
	if len(jobs) != 0 {
		t.Fatal("delete failed")
	}
}
