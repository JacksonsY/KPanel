package panel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/store"
)

func TestCombinedAuthStatusPreservesSessionAndCSRFSecurity(t *testing.T) {
	s, tokenPath := newTestServer(t)
	check := func(cookies ...*http.Cookie) (bool, *authResponse) {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "http://panel.test/api/v1/auth/bootstrap", nil)
		for _, cookie := range cookies {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("unexpected status/cache: %d %v", w.Code, w.Header())
		}
		var body struct {
			Authenticated bool          `json:"authenticated"`
			Session       *authResponse `json:"session"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"passwordHash", "tokenHash", "totpSecret", "recoveryCode"} {
			if strings.Contains(w.Body.String(), secret) {
				t.Fatalf("private credential field leaked: %s", secret)
			}
		}
		return body.Authenticated, body.Session
	}
	if authenticated, session := check(); authenticated || session != nil {
		t.Fatal("uninitialized panel exposed session")
	}
	sessionCookie, csrfCookie := bootstrapCookies(t, s, tokenPath)
	if authenticated, session := check(); authenticated || session != nil {
		t.Fatal("guest exposed session")
	}
	if authenticated, session := check(&http.Cookie{Name: s.config.CookieName, Value: "forged"}); authenticated || session != nil {
		t.Fatal("forged session accepted")
	}
	if authenticated, session := check(sessionCookie, csrfCookie); !authenticated || session == nil || session.CSRFToken != csrfCookie.Value || session.User.Username != "admin" {
		t.Fatal("valid session not restored")
	}
	if authenticated, session := check(sessionCookie); !authenticated || session == nil || session.CSRFToken != "" {
		t.Fatal("missing CSRF cookie created write authority")
	}
	if authenticated, session := check(sessionCookie, &http.Cookie{Name: csrfCookie.Name, Value: "stale"}); !authenticated || session == nil || session.CSRFToken != "" {
		t.Fatal("stale CSRF cookie created write authority")
	}
	if err := s.auth.Logout(sessionCookie.Value); err != nil {
		t.Fatal(err)
	}
	if authenticated, session := check(sessionCookie, csrfCookie); authenticated || session != nil {
		t.Fatal("revoked session accepted")
	}
	// Expiry is checked by the same authenticator as the legacy session route.
	credentials, err := s.auth.Login("192.0.2.1", "admin", "a-strong-password-1", "")
	if err != nil {
		t.Fatal(err)
	}
	active, err := s.auth.Authenticate(credentials.Token)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.PutSession(store.Session{TokenHash: active.TokenHash, CSRFHash: active.CSRFHash,
		UserID: active.User.ID, CreatedAt: time.Now().Add(-2 * time.Hour), ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if authenticated, session := check(&http.Cookie{Name: sessionCookie.Name, Value: credentials.Token}, csrfCookie); authenticated || session != nil {
		t.Fatal("expired session accepted")
	}
}
