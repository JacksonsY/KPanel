//go:build !windows

package filemanager

import "path"

// All ordinary rename, move, copy-publication, trash and restore paths use
// the same pinned-parent publication boundary as transactional receives.
func renameNoReplaceRoot(root *fileRoot, oldVirtual, newVirtual string) error {
	from, _, err := openDirectoryRoot(root, path.Dir(oldVirtual), false)
	if err != nil {
		return err
	}
	defer from.Close()
	to, _, err := openDirectoryRoot(root, path.Dir(newVirtual), false)
	if err != nil {
		return err
	}
	defer to.Close()
	return publishReceiveObject(from, path.Base(oldVirtual), to, path.Base(newVirtual), false)
}
