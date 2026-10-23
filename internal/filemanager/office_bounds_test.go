package filemanager

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
)

func TestOfficeBoundsDirectoryBeforeZIPParsing(t *testing.T) {
	data := append([]byte(nil), officeFixture(t, map[string]string{})...)
	// EOCD advertises more entries than the Office budget. The declared limit
	// must win before the ZIP parser allocates or diagnoses the directory.
	end := len(data) - 22 - len("preserve package comment")
	binary.LittleEndian.PutUint16(data[end+8:], 4096)
	binary.LittleEndian.PutUint16(data[end+10:], 4096)
	if _, err := openOffice(context.Background(), data); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("directory budget error=%v", err)
	}
}

func TestOfficeBoundedDirectoryPreservesPackageComment(t *testing.T) {
	p, err := openOffice(context.Background(), officeFixture(t, map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if p.zip.Comment != "preserve package comment" {
		t.Fatalf("comment=%q", p.zip.Comment)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openOffice(ctx, officeFixture(t, map[string]string{})); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
}
