package filemanager

import (
	"errors"
	"os"
	"strings"
)

// openDirectoryRoot resolves one component at a time and pins each checked
// directory before descending. The returned root owns that directory identity;
// callers must keep later reads, writes and publication relative to it.
func openDirectoryRoot(root *fileRoot, directory string, create bool) (*fileRoot, os.FileInfo, error) {
	normalized, err := normalizeVirtual(directory)
	if err != nil || normalized != directory {
		return nil, nil, ErrInvalidPath
	}
	pinned, info, err := openCheckedDirectory(rawFileRoot(root), rootName(directory), create, 0700)
	if err != nil {
		return nil, nil, err
	}
	return wrapFileRoot(pinned), info, nil
}

func openCheckedDirectory(root *os.Root, relative string, create bool, mode os.FileMode) (*os.Root, os.FileInfo, error) {
	cursor, err := root.OpenRoot(".")
	if err != nil {
		return nil, nil, err
	}
	info, err := cursor.Stat(".")
	if err != nil {
		_ = cursor.Close()
		return nil, nil, err
	}
	for _, component := range strings.Split(relative, "/") {
		if component == "." || component == "" {
			continue
		}
		if create {
			if err := cursor.Mkdir(component, mode); err != nil && !errors.Is(err, os.ErrExist) {
				_ = cursor.Close()
				return nil, nil, err
			}
		}
		before, err := cursor.Lstat(component)
		if err != nil {
			_ = cursor.Close()
			return nil, nil, err
		}
		if before.Mode()&os.ModeSymlink != 0 {
			_ = cursor.Close()
			return nil, nil, ErrSymlink
		}
		if !before.IsDir() {
			_ = cursor.Close()
			return nil, nil, ErrNotDirectory
		}
		next, err := cursor.OpenRoot(component)
		if err != nil {
			_ = cursor.Close()
			return nil, nil, err
		}
		opened, err := next.Stat(".")
		_ = cursor.Close()
		if err != nil || !os.SameFile(before, opened) {
			_ = next.Close()
			return nil, nil, ErrConflict
		}
		cursor, info = next, opened
	}
	return cursor, info, nil
}
