//go:build linux

package appmarket

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureAppJobDirectoryRejectsGroupWritableDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs")
	if err := os.Mkdir(path, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o770); err != nil {
		t.Fatal(err)
	}
	if err := ensureAppJobDirectory(path); err == nil {
		t.Fatal("group-writable application job directory was accepted")
	}
}

func TestEnsureAppJobDirectoryRejectsForeignOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing directory ownership requires root")
	}
	path := filepath.Join(t.TempDir(), "jobs")
	if err := os.Mkdir(path, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(path, 65534, 65534); err != nil {
		t.Fatal(err)
	}
	if err := ensureAppJobDirectory(path); err == nil {
		t.Fatal("foreign-owned application job directory was accepted")
	}
}
