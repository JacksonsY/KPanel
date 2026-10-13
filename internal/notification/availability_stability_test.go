package notification

import (
	"github.com/kejilion/kejilion-panel/internal/cluster"
	"testing"
	"time"
)

func TestAvailabilityRecoveryNeedsDistinctFreshHealthySamples(t *testing.T) {
	now := time.Now().UTC()
	s := &Service{sustain: 3, alerts: map[string]alertState{"host:availability": {Active: true}}}
	sent := 0
	send := func(cluster.Host, string, string, string) (bool, bool) { sent++; return true, true }
	host := cluster.Host{ID: "host", State: cluster.HostOnline, LastSuccessAt: &now}
	for i := 0; i < 8; i++ {
		s.handleAvailability(host, now, "zh-CN", send)
	}
	if sent != 0 {
		t.Fatal("cached sample triggered recovery")
	}
	now = now.Add(time.Second)
	s.handleAvailability(host, now, "zh-CN", send)
	host.State = cluster.HostDegraded
	s.handleAvailability(host, now, "zh-CN", send)
	if sent != 0 {
		t.Fatal("degraded sample triggered recovery")
	}
	host.State = cluster.HostOnline
	for i := 0; i < 3; i++ {
		now = now.Add(time.Second)
		s.handleAvailability(host, now, "zh-CN", send)
		if i < 2 && sent != 0 {
			t.Fatal("degraded interval did not reset confirmation")
		}
	}
	if sent != 1 {
		t.Fatalf("recoveries=%d", sent)
	}
}
