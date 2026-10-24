package panel

import (
	"encoding/json"
	"github.com/kejilion/kejilion-panel/internal/sitetraffic"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSiteTrafficAPIBoundaries(t *testing.T) {
	s, token := newTestServer(t)
	if got := performRequest(s, "GET", "/api/v1/site-traffic", nil, nil); got.Code != 401 {
		t.Fatalf("anonymous %d", got.Code)
	}
	session, csrf := bootstrapCookies(t, s, token)
	call := func(method, path, body, origin, csrfValue string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Host = "panel.test"
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrfValue)
		req.AddCookie(session)
		req.AddCookie(csrf)
		out := httptest.NewRecorder()
		s.ServeHTTP(out, req)
		return out
	}
	got := call("GET", "/api/v1/site-traffic", "", "http://panel.test", csrf.Value)
	var snapshot sitetraffic.Snapshot
	if got.Code != 200 || json.Unmarshal(got.Body.Bytes(), &snapshot) != nil || snapshot.Enabled || snapshot.Requests != 0 {
		t.Fatalf("initial %d %s", got.Code, got.Body.String())
	}
	for _, tc := range []struct {
		method, path, body, origin, token string
		status                            int
	}{
		{"POST", "/api/v1/site-traffic/actions", `{"action":"enable"}`, "http://panel.test", "", 403},
		{"POST", "/api/v1/site-traffic/actions", `{"action":"enable"}`, "https://attacker.example", csrf.Value, 403},
		{"POST", "/api/v1/site-traffic/actions", `{"action":"enable","path":"/etc/passwd"}`, "http://panel.test", csrf.Value, 422},
		{"POST", "/api/v1/site-traffic/actions", `{"action":"clear"} {}`, "http://panel.test", csrf.Value, 422},
		{"POST", "/api/v1/site-traffic/actions?x=1", `{"action":"clear"}`, "http://panel.test", csrf.Value, 422},
		{"GET", "/api/v1/site-traffic?period=24h&period=30d", "", "http://panel.test", csrf.Value, 422},
		{"GET", "/api/v1/site-traffic?period=year", "", "http://panel.test", csrf.Value, 422},
		{"GET", "/api/v1/site-traffic?hostId=remote", "", "http://panel.test", csrf.Value, 422},
		{"GET", "/api/v1/site-traffic/actions", "", "http://panel.test", csrf.Value, 405},
	} {
		if got := call(tc.method, tc.path, tc.body, tc.origin, tc.token); got.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, got.Code, got.Body.String())
		}
	}
	for _, action := range []string{"enable", "pause", "clear"} {
		got := call("POST", "/api/v1/site-traffic/actions", `{"action":"`+action+`"}`, "http://panel.test", csrf.Value)
		if got.Code != 200 {
			t.Fatalf("%s: %d %s", action, got.Code, got.Body.String())
		}
	}
	audits, _, _ := s.store.ListAudit(20, "")
	found := false
	for _, e := range audits {
		if e.Action == "site.monitor.clear" && e.Result == "success" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing control audit")
	}

}
