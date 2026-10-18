//go:build linux

package appmarket

import (
	"os"
	"syscall"
)

func trustedJobDirectoryOwner(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}

func trustedJobDirectoryMode(info os.FileInfo) bool {
	return info.Mode().Perm()&0o022 == 0
}
