//go:build linux

package systeminfo

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var storageRoots = map[string]bool{
	"/": true, "/home": true, "/home/docker": true, "/home/web": true,
	"/opt": true, "/root": true, "/srv": true, "/tmp": true,
	"/usr": true, "/var": true, "/var/lib/docker": true, "/var/log": true,
}

type inodeIdentity struct{ device, inode uint64 }

const maxStorageRootEntries = 4096
const storageReadBatch = 128
const maxStorageDepth = 64

func (c *Collector) collectStorageUsage(ctx context.Context, requested string) (StorageUsage, error) {
	root := filepath.Clean(requested)
	if !filepath.IsAbs(root) || !storageRoots[root] {
		return StorageUsage{}, errors.New("storage path is not an allowed analysis root")
	}
	return collectStorageRoot(ctx, root, c.Now().UTC())
}

func collectStorageRoot(ctx context.Context, root string, collectedAt time.Time) (StorageUsage, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() {
		return StorageUsage{}, errors.New("storage analysis root is unavailable")
	}
	rootStat, ok := rootInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return StorageUsage{}, errors.New("storage metadata is unavailable")
	}
	entries, truncated, err := readStorageRootEntries(ctx, root)
	if err != nil {
		return StorageUsage{}, err
	}
	result := StorageUsage{Path: root, Items: make([]StorageUsageItem, 0, len(entries)), CollectedAt: collectedAt, Truncated: truncated}
	jobs := make(chan fs.DirEntry)
	items := make(chan StorageUsageItem, len(entries))
	workers := min(4, max(1, runtime.NumCPU()), len(entries))
	perChildLimit := max(2_000, min(25_000, maxStorageEntries/max(1, len(entries))))
	var scanned atomic.Int64
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for entry := range jobs {
				items <- scanStorageChild(ctx, root, entry, uint64(rootStat.Dev), perChildLimit, &scanned)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, entry := range entries {
			child := filepath.Join(root, entry.Name())
			if root == "/" && (child == "/proc" || child == "/sys" || child == "/dev" || child == "/run") {
				continue
			}
			jobs <- entry
		}
	}()
	group.Wait()
	close(items)
	for item := range items {
		result.Items = append(result.Items, item)
		result.Scanned += item.Scanned
		result.Truncated = result.Truncated || item.Truncated
	}
	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].SizeBytes != result.Items[j].SizeBytes {
			return result.Items[i].SizeBytes > result.Items[j].SizeBytes
		}
		return result.Items[i].Path < result.Items[j].Path
	})
	if len(result.Items) > maxStorageItems {
		result.Items = result.Items[:maxStorageItems]
		result.Truncated = true
	}
	return result, nil
}

// Read only a bounded prefix before allocating worker queues and result storage.
func readStorageRootEntries(ctx context.Context, root string) ([]fs.DirEntry, bool, error) {
	directory, err := os.Open(root)
	if err != nil {
		return nil, false, err
	}
	defer directory.Close()
	entries := make([]fs.DirEntry, 0, maxStorageRootEntries)
	for {
		if ctx.Err() != nil {
			return entries, true, nil
		}
		batch, readErr := directory.ReadDir(min(storageReadBatch, maxStorageRootEntries+1-len(entries)))
		entries = append(entries, batch...)
		if len(entries) > maxStorageRootEntries {
			return entries[:maxStorageRootEntries], true, nil
		}
		if errors.Is(readErr, io.EOF) {
			return entries, false, nil
		}
		if readErr != nil {
			return nil, false, readErr
		}
	}
}

func scanStorageChild(ctx context.Context, root string, entry fs.DirEntry, rootDevice uint64, entryLimit int, scanned *atomic.Int64) StorageUsageItem {
	child := filepath.Join(root, entry.Name())
	item := StorageUsageItem{Path: child, Kind: "file"}
	if entry.IsDir() {
		item.Kind = "directory"
	}
	seen := make(map[inodeIdentity]struct{}, min(entryLimit, 4096))
	var walk func(string, fs.DirEntry, int)
	walk = func(current string, value fs.DirEntry, depth int) {
		if ctx.Err() != nil || item.Scanned >= entryLimit || scanned.Load() >= maxStorageEntries {
			item.Truncated = true
			return
		}
		// Even unreadable or cross-device entries consume the traversal budget.
		if scanned.Add(1) > maxStorageEntries {
			scanned.Add(-1)
			item.Truncated = true
			return
		}
		item.Scanned++
		info, infoErr := value.Info()
		if infoErr != nil {
			item.Truncated = true
			return
		}
		stat, statOK := info.Sys().(*syscall.Stat_t)
		if !statOK || uint64(stat.Dev) != rootDevice {
			return
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return
		}
		identity := inodeIdentity{device: uint64(stat.Dev), inode: stat.Ino}
		if _, duplicate := seen[identity]; duplicate {
			return
		}
		seen[identity] = struct{}{}
		if stat.Blocks > 0 {
			item.SizeBytes += uint64(stat.Blocks) * 512
		}
		if !info.IsDir() {
			return
		}
		if depth >= maxStorageDepth {
			item.Truncated = true
			return
		}
		// WalkDir reads and sorts a whole directory before the next budget
		// check. Stream bounded batches instead, including below the root.
		if ctx.Err() != nil || item.Scanned >= entryLimit || scanned.Load() >= maxStorageEntries {
			item.Truncated = true
			return
		}
		fd, openErr := syscall.Open(current, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if openErr != nil {
			item.Truncated = true
			return
		}
		directory := os.NewFile(uintptr(fd), current)
		defer directory.Close()
		opened, statErr := directory.Stat()
		if statErr != nil || !os.SameFile(info, opened) {
			item.Truncated = true
			return
		}
		for {
			if ctx.Err() != nil || item.Scanned >= entryLimit || scanned.Load() >= maxStorageEntries {
				item.Truncated = true
				return
			}
			batch, readErr := directory.ReadDir(min(storageReadBatch, entryLimit-item.Scanned))
			for _, next := range batch {
				walk(filepath.Join(current, next.Name()), next, depth+1)
				if ctx.Err() != nil || item.Scanned >= entryLimit || scanned.Load() >= maxStorageEntries {
					item.Truncated = true
					return
				}
			}
			if errors.Is(readErr, io.EOF) {
				return
			}
			if readErr != nil {
				item.Truncated = true
				return
			}
		}
	}
	walk(child, entry, 0)
	if ctx.Err() != nil {
		item.Truncated = true
	}
	return item
}
