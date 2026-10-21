package terminalbatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRetainsEveryRecentBatchAndHostOrderAcrossRestart(t *testing.T) {
	driver := newFake()
	manager, root := openTest(t, driver)
	hosts := targets(14)
	// Deliberately use a non-alphabetical submission order.
	hosts[0], hosts[13] = hosts[13], hosts[0]
	for round := 1; round <= 12; round++ {
		driver.mu.Lock()
		driver.inputs = map[string]string{}
		driver.mu.Unlock()
		id := fmt.Sprintf("%032x", round)
		if _, err := manager.Create("user", id, "true", hosts); err != nil {
			t.Fatal(err)
		}
		finished(t, manager)
	}
	check := func(m *Manager, restored bool) {
		t.Helper()
		jobs, warning, available := m.List("user")
		if !available || warning != "" || len(jobs) != 12 {
			t.Fatalf("available=%v warning=%q jobs=%d", available, warning, len(jobs))
		}
		for round, job := range jobs {
			if job.ID != fmt.Sprintf("%032x", 12-round) || len(job.Results) != len(hosts) {
				t.Fatalf("missing/reordered batch: %+v", job)
			}
			for index, result := range job.Results {
				if result.Host != hosts[index] || result.State != "succeeded" {
					t.Fatalf("missing/reordered host %d: %+v", index, result)
				}
				if !restored && round < maxOutputJobs && (!result.OutputAvailable || !strings.Contains(result.Output, "private-output")) {
					t.Fatal("recent output was released prematurely")
				}
				if (restored || round >= maxOutputJobs) && (result.OutputAvailable || result.Output != "") {
					t.Fatal("released output was still reported as available")
				}
			}
		}
	}
	check(manager, false)
	manager.Close()
	restored, err := Open(root, driver, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	check(restored, true)
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.opens != 12*len(hosts) {
		t.Fatal("restart replayed commands")
	}
}

func TestHistoryExpiresAfterOneDayWithoutRemovingActiveOrPendingCleanup(t *testing.T) {
	manager, root := openTest(t, newFake())
	now := time.Now().UTC()
	makeJob := func(n int, age time.Duration, state string) Job {
		return Job{ID: fmt.Sprintf("%032x", n), UserID: "user", Command: "true", State: state, CreatedAt: now.Add(-age), UpdatedAt: now.Add(-age), Results: []Result{{Host: targets(1)[0], State: "succeeded"}}}
	}
	recent := makeJob(1, 23*time.Hour, "completed")
	expired := makeJob(2, 25*time.Hour, "completed")
	running := makeJob(3, 25*time.Hour, "running")
	cleanup := makeJob(4, 25*time.Hour, "completed")
	manager.mu.Lock()
	manager.jobs = []Job{recent, expired, running, cleanup}
	manager.cleanup[cleanup.ID] = map[int]Session{0: {ID: "pending"}}
	manager.mu.Unlock()
	jobs, _, _ := manager.List("user")
	if len(jobs) != 3 {
		t.Fatalf("unexpected visible records: %+v", jobs)
	}
	if _, err := manager.Get("user", expired.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("expired record remained accessible")
	}
	manager.mu.Lock()
	manager.pruneLocked(now)
	count := len(manager.jobs)
	delete(manager.cleanup, cleanup.ID)
	// Cleanup is now confirmed; only the recent completed record must remain.
	manager.jobs[1].State = "completed"
	manager.pruneLocked(now)
	err := manager.persistLocked()
	manager.mu.Unlock()
	if count != 3 || err != nil {
		t.Fatalf("protected records=%d persist=%v", count, err)
	}
	data, err := os.ReadFile(filepath.Join(root, "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var disk diskState
	if err = json.Unmarshal(data, &disk); err != nil {
		t.Fatal(err)
	}
	if len(disk.Jobs) != 1 || disk.Jobs[0].ID != recent.ID {
		t.Fatalf("expired records persisted: %+v", disk.Jobs)
	}
	restored, err := Open(root, newFake(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	jobs, _, _ = restored.List("user")
	if len(jobs) != 1 || jobs[0].ID != recent.ID {
		t.Fatalf("unexpected restored history: %+v", jobs)
	}
}
