package panel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVMRoutesRequireSessionAndExactPaths(t *testing.T) {
	s, path := newTestServer(t)
	session, csrf := bootstrapCookies(t, s, path)
	a := &stubAgent{response: AgentResponse{StatusCode: 200, ContentType: "application/json", Body: []byte(`{"machines":[]}`)}}
	s.agent = a
	r := httptest.NewRequest(http.MethodGet, "/api/v1/virtual-machines", nil)
	r.Host = "panel.test"
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 401 || len(a.snapshotCalls()) != 0 {
		t.Fatal("unauthenticated VM read reached Agent")
	}
	w = authenticatedSiteRequest(s, session, csrf, http.MethodGet, "/api/v1/virtual-machines", nil, false)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, target := range []string{"/api/v1/virtual-machines?all=true", "/api/v1/virtual-machines/../../system", "/api/v1/virtual-machines/not-a-uuid/console"} {
		w = authenticatedSiteRequest(s, session, csrf, http.MethodGet, target, nil, false)
		if w.Code != 404 {
			t.Fatalf("accepted %s: %d", target, w.Code)
		}
	}
	if len(a.snapshotCalls()) != 1 {
		t.Fatal("invalid VM path reached Agent")
	}
}

func TestVMActionsRequireOriginCSRFAndValidateBeforeForwarding(t *testing.T) {
	s, path := newTestServer(t)
	session, csrf := bootstrapCookies(t, s, path)
	a := &stubAgent{response: AgentResponse{StatusCode: 202, ContentType: "application/json", Body: []byte(`{"status":"queued"}`)}}
	s.agent = a
	body := []byte(`{"action":"install","requestId":"11111111-1111-1111-1111-111111111111"}`)
	w := authenticatedSiteRequest(s, session, csrf, http.MethodPost, "/api/v1/virtual-machines/actions", body, false)
	if w.Code != 403 || len(a.snapshotCalls()) != 0 {
		t.Fatal("missing CSRF reached Agent")
	}
	r := newAuthenticatedSiteRequest(session, csrf, http.MethodPost, "/api/v1/virtual-machines/actions", body, true)
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 || len(a.snapshotCalls()) != 0 {
		t.Fatal("cross-origin write reached Agent")
	}
	for _, bad := range []string{`{"action":"exec","requestId":"11111111-1111-1111-1111-111111111111"}`, `{"action":"install","requestId":"11111111-1111-1111-1111-111111111111","command":"reboot"}`} {
		w = authenticatedSiteRequest(s, session, csrf, http.MethodPost, "/api/v1/virtual-machines/actions", []byte(bad), true)
		if w.Code < 400 {
			t.Fatal("accepted malformed action")
		}
	}
	if len(a.snapshotCalls()) != 0 {
		t.Fatal("invalid action reached Agent")
	}
	w = authenticatedSiteRequest(s, session, csrf, http.MethodPost, "/api/v1/virtual-machines/actions", body, true)
	if w.Code != 202 || len(a.snapshotCalls()) != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	if strings.Contains(a.snapshotCalls()[0].path, "command") {
		t.Fatal("unexpected command transport")
	}
}
