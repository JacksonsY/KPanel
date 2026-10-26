//go:build !windows

package filemanager

import "os"

func openAccessTarget(root *fileRoot, name string) (*os.File, error) { return root.Open(name) }

// Source-preserving operations start private; final metadata is applied only
// after copying content. Ordinary copies inherit the destination policy.
func createFileWithSourceAccess(root *fileRoot, name string, flags int, mode os.FileMode, source *os.File) (*os.File, error) {
	if source != nil {
		mode = 0600
	}
	return root.OpenFile(name, flags, mode)
}

func mkdirWithSourceAccess(root *fileRoot, name string, mode os.FileMode, source *os.File) error {
	if source != nil {
		mode = 0700
	}
	return root.Mkdir(name, mode)
}
