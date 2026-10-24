package hostbackup

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

// openRestoreParent pins each directory component before traversing the next.
// A Root at the host filesystem root alone is insufficient: it permits links
// between two different host directories inside that root.
func openRestoreParent(path string) (*os.Root, error) {
	if !filepath.IsAbs(path) {
		return nil, backup.ErrInvalid
	}
	path = filepath.Clean(path)
	volume := filepath.VolumeName(path)
	root, err := os.OpenRoot(volume + string(filepath.Separator))
	if err != nil {
		return nil, err
	}
	for _, component := range strings.Split(strings.TrimPrefix(path[len(volume):], string(filepath.Separator)), string(filepath.Separator)) {
		if component == "" {
			continue
		}
		before, err := root.Lstat(component)
		if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			root.Close()
			return nil, errors.Join(backup.ErrInvalid, err)
		}
		next, err := root.OpenRoot(component)
		if err != nil {
			root.Close()
			return nil, err
		}
		after, err := next.Stat(".")
		root.Close()
		if err != nil || !os.SameFile(before, after) {
			next.Close()
			return nil, errors.Join(backup.ErrInvalid, err)
		}
		root = next
	}
	return root, nil
}

func (r *restoreRoot) bindParent(recovering bool) error {
	parent, err := openRestoreParent(filepath.Dir(r.Target))
	if err != nil {
		return err
	}
	identity, err := restoreParentIdentity(parent)
	if err != nil || recovering && (r.ParentIdentity == "" || r.ParentIdentity != identity) {
		parent.Close()
		return errors.Join(backup.ErrInvalid, err)
	}
	r.ParentIdentity, r.parent = identity, parent
	return nil
}

func (j *restoreJournal) closeParents() {
	for i := range j.Roots {
		if j.Roots[i].parent != nil {
			j.Roots[i].parent.Close()
			j.Roots[i].parent = nil
		}
		if j.Roots[i].workspace != nil {
			j.Roots[i].workspace.Close()
			j.Roots[i].workspace = nil
		}
		if j.Roots[i].workspaceParent != nil {
			j.Roots[i].workspaceParent.Close()
			j.Roots[i].workspaceParent = nil
		}
	}
}

// Detect a moved/replaced destination. This identity check is not sufficient for
// Docker's later pathname resolution: docker_handoff.go separately requires
// trusted bind ancestors. Filesystem cleanup keeps the transaction's parent.
func (j *restoreJournal) checkParentPaths() error {
	for _, entry := range j.Roots {
		parent, err := openRestoreParent(filepath.Dir(entry.Target))
		if err != nil {
			return err
		}
		identity, err := restoreParentIdentity(parent)
		parent.Close()
		if err != nil || identity != entry.ParentIdentity {
			return errors.Join(backup.ErrInvalid, err)
		}
	}
	return nil
}

func removeRestoreTree(parent *os.Root, name string) error {
	if err := restoreNoNestedMounts(parent, name); err != nil {
		return err
	}
	// Root.RemoveAll removes a final symlink itself, never its destination.
	return parent.RemoveAll(name)
}

func syncRestoreParent(parent *os.Root) error {
	if runtime.GOOS == "windows" {
		return nil // same directory-fsync policy as backup.SyncDir
	}
	f, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
