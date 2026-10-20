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
