package webbrowser

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

var sessionIDPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Service struct {
	Token      string
	Executable string
	Private    []netip.Prefix
	mu         sync.Mutex
	sessions   map[string]*serviceSession
	ctx        context.Context
	cancel     context.CancelFunc
}
type serviceSession struct {
	browser  *browserSession
	attached bool
	expires  time.Time
}

func NewService(executable, token string, private []netip.Prefix) (*Service, error) {
	if len(token) < 32 || len(token) > 256 || strings.ContainsAny(token, "\r\n\x00") {
		return nil, errors.New("browser token must contain 32-256 bytes")
	}
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("browser executable is unavailable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{Token: token, Executable: executable, Private: private, sessions: make(map[string]*serviceSession), ctx: ctx, cancel: cancel}
	go s.reap()
	return s, nil
}

func (s *Service) reap() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-ticker.C:
			s.expire(now)
		}
	}
}
func (s *Service) expire(now time.Time) {
	var expired []*browserSession
	s.mu.Lock()
	for id, entry := range s.sessions {
		if !entry.attached && now.After(entry.expires) {
			delete(s.sessions, id)
			expired = append(expired, entry.browser)
		}
	}
	s.mu.Unlock()
	for _, browser := range expired {
		browser.Close()
	}
}
func (s *Service) Close() {
	s.cancel()
	s.mu.Lock()
	sessions := s.sessions
	s.sessions = make(map[string]*serviceSession)
	s.mu.Unlock()
	for _, entry := range sessions {
		entry.browser.Close()
	}
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" || r.URL.RawQuery != "" || r.URL.RawPath != "" || r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.Token)) != 1 {
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ready": true, "maxTabs": MaxTabs})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	if !strings.HasPrefix(r.URL.Path, "/v1/sessions/") || !sessionIDPattern.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		http.Error(w, "browser_unavailable", 503)
		return
	}
	entry := s.sessions[id]
	if entry == nil {
		if len(s.sessions) >= 2 {
			s.mu.Unlock()
			http.Error(w, "browser_busy", 429)
			return
		}
		entry = &serviceSession{browser: newBrowserSession(s.ctx)}
		s.sessions[id] = entry
	}
	if entry.attached {
		s.mu.Unlock()
		http.Error(w, "workspace_in_use", 409)
		return
	}
	entry.attached = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		entry.attached = false
		entry.expires = time.Now().Add(90 * time.Second)
		s.mu.Unlock()
		_ = entry.browser.Execute(Command{Type: "visible", Visible: false}, s.Executable, s.Private)
	}()
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	defer ws.CloseNow()
	ws.SetReadLimit(MaxCommandBytes)
	ctx, cancel := context.WithTimeout(entry.browser.ctx, 30*time.Minute)
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); defer cancel(); s.writeEvents(ctx, ws, entry.browser) }()
	entry.browser.signal()
	window := time.Now()
	count := 0
	for {
		kind, data, err := ws.Read(ctx)
		if err != nil {
			break
		}
		if time.Since(window) >= time.Second {
			window = time.Now()
			count = 0
		}
		count++
		if count > 120 {
			break
		}
		if kind != websocket.MessageText {
			break
		}
		command, err := DecodeCommand(data)
		if err == nil && command.Type == "end" {
			s.mu.Lock()
			delete(s.sessions, id)
			s.mu.Unlock()
			entry.browser.Close()
			break
		}
		if err == nil {
			err = entry.browser.Execute(command, s.Executable, s.Private)
		}
		if err != nil {
			entry.browser.notice(Event{Type: "error", Code: err.Error()})
		}
	}
	cancel()
	_ = ws.Close(websocket.StatusNormalClosure, "disconnected")
	<-done
}

func (s *Service) writeEvents(ctx context.Context, ws *websocket.Conn, browser *browserSession) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	refresh := time.NewTicker(2 * time.Second)
	defer refresh.Stop()
	lastActive := ""
	writeEvent := func(event Event) error {
		data, err := json.Marshal(event)
		if err != nil || len(data) > MaxEventBytes {
			return errors.New("invalid_event")
		}
		write, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		return ws.Write(write, websocket.MessageText, data)
	}
	for {
		var event Event
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ping, stop := context.WithTimeout(ctx, 5*time.Second)
			err := ws.Ping(ping)
			stop()
			if err != nil {
				return
			}
			continue
		case <-browser.dirty:
			event = browser.Snapshot()
		case <-refresh.C:
			browser.mu.Lock()
			running := browser.visible && len(browser.tabs) > 0
			browser.mu.Unlock()
			if !running {
				continue
			}
			event = browser.Snapshot()
		case event = <-browser.frames:
		case event = <-browser.notices:
		case event = <-browser.keyframes:
		}
		if event.Type == "frame" {
			if lastActive != event.Tab {
				state := browser.Snapshot()
				if state.Active != event.Tab {
					continue
				}
				if writeEvent(state) != nil {
					return
				}
				lastActive = state.Active
			}
			// Sending state can wait on the client. Recheck after that write so
			// a concurrent hide, resize or selection cannot release a stale frame.
			browser.mu.Lock()
			current := browser.visible && event.Tab == browser.active && event.Width == browser.width && event.Height == browser.height
			browser.mu.Unlock()
			if !current {
				continue
			}
		}
		if event.Type == "state" {
			lastActive = event.Active
		}
		if writeEvent(event) != nil {
			return
		}
	}
}
