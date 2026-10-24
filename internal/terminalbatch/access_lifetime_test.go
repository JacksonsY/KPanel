package terminalbatch

import (
	"context"
	"strings"
	"testing"
)

func TestTargetAccessLifetimeCancelsOldRemoteWorkWithoutStoppingLocal(t *testing.T) {
	driver := newFake()
	manager, _ := openTest(t, driver)
	lifetime, disable := context.WithCancel(context.Background())
	disable()
	// Even if a different lifetime is now enabled, targets already queued with
	// the old lifetime must never start. The unrelated local target still runs.
	_, err := manager.Create("user", strings.Repeat("b", 32), "echo safe", []Host{
		{ID: "remote", Name: "Remote", Shell: "posix", Context: lifetime},
		{ID: "local", Name: "Local", Shell: "posix"},
	})
	if err != nil {
		t.Fatal(err)
	}
	job := finished(t, manager)
	if job.Results[0].State != "stopped" || job.Results[1].State != "succeeded" {
		t.Fatal(job.Results)
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.opens != 1 || driver.inputs["remote"] != "" {
		t.Fatal("old remote command executed", driver.opens, driver.inputs)
	}
}
