package panel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

func TestSystemServiceReadRoutesAuthenticateAndPreserveExactUnits(t *testing.T) {
	server, tokenPath := newTestServer(t)
	session, csrf := bootstrapCookies(t, server, tokenPath)
	agent := &stubAgent{response: AgentResponse{StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"supported":true,"services":[]}`)}}
	server.agent = agent
	request := httptest.NewRequest(http.MethodGet, "/api/v1/system/services", nil)
	request.Host = "panel.test"
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || len(agent.snapshotCalls()) != 0 {
		t.Fatal("unauthenticated service read reached Agent")
	}
	for _, path := range []string{"/api/v1/system/services", "/api/v1/system/services/worker%40blue.service", "/api/v1/system/services/path%5Cx2dworker.service/logs"} {
		response = authenticatedSiteRequest(server, session, csrf, http.MethodGet, path, nil, false)
		if response.Code != http.StatusOK {
			t.Fatalf("read %s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	calls := agent.snapshotCalls()
	if len(calls) != 3 || calls[1].path != "/v1/system/services/worker@blue.service" || calls[2].path != "/v1/system/services/path%5Cx2dworker.service/logs" {
		t.Fatalf("forwarded paths=%#v", calls)
	}
	for _, path := range []string{"/api/v1/system/services?all=true", "/api/v1/system/services/*.service", "/api/v1/system/services/a%2Fssh.service", "/api/v1/system/services/ssh.service/logs?lines=500", "/api/v1/system/services/ssh.service/other"} {
		response = authenticatedSiteRequest(server, session, csrf, http.MethodGet, path, nil, false)
		if response.Code != http.StatusNotFound {
			t.Errorf("invalid path %s status=%d", path, response.Code)
		}
	}
	if len(agent.snapshotCalls()) != 3 {
		t.Fatal("invalid path reached Agent")
	}
}

func TestSystemServiceActionRequiresSessionOriginCSRFAndAuditsAcceptance(t *testing.T) {
	server, tokenPath := newTestServer(t)
	session, csrf := bootstrapCookies(t, server, tokenPath)
	agent := &stubAgent{response: AgentResponse{StatusCode: http.StatusAccepted, ContentType: "application/json", Body: []byte(`{"name":"kejilion-agent.service","action":"stop","accepted":true,"message":"queued"}`)}}
	server.agent = agent
	body := []byte(`{"name":"kejilion-agent.service","action":"stop","expectedResourceVersion":"` + strings.Repeat("a", 64) + `"}`)
	response := authenticatedSiteRequest(server, session, csrf, http.MethodPost, "/api/v1/system/service-actions", body, false)
	if response.Code != http.StatusForbidden || len(agent.snapshotCalls()) != 0 {
		t.Fatal("missing CSRF reached Agent")
	}
	request := newAuthenticatedSiteRequest(session, csrf, http.MethodPost, "/api/v1/system/service-actions", body, true)
	request.Header.Set("Origin", "https://evil.example")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || len(agent.snapshotCalls()) != 0 {
		t.Fatal("cross-origin write reached Agent")
	}
	response = authenticatedSiteRequest(server, session, csrf, http.MethodPost, "/api/v1/system/service-actions", body, true)
	if response.Code != http.StatusAccepted {
		t.Fatalf("self service was refused: %d %s", response.Code, response.Body.String())
	}
	calls := agent.snapshotCalls()
	if len(calls) != 1 || calls[0].path != "/v1/system/service-actions" || calls[0].rawQuery != "" {
		t.Fatalf("calls=%#v", calls)
	}
	var input contract.SystemServiceActionRequest
	if err := json.Unmarshal(calls[0].body, &input); err != nil || input.Name != "kejilion-agent.service" || input.Action != "stop" {
		t.Fatalf("input=%#v err=%v", input, err)
	}
	events, _, _ := server.store.ListAudit(100, "")
	found := 0
	for _, event := range events {
		if event.Action != "system.services.stop" {
			continue
		}
		found++
		if event.Result != "intent" && event.Result != "accepted" {
			t.Fatalf("queued operation claimed completion: %#v", event)
		}
	}
	if found != 2 {
		t.Fatalf("audit count=%d", found)
	}
	for _, bad := range []string{`{"name":"*.service","action":"stop","expectedResourceVersion":"` + strings.Repeat("a", 64) + `"}`, `{"name":"ssh.service","action":"mask","expectedResourceVersion":"` + strings.Repeat("a", 64) + `"}`} {
		response = authenticatedSiteRequest(server, session, csrf, http.MethodPost, "/api/v1/system/service-actions", []byte(bad), true)
		if response.Code != http.StatusUnprocessableEntity {
			t.Errorf("bad input status=%d", response.Code)
		}
	}
	if len(agent.snapshotCalls()) != 1 {
		t.Fatal("invalid action reached Agent")
	}
}

func TestSystemServiceFailedOutcomeAuditRemainsUnknown(t *testing.T) {
	server, tokenPath := newTestServer(t)
	session, csrf := bootstrapCookies(t, server, tokenPath)
	server.agent = &stubAgent{response: AgentResponse{StatusCode: http.StatusGatewayTimeout, ContentType: "application/json", Body: []byte(`{"code":"system_services_timeout"}`)}}
	body := []byte(`{"name":"ssh.service","action":"restart","expectedResourceVersion":"` + strings.Repeat("a", 64) + `"}`)
	response := authenticatedSiteRequest(server, session, csrf, http.MethodPost, "/api/v1/system/service-actions", body, true)
	if response.Code != http.StatusGatewayTimeout {
		t.Fatal("timeout was hidden")
	}
	events, _, _ := server.store.ListAudit(100, "")
	found := false
	for _, event := range events {
		if event.Action == "system.services.restart" && event.Result == "unknown" {
			found = true
		}
	}
	if !found {
		t.Fatal("uncertain mutation outcome was not recorded")
	}
}
