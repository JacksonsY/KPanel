package webbrowser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestServiceAuthenticationCapacityAndExpiry(t *testing.T) {
	executable, _ := os.Executable()
	service, err := NewService(executable, strings.Repeat("s", 48), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	for _, header := range []http.Header{{}, {"Authorization": {"Bearer wrong"}}, {"Authorization": {"Bearer " + service.Token}, "Origin": {"https://attacker.invalid"}}} {
		request := httptest.NewRequest("GET", "/healthz", nil)
		request.Header = header
		response := httptest.NewRecorder()
		service.ServeHTTP(response, request)
		if response.Code != 401 {
			t.Fatal("unauthorized service access", response.Code)
		}
	}
	server := httptest.NewServer(service)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dial := func(id string) (*websocket.Conn, *http.Response, error) {
		return websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/sessions/"+strings.Repeat(id, 64), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + service.Token}}})
	}
	first, _, err := dial("a")
	if err != nil {
		t.Fatal(err)
	}
	defer first.CloseNow()
	if conn, response, err := dial("a"); err == nil {
		conn.CloseNow()
		t.Fatal("duplicate attachment")
	} else if response.StatusCode != 409 {
		t.Fatal(response.StatusCode)
	}
	second, _, err := dial("b")
	if err != nil {
		t.Fatal(err)
	}
	defer second.CloseNow()
	if conn, response, err := dial("c"); err == nil {
		conn.CloseNow()
		t.Fatal("unbounded sessions")
	} else if response.StatusCode != 429 {
		t.Fatal(response.StatusCode)
	}
	service.mu.Lock()
	retained := service.sessions[strings.Repeat("a", 64)]
	expired := service.sessions[strings.Repeat("b", 64)]
	expired.attached = false
	expired.expires = time.Now().Add(-time.Second)
	service.mu.Unlock()
	service.expire(time.Now())
	service.mu.Lock()
	count := len(service.sessions)
	service.mu.Unlock()
	if count != 1 || retained.browser.ctx.Err() != nil || expired.browser.ctx.Err() == nil {
		t.Fatal("expiry crossed active-session boundary")
	}
}
