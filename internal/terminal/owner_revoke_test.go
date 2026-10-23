package terminal

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCloseOwnerInvalidatesAnInFlightSpawn(t *testing.T) {
	for _, failCleanup := range []bool{false, true} {
		t.Run(map[bool]string{false: "closed", true: "cleanup-retried"}[failCleanup], func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			p := &pendingCleanupProcess{fakeProcess: newFakeProcess(), fail: failCleanup}
			m := New(Config{MaxSessions: 1, Starter: func(uint16, uint16) (Process, error) {
				close(entered)
				<-release
				return p, nil
			}})
			defer m.CloseAll()
			done := make(chan error, 1)
			go func() { _, err := m.Open("owner", 24, 80); done <- err }()
			<-entered
			m.CloseOwner("owner")
			unblock()
			select {
			case err := <-done:
				if !errors.Is(err, ErrClosed) {
					t.Fatalf("revoked open=%v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("revoked spawn did not finish")
			}
			if failCleanup {
				if !m.Busy() {
					t.Fatal("failed termination lost process ownership")
				}
				if _, err := m.OpenWithStarter("other", 24, 80, func(uint16, uint16) (Process, error) { return newFakeProcess(), nil }); !errors.Is(err, ErrLimit) {
					t.Fatalf("cleanup quota=%v", err)
				}
				p.cleanupMu.Lock()
				p.fail = false
				p.cleanupMu.Unlock()
				m.reap(time.Now())
			}
			p.mu.Lock()
			killed := p.killed
			p.mu.Unlock()
			if !killed || m.Busy() {
				t.Fatal("revoked process was not cleaned up")
			}
		})
	}
}
