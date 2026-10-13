package agent

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
	"github.com/kejilion/kejilion-panel/internal/filemanager"
)

func testPreviewDisk(t *testing.T, dir string) *previewDisk {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("private Unix directory permissions are required for persistent previews")
	}
	d := newPreviewDisk(dir)
	if d == nil {
		t.Fatal("cache unavailable")
	}
	t.Cleanup(d.close)
	return d
}

func TestPreviewDiskRecoveryIntegrityQuotaAndTTL(t *testing.T) {
	dir := t.TempDir()
	d := testPreviewDisk(t, dir)
	if duplicate := newPreviewDisk(dir); duplicate != nil {
		duplicate.close()
		t.Fatal("two writers acquired one cache directory")
	}
	d.count = 2
	d.limit = 2 * (previewDiskHeader + 3)
	clock := time.Now()
	d.now = func() time.Time { return clock }
	d.put("a", []byte("aaa"), "image/jpeg")
	clock = clock.Add(time.Second)
	d.put("b", []byte("bbb"), "image/png")
	clock = clock.Add(time.Second)
	if _, _, ok := d.get("a"); !ok {
		t.Fatal("a missing")
	}
	clock = clock.Add(time.Second)
	d.put("c", []byte("ccc"), "image/jpeg")
	if _, _, ok := d.get("b"); ok {
		t.Fatal("LRU victim kept")
	}
	if d.bytes > d.limit || len(d.entries) > d.count {
		t.Fatal("quota exceeded")
	}
	d.close()
	recovered := testPreviewDisk(t, dir)
	if got, mime, ok := recovered.get("a"); !ok || string(got) != "aaa" || mime != "image/jpeg" {
		t.Fatal("restart miss")
	}
	if _, _, ok := recovered.get("a-new-version"); ok {
		t.Fatal("version collision")
	}
	hash := previewDiskKey("a")
	name := hex.EncodeToString(hash[:]) + ".preview"
	data, err := os.ReadFile(filepath.Join(dir, "gallery-previews-v1", name))
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 1
	if err = os.WriteFile(filepath.Join(dir, "gallery-previews-v1", name), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := recovered.get("a"); ok {
		t.Fatal("corrupt body accepted")
	}
	recovered.now = func() time.Time { return time.Now().Add(previewDiskTTL + time.Hour) }
	if _, _, ok := recovered.get("c"); ok {
		t.Fatal("expired entry kept")
	}
}

func TestPreviewDiskPrivatePermissionsAndBounds(t *testing.T) {
	dir := t.TempDir()
	d := testPreviewDisk(t, dir)
	d.put("/private/source.jpg\x00version", []byte("data"), "image/jpeg")
	d.put("oversized", make([]byte, previewDiskItemBytes+1), "image/png")
	if len(d.entries) != 1 {
		t.Fatal("entry size bound ignored")
	}
	for name := range d.entries {
		if !validPreviewDiskName(name) || strings.Contains(name, "private") {
			t.Fatal("source path exposed")
		}
		info, err := os.Stat(filepath.Join(dir, "gallery-previews-v1", name))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("cache file permissions")
		}
		if err = os.Chmod(filepath.Join(dir, "gallery-previews-v1", name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, ok := d.get("/private/source.jpg\x00version"); ok {
		t.Fatal("loosened permissions accepted")
	}
	// A directory with unexpectedly many objects must fail closed with bounded
	// enumeration, leaving foreign names untouched.
	d.close()
	cacheDir := filepath.Join(dir, "gallery-previews-v1")
	for i := range previewDiskEntries*2 + 1 {
		if err := os.WriteFile(filepath.Join(cacheDir, fmt.Sprint("foreign-", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if grown := newPreviewDisk(dir); grown != nil {
		grown.close()
		t.Fatal("unbounded directory recovered")
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "foreign-0")); err != nil {
		t.Fatal("foreign object removed")
	}
}

func TestPreviewDiskRejectsSymlinksAndRecoversInterruptedWrites(t *testing.T) {
	dir := t.TempDir()
	d := testPreviewDisk(t, dir)
	d.put("a", []byte("payload"), "image/jpeg")
	d.close()
	root := filepath.Join(dir, "gallery-previews-v1")
	orphan := "tmp-" + strings.Repeat("b", 64) + ".preview"
	if err := os.WriteFile(filepath.Join(root, orphan), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	hash := previewDiskKey("a")
	name := hex.EncodeToString(hash[:]) + ".preview"
	if err := os.Remove(filepath.Join(root, name)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, name)); err != nil {
		t.Fatal(err)
	}
	recovered := testPreviewDisk(t, dir)
	if _, _, ok := recovered.get("a"); ok {
		t.Fatal("symlink accepted")
	}
	if body, _ := os.ReadFile(outside); string(body) != "private" {
		t.Fatal("outside modified")
	}
	if _, err := os.Lstat(filepath.Join(root, orphan)); !os.IsNotExist(err) {
		t.Fatal("orphan retained")
	}
	other := t.TempDir()
	if err := os.Symlink(root, filepath.Join(other, "gallery-previews-v1")); err != nil {
		t.Fatal(err)
	}
	if cache := newPreviewDisk(other); cache != nil {
		cache.close()
		t.Fatal("symlink directory accepted")
	}
	// An unavailable store must remain optional; publication failure cannot panic.
	recovered.root.Close()
	recovered.put("new", []byte("data"), "image/png")
	if _, _, ok := recovered.get("new"); ok {
		t.Fatal("failed write indexed")
	}
}

func previewFixture(t *testing.T, count int) (*Server, []contract.FileEntry, string) {
	t.Helper()
	root := t.TempDir()
	s := testServer(t)
	s.files = writeTestPhoto(t, root)
	picture := image.NewNRGBA(image.Rect(0, 0, 512, 384))
	state := uint32(1)
	for i := 0; i < len(picture.Pix); i += 4 {
		state = state*1664525 + 1013904223
		picture.Pix[i] = byte(state >> 24)
		state = state*1664525 + 1013904223
		picture.Pix[i+1] = byte(state >> 24)
		state = state*1664525 + 1013904223
		picture.Pix[i+2] = byte(state >> 24)
		picture.Pix[i+3] = 255
	}
	var data bytes.Buffer
	if err := jpeg.Encode(&data, picture, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	// Keep queue/race tests focused on scheduling, not expensive resampling.
	// Legal trailing JPEG padding exercises the real >256KiB warmup condition.
	if data.Len() < 300<<10 {
		data.Write(make([]byte, (300<<10)-data.Len()))
	}
	entries := make([]contract.FileEntry, 0, count)
	for i := range count {
		name := string(rune('a'+i)) + ".jpg"
		if err := os.WriteFile(filepath.Join(root, name), data.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		entry, err := s.files.Stat("/" + name)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	return s, entries, root
}

func awaitPreview(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("preview did not finish")
	}
}

func TestPreviewQueueDeduplicatesPrioritizesAndCancels(t *testing.T) {
	s, entries, _ := previewFixture(t, 18)
	q := s.previewJobs
	s.previewGate = make(chan struct{})
	first, err := q.submit(entries[0], false)
	if err != nil {
		t.Fatal(err)
	}
	// Wait until the first job is active; queued work owns no download slots.
	deadline := time.Now().Add(time.Second)
	for {
		q.mu.Lock()
		active := len(q.queued) == 0
		q.mu.Unlock()
		if active {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker not active")
		}
		time.Sleep(time.Millisecond)
	}
	var warmups []*previewJob
	for i := 1; i < previewQueueLimit; i++ {
		job, submitErr := q.submit(entries[i], false)
		if submitErr != nil {
			t.Fatal(err)
		}
		warmups = append(warmups, job)
	}
	same, err := q.submit(entries[0], true)
	if err != nil || same != first {
		t.Fatal("active request not coalesced")
	}
	foreground, err := q.submit(entries[16], true)
	if err != nil {
		t.Fatal("foreground did not displace warmup")
	}
	if _, err = q.submit(entries[17], false); err != errPreviewQueueFull {
		t.Fatal("unbounded warmup queue")
	}
	q.mu.Lock()
	if len(q.jobs) != previewQueueLimit {
		t.Fatal("queue bound")
	}
	q.mu.Unlock()
	// Keep the real decode gate closed after one job, so priority is observable.
	step := func() {
		t.Helper()
		select {
		case <-s.previewGate:
		case <-time.After(5 * time.Second):
			t.Fatal("worker did not start decode")
		}
		select {
		case s.previewGate <- struct{}{}:
		case <-time.After(5 * time.Second):
			t.Fatal("worker did not finish decode")
		}
	}
	step()
	awaitPreview(t, first.done)
	step()
	awaitPreview(t, foreground.done)
	if foreground.err != nil {
		t.Fatal(foreground.err)
	}
	for _, job := range warmups[:len(warmups)-1] {
		select {
		case <-job.done:
			t.Fatal("warmup completed before foreground")
		default:
		}
	}
	// Active optional work is blocked on a cancellable decode gate; shutdown
	// must release its source descriptor and all queued waiters.
	q.close()
	q.mu.Lock()
	if len(q.queued) != 0 {
		t.Fatal("shutdown retained queued work")
	}
	q.mu.Unlock()
	if _, err = q.submit(entries[17], true); err != context.Canceled {
		t.Fatal("work accepted after shutdown")
	}
}

func TestPreviewQueueWarmupAndVersionChange(t *testing.T) {
	s, entries, root := previewFixture(t, 1)
	entry := entries[0]
	thumb := "/v1/files/content?path=" + url.QueryEscape(entry.Path) + "&mode=thumbnail&version=" + url.QueryEscape(entry.ResourceVersion)
	if result := fileRequest(s, http.MethodGet, thumb, ""); result.Code != 200 {
		t.Fatal(result.Body.String())
	}
	s.previewJobs.mu.Lock()
	job := s.previewJobs.jobs[screenPreviewKey(entry)]
	s.previewJobs.mu.Unlock()
	if job != nil {
		awaitPreview(t, job.done)
	}
	if _, _, ok := s.previews.get(screenPreviewKey(entry)); !ok {
		t.Fatal("thumbnail did not warm preview")
	}
	s.previews = newPreviewCache()
	s.previewGate <- struct{}{}
	job, err := s.previewJobs.submit(entry, true)
	if err != nil {
		t.Fatal(err)
	}
	// Replace the original while queued work waits for decode, preserving a valid
	// image body but changing the source's resource version.
	if err = os.Chtimes(filepath.Join(root, "a.jpg"), time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	<-s.previewGate
	awaitPreview(t, job.done)
	if !errors.Is(job.err, filemanager.ErrConflict) {
		t.Fatalf("replaced source cached: %v", job.err)
	}
}

func TestPreviewQueueForegroundWakesIdleWarmup(t *testing.T) {
	s, entries, _ := previewFixture(t, 1)
	q := s.previewJobs
	q.mu.Lock()
	q.quietUntil = time.Now().Add(time.Hour)
	q.mu.Unlock()
	job, err := q.submit(entries[0], false)
	if err != nil {
		t.Fatal(err)
	}
	// Let the optional worker enter its idle select, then promote that exact job.
	// Completion is asserted, rather than a machine-specific decode duration.
	time.Sleep(20 * time.Millisecond)
	promoted, err := q.submit(entries[0], true)
	if err != nil || promoted != job {
		t.Fatal("warmup not promoted")
	}
	awaitPreview(t, job.done)
	if job.err != nil {
		t.Fatal(job.err)
	}
}

func TestPreviewPersistentHandlerRestartRevalidatesSource(t *testing.T) {
	s, entries, root := previewFixture(t, 1)
	stateDir := t.TempDir()
	probe := testPreviewDisk(t, stateDir)
	probe.close()
	entry := entries[0]
	path := "/v1/files/content?path=" + url.QueryEscape(entry.Path) + "&mode=preview&version=" + url.QueryEscape(entry.ResourceVersion)
	request := func(handler http.Handler) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
		return r
	}
	handler, stop := NewCachedFileHandler(s.files, stateDir)
	first := request(handler)
	stop()
	if first.Code != 200 {
		t.Fatal(first.Body.String())
	}
	// Recreate both the handler and RAM cache. A disk hit is proven by an occupied
	// decode gate in the equivalent Server path, not merely by elapsed time.
	other := testServer(t)
	other.files = s.files
	other.previewJobs.close()
	other.previewJobs = newPreviewQueue(other, stateDir)
	other.previewGate <- struct{}{}
	defer func() { <-other.previewGate }()
	otherHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { other.fileRead(w, r, "preview-test", false) })
	restarted := request(otherHandler)
	if restarted.Code != 200 || !bytes.Equal(first.Body.Bytes(), restarted.Body.Bytes()) {
		t.Fatal("restart did not reuse disk")
	}
	if err := os.Chtimes(filepath.Join(root, "a.jpg"), time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if changed := request(otherHandler); changed.Code != 409 {
		t.Fatalf("stale cache: %d", changed.Code)
	}
	if err := os.Remove(filepath.Join(root, "a.jpg")); err != nil {
		t.Fatal(err)
	}
	if deleted := request(otherHandler); deleted.Code != 404 {
		t.Fatalf("deleted file served from cache: %d", deleted.Code)
	}
}
