package panel

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/terminalhistory"
)

func TestTerminalHistoryAPIAppendRemoveClearAndAuditMetadata(t *testing.T) {
	server, tokenPath := newTestServer(t)
	if response := performRequest(server, http.MethodGet, terminalHistoryPath, nil, nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET status = %d", response.Code)
	}
	sessionCookie, csrfCookie := bootstrapCookies(t, server, tokenPath)
	headers := map[string]string{
		"Content-Type": "application/json", "Origin": "http://panel.test", "X-CSRF-Token": csrfCookie.Value,
	}

	body := []byte(`{"items":[{"command":"mysql -p history-secret-command","hostId":"local"},{"command":"df -h","hostId":""}]}`)
	missingOrigin := authenticatedRequest(server, http.MethodPost, terminalHistoryPath, body, sessionCookie, csrfCookie, map[string]string{
		"Content-Type": "application/json", "X-CSRF-Token": csrfCookie.Value,
	})
	if missingOrigin.Code != http.StatusForbidden || !strings.Contains(missingOrigin.Body.String(), "origin_validation_failed") {
		t.Fatalf("missing Origin response = %d %s", missingOrigin.Code, missingOrigin.Body.String())
	}
	missingCSRF := authenticatedRequest(server, http.MethodPost, terminalHistoryPath, body, sessionCookie, csrfCookie, map[string]string{
		"Content-Type": "application/json", "Origin": "http://panel.test",
	})
	if missingCSRF.Code != http.StatusForbidden || !strings.Contains(missingCSRF.Body.String(), "csrf_validation_failed") {
		t.Fatalf("missing CSRF response = %d %s", missingCSRF.Code, missingCSRF.Body.String())
	}

	appended := authenticatedRequest(server, http.MethodPost, terminalHistoryPath, body, sessionCookie, csrfCookie, headers)
	if appended.Code != http.StatusOK {
		t.Fatalf("append failed: %d %s", appended.Code, appended.Body.String())
	}
	var snapshot terminalhistory.Snapshot
	if err := json.Unmarshal(appended.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if !snapshot.Available || len(snapshot.Items) != 2 || snapshot.Items[0].Command != "df -h" || snapshot.Items[1].HostID != "local" {
		t.Fatalf("unexpected history after append: %#v", snapshot)
	}
	loaded := authenticatedRequest(server, http.MethodGet, terminalHistoryPath, nil, sessionCookie, csrfCookie, nil)
	if loaded.Code != http.StatusOK || !strings.Contains(loaded.Body.String(), "history-secret-command") {
		t.Fatalf("history was not returned: %d %s", loaded.Code, loaded.Body.String())
	}

	removed := authenticatedRequest(server, http.MethodPost, terminalHistoryRemovePath, []byte(`{"command":"df -h"}`), sessionCookie, csrfCookie, headers)
	if removed.Code != http.StatusOK || strings.Contains(removed.Body.String(), `"df -h"`) {
		t.Fatalf("remove response = %d %s", removed.Code, removed.Body.String())
	}
	cleared := authenticatedRequest(server, http.MethodDelete, terminalHistoryPath, nil, sessionCookie, csrfCookie, headers)
	if cleared.Code != http.StatusOK || strings.Contains(cleared.Body.String(), "history-secret-command") {
		t.Fatalf("clear response = %d %s", cleared.Code, cleared.Body.String())
	}

	events, _, _ := server.store.ListAudit(50, "")
	auditData, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	audit := string(auditData)
	if !strings.Contains(audit, "terminal.history.clear") || !strings.Contains(audit, "terminal.history.remove") {
		t.Fatalf("history clear/remove were not audited: %s", audit)
	}
	if strings.Contains(audit, "history-secret-command") || strings.Contains(audit, "df -h") || strings.Contains(audit, "terminal.history.append") {
		t.Fatalf("terminal history audit leaked command text or logged appends: %s", audit)
	}
	if panelBackupPath("terminal-history/history.json") {
		t.Fatal("terminal history must stay out of panel backups")
	}
}

func TestTerminalHistoryAPIRejectsInvalidRequests(t *testing.T) {
	server, tokenPath := newTestServer(t)
	sessionCookie, csrfCookie := bootstrapCookies(t, server, tokenPath)
	headers := map[string]string{
		"Content-Type": "application/json", "Origin": "http://panel.test", "X-CSRF-Token": csrfCookie.Value,
	}

	if response := authenticatedRequest(server, http.MethodGet, terminalHistoryPath+"?extra=1", nil, sessionCookie, csrfCookie, nil); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid query status = %d", response.Code)
	}
	wrongMethod := authenticatedRequest(server, http.MethodPut, terminalHistoryPath, []byte(`{}`), sessionCookie, csrfCookie, headers)
	if wrongMethod.Code != http.StatusMethodNotAllowed || wrongMethod.Header().Get("Allow") != "GET, POST, DELETE" {
		t.Fatalf("wrong method response = %d Allow=%q", wrongMethod.Code, wrongMethod.Header().Get("Allow"))
	}
	wrongRemoveMethod := authenticatedRequest(server, http.MethodGet, terminalHistoryRemovePath, nil, sessionCookie, csrfCookie, nil)
	if wrongRemoveMethod.Code != http.StatusMethodNotAllowed || wrongRemoveMethod.Header().Get("Allow") != "POST" {
		t.Fatalf("wrong remove method response = %d Allow=%q", wrongRemoveMethod.Code, wrongRemoveMethod.Header().Get("Allow"))
	}
	for _, body := range []string{
		`{"items":[{"command":"echo\u0000secret","hostId":"local"}]}`,
		`{"items":[{"command":"uptime","hostId":"local","extra":true}]}`,
		`{"items":[]}`,
	} {
		response := authenticatedRequest(server, http.MethodPost, terminalHistoryPath, []byte(body), sessionCookie, csrfCookie, headers)
		if response.Code != http.StatusBadRequest && response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("invalid append %s response = %d %s", body, response.Code, response.Body.String())
		}
	}
	if items := server.terminalHistory.Snapshot().Items; len(items) != 0 {
		t.Fatalf("invalid requests changed history: %#v", items)
	}
}
