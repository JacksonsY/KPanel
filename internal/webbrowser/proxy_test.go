package webbrowser

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestEgressPinsDNSAndRejectsMixedAndMetadata(t *testing.T) {
	p := NewEgress([]netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")})
	defer p.Close()
	for _, value := range []string{"169.254.169.254", "100.100.100.200", "168.63.129.16", "fd00:ec2::254", "224.0.0.1", "::ffff:169.254.169.254"} {
		if p.allowed(netip.MustParseAddr(value)) {
			t.Fatalf("allowlist enabled reserved target %s", value)
		}
	}
	p.Private = nil
	p.Lookup = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("127.0.0.1")}, nil
	}
	called := false
	p.Dial = func(context.Context, string, string) (net.Conn, error) { called = true; return nil, nil }
	if _, err := p.dial(context.Background(), "tcp", "example.com:443"); err == nil || called {
		t.Fatal("mixed DNS answer reached dialer")
	}
	p.Lookup = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
	}
	p.Dial = func(_ context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != "1.1.1.1:443" {
			t.Fatal("unresolved hostname reached dialer")
		}
		a, b := net.Pipe()
		_ = b.Close()
		return a, nil
	}
	conn, err := p.dial(context.Background(), "tcp", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
}

func TestEgressProxiesPlainWebSocketWithoutLosingPolicy(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer ws.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		kind, data, err := ws.Read(ctx)
		if err == nil {
			_ = ws.Write(ctx, kind, data)
		}
	}))
	defer app.Close()
	p := NewEgress([]netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")})
	defer p.Close()
	server := httptest.NewServer(p)
	defer server.Close()
	proxy, _ := url.Parse(server.URL)
	transport := &http.Transport{Proxy: http.ProxyURL(proxy)}
	defer transport.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+app.URL[4:], &websocket.DialOptions{HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	if err = ws.Write(ctx, websocket.MessageText, []byte("echo")); err != nil {
		t.Fatal(err)
	}
	_, data, err := ws.Read(ctx)
	if err != nil || string(data) != "echo" {
		t.Fatal("websocket proxy failed", err)
	}
}

func TestEgressPrivateApplicationRequiresExplicitRange(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Proxy-Authorization") != "" {
			t.Error("proxy credential leaked")
		}
		_, _ = io.WriteString(w, "application")
	}))
	defer app.Close()
	for _, allowed := range []bool{false, true} {
		var ranges []netip.Prefix
		if allowed {
			ranges = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
		}
		p := NewEgress(ranges)
		defer p.Close()
		server := httptest.NewServer(p)
		defer server.Close()
		proxy, _ := url.Parse(server.URL)
		transport := &http.Transport{Proxy: http.ProxyURL(proxy)}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport}
		response, err := client.Get(app.URL)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if allowed && string(body) != "application" {
			t.Fatalf("explicit private application failed: %s", body)
		}
		if !allowed && response.StatusCode != 502 {
			t.Fatal("private application bypassed policy")
		}
	}
}
