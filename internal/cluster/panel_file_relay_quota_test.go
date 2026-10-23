package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestPanelFileRelayBoundsRetryHistoryWithoutRepeatingBody(t *testing.T) {
	now := time.Now().UTC()
	sid := fmt.Sprintf("%032x", 1)
	s := newPanelFileRelaySession(sid, http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		_, _ = io.Copy(io.Discard, req.Body)
		<-req.Context().Done()
	}), now)
	defer s.abort(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	run := func(command FileRelayCommand) FileRelayEvent {
		if err := s.handleCommand(ctx, command, now); err != nil {
			t.Fatal(err)
		}
		select {
		case event := <-s.events:
			return event
		case <-ctx.Done():
			t.Fatal(ctx.Err())
			return FileRelayEvent{}
		}
	}
	run(FileRelayCommand{ID: fmt.Sprintf("%032x", 2), RequestID: sid, Kind: "request", Method: http.MethodPost, Path: "/v1/files/actions", BodyLength: -1})
	var first, last FileRelayCommand
	for i := 0; i < panelFileRelayProcessedLimit*3; i++ {
		last = FileRelayCommand{ID: fmt.Sprintf("%032x", i+3), RequestID: sid, Kind: "body", Offset: int64(i), Data: []byte("x")}
		if i == 0 {
			first = last
		}
		if event := run(last); event.Kind != "accepted" {
			t.Fatalf("body event=%v", event)
		}
	}
	if len(s.processed) > panelFileRelayProcessedLimit {
		t.Fatalf("retained commands=%d", len(s.processed))
	}
	if event := run(last); event.Kind != "accepted" {
		t.Fatalf("recent retry=%v", event)
	}
	if s.bodyOffset != panelFileRelayProcessedLimit*3 {
		t.Fatal("recent retry repeated body bytes")
	}
	if event := run(first); event.Kind != "error" {
		t.Fatalf("expired retry=%v", event)
	}
	if s.bodyOffset != panelFileRelayProcessedLimit*3 {
		t.Fatal("expired retry repeated body bytes")
	}
}

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
