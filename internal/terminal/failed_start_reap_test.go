package terminal

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type reapingProcess struct {
	*pendingCleanupProcess
	waits  atomic.Int32
	waited chan struct{}
}

func (p *reapingProcess) Wait() error {
	if p.waits.Add(1) == 1 {
		close(p.waited)
	}
	return nil
}

func TestFailedStartedProcessIsReapedExactlyOnce(t *testing.T) {
	for _, reason := range []string{"starter-error", "canceled", "owner-revoked"} {
		for _, failCleanup := range []bool{false, true} {
			t.Run(reason+map[bool]string{false: "/closed", true: "/retry"}[failCleanup], func(t *testing.T) {
				p := &reapingProcess{pendingCleanupProcess: &pendingCleanupProcess{fakeProcess: newFakeProcess(), fail: failCleanup}, waited: make(chan struct{})}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				m := New(Config{MaxSessions: 1})
				defer m.CloseAll()
				starterError := errors.New("started but failed initialization")
				want := starterError
				start := func(uint16, uint16) (Process, error) {
					switch reason {
					case "canceled":
						cancel()
						want = context.Canceled
					case "owner-revoked":
						m.CloseOwner("owner")
						want = ErrClosed
					default:
						return p, starterError
					}
					return p, nil
				}
				if _, err := m.open(ctx, "owner", 24, 80, start, false); !errors.Is(err, want) {
					t.Fatalf("open=%v, want %v", err, want)
				}
				if failCleanup {
					if !m.Busy() {
						t.Fatal("unconfirmed cleanup lost its capacity reservation")
					}
					p.cleanupMu.Lock()
					p.fail = false
					p.cleanupMu.Unlock()
					m.reap(time.Now())
				}
				select {
				case <-p.waited:
				case <-time.After(time.Second):
					t.Fatal("terminated child was never reaped")
				}
				m.CloseAll()
				if n := p.waits.Load(); n != 1 {
					t.Fatalf("child waited %d times", n)
				}
			})
		}
	}
}
