package agent

import (
	"encoding/json"
	"github.com/kejilion/kejilion-panel/internal/terminal"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRetainedTerminalOpenAndContainerRejection(t *testing.T) {
	s := &Server{}
	p := &blockedAgentTerminal{entered: make(chan struct{}), done: make(chan struct{})}
	s.terminals = terminal.New(terminal.Config{Starter: func(uint16, uint16) (terminal.Process, error) { return p, nil }})
	defer s.terminals.CloseAll()
	for _, tc := range []struct {
		body   string
		status int
	}{{`{"owner":"owner","rows":24,"columns":80,"retained":true}`, 201}, {`{"owner":"owner","rows":24,"columns":80,"retained":true,"containerId":"id","resourceVersion":"version"}`, 400}} {
		r := httptest.NewRequest(http.MethodPost, "/v1/terminals", strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer "+strings.Repeat("x", 32))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.terminalOpen(w, r)
		if w.Code != tc.status {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		if w.Code == 201 {
			var got terminal.Snapshot
			if json.Unmarshal(w.Body.Bytes(), &got) != nil || !got.Retained {
				t.Fatalf("missing retention ack: %s", w.Body.String())
			}
		}
	}
}

func TestTerminalCapabilityProbeAdvertisesRetentionWithoutStartingPTY(t *testing.T) {
	starts := 0
	s := &Server{terminals: terminal.New(terminal.Config{Starter: func(uint16, uint16) (terminal.Process, error) {
		starts++
		return nil, nil
	}})}
	defer s.terminals.CloseAll()
	r := httptest.NewRequest(http.MethodPost, "/v1/terminals/capabilities/input-protocol", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.terminalOperation(w, r, "probe")
	var capability struct {
		Protocol string `json:"protocol"`
		Retained bool   `json:"retained"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &capability) != nil || !capability.Retained || capability.Protocol != terminal.InputProtocol || starts != 0 {
		t.Fatalf("capability=%s status=%d starts=%d", w.Body.String(), w.Code, starts)
	}
}
