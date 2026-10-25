package panel

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

func TestRemoteAccessHostAPIIsAuthenticatedAndIsolatesCleanup(t *testing.T) {
	s, token := newTestServer(t)
	enrollment, err := s.cluster.CreateLightEnrollmentForOrigin("https://panel.test")
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(enrollment.Command)
	host, err := s.cluster.EnrollLightNodeAtOrigin("198.51.100.10", "https://panel.test", cluster.LightEnrollRequest{Token: strings.Trim(fields[len(fields)-1], "'"), Name: "edge-policy", NodeVersion: "0.40.0"})
	if err != nil {
		t.Fatal(err)
	}
	route := "/api/v1/cluster/hosts/" + host.NodeID + "/remote-access"
	if got := performRequest(s, http.MethodGet, route, nil, nil); got.Code != 401 {
		t.Fatal(got.Code, got.Body.String())
	}
	session, csrf := bootstrapCookies(t, s, token)
	before := authenticatedRequest(s, http.MethodGet, route, nil, session, csrf, nil)
	var value cluster.HostRemoteAccess
	if before.Code != 200 || json.Unmarshal(before.Body.Bytes(), &value) != nil || !value.TerminalEnabled || !value.FilesEnabled {
		t.Fatal(before.Code, before.Body.String())
	}
	body := []byte(`{"terminalEnabled":false,"filesEnabled":true,"expectedResourceVersion":"` + value.ResourceVersion + `"}`)
	for _, headers := range []map[string]string{{"Content-Type": "application/json", "Origin": "http://panel.test"}, {"Content-Type": "application/json", "X-CSRF-Token": csrf.Value}} {
		if got := authenticatedRequest(s, http.MethodPut, route, body, session, csrf, headers); got.Code != 403 {
			t.Fatal(got.Code, got.Body.String())
		}
	}
	s.terminalSessions["selected"] = panelTerminalSession{ID: "selected", HostID: host.NodeID, BackendSessionID: "selected-backend"}
	s.terminalSessions["other"] = panelTerminalSession{ID: "other", HostID: "other-host", BackendSessionID: "other-backend"}
	s.terminalSessions["local"] = panelTerminalSession{ID: "local", HostID: cluster.LocalHostID, BackendSessionID: "local-backend"}
	headers := map[string]string{"Content-Type": "application/json", "Origin": "http://panel.test", "X-CSRF-Token": csrf.Value}
	after := authenticatedRequest(s, http.MethodPut, route, body, session, csrf, headers)
	if after.Code != 200 || json.Unmarshal(after.Body.Bytes(), &value) != nil || value.TerminalEnabled || !value.GlobalTerminalEnabled || value.EffectiveTerminalEnabled {
		t.Fatal(after.Code, after.Body.String())
	}
	if !s.terminalSessions["selected"].CloseRequested || s.terminalSessions["other"].CloseRequested || s.terminalSessions["local"].CloseRequested {
		t.Fatal("cleanup affected another host", s.terminalSessions)
	}
	if !s.cluster.RemoteAccess().TerminalEnabled {
		t.Fatal("host switch changed the global switch")
	}
	if got := authenticatedRequest(s, http.MethodPut, route, body, session, csrf, headers); got.Code != 409 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := authenticatedRequest(s, http.MethodGet, "/api/v1/cluster/hosts/local/remote-access", nil, session, csrf, nil); got.Code != 404 {
		t.Fatal(got.Code, got.Body.String())
	}
}
