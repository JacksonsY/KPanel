package filemanager

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestShareStreamVerifiesEachBlockBeforeServing(t *testing.T) {
	manager, root := newTestManager(t)
	original := bytes.Repeat([]byte("A"), int(shareBlockSize*2+17))
	path := filepath.Join(root, "shared.bin")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	file, _, err := manager.OpenShare(context.Background(), "/shared.bin", "")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	first := make([]byte, 5)
	if _, err := io.ReadFull(file, first); err != nil || !bytes.Equal(first, original[:5]) {
		t.Fatalf("initial bytes=%q err=%v", first, err)
	}
	changed := bytes.Repeat([]byte("B"), len(original))
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	// A previously verified block remains safe and immutable to this reader.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	cached := make([]byte, shareBlockSize)
	if _, err := io.ReadFull(file, cached); err != nil || !bytes.Equal(cached, original[:shareBlockSize]) {
		t.Fatalf("cached block did not preserve authorized bytes: %v", err)
	}
	buffer := make([]byte, 8)
	if n, err := file.Read(buffer); n != 0 || !errors.Is(err, ErrConflict) {
		t.Fatalf("changed block exposed %d bytes, err=%v", n, err)
	}
}

func TestShareStreamRangeAcrossBlocks(t *testing.T) {
	manager, root := newTestManager(t)
	original := bytes.Repeat([]byte("ab"), int(shareBlockSize)+5)
	if err := os.WriteFile(filepath.Join(root, "range.bin"), original, 0o600); err != nil {
		t.Fatal(err)
	}
	file, _, err := manager.OpenShare(context.Background(), "/range.bin", "")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Seek(shareBlockSize-3, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	result := make([]byte, 8)
	if _, err := io.ReadFull(file, result); err != nil || !bytes.Equal(result, original[shareBlockSize-3:shareBlockSize+5]) {
		t.Fatalf("range=%q err=%v", result, err)
	}
	if _, err := file.Seek(-3, io.SeekEnd); err != nil {
		t.Fatal(err)
	}
	last, err := io.ReadAll(file)
	if err != nil || !bytes.Equal(last, original[len(original)-3:]) {
		t.Fatalf("suffix=%q err=%v", last, err)
	}
}
