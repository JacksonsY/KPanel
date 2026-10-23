//go:build linux

package filemanager

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileRootOperationsRejectLinkedParents(t *testing.T) {
	operations := map[string]func(*fileRoot) error{
		"read": func(root *fileRoot) error { _, err := root.ReadFile("linked/sentinel"); return err },
		"write": func(root *fileRoot) error {
			file, err := root.OpenFile("linked/sentinel", os.O_WRONLY|os.O_TRUNC, 0600)
			if file != nil {
				file.Close()
			}
			return err
		},
		"mkdir":      func(root *fileRoot) error { return root.Mkdir("linked/new", 0700) },
		"mkdir-all":  func(root *fileRoot) error { return root.MkdirAll("linked/new/nested", 0700) },
		"chmod":      func(root *fileRoot) error { return root.Chmod("linked/sentinel", 0777) },
		"times":      func(root *fileRoot) error { return root.Chtimes("linked/sentinel", time.Unix(1, 0), time.Unix(1, 0)) },
		"rename":     func(root *fileRoot) error { return root.Rename("source", "linked/new") },
		"remove":     func(root *fileRoot) error { return root.Remove("linked/sentinel") },
		"remove-all": func(root *fileRoot) error { return root.RemoveAll("linked/sentinel") },
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			m, root := newTestManager(t)
			mustMkdirAll(t, filepath.Join(root, "other"))
			mustWrite(t, filepath.Join(root, "other", "sentinel"), "unchanged")
			mustWrite(t, filepath.Join(root, "source"), "source")
			if err := os.Symlink("other", filepath.Join(root, "linked")); err != nil {
				t.Fatal(err)
			}
			if err := operation(m.rootFS); err == nil {
				t.Fatal("operation followed a linked parent")
			}
			content, err := os.ReadFile(filepath.Join(root, "other", "sentinel"))
			if err != nil || string(content) != "unchanged" {
				t.Fatalf("sentinel=%q error=%v", content, err)
			}
			entries, err := os.ReadDir(filepath.Join(root, "other"))
			if err != nil || len(entries) != 1 {
				t.Fatalf("other directory changed: %v %v", entries, err)
			}
		})
	}
}

func TestFileRootRejectsLeafLinksButCanUnlinkTheLink(t *testing.T) {
	m, root := newTestManager(t)
	mustWrite(t, filepath.Join(root, "sentinel"), "unchanged")
	if err := os.Symlink("sentinel", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if file, err := m.rootFS.OpenFile("link", os.O_WRONLY|os.O_TRUNC, 0600); err == nil {
		file.Close()
		t.Fatal("write followed leaf link")
	}
	if err := m.rootFS.Chmod("link", 0777); err == nil {
		t.Fatal("chmod followed leaf link")
	}
	if err := m.rootFS.Chtimes("link", time.Unix(1, 0), time.Unix(1, 0)); err == nil {
		t.Fatal("timestamp update followed leaf link")
	}
	if err := m.rootFS.RemoveAll("link"); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(root, "sentinel")); err != nil || string(content) != "unchanged" {
		t.Fatalf("sentinel=%q error=%v", content, err)
	}
}
