//go:build linux

package systeminfo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestCollectStorageRootRanksImmediateChildren(t *testing.T) {
	root := t.TempDir()
	large := filepath.Join(root, "large")
	small := filepath.Join(root, "small")
	if err := os.MkdirAll(large, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(small, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(large, "payload"), make([]byte, 64<<10), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(small, "payload"), make([]byte, 4<<10), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := collectStorageRoot(context.Background(), root, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 || result.Items[0].Path != large || result.Items[0].SizeBytes <= result.Items[1].SizeBytes || result.Scanned < 4 {
		t.Fatalf("storage usage=%#v", result)
	}
}

func TestStorageRootEnumerationIsBounded(t *testing.T) {
	root := t.TempDir()
	for index := range 4097 {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprint(index)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := collectStorageRoot(context.Background(), root, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated || result.Scanned > 4096 || len(result.Items) > maxStorageItems {
		t.Fatalf("wide root exceeded bounded prefix: %#v", result)
	}
}

func TestStorageChildBudgetPreventsWideDirectoryAllocation(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := range 3000 {
		if err := os.WriteFile(filepath.Join(child, fmt.Sprint(index)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	allocations := testing.AllocsPerRun(1, func() {
		var scanned atomic.Int64
		item := scanStorageChild(context.Background(), root, entries[0], uint64(info.Sys().(*syscall.Stat_t).Dev), 1, &scanned)
		if !item.Truncated || item.Scanned != 1 {
			t.Fatalf("budget result=%#v", item)
		}
	})
	t.Logf("one-entry scan allocations with 3000 children: %.0f", allocations)
	if allocations > 512 {
		t.Fatalf("exhausted scan still allocated for directory contents: %.0f", allocations)
	}
}

func TestStorageRootCancelledBeforeEnumeration(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "data"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := collectStorageRoot(ctx, root, time.Now())
	if err != nil || !result.Truncated || result.Scanned != 0 || len(result.Items) != 0 {
		t.Fatalf("cancelled result=%#v err=%v", result, err)
	}
}

func TestStorageUsageRejectsArbitraryRoot(t *testing.T) {
	collector := NewCollector()
	if _, err := collector.StorageUsage(context.Background(), t.TempDir()); err == nil {
		t.Fatal("arbitrary storage root was accepted")
	}
}

func TestScanStorageChildHonorsGlobalEntryLimit(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "payload"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	rootStat := rootInfo.Sys().(*syscall.Stat_t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var scanned atomic.Int64
	scanned.Store(maxStorageEntries)
	item := scanStorageChild(context.Background(), root, entries[0], uint64(rootStat.Dev), 2_000, &scanned)
	if !item.Truncated || item.Scanned != 0 || scanned.Load() != maxStorageEntries {
		t.Fatalf("global limit result=%#v scanned=%d", item, scanned.Load())
	}
}
