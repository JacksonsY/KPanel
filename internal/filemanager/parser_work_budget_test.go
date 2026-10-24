package filemanager

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestArchiveBudgetIncludesInvisibleExtensionRecords(t *testing.T) {
	var sample bytes.Buffer
	tw := tar.NewWriter(&sample)
	if err := tw.WriteHeader(&tar.Header{Name: strings.Repeat("x", 200), Mode: 0600, Typeflag: tar.TypeReg, Format: tar.FormatGNU}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if sample.Bytes()[156] != tar.TypeGNULongName {
		t.Fatal("fixture lacks a GNU extension record")
	}
	var packed bytes.Buffer
	gz := gzip.NewWriter(&packed)
	for n := 0; n < 1200; n++ {
		if _, err := gz.Write(sample.Bytes()[:1024]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := gz.Write(sample.Bytes()[1024:]); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	m, root := newTestManager(t)
	filename := filepath.Join(root, "extensions.tar.gz")
	if err := os.WriteFile(filename, packed.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx := context.WithValue(context.Background(), archiveContextKey{}, &archiveOperation{inspect: func(string, os.FileInfo, int64) error { return nil }})
	budget := &copyBudget{maxEntries: 1, maxBytes: 1}
	var times []archiveDirectoryTime
	if err := m.readArchive(ctx, f, int64(packed.Len()), archiveFormatTARGZ, "/", budget, &times); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("extension stream escaped its budget: %v", err)
	}
}

func TestWordCellParagraphsHaveLinearAllocationBudget(t *testing.T) {
	const paragraphs = 20_000
	part := `<w:document xmlns:w="` + nsWord + `"><w:body><w:tbl><w:tr><w:tc>` + strings.Repeat(`<w:p><w:r><w:t>`+strings.Repeat("x", 80)+`</w:t></w:r></w:p>`, paragraphs) + `</w:tc></w:tr></w:tbl></w:body></w:document>`
	data := officeFixture(t, map[string]string{"word/document.xml": part})
	m, root := newTestManager(t)
	if err := os.WriteFile(filepath.Join(root, "paragraphs.docx"), data, 0600); err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	doc, err := m.ReadOffice(context.Background(), "/paragraphs.docx")
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if text := doc.Sections[0].Items[0].Table[0][0].Text; len(text) != paragraphs*81-1 {
		t.Fatalf("cell lost paragraph data: %d", len(text))
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	if allocated > 192<<20 {
		t.Fatalf("bounded 1.6 MiB cell allocated %d bytes", allocated)
	}
	t.Logf("cell parser allocated %d bytes", allocated)
}
