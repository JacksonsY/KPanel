//go:build linux

package filemanager

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

func TestArchiveRecoveryRetainsUnidentifiedTemp(t *testing.T) {
	m, root := newTestManager(t)
	id := strings.Repeat("b", 32)
	temp := archiveJobTemp("/", "compress", id, 0)
	mustWrite(t, filepath.Join(root, filepath.Base(temp), "keep"), "replacement")
	record := archiveJobRecord{Job: contract.FileArchiveJob{ID: id, Action: "compress", State: "running", Target: "/", Sources: []string{"/source"}, Name: "result.zip", CreatedAt: time.Now(), UpdatedAt: time.Now()}, Temps: []string{temp}}
	data, err := json.Marshal(map[string]any{"version": 1, "records": []archiveJobRecord{record}})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(m.archiveStatePath(), "/"))), string(data))
	if _, err := m.ArchiveJobs(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, filepath.Base(temp), "keep")); err != nil || string(data) != "replacement" {
		t.Fatalf("unidentified object removed: %q %v", data, err)
	}
}

func TestArchiveStageBootstrapAvoidsWritableTarget(t *testing.T) {
	for _, action := range []string{"compress", "extract"} {
		t.Run(action, func(t *testing.T) {
			m, root := newTestManager(t)
			mustWrite(t, filepath.Join(root, "source"), "finite payload")
			mustMkdirAll(t, filepath.Join(root, "target"))
			if err := os.Chmod(filepath.Join(root, "target"), 0777); err != nil {
				t.Fatal(err)
			}
			writeZIPFixture(t, filepath.Join(root, "source.zip"), "source", 0600, "finite payload")
			prefix := ".kpanel-archive-"
			if action == "extract" {
				prefix = ".kpanel-extract-"
			}
			checked := false
			op := &archiveOperation{tempSuffix: "bootstrap", progress: func(_ *copyBudget) {
				if checked {
					return
				}
				checked = true
				if _, err := os.Lstat(filepath.Join(root, "target", prefix+"bootstrap")); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("stage was bootstrapped below a writable parent: %v", err)
				}
				if _, err := os.Lstat(filepath.Join(root, prefix+"bootstrap")); err != nil {
					t.Errorf("private ancestor stage missing: %v", err)
				}
			}}
			ctx := context.WithValue(context.Background(), archiveContextKey{}, op)
			var err error
			if action == "compress" {
				_, err = m.compressArchive(ctx, []string{"/source"}, "/target", "result.zip", archiveFormatZIP, nil)
			} else {
				_, err = m.extractArchive(ctx, "/source.zip", "/target", "result", archiveFormatZIP, "")
			}
			if err != nil || !checked {
				t.Fatalf("archive failed: %v checked=%v", err, checked)
			}
		})
	}
}

func TestArchiveRecoveryRetainsReplacedParentAndStage(t *testing.T) {
	for _, mode := range []string{"parent", "stage"} {
		t.Run(mode, func(t *testing.T) {
			m, root := newTestManager(t)
			id := strings.Repeat("c", 32)
			temp := archiveJobTemp("/target", "compress", id, 0)
			stage := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(temp, "/")))
			mustWrite(t, filepath.Join(stage, "body"), "original")
			parentInfo, err := os.Stat(filepath.Join(root, "target"))
			if err != nil {
				t.Fatal(err)
			}
			stageInfo, err := os.Stat(stage)
			if err != nil {
				t.Fatal(err)
			}
			record := archiveJobRecord{Job: contract.FileArchiveJob{ID: id, Action: "compress", State: "running", Target: "/target", Sources: []string{"/source"}, Name: "result.zip", CreatedAt: time.Now(), UpdatedAt: time.Now()}, Temps: []string{temp}, TempIdentities: map[string]archiveTempIdentity{temp: {Parent: stableReceiveIdentity(parentInfo), Object: stableReceiveIdentity(stageInfo), Directory: "/target", Workspace: stableReceiveIdentity(parentInfo)}}}
			original := stage
			if mode == "parent" {
				original = filepath.Join(root, "target")
			}
			if err := os.Rename(original, filepath.Join(root, "moved")); err != nil {
				t.Fatal(err)
			}
			mustWrite(t, filepath.Join(stage, "keep"), "replacement")
			data, err := json.Marshal(map[string]any{"version": 1, "records": []archiveJobRecord{record}})
			if err != nil {
				t.Fatal(err)
			}
			mustWrite(t, filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(m.archiveStatePath(), "/"))), string(data))
			jobs, err := m.ArchiveJobs()
			if err != nil || len(jobs) != 1 || jobs[0].State != "interrupted" {
				t.Fatalf("recovery=%+v %v", jobs, err)
			}
			if data, err := os.ReadFile(filepath.Join(stage, "keep")); err != nil || string(data) != "replacement" {
				t.Fatalf("replacement removed: %q %v", data, err)
			}
		})
	}
}

func TestCompressRejectsOutputParentOrStageReplacement(t *testing.T) {
	for _, mode := range []string{"parent", "stage", "cancel-stage"} {
		t.Run(mode, func(t *testing.T) {
			m, root := newTestManager(t)
			mustWrite(t, filepath.Join(root, "source"), "finite payload")
			mustMkdirAll(t, filepath.Join(root, "target"))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			swapped := false
			op := &archiveOperation{tempSuffix: "output-boundary", progress: func(_ *copyBudget) {
				if swapped {
					return
				}
				swapped = true
				original := filepath.Join(root, "target", ".kpanel-archive-output-boundary")
				if mode == "parent" {
					original = filepath.Join(root, "target")
				}
				if err := os.Rename(original, filepath.Join(root, "moved")); err != nil {
					t.Fatal(err)
				}
				if mode == "parent" {
					mustMkdirAll(t, original)
				} else {
					mustWrite(t, original, "replacement")
				}
				if mode == "cancel-stage" {
					cancel()
				}
			}}
			ctx = context.WithValue(ctx, archiveContextKey{}, op)
			_, err := m.compressArchive(ctx, []string{"/source"}, "/target", "result.zip", archiveFormatZIP, nil)
			if err == nil || !swapped {
				t.Fatalf("replacement was accepted: swapped=%v err=%v", swapped, err)
			}
			if _, err := os.Lstat(filepath.Join(root, "target", "result.zip")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("published into replacement: %v", err)
			}
			if mode != "parent" {
				data, err := os.ReadFile(filepath.Join(root, "target", ".kpanel-archive-output-boundary"))
				if err != nil || string(data) != "replacement" {
					t.Fatalf("cleanup changed replacement: %q %v", data, err)
				}
			}
		})
	}
}
