//go:build windows

package filemanager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

func TestWindowsVolumeArchiveJobsPersistAndReopen(t *testing.T) {
	for _, drive := range []string{"C", "D"} {
		t.Run(drive, func(t *testing.T) {
			_, c, d := windowsVolumeManager(t)
			config := Config{Root: "/", VolumeRoots: map[string]string{"C": c, "D": d}, TrashVirtual: "/" + drive + "/private/state/file-trash"}
			m, err := New(config)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = m.Close() })
			mustWrite(t, filepath.Join(d, "source.txt"), "archive payload")
			if _, err := m.List(context.Background(), "/D", 500); err != nil {
				t.Fatal(err)
			}
			if jobs, err := m.ArchiveJobs(); err != nil || len(jobs) != 0 {
				t.Fatalf("initial archive state: jobs=%v err=%v", jobs, err)
			}
			source, err := m.Stat("/D/source.txt")
			if err != nil {
				t.Fatal(err)
			}
			job, err := m.StartArchiveJob(contract.FileActionRequest{Action: "compress", Sources: []string{source.Path}, Target: "/D", Name: "result.zip", Format: "zip", ExpectedResourceVersion: source.ResourceVersion})
			if err != nil {
				t.Fatal(err)
			}
			if result := waitArchiveJob(t, m, job.ID); result.State != "complete" {
				t.Fatalf("compress: %#v", result)
			}
			if err := m.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := New(config)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			jobs, err := reopened.ArchiveJobs()
			if err != nil || len(jobs) != 1 || jobs[0].ID != job.ID || jobs[0].State != "complete" {
				t.Fatalf("recovered archive state: jobs=%v err=%v", jobs, err)
			}
			archive, err := reopened.Stat("/D/result.zip")
			if err != nil {
				t.Fatal(err)
			}
			job, err = reopened.StartArchiveJob(contract.FileActionRequest{Action: "extract", Sources: []string{archive.Path}, Target: "/D", Name: "restored", ExpectedResourceVersion: archive.ResourceVersion})
			if err != nil {
				t.Fatal(err)
			}
			if result := waitArchiveJob(t, reopened, job.ID); result.State != "complete" {
				t.Fatalf("extract: %#v", result)
			}
			payload, err := os.ReadFile(filepath.Join(d, "restored", "source.txt"))
			if err != nil || string(payload) != "archive payload" {
				t.Fatalf("extracted payload=%q err=%v", payload, err)
			}
		})
	}
}

func TestWindowsNativeVolumeArchiveState(t *testing.T) {
	state := t.TempDir()
	volume := filepath.VolumeName(state)
	virtual := "/" + strings.ToUpper(volume[:1]) + filepath.ToSlash(strings.TrimPrefix(state, volume))
	m, err := New(Config{Root: "/", VolumeRoots: map[string]string{volume[:1]: volume + `\`}, TrashVirtual: virtual + "/file-trash"})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if _, err := m.List(context.Background(), virtual, 500); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ArchiveJobs(); err != nil {
		t.Fatalf("native volume archive state: %v", err)
	}
}

func TestWindowsVolumeArchiveStateRejectsNonDirectory(t *testing.T) {
	m, c, _ := windowsVolumeManager(t)
	mustWrite(t, filepath.Join(c, "private-trash"), "not a directory")
	if _, err := m.ArchiveJobs(); !errors.Is(err, ErrArchiveJobsUnavailable) {
		t.Fatalf("non-directory state ancestor: %v", err)
	}
}
