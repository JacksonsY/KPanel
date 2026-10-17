package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A running job keeps its lease until the worker returns, whatever a
// concurrent abort request saw when it checked the record.
func TestAbortDoesNotReleaseARunningJob(t *testing.T) {
	m, err := OpenManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	record, err := m.Reserve("import", nil)
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	if err := m.Run(record.ID, func(ctx context.Context, _ string) error {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := m.Abort(record.ID, "upload_cancelled"); !errors.Is(err, ErrBusy) {
		t.Fatal("aborted a running job", err)
	}
	if _, err := m.Reserve("export", []string{"panel"}); !errors.Is(err, ErrBusy) {
		t.Fatal("a second job started beside a running one", err)
	}
	close(release)
	finished, _ := m.Get(record.ID)
	for deadline := time.Now().Add(5 * time.Second); finished.Status == "running" && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
		finished, _ = m.Get(record.ID)
	}
	if finished.Status != "ready" || m.Busy() {
		t.Fatal("running job did not finish normally", finished, m.Busy())
	}
}

// Cancellation must not hide a restore that still needs recovery.
func TestCancelledRestoreKeepsRecoveryCode(t *testing.T) {
	dir := t.TempDir()
	m, err := OpenManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	record, err := m.Reserve("restore", []string{"apps"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Run(record.ID, func(ctx context.Context, _ string) error {
		<-ctx.Done()
		return &Failure{Code: "recovery_required", Err: ctx.Err()}
	}); err != nil {
		t.Fatal(err)
	}
	m.Close()
	reopened, err := OpenManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, _ := reopened.Get(record.ID)
	if got.ErrorCode != "recovery_required" {
		t.Fatal("recovery state was replaced", got.ErrorCode)
	}
}

// A record written by a newer release must not stop an older one from
// opening the manager after a rollback.
func TestOpenManagerIgnoresFieldsFromNewerReleases(t *testing.T) {
	dir := t.TempDir()
	m, err := OpenManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	record, err := m.Reserve("export", []string{"panel"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Abort(record.ID, "start_failed"); err != nil {
		t.Fatal(err)
	}
	m.Close()
	path := filepath.Join(dir, record.ID, "record.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body = append([]byte(`{"fieldFromNewerRelease":true,`), body[1:]...)
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenManager(dir)
	if err != nil {
		t.Fatal("record from a newer release blocked startup", err)
	}
	defer reopened.Close()
	if got, err := reopened.Get(record.ID); err != nil || got.ErrorCode != "start_failed" {
		t.Fatal(got, err)
	}
}
