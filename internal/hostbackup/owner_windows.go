//go:build windows

package hostbackup

import "os"

func restoreOwnership(path string, uid, gid int) error                          { return nil }
func restoreSymlinkOwnership(path string, uid, gid int) error                   { return nil }
func copyFileOwnership(f *os.File, info os.FileInfo) error                      { return nil }
func copyOwnershipIn(root *os.Root, name string, info os.FileInfo) error        { return nil }
func copySymlinkOwnershipIn(root *os.Root, name string, info os.FileInfo) error { return nil }
func ownedByProcess(info os.FileInfo) bool                                      { return true }
