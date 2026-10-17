//go:build !linux

package hostbackup

import "os"

// Non-Linux hosts exercise the archive protocol with isolated fixtures only.
func noNestedMounts(string) error { return nil }

func restoreNoNestedMounts(*os.Root, string) error { return nil }
