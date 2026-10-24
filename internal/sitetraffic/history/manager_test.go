package history

import (
	"context"
	"fmt"
	"github.com/kejilion/kejilion-panel/internal/sitetraffic"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagerActualLogsRestartPauseAndClear(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "log/nginx"), 0700)
	line := fmt.Sprintf("81.2.69.160 - - [%s] \"GET /real?key=secret HTTP/1.1\" 200 120 \"-\" \"agent\"\n", time.Now().Format("02/Jan/2006:15:04:05 -0700"))
	log := filepath.Join(root, "log/nginx/site.example.access.log")
	os.WriteFile(log, []byte(line), 0600)
	dbdir := filepath.Join(t.TempDir(), "stats")
	reader := sitetraffic.NewReader(root)
	var calls atomic.Int64
	source := func(ctx context.Context, in sitetraffic.CollectInput) (sitetraffic.Batch, error) {
		calls.Add(1)
		return reader.Collect(ctx, in)
	}
	m := New(dbdir, source)
	m.collect(ctx)
	if calls.Load() != 0 {
		t.Fatal("unused feature collected")
	}
	m.Action(ctx, "enable")
	m.collect(ctx)
	snap, err := m.Query(ctx, "24h", "site.example")
	if err != nil || snap.Requests != 1 || snap.Paths[0].Name != "/real" {
		t.Fatalf("real pipeline %+v %v", snap, err)
	}
	m.Close()
	m = New(dbdir, source)
	defer m.Close()
	m.collect(ctx)
	snap, _ = m.Query(ctx, "24h", "")
	if snap.Requests != 1 {
		t.Fatal("restart duplicate")
	}
	m.Action(ctx, "pause")
	before := calls.Load()
	m.collect(ctx)
	if calls.Load() != before {
		t.Fatal("paused collection")
	}
	m.Action(ctx, "clear")
	m.Action(ctx, "enable")
	m.collect(ctx)
	snap, _ = m.Query(ctx, "24h", "")
	if snap.Requests != 0 {
		t.Fatal("clear replay")
	}
	f, _ := os.OpenFile(log, os.O_WRONLY|os.O_APPEND, 0600)
	f.WriteString(line)
	f.Close()
	m.collect(ctx)
	snap, _ = m.Query(ctx, "24h", "")
	if snap.Requests != 1 {
		t.Fatal("new request missing")
	}
}
