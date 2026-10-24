//go:build !windows

package hostbackup

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestExportRejectsFileSwappedToFIFOWithoutBlocking(t *testing.T) {
	e, _, p := hostFixture(t)
	e.exportHook = func(stage, relative string) {
		if stage != "before-open" || relative != "data" {
			return
		}
		name := e.host("/home/app/data")
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(name, 0600); err != nil {
			t.Fatal(err)
		}
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := e.writePayload(ctx, filepath.Join(t.TempDir(), "fifo.payload"), p); err == nil {
		t.Fatal("FIFO replacement accepted")
	}
	if time.Since(started) > time.Second {
		t.Fatal("FIFO open waited for a writer")
	}
}
