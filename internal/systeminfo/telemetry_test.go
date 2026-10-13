package systeminfo

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

func TestTelemetryBlockedReadRetainsOneWorkerAndRejectsLateResult(t *testing.T) {
	c := NewCollector()
	release := make(chan struct{})
	var calls atomic.Int32
	read := func(context.Context) (contract.SystemSummary, error) {
		calls.Add(1)
		<-release
		return contract.SystemSummary{Hostname: "late"}, nil
	}
	value, err := c.collectTelemetry(context.Background(), 20*time.Millisecond, read)
	if !errors.Is(err, context.DeadlineExceeded) || value.Hostname != "" {
		t.Fatalf("timeout: %+v %v", value, err)
	}
	c.telemetryMu.Lock()
	pending := c.telemetryPending
	c.telemetryMu.Unlock()
	for i := 0; i < 100; i++ {
		_, err = c.collectTelemetry(context.Background(), 20*time.Millisecond, read)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("blocked workers=%d", calls.Load())
	}
	close(release)
	<-pending.done
	if pending.value.Hostname != "" || !errors.Is(pending.err, context.DeadlineExceeded) {
		t.Fatal("late sample accepted")
	}
	value, err = c.collectTelemetry(context.Background(), time.Second, func(context.Context) (contract.SystemSummary, error) {
		return contract.SystemSummary{Hostname: "recovered"}, nil
	})
	if err != nil || value.Hostname != "recovered" {
		t.Fatalf("recovery: %+v %v", value, err)
	}
}

func TestTelemetryCanceledCallerDoesNotStartCollection(t *testing.T) {
	c := NewCollector()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.collectTelemetry(ctx, time.Second, func(context.Context) (contract.SystemSummary, error) {
		t.Error("read called")
		return contract.SystemSummary{}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
