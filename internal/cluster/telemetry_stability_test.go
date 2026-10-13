package cluster

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

type stabilityTelemetry func(context.Context) (contract.HostTelemetry, error)

func (f stabilityTelemetry) Telemetry(ctx context.Context) (contract.HostTelemetry, error) {
	return f(ctx)
}

func newStabilityService(t *testing.T, source TelemetrySource) *Service {
	t.Helper()
	s, err := NewService(ServiceConfig{DataDir: t.TempDir(), Telemetry: source, Remote: localNodeTestRemote{}, PanelVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestLocalTelemetryReaderCancellationDoesNotPoisonHealth(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	s := newStabilityService(t, stabilityTelemetry(func(ctx context.Context) (contract.HostTelemetry, error) {
		calls.Add(1)
		close(started)
		select {
		case <-release:
			return validLocalNodeTelemetry(time.Now()), nil
		case <-ctx.Done():
			return contract.HostTelemetry{}, ctx.Err()
		}
	}))
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan Host, 1)
	go func() { first <- s.localHostSummary(ctx) }()
	<-started
	cancel()
	if host := <-first; host.ConsecutiveFailures != 0 {
		t.Fatalf("canceled reader changed health: %+v", host)
	}
	for i := 0; i < 10; i++ {
		if host := s.localHostSummary(ctx); host.ConsecutiveFailures != 0 {
			t.Fatal("canceled reads counted as failures")
		}
	}
	second := make(chan Host, 1)
	go func() { second <- s.localHostSummary(context.Background()) }()
	close(release)
	if host := <-second; host.State != HostOnline || host.ConsecutiveFailures != 0 {
		t.Fatalf("healthy collection: %+v", host)
	}
	if calls.Load() != 1 {
		t.Fatalf("collections=%d", calls.Load())
	}
}

func TestLocalTelemetryFailureCooldownCountsCollectionsNotReaders(t *testing.T) {
	var calls atomic.Int32
	s := newStabilityService(t, stabilityTelemetry(func(context.Context) (contract.HostTelemetry, error) {
		calls.Add(1)
		return contract.HostTelemetry{}, errors.New("agent down")
	}))
	results := make(chan Host, 32)
	for i := 0; i < 32; i++ {
		go func() { results <- s.localHostSummary(context.Background()) }()
	}
	for i := 0; i < 32; i++ {
		if h := <-results; h.ConsecutiveFailures != 1 || h.State != HostDegraded {
			t.Fatalf("reader multiplied failure: %+v", h)
		}
	}
	s.invalidateLocalTelemetry()
	if h := s.localHostSummary(context.Background()); h.ConsecutiveFailures != 1 {
		t.Fatal("refresh bypassed failure cooldown")
	}
	if calls.Load() != 1 {
		t.Fatalf("collections=%d", calls.Load())
	}
}

func TestLocalTelemetryCloseCancelsOwnedCollection(t *testing.T) {
	started := make(chan struct{})
	s := newStabilityService(t, stabilityTelemetry(func(ctx context.Context) (contract.HostTelemetry, error) {
		close(started)
		<-ctx.Done()
		return contract.HostTelemetry{}, ctx.Err()
	}))
	done := make(chan struct{})
	go func() { defer close(done); _, _ = s.localTelemetry(context.Background()) }()
	<-started
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reader survived service shutdown")
	}
	if s.localHost.consecutiveFailures != 0 {
		t.Fatal("shutdown counted as agent failure")
	}
}

func TestSummaryTransportDoesNotCompeteWithTerminalControl(t *testing.T) {
	blocked := make(chan struct{}, 2)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/busy" {
			blocked <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c, err := NewRemoteClient(RemoteClientConfig{Dialer: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			req, _ := http.NewRequest(http.MethodGet, "http://8.8.8.8:1801/busy", nil)
			var value any
			_, err := c.doJSONWithHeaders(c.summaryHTTPClient(), req, 1024, &value)
			done <- err
		}()
	}
	defer func() {
		close(release)
		for i := 0; i < 2; i++ {
			if err := <-done; err != nil {
				t.Error(err)
			}
		}
	}()
	for i := 0; i < 2; i++ {
		select {
		case <-blocked:
		case <-time.After(time.Second):
			t.Fatal("summary did not start")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://8.8.8.8:1801/control", nil)
	var value any
	if _, err := c.doJSONWithHeaders(c.client, req, 1024, &value); err != nil {
		t.Fatalf("summary starved terminal control: %v", err)
	}
}

func TestSummaryTransportAcceptsSlowHealthyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(3200 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c, err := NewRemoteClient(RemoteClientConfig{Dialer: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://8.8.8.8:1801/summary", nil)
	var value any
	if _, err := c.doJSONWithHeaders(c.summaryHTTPClient(), req, 1024, &value); err != nil {
		t.Fatal(err)
	}
	if got := remoteErrorCode(classifyRemoteTransportError(context.DeadlineExceeded)); got != "timeout" {
		t.Fatalf("code=%s", got)
	}
}
