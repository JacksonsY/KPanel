//go:build !windows

package hostbackup

import (
	"fmt"
	"os"
	"syscall"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

func restoreParentIdentity(root *os.Root) (string, error) {
	info, err := root.Stat(".")
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", backup.ErrInvalid
	}
	return fmt.Sprintf("unix:%d:%d", stat.Dev, stat.Ino), nil
}
