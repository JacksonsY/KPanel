package cluster

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestPanelFileRelayQuotasAndControllerOwnership(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	r := newPanelFileRelay()
	defer r.closeAll()
	r.setHandler(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) { <-req.Context().Done() }))
	request := func(id int) FileRelayPollRequest {
		sid := fmt.Sprintf("%032x", id+1)
		return FileRelayPollRequest{SessionID: sid, Command: &FileRelayCommand{ID: fmt.Sprintf("%032x", id+10000), RequestID: sid, Kind: "request", Method: http.MethodGet, Path: "/v1/files", ExpiresAt: now.Add(time.Minute).Unix()}}
	}
	for i := 0; i < panelFileRelayMaxSessions; i++ {
		owner := fmt.Sprintf("controller-%d", i/panelFileRelayMaxControllerSessions)
		if _, err := r.poll(context.Background(), owner, request(i), now); err != nil {
			t.Fatalf("admit %d: %v", i, err)
		}
		if i == panelFileRelayMaxControllerSessions-1 {
			if _, err := r.poll(context.Background(), owner, request(1000), now); !errors.Is(err, ErrRateLimited) {
				t.Fatalf("controller quota: %v", err)
			}
		}
	}
	if _, err := r.poll(context.Background(), "new-controller", request(2000), now); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("global quota: %v", err)
	}
	if _, err := r.poll(context.Background(), "other-controller", request(0), now); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("ownership: %v", err)
	}
	// Existing sessions still accept control frames when capacity is full.
	control := request(0)
	control.Command = &FileRelayCommand{ID: fmt.Sprintf("%032x", 3000), RequestID: control.SessionID, Kind: "cancel", ExpiresAt: now.Add(time.Minute).Unix()}
	if _, err := r.poll(context.Background(), "controller-0", control, now); errors.Is(err, ErrRateLimited) {
		t.Fatal("cancel denied by quota")
	}
	r.mu.Lock()
	r.gcLocked(now.Add(panelFileRelaySessionTTL + time.Second))
	count := len(r.sessions)
	r.mu.Unlock()
	if count != 0 {
		t.Fatalf("expired sessions retained: %d", count)
	}
	if _, err := r.poll(context.Background(), "new-controller", request(2001), now); err != nil {
		t.Fatalf("capacity not reclaimed: %v", err)
	}
}

func TestPanelFileRelayGCReclaimsCanceledContext(t *testing.T) {
	r := newPanelFileRelay()
	defer r.closeAll()
	now := time.Now().UTC()
	s := newPanelFileRelaySession("session", nil, now)
	s.stop()
	s.touch(now.Add(time.Hour))
	r.sessions[s.id] = s
	r.gcLocked(now)
	if len(r.sessions) != 0 {
		t.Fatal("canceled session survived refreshed expiry")
	}
}
