package terminalbatch

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

type timedBatchDriver struct{ *fakeDriver }

func (d timedBatchDriver) Open(ctx context.Context, user string, host Host) (Session, error) {
	timer := time.NewTimer(5 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return Session{}, ctx.Err()
	case <-timer.C:
	}
	return d.fakeDriver.Open(ctx, user, host)
}

// Real scheduler and metadata persistence, simulated PTYs. Include cleanup in
// the measured lifetime and verify one execution per target, not just latency.
func TestSharedTerminalBatchPerformance(t *testing.T) {
	if os.Getenv("KPANEL_SHARED_TERMINAL_BENCH") != "1" {
		t.Skip("opt-in batch lifecycle comparison")
	}
	samples := make([]time.Duration, 12)
	for i := range samples {
		d := newFake()
		m, _ := openTest(t, timedBatchDriver{d})
		start := time.Now()
		if _, err := m.Create("user", strings.Repeat("a", 32), "true", targets(8)); err != nil {
			t.Fatal(err)
		}
		job := finished(t, m)
		samples[i] = time.Since(start)
		t.Logf("sample=%d total_ns=%d", i, samples[i].Nanoseconds())
		for _, result := range job.Results {
			if result.State != "succeeded" {
				t.Fatalf("result=%#v", result)
			}
		}
		d.mu.Lock()
		live, opens, sends, peak := len(d.live), d.opens, d.sends, d.max
		d.mu.Unlock()
		if live != 0 || opens != 8 || sends != 8 || peak > 4 {
			t.Fatalf("live=%d opens=%d sends=%d peak=%d", live, opens, sends, peak)
		}
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	t.Logf("n=12 targets=8 injected_open_delay=5ms total_p50=%v total_p95=%v executions=96 cleanup_confirmed=true", samples[5].Round(time.Microsecond), samples[11].Round(time.Microsecond))
}
