package virtualmachine

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

func lockDirectory(dir, name string) (func(), error) {
	fd, err := unix.Open(filepath.Join(dir, name), unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	if err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, ErrBusy
	}
	return func() { _ = unix.Flock(fd, unix.LOCK_UN); _ = f.Close() }, nil
}
