package scenepacks

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func uploadFixture(t *testing.T) map[string][]byte {
	t.Helper()
	manifest, err := os.ReadFile("../../share-themes/minimal/dist/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var pack Pack
	if json.Unmarshal(manifest, &pack) != nil {
		t.Fatal("manifest")
	}
	pack.ID = "my-theme"
	manifest, _ = json.Marshal(pack)
	return map[string][]byte{"manifest.json": manifest, "index.html": []byte("<script src=theme.js></script>"), "theme.js": []byte("parent.postMessage({source:'kpanel-share-theme',type:'ready'},'*')")}
}

func zipFixture(t *testing.T, bodies map[string][]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for name, body := range bodies {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func offlineThemes(t *testing.T, root string) *Store {
	t.Helper()
	s := OpenShareThemes(root, func(context.Context, string, int64) ([]byte, error) { return nil, ErrSource })
	t.Cleanup(s.Close)
	return s
}

func TestUploadThemeOfflineLifecycle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	s := offlineThemes(t, root)
	list, err := s.List(ctx)
	if err != nil || list.Warning == "" || list.ResourceVersion == "" {
		t.Fatal("offline upload unavailable", err)
	}
	data := zipFixture(t, uploadFixture(t))
	view, err := s.UploadTheme(ctx, data, list.ResourceVersion)
	if err != nil || !view.Uploaded || !view.Installed || s.ActiveTheme() != nil {
		t.Fatal("upload", view, err)
	}
	if _, err := s.UploadTheme(ctx, data, list.ResourceVersion); !errors.Is(err, ErrConflict) {
		t.Fatal("stale write", err)
	}
	list, _ = s.List(ctx)
	if _, err := s.UploadTheme(ctx, data, list.ResourceVersion); !errors.Is(err, ErrConflict) {
		t.Fatal("overwrite", err)
	}
	if err := s.Select(view.ID, list.ResourceVersion); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = offlineThemes(t, root)
	if s.ActiveTheme() == nil {
		t.Fatal("selection not restored")
	}
	// A later catalog collision must not relabel a local upload or offer updates.
	s.catalog.Packs = []Pack{{ID: view.ID, Version: "99.0.0"}}
	list, _ = s.List(ctx)
	if len(list.Packs) != 1 || !list.Packs[0].Uploaded || list.Packs[0].Version != view.Version {
		t.Fatal("local identity lost", list)
	}
	body, _, err := s.File(view.ID, s.state.Installed[view.ID].Token, "index.html")
	if err != nil || !bytes.Equal(body, uploadFixture(t)["index.html"]) {
		t.Fatal("file", err)
	}
	token := s.state.Installed[view.ID].Token
	if err := s.Delete(view.ID, list.Packs[0].ResourceVersion); err != nil {
		t.Fatal(err)
	}
	if s.ActiveTheme() != nil {
		t.Fatal("delete did not restore default")
	}
	if _, _, err := s.File(view.ID, token, "index.html"); !errors.Is(err, ErrNotFound) {
		t.Fatal("capability not revoked")
	}
}

func TestUploadThemeRejectsUnsafeArchives(t *testing.T) {
	for _, name := range []string{"../escape.js", "/escape.js", "a\\escape.js", "a/../../escape.js", "script.svg", "a/b/c/d/e.js", "INDEX.html"} {
		t.Run(name, func(t *testing.T) {
			files := uploadFixture(t)
			files[name] = []byte("x")
			if _, _, err := decodeThemeZIP(zipFixture(t, files)); !errors.Is(err, ErrInvalid) {
				t.Fatal("unsafe path accepted", err)
			}
		})
	}
	for _, test := range []string{"missing-entry", "bad-manifest", "large-entry", "expanded-quota", "too-many", "wrong-runtime"} {
		t.Run(test, func(t *testing.T) {
			files := uploadFixture(t)
			switch test {
			case "missing-entry":
				delete(files, "index.html")
			case "bad-manifest":
				files["manifest.json"] = []byte("{}")
			case "large-entry":
				files["index.html"] = bytes.Repeat([]byte("a"), 64<<10+1)
			case "expanded-quota":
				files["large.js"] = bytes.Repeat([]byte("a"), 5<<20)
			case "too-many":
				for i := 0; i < 40; i++ {
					files["f"+strings.Repeat("a", i)+".js"] = []byte("x")
				}
			case "wrong-runtime":
				files["manifest.json"] = bytes.ReplaceAll(files["manifest.json"], []byte("kpanel-share-theme@2"), []byte("kpanel-scene-pack@1"))
			}
			if _, _, err := decodeThemeZIP(zipFixture(t, files)); !errors.Is(err, ErrInvalid) {
				t.Fatal("invalid archive accepted", err)
			}
		})
	}
	for _, mode := range []os.FileMode{0, os.ModeSymlink | 0777} {
		var out bytes.Buffer
		w := zip.NewWriter(&out)
		for range 2 {
			header := &zip.FileHeader{Name: "index.html"}
			header.SetMode(mode)
			f, _ := w.CreateHeader(header)
			_, _ = f.Write([]byte("x"))
		}
		_ = w.Close()
		if _, _, err := decodeThemeZIP(out.Bytes()); !errors.Is(err, ErrInvalid) {
			t.Fatal("duplicate/symlink accepted")
		}
	}
}

func TestUploadThemeFailureRollbackAndQuota(t *testing.T) {
	s := offlineThemes(t, t.TempDir())
	list, _ := s.List(context.Background())
	data := zipFixture(t, uploadFixture(t))
	s.writeState = func(string, []byte) error { return errors.New("disk full") }
	if _, err := s.UploadTheme(context.Background(), data, list.ResourceVersion); !errors.Is(err, ErrUnavailable) {
		t.Fatal("write failure", err)
	}
	if len(s.state.Installed) != 0 {
		t.Fatal("failed upload committed")
	}
	entries, err := os.ReadDir(filepath.Join(s.root, "objects"))
	if err != nil || len(entries) != 0 {
		t.Fatal("orphan objects", err)
	}
	for i := 0; i < MaxInstalled; i++ {
		s.state.Installed[strings.Repeat("a", i+1)] = installedPack{}
	}
	if _, err := s.UploadTheme(context.Background(), data, s.stateVersion()); !errors.Is(err, ErrQuota) {
		t.Fatal("quota", err)
	}
}
