//go:build !linux

package filemanager

import "os"

type fileRoot = os.Root

func openFileRoot(root string) (*fileRoot, error) { return os.OpenRoot(root) }

func rawFileRoot(root *fileRoot) *os.Root  { return root }
func wrapFileRoot(root *os.Root) *fileRoot { return root }
