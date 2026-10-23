package filemanager

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveBudgetCountsImplicitParents(t *testing.T) {
	for _, format := range []string{"zip", "tar"} {
		for _, valid := range []bool{false, true} {
			t.Run(format+map[bool]string{false: "-overflow", true: "-shared-parents"}[valid], func(t *testing.T) {
				m, root := newTestManager(t)
				mustMkdirAll(t, filepath.Join(root, "out"))
				names := []string{"a/b/c/d/file"}
				if valid {
					names = []string{"a/b/file", "a/", "a/b/"}
				}
				var data bytes.Buffer
				budget := &copyBudget{maxEntries: 3, maxBytes: 1024}
				var times []archiveDirectoryTime
				var err error
				if format == "zip" {
					writer := zip.NewWriter(&data)
					for _, name := range names {
						if _, err := writer.Create(name); err != nil {
							t.Fatal(err)
						}
					}
					if err := writer.Close(); err != nil {
						t.Fatal(err)
					}
					reader, readErr := zip.NewReader(bytes.NewReader(data.Bytes()), int64(data.Len()))
					if readErr != nil {
						t.Fatal(readErr)
					}
					err = m.extractZIP(context.Background(), reader, "/out", budget, map[string]struct{}{}, &times)
				} else {
					writer := tar.NewWriter(&data)
					for _, name := range names {
						kind := byte(tar.TypeReg)
						if strings.HasSuffix(name, "/") {
							kind = tar.TypeDir
						}
						if err := writer.WriteHeader(&tar.Header{Name: name, Typeflag: kind, Mode: 0755}); err != nil {
							t.Fatal(err)
						}
					}
					if err := writer.Close(); err != nil {
						t.Fatal(err)
					}
					err = m.extractTAR(context.Background(), tar.NewReader(&data), "/out", budget, map[string]struct{}{}, &times)
				}
				if valid && err != nil {
					t.Fatal(err)
				}
				if !valid && !errors.Is(err, ErrTooLarge) {
					t.Fatalf("implicit parents bypassed budget: %v", err)
				}
				objects := 0
				if err := filepath.WalkDir(filepath.Join(root, "out"), func(name string, _ os.DirEntry, err error) error {
					if name != filepath.Join(root, "out") {
						objects++
					}
					return err
				}); err != nil {
					t.Fatal(err)
				}
				if objects > 3 || valid && objects != 3 {
					t.Fatalf("expanded objects=%d", objects)
				}
			})
		}
	}
}

func TestArchiveBatchCountsEqualMembersInSeparateOutputs(t *testing.T) {
	m, root := newTestManager(t)
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	if _, err := writer.Create("a/b/file"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bundle.zip"), data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	budget := &copyBudget{maxEntries: 5, maxBytes: 1024}
	ctx := context.WithValue(context.Background(), archiveContextKey{}, &archiveOperation{budget: budget})
	if _, err := m.extractArchive(ctx, "/bundle.zip", "/", "one", archiveFormatZIP, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := m.extractArchive(ctx, "/bundle.zip", "/", "two", archiveFormatZIP, ""); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("second extraction reused first output's object allowance: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "two")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("over-budget output was published: %v", err)
	}
	stages, err := filepath.Glob(filepath.Join(root, ".kpanel-extract-*"))
	if err != nil || len(stages) != 0 {
		t.Fatalf("failed extraction left staging objects: %v %v", stages, err)
	}
}
