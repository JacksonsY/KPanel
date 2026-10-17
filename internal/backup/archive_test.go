package backup

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBackupArchiveAuthenticatedRoundTrip(t *testing.T) {
	source := filepath.Join(t.TempDir(), "panel.json")
	payload := bytes.Repeat([]byte("private API configuration\n"), 10000)
	if err := os.WriteFile(source, payload, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	want, err := Write(&out, "long-backup-password", "test", []Source{{"panel", source}})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte("private API")) {
		t.Fatal("plaintext leaked")
	}
	dir := t.TempDir()
	got, err := Read(bytes.NewReader(out.Bytes()), "long-backup-password", dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Parts[0] != want.Parts[0] {
		t.Fatal("manifest changed")
	}
	data, err := os.ReadFile(filepath.Join(dir, "panel.payload"))
	if err != nil || !bytes.Equal(data, payload) {
		t.Fatal("payload changed", err)
	}
	cases := map[string][]byte{"truncated": out.Bytes()[:out.Len()-1], "appended": append(append([]byte{}, out.Bytes()...), 1)}
	changed := append([]byte{}, out.Bytes()...)
	changed[len(changed)/2] ^= 1
	cases["tampered"] = changed
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Read(bytes.NewReader(data), "long-backup-password", t.TempDir()); err == nil {
				t.Fatal("accepted damaged archive")
			}
		})
	}
	if _, err := Read(bytes.NewReader(out.Bytes()), "wrong-password", t.TempDir()); err == nil {
		t.Fatal("accepted wrong password")
	}
}

// A PAX sparse entry stores a few bytes but reads back at its declared size,
// so an import must refuse it before writing anything.
func TestBackupArchiveRejectsSparsePayload(t *testing.T) {
	const size = 1 << 20
	digest := sha256.Sum256(make([]byte, size))
	manifest, err := json.Marshal(Manifest{Format: FormatVersion, Version: "test", CreatedAt: time.Now().UTC(), Parts: []Part{{Module: "apps", Size: size, SHA256: hex.EncodeToString(digest[:])}}})
	if err != nil {
		t.Fatal(err)
	}
	var plain bytes.Buffer
	tw := tar.NewWriter(&plain)
	if err := tw.WriteHeader(&tar.Header{Name: "manifest.json", Typeflag: tar.TypeReg, Mode: 0600, Size: int64(len(manifest))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(manifest); err != nil {
		t.Fatal(err)
	}
	if err := tw.Flush(); err != nil {
		t.Fatal(err)
	}
	plain.Write(paxSparseEntry("apps.payload", size))
	plain.Write(make([]byte, 1024))
	var sealed bytes.Buffer
	if err := encrypt(&sealed, bytes.NewReader(plain.Bytes()), "long-backup-password"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := Read(bytes.NewReader(sealed.Bytes()), "long-backup-password", dir); err == nil {
		t.Fatal("accepted a sparse payload")
	}
	if _, err := os.Lstat(filepath.Join(dir, "apps.payload")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("sparse payload was written", err)
	}
}

// paxSparseEntry returns a raw PAX 1.0 sparse entry without data segments.
// archive/tar's writer drops GNU.sparse records, so the blocks are built here.
func paxSparseEntry(name string, realSize int64) []byte {
	record := func(key, value string) string {
		line := " " + key + "=" + value + "\n"
		size := len(line)
		for size != len(strconv.Itoa(size))+len(line) {
			size = len(strconv.Itoa(size)) + len(line)
		}
		return strconv.Itoa(size) + line
	}
	padded := func(data []byte) []byte {
		return append(data, make([]byte, (512-len(data)%512)%512)...)
	}
	block := func(name string, typeflag byte, size int) []byte {
		b := make([]byte, 512)
		copy(b, name)
		copy(b[100:], "0000600\x00")
		copy(b[108:], "0000000\x00")
		copy(b[116:], "0000000\x00")
		copy(b[124:], fmt.Sprintf("%011o\x00", size))
		copy(b[136:], "00000000000\x00")
		b[156] = typeflag
		copy(b[257:], "ustar\x0000")
		copy(b[148:], "        ")
		sum := 0
		for _, c := range b {
			sum += int(c)
		}
		copy(b[148:], fmt.Sprintf("%06o\x00 ", sum))
		return b
	}
	pax := record("GNU.sparse.major", "1") + record("GNU.sparse.minor", "0") +
		record("GNU.sparse.name", name) + record("GNU.sparse.realsize", strconv.FormatInt(realSize, 10))
	sparseMap := padded([]byte("0\n"))
	var out bytes.Buffer
	out.Write(block("PaxHeaders/"+name, 'x', len(pax)))
	out.Write(padded([]byte(pax)))
	out.Write(block(name, '0', len(sparseMap)))
	out.Write(sparseMap)
	return out.Bytes()
}

func TestBackupArchiveSelectionAndFileBoundary(t *testing.T) {
	for _, selection := range [][]string{nil, {"panel", "panel"}, {"../../etc"}} {
		if _, err := Selection(selection); err == nil {
			t.Fatal(selection)
		}
	}
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	_ = os.WriteFile(outside, []byte("secret"), 0600)
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err == nil {
		if _, err := OpenRegular(filepath.Join(dir, "link"), 100); err == nil {
			t.Fatal("followed symbolic link")
		}
	}
}

func TestBackupJobRecoveryAndSingleWriter(t *testing.T) {
	dir := t.TempDir()
	m, err := OpenManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	record, err := m.Reserve("import", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Reserve("export", []string{"panel"}); !errors.Is(err, ErrBusy) {
		t.Fatal("allowed parallel job")
	}
	if err := m.ExpireQueued(time.Now().Add(3 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if m.Busy() {
		t.Fatal("upload lease never released")
	}
	r, _ := m.Get(record.ID)
	if r.ErrorCode != "upload_expired" {
		t.Fatal(r)
	}
	active, err := m.Reserve("export", []string{"panel"})
	if err != nil {
		t.Fatal(err)
	}
	wait := make(chan struct{})
	if err := m.Run(active.ID, func(ctx context.Context, id string) error { <-wait; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := m.Run(active.ID, func(context.Context, string) error { return nil }); !errors.Is(err, ErrBusy) {
		t.Fatal("job ran twice")
	}
	close(wait)
	m.Close()
	reopened, err := OpenManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	finished, _ := reopened.Get(active.ID)
	if finished.Status != "completed" {
		t.Fatal(finished)
	}
}

func TestBackupLegacyExportRemainsDownloadableDuringUploadRetry(t *testing.T) {
	m, err := OpenManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	r, err := m.Reserve("export", []string{"panel"})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(m.Root, r.ID, "backup.kpb"), []byte("legacy encrypted fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = m.Update(r.ID, func(r *Record) { r.Status = "completed"; r.Size = 24 }); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.active = ""
	m.mu.Unlock()
	r, err = m.ReserveUpload(r.ID)
	if err != nil || !r.LocalReady {
		t.Fatal(r, err)
	}
	if err = m.Abort(r.ID, "remote_upload_failed"); err != nil {
		t.Fatal(err)
	}
	r, _ = m.Get(r.ID)
	if !r.LocalReady {
		t.Fatal("failed retry hid local package")
	}
}

func TestOpenManagerAcceptsRecordsBeyondTheLegacy32KiBCap(t *testing.T) {
	// A legal large Docker import can approach 3x4096 root refs, which the
	// legacy 32 KiB record budget rejected after the next restart. Build a
	// record past that cap and confirm the manager still opens.
	dir := t.TempDir()
	id := strings.Repeat("c", 32)
	jobDir := filepath.Join(dir, id)
	if err := os.MkdirAll(jobDir, 0o700); err != nil {
		t.Fatal(err)
	}
	roots := make([]RootRef, 700)
	for index := range roots {
		roots[index] = RootRef{Path: fmt.Sprintf("/var/lib/docker/volumes/expected-%06d", index), Module: "docker"}
	}
	record := Record{ID: id, Action: "import", Status: "completed", Modules: []string{"docker"}, Roots: roots, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := WriteJSON(filepath.Join(jobDir, "record.json"), record); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(jobDir, "record.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(body) <= 32<<10 {
		t.Fatalf("fixture must exceed the legacy 32 KiB cap, got %d bytes", len(body))
	}
	manager, err := OpenManager(dir)
	if err != nil {
		t.Fatalf("manager rejected a record the write side can legally produce: %v", err)
	}
	defer manager.Close()
	got, err := manager.Get(id)
	if err != nil {
		t.Fatalf("record beyond the legacy cap was dropped: %v", err)
	}
	if len(got.Roots) != len(roots) {
		t.Fatalf("root inventory lost after reopen: got %d want %d", len(got.Roots), len(roots))
	}
}

func TestManagerRejectsRecordBeyondItsReopenBudget(t *testing.T) {
	dir := t.TempDir()
	manager, err := OpenManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	record, err := manager.Reserve("import", []string{"apps"})
	if err != nil {
		manager.Close()
		t.Fatal(err)
	}
	err = manager.Update(record.ID, func(current *Record) {
		current.Roots = []RootRef{{Path: "/home/" + strings.Repeat("a", maxRecordBytes), Module: "apps"}}
	})
	if !errors.Is(err, ErrInvalid) {
		manager.Close()
		t.Fatalf("oversized record error = %v, want ErrInvalid", err)
	}
	got, err := manager.Get(record.ID)
	if err != nil || len(got.Roots) != 0 {
		manager.Close()
		t.Fatalf("oversized update changed in-memory record: roots=%d err=%v", len(got.Roots), err)
	}
	manager.Close()
	reopened, err := OpenManager(dir)
	if err != nil {
		t.Fatalf("reopen after rejected update: %v", err)
	}
	reopened.Close()
}
