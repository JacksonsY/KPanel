package panel

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSystemPackagesReadRequiresSessionAndProxiesSnapshot(t *testing.T) {
	server, tokenPath := newTestServer(t)
	sessionCookie, csrfCookie := bootstrapCookies(t, server, tokenPath)
	body := []byte(`{"manager":"APT","items":[],"maintenance":{"state":"idle"}}`)
	agent := &stubAgent{response: AgentResponse{StatusCode: http.StatusOK, ContentType: "application/json", Body: body}}
	server.agent = agent
	unauthorized := httptest.NewRecorder()
	server.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "http://panel.test/api/v1/system/packages", nil))
	if unauthorized.Code != http.StatusUnauthorized || len(agent.snapshotCalls()) != 0 {
		t.Fatalf("unauthenticated read: status=%d calls=%v", unauthorized.Code, agent.snapshotCalls())
	}
	response := authenticatedSiteRequest(server, sessionCookie, csrfCookie, http.MethodGet, "/api/v1/system/packages", nil, false)
	if response.Code != http.StatusOK || response.Body.String() != string(body) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	calls := agent.snapshotCalls()
	if len(calls) != 1 || calls[0].path != "/v1/system/packages" {
		t.Fatalf("agent calls = %#v", calls)
	}
	if _, allowed := allowedAgentPath("/api/v1/system/packages/unknown"); allowed {
		t.Fatal("unexpected package subroute allowed")
	}
}

func TestSystemPackagesActionRequiresCSRFAndProxiesTypedRequest(t *testing.T) {
	server, tokenPath := newTestServer(t)
	sessionCookie, csrfCookie := bootstrapCookies(t, server, tokenPath)
	agent := &stubAgent{response: AgentResponse{StatusCode: http.StatusAccepted, ContentType: "application/json", Body: []byte(`{"status":"accepted"}`)}}
	server.agent = agent
	body := []byte(`{"action":"install","items":["curl","htop"],"expectedResourceVersion":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`)
	withoutCSRF := authenticatedSiteRequest(server, sessionCookie, csrfCookie, http.MethodPost, "/api/v1/system/packages/actions", body, false)
	if withoutCSRF.Code == http.StatusAccepted {
		t.Fatal("request without CSRF was accepted")
	}
	response := authenticatedSiteRequest(server, sessionCookie, csrfCookie, http.MethodPost, "/api/v1/system/packages/actions", body, true)
	if response.Code < 200 || response.Code >= 300 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	calls := agent.snapshotCalls()
	if len(calls) != 1 || calls[0].path != "/v1/system/packages/actions" {
		t.Fatalf("agent calls = %#v", calls)
	}
}

func TestSystemPackagesActionRejectsArbitraryPackageName(t *testing.T) {
	server, tokenPath := newTestServer(t)
	sessionCookie, csrfCookie := bootstrapCookies(t, server, tokenPath)
	body := []byte(`{"action":"install","items":["curl;reboot"],"expectedResourceVersion":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`)
	response := authenticatedSiteRequest(server, sessionCookie, csrfCookie, http.MethodPost, "/api/v1/system/packages/actions", body, true)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body = %s", response.Code, response.Body.String())
	}
}
