//go:build linux

package main

import (
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func openEnrollmentTokenDescriptor() (io.ReadCloser, error) {
	// Duplicate before creating an os.File: absent inherited FD3 may have
	// already been reused by the Go runtime. Never assume ownership of it.
	fd, err := unix.FcntlInt(3, unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "enrollment token descriptor")
	info, err := file.Stat()
	if err != nil || !(info.Mode().IsRegular() || info.Mode()&os.ModeNamedPipe != 0) {
		file.Close()
		return nil, errors.New("enrollment token descriptor unavailable")
	}
	return file, nil
}
