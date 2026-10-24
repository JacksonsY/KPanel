//go:build !linux

package hostbackup

import (
	"os"
	"path/filepath"
)

func openRestoreAncestor(root *os.Root) (*os.Root, error) {
	return openRestoreParent(filepath.Dir(root.Name()))
}

// Non-Linux platforms only run isolated, non-concurrent protocol fixtures.
func renameRestoreObject(from *os.Root, old string, to *os.Root, next string) error {
	if _, err := to.Lstat(next); !os.IsNotExist(err) {
		return os.ErrExist
	}
	if from == to {
		return from.Rename(old, next)
	}
	return os.Rename(filepath.Join(from.Name(), old), filepath.Join(to.Name(), next))
}
