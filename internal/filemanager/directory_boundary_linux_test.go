//go:build linux

package filemanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenameRejectsLinkedParentComponents(t *testing.T) {
	for _, sourceLink := range []bool{false, true} {
		t.Run(map[bool]string{false: "destination", true: "source"}[sourceLink], func(t *testing.T) {
			m, root := newTestManager(t)
			for _, name := range []string{"source", "target", "other"} {
				mustMkdirAll(t, filepath.Join(root, name, "nested"))
			}
			mustWrite(t, filepath.Join(root, "source", "nested", "file"), "source")
			linked := "target"
			if sourceLink {
				linked = "source"
				mustWrite(t, filepath.Join(root, "other", "nested", "file"), "other")
			}
			if err := os.Rename(filepath.Join(root, linked), filepath.Join(root, linked+"-held")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("other", filepath.Join(root, linked)); err != nil {
				t.Fatal(err)
			}
			if err := renameNoReplaceRoot(m.rootFS, "/source/nested/file", "/target/nested/file"); err == nil {
				t.Fatal("rename followed a linked parent")
			}
			entries, err := os.ReadDir(filepath.Join(root, "target-held", "nested"))
			if sourceLink {
				entries, err = os.ReadDir(filepath.Join(root, "target", "nested"))
			}
			if err != nil || len(entries) != 0 {
				t.Fatalf("unintended target changed: %v %v", entries, err)
			}
		})
	}
}

func TestPinnedPublicationKeepsOpenedDirectoryIdentity(t *testing.T) {
	m, root := newTestManager(t)
	for _, name := range []string{"source", "target", "other"} {
		mustMkdirAll(t, filepath.Join(root, name))
	}
	mustWrite(t, filepath.Join(root, "source", "file"), "source")
	from, _, err := m.openReceiveDirectory("/source", false)
	if err != nil {
		t.Fatal(err)
	}
	defer from.Close()
	to, _, err := m.openReceiveDirectory("/target", false)
	if err != nil {
		t.Fatal(err)
	}
	defer to.Close()
	if err := os.Rename(filepath.Join(root, "target"), filepath.Join(root, "target-held")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("other", filepath.Join(root, "target")); err != nil {
		t.Fatal(err)
	}
	if err := publishReceiveObject(from, "file", to, "file", false); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(root, "target-held", "file")); err != nil || string(content) != "source" {
		t.Fatalf("pinned result=%q error=%v", content, err)
	}
	if _, err := os.Stat(filepath.Join(root, "other", "file")); !os.IsNotExist(err) {
		t.Fatalf("replacement directory changed: %v", err)
	}
}
