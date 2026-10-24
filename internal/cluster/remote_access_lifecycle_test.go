package cluster

import (
	"context"
	"errors"
	"github.com/flynn/noise"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type independentCanceledOpenRemote struct {
	remoteV2API
	remoteV2TerminalAPI
	service    *Service
	closeCalls int
}

func (r *independentCanceledOpenRemote) TerminalOpenV2(ctx context.Context, _, _, _ string, _ noise.DHKey, _ []byte, _ time.Time, _ TerminalOpenRequest) (TerminalOpenResponse, error) {
	if _, err := r.service.UpdateRemoteAccess("initial", false, true); err != nil {
		return TerminalOpenResponse{}, err
	}
	<-ctx.Done()
	return TerminalOpenResponse{SessionID: "known-created-session"}, nil
}
func (r *independentCanceledOpenRemote) TerminalCloseV2(context.Context, string, string, string, noise.DHKey, []byte, time.Time, TerminalCloseRequest) error {
	r.closeCalls++
	return errors.New("peer temporarily unreachable")
}
func TestIndependentCanceledOpenDoesNotDiscardKnownIdentity(t *testing.T) {
	s := newLightServiceForTest(t, &serviceTestClock{now: time.Now().UTC()})
	record := testHostRecordV2(1, time.Now().UTC())
	record.Scope = SummaryTerminalFilesScope
	record.ResourceVersion = hostResourceVersionV2(record)
	name, err := s.secretsV2.WriteHostCredential(record.ID, testHostCredentialV2(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	record.CredentialFile = name
	if err := s.storeV2.AddHost(record); err != nil {
		t.Fatal(err)
	}
	remote := &independentCanceledOpenRemote{service: s}
	s.remoteV2 = remote
	response, err := s.TerminalOpen(context.Background(), record.ID, TerminalOpenRequest{Rows: 24, Columns: 80})
	if !errors.Is(err, ErrRemoteAccessDisabled) || remote.closeCalls != 1 {
		t.Fatalf("unexpected setup: response=%+v err=%v closes=%d", response, err, remote.closeCalls)
	}
	if response.SessionID == "" {
		t.Fatal("canceled successful open discarded known backend identity after failed close; caller cannot register pending cleanup")
	}
}

func TestIndependentDisabledStreamCannotReconnectInNewEpoch(t *testing.T) {
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
	// A cancelled settings HTTP request cannot confirm backend close. The server
	// keeps this session pending, exactly the state represented here.
	cleanup, stop := context.WithCancel(context.Background())
	stop()
	if err := f.service.TerminalClose(cleanup, enrolled.NodeID, TerminalCloseRequest{SessionID: opened.SessionID}); err == nil {
		t.Fatal("cancelled cleanup unexpectedly succeeded")
	}
	old.close()
	if _, err := f.service.UpdateRemoteAccess(disabled.ResourceVersion, true, true); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		stream.mu.Lock()
		current := stream.conn
		stream.mu.Unlock()
		if current != nil && current != old {
			t.Fatal("old stream automatically reattached to its PTY after disable/enable despite pending close")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
