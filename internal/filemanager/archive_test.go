package filemanager

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

func TestCompressAndExtractSupportedFormats(t *testing.T) {
	for _, format := range []struct {
		name     string
		fileName string
	}{
		{name: archiveFormatZIP, fileName: "website.zip"},
		{name: archiveFormatTAR, fileName: "website.tar"},
		{name: archiveFormatTARGZ, fileName: "website.tar.gz"},
	} {
		t.Run(format.name, func(t *testing.T) {
			manager, root := newTestManager(t)
			mustMkdirAll(t, filepath.Join(root, "website", "assets"))
			mustWrite(t, filepath.Join(root, "website", "index.html"), "hello")
			mustWrite(t, filepath.Join(root, "website", "assets", "app.js"), "script")
			modified := time.Date(2026, 7, 1, 12, 30, 20, 0, time.UTC)
			if err := os.Chtimes(filepath.Join(root, "website", "index.html"), modified, modified); err != nil {
				t.Fatal(err)
			}

			compressed, err := manager.Action(context.Background(), contract.FileActionRequest{
				Action:  "compress",
				Sources: []string{"/website"},
				Target:  "/",
				Name:    format.fileName,
				Format:  format.name,
			})
			if err != nil || len(compressed.Succeeded) != 1 {
				t.Fatalf("compress result = %#v err=%v", compressed, err)
			}
			if _, err := os.Stat(filepath.Join(root, format.fileName)); err != nil {
				t.Fatal(err)
			}

			extracted, err := manager.Action(context.Background(), contract.FileActionRequest{
				Action:                  "extract",
				Sources:                 []string{"/" + format.fileName},
				Target:                  "/",
				Name:                    "restored-" + strings.ReplaceAll(format.name, ".", "-"),
				Format:                  format.name,
				ExpectedResourceVersion: compressed.Succeeded[0].ResourceVersion,
			})
			if err != nil || len(extracted.Succeeded) != 1 {
				t.Fatalf("extract result = %#v err=%v", extracted, err)
			}
			restoredRoot := filepath.Join(root, "restored-"+strings.ReplaceAll(format.name, ".", "-"))
			content, err := os.ReadFile(filepath.Join(restoredRoot, "index.html"))
			if err != nil || string(content) != "hello" {
				t.Fatalf("restored index = %q err=%v", content, err)
			}
			if _, err := os.Stat(filepath.Join(restoredRoot, "website", "index.html")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("single selected directory must not be nested twice: %v", err)
			}
			restoredInfo, err := os.Stat(filepath.Join(restoredRoot, "index.html"))
			if err != nil {
				t.Fatal(err)
			}
			if restoredInfo.ModTime().Sub(modified).Abs() > 2*time.Second {
				t.Fatalf("restored modification time = %v", restoredInfo.ModTime())
			}
		})
	}
}

func TestExtractKeepsWritesInsideOpenedStageAfterSiblingReplacement(t *testing.T) {
	for _, replaceWithLink := range []bool{true, false} {
		t.Run(strconv.FormatBool(replaceWithLink), func(t *testing.T) {
			manager, root := newTestManager(t)
			mustMkdirAll(t, filepath.Join(root, "source"))
			mustMkdirAll(t, filepath.Join(root, "target"))
			mustMkdirAll(t, filepath.Join(root, "sensitive"))
			mustWrite(t, filepath.Join(root, "source", "first.txt"), "first")
			mustWrite(t, filepath.Join(root, "source", "second.txt"), "second")
			mustWrite(t, filepath.Join(root, "sensitive", "sentinel"), "keep")
			mustWrite(t, filepath.Join(root, "sensitive", "first.txt"), "existing")
			_, err := manager.Action(context.Background(), contract.FileActionRequest{
				Action: "compress", Sources: []string{"/source"}, Target: "/", Name: "bundle.zip", Format: archiveFormatZIP,
			})
			if err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(root, "target", ".kpanel-extract-swap-test")
			moved := filepath.Join(root, "target", "moved-stage")
			swapped := false
			operation := &archiveOperation{tempSuffix: "swap-test", progress: func(budget *copyBudget) {
				if swapped || budget.bytes == 0 {
					return
				}
				if err := os.Rename(stage, moved); err != nil {
					if runtime.GOOS == "windows" {
						t.Skipf("platform cannot move an opened staging directory: %v", err)
					}
					t.Fatal(err)
				}
				swapped = true
				if replaceWithLink {
					if err := os.Symlink("../sensitive", stage); err != nil {
						t.Fatal(err)
					}
				} else {
					mustWrite(t, filepath.Join(stage, "sentinel"), "replacement")
				}
			}}
			ctx := context.WithValue(context.Background(), archiveContextKey{}, operation)
			_, err = manager.extractArchive(ctx, "/bundle.zip", "/target", "result", archiveFormatZIP, "")
			if err == nil || !swapped {
				t.Fatalf("replaced stage must fail, swapped=%v err=%v", swapped, err)
			}
			for _, candidate := range []string{filepath.Join(root, "sensitive", "second.txt"), filepath.Join(stage, "second.txt"), filepath.Join(root, "target", "result")} {
				if _, err := os.Lstat(candidate); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("replacement received or published extraction output %s: %v", candidate, err)
				}
			}
			if !replaceWithLink {
				content, err := os.ReadFile(filepath.Join(stage, "sentinel"))
				if err != nil || string(content) != "replacement" {
					t.Fatalf("cleanup changed the replacement directory: %q %v", content, err)
				}
			}
		})
	}
}

func TestCompressRejectsNestedDirectorySubstitution(t *testing.T) {
	manager, root := newTestManager(t)
	mustWrite(t, filepath.Join(root, "source", "outer", "nested", "public.txt"), "public")
	mustWrite(t, filepath.Join(root, "sensitive", "secret.txt"), "secret")
	swapped := false
	operation := &archiveOperation{progress: func(budget *copyBudget) {
		if swapped || budget.entries != 3 {
			return
		}
		original := filepath.Join(root, "source", "outer", "nested")
		if err := os.Rename(original, filepath.Join(root, "moved-source")); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("platform cannot move opened directories: %v", err)
			}
			t.Fatal(err)
		}
		if err := os.Symlink("../../sensitive", original); err != nil {
			t.Fatal(err)
		}
		swapped = true
	}}
	ctx := context.WithValue(context.Background(), archiveContextKey{}, operation)
	_, err := manager.compressArchive(ctx, []string{"/source"}, "/", "result.zip", archiveFormatZIP, nil)
	if err == nil || !swapped {
		t.Fatalf("substituted source must be rejected, swapped=%v err=%v", swapped, err)
	}
	if _, err := os.Stat(filepath.Join(root, "result.zip")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("substituted source was published: %v", err)
	}
}

func TestArchiveTraversalKeepsOpenedParentForLaterFiles(t *testing.T) {
	manager, root := newTestManager(t)
	for _, directory := range []string{"source/outer/nested", "sensitive"} {
		mustWrite(t, filepath.Join(root, directory, "first.txt"), "first")
		mustWrite(t, filepath.Join(root, directory, "second.txt"), directory)
	}
	swapped := false
	walks := 0
	operation := &archiveOperation{progress: func(budget *copyBudget) {
		if budget.entries == 1 {
			walks++
		}
		if swapped || walks != 2 || budget.entries != 4 {
			return
		}
		original := filepath.Join(root, "source", "outer", "nested")
		if err := os.Rename(original, filepath.Join(root, "moved-source")); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("platform cannot move opened directories: %v", err)
			}
			t.Fatal(err)
		}
		if err := os.Symlink("../../sensitive", original); err != nil {
			t.Fatal(err)
		}
		swapped = true
	}}
	ctx := context.WithValue(context.Background(), archiveContextKey{}, operation)
	var output bytes.Buffer
	if err := manager.ExportZIP(ctx, []string{"/source"}, nil, &output); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range reader.File {
		if !strings.HasSuffix(entry.Name, "/second.txt") {
			continue
		}
		input, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(input)
		_ = input.Close()
		if err != nil || string(content) != "source/outer/nested" {
			t.Fatalf("archive read from substituted parent: %q %v", content, err)
		}
		found = true
	}
	if !found || !swapped {
		t.Fatalf("regression scenario incomplete: file=%v swap=%v", found, swapped)
	}
}

func TestCompressMultipleEntriesKeepsTheirBaseNames(t *testing.T) {
	manager, root := newTestManager(t)
	mustWrite(t, filepath.Join(root, "one.txt"), "one")
	mustWrite(t, filepath.Join(root, "folder", "two.txt"), "two")

	_, err := manager.Action(context.Background(), contract.FileActionRequest{
		Action: "compress", Sources: []string{"/one.txt", "/folder"},
		Target: "/", Name: "bundle.zip", Format: archiveFormatZIP,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Action(context.Background(), contract.FileActionRequest{
		Action: "extract", Sources: []string{"/bundle.zip"},
		Target: "/", Name: "bundle", Format: archiveFormatZIP,
	})
	if err != nil {
		t.Fatal(err)
	}
	for relative, expected := range map[string]string{
		"one.txt": "one", "folder/two.txt": "two",
	} {
		content, readErr := os.ReadFile(filepath.Join(root, "bundle", filepath.FromSlash(relative)))
		if readErr != nil || string(content) != expected {
			t.Fatalf("%s = %q err=%v", relative, content, readErr)
		}
	}
}

func TestCompressAndExtractEmptyDirectory(t *testing.T) {
	manager, root := newTestManager(t)
	mustMkdirAll(t, filepath.Join(root, "empty"))
	for _, format := range []struct {
		name string
		file string
	}{
		{name: archiveFormatZIP, file: "empty.zip"},
		{name: archiveFormatTAR, file: "empty.tar"},
		{name: archiveFormatTARGZ, file: "empty.tar.gz"},
	} {
		result, err := manager.Action(context.Background(), contract.FileActionRequest{
			Action: "compress", Sources: []string{"/empty"}, Target: "/",
			Name: format.file, Format: format.name,
		})
		if err != nil {
			t.Fatalf("compress %s: %v", format.name, err)
		}
		_, err = manager.Action(context.Background(), contract.FileActionRequest{
			Action: "extract", Sources: []string{"/" + format.file}, Target: "/",
			Name: "restored-" + strings.ReplaceAll(format.name, ".", "-"), Format: format.name,
			ExpectedResourceVersion: result.Succeeded[0].ResourceVersion,
		})
		if err != nil {
			t.Fatalf("extract %s: %v", format.name, err)
		}
	}
}

func TestExtractRejectsTraversalLinksAndSpecialEntries(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		format string
		write  func(*testing.T, string)
	}{
		{
			name: "traversal.zip", format: archiveFormatZIP,
			write: func(t *testing.T, target string) {
				writeZIPFixture(t, target, "../escaped.txt", 0644, "escaped")
			},
		},
		{
			name: "symlink.zip", format: archiveFormatZIP,
			write: func(t *testing.T, target string) {
				writeZIPFixture(t, target, "link", os.ModeSymlink|0777, "/etc/passwd")
			},
		},
		{
			name: "hardlink.tar.gz", format: archiveFormatTARGZ,
			write: func(t *testing.T, target string) {
				writeTARGZFixture(t, target, &tar.Header{
					Name: "link", Typeflag: tar.TypeLink, Linkname: "target", Mode: 0644,
				}, "")
			},
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			manager, root := newTestManager(t)
			fixture.write(t, filepath.Join(root, fixture.name))
			_, err := manager.Action(context.Background(), contract.FileActionRequest{
				Action: "extract", Sources: []string{"/" + fixture.name},
				Target: "/", Name: "output", Format: fixture.format,
			})
			if !errors.Is(err, ErrInvalidArchive) {
				t.Fatalf("expected invalid archive, got %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "output")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed extraction left output: %v", err)
			}
			matches, _ := filepath.Glob(filepath.Join(root, ".kpanel-extract-*"))
			if len(matches) != 0 {
				t.Fatalf("failed extraction left temporary paths: %#v", matches)
			}
		})
	}
}

func TestArchiveBudgetsCancellationAndCollisionsAreAtomic(t *testing.T) {
	root := t.TempDir()
	manager, err := New(Config{Root: root, MaxCopyBytes: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	writeZIPFixture(t, filepath.Join(root, "large.zip"), "large.txt", 0644, "12345")
	if _, err := manager.Action(context.Background(), contract.FileActionRequest{
		Action: "extract", Sources: []string{"/large.zip"}, Target: "/",
		Name: "large", Format: archiveFormatZIP,
	}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected extraction budget failure, got %v", err)
	}
	mustWrite(t, filepath.Join(root, "source.txt"), "1234")
	mustWrite(t, filepath.Join(root, "existing.zip"), "keep")
	if _, err := manager.Action(context.Background(), contract.FileActionRequest{
		Action: "compress", Sources: []string{"/source.txt"}, Target: "/",
		Name: "existing.zip", Format: archiveFormatZIP,
	}); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected archive collision, got %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(root, "existing.zip"))
	if string(content) != "keep" {
		t.Fatalf("existing archive was overwritten: %q", content)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := manager.Action(cancelled, contract.FileActionRequest{
		Action: "compress", Sources: []string{"/source.txt"}, Target: "/",
		Name: "cancelled.tar.gz", Format: archiveFormatTARGZ,
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "cancelled.tar.gz")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled archive left output: %v", err)
	}
}

func TestZIPDirectoryEntryCountIsBoundedBeforeAllocation(t *testing.T) {
	manager, root := newTestManager(t)
	endRecord := make([]byte, 22)
	copy(endRecord, []byte{'P', 'K', 0x05, 0x06})
	binary.LittleEndian.PutUint16(endRecord[8:10], uint16(maxCopyEntries+1))
	binary.LittleEndian.PutUint16(endRecord[10:12], uint16(maxCopyEntries+1))
	if err := os.WriteFile(filepath.Join(root, "many.zip"), endRecord, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := manager.Action(context.Background(), contract.FileActionRequest{
		Action: "extract", Sources: []string{"/many.zip"}, Target: "/",
		Name: "many", Format: archiveFormatZIP,
	})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected bounded ZIP directory rejection, got %v", err)
	}
}

func BenchmarkCompressTARGZ8MiB(b *testing.B) {
	manager, root := newTestManager(b)
	content := make([]byte, 8<<20)
	for index := range content {
		content[index] = byte(index * 31)
	}
	if err := os.WriteFile(filepath.Join(root, "payload.bin"), content, 0600); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		name := "payload-" + strconv.Itoa(index) + ".tar.gz"
		if _, err := manager.Action(context.Background(), contract.FileActionRequest{
			Action: "compress", Sources: []string{"/payload.bin"}, Target: "/",
			Name: name, Format: archiveFormatTARGZ,
		}); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

func BenchmarkExtractTARGZ8MiB(b *testing.B) {
	manager, root := newTestManager(b)
	content := make([]byte, 8<<20)
	for index := range content {
		content[index] = byte(index * 31)
	}
	if err := os.WriteFile(filepath.Join(root, "payload.bin"), content, 0600); err != nil {
		b.Fatal(err)
	}
	if _, err := manager.Action(context.Background(), contract.FileActionRequest{
		Action: "compress", Sources: []string{"/payload.bin"}, Target: "/",
		Name: "payload.tar.gz", Format: archiveFormatTARGZ,
	}); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		name := "restored-" + strconv.Itoa(index)
		if _, err := manager.Action(context.Background(), contract.FileActionRequest{
			Action: "extract", Sources: []string{"/payload.tar.gz"}, Target: "/",
			Name: name, Format: archiveFormatTARGZ,
		}); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

func writeZIPFixture(t *testing.T, target, name string, mode os.FileMode, content string) {
	t.Helper()
	file, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(mode)
	header.UncompressedSize64 = uint64(len(content))
	entry, err := writer.CreateHeader(header)
	if err == nil {
		_, err = entry.Write([]byte(content))
	}
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
}

func writeTARGZFixture(t *testing.T, target string, header *tar.Header, content string) {
	t.Helper()
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	tarWriter := tar.NewWriter(gzipWriter)
	header.Size = int64(len(content))
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, output.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}
