package webbrowser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestResizeSeedKeepsLatestWithoutDroppingNotices(t *testing.T) {
	session := newBrowserSession(context.Background())
	defer session.Close()
	for i := 0; i < cap(session.notices); i++ {
		session.notice(Event{Type: "error"})
	}
	session.mu.Lock()
	session.seedFrame(Event{Type: "frame", Image: "old"})
	session.seedFrame(Event{Type: "frame", Image: "latest"})
	session.mu.Unlock()
	if len(session.keyframes) != 1 || (<-session.keyframes).Image != "latest" || len(session.notices) != cap(session.notices) {
		t.Fatal("resize seed lost or displaced a notification")
	}
}

func TestServiceDiscardsStaleViewportFramesFromBothQueues(t *testing.T) {
	executable, _ := os.Executable()
	service, err := NewService(executable, strings.Repeat("s", 48), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	server := httptest.NewServer(service)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id := strings.Repeat("a", 64)
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/sessions/"+id, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + service.Token}}})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	read := func() Event {
		t.Helper()
		_, data, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var event Event
		if err := json.Unmarshal(data, &event); err != nil {
			t.Fatal(err)
		}
		return event
	}
	if read().Type != "state" {
		t.Fatal("missing initial state")
	}
	service.mu.Lock()
	session := service.sessions[id].browser
	service.mu.Unlock()
	// Transport-only fixture: no Chromium process or fake CDP connection.
	session.mu.Lock()
	session.active, session.width, session.height = "tab-1", 640, 480
	session.mu.Unlock()
	session.signal()
	if state := read(); state.Type != "state" || state.Active != "tab-1" {
		t.Fatal("missing fixture state", state.Type)
	}
	for _, queue := range []chan Event{session.frames, session.notices} {
		for _, frame := range []Event{
			{Type: "frame", Tab: "tab-1", Width: 1280, Height: 720, Image: "stale"},
			{Type: "frame", Tab: "tab-1", Width: 640, Height: 480, Image: "current"},
		} {
			select {
			case queue <- frame:
			case <-ctx.Done():
				t.Fatal("frame delivery blocked")
			}
		}
		for {
			event := read()
			if event.Type != "frame" {
				continue
			}
			if event.Image != "current" || event.Width != 640 || event.Height != 480 {
				t.Fatal("delivered an obsolete viewport after resize")
			}
			break
		}
	}
	session.mu.Lock()
	session.seedFrame(Event{Type: "frame", Tab: "tab-1", Width: 640, Height: 480, Image: "seed"})
	session.mu.Unlock()
	for event := read(); ; event = read() {
		if event.Type == "frame" {
			if event.Image != "seed" {
				t.Fatal("missing current resize seed")
			}
			break
		}
	}
}

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
