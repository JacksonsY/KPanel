//go:build !windows

package sitetraffic

import (
	"fmt"
	"os"
	"syscall"
)

func openRegular(root *os.Root, name string) (*os.File, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		file.Close()
		return nil, fmt.Errorf("file changed")
	}
	return file, nil
}
func fileID(file *os.File) (string, error) {
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	s := info.Sys().(*syscall.Stat_t)
	return fmt.Sprintf("%x-%x", s.Dev, s.Ino), nil
}
