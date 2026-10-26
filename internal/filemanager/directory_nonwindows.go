//go:build !windows

package filemanager

import "os"

func openDirectoryRoot(root *fileRoot, directory string, create bool) (*fileRoot, os.FileInfo, error) {
	pinned, info, err := openOSDirectoryRoot(rawFileRoot(root), directory, create)
	if err != nil {
		return nil, nil, err
	}
	return wrapFileRoot(pinned), info, nil
}
