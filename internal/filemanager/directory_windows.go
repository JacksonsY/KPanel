//go:build windows

package filemanager

import "os"

func openDirectoryRoot(root *fileRoot, directory string, create bool) (*fileRoot, os.FileInfo, error) {
	normalized, err := normalizeVirtual(directory)
	if err != nil || normalized != directory {
		return nil, nil, ErrInvalidPath
	}
	name := rootName(directory)
	if create {
		if err := root.MkdirAll(name, 0700); err != nil {
			return nil, nil, err
		}
	}
	before, err := root.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	pinned, err := root.OpenRoot(name)
	if err != nil {
		return nil, nil, err
	}
	opened, err := pinned.Stat(".")
	if err != nil || !os.SameFile(before, opened) {
		pinned.Close()
		return nil, nil, ErrConflict
	}
	return pinned, opened, nil
}
