package webbrowser

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestRealBrowserNavigationCanStopAndPopupSelfClose(t *testing.T) {
	executable := os.Getenv("KPANEL_TEST_CHROME")
	if executable == "" {
		t.Skip("set KPANEL_TEST_CHROME")
	}
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/hung":
			<-r.Context().Done()
		case "/slow":
			_, _ = w.Write([]byte(`<title>Slow</title><img src="/hung">`))
			w.(http.Flusher).Flush()
		case "/popup":
			_, _ = w.Write([]byte(`<title>Popup</title><script>setTimeout(()=>window.close(),1000)</script>`))
		default:
			_, _ = w.Write([]byte(`<title>Main</title><button style="position:absolute;left:0;top:0;width:200px;height:70px" onclick="window.open('/popup')">Popup</button>`))
		}
	}))
	defer app.Close()
	s := newBrowserSession(context.Background())
	defer s.Close()
	ranges := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	execute := func(c Command) {
		t.Helper()
		if err := s.Execute(c, executable, ranges); err != nil {
			t.Fatal(err)
		}
	}
	wait := func(match func(Event) bool) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if match(s.Snapshot()) {
				return
			}
			time.Sleep(30 * time.Millisecond)
		}
		t.Fatal("browser condition not reached")
	}
	execute(Command{Type: "new", URL: app.URL + "/slow"})
	wait(func(e Event) bool { return len(e.Tabs) == 1 && e.Tabs[0].Title == "Slow" && e.Tabs[0].Loading })
	started := time.Now()
	execute(Command{Type: "stop"})
	if time.Since(started) > 3*time.Second {
		t.Fatal("stop was blocked by navigation")
	}
	execute(Command{Type: "navigate", URL: app.URL})
	wait(func(e Event) bool { return len(e.Tabs) == 1 && e.Tabs[0].Title == "Main" && !e.Tabs[0].Loading })
	execute(Command{Type: "mouse", Action: "mousePressed", Button: "left", X: 50, Y: 30})
	execute(Command{Type: "mouse", Action: "mouseReleased", Button: "left", X: 50, Y: 30})
	wait(func(e Event) bool { return len(e.Tabs) == 2 })
	wait(func(e Event) bool { return len(e.Tabs) == 1 && e.Active == e.Tabs[0].ID })
	execute(Command{Type: "new", URL: app.URL})
	wait(func(e Event) bool { return len(e.Tabs) == 2 && e.Tabs[1].Title == "Main" })
	s.mu.Lock()
	streaming := s.selected("").streaming
	s.mu.Unlock()
	if !streaming {
		t.Fatal("closed popup prevented next tab stream")
	}
}

func TestRealBrowserRoundTrip(t *testing.T) {
	executable := os.Getenv("KPANEL_TEST_CHROME")
	if executable == "" {
		t.Skip("set KPANEL_TEST_CHROME to an isolated test browser executable")
	}
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><title>Fixture</title><style>body{margin:0}input{position:absolute;left:20px;top:20px;width:300px;height:40px}</style><input oninput="document.title=this.value"><a style="position:absolute;top:100px" href="/next">Next</a>`))
	}))
	defer app.Close()
	service, err := NewService(executable, strings.Repeat("t", 48), []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	server := httptest.NewServer(service)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/sessions/"+strings.Repeat("a", 64), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + service.Token}}})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	ws.SetReadLimit(MaxEventBytes)
	sendCommand := func(c Command) {
		t.Helper()
		data, _ := json.Marshal(c)
		if ws.Write(ctx, websocket.MessageText, data) != nil {
			t.Fatal("write failed")
		}
	}
	readUntil := func(match func(Event) bool) Event {
		t.Helper()
		for {
			_, data, err := ws.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var event Event
			if json.Unmarshal(data, &event) != nil {
				t.Fatal("invalid event")
			}
			if event.Type == "error" {
				t.Fatal(event.Code)
			}
			if match(event) {
				return event
			}
		}
	}
	readUntil(func(e Event) bool { return e.Type == "state" })
	sendCommand(Command{Type: "new", URL: app.URL})
	state := readUntil(func(e Event) bool { return e.Type == "state" && len(e.Tabs) > 0 && e.Tabs[0].Title == "Fixture" })
	frame := readUntil(func(e Event) bool { return e.Type == "frame" })
	data, _ := base64.StdEncoding.DecodeString(frame.Image)
	image, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil || image.Bounds().Dx() < 300 {
		t.Fatal("invalid browser frame", err)
	}
	id := state.Active
	sendCommand(Command{Type: "mouse", Tab: id, Action: "mousePressed", Button: "left", X: 60, Y: 40})
	sendCommand(Command{Type: "mouse", Tab: id, Action: "mouseReleased", Button: "left", X: 60, Y: 40})
	sendCommand(Command{Type: "text", Tab: id, Text: "中文测试"})
	readUntil(func(e Event) bool { return e.Type == "state" && len(e.Tabs) > 0 && e.Tabs[0].Title == "中文测试" })
	sendCommand(Command{Type: "navigate", Tab: id, URL: app.URL + "/next"})
	readUntil(func(e Event) bool {
		return e.Type == "state" && len(e.Tabs) > 0 && strings.HasSuffix(e.Tabs[0].URL, "/next") && e.Tabs[0].Back
	})
	sendCommand(Command{Type: "back", Tab: id})
	readUntil(func(e Event) bool {
		return e.Type == "state" && len(e.Tabs) > 0 && !strings.HasSuffix(e.Tabs[0].URL, "/next") && e.Tabs[0].Forward
	})
	sendCommand(Command{Type: "resize", Width: 640, Height: 480})
	readUntil(func(e Event) bool { return e.Type == "frame" && e.Width == 640 && e.Height == 480 })
	sendCommand(Command{Type: "close", Tab: id})
	readUntil(func(e Event) bool { return e.Type == "state" && len(e.Tabs) == 0 })
	service.mu.Lock()
	entry := service.sessions[strings.Repeat("a", 64)]
	service.mu.Unlock()
	entry.browser.mu.Lock()
	running := entry.browser.root != nil
	entry.browser.mu.Unlock()
	if running {
		t.Fatal("last tab retained browser process")
	}
	sendCommand(Command{Type: "end"})
}
