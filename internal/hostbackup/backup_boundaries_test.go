package hostbackup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

func TestMountPathsMustMatchDockersCanonicalPaths(t *testing.T) {
	for _, invalid := range []string{"/home/app/../docker/kpanel", "/data/../host", "/data/", "//data", "/data\x00hidden", "/data\\hidden", "relative"} {
		for _, field := range []string{"source", "destination", "structured-source", "structured-target", "tmpfs-target"} {
			t.Run(field+"/"+invalid, func(t *testing.T) {
				e, _, p := hostFixture(t)
				c := &p.Containers[0]
				source, target := "/home/app", "/data"
				if strings.Contains(field, "source") {
					source = invalid
				} else {
					target = invalid
				}
				c.Mounts[0].Source, c.Mounts[0].Destination = source, target
				if strings.HasPrefix(field, "structured") || field == "tmpfs-target" {
					c.HostConfig["Binds"] = rawJSON([]string{})
					kind := "bind"
					if field == "tmpfs-target" {
						kind = "tmpfs"
						c.Mounts[0].Type = kind
					}
					c.HostConfig["Mounts"] = rawJSON([]map[string]any{{"Type": kind, "Source": source, "Target": target}})
				} else {
					c.HostConfig["Binds"] = rawJSON([]string{source + ":" + target + ":rw"})
				}
				if _, err := nativeMountConfig(*c); err == nil {
					t.Fatal("noncanonical native mount accepted")
				}
				if err := e.validatePayload(p); err == nil {
					t.Fatal("noncanonical payload accepted")
				}
			})
		}
	}
	e, _, p := hostFixture(t)
	p = hostReachingPayload(p)
	if err := e.validatePayload(p); err != nil {
		t.Fatal("canonical advanced administrator config was blocked:", err)
	}
}

func TestExportPinsDirectoriesAcrossSymlinkReplacement(t *testing.T) {
	for _, stage := range []string{"before-open", "directory-opened"} {
		t.Run(stage, func(t *testing.T) {
			e, _, p := hostFixture(t)
			dir := e.host("/home/app/nested")
			outside := t.TempDir()
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "content"), []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(outside, "content"), []byte("outside-sensitive-marker"), 0600); err != nil {
				t.Fatal(err)
			}
			called := false
			e.exportHook = func(at, relative string) {
				if at != stage || relative != "nested" {
					return
				}
				called = true
				if err := os.Rename(dir, dir+"-moved"); err != nil {
					t.Fatal(err)
				}
				requireParentSymlink(t, outside, dir)
			}
			path := filepath.Join(t.TempDir(), "apps.payload")
			err := e.writePayload(context.Background(), path, p)
			if !called {
				t.Fatal("race seam was not exercised")
			}
			if stage == "before-open" {
				if err == nil {
					t.Fatal("directory replaced between lstat/open was accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			entries := exportedEntries(t, path)
			if got := string(entries["data/"+p.Roots[0].ID+"/nested/content"]); got != "original" {
				t.Fatalf("unpinned content: %q", got)
			}
			for _, body := range entries {
				if bytes.Contains(body, []byte("outside-sensitive-marker")) {
					t.Fatal("outside tree exported")
				}
			}
		})
	}
}

func exportedEntries(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	out := map[string][]byte{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		out[h.Name] = data
	}
	return out
}

func TestExportRegularRootAndFileIdentity(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "regular-root", true: "swapped-file"}[replace], func(t *testing.T) {
			e, _, p := hostFixture(t)
			p.Roots[0].Path = "/home/app/data"
			p.Roots[0].Directory = false
			if replace {
				e.exportHook = func(stage, relative string) {
					if stage == "before-open" && relative == "." {
						if err := os.Rename(e.host(p.Roots[0].Path), e.host(p.Roots[0].Path)+"-old"); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(e.host(p.Roots[0].Path), []byte("evil"), 0640); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			path := filepath.Join(t.TempDir(), "file.payload")
			err := e.writePayload(context.Background(), path, p)
			if replace {
				if err == nil {
					t.Fatal("same-size replacement was accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := string(exportedEntries(t, path)["data/"+p.Roots[0].ID]); got != "old!" {
				t.Fatal(got)
			}
		})
	}
}

func TestExportHonorsCancellation(t *testing.T) {
	e, _, p := hostFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.exportHook = func(stage, relative string) {
		if stage == "directory-opened" {
			cancel()
		}
	}
	if err := e.writePayload(ctx, filepath.Join(t.TempDir(), "apps.payload"), p); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestExportDirectoryBatchesAndCumulativeBudgets(t *testing.T) {
	e, _, p := hostFixture(t)
	for n := 0; n < 260; n++ {
		if err := os.WriteFile(e.host(fmt.Sprintf("/home/app/item-%03d", n)), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "batch.payload")
	if err := e.writePayload(context.Background(), path, p); err != nil {
		t.Fatal(err)
	}
	if len(exportedEntries(t, path)) != 263 {
		t.Fatal("directory batches omitted entries")
	}
	for _, limit := range []string{"entries", "bytes"} {
		tw := tar.NewWriter(io.Discard)
		w := exportWalker{ctx: context.Background(), writer: tw}
		if limit == "entries" {
			w.count = 100000
		} else {
			w.bytes = backup.MaxBytes
		}
		if err := w.writeRoot(e.host(p.Roots[0].Path), p.Roots[0]); err == nil {
			t.Fatal("cumulative " + limit + " budget was ignored")
		}
		_ = tw.Close()
	}
}

func TestExportRejectsFileChangedAfterOpening(t *testing.T) {
	e, _, p := hostFixture(t)
	called := false
	e.exportHook = func(stage, relative string) {
		if stage != "file-opened" || relative != "data" {
			return
		}
		called = true
		if err := os.WriteFile(e.host("/home/app/data"), []byte("changed-content"), 0640); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.writePayload(context.Background(), filepath.Join(t.TempDir(), "changed.payload"), p); err == nil {
		t.Fatal("concurrent file mutation accepted")
	}
	if !called {
		t.Fatal("after-open race not exercised")
	}
}

func TestReviewRedactionDoesNotChangeExecutionPayload(t *testing.T) {
	e, d, p := volumePayload(t)
	const marker = "review-secret-marker-8231"
	p.Containers[0].HostConfig["LogConfig"] = rawJSON(map[string]any{"Type": "fluentd", "Config": map[string]string{"token": marker, "endpoint": "https://logger.invalid/?api_key=" + marker}})
	p.Networks["lan"] = rawJSON(map[string]any{"Driver": "bridge", "Options": map[string]string{"access_token": marker}, "Labels": map[string]string{"secret": marker}})
	p.Containers[0].Networks["lan"] = rawJSON(map[string]any{})
	v := p.Volumes["planned-data"]
	v.Labels["password"] = marker
	p.Volumes[v.Name] = v
	networkCreated := false
	e.Client.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == "GET" && r.URL.Path == "/networks/lan" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(p.Networks["lan"])), Header: make(http.Header)}, nil
		}
		if r.Method == "GET" && r.URL.Path == "/networks" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[]`)), Header: make(http.Header)}, nil
		}
		if r.Method == "POST" && r.URL.Path == "/networks/create" {
			var body json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				return nil, err
			}
			if !bytes.Contains(body, []byte(marker)) {
				t.Fatal("network executed redacted config")
			}
			networkCreated = true
			return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
		}
		return d.request(r)
	})
	dir := t.TempDir()
	if err := otherHost(t, e).writePayload(context.Background(), filepath.Join(dir, "apps.payload"), p); err != nil {
		t.Fatal(err)
	}
	read, facts, err := e.readPayload(context.Background(), filepath.Join(dir, "apps.payload"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	review := ImportReview{Modules: []ModuleReview{moduleReview(read, facts, []string{"/home/app"})}}
	if body := rawJSON(review); bytes.Contains(body, []byte(marker)) {
		t.Fatal("display projection leaked a labelled secret")
	}
	if !bytes.Contains(rawJSON(read), []byte(marker)) || !bytes.Contains(rawJSON(p), []byte(marker)) {
		t.Fatal("projection mutated the payload")
	}
	if err := saveReview(dir, review); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(dir, reviewFile)); err != nil || bytes.Contains(body, []byte(marker)) {
		t.Fatalf("persistent review leaked: %s %v", body, err)
	}
	if err := e.Restore(context.Background(), backup.NewID(), dir, []string{"apps"}, true); err != nil {
		t.Fatal(err, restoreFailureCause(err))
	}
	for _, c := range d.containers {
		if !bytes.Contains(c.HostConfig["LogConfig"], []byte(marker)) {
			t.Fatal("restore executed redacted HostConfig")
		}
	}
	if !networkCreated || d.volumes[v.Name].Labels["password"] != marker {
		t.Fatal("network/volume execution lost original configuration")
	}
}

func TestLoadReviewScrubsLegacyPersistentCopies(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(string(rune('0'+version)), func(t *testing.T) {
			dir := t.TempDir()
			const marker = "legacy-review-secret-9165"
			r := ImportReview{Version: version, Modules: []ModuleReview{{Module: "apps", Origin: OriginForeign, Containers: []ContainerReview{{Name: "app", HostConfig: rawJSON(map[string]any{"LogConfig": map[string]any{"Config": map[string]string{"token": marker}}})}}, Networks: []NetworkReview{{Name: "lan", Options: []string{"password=" + marker}, Config: rawJSON(map[string]string{"token": marker})}}, Volumes: []VolumeReview{{Name: "data", Config: rawJSON(map[string]string{"api_key": marker})}}}}}
			if err := os.WriteFile(filepath.Join(dir, reviewFile), rawJSON(r), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := loadReview(dir, []string{"apps"})
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(rawJSON(got), []byte(marker)) {
				t.Fatal("old review response leaked secret")
			}
			body, err := os.ReadFile(filepath.Join(dir, reviewFile))
			if err != nil || bytes.Contains(body, []byte(marker)) {
				t.Fatal("old review disk copy was not scrubbed", err)
			}
			if version == 1 && got.Complete([]string{"apps"}) {
				t.Fatal("scrubbing upgraded an incomplete review")
			}
		})
	}
}
