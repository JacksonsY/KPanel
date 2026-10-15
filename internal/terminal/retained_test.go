package terminal

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetainedIdleLifetimeQuotaAndOwnership(t *testing.T) {
	now := time.Now().UTC()
	m := New(Config{Now: func() time.Time { return now }, Starter: func(uint16, uint16) (Process, error) { return newFakeProcess(), nil }, MaxOwnerSessions: 2, MaxSessions: 3})
	defer m.CloseAll()
	kept, err := m.OpenRetained("owner", 24, 80)
	if err != nil || !kept.Retained {
		t.Fatalf("open: %+v %v", kept, err)
	}
	ordinary, err := m.Open("owner", 24, 80)
	if err != nil || ordinary.Retained {
		t.Fatalf("ordinary: %+v %v", ordinary, err)
	}
	if _, err = m.OpenRetained("owner", 24, 80); !errors.Is(err, ErrLimit) {
		t.Fatalf("owner quota: %v", err)
	}
	if _, err = m.Output(context.Background(), "other", kept.ID, 0, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ownership: %v", err)
	}
	if _, err = m.OpenRetained("other", 24, 80); err != nil {
		t.Fatal(err)
	}
	if _, err = m.OpenRetained("third", 24, 80); !errors.Is(err, ErrLimit) {
		t.Fatalf("global quota: %v", err)
	}
	m.reap(now.Add(DefaultIdleTimeout + time.Minute))
	item, _ := m.lookup("owner", kept.ID)
	if !item.isActive() {
		t.Fatal("retained session idle-reaped")
	}
	item, _ = m.lookup("owner", ordinary.ID)
	if item.isActive() {
		t.Fatal("ordinary session survived idle")
	}
	m.reap(now.Add(DefaultLifetime))
	item, _ = m.lookup("owner", kept.ID)
	if item.isActive() {
		t.Fatal("retained session exceeded hard lifetime")
	}
}

func TestRetainedWriterHandoffFencesOldStreams(t *testing.T) {
	p := newFakeProcess()
	m := New(Config{Starter: func(uint16, uint16) (Process, error) { return p, nil }})
	defer m.CloseAll()
	s, err := m.OpenRetained("owner", 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	old := "00000000000000000000000000000001"
	next := "00000000000000000000000000000002"
	for _, f := range []InputFrame{{Stream: old}, {Stream: old, Seq: 1, Data: []byte("once")}, {Stream: next}, {Stream: next, Seq: 1, Data: []byte("next")}, {Stream: next, Seq: 1, Data: []byte("next")}} {
		if err := m.InputSequenced("owner", s.ID, f); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []InputFrame{{Stream: old}, {Stream: old, Seq: 2, Data: []byte("stale")}} {
		if err := m.InputSequenced("owner", s.ID, f); !errors.Is(err, ErrInputSequence) {
			t.Fatalf("stale accepted: %v", err)
		}
	}
	p.mu.Lock()
	got := p.input.String()
	p.mu.Unlock()
	if got != "oncenext" {
		t.Fatalf("duplicate write: %q", got)
	}
}

func TestRetainedWriterHandoffCannotClearUncertainWrite(t *testing.T) {
	p := &partialInputProcess{fakeProcess: newFakeProcess()}
	m := New(Config{Starter: func(uint16, uint16) (Process, error) { return p, nil }})
	defer m.CloseAll()
	s, _ := m.OpenRetained("owner", 24, 80)
	if err := m.InputSequenced("owner", s.ID, InputFrame{Stream: "00000000000000000000000000000001", Seq: 1, Data: []byte("unsafe")}); !errors.Is(err, ErrInputUncertain) {
		t.Fatal(err)
	}
	if err := m.InputSequenced("owner", s.ID, InputFrame{Stream: "00000000000000000000000000000002"}); !errors.Is(err, ErrInputUncertain) {
		t.Fatal(err)
	}
	if p.calls != 1 {
		t.Fatalf("replayed %d writes", p.calls)
	}
}
