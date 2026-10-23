package terminal

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCanceledTerminalOpenCleansUpBeforePublishing(t *testing.T) {
	for _, retained := range []bool{false, true} {
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		unblock := func() { once.Do(func() { close(release) }) }
		p := newFakeProcess()
		m := New(Config{Starter: func(uint16, uint16) (Process, error) { close(entered); <-release; return p, nil }})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			var err error
			if retained {
				_, err = m.OpenRetainedContext(ctx, "owner", 24, 80)
			} else {
				_, err = m.OpenContext(ctx, "owner", 24, 80)
			}
			done <- err
		}()
		<-entered
		cancel()
		unblock()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("open cancellation=%v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("canceled start did not return")
		}
		p.mu.Lock()
		killed := p.killed
		p.mu.Unlock()
		if !killed || m.Busy() {
			t.Fatal("canceled start published an owned process")
		}
		m.CloseAll()
	}
}

func TestQueuedAdmissionsRemainBoundedAndOwnerRevocationIsLocal(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	m := New(Config{MaxSessions: 2, MaxOwnerSessions: 1})
	defer m.CloseAll()
	first := make(chan error, 1)
	go func() {
		_, err := m.OpenWithStarter("other", 24, 80, func(uint16, uint16) (Process, error) { close(entered); <-release; return newFakeProcess(), nil })
		first <- err
	}()
	<-entered
	second := make(chan error, 1)
	go func() {
		_, err := m.OpenWithStarter("owner", 24, 80, func(uint16, uint16) (Process, error) {
			t.Error("revoked queued admission reached its starter")
			return newFakeProcess(), nil
		})
		second <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		m.mu.RLock()
		pending := m.opening
		m.mu.RUnlock()
		if pending == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("queued admission not reserved")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := m.Open("third", 24, 80); !errors.Is(err, ErrLimit) {
		t.Fatalf("queued quota=%v", err)
	}
	m.CloseOwner("owner")
	unblock()
	if err := <-first; err != nil {
		t.Fatalf("another owner's admission was revoked: %v", err)
	}
	if err := <-second; !errors.Is(err, ErrClosed) {
		t.Fatalf("queued revoked result=%v", err)
	}
	m.mu.RLock()
	metadata := len(m.openingOwners)
	pending := m.opening
	m.mu.RUnlock()
	if metadata != 0 || pending != 0 {
		t.Fatalf("admission state retained: owners=%d pending=%d", metadata, pending)
	}
}
