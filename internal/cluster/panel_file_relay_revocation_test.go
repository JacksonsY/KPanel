package cluster

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestV2ControllerRevocationCancelsOnlyOwnedPollingRelays(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(fmt.Sprintf("remote=%t", remote), func(t *testing.T) {
			started := make(chan *http.Request, 2)
			f := newStreamFixture(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				started <- r
				<-r.Context().Done()
			}))
			r := f.service.panelFileRelay
			now := f.service.now().UTC()
			open := func(owner string, serial int) *http.Request {
				t.Helper()
				id := fmt.Sprintf("%032x", serial)
				input := FileRelayPollRequest{SessionID: id, Command: &FileRelayCommand{
					ID: fmt.Sprintf("%032x", serial+100), RequestID: id, Kind: "request",
					Method: http.MethodGet, Path: "/v1/files", ExpiresAt: now.Add(time.Minute).Unix(),
				}}
				if _, err := r.poll(context.Background(), owner, input, now, func() bool { return true }); err != nil {
					t.Fatal(err)
				}
				select {
				case request := <-started:
					return request
				case <-time.After(time.Second):
					t.Fatal("relay handler did not start")
					return nil
				}
			}
			owned := open(f.controller, 1)
			other := open("other-controller", 2)
			if remote {
				envelope, _, err := sealV2Request(http.MethodPost, v2RevokePath, v2Envelope{
					Protocol: FederationProtocolV2, ControllerID: f.controller, TargetID: f.service.NodeID(),
					Timestamp: now.Unix(), RequestID: fmt.Sprintf("%032x", 200),
				}, f.key, nodeNoiseKeyV2(f.service.nodeIdentityV2).Public, nil, []byte(`{}`))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.service.HandleFederationV2(context.Background(), "127.0.0.1", v2RevokePath, "", envelope); err != nil {
					t.Fatal(err)
				}
			} else if err := f.service.DeleteController(f.controller); err != nil {
				t.Fatal(err)
			}
			if owned.Context().Err() == nil {
				t.Error("revoked controller retained its running polling relay")
			}
			if other.Context().Err() != nil {
				t.Error("revocation canceled another controller's relay")
			}
			r.mu.Lock()
			count := len(r.sessions)
			r.mu.Unlock()
			if count != 1 {
				t.Errorf("revocation did not release only the owner's quota: sessions=%d", count)
			}
		})
	}
}

func TestPanelFileRelayRechecksAdmissionAndRejectsCanceledCommands(t *testing.T) {
	r := newPanelFileRelay()
	defer r.closeAll()
	r.setHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("revoked relay reached the file handler")
	}))
	now := time.Now().UTC()
	id := fmt.Sprintf("%032x", 1)
	input := FileRelayPollRequest{SessionID: id, Command: &FileRelayCommand{
		ID: fmt.Sprintf("%032x", 2), RequestID: id, Kind: "request", Method: http.MethodGet,
		Path: "/v1/files", ExpiresAt: now.Add(time.Minute).Unix(),
	}}
	// Authentication may have succeeded earlier; admission must use current state.
	for _, allowed := range []func() bool{nil, func() bool { return false }} {
		if _, err := r.poll(context.Background(), "revoked", input, now, allowed); !errors.Is(err, ErrAuthentication) {
			t.Fatalf("stale admission error=%v", err)
		}
		if len(r.sessions) != 0 {
			t.Fatal("stale admission consumed session capacity")
		}
	}
	// A command already holding the old session cannot restart it after cleanup.
	session := newPanelFileRelaySession(id, r.handler, now)
	session.controllerID = "revoked"
	r.sessions[id] = session
	r.closeController("revoked")
	if err := session.handleCommand(context.Background(), *input.Command, now); !errors.Is(err, ErrFileRelayUnavailable) {
		t.Fatalf("canceled session accepted a late command: %v", err)
	}
}
