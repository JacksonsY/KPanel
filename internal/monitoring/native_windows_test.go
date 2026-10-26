//go:build windows

package monitoring

import (
	"context"
	"github.com/kejilion/kejilion-panel/internal/systeminfo"
	"slices"
	"testing"
	"time"
)

func TestWindowsNativeHistoryPersistsAcrossRestart(t *testing.T) {
	directory := t.TempDir()
	collector := systeminfo.NewCollector()
	collector.CPUSampleInterval = 20 * time.Millisecond
	now := time.Now().UTC()
	config := Config{StateDir: directory, System: collector, Now: func() time.Time { return now }}
	s, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = s.Sample(ctx); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	history, err := restarted.History(ctx, "1h")
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Host) != 1 || history.Host[0].MemoryTotalBytes == 0 || history.Host[0].CPUCores == 0 || history.Host[0].DiskTotalBytes == 0 {
		t.Fatalf("native history missing real counters: %#v", history.Host)
	}
	if !slices.Equal(history.UnavailableMetrics, []string{"load", "swap", "diskIO", "networkConnections"}) {
		t.Fatal("unsupported counters presented as measured zero")
	}
	if history.Storage.MaxStorageBytes <= 0 || history.Storage.MaxRollupStorageBytes <= 0 {
		t.Fatal("history has no storage budget")
	}
}
