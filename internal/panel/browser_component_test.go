package panel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowserComponentRequiresSessionOriginCSRFTypedVersionedAction(t *testing.T) {
	s, path := newTestServer(t)
	session, csrf := bootstrapCookies(t, s, path)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		r := httptest.NewRequest(method, "/api/v1/browser/component", nil)
		r.Host = "panel.test"
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatal("anonymous component access", w.Code)
		}
	}
	body := []byte(`{"action":"install","expectedResourceVersion":"sha256:` + strings.Repeat("a", 64) + `"}`)
	if w := authenticatedSiteRequest(s, session, csrf, "POST", "/api/v1/browser/component/actions", body, false); w.Code != 403 {
		t.Fatal("missing CSRF accepted", w.Code)
	}
	if w := authenticatedRequest(s, "POST", "/api/v1/browser/component/actions", body, session, csrf, map[string]string{"Origin": "http://evil.test", "Content-Type": "application/json", "X-CSRF-Token": csrf.Value}); w.Code != 403 {
		t.Fatal("foreign origin accepted", w.Code)
	}
	for _, bad := range []string{`{"action":"sh","expectedResourceVersion":"sha256:` + strings.Repeat("a", 64) + `"}`, `{"action":"install","expectedResourceVersion":"stale"}`, `{"action":"install","command":"touch /tmp/x"}`} {
		w := authenticatedSiteRequest(s, session, csrf, "POST", "/api/v1/browser/component/actions", []byte(bad), true)
		if w.Code != 400 && w.Code != 422 {
			t.Fatal("invalid request accepted", w.Code, w.Body.String())
		}
	}
}
