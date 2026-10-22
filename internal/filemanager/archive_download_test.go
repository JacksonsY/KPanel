package filemanager

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestExportArchiveMembersFormats(t *testing.T) {
	for _, format := range []string{"zip", "tar", "tar.gz"} {
		t.Run(format, func(t *testing.T) {
			m, root := newTestManager(t)
			source := makeArchiveFixture(t, m, root, format)
			before, _ := os.ReadDir(root)
			var raw bytes.Buffer
			if err := m.ExportArchiveMembers(context.Background(), source.Path, source.ResourceVersion, []string{"assets/app.js"}, true, &raw); err != nil || raw.String() != "script" {
				t.Fatalf("raw=%q err=%v", raw.String(), err)
			}
			var output bytes.Buffer
			if err := m.ExportArchiveMembers(context.Background(), source.Path, source.ResourceVersion, []string{"assets", "assets/app.js", "index.html"}, false, &output); err != nil {
				t.Fatal(err)
			}
			reader, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{}
			for _, entry := range reader.File {
				if entry.FileInfo().IsDir() {
					continue
				}
				if _, duplicate := files[entry.Name]; duplicate {
					t.Fatalf("duplicate %s", entry.Name)
				}
				input, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				content, err := io.ReadAll(input)
				input.Close()
				if err != nil {
					t.Fatal(err)
				}
				files[entry.Name] = string(content)
			}
			if len(files) != 2 || files["assets/app.js"] != "script" || files["index.html"] != "hello" {
				t.Fatalf("files=%#v", files)
			}
			after, _ := os.ReadDir(root)
			if len(before) != len(after) {
				t.Fatal("download created filesystem entries")
			}
		})
	}
}

func TestExportArchiveMembersPreflight(t *testing.T) {
	m, root := newTestManager(t)
	source := makeArchiveFixture(t, m, root, "zip")
	for _, test := range []struct {
		name    string
		members []string
		raw     bool
		version string
		want    error
	}{
		{"missing", []string{"missing"}, false, source.ResourceVersion, ErrConflict},
		{"raw directory", []string{"assets"}, true, source.ResourceVersion, ErrInvalidArchive},
		{"traversal", []string{"../index.html"}, true, source.ResourceVersion, ErrInvalidArchive},
		{"absolute", []string{"/index.html"}, true, source.ResourceVersion, ErrInvalidArchive},
		{"raw batch", []string{"assets/app.js", "index.html"}, true, source.ResourceVersion, ErrInvalidArchive},
		{"empty", nil, false, source.ResourceVersion, ErrInvalidArchive},
		{"stale", []string{"index.html"}, true, "stale", ErrConflict},
		{"no version", []string{"index.html"}, true, "", ErrInvalidArchive},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := m.ExportArchiveMembers(context.Background(), source.Path, test.version, test.members, test.raw, &output)
			if !errors.Is(err, test.want) || output.Len() != 0 {
				t.Fatalf("err=%v output=%d", err, output.Len())
			}
		})
	}
	m.maxCopyBytes = 5
	var output bytes.Buffer
	if err := m.ExportArchiveMembers(context.Background(), source.Path, source.ResourceVersion, []string{"index.html"}, true, &output); !errors.Is(err, ErrTooLarge) || output.Len() != 0 {
		t.Fatalf("budget err=%v bytes=%d", err, output.Len())
	}
}

func TestExportArchiveMembersRejectsUnsafeLateEntriesAndEmptyFiles(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		m, root := newTestManager(t)
		var source bytes.Buffer
		writer := zip.NewWriter(&source)
		if _, err := writer.Create("empty.txt"); err != nil {
			t.Fatal(err)
		}
		if unsafe {
			header := &zip.FileHeader{Name: "late-link"}
			header.SetMode(os.ModeSymlink | 0777)
			if _, err := writer.CreateHeader(header); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "empty.zip"), source.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		entry, err := m.Stat("/empty.zip")
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		err = m.ExportArchiveMembers(context.Background(), entry.Path, entry.ResourceVersion, []string{"empty.txt"}, true, &output)
		if unsafe && !errors.Is(err, ErrInvalidArchive) || !unsafe && err != nil || output.Len() != 0 {
			t.Fatalf("unsafe=%v err=%v bytes=%d", unsafe, err, output.Len())
		}
	}
}
