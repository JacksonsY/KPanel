//go:build !windows

package hostbackup

import (
	"errors"
	"os"
	"syscall"
)

func restoreOwnership(path string, uid, gid int) error        { return os.Chown(path, uid, gid) }
func restoreSymlinkOwnership(path string, uid, gid int) error { return os.Lchown(path, uid, gid) }
func fileOwnership(info os.FileInfo) (int, int, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, errors.New("file ownership unavailable")
	}
	return int(st.Uid), int(st.Gid), nil
}
func copyFileOwnership(f *os.File, info os.FileInfo) error {
	uid, gid, err := fileOwnership(info)
	if err != nil {
		return err
	}
	return f.Chown(uid, gid)
}
func copyOwnershipIn(root *os.Root, name string, info os.FileInfo) error {
	uid, gid, err := fileOwnership(info)
	if err != nil {
		return err
	}
	return root.Chown(name, uid, gid)
}
func copySymlinkOwnershipIn(root *os.Root, name string, info os.FileInfo) error {
	uid, gid, err := fileOwnership(info)
	if err != nil {
		return err
	}
	return root.Lchown(name, uid, gid)
}
func ownedByProcess(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Geteuid()
}
