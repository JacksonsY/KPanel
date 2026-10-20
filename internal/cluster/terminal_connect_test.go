package cluster

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/terminal"
)

func TestTerminalReadyRoleProbesBeforeAnyPTYAndPreservesOwner(t *testing.T) {
	f := newStreamFixture(t, nil)
	m := newEchoManager(t)
	f.service.terminal = managerTerminalBackend{manager: m}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, err := f.dial(ctx, f.key, fileStreamHello{Role: streamRolePanelTerminalReady})
	if err != nil {
		t.Fatal(err)
	}
	defer c.close()
	c, caps, err := readTerminalStreamReady(ctx, c, termInputProbe)
	if err != nil || !caps.Retained || caps.InputProtocol != terminal.InputProtocol {
		t.Fatalf("caps=%+v err=%v", caps, err)
	}
	// Capability readiness never has a session ID or creates a shell.
	if caps.SessionID != "" {
		t.Fatal("probe created a session")
	}
	c, ready, err := startTerminalStream(ctx, c, termOpen, terminalStreamOpen{Rows: 24, Columns: 80, Retained: true, InputProtocol: caps.InputProtocol})
	if err != nil || !ready.Retained {
		t.Fatalf("open=%+v err=%v", ready, err)
	}
	if _, err := m.Output(ctx, "other-owner", ready.SessionID, 0, 0); !errors.Is(err, terminal.ErrNotFound) {
		t.Fatalf("cross-owner read: %v", err)
	}
	if _, err := m.Output(ctx, "federation:"+f.controller, ready.SessionID, 0, 0); err != nil {
		t.Fatal(err)
	}
}

func TestTerminalReadyRoleCannotExpandReadOnlyScope(t *testing.T) {
	f := newStreamFixture(t, nil)
	f.service.terminal = managerTerminalBackend{manager: newEchoManager(t)}
	record, err := f.service.storeV2.Controller(f.controller)
	if err != nil {
		t.Fatal(err)
	}
	record.Scope = SummaryScope
	if err := f.service.storeV2.DeleteController(record.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.service.storeV2.AddController(record); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, err := f.dial(ctx, f.key, fileStreamHello{Role: streamRolePanelTerminalReady})
	if c != nil {
		c.close()
	}
	if err == nil {
		t.Fatal("read-only controller got a terminal stream")
	}
}

type countingCapabilitiesBackend struct {
	managerTerminalBackend
	probes atomic.Int32
}

func (b *countingCapabilitiesBackend) TerminalCapabilities(context.Context) (TerminalCapabilities, error) {
	b.probes.Add(1)
	return TerminalCapabilities{Protocol: terminal.InputProtocol, Retained: true}, nil
}
func TestTerminalStreamNegotiatesCombinedCapabilitiesOnce(t *testing.T) {
	f := newStreamFixture(t, nil)
	b := &countingCapabilitiesBackend{managerTerminalBackend: managerTerminalBackend{manager: newEchoManager(t)}}
	f.service.terminal = b
	var dials atomic.Int32
	s, _, err := openStreamTerminal(context.Background(), context.Background(), "host", f.panelTerminalDialer(t, &dials), nil, 24, 80, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.shutdown()
	if b.probes.Load() != 1 {
		t.Fatalf("probes=%d", b.probes.Load())
	}
}

func TestTerminalReconnectJitterIsBoundedAndVaried(t *testing.T) {
	for _, cap := range []time.Duration{250 * time.Millisecond, time.Second, 4 * time.Second} {
		seen := map[time.Duration]bool{}
		for range 100 {
			d := terminalReconnectDelay(cap)
			if d < cap/2 || d > cap {
				t.Fatalf("delay %v outside [%v,%v]", d, cap/2, cap)
			}
			seen[d] = true
		}
		if len(seen) < 2 {
			t.Fatal("retry delays are synchronized")
		}
	}
}

func TestTerminalReconnectStopsOnDefinitiveAuthenticationFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	s := &streamTerminal{ctx: ctx, buffer: terminal.NewBuffer(terminal.DefaultBufferBytes, 0), dial: func(context.Context, byte, any) (*fileStreamConn, terminalStreamReady, error) {
		calls++
		return nil, terminalStreamReady{}, ErrAuthentication
	}}
	s.reconnect()
	if calls != 1 || s.isDegraded() {
		t.Fatalf("calls=%d degraded=%v", calls, s.isDegraded())
	}
}

type forgedTerminalMetadataBackend struct{ managerTerminalBackend }

func (b forgedTerminalMetadataBackend) Open(ctx context.Context, owner string, rows, cols uint16) (terminal.Snapshot, error) {
	s, err := b.managerTerminalBackend.Open(ctx, owner, rows, cols)
	s.Retained = true
	s.CreatedAt = time.Now().Add(100 * 365 * 24 * time.Hour)
	return s, err
}
func (b forgedTerminalMetadataBackend) OpenRetained(ctx context.Context, owner string, rows, cols uint16) (terminal.Snapshot, error) {
	s, err := b.managerTerminalBackend.OpenRetained(ctx, owner, rows, cols)
	s.CreatedAt = time.Now().Add(100 * 365 * 24 * time.Hour)
	return s, err
}
func TestTerminalPeerCannotExtendLocalRetentionOrUpgradeTemporarySession(t *testing.T) {
	f := newStreamFixture(t, nil)
	f.service.terminal = forgedTerminalMetadataBackend{managerTerminalBackend{manager: newEchoManager(t)}}
	var dials atomic.Int32
	if s, _, err := openStreamTerminal(context.Background(), context.Background(), "host", f.panelTerminalDialer(t, &dials), nil, 24, 80); !errors.Is(err, ErrAuthentication) || s != nil {
		t.Fatalf("unexpected retention: stream=%v err=%v", s, err)
	}
	before := time.Now()
	s, opened, err := openStreamTerminal(context.Background(), context.Background(), "host", f.panelTerminalDialer(t, &dials), nil, 24, 80, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.shutdown()
	if s.createdAt.Before(before) || s.createdAt.After(time.Now()) || !opened.CreatedAt.Equal(s.createdAt) {
		t.Fatal("peer clock controls local lifetime")
	}
	p := newPanelStreams()
	defer p.closeAll()
	if !p.putTerminal(s) {
		t.Fatal("admission failed")
	}
	p.expire(opened.CreatedAt.Add(terminal.DefaultLifetime))
	if p.terminal("host", opened.SessionID) != nil {
		t.Fatal("future peer clock escaped lifetime")
	}
}

func TestTerminalDuplicateHandleNeverDisplacesTrackedSession(t *testing.T) {
	p := newPanelStreams()
	defer p.closeAll()
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	first := &streamTerminal{ctx: ctx, hostID: "host", sessionID: "duplicate", cancel: stop, buffer: terminal.NewBuffer(32, 0)}
	second := &streamTerminal{ctx: ctx, hostID: "host", sessionID: "duplicate"}
	if !p.putTerminal(first) || p.putTerminal(second) || p.terminal("host", "duplicate") != first {
		t.Fatal("peer duplicate displaced tracked session")
	}
}

func TestTerminalOutboundRegistrationRejectsRevocationDuringDial(t *testing.T) {
	f := newStreamFixture(t, nil)
	f.service.terminal = managerTerminalBackend{manager: newEchoManager(t)}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, err := f.dial(ctx, f.key, fileStreamHello{Role: streamRolePanelTerminal})
	if err != nil {
		t.Fatal(err)
	}
	h := newFileStreamHub()
	defer h.closeAll()
	if h.track(c, "host:removed", func() bool { return false }) {
		t.Fatal("revoked peer admitted after dial")
	}
	if c.ctx.Err() == nil || len(h.connections) != 0 {
		t.Fatal("late socket remained alive or indexed")
	}
}
