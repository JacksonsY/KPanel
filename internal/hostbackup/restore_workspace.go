package hostbackup

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

func requireRestorePrivateParent(root *os.Root) error {
	info, err := root.Stat(".")
	if err != nil || !info.IsDir() || !protectedRestoreDirectory(info) {
		return errors.Join(backup.ErrInvalid, err)
	}
	return nil
}

// Staging and retained copies live under a protected directory on the same
// filesystem. The destination parent may be writable by another local user;
// an inode check followed by RemoveAll in that parent would still race.
func (r *restoreRoot) createWorkspace() error {
	parentInfo, err := r.parent.Stat(".")
	if err != nil {
		return err
	}
	anchor, err := r.parent.OpenRoot(".")
	if err != nil {
		return err
	}
	for path := filepath.Dir(r.Target); ; path = filepath.Dir(path) {
		info, statErr := anchor.Stat(".")
		if statErr != nil || !sameRestoreFilesystem(parentInfo, info) {
			anchor.Close()
			return errors.Join(errors.New("restore needs a protected workspace on the destination filesystem"), statErr)
		}
		if requireRestorePrivateParent(anchor) == nil {
			name := ".kpanel-restore-" + backup.NewID()
			if err := anchor.Mkdir(name, 0700); err != nil {
				anchor.Close()
				return err
			}
			workspace, err := anchor.OpenRoot(name)
			if err == nil {
				err = syncRestoreParent(anchor)
			}
			if err != nil {
				if workspace != nil {
					workspace.Close()
				}
				anchor.Close()
				return err
			}
			r.workspaceParent = anchor
			r.Workspace, r.workspace = filepath.Join(path, name), workspace
			r.WorkspaceIdentity, err = restoreParentIdentity(workspace)
			r.Previous, r.Next = filepath.Join(r.Workspace, "previous"), filepath.Join(r.Workspace, "next")
			return err
		}
		if filepath.Dir(path) == path {
			anchor.Close()
			return backup.ErrInvalid
		}
		// Ascend from the pinned object, never from Target's replaceable path.
		// A later path mismatch leaves this journal available for attention.
		next, err := openRestoreAncestor(anchor)
		anchor.Close()
		if err != nil {
			return err
		}
		anchor = next
	}
}

func (r *restoreRoot) bindWorkspace() error {
	if r.WorkspaceIdentity == "" || r.Workspace == "" || r.Previous != filepath.Join(r.Workspace, "previous") || r.Next != filepath.Join(r.Workspace, "next") {
		return backup.ErrInvalid
	}
	anchor, err := openRestoreParent(filepath.Dir(r.Workspace))
	if err != nil {
		return err
	}
	if err := requireRestorePrivateParent(anchor); err != nil {
		anchor.Close()
		return err
	}
	root, err := anchor.OpenRoot(filepath.Base(r.Workspace))
	if err != nil {
		anchor.Close()
		return err
	}
	identity, err := restoreParentIdentity(root)
	if err != nil || identity != r.WorkspaceIdentity || requireRestorePrivateParent(root) != nil {
		root.Close()
		anchor.Close()
		return errors.Join(backup.ErrInvalid, err)
	}
	r.workspace, r.workspaceParent = root, anchor
	return nil
}

func restoreIdentityAt(root *os.Root, name, identity string) error {
	actual, err := restoreObjectIdentity(root, name)
	if err != nil {
		return err
	}
	if identity == "" || identity != actual {
		return errors.New("restore object ownership changed; retained recovery data requires attention")
	}
	return nil
}

// Capture first into the protected workspace, then verify. A path swap before
// rename can only quarantine an unknown object, never delete or overwrite it.
// Retain that object under its journaled name for operator inspection.
func (r *restoreRoot) capture(source, destination, identity string) error {
	if r.workspace == nil || identity == "" {
		return backup.ErrInvalid
	}
	if err := renameRestoreObject(r.parent, source, r.workspace, destination); err != nil {
		return err
	}
	if err := restoreIdentityAt(r.workspace, destination, identity); err != nil {
		return err
	}
	return errors.Join(syncRestoreParent(r.parent), syncRestoreParent(r.workspace))
}

func (r *restoreRoot) removeOwned(name, identity string) error {
	if r.workspace == nil {
		return backup.ErrInvalid
	}
	if err := restoreIdentityAt(r.workspace, name, identity); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	// The containing directory is pinned and private throughout this operation;
	// an unprivileged caller cannot exchange its direct children after validation.
	return removeRestoreTree(r.workspace, name)
}

func (r *restoreRoot) rollbackData() error {
	if r.parent == nil {
		return backup.ErrInvalid
	}
	if r.workspace == nil {
		// Nothing was staged yet. Never inspect legacy sibling paths as owned.
		if r.Workspace != "" || r.Applied {
			return backup.ErrInvalid
		}
		if r.HadPrevious {
			return restoreIdentityAt(r.parent, filepath.Base(r.Target), r.TargetIdentity)
		}
		return nil
	}
	target := filepath.Base(r.Target)
	_, previousErr := r.workspace.Lstat("previous")
	if previousErr != nil && !errors.Is(previousErr, os.ErrNotExist) {
		return previousErr
	}
	if previousErr == nil {
		if err := restoreIdentityAt(r.workspace, "previous", r.TargetIdentity); err != nil {
			return err
		}
	}
	if previousErr == nil || r.Applied && !r.HadPrevious {
		if _, err := r.workspace.Lstat("discard"); err == nil {
			if err := r.removeOwned("discard", r.NextIdentity); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if _, err := r.parent.Lstat(target); err == nil {
			if err := r.capture(target, "discard", r.NextIdentity); err != nil {
				return err
			}
			if err := r.removeOwned("discard", r.NextIdentity); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if previousErr == nil {
			if err := renameRestoreObject(r.workspace, "previous", r.parent, target); err != nil {
				return err
			}
		}
	}
	if r.HadPrevious {
		if err := restoreIdentityAt(r.parent, target, r.TargetIdentity); err != nil {
			return err
		}
	}
	return r.removeOwned("next", r.NextIdentity)
}

// Remove only empty, verified workspaces after the terminal journal is durable.
// Failed best-effort removal leaves an empty private directory, never data loss.
func (j *restoreJournal) releaseWorkspaces() {
	for index := range j.Roots {
		r := &j.Roots[index]
		if r.Workspace == "" {
			continue
		}
		anchor := r.workspaceParent
		if anchor == nil {
			continue
		}
		if requireRestorePrivateParent(anchor) == nil && restoreIdentityAt(anchor, filepath.Base(r.Workspace), r.WorkspaceIdentity) == nil {
			if r.workspace != nil {
				r.workspace.Close()
				r.workspace = nil
			}
			_ = anchor.Remove(filepath.Base(r.Workspace))
		}
	}
}
