package agent

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

func TestThumbnailDiskRestartAndFileRevalidation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("persistent cache locking requires Linux")
	}
	root, state := t.TempDir(), t.TempDir()
	manager := writeTestPhoto(t, root)
	path := thumbnailPath(t, manager)
	request := func(handler http.Handler) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
		return r
	}
	handler, stop := NewCachedFileHandler(manager, state)
	first := request(handler)
	stop()
	if first.Code != 200 {
		t.Fatalf("first: %d %s", first.Code, first.Body.String())
	}
	s := testServer(t)
	s.files = manager
	s.thumbnailDisk.close()
	s.thumbnailDisk = newThumbnailDisk(state)
	t.Cleanup(s.thumbnailDisk.close)
	if s.thumbnailDisk == nil {
		t.Fatal("disk cache unavailable")
	}
	// An occupied decoder gate proves that restart reuse did not regenerate.
	s.thumbnailGate <- struct{}{}
	defer func() { <-s.thumbnailGate }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r := httptest.NewRecorder()
	s.fileRead(r, httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx), "thumbnail-restart", false)
	if r.Code != 200 || !bytes.Equal(r.Body.Bytes(), first.Body.Bytes()) {
		t.Fatalf("restart: %d %s", r.Code, r.Body.String())
	}
	if err := os.Chtimes(filepath.Join(root, "photo.png"), time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if changed := request(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { s.fileRead(w, r, "version", false) })); changed.Code != 409 {
		t.Fatalf("changed file served: %d", changed.Code)
	}
	if err := os.Remove(filepath.Join(root, "photo.png")); err != nil {
		t.Fatal(err)
	}
	if deleted := request(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { s.fileRead(w, r, "deleted", false) })); deleted.Code != 404 {
		t.Fatalf("deleted file served: %d", deleted.Code)
	}
}

func TestThumbnailDiskIntegrityQuotaAndNamespace(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("persistent cache locking requires Linux")
	}
	state := t.TempDir()
	d := newThumbnailDisk(state)
	if d == nil {
		t.Fatal("disk unavailable")
	}
	defer d.close()
	d.count = 2
	d.limit = 2 * (previewDiskHeader + 3)
	d.put("a", []byte("aaa"), "image/jpeg")
	d.put("b", []byte("bbb"), "image/png")
	d.put("c", []byte("ccc"), "image/jpeg")
	if len(d.entries) != 2 || d.bytes > d.limit {
		t.Fatal("quota exceeded")
	}
	if _, _, ok := d.get("a"); ok {
		t.Fatal("old entry retained")
	}
	if previewDiskKey("c") == d.key("c") {
		t.Fatal("derivative specifications collide")
	}
	for name := range d.entries {
		if err := os.WriteFile(filepath.Join(state, "gallery-thumbnails-v1", name), []byte("broken"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, ok := d.get("c"); ok {
		t.Fatal("corruption served")
	}
	d.close()
	if err := os.RemoveAll(filepath.Join(state, "gallery-thumbnails-v1")); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(state, "gallery-thumbnails-v1")); err != nil {
		t.Fatal(err)
	}
	if unsafe := newThumbnailDisk(state); unsafe != nil {
		unsafe.close()
		t.Fatal("symlink cache accepted")
	}
}

type thumbnailCountedReader struct {
	io.ReadSeekCloser
	reads *atomic.Int64
}

func (r thumbnailCountedReader) Read(b []byte) (int, error) {
	r.reads.Add(1)
	return r.ReadSeekCloser.Read(b)
}

func TestThumbnailConcurrentMissesReuseGeneration(t *testing.T) {
	manager := writeTestPhoto(t, t.TempDir())
	s := testServer(t)
	s.files = manager
	entry, err := manager.Stat("/photo.png")
	if err != nil {
		t.Fatal(err)
	}
	var reads [2]atomic.Int64
	var readers [2]io.ReadSeekCloser
	for i := range readers {
		file, _, err := manager.Open(context.Background(), entry.Path)
		if err != nil {
			t.Fatal(err)
		}
		readers[i] = thumbnailCountedReader{file, &reads[i]}
		defer file.Close()
	}
	s.thumbnailGate <- struct{}{}
	var wg sync.WaitGroup
	wg.Add(2)
	var failures atomic.Int64
	for i := range readers {
		go func(i int, entry contract.FileEntry) {
			defer wg.Done()
			if _, _, err := s.loadThumbnail(context.Background(), readers[i], entry); err != nil {
				failures.Add(1)
			}
		}(i, entry)
	}
	<-s.thumbnailGate
	wg.Wait()
	if failures.Load() != 0 || (reads[0].Load() > 0) == (reads[1].Load() > 0) {
		t.Fatalf("expected exactly one source decode: reads=%d/%d errors=%d", reads[0].Load(), reads[1].Load(), failures.Load())
	}
}
