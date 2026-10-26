//go:build windows

package filemanager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

func TestWindowsReceiveRestartRecovery(t *testing.T) {
	for _, action := range []string{"resume", "abort", "expire", "replacement", "parent-replacement"} {
		t.Run(action, func(t *testing.T) {
			ctx := context.Background()
			m, c, d := windowsVolumeManager(t)
			if err := os.Mkdir(filepath.Join(c, "target"), 0700); err != nil {
				t.Fatal(err)
			}
			key := strings.Repeat("a", 64)
			input := contract.FileReceiveInput{Directory: "/C/target", Name: "中文.bin", Kind: "file", SizeBytes: 11, SourceKey: key}
			session, err := m.BeginReceive(ctx, input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := m.WriteReceiveChunk(ctx, session.ID, key, 0, strings.NewReader("hello "), receiveDigest([]byte("hello "))); err != nil {
				t.Fatal(err)
			}
			if err := m.Close(); err != nil {
				t.Fatal(err)
			}
			temp := filepath.Join(c, "target", ".kpanel-upload-"+session.ID)
			if action == "replacement" {
				if err := os.Rename(temp, temp+".owned"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(temp, []byte("unrelated"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if action == "parent-replacement" {
				if err := os.Rename(filepath.Join(c, "target"), filepath.Join(c, "original")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(c, "target"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(temp, []byte("unrelated"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			m, err = New(Config{Root: "/", VolumeRoots: map[string]string{"C": c, "D": d}, ProtectedVirtual: []string{"/C/Secrets"}, TrashVirtual: "/C/private-trash"})
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			switch action {
			case "resume":
				status, err := m.ReceiveStatus(ctx, session.ID, key)
				if err != nil || status.Offset != 6 || status.PrefixSHA256 != receiveDigest([]byte("hello ")) {
					t.Fatalf("resume: %+v %v", status, err)
				}
				if _, err := m.WriteReceiveChunk(ctx, session.ID, key, 6, strings.NewReader("world"), receiveDigest([]byte("world"))); err != nil {
					t.Fatal(err)
				}
				if _, err := m.CommitReceive(ctx, session.ID, key, 11, receiveDigest([]byte("hello world"))); err != nil {
					t.Fatal(err)
				}
				if content, err := os.ReadFile(filepath.Join(c, "target", "中文.bin")); err != nil || string(content) != "hello world" {
					t.Fatalf("committed bytes %q: %v", content, err)
				}
			case "abort":
				if err := m.AbortReceive(ctx, session.ID, key); err != nil {
					t.Fatal(err)
				}
			case "expire":
				m.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
				input.Name, input.SourceKey = "new.bin", strings.Repeat("b", 64)
				if _, err := m.BeginReceive(ctx, input); err != nil {
					t.Fatal(err)
				}
			case "replacement", "parent-replacement":
				if err := m.AbortReceive(ctx, session.ID, key); !errors.Is(err, ErrConflict) {
					t.Fatalf("replacement accepted: %v", err)
				}
				if content, err := os.ReadFile(temp); err != nil || string(content) != "unrelated" {
					t.Fatalf("replacement modified %q: %v", content, err)
				}
				return
			}
			if _, err := os.Stat(temp); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("owned temporary bytes retained: %v", err)
			}
		})
	}
}
