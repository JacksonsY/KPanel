package agent

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	previewDiskBytes     = 256 << 20
	previewDiskEntries   = 512
	previewDiskItemBytes = 2 << 20
	previewDiskTTL       = 30 * 24 * time.Hour
	previewDiskHeader    = 69 // magic, content type, source-key hash, body checksum
)

type previewDiskEntry struct {
	name string
	size int64
	used time.Time
}
type previewDisk struct {
	mu      sync.Mutex
	root    *os.Root
	lock    *os.File
	entries map[string]previewDiskEntry
	bytes   int64
	limit   int64
	count   int
	now     func() time.Time
}

// A disposable derivative store, never a source of file authorization or metadata.
// Root-relative operations cannot follow a cache symlink outside this directory.
func newPreviewDisk(stateDir string) *previewDisk {
	if !filepath.IsAbs(stateDir) {
		return nil
	}
	state, err := os.OpenRoot(stateDir)
	if err != nil {
		return nil
	}
	defer state.Close()
	const directory = "gallery-previews-v1"
	if err = state.Mkdir(directory, 0700); err != nil && !os.IsExist(err) {
		return nil
	}
	info, err := state.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return nil
	}
	root, err := state.OpenRoot(directory)
	if err != nil {
		return nil
	}
	lock, err := root.OpenFile("lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		root.Close()
		return nil
	}
	lockInfo, err := root.Lstat("lock")
	if err != nil || !lockInfo.Mode().IsRegular() || lockInfo.Mode().Perm()&0077 != 0 {
		lock.Close()
		root.Close()
		return nil
	}
	if err = lockPreviewDisk(lock); err != nil {
		lock.Close()
		root.Close()
		return nil
	}
	d := &previewDisk{root: root, lock: lock, entries: make(map[string]previewDiskEntry), limit: previewDiskBytes, count: previewDiskEntries, now: time.Now}
	// Bounded recovery: unexpected directory growth disables persistence instead
	// of allocating an unbounded index or deleting unrelated names.
	f, err := root.Open(".")
	if err != nil {
		d.close()
		return nil
	}
	items, err := f.ReadDir(previewDiskEntries*2 + 1)
	f.Close()
	if (err != nil && err != io.EOF) || len(items) > previewDiskEntries*2 {
		d.close()
		return nil
	}
	for _, item := range items {
		name := item.Name()
		if strings.HasPrefix(name, "tmp-") && validPreviewDiskName(strings.TrimPrefix(name, "tmp-")) {
			_ = root.Remove(name)
			continue
		}
		if !validPreviewDiskName(name) {
			continue
		}
		info, err := root.Lstat(name)
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() <= previewDiskHeader || info.Size() > previewDiskItemBytes+previewDiskHeader || d.now().Sub(info.ModTime()) > previewDiskTTL {
			_ = root.Remove(name)
			continue
		}
		d.entries[name] = previewDiskEntry{name: name, size: info.Size(), used: info.ModTime()}
		d.bytes += info.Size()
	}
	d.prune()
	if d.bytes > d.limit || len(d.entries) > d.count {
		d.close()
		return nil
	}
	return d
}

func validPreviewDiskName(name string) bool {
	if len(name) != 64+len(".preview") || !strings.HasSuffix(name, ".preview") {
		return false
	}
	_, err := hex.DecodeString(name[:64])
	return err == nil
}

func previewDiskKey(key string) [32]byte {
	// Bump this schema when generation dimensions, encoding or orientation rules change.
	return sha256.Sum256([]byte("screen-v1-1440-static-orientation\x00" + key))
}

func (d *previewDisk) get(key string) ([]byte, string, bool) {
	if d == nil {
		return nil, "", false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.root == nil {
		return nil, "", false
	}
	hash := previewDiskKey(key)
	name := hex.EncodeToString(hash[:]) + ".preview"
	entry, ok := d.entries[name]
	if !ok {
		return nil, "", false
	}
	info, err := d.root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() != entry.size || d.now().Sub(info.ModTime()) > previewDiskTTL {
		d.remove(name)
		return nil, "", false
	}
	f, err := d.root.Open(name)
	if err != nil {
		d.remove(name)
		return nil, "", false
	}
	data, err := io.ReadAll(io.LimitReader(f, previewDiskItemBytes+previewDiskHeader+1))
	f.Close()
	if err != nil || len(data) <= previewDiskHeader || len(data) > previewDiskItemBytes+previewDiskHeader || string(data[:4]) != "KPV1" || !bytes.Equal(data[5:37], hash[:]) {
		d.remove(name)
		return nil, "", false
	}
	body := data[previewDiskHeader:]
	checksum := sha256.Sum256(body)
	mime := "image/jpeg"
	if data[4] == 2 {
		mime = "image/png"
	} else if data[4] != 1 {
		d.remove(name)
		return nil, "", false
	}
	if !bytes.Equal(data[37:69], checksum[:]) {
		d.remove(name)
		return nil, "", false
	}
	entry.used = d.now()
	d.entries[name] = entry
	// Persist approximate LRU at most hourly, avoiding a metadata write per view.
	if entry.used.Sub(info.ModTime()) >= time.Hour {
		_ = d.root.Chtimes(name, entry.used, entry.used)
	}
	return body, mime, true
}

func (d *previewDisk) put(key string, body []byte, mime string) {
	if d == nil || len(body) == 0 || len(body) > previewDiskItemBytes || (mime != "image/jpeg" && mime != "image/png") {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.root == nil {
		return
	}
	hash := previewDiskKey(key)
	name := hex.EncodeToString(hash[:]) + ".preview"
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return
	}
	temp := "tmp-" + hex.EncodeToString(random[:]) + ".preview"
	f, err := d.root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer d.root.Remove(temp)
	header := make([]byte, previewDiskHeader)
	copy(header, "KPV1")
	header[4] = 1
	if mime == "image/png" {
		header[4] = 2
	}
	copy(header[5:37], hash[:])
	checksum := sha256.Sum256(body)
	copy(header[37:], checksum[:])
	_, err = f.Write(header)
	if err == nil {
		_, err = f.Write(body)
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return
	}
	// Remove quota victims before publication; temporary writes add at most one item.
	entry := previewDiskEntry{name: name, size: int64(len(body) + previewDiskHeader), used: d.now()}
	d.prune()
	old, exists := d.entries[name]
	additional := 1
	if exists {
		additional = 0
	}
	for d.bytes-old.size+entry.size > d.limit || len(d.entries)+additional > d.count {
		var victim previewDiskEntry
		for _, candidate := range d.entries {
			if candidate.name != name && (victim.name == "" || candidate.used.Before(victim.used)) {
				victim = candidate
			}
		}
		if victim.name == "" {
			return
		}
		d.remove(victim.name)
		// Failed unlink remains accounted for, and prevents further publication.
		if _, remains := d.entries[victim.name]; remains {
			return
		}
	}
	if err = d.root.Rename(temp, name); err != nil {
		return
	}
	d.entries[name] = entry
	d.bytes += entry.size - old.size
}

func (d *previewDisk) prune() {
	entries := make([]previewDiskEntry, 0, len(d.entries))
	for _, e := range d.entries {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].used.Before(entries[j].used) })
	for _, e := range entries {
		if d.bytes <= d.limit && len(d.entries) <= d.count && d.now().Sub(e.used) <= previewDiskTTL {
			break
		}
		d.remove(e.name)
	}
}
func (d *previewDisk) remove(name string) {
	if err := d.root.Remove(name); err != nil && !os.IsNotExist(err) {
		return
	}
	if entry, ok := d.entries[name]; ok {
		d.bytes -= entry.size
		delete(d.entries, name)
	}
}
func (d *previewDisk) close() {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.root != nil {
		d.root.Close()
		d.root = nil
	}
	if d.lock != nil {
		d.lock.Close()
		d.lock = nil
	}
}
