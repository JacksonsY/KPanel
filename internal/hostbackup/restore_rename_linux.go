package hostbackup

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// The descriptor pins the original directory even when an adversary renames
// any component of its former pathname. Only this process owns the fd number.
func openRestoreAncestor(root *os.Root) (*os.Root, error) {
	f, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return os.OpenRoot(fmt.Sprintf("/proc/self/fd/%d/..", f.Fd()))
}

// NOREPLACE prevents an attacker-created destination from being overwritten.
func renameRestoreObject(from *os.Root, old string, to *os.Root, next string) error {
	src, err := from.Open(".")
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := to.Open(".")
	if err != nil {
		return err
	}
	defer dst.Close()
	return unix.Renameat2(int(src.Fd()), old, int(dst.Fd()), next, unix.RENAME_NOREPLACE)
}
