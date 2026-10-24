//go:build !windows

package hostbackup

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

func TestCreateRejectsUnsafeSourceBeforeStopping(t *testing.T) {
	e, d, p := hostFixture(t)
	d.containers[p.Containers[0].ID].State.Running = true
	if err := os.Chmod(e.host("/home"), 0777); err != nil {
		t.Fatal(err)
	}
	i, err := e.Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := e.Create(context.Background(), directory, []string{"apps"}, i.Revision); err == nil || !strings.Contains(err.Error(), "trusted") {
		t.Fatalf("unsafe source accepted: %v", err)
	}
	for _, call := range d.calls {
		if strings.HasPrefix(call, "POST ") {
			t.Fatalf("unsafe preflight changed Docker: %s", call)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "export-journal.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preflight left recovery journal: %v", err)
	}
}

func TestCreateRefusesUnsafeRestartAndRetainsRecoveryJournal(t *testing.T) {
	e, d, p := hostFixture(t)
	id := p.Containers[0].ID
	d.containers[id].State.Running = true
	i, err := e.Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "data"), []byte("outside-marker"), 0600); err != nil {
		t.Fatal(err)
	}
	stopped := false
	e.Client.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		response, err := d.request(r)
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/stop") {
			stopped = true
			if err := os.Chmod(e.host("/home"), 0777); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(e.host("/home/app"), e.host("/home/app-old")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, e.host("/home/app")); err != nil {
				t.Fatal(err)
			}
		}
		return response, err
	})
	directory := filepath.Join(t.TempDir(), backup.NewID())
	err = e.Create(context.Background(), directory, []string{"apps"}, i.Revision)
	if !stopped || backup.FailureCode(err) != "recovery_required" || d.containers[id].State.Running {
		t.Fatalf("unsafe restart: stopped=%t running=%t err=%v", stopped, d.containers[id].State.Running, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "export-journal.json")); err != nil {
		t.Fatalf("lost source recovery journal: %v", err)
	}
	err = e.Recover(context.Background(), filepath.Base(directory), directory)
	if backup.FailureCode(err) != "recovery_required" {
		t.Fatalf("unsafe recovery was not visible: %v", err)
	}
	for _, call := range d.calls {
		if strings.HasSuffix(call, "/start") {
			t.Fatalf("unsafe source sent to Docker start: %s", call)
		}
	}
}

func TestCreateRestartsTrustedSourceAfterCancelledExport(t *testing.T) {
	e, d, p := hostFixture(t)
	id := p.Containers[0].ID
	d.containers[id].State.Running = true
	i, err := e.Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := false
	e.Client.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		response, err := d.request(r)
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/stop") {
			stopped = true
		}
		return response, err
	})
	e.exportHook = func(stage, relative string) {
		// Measure and export share the walker; only the export opens contents.
		if stopped && stage == "file-opened" {
			cancel()
		}
	}
	directory := t.TempDir()
	err = e.Create(ctx, directory, []string{"apps"}, i.Revision)
	if !errors.Is(err, context.Canceled) || !stopped || !d.containers[id].State.Running {
		t.Fatalf("trusted source recovery: stopped=%t running=%t err=%v", stopped, d.containers[id].State.Running, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "export-journal.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful restart left journal: %v", err)
	}
}

func TestRecoverExportChecksCurrentDaemonMounts(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		t.Run(map[bool]string{false: "trusted", true: "changed-actual-mount"}[unsafe], func(t *testing.T) {
			e, d, p := hostFixture(t)
			id := p.Containers[0].ID
			if unsafe {
				if err := os.MkdirAll(e.host("/home/mutable/leaf"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(e.host("/home/mutable"), 0777); err != nil {
					t.Fatal(err)
				}
				d.containers[id].Mounts = []Mount{{Type: "bind", Source: "/home/mutable/leaf"}}
			}
			job := backup.NewID()
			directory := filepath.Join(t.TempDir(), job)
			if err := backup.PrivateDir(directory); err != nil {
				t.Fatal(err)
			}
			journal := filepath.Join(directory, "export-journal.json")
			// The journal's mount declaration must never override the daemon.
			if err := backup.WriteJSON(journal, p.Containers); err != nil {
				t.Fatal(err)
			}
			err := e.Recover(context.Background(), job, directory)
			if unsafe {
				if backup.FailureCode(err) != "recovery_required" || d.containers[id].State.Running {
					t.Fatalf("unsafe current mounts started: %v", err)
				}
				if _, err := os.Stat(journal); err != nil {
					t.Fatal(err)
				}
			} else {
				if err != nil || !d.containers[id].State.Running {
					t.Fatalf("trusted source not recovered: %v", err)
				}
				if _, err := os.Stat(journal); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("completed recovery left journal: %v", err)
				}
			}
		})
	}
}

func TestRestoreStartChecksCurrentDaemonMounts(t *testing.T) {
	e, d, p := hostFixture(t)
	p.Containers[0].Running = true
	directory := t.TempDir()
	if err := e.writePayload(context.Background(), filepath.Join(directory, "apps.payload"), p); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(e.host("/mutable/leaf"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(e.host("/mutable"), 0777); err != nil {
		t.Fatal(err)
	}
	created := false
	e.Client.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		response, err := d.request(r)
		if r.Method == "POST" && r.URL.Path == "/containers/create" {
			created = true
			d.containers[strings.Repeat("c", 64)].Mounts = []Mount{{Type: "bind", Source: "/mutable/leaf"}}
		}
		return response, err
	})
	if err := e.Restore(context.Background(), backup.NewID(), directory, []string{"apps"}, false); err == nil || !created {
		t.Fatalf("unsafe actual destination mounts accepted: created=%t err=%v", created, err)
	}
	for _, call := range d.calls {
		if strings.HasSuffix(call, "/start") {
			t.Fatalf("unsafe actual destination mounts started: %s", call)
		}
	}
}
