package panel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/desktopworkspace"
	"github.com/kejilion/kejilion-panel/internal/terminal"
)

type retainedAgentStub struct {
	terminalAgentStub
	unsupported bool
	failInput   bool
	failClose   bool
	beforeInput func()
}

func (a *retainedAgentStub) Do(ctx context.Context, method, path, query, id string, body []byte) (AgentResponse, error) {
	if strings.HasSuffix(path, "/input") && a.beforeInput != nil {
		a.beforeInput()
	}
	if strings.HasSuffix(path, "/input") && a.failInput {
		return AgentResponse{}, errors.New("write outcome unknown")
	}
	if strings.HasSuffix(path, "/close") && a.failClose {
		return AgentResponse{}, errors.New("node offline")
	}
	response, err := a.terminalAgentStub.Do(ctx, method, path, query, id, body)
	if path == "/v1/terminals" && err == nil && !a.unsupported {
		var input struct {
			Retained bool `json:"retained"`
		}
		_ = json.Unmarshal(body, &input)
		var payload map[string]any
		_ = json.Unmarshal(response.Body, &payload)
		payload["retained"] = input.Retained
		response.Body, _ = json.Marshal(payload)
	}
	return response, err
}

func retainedHeaders(csrf *http.Cookie) map[string]string {
	return map[string]string{"Content-Type": "application/json", "Origin": "http://panel.test", "X-CSRF-Token": csrf.Value}
}
func saveRetainedShortcut(t *testing.T, s *Server) string {
	t.Helper()
	state := s.desktopWorkspace.Workspace()
	saved, err := s.desktopWorkspace.Replace(desktopworkspace.ReplaceInput{ExpectedResourceVersion: state.ResourceVersion, Shortcuts: []desktopworkspace.ShortcutInput{{ID: panelTestShortcutID, Name: "Test command", TargetType: desktopworkspace.ShortcutTargetCommand, Command: "printf secret-test-command"}}})
	if err != nil {
		t.Fatal(err)
	}
	return saved.ResourceVersion
}
func retainedShortcutBody(version string) []byte {
	data, _ := json.Marshal(map[string]any{"hostId": "local", "rows": 24, "columns": 80, "retained": true, "shortcutId": panelTestShortcutID, "shortcutVersion": version})
	return data
}
func TestRetainedTerminalListIsAuthenticatedOwnedAndDoesNotExtendLifetime(t *testing.T) {
	s, path := newTestServer(t)
	s.agent = &retainedAgentStub{}
	cookie, csrf := bootstrapCookies(t, s, path)
	if got := performRequest(s, http.MethodGet, "/api/v1/terminal-sessions", nil, nil); got.Code != 401 {
		t.Fatalf("unauthenticated list: %d", got.Code)
	}
	opened := authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions", []byte(`{"hostId":"local","rows":24,"columns":80,"retained":true}`), cookie, csrf, retainedHeaders(csrf))
	if opened.Code != 201 {
		t.Fatalf("open: %d %s", opened.Code, opened.Body)
	}
	var item terminalOpenResponse
	_ = json.Unmarshal(opened.Body.Bytes(), &item)
	if !item.Retained || item.ExpiresAt == nil {
		t.Fatalf("missing retention: %+v", item)
	}
	s.terminalMu.Lock()
	original := s.terminalSessions[item.SessionID]
	s.terminalSessions["foreign"] = panelTerminalSession{ID: "foreign", UserID: "another", Retained: true}
	s.terminalSessions["temporary"] = panelTerminalSession{ID: "temporary", UserID: original.UserID}
	s.terminalMu.Unlock()
	got := authenticatedRequest(s, http.MethodGet, "/api/v1/terminal-sessions", nil, cookie, csrf, nil)
	var listed struct {
		Items []terminalOpenResponse `json:"items"`
	}
	_ = json.Unmarshal(got.Body.Bytes(), &listed)
	if got.Code != 200 || len(listed.Items) != 1 || listed.Items[0].SessionID != item.SessionID {
		t.Fatalf("list: %d %s", got.Code, got.Body)
	}
	if strings.Contains(got.Body.String(), "backend-terminal") || strings.Contains(got.Body.String(), "panel:") {
		t.Fatal("backend identity leaked")
	}
	s.terminalMu.Lock()
	updated := s.terminalSessions[item.SessionID].UpdatedAt
	s.terminalMu.Unlock()
	if !updated.Equal(original.UpdatedAt) {
		t.Fatal("listing extended lifetime")
	}
	bad := authenticatedRequest(s, http.MethodGet, "/api/v1/terminal-sessions?owner=another", nil, cookie, csrf, nil)
	if bad.Code != 400 {
		t.Fatalf("query: %d", bad.Code)
	}
}
func TestRetainedShortcutConcurrentRefreshExecutesOnce(t *testing.T) {
	s, path := newTestServer(t)
	stub := &retainedAgentStub{}
	s.agent = stub
	cookie, csrf := bootstrapCookies(t, s, path)
	version := saveRetainedShortcut(t, s)
	var wg sync.WaitGroup
	ids := make(chan string, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions", retainedShortcutBody(version), cookie, csrf, retainedHeaders(csrf))
			if got.Code != 200 && got.Code != 201 {
				t.Errorf("open: %d %s", got.Code, got.Body)
				return
			}
			var result terminalOpenResponse
			_ = json.Unmarshal(got.Body.Bytes(), &result)
			ids <- result.SessionID
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for next := range ids {
		if id != "" && next != id {
			t.Fatal("refresh created a duplicate")
		}
		id = next
	}
	if stub.opened != 1 || len(stub.inputs) != 1 || string(stub.inputs[0]) != "printf secret-test-command\n" {
		t.Fatalf("opened=%d inputs=%q", stub.opened, stub.inputs)
	}
	got := authenticatedRequest(s, http.MethodGet, "/api/v1/terminal-sessions", nil, cookie, csrf, nil)
	if strings.Contains(got.Body.String(), "secret-test-command") {
		t.Fatal("command leaked into registry response")
	}
	events, _, _ := s.store.ListAudit(20, "")
	audit, _ := json.Marshal(events)
	if strings.Contains(string(audit), "secret-test-command") {
		t.Fatal("command leaked into audit")
	}
}
func TestRetainedShortcutValidationAndUncertainLaunchNeverReplay(t *testing.T) {
	s, path := newTestServer(t)
	stub := &retainedAgentStub{failInput: true}
	s.agent = stub
	cookie, csrf := bootstrapCookies(t, s, path)
	version := saveRetainedShortcut(t, s)
	for _, headers := range []map[string]string{nil, {"Content-Type": "application/json", "Origin": "http://panel.test"}} {
		got := authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions", retainedShortcutBody(version), cookie, csrf, headers)
		if got.Code != 403 {
			t.Fatalf("unguarded open: %d", got.Code)
		}
	}
	got := authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions", retainedShortcutBody("stale"), cookie, csrf, retainedHeaders(csrf))
	if got.Code != 409 || stub.opened != 0 {
		t.Fatalf("stale shortcut: %d opened%d", got.Code, stub.opened)
	}
	got = authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions", retainedShortcutBody(version), cookie, csrf, retainedHeaders(csrf))
	var item terminalOpenResponse
	_ = json.Unmarshal(got.Body.Bytes(), &item)
	if got.Code != 201 || !item.LaunchUncertain {
		t.Fatalf("uncertain: %d %s", got.Code, got.Body)
	}
	stub.failInput = false
	retry := authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions", retainedShortcutBody(version), cookie, csrf, retainedHeaders(csrf))
	if retry.Code != 200 || stub.opened != 1 || len(stub.inputs) != 0 {
		t.Fatalf("uncertain replayed: %d opened%d inputs%d", retry.Code, stub.opened, len(stub.inputs))
	}
	input := authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions/"+item.SessionID+"/input", []byte(`{"data":"eAo"}`), cookie, csrf, retainedHeaders(csrf))
	if input.Code != 409 {
		t.Fatalf("uncertain input not fenced: %d", input.Code)
	}
	stub.failClose = true
	closed := authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions/"+item.SessionID+"/close", []byte(`{}`), cookie, csrf, retainedHeaders(csrf))
	if closed.Code != 502 {
		t.Fatalf("close: %d", closed.Code)
	}
	retry = authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions", retainedShortcutBody(version), cookie, csrf, retainedHeaders(csrf))
	if retry.Code != 409 || stub.opened != 1 {
		t.Fatal("failed close replaced a live session")
	}
}
func TestRetainedAgentNegotiationRejectsOldAgent(t *testing.T) {
	s, path := newTestServer(t)
	stub := &retainedAgentStub{unsupported: true}
	s.agent = stub
	cookie, csrf := bootstrapCookies(t, s, path)
	got := authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions", []byte(`{"hostId":"local","rows":24,"columns":80,"retained":true}`), cookie, csrf, retainedHeaders(csrf))
	if got.Code < 400 || stub.closed != 1 || len(s.terminalSessions) != 0 {
		t.Fatalf("old Agent silently downgraded: %d closed%d", got.Code, stub.closed)
	}
}
func TestRetainedRegistrySkipsIdleButKeepsUnconfirmedExpiry(t *testing.T) {
	s, _ := newTestServer(t)
	now := time.Now().UTC()
	s.terminalSessions["retained"] = panelTerminalSession{ID: "retained", Retained: true, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	if got := s.pruneTerminalSessions(now.Add(-panelTerminalIdleTTL)); len(got) != 0 {
		t.Fatal("background session idle-reaped")
	}
	s.terminalSessions["expired"] = panelTerminalSession{ID: "expired", Retained: true, CreatedAt: now.Add(-terminal.DefaultLifetime - time.Minute), UpdatedAt: now}
	got := s.pruneTerminalSessions(now.Add(-panelTerminalIdleTTL))
	if len(got) != 1 || !s.terminalSessions["expired"].CloseRequested {
		t.Fatal("expiry lost unconfirmed cleanup identity")
	}
}

func TestRetainedShortcutInputFrozenUntilLaunchAcknowledged(t *testing.T) {
	s, path := newTestServer(t)
	cookie, csrf := bootstrapCookies(t, s, path)
	version := saveRetainedShortcut(t, s)
	checked := false
	s.agent = &retainedAgentStub{beforeInput: func() {
		s.terminalMu.Lock()
		var id string
		for _, item := range s.terminalSessions {
			if item.ShortcutID == panelTestShortcutID {
				id = item.ID
				if !item.LaunchUncertain {
					t.Error("launch exposed before acknowledgement")
				}
			}
		}
		s.terminalMu.Unlock()
		if id == "" {
			t.Fatal("launch not registered")
		}
		got := authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions/"+id+"/input-transport", []byte(`{}`), cookie, csrf, retainedHeaders(csrf))
		if got.Code != 409 {
			t.Errorf("concurrent claim: %d %s", got.Code, got.Body)
		}
		checked = true
	}}
	got := authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions", retainedShortcutBody(version), cookie, csrf, retainedHeaders(csrf))
	var result terminalOpenResponse
	_ = json.Unmarshal(got.Body.Bytes(), &result)
	if !checked || got.Code != 201 || result.LaunchUncertain {
		t.Fatalf("ack: checked=%v response=%d %s", checked, got.Code, got.Body)
	}
}
