package webbrowser

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kejilion/kejilion-panel/internal/netpolicy"
)

// Egress checks every actual dial, including CONNECT and redirected resources.
// Operator-granted private ranges never override metadata/reserved classes.
type Egress struct {
	Private     []netip.Prefix
	Lookup      func(context.Context, string, string) ([]netip.Addr, error)
	Dial        func(context.Context, string, string) (net.Conn, error)
	gate        chan struct{}
	mu          sync.Mutex
	connections map[net.Conn]struct{}
	closed      bool
	transport   *http.Transport
}

func NewEgress(private []netip.Prefix) *Egress {
	p := &Egress{Private: private, Lookup: net.DefaultResolver.LookupNetIP, Dial: (&net.Dialer{Timeout: 8 * time.Second, KeepAlive: 20 * time.Second}).DialContext, gate: make(chan struct{}, 32), connections: make(map[net.Conn]struct{})}
	p.transport = &http.Transport{Proxy: nil, DialContext: p.dial, MaxIdleConns: 16, MaxIdleConnsPerHost: 4, MaxConnsPerHost: 8, IdleConnTimeout: 30 * time.Second, ResponseHeaderTimeout: 20 * time.Second, MaxResponseHeaderBytes: 64 << 10}
	return p
}

func (p *Egress) allowed(ip netip.Addr) bool {
	ip = ip.Unmap()
	switch netpolicy.Classify(ip) {
	case netpolicy.Public:
		return true
	case netpolicy.Private, netpolicy.Shared, netpolicy.Loopback:
		for _, prefix := range p.Private {
			if prefix.Contains(ip) {
				return true
			}
		}
	}
	return false
}

func (p *Egress) dial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	portNumber, portErr := strconv.Atoi(port)
	if portErr != nil || portNumber < 1 || portNumber > 65535 || strings.ContainsAny(host, "\r\n\x00%") {
		return nil, errors.New("blocked_target")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	ips, err := p.Lookup(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("target_unavailable")
	}
	for _, ip := range ips {
		if !p.allowed(ip) {
			return nil, errors.New("blocked_target")
		}
	}
	for _, ip := range ips {
		conn, e := p.Dial(ctx, network, net.JoinHostPort(ip.Unmap().String(), port))
		if e == nil {
			return conn, nil
		}
	}
	return nil, errors.New("target_unavailable")
}

func (p *Egress) Close() {
	p.transport.CloseIdleConnections()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	for conn := range p.connections {
		_ = conn.Close()
	}
}

func (p *Egress) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	select {
	case p.gate <- struct{}{}:
		defer func() { <-p.gate }()
	default:
		http.Error(w, "browser_busy", 429)
		return
	}
	if r.Method == http.MethodConnect {
		p.connect(w, r)
		return
	}
	if r.URL.Scheme != "http" || r.URL.User != nil || r.URL.Host == "" {
		http.Error(w, "invalid_target", 400)
		return
	}
	if r.ContentLength > 16<<20 {
		http.Error(w, "request_too_large", 413)
		return
	}
	req := r.Clone(r.Context())
	req.RequestURI = ""
	req.Header = r.Header.Clone()
	upgrade := strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
	for _, key := range []string{"Proxy-Authorization", "Proxy-Connection", "Connection", "Keep-Alive", "Upgrade"} {
		req.Header.Del(key)
	}
	if upgrade {
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
	}
	req.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	response, err := p.transport.RoundTrip(req)
	if err != nil {
		http.Error(w, "target_unavailable", 502)
		return
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusSwitchingProtocols {
		duplex, ok := response.Body.(io.ReadWriteCloser)
		if !upgrade || !ok {
			http.Error(w, "unsupported_upgrade", 502)
			return
		}
		h, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "unsupported_upgrade", 500)
			return
		}
		client, buffered, err := h.Hijack()
		if err != nil {
			return
		}
		defer client.Close()
		_ = client.SetDeadline(time.Now().Add(30 * time.Minute))
		if _, err = buffered.WriteString("HTTP/1.1 101 Switching Protocols\r\n"); err != nil {
			return
		}
		if response.Header.Write(buffered) != nil {
			return
		}
		if _, err = buffered.WriteString("\r\n"); err != nil || buffered.Flush() != nil {
			return
		}
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}
		p.connections[client] = struct{}{}
		p.mu.Unlock()
		defer func() { p.mu.Lock(); delete(p.connections, client); p.mu.Unlock() }()
		done := make(chan struct{})
		go func() { _, _ = io.Copy(duplex, buffered); _ = duplex.Close(); close(done) }()
		_, _ = io.Copy(client, duplex)
		_ = client.Close()
		<-done
		return
	}
	for key, values := range response.Header {
		if strings.EqualFold(key, "Connection") || strings.EqualFold(key, "Transfer-Encoding") {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	// A request has a hard lifetime as well as bounded concurrency. Stream, never buffer pages.
	_, _ = io.Copy(w, response.Body)
}

func (p *Egress) connect(w http.ResponseWriter, r *http.Request) {
	upstream, err := p.dial(r.Context(), "tcp", r.Host)
	if err != nil {
		http.Error(w, "blocked_or_unavailable", 403)
		return
	}
	defer upstream.Close()
	h, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "unsupported", 500)
		return
	}
	client, buffer, err := h.Hijack()
	if err != nil {
		return
	}
	defer client.Close()
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.connections[client] = struct{}{}
	p.connections[upstream] = struct{}{}
	p.mu.Unlock()
	defer func() { p.mu.Lock(); delete(p.connections, client); delete(p.connections, upstream); p.mu.Unlock() }()
	deadline := time.Now().Add(30 * time.Minute)
	_ = client.SetDeadline(deadline)
	_ = upstream.SetDeadline(deadline)
	if _, err = buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if buffer.Flush() != nil {
		return
	}
	done := make(chan struct{})
	go func() { _, _ = io.Copy(upstream, buffer); _ = upstream.Close(); close(done) }()
	_, _ = io.Copy(client, upstream)
	_ = client.Close()
	<-done
}
