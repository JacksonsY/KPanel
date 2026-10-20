package cluster

import (
	"context"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Drop only the first connection's return traffic, while TCP writes still
// succeed. Later dials are healthy, simulating a recoverable half-open path.
type terminalHalfOpenConn struct {
	net.Conn
	drop atomic.Bool
	done chan struct{}
	once sync.Once
}

func (c *terminalHalfOpenConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if c.drop.Load() {
		<-c.done
		return 0, net.ErrClosed
	}
	return n, err
}
func (c *terminalHalfOpenConn) Close() error {
	c.once.Do(func() { close(c.done) })
	return c.Conn.Close()
}

func TestTerminalHalfOpenPerformance(t *testing.T) {
	if os.Getenv("KPANEL_TERMINAL_CONNECT_BENCH") != "1" {
		t.Skip("set KPANEL_TERMINAL_CONNECT_BENCH=1")
	}
	f := newStreamFixture(t, nil)
	f.service.terminal = managerTerminalBackend{manager: newEchoManager(t)}
	transport := f.server.Client().Transport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	var first atomic.Pointer[terminalHalfOpenConn]
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		c, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		wrapped := &terminalHalfOpenConn{Conn: c, done: make(chan struct{})}
		if first.CompareAndSwap(nil, wrapped) {
			return wrapped, nil
		}
		return c, nil
	}
	f.client.streamClient = &http.Client{Transport: transport}
	var dials atomic.Int32
	s, _, err := openStreamTerminal(context.Background(), context.Background(), "host", f.panelTerminalDialer(t, &dials), nil, 24, 80, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.shutdown()
	first.Load().drop.Store(true)
	start := time.Now()
	deadline := time.NewTimer(49 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline.C:
			if os.Getenv("KPANEL_TERMINAL_EXPECT_RECOVERY") == "1" {
				t.Fatal("silent return path was not recovered")
			}
			t.Log("half_open_detected=false observation=49s writes_remained_available=true")
			return
		case <-tick.C:
			s.mu.Lock()
			ready := s.conn != nil && dials.Load() > 1
			s.mu.Unlock()
			if ready {
				t.Logf("half_open_detected=true reconnect_started=%v", time.Since(start).Round(time.Millisecond))
				if err := s.input(context.Background(), TerminalInputRequest{SessionID: s.sessionID, Data: "cmVjb3ZlcmVk"}); err != nil {
					t.Fatal(err)
				}
				readStreamTerminalUntil(t, s, 0, "recovered")
				return
			}
		}
	}
}
