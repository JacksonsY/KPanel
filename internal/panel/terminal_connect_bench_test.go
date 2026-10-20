package panel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"
)

type terminalConnectAgent struct {
	retainedAgentStub
	delay  time.Duration
	probes int
}

func (a *terminalConnectAgent) Do(ctx context.Context, method, path, query, id string, body []byte) (AgentResponse, error) {
	if path == "/v1/terminals/capabilities/input-protocol" {
		a.probes++
		timer := time.NewTimer(a.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return AgentResponse{}, ctx.Err()
		case <-timer.C:
		}
	}
	return a.retainedAgentStub.Do(ctx, method, path, query, id, body)
}

// Same authenticated handler and stub on both revisions. The injected delay
// isolates Agent probe overhead; it does not represent native PTY startup time.
func TestLocalTerminalConnectPerformance(t *testing.T) {
	if os.Getenv("KPANEL_TERMINAL_CONNECT_BENCH") != "1" {
		t.Skip("set KPANEL_TERMINAL_CONNECT_BENCH=1")
	}
	for _, delay := range []time.Duration{0, 5 * time.Millisecond, 25 * time.Millisecond} {
		t.Run(fmt.Sprint(delay), func(t *testing.T) {
			s, path := newTestServer(t)
			a := &terminalConnectAgent{delay: delay}
			s.agent = a
			cookie, csrf := bootstrapCookies(t, s, path)
			var times []time.Duration
			for range 20 {
				start := time.Now()
				response := authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions", []byte(`{"hostId":"local","rows":24,"columns":80,"retained":true}`), cookie, csrf, retainedHeaders(csrf))
				times = append(times, time.Since(start))
				if response.Code != http.StatusCreated {
					t.Fatalf("open %d: %s", response.Code, response.Body.String())
				}
				var opened terminalOpenResponse
				if err := json.Unmarshal(response.Body.Bytes(), &opened); err != nil {
					t.Fatal(err)
				}
				closed := authenticatedRequest(s, http.MethodPost, "/api/v1/terminal-sessions/"+opened.SessionID+"/close", []byte(`{}`), cookie, csrf, retainedHeaders(csrf))
				if closed.Code != http.StatusOK {
					t.Fatalf("close %d: %s", closed.Code, closed.Body.String())
				}
			}
			sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
			t.Logf("agent_probe_delay=%v n=%d open_p50=%v open_p95=%v capability_calls=%d", delay, len(times), times[9].Round(time.Microsecond), times[18].Round(time.Microsecond), a.probes)
		})
	}
}
