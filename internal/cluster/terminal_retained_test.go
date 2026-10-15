package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kejilion/kejilion-panel/internal/terminal"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetainedBackendRejectsLegacyWithoutOpening(t *testing.T) {
	b := legacyTerminalBackend{TerminalBackend: managerTerminalBackend{manager: newEchoManager(t)}}
	if _, err := openTerminalBackend(context.Background(), b, "owner", 24, 80, true); !errors.Is(err, ErrTerminalUnavailable) {
		t.Fatal(err)
	}
}

func TestRetainedStreamSurvivesIdleAndReconnectsSameProcess(t *testing.T) {
	f := newStreamFixture(t, nil)
	m := newEchoManager(t)
	f.service.terminal = managerTerminalBackend{manager: m}
	var dials atomic.Int32
	stream, opened, err := openStreamTerminal(context.Background(), context.Background(), "host", f.panelTerminalDialer(t, &dials), nil, 24, 80, true)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.shutdown()
	if !opened.Retained || !m.IsRetained("federation:"+f.controller, opened.SessionID) {
		t.Fatal("retention not propagated")
	}
	f.service.streams.putTerminal(stream)
	f.service.streams.expire(time.Now().Add(time.Hour))
	if f.service.streams.terminal("host", opened.SessionID) != stream {
		t.Fatal("retained stream idle reaped")
	}
	stream.mu.Lock()
	conn := stream.conn
	stream.mu.Unlock()
	conn.close()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		stream.mu.Lock()
		ready := stream.conn != nil && stream.conn != conn
		stream.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	stream.mu.Lock()
	ready := stream.conn != nil && stream.conn != conn
	stream.mu.Unlock()
	if !ready {
		t.Fatal("same session not reattached")
	}
	if err := stream.input(context.Background(), TerminalInputRequest{SessionID: opened.SessionID, Data: "ZWNobw"}); err != nil {
		t.Fatal(err)
	}
	readStreamTerminalUntil(t, stream, 0, "echo")
	f.service.streams.expire(opened.CreatedAt.Add(terminal.DefaultLifetime))
	if f.service.streams.terminal("host", opened.SessionID) != nil {
		t.Fatal("stream exceeded lifetime")
	}
}

func TestRetainedRelayRequiresExplicitAcknowledgement(t *testing.T) {
	for _, ack := range []bool{false, true} {
		t.Run(map[bool]string{false: "old-peer", true: "retained-peer"}[ack], func(t *testing.T) {
			now := time.Now()
			session := &lightTerminalSession{id: "session", retained: true, notify: make(chan struct{})}
			command := &lightTerminalCommand{command: TerminalRelayCommand{ID: "cmd", SessionID: "session", Path: v2TerminalOpenPath}, done: make(chan error, 1)}
			node := &lightTerminalNode{pending: map[string]*lightTerminalCommand{"cmd": command}}
			node.finishCommand(TerminalRelayEvent{CommandID: "cmd", SessionID: "session", Kind: "opened", Retained: ack}, session, now)
			err := <-command.done
			if (err == nil) != ack {
				t.Fatalf("ack %v: %v", ack, err)
			}
		})
	}
}

func TestRetainedWireFlagsAreOptionalAndRoundTrip(t *testing.T) {
	for _, v := range []bool{false, true} {
		b, _ := json.Marshal(TerminalOpenRequest{Rows: 24, Columns: 80, Retained: v})
		var req TerminalOpenRequest
		if json.Unmarshal(b, &req) != nil || req.Retained != v {
			t.Fatal(string(b))
		}
		b, _ = json.Marshal(TerminalOpenResponse{SessionID: "id", Retained: v})
		var res TerminalOpenResponse
		if json.Unmarshal(b, &res) != nil || res.Retained != v {
			t.Fatal(string(b))
		}
	}
}

type falseRetainedBackend struct {
	managerTerminalBackend
	closed bool
}

func (b *falseRetainedBackend) OpenRetained(ctx context.Context, owner string, rows, cols uint16) (terminal.Snapshot, error) {
	return b.Open(ctx, owner, rows, cols)
}
func (b *falseRetainedBackend) Close(ctx context.Context, owner, id string) error {
	b.closed = true
	return b.managerTerminalBackend.Close(ctx, owner, id)
}
func TestRetainedBackendCleansUnacknowledgedLaunch(t *testing.T) {
	b := &falseRetainedBackend{managerTerminalBackend: managerTerminalBackend{manager: newEchoManager(t)}}
	if _, err := openTerminalBackend(context.Background(), b, "owner", 24, 80, true); !errors.Is(err, ErrTerminalUnavailable) || !b.closed {
		t.Fatalf("downgrade: %v closed=%v", err, b.closed)
	}
}
