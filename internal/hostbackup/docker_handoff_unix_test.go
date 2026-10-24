//go:build !windows

package hostbackup

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

func TestRestoreDockerRejectsWritableBindAncestorBeforeHandoff(t *testing.T) {
	e, d, p := hostFixture(t)
	if err := os.Chmod(e.host("/home"), 0777); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := e.writePayload(context.Background(), filepath.Join(directory, "apps.payload"), p); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "data"), []byte("outside-sensitive-marker"), 0600); err != nil {
		t.Fatal(err)
	}
	handedOff := false
	e.Client.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == "POST" && r.URL.Path == "/containers/create" {
			handedOff = true
			// Deterministic daemon-side re-resolution, after the final parent
			// identity check: an untrusted parent writer can replace the leaf.
			if err := os.Rename(e.host("/home/app"), e.host("/home/app-moved")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, e.host("/home/app")); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(e.host("/home/app/data"))
			if err != nil || string(data) != "outside-sensitive-marker" {
				t.Fatalf("daemon resolution = %q, %v", data, err)
			}
		}
		return d.request(r)
	})
	err := e.Restore(context.Background(), backup.NewID(), directory, []string{"apps"}, false)
	if handedOff || err == nil {
		t.Fatalf("unsafe Docker handoff=%t, restore=%v", handedOff, err)
	}
	if data, err := os.ReadFile(e.host("/home/app/data")); err != nil || string(data) != "old!" {
		t.Fatalf("preflight changed original data: %q, %v", data, err)
	}
	if entries, err := os.ReadDir(e.host("/home")); err != nil || len(entries) != 1 || entries[0].Name() != "app" {
		t.Fatalf("preflight left transaction objects: %v, %v", entries, err)
	}
}

func TestRestoreDockerNestedBindParentsMustBeTrusted(t *testing.T) {
	for _, where := range []string{"root", "nested", "owner"} {
		t.Run(where, func(t *testing.T) {
			e, d, p := hostFixture(t)
			for _, name := range []string{"nested", "nested/leaf"} {
				if err := os.Mkdir(e.host("/home/app/"+name), 0755); err != nil {
					t.Fatal(err)
				}
			}
			c := &p.Containers[0]
			c.Mounts = append(c.Mounts, Mount{Type: "bind", Source: "/home/app/nested/leaf", Destination: "/nested"})
			c.HostConfig["Binds"] = rawJSON([]string{"/home/app:/data:rw", "/home/app/nested/leaf:/nested:ro"})
			mutable := e.host("/home/app")
			if where != "root" {
				mutable += "/nested"
			}
			if where == "owner" {
				if os.Geteuid() != 0 {
					t.Skip("different owner fixture requires root")
				}
				if err := os.Chown(mutable, 65534, 65534); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Chmod(mutable, 0777); err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			if err := e.writePayload(context.Background(), filepath.Join(directory, "apps.payload"), p); err != nil {
				t.Fatal(err)
			}
			// Fix the live parent after export: only the archive carries the
			// unsafe permissions, so a live-only preflight would miss this.
			if err := os.Chown(mutable, os.Geteuid(), os.Getegid()); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(mutable, 0755); err != nil {
				t.Fatal(err)
			}
			err := e.Restore(context.Background(), backup.NewID(), directory, []string{"apps"}, false)
			if err == nil || !strings.Contains(err.Error(), "trusted") {
				t.Fatalf("unsafe archived bind parent accepted: %v", err)
			}
			for _, call := range d.calls {
				if strings.HasPrefix(call, "POST ") || strings.HasPrefix(call, "DELETE ") {
					t.Fatalf("preflight changed Docker: %s", call)
				}
			}
			if data, err := os.ReadFile(e.host("/home/app/data")); err != nil || string(data) != "old!" {
				t.Fatalf("preflight changed data: %q, %v", data, err)
			}
		})
	}
}

func TestRestoreDockerKeepsApplicationOwnedLeaf(t *testing.T) {
	e, _, p := hostFixture(t)
	if err := os.Chmod(e.host("/home/app"), 0777); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		if err := os.Chown(e.host("/home/app"), 65534, 65534); err != nil {
			t.Fatal(err)
		}
	}
	directory := t.TempDir()
	if err := e.writePayload(context.Background(), filepath.Join(directory, "apps.payload"), p); err != nil {
		t.Fatal(err)
	}
	if err := e.Restore(context.Background(), backup.NewID(), directory, []string{"apps"}, false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(e.host("/home/app"))
	if err != nil || info.Mode().Perm() != 0777 {
		t.Fatalf("leaf permissions lost: %v, %v", info, err)
	}
	if os.Geteuid() == 0 && ownedByProcess(info) {
		t.Fatal("application leaf ownership lost")
	}
}

func TestStableDockerBindPathTrustedAliases(t *testing.T) {
	e := &Engine{Root: t.TempDir()}
	for _, name := range []string{"/var", "/run", "/trusted", "/writable/nested"} {
		if err := os.MkdirAll(e.host(name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"/run/docker.sock", "/trusted/file"} {
		if err := os.WriteFile(e.host(name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for source, target := range map[string]string{"/var/run": "../run", "/absolute": "/run", "/indirect": "/writable/nested", "/alias": "/indirect/../../trusted/file", "/loop": "/loop"} {
		if err := os.Symlink(target, e.host(source)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(e.host("/writable"), 0777); err != nil {
		t.Fatal(err)
	}
	for source, valid := range map[string]bool{"/var/run/docker.sock": true, "/absolute/docker.sock": true, "/run/missing": true, "/alias": false, "/loop": false, "/writable/nested": false} {
		if err := stableDockerBindPath(e.Root, source, true); (err == nil) != valid {
			t.Errorf("%s valid=%t: %v", source, valid, err)
		}
	}
}

func TestRestoreRollbackRefusesUntrustedOldBind(t *testing.T) {
	e, d, p := hostFixture(t)
	if err := os.Chmod(e.host("/home"), 0777); err != nil {
		t.Fatal(err)
	}
	j := &restoreJournal{Containers: []restoreContainer{{Name: p.Containers[0].Name, OldID: p.Containers[0].ID, OldRunning: true}}}
	if err := e.rollback(context.Background(), j); err == nil {
		t.Fatal("unsafe old bind restarted")
	}
	for _, call := range d.calls {
		if strings.HasSuffix(call, "/start") {
			t.Fatal("started old container with untrusted bind")
		}
	}
}

func TestStableDockerBindPreventsUnprivilegedRedirection(t *testing.T) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		t.Skip("requires Linux root to drop child credentials")
	}
	base := t.TempDir()
	for _, name := range []string{base, filepath.Dir(base)} {
		if err := os.Chmod(name, 0755); err != nil {
			t.Fatal(err)
		}
	}
	leaf := filepath.Join(base, "parent", "data")
	if err := os.MkdirAll(leaf, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(leaf, 65534, 65534); err != nil {
		t.Fatal(err)
	}
	if err := stableDockerBindPath(base, "/parent/data", false); err != nil {
		t.Fatal(err)
	}
	run := func(command string, args ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, command, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
		return cmd.Run()
	}
	if err := run("/usr/bin/touch", filepath.Join(leaf, "application-data")); err != nil {
		t.Fatalf("application could not write its data: %v", err)
	}
	if err := run("/bin/mv", "--", leaf, leaf+"-moved"); err == nil {
		t.Fatal("application redirected a verified Docker bind source")
	}
	if _, err := os.Stat(filepath.Join(leaf, "application-data")); err != nil {
		t.Fatal(err)
	}
}
