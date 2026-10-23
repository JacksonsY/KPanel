package terminal

import (
	"fmt"
	"testing"
	"time"
)

func TestFinishedCacheBoundsCountAndAllocatedBytes(t *testing.T) {
	for _, test := range []struct {
		name               string
		count, bytes, want int
	}{
		{"entries", maxFinishedSessions + 4, 1, maxFinishedSessions},
		{"bytes", 18, 1 << 20, maxFinishedBufferBytes / (1 << 20)},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Now().UTC()
			m := New(Config{Now: func() time.Time { return now }})
			defer m.CloseAll()
			for i := 0; i < test.count; i++ {
				id := fmt.Sprintf("finished-%03d", i)
				m.sessions[id] = &session{id: id, closed: true, finishedAt: now.Add(time.Duration(i) * time.Millisecond), buffer: make([]byte, 1, test.bytes)}
			}
			// Failed cleanup must remain registered even when its output exceeds
			// the completed-cache budget. It is still a real process owner.
			m.sessions["pending"] = &session{id: "pending", closeFailed: true, buffer: make([]byte, 1, maxFinishedBufferBytes+1)}
			m.trimFinished()
			if len(m.sessions) != test.want+1 {
				t.Fatalf("retained = %d", len(m.sessions))
			}
			if _, ok := m.sessions["pending"]; !ok {
				t.Fatal("pending cleanup evicted")
			}
			if _, ok := m.sessions["finished-000"]; ok {
				t.Fatal("oldest output not evicted")
			}
			// Fixtures have no process; prevent cleanup of their synthetic owner.
			delete(m.sessions, "pending")
		})
	}
}

func TestFinishedCacheReadsDoNotRenewRetention(t *testing.T) {
	now := time.Now().UTC()
	m := New(Config{Now: func() time.Time { return now }})
	defer m.CloseAll()
	item := &session{id: "finished", owner: "owner", closed: true, finishedAt: now, updatedAt: now, notify: make(chan struct{})}
	m.sessions[item.id] = item
	if _, err := item.currentOutput(0, 1, now.Add(finishedRetention)); err != nil {
		t.Fatal(err)
	}
	m.reap(now.Add(finishedRetention))
	if len(m.sessions) != 0 {
		t.Fatal("reading renewed completed output")
	}
}
