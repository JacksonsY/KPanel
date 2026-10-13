package panel

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func waitAgentReaders(t *testing.T, group *agentReadGroup, key string, count int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		group.mu.Lock()
		call := group.calls[key]
		ok := call != nil && call.waiters == count
		group.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("readers did not join shared request")
}

func TestAgentReadCancellationIsPerReader(t *testing.T) {
	var group agentReadGroup
	var calls atomic.Int32
	release := make(chan struct{})
	read := func(ctx context.Context) (AgentResponse, error) {
		calls.Add(1)
		select {
		case <-release:
			return AgentResponse{StatusCode: 200}, nil
		case <-ctx.Done():
			return AgentResponse{}, ctx.Err()
		}
	}
	first, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstDone := make(chan error, 1)
	go func() { _, err, _ := group.do(first, "health", read); firstDone <- err }()
	waitAgentReaders(t, &group, "health", 1)
	second, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	secondDone := make(chan error, 1)
	go func() { _, err, _ := group.do(second, "health", read); secondDone <- err }()
	waitAgentReaders(t, &group, "health", 2)
	cancel()
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("upstream reads=%d", calls.Load())
	}
}

func TestAgentReadAllReadersCancelReleasesUpstream(t *testing.T) {
	var group agentReadGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, stopped := make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _ = group.do(ctx, "health", func(ctx context.Context) (AgentResponse, error) {
			close(started)
			<-ctx.Done()
			close(stopped)
			return AgentResponse{}, ctx.Err()
		})
	}()
	<-started
	cancel()
	<-done
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("abandoned read remained active")
	}
	response, err, _ := group.do(context.Background(), "health", func(context.Context) (AgentResponse, error) {
		return AgentResponse{StatusCode: 200}, nil
	})
	if err != nil || response.StatusCode != 200 {
		t.Fatal(response, err)
	}
}

// Exercise the production transport, including connection reuse, against a
// real Unix socket. No Docker daemon or host services are changed.
func TestAgentUnixSocketIsolationAndRestart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket lifecycle is validated on Linux")
	}
	dir := t.TempDir()
	socket := filepath.Join(dir, "agent.sock")
	tokenFile := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenFile, []byte("test-token"), 0600); err != nil {
		t.Fatal(err)
	}
	var streams atomic.Int32
	start := func() *http.Server {
		listener, err := net.Listen("unix", socket)
		if err != nil {
			t.Fatal(err)
		}
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test-token" {
				w.WriteHeader(401)
				return
			}
			if r.URL.Path == "/stream" {
				streams.Add(1)
				defer streams.Add(-1)
				w.WriteHeader(200)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				return
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})}
		go func() { _ = server.Serve(listener) }()
		t.Cleanup(func() { _ = server.Close() })
		return server
	}
	server := start()
	client := NewAgentClient(socket, tokenFile, 4096)
	t.Cleanup(client.client.CloseIdleConnections)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Hold more streams than the default idle connection count. Health,
	// file listings and terminal control must still obtain a connection.
	for range 4 {
		response, err := client.Open(ctx, http.MethodGet, "/stream", "", "stream")
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
	}
	if streams.Load() != 4 {
		t.Fatal("streams not established")
	}
	for _, path := range []string{"/v1/health", "/v1/files", "/v1/terminal/sessions"} {
		probe, stop := context.WithTimeout(ctx, time.Second)
		response, err := client.Get(probe, path, "", "probe")
		stop()
		if err != nil || response.StatusCode != 200 {
			t.Fatal(path, response, err)
		}
	}
	_ = server.Close()
	if _, err := client.Get(ctx, "/v1/health", "", "down"); err == nil {
		t.Fatal("unreachable Agent reported healthy")
	}
	start()
	response, err := client.Get(ctx, "/v1/health", "", "restarted")
	if err != nil || response.StatusCode != 200 {
		t.Fatal("client failed to reconnect after Agent restart", response, err)
	}
}
