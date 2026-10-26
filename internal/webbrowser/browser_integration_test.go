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

	"github.com/chromedp/chromedp"
	"github.com/coder/websocket"
)

func TestRealBrowserWheelAndViewport(t *testing.T) {
	executable := os.Getenv("KPANEL_TEST_CHROME")
	if executable == "" {
		t.Skip("set KPANEL_TEST_CHROME")
	}
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<title>Scroll</title><style>body{margin:0;width:3000px;height:4000px;background:linear-gradient(red,blue)}</style>`))
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
	execute(Command{Type: "resize", Width: 1088, Height: 560})
	execute(Command{Type: "new", URL: app.URL})
	deadline := time.Now().Add(8 * time.Second)
	for s.Snapshot().Tabs[0].Loading && time.Now().Before(deadline) {
		time.Sleep(30 * time.Millisecond)
	}
	first := s.active
	tab := s.selected(first)
	for _, delta := range [][2]float64{{0, 400}, {300, 0}, {0, -200}, {-150, 0}} {
		before, err := chromedp.Run(tab.ctx, chromedp.Evaluate[[2]float64](`[scrollX,scrollY]`))
		if err != nil {
			t.Fatal(err)
		}
		var after [2]float64

		execute(Command{Type: "mouse", Action: "mouseWheel", Button: "none", X: 200, Y: 200, DeltaX: delta[0], DeltaY: delta[1]})
		deadline = time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			after, err = chromedp.Run(tab.ctx, chromedp.Evaluate[[2]float64](`[scrollX,scrollY]`))
			if err != nil {
				t.Fatal(err)
			}
			if after != before {
				break
			}
			time.Sleep(30 * time.Millisecond)
		}
		if after == before {
			t.Fatalf("wheel %v did not move page: %v", delta, after)
		}
	}
	execute(Command{Type: "new", URL: app.URL})
	execute(Command{Type: "resize", Width: 1200, Height: 700})
	deadline = time.Now().Add(8 * time.Second)
	for s.Snapshot().Tabs[1].Loading && time.Now().Before(deadline) {
		time.Sleep(30 * time.Millisecond)
	}
	execute(Command{Type: "resize", Width: 1400, Height: 800})
	execute(Command{Type: "select", Tab: first})
	size, err := chromedp.Run(tab.ctx, chromedp.Evaluate[[2]int](`[innerWidth,innerHeight]`))
	if err != nil {
		t.Fatal(err)
	}
	if size != [2]int{1400, 800} {
		t.Fatalf("selected tab kept stale viewport: %v", size)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("resized tab did not produce a full viewport frame")
		case frame := <-s.frames:
			if frame.Tab != first || frame.Width != 1400 || frame.Height != 800 {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(frame.Image)
			if err != nil {
				t.Fatal(err)
			}
			image, err := jpeg.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if image.Width != 1400 || image.Height != 800 {
				t.Fatalf("capture scaled inside viewport: %dx%d", image.Width, image.Height)
			}
			return
		}
	}
}

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
	frameContext, stopFrames := context.WithCancel(context.Background())
	defer stopFrames()
	frames := make(chan Event, 1)
	go func() {
		for {
			select {
			case <-frameContext.Done():
				return
			case event := <-s.frames:
				select {
				case <-frames:
				default:
				}
				select {
				case frames <- event:
				default:
				}
			}
		}
	}()
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
	active := s.Snapshot().Active
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatal("closed popup prevented next tab frame")
			return
		case frame := <-frames:
			if frame.Tab != active {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(frame.Image)
			if err != nil {
				t.Fatal(err)
			}
			image, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil || image.Bounds().Dx() < 320 {
				t.Fatal("invalid new tab frame", err)
			}
			return
		}
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
	for i := 0; i < 8; i++ {
		sendCommand(Command{Type: "new", URL: app.URL})
	}
	state = readUntil(func(e Event) bool { return e.Type == "state" && len(e.Tabs) == 8 })
	if err := entry.browser.Execute(Command{Type: "new", URL: app.URL}, executable, nil); err == nil || err.Error() != "tab_limit" {
		t.Fatal("ninth tab did not respect the eight-tab limit", err)
	}
	sendCommand(Command{Type: "select", Tab: state.Tabs[0].ID})
	readUntil(func(e Event) bool { return e.Type == "frame" && e.Tab == state.Tabs[0].ID })
	sendCommand(Command{Type: "end"})
}
