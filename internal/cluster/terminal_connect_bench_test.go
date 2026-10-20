//go:build linux

package cluster_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

// The same harness runs against the parent and candidate. Pairing and summary
// discovery are excluded; each open creates a fresh TLS/Noise stream and PTY.
// These are controlled loopback measurements, not browser/public-network timings.
func TestTerminalConnectPerformance(t *testing.T) {
	if os.Getenv("KPANEL_TERMINAL_CONNECT_BENCH") != "1" {
		t.Skip("set KPANEL_TERMINAL_CONNECT_BENCH=1")
	}
	for _, rtt := range []time.Duration{0, 50 * time.Millisecond, 150 * time.Millisecond} {
		t.Run(fmt.Sprint(rtt), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			center := newFileStreamNodeWith(t, fileStreamNodeOptions{latency: rtt / 2})
			target := newFileStreamNodeWith(t, fileStreamNodeOptions{advertiseStream: true, advertiseTerminalReady: true, terminal: managerBackend{manager: newIntegrationEchoManager(t)}})
			host := pairStreamNodes(t, ctx, center, target)
			var opens, echoes []time.Duration
			for sample := range 12 {
				started := time.Now()
				opened, err := center.service.TerminalOpen(ctx, host.ID, cluster.TerminalOpenRequest{Rows: 24, Columns: 80})
				if err != nil {
					t.Fatal(err)
				}
				opens = append(opens, time.Since(started))
				offset := opened.Offset
				for key := range 5 {
					marker := fmt.Sprintf("sample-%d-key-%d", sample, key)
					started = time.Now()
					if err := center.service.TerminalInput(ctx, host.ID, cluster.TerminalInputRequest{SessionID: opened.SessionID, Data: base64.StdEncoding.EncodeToString([]byte(marker))}); err != nil {
						t.Fatal(err)
					}
					var seen strings.Builder
					for !strings.Contains(seen.String(), marker) {
						output, err := center.service.TerminalOutput(ctx, host.ID, cluster.TerminalOutputRequest{SessionID: opened.SessionID, Offset: offset, Wait: 1000})
						if err != nil {
							t.Fatal(err)
						}
						seen.Write(output.Data)
						offset = output.NextOffset
					}
					echoes = append(echoes, time.Since(started))
				}
				if err := center.service.TerminalClose(ctx, host.ID, cluster.TerminalCloseRequest{SessionID: opened.SessionID}); err != nil {
					t.Fatal(err)
				}
			}
			percentile := func(samples []time.Duration, p int) time.Duration {
				sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
				return samples[(len(samples)*p+99)/100-1].Round(time.Microsecond)
			}
			t.Logf("RTT=%v open_n=%d open_p50=%v open_p95=%v echo_n=%d echo_p50=%v echo_p95=%v stream_connections=%d", rtt, len(opens), percentile(opens, 50), percentile(opens, 95), len(echoes), percentile(echoes, 50), percentile(echoes, 95), target.calls.Load())
		})
	}
}
