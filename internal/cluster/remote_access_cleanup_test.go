package cluster

import (
	"context"
	"github.com/kejilion/kejilion-panel/internal/terminal"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIndependentRepairRetiredStreamCleanupSurvivesJanitor(t *testing.T) {
	f := newStreamFixture(t, http.NotFoundHandler())
	enrollment, err := f.service.CreateLightEnrollment()
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(enrollment.Command)
	key, _ := GenerateFederationV2Keypair()
	enrolled, err := f.service.EnrollLightNode("198.51.100.10", LightEnrollRequest{Token: strings.Trim(fields[len(fields)-1], "'"), Name: "review-light", NodeVersion: "1.14.1", TerminalPublicKey: encodeTestKey(key.Public)})
	if err != nil {
		t.Fatal(err)
	}
	reportLinuxLightHostForTest(t, f.service, enrolled, time.Now().UTC())
	peer, _ := decodeTerminalRelayPublicKey(enrolled.TerminalPeerPublicKey)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	relay, _ := NewTerminalRelayClient(f.server.Client())
	manager := newEchoManager(t)
	var connected atomic.Bool
	done := make(chan error, 1)
	go func() {
		done <- relay.RunTerminalStream(ctx, f.server.URL, enrolled.NodeID, enrolled.TargetNodeID, key, peer, manager, "review-owner", func() { connected.Store(true) })
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for (!f.service.fileStreamHub.terminalAvailable(enrolled.NodeID) || !connected.Load()) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !connected.Load() {
		t.Fatal("control connection not ready")
	}
	opened, err := f.service.TerminalOpen(ctx, enrolled.NodeID, TerminalOpenRequest{Rows: 24, Columns: 80, Retained: true})
	if err != nil {
		t.Fatal(err)
	}
	stream := f.service.streams.terminal(enrolled.NodeID, opened.SessionID)
	if stream == nil {
		t.Fatal("expected stream")
	}
	defer stream.shutdown()
	stream.mu.Lock()
	old := stream.conn
	stream.mu.Unlock()
	disabled, err := f.service.UpdateRemoteAccess("initial", false, true)
	if err != nil {
		t.Fatal(err)
	}
	_ = disabled
	_ = old
	deadline = time.Now().Add(time.Second)
	for stream.ctx.Err() == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	f.service.streams.expire(time.Now())
	err = f.service.TerminalClose(ctx, enrolled.NodeID, TerminalCloseRequest{SessionID: opened.SessionID})
	output, probeErr := manager.Output(ctx, "review-owner", opened.SessionID, 0, 0)
	if err != nil || probeErr != nil || !output.Closed {
		t.Fatalf("retired stream close after janitor: close=%v probe=%v closed=%v", err, probeErr, output.Closed)
	}
}

func TestIndependentRepairCleanupConnectionNeverReconnects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleanup := &streamTerminal{ctx: ctx, cancel: cancel, buffer: terminal.NewBuffer(terminal.DefaultBufferBytes, 0), pending: make(map[uint64]chan string)}
	var panicValue any
	func() { defer func() { panicValue = recover() }(); cleanup.reconnect() }()
	if panicValue != nil {
		t.Fatalf("cleanup-only stream has no dialer, but reconnect panics: %v", panicValue)
	}
}
