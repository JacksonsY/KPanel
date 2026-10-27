package panel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/coder/websocket"
	webbrowser "github.com/kejilion/kejilion-panel/internal/browserprotocol"
)

var browserWorkspacePattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func (s *Server) browserServiceRequest(ctx context.Context, path string) (*http.Request, error) {
	serviceURL, tokenFile := s.config.BrowserServiceURL, s.config.BrowserTokenFile
	implicit := serviceURL == "" && tokenFile == ""
	if implicit {
		markerFile, err := os.Open("/run/kejilion-panel/browser/browser-enabled")
		if err != nil {
			return nil, errors.New("browser_unconfigured")
		}
		marker, err := io.ReadAll(io.LimitReader(markerFile, 8))
		_ = markerFile.Close()
		if err != nil || string(marker) != "enabled" {
			return nil, errors.New("browser_unconfigured")
		}
		serviceURL, tokenFile = "http://browser:8092", "/run/kejilion-panel/browser/browser-token"
	}
	file, err := os.Open(tokenFile)
	if err != nil {
		if implicit && errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("browser_unconfigured")
		}
		return nil, errors.New("browser_unavailable")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 256 {
		return nil, errors.New("browser_unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(file, 257))
	if err != nil {
		return nil, errors.New("browser_unavailable")
	}
	token := strings.TrimSpace(string(data))
	if len(token) < 32 || len(token) > 256 || strings.ContainsAny(token, "\r\n\x00") {
		return nil, errors.New("browser_unavailable")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(serviceURL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	return request, nil
}

func (s *Server) handleBrowser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet || r.URL.RawPath != "" || r.URL.RawQuery != "" {
		s.writeProblem(w, r, 404, "route_not_found", "Route not found", "")
		return
	}
	token, session, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()
	if r.URL.Path == "/api/v1/browser/status" {
		probe, stop := context.WithTimeout(ctx, 3*time.Second)
		defer stop()
		request, err := s.browserServiceRequest(probe, "/healthz")
		if err != nil {
			s.writeJSON(w, 200, map[string]any{"ready": false, "reason": err.Error(), "maxTabs": webbrowser.MaxTabs})
			return
		}
		client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		response, err := client.Do(request)
		ready := err == nil && response.StatusCode == 200
		if response != nil {
			defer response.Body.Close()
			var health struct {
				Ready bool `json:"ready"`
			}
			if json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&health) != nil || !health.Ready {
				ready = false
			}
		}
		s.writeJSON(w, 200, map[string]any{"ready": ready, "reason": map[bool]string{true: "", false: "browser_unavailable"}[ready], "maxTabs": webbrowser.MaxTabs})
		return
	}
	workspace := strings.TrimPrefix(r.URL.Path, "/api/v1/browser/stream/")
	if !strings.HasPrefix(r.URL.Path, "/api/v1/browser/stream/") || !browserWorkspacePattern.MatchString(workspace) {
		s.writeProblem(w, r, 404, "route_not_found", "Route not found", "")
		return
	}
	if r.Header.Get("Origin") == "" || !s.checkOrigin(w, r) {
		return
	}
	s.requestsMu.Lock()
	if s.requestsClosed {
		s.requestsMu.Unlock()
		return
	}
	s.browserNext++
	connectionID := s.browserNext
	s.browserCancels[connectionID] = cancel
	s.requestsMu.Unlock()
	defer func() { s.requestsMu.Lock(); delete(s.browserCancels, connectionID); s.requestsMu.Unlock() }()
	select {
	case s.browserGate <- struct{}{}:
		defer func() { <-s.browserGate }()
	default:
		s.writeProblem(w, r, 429, "browser_busy", "Browser capacity reached", "")
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	defer ws.CloseNow()
	ws.SetReadLimit(webbrowser.MaxCommandBytes)
	authCtx, stop := context.WithTimeout(ctx, 5*time.Second)
	kind, data, err := ws.Read(authCtx)
	stop()
	var hello struct {
		CSRF string `json:"csrf"`
	}
	if err != nil || kind != websocket.MessageText || len(data) > 4096 || decodeTerminalSocketJSON(data, &hello) != nil || !secureStringEqual(hello.CSRF, s.csrfCookieValue(r)) || s.auth.ValidateCSRF(session, hello.CSRF) != nil {
		return
	}
	if _, err = s.auth.Authenticate(token); err != nil {
		return
	}
	identity := sha256.Sum256([]byte(token + ":" + workspace))
	request, err := s.browserServiceRequest(ctx, "/v1/sessions/"+hex.EncodeToString(identity[:]))
	if err != nil {
		_ = ws.Close(websocket.StatusTryAgainLater, "browser_unavailable")
		return
	}
	upstreamURL, _ := url.Parse(request.URL.String())
	if upstreamURL.Scheme == "https" {
		upstreamURL.Scheme = "wss"
	} else {
		upstreamURL.Scheme = "ws"
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	openCtx, openStop := context.WithTimeout(ctx, 10*time.Second)
	upstream, response, err := websocket.Dial(openCtx, upstreamURL.String(), &websocket.DialOptions{HTTPClient: client, HTTPHeader: request.Header, CompressionMode: websocket.CompressionDisabled})
	openStop()
	if response != nil && err != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		_ = ws.Close(websocket.StatusTryAgainLater, "browser_unavailable")
		return
	}
	defer upstream.CloseNow()
	defer func() {
		s.requestsMu.Lock()
		closing := s.requestsClosed
		s.requestsMu.Unlock()
		_, authErr := s.auth.Authenticate(token)
		if closing || authErr != nil || !time.Now().Before(session.ExpiresAt) {
			finish, stop := context.WithTimeout(context.Background(), 2*time.Second)
			defer stop()
			_ = upstream.Write(finish, websocket.MessageText, []byte(`{"type":"end"}`))
		}
	}()
	upstream.SetReadLimit(webbrowser.MaxEventBytes)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer cancel()
		for {
			kind, data, err := upstream.Read(ctx)
			if err != nil || kind != websocket.MessageText {
				return
			}
			write, stop := context.WithTimeout(ctx, 5*time.Second)
			err = ws.Write(write, kind, data)
			stop()
			if err != nil {
				return
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := s.auth.Authenticate(token); err != nil || !time.Now().Before(session.ExpiresAt) {
					cancel()
					return
				}
				ping, stop := context.WithTimeout(ctx, 5*time.Second)
				err := ws.Ping(ping)
				stop()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()
	window := time.Now()
	count := 0
	for {
		kind, data, err := ws.Read(ctx)
		if err != nil || kind != websocket.MessageText {
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
		if _, err = webbrowser.DecodeCommand(data); err != nil {
			break
		}
		write, stop := context.WithTimeout(ctx, 5*time.Second)
		err = upstream.Write(write, kind, data)
		stop()
		if err != nil {
			break
		}
	}
	cancel()
	_ = ws.Close(websocket.StatusNormalClosure, "disconnected")
	<-done
}
