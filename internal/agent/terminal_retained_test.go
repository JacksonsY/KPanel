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
