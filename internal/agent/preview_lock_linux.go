//go:build linux

package agent

import (
	"golang.org/x/sys/unix"
	"os"
)

func lockPreviewDisk(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}
