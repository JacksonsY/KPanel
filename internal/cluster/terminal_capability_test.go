package cluster

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/flynn/noise"
)

func TestRetainedStreamPreflightNeverSendsAnOpenToLegacyOrFailedProbe(t *testing.T) {
	for _, probeErr := range []error{nil, io.EOF, context.DeadlineExceeded, ErrAuthentication} {
		calls := 0
		dial := func(context.Context, byte, any) (*fileStreamConn, terminalStreamReady, error) {
			calls++
			return nil, terminalStreamReady{}, probeErr
		}
		_, _, err := openStreamTerminal(context.Background(), context.Background(), "host", dial, nil, 24, 80, true)
		if calls != 1 || err == nil || errors.Is(err, ErrTerminalRetentionUnavailable) != (probeErr == nil) {
			t.Fatalf("probe=%v: calls=%d error=%v", probeErr, calls, err)
		}
		if (probeErr == ErrAuthentication || probeErr == context.DeadlineExceeded) && isStreamDialError(err) {
			t.Fatal("authentication or timeout rejection was made retryable")
		}
	}
}

func TestLightPollingFallbackCannotQueueRetainedOpen(t *testing.T) {
	now := time.Now()
	service := newLightServiceForTest(t, &serviceTestClock{now: now})
	enrollment, err := service.CreateLightEnrollment()
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(enrollment.Command)
	key, err := GenerateFederationV2Keypair()
	if err != nil {
		t.Fatal(err)
	}
	enrolled, err := service.EnrollLightNode("198.51.100.10", LightEnrollRequest{
		Token: strings.Trim(fields[len(fields)-1], "'"), Name: "legacy", NodeVersion: "0.40.0", TerminalPublicKey: encodeTestKey(key.Public),
	})
	if err != nil {
		t.Fatal(err)
	}
	reportLinuxLightHostForTest(t, service, enrolled, now)
	node := service.lightTerminal.node(enrolled.NodeID, true)
	node.available, node.lastPoll = true, now
	_, err = service.TerminalOpen(context.Background(), enrolled.NodeID, TerminalOpenRequest{Rows: 24, Columns: 80, Retained: true})
	if !errors.Is(err, ErrTerminalRetentionUnavailable) {
		t.Fatalf("polling-only retained open = %v", err)
	}
	// Also exercise the final relay guard reached after a pre-open stream failure.
	_, err = service.lightTerminal.open(context.Background(), enrolled.NodeID, 24, 80, true)
	if !errors.Is(err, ErrTerminalRetentionUnavailable) {
		t.Fatalf("stream-to-polling fallback = %v", err)
	}
	if len(node.pending) != 0 || len(node.queued) != 0 || len(node.sessions) != 0 {
		t.Fatal("unnegotiated retained command or session was queued")
	}
}

func TestRetainedV2PreflightNeverSendsOpenOnMissingOrFailedCapability(t *testing.T) {
	for _, status := range []int{200, 404, 401, 403, 429, 502} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			client := &RemoteClient{client: &http.Client{Transport: v2RoundTripper(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Path != v2TerminalCapabilitiesPath {
					t.Fatalf("PTY open sent after capability failure: %s", r.URL.Path)
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"code":"rejected"}`))}, nil
			})}}
			key, err := noise.DH25519.GenerateKeypair(nil)
			if err != nil {
				t.Fatal(err)
			}
			peer, err := noise.DH25519.GenerateKeypair(nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.TerminalOpenV2(context.Background(), "https://panel.example.com", strings.Repeat("a", 32), strings.Repeat("b", 32), key, peer.Public, time.Now(), TerminalOpenRequest{Rows: 24, Columns: 80, Retained: true})
			if calls != 1 || err == nil || errors.Is(err, ErrTerminalRetentionUnavailable) != (status == 404) {
				t.Fatalf("status=%d calls=%d err=%v", status, calls, err)
			}
		})
	}
}
