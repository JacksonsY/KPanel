//go:build !windows

package hostbackup

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

func ownershipRoot(t *testing.T) (*Engine, *restoreJournal) {
	t.Helper()
	e := &Engine{Root: t.TempDir()}
	parent := filepath.Join(e.Root, "home")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	id := backup.NewID()
	target := filepath.Join(parent, "data")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "old"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	r := restoreRoot{Target: target, Previous: target + ".kpanel-restore-" + id, Next: target + ".kpanel-next-" + id, HadPrevious: true}
	if err := r.bindParent(false); err != nil {
		t.Fatal(err)
	}
	r.TargetIdentity, _ = restoreObjectIdentity(r.parent, "data")
	if err := r.createWorkspace(); err != nil {
		t.Fatal(err)
	}
	j := &restoreJournal{FilesystemVersion: 2, ID: id, Phase: "applying", Roots: []restoreRoot{r}}
	t.Cleanup(j.closeParents)
	return e, j
}

func ownershipStage(t *testing.T, r *restoreRoot) {
	t.Helper()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "new"), []byte("restored"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := copyTreeIn(context.Background(), source, r.workspace, "next", func(os.FileInfo) error {
		var err error
		r.NextIdentity, err = restoreObjectIdentity(r.workspace, "next")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func ownershipPublish(t *testing.T, r *restoreRoot) {
	t.Helper()
	if err := r.capture("data", "previous", r.TargetIdentity); err != nil {
		t.Fatal(err)
	}
	r.Applied = true
	if err := renameRestoreObject(r.workspace, "next", r.parent, "data"); err != nil {
		t.Fatal(err)
	}
}

func ownershipSubstitute(t *testing.T, path string) {
	t.Helper()
	if err := os.Rename(path, path+"-retained"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "sentinel"), []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
}

func ownershipSentinel(t *testing.T, path string) {
	t.Helper()
	if data, err := os.ReadFile(filepath.Join(path, "sentinel")); err != nil || string(data) != "untouched" {
		t.Fatalf("unknown object changed: %q %v", data, err)
	}
}

func TestRestoreOwnershipRollbackRetainsSubstitutedObjects(t *testing.T) {
	for _, name := range []string{"next", "previous", "target", "discard"} {
		t.Run(name, func(t *testing.T) {
			e, j := ownershipRoot(t)
			r := &j.Roots[0]
			ownershipStage(t, r)
			path := r.Next
			if name != "next" {
				ownershipPublish(t, r)
				path = r.Previous
			}
			if name == "target" {
				path = r.Target
			}
			if name == "discard" {
				if err := r.capture("data", "discard", r.NextIdentity); err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(r.Workspace, "discard")
			}
			ownershipSubstitute(t, path)
			if err := e.rollback(context.Background(), j); err == nil {
				t.Fatal("rollback accepted unknown object")
			}
			if name == "target" {
				path = filepath.Join(r.Workspace, "discard")
			}
			ownershipSentinel(t, path)
		})
	}
}

func TestRestoreOwnershipCleanupRetainsSubstitutedObjects(t *testing.T) {
	for _, name := range []string{"previous", "next"} {
		t.Run(name, func(t *testing.T) {
			e, j := ownershipRoot(t)
			r := &j.Roots[0]
			ownershipStage(t, r)
			if name == "previous" {
				ownershipPublish(t, r)
			}
			path := filepath.Join(r.Workspace, name)
			ownershipSubstitute(t, path)
			if err := e.cleanupRecovery(context.Background(), j); err == nil {
				t.Fatal("cleanup accepted unknown object")
			}
			ownershipSentinel(t, path)
		})
	}
}

func TestRestoreOwnershipTargetSwapBeforeCaptureIsQuarantined(t *testing.T) {
	e, j := ownershipRoot(t)
	r := &j.Roots[0]
	ownershipStage(t, r)
	ownershipSubstitute(t, r.Target)
	if err := r.capture("data", "previous", r.TargetIdentity); err == nil {
		t.Fatal("captured unknown original")
	}
	if err := e.rollback(context.Background(), j); err == nil {
		t.Fatal("restored unknown original")
	}
	ownershipSentinel(t, r.Previous)
	if data, err := os.ReadFile(filepath.Join(r.Target+"-retained", "old")); err != nil || string(data) != "original" {
		t.Fatal("original lost", err)
	}
}

func TestRestoreOwnershipRecoveryPreservesCommitPointAndUnknownData(t *testing.T) {
	for _, phase := range []string{"applying", "cleanup_pending"} {
		t.Run(phase, func(t *testing.T) {
			e, j := ownershipRoot(t)
			r := &j.Roots[0]
			ownershipStage(t, r)
			ownershipPublish(t, r)
			ownershipSubstitute(t, r.Previous)
			j.Phase = phase
			dir := filepath.Join(t.TempDir(), j.ID)
			if err := backup.PrivateDir(dir); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "restore-journal.json")
			if err := backup.WriteJSON(path, j); err != nil {
				t.Fatal(err)
			}
			j.closeParents()
			for n := 0; n < 2; n++ {
				if err := e.Recover(context.Background(), j.ID, dir); err == nil {
					t.Fatal("recovery accepted unknown data")
				}
				ownershipSentinel(t, r.Previous)
				var saved restoreJournal
				data, _ := os.ReadFile(path)
				if json.Unmarshal(data, &saved) != nil {
					t.Fatal("journal lost")
				}
				want := "needs_attention"
				if phase == "cleanup_pending" {
					want = phase
				}
				if saved.Phase != want {
					t.Fatalf("phase %s want %s", saved.Phase, want)
				}
				if data, err := os.ReadFile(filepath.Join(r.Target, "new")); err != nil || string(data) != "restored" {
					t.Fatal("committed destination changed", err)
				}
			}
		})
	}
}

func TestRestoreOwnershipRecoveryOfPartialStage(t *testing.T) {
	e, j := ownershipRoot(t)
	r := &j.Roots[0]
	dir := filepath.Join(t.TempDir(), j.ID)
	if err := backup.PrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(dir, "restore-journal.json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := copyTreeIn(ctx, t.TempDir(), r.workspace, "next", func(os.FileInfo) error {
		var err error
		r.NextIdentity, err = restoreObjectIdentity(r.workspace, "next")
		if err != nil {
			return err
		}
		return backup.WriteJSON(journalPath, j)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("copy=%v", err)
	}
	next := r.Next
	j.closeParents()
	for n := 0; n < 2; n++ {
		if err := e.Recover(context.Background(), j.ID, dir); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Lstat(next); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial stage retained", err)
	}
	if data, err := os.ReadFile(filepath.Join(r.Target, "old")); err != nil || string(data) != "original" {
		t.Fatal("original lost", err)
	}
}

func TestRestoreOwnershipRecoveryRejectsReplacementWorkspace(t *testing.T) {
	e, j := ownershipRoot(t)
	r := &j.Roots[0]
	ownershipStage(t, r)
	ownershipPublish(t, r)
	ownershipSubstitute(t, r.Workspace)
	dir := filepath.Join(t.TempDir(), j.ID)
	if err := backup.PrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := backup.WriteJSON(filepath.Join(dir, "restore-journal.json"), j); err != nil {
		t.Fatal(err)
	}
	j.closeParents()
	if err := e.Recover(context.Background(), j.ID, dir); err == nil {
		t.Fatal("accepted replacement workspace")
	}
	ownershipSentinel(t, r.Workspace)
	if data, err := os.ReadFile(filepath.Join(r.Target, "new")); err != nil || string(data) != "restored" {
		t.Fatal("replacement changed destination", err)
	}
}

func TestRestoreOwnershipFileOnlyWritableParentUsesProtectedWorkspace(t *testing.T) {
	e, d, payload := hostFixture(t)
	// File-only restoration keeps its pinned-handle guarantee in writable
	// parents. Such paths cannot subsequently be handed to Docker by name.
	payload.Containers = nil
	d.containers = map[string]*fixtureContainer{}
	if err := os.Chmod(e.host("/home"), 0777); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := e.writePayload(context.Background(), filepath.Join(directory, "apps.payload"), payload); err != nil {
		t.Fatal(err)
	}
	if err := e.Restore(context.Background(), backup.NewID(), directory, []string{"apps"}, false); err != nil {
		t.Fatalf("restore: %v", errors.Unwrap(err))
	}
	var j restoreJournal
	data, _ := os.ReadFile(filepath.Join(directory, "restore-journal.json"))
	if json.Unmarshal(data, &j) != nil || j.Phase != "completed" {
		t.Fatalf("journal=%s", data)
	}
	for _, r := range j.Roots {
		if filepath.Dir(r.Workspace) == filepath.Dir(r.Target) {
			t.Fatal("workspace remained in writable parent")
		}
		if _, err := os.Lstat(r.Workspace); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("completed workspace retained", err)
		}
	}
}

func TestRestoreOwnershipDirectoryBootstrapRejectsWritableParent(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0777); err != nil {
		t.Fatal(err)
	}
	root, err := openRestoreParent(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := copyTreeIn(context.Background(), t.TempDir(), root, "next"); err == nil {
		t.Fatal("unsafe mkdir/OpenRoot bootstrap accepted")
	}
	if _, err := root.Lstat("next"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("created an unbound stage", err)
	}
}

func TestRestoreOwnershipWorkspaceCreationUsesPinnedAncestor(t *testing.T) {
	base, outside := t.TempDir(), t.TempDir()
	parent := filepath.Join(base, "parent")
	if err := os.Mkdir(parent, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0777); err != nil {
		t.Fatal(err)
	}
	r := restoreRoot{Target: filepath.Join(parent, "data")}
	if err := r.bindParent(false); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parent, filepath.Join(base, "moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "sentinel"), []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(outside, parent); err != nil {
		t.Fatal(err)
	}
	if err := r.createWorkspace(); err != nil {
		t.Fatal(err)
	}
	j := &restoreJournal{Roots: []restoreRoot{r}}
	defer j.closeParents()
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 1 {
		t.Fatalf("created workspace through replacement parent: %v %v", entries, err)
	}
	ownershipSentinel(t, parent)
	if filepath.Dir(r.Workspace) != base {
		t.Fatalf("workspace not below pinned ancestor: %s", r.Workspace)
	}
	if err := j.checkParentPaths(); err == nil {
		t.Fatal("replacement parent remained usable")
	}
}

func TestRestoreOwnershipUnprivilegedSiblingRenameCannotReachWorkspace(t *testing.T) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		t.Skip("requires isolated Linux root to drop child credentials")
	}
	base := t.TempDir()
	for _, path := range []string{base, filepath.Dir(base)} {
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	parent := filepath.Join(base, "writable")
	if err := os.Mkdir(parent, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0777); err != nil {
		t.Fatal(err)
	}
	r := restoreRoot{Target: filepath.Join(parent, "data")}
	if err := r.bindParent(false); err != nil {
		t.Fatal(err)
	}
	if err := r.createWorkspace(); err != nil {
		t.Fatal(err)
	}
	j := &restoreJournal{Roots: []restoreRoot{r}}
	defer j.closeParents()
	ownershipStage(t, &r)
	victim := filepath.Join(parent, "root-owned-victim")
	if err := os.Mkdir(victim, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(victim, "sentinel"), []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	move := func(from, to string) error {
		cmd := exec.Command("/bin/mv", "--", from, to)
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
		return cmd.Run()
	}
	// The exact supported threat: the low-privilege user may rename a
	// root-owned nonempty sibling, without reading or writing its contents.
	if err := move(victim, victim+"-moved"); err != nil {
		t.Fatalf("threat precondition unavailable: %v", err)
	}
	if err := move(r.Next, filepath.Join(parent, "stolen")); err == nil {
		t.Fatal("unprivileged user moved the stage")
	}
	if err := move(victim+"-moved", r.Next); err == nil {
		t.Fatal("unprivileged user substituted the stage")
	}
	ownershipSentinel(t, victim+"-moved")
	if err := restoreIdentityAt(r.workspace, "next", r.NextIdentity); err != nil {
		t.Fatal("owned stage identity lost", err)
	}
}
