//go:build !windows

package hostbackup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// The staging path sits beside the restore root, so a local user who owns
// that parent can swap it for a link while the copy runs. Nothing may then be
// written, owned or chmodded through the link, and publish must refuse it.
func TestCopyTreeIgnoresStagingPathSwappedForLink(t *testing.T) {
	source := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "profile.d"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "profile.d", "x.sh"), []byte("echo staged\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(source, 0777); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Chmod(outside, 0700); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	next := filepath.Join(parent, "data.kpanel-next-test")
	ctx := &swapContext{Context: context.Background(), swap: func() error {
		if _, err := os.Lstat(next); err == nil {
			if err := os.Rename(next, filepath.Join(parent, "moved")); err != nil {
				return err
			}
		}
		return os.Symlink(outside, next)
	}}
	staged, err := copyTree(ctx, source, next)
	if ctx.err != nil {
		t.Fatal(ctx.err)
	}
	if entries, readErr := os.ReadDir(outside); readErr != nil || len(entries) != 0 {
		t.Fatalf("staging wrote through the swapped link: %v %v", entries, readErr)
	}
	if info, statErr := os.Stat(outside); statErr != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("staging changed the link target: %v %v", info, statErr)
	}
	if err == nil {
		target := filepath.Join(parent, "data")
		if err := publishStaged(next, target, staged); err == nil {
			t.Fatal("published a swapped staging path")
		}
		if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("swapped staging path reached the restore target", err)
		}
	}
}

// swapContext runs swap once, on the first cancellation check of the copy.
type swapContext struct {
	context.Context
	once sync.Once
	swap func() error
	err  error
}

func (c *swapContext) Err() error {
	c.once.Do(func() { c.err = c.swap() })
	return nil
}
