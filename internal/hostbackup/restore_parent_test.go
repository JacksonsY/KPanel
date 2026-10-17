package hostbackup

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

func requireParentSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
}

func TestRestoreParentRegularFileStageAndPublishStayPinned(t *testing.T) {
	base, outside := t.TempDir(), t.TempDir()
	parent, moved := filepath.Join(base, "parent"), filepath.Join(base, "moved")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(source, []byte("restored"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "data"), []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := openRestoreParent(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Rename(parent, moved); err != nil {
		t.Fatal(err)
	}
	requireParentSymlink(t, outside, parent)
	staged, err := copyTreeIn(context.Background(), source, root, "next")
	if err != nil {
		t.Fatal(err)
	}
	if err := publishStagedIn(root, "next", "data", staged); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(moved, "data")); err != nil || string(data) != "restored" {
		t.Fatalf("pinned publish = %q %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(outside, "data")); err != nil || string(data) != "sentinel" {
		t.Fatalf("outside publish = %q %v", data, err)
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 1 {
		t.Fatalf("outside gained staging files %v %v", entries, err)
	}
}

func TestRestoreParentRollbackAndCleanupStayPinned(t *testing.T) {
	for _, cleanup := range []bool{false, true} {
		t.Run(map[bool]string{false: "rollback", true: "cleanup"}[cleanup], func(t *testing.T) {
			base, outside := t.TempDir(), t.TempDir()
			parent, moved := filepath.Join(base, "parent"), filepath.Join(base, "moved")
			if err := os.Mkdir(parent, 0700); err != nil {
				t.Fatal(err)
			}
			entry := restoreRoot{Target: filepath.Join(parent, "data"), Previous: filepath.Join(parent, "previous"), Next: filepath.Join(parent, "next"), HadPrevious: true, Applied: true}
			for _, name := range []string{"data", "previous", "next"} {
				if err := os.WriteFile(filepath.Join(parent, name), []byte(name), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(outside, name), []byte("sentinel-"+name), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := entry.bindParent(false); err != nil {
				t.Fatal(err)
			}
			j := restoreJournal{Roots: []restoreRoot{entry}}
			defer j.closeParents()
			if err := os.Rename(parent, moved); err != nil {
				t.Fatal(err)
			}
			requireParentSymlink(t, outside, parent)
			if err := j.checkParentPaths(); err == nil {
				t.Fatal("accepted replaced ancestor")
			}
			e := &Engine{}
			var err error
			if cleanup {
				err = e.cleanupRecovery(context.Background(), &j)
			} else {
				err = e.rollback(context.Background(), &j)
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"data", "previous", "next"} {
				data, err := os.ReadFile(filepath.Join(outside, name))
				if err != nil || string(data) != "sentinel-"+name {
					t.Fatalf("outside %s changed: %q %v", name, data, err)
				}
			}
			want := "previous"
			if cleanup {
				want = "data"
			}
			if data, err := os.ReadFile(filepath.Join(moved, "data")); err != nil || string(data) != want {
				t.Fatalf("pinned data = %q, %v", data, err)
			}
			for _, name := range []string{"previous", "next"} {
				if _, err := os.Lstat(filepath.Join(moved, name)); !os.IsNotExist(err) {
					t.Fatalf("retained %s: %v", name, err)
				}
			}
		})
	}
}

func TestRestoreParentRecoveryRejectsReplacementAndLegacyIdentity(t *testing.T) {
	for _, mode := range []string{"symlink", "directory", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			e := &Engine{Root: t.TempDir()}
			parent := filepath.Join(e.Root, "home")
			if err := os.Mkdir(parent, 0700); err != nil {
				t.Fatal(err)
			}
			id := backup.NewID()
			dir := filepath.Join(t.TempDir(), id)
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(parent, "data")
			entry := restoreRoot{Target: target, Previous: target + ".kpanel-restore-" + id, Next: target + ".kpanel-next-" + id, Applied: true}
			if err := entry.bindParent(false); err != nil {
				t.Fatal(err)
			}
			entry.parent.Close()
			entry.parent = nil
			if mode == "legacy" {
				entry.ParentIdentity = ""
			} else {
				if err := os.Rename(parent, filepath.Join(e.Root, "original")); err != nil {
					t.Fatal(err)
				}
				if mode == "symlink" {
					requireParentSymlink(t, t.TempDir(), parent)
				} else if err := os.Mkdir(parent, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(target, []byte("sentinel"), 0600); err != nil {
				t.Fatal(err)
			}
			j := restoreJournal{ID: id, Phase: "applying", Roots: []restoreRoot{entry}}
			journal := filepath.Join(dir, "restore-journal.json")
			if err := backup.WriteJSON(journal, j); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(journal)
			if err := e.Recover(context.Background(), id, dir); err == nil {
				t.Fatal("recovered through unproven parent identity")
			}
			if data, err := os.ReadFile(target); err != nil || string(data) != "sentinel" {
				t.Fatalf("replacement changed %q: %v", data, err)
			}
			if after, _ := os.ReadFile(journal); string(after) != string(before) {
				t.Fatal("lost retained recovery journal")
			}
		})
	}
}

func TestRestoreParentSwapDuringRealRestoreDoesNotTouchOutside(t *testing.T) {
	e, d, payload := hostFixture(t)
	directory := t.TempDir()
	if err := os.WriteFile(e.host("/home/app/data"), []byte("new!"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.writePayload(context.Background(), filepath.Join(directory, "apps.payload"), payload); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.host("/home/app/data"), []byte("old!"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, "app"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "app", "data"), []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	// Perform the swap only after destination parents were pinned, at the
	// existing-container rename immediately before the filesystem apply loop.
	swapped := false
	e.Client.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		response, err := d.request(r)
		if !swapped && strings.HasSuffix(r.URL.Path, "/rename") {
			swapped = true
			if err := os.Rename(e.host("/home"), e.host("/moved-home")); err != nil {
				t.Fatal(err)
			}
			requireParentSymlink(t, outside, e.host("/home"))
		}
		return response, err
	})
	err := e.Restore(context.Background(), backup.NewID(), directory, []string{"apps"}, false)
	if !swapped || err == nil {
		t.Fatalf("swap=%t restore=%v", swapped, err)
	}
	if data, err := os.ReadFile(filepath.Join(outside, "app", "data")); err != nil || string(data) != "sentinel" {
		t.Fatalf("outside data changed %q %v", data, err)
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 1 {
		t.Fatalf("outside gained staging/previous objects: %v %v", entries, err)
	}
	if data, err := os.ReadFile(e.host("/moved-home/app/data")); err != nil || string(data) != "old!" {
		t.Fatalf("original data changed %q %v", data, err)
	}
	if entries, err := os.ReadDir(e.host("/moved-home")); err != nil || len(entries) != 1 {
		t.Fatalf("pinned staging cleanup failed %v %v", entries, err)
	}
}
