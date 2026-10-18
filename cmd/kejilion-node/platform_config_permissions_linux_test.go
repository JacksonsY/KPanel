//go:build linux

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePlatformConfigAccess(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "node.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePlatformConfig(file, false); err != nil {
		file.Close()
		t.Fatalf("private node config rejected: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	file, err = os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePlatformConfig(file, false); err != nil {
		file.Close()
		t.Fatalf("service-readable node config rejected: %v", err)
	}
	if err := validatePlatformConfig(file, true); err == nil {
		file.Close()
		t.Fatal("group-readable secret config accepted")
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	file, err = os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePlatformConfig(file, false); err == nil {
		file.Close()
		t.Fatal("world-readable node config accepted")
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePlatformConfigAccessRejectsForeignOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to change the test file owner")
	}
	path := filepath.Join(t.TempDir(), "node.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(path, 65534, 65534); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := validatePlatformConfig(file, false); err == nil {
		t.Fatal("foreign-owned node config accepted")
	}
}
