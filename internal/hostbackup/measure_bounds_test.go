package hostbackup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

func TestMeasureUsesPinnedBatchesWithoutReadingContents(t *testing.T) {
	e, _, p := hostFixture(t)
	for _, directory := range []string{".", "nested"} {
		if err := os.MkdirAll(e.host("/home/app/"+directory), 0700); err != nil {
			t.Fatal(err)
		}
		for n := 0; n < 260; n++ {
			if err := os.WriteFile(e.host(fmt.Sprintf("/home/app/%s/item-%03d", directory, n)), []byte("x"), 0000); err != nil {
				t.Fatal(err)
			}
		}
	}
	batches := map[string]int{}
	e.exportHook = func(stage, relative string) {
		if stage == "file-opened" {
			t.Fatal("metadata measurement opened file contents")
		}
		if stage == "directory-batch" {
			batches[relative]++
		}
	}
	r := p.Roots[0]
	r.Bytes, r.Entries = 0, 0
	if err := e.measure(context.Background(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Entries != 523 || r.Bytes != 524 || batches["."] != 3 || batches["nested"] != 3 {
		t.Fatalf("measurement=%+v batches=%v", r, batches)
	}
	for _, limit := range []string{"entries", "bytes", "cancel"} {
		t.Run(limit, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r := p.Roots[0]
			r.Bytes, r.Entries = 0, 0
			if limit == "entries" {
				r.Entries = 99998 // A finite fixture crosses the same real budget.
			} else if limit == "bytes" {
				r.Bytes = backup.MaxBytes
			}
			visited := 0
			e.exportHook = func(stage, relative string) {
				if stage == "before-open" {
					visited++
				}
				if limit == "cancel" && stage == "directory-batch" {
					cancel()
				}
			}
			err := e.measure(ctx, &r)
			if err == nil || limit == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("measurement ignored %s: %v", limit, err)
			}
			if visited > 4 {
				t.Fatalf("measurement continued past %s budget: %d entries", limit, visited)
			}
		})
	}
}

func TestDirectoryBatchReaderBoundsAndCancellation(t *testing.T) {
	directory := t.TempDir()
	for n := 0; n < 260; n++ {
		if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("item-%03d", n)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, stop := range []bool{false, true} {
		f, err := os.Open(directory)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		count, batches := 0, 0
		err = readDirectoryBatches(ctx, f, func(batch []os.DirEntry) error {
			if len(batch) > 128 {
				t.Fatalf("unbounded batch: %d", len(batch))
			}
			count += len(batch)
			batches++
			if stop {
				cancel()
			}
			return nil
		})
		cancel()
		f.Close()
		if stop {
			if !errors.Is(err, context.Canceled) || count != 128 || batches != 1 {
				t.Fatalf("cancelled batch reader: count=%d batches=%d err=%v", count, batches, err)
			}
		} else if err != nil || count != 260 || batches != 3 {
			t.Fatalf("batch reader: count=%d batches=%d err=%v", count, batches, err)
		}
	}
}

func TestExportPayloadsShareActualJobBudgets(t *testing.T) {
	e, _, p := hostFixture(t)
	for _, limit := range []string{"entries", "bytes"} {
		t.Run(limit, func(t *testing.T) {
			w := exportWalker{ctx: context.Background()}
			if limit == "entries" {
				w.count = 99998
			} else {
				w.bytes = backup.MaxBytes - 4
			}
			dir := t.TempDir()
			if err := e.writePayloadWithWalker(filepath.Join(dir, "first.payload"), p, &w); err != nil {
				t.Fatal(err)
			}
			if err := e.writePayloadWithWalker(filepath.Join(dir, "second.payload"), p, &w); !errors.Is(err, backup.ErrInvalid) {
				t.Fatalf("second module bypassed %s limit: %v", limit, err)
			}
		})
	}
}
