//go:build !windows

package hostbackup

import (
	"fmt"
	"os"
	"syscall"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

func restoreParentIdentity(root *os.Root) (string, error) {
	return restoreObjectIdentity(root, ".")
}

func restoreObjectIdentity(root *os.Root, name string) (string, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() && !info.Mode().IsRegular() {
		return "", backup.ErrInvalid
	}
	return fmt.Sprintf("unix:%d:%d", stat.Dev, stat.Ino), nil
}

func sameRestoreFilesystem(a, b os.FileInfo) bool {
	x, xok := a.Sys().(*syscall.Stat_t)
	y, yok := b.Sys().(*syscall.Stat_t)
	return xok && yok && x.Dev == y.Dev
}

func protectedRestoreDirectory(info os.FileInfo) bool {
	return ownedByProcess(info) && info.Mode().Perm()&0022 == 0
}
