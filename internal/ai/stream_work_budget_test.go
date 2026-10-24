package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestNativeTextFragmentsHaveLinearAllocationBudget(t *testing.T) {
	const count = 2048
	fragment := strings.Repeat("x", 64)
	for _, protocol := range []ProviderProtocol{ProtocolAnthropic, ProtocolGemini} {
		t.Run(string(protocol), func(t *testing.T) {
			var wire strings.Builder
			if protocol == ProtocolAnthropic {
				wire.WriteString("data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
				for i := 0; i < count; i++ {
					fmt.Fprintf(&wire, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\n", fragment)
				}
				wire.WriteString("data: {\"type\":\"message_stop\"}\n\n")
			} else {
				wire.WriteString("data: {\"candidates\":[{\"content\":{\"parts\":[")
				for i := 0; i < count; i++ {
					if i > 0 {
						wire.WriteByte(',')
					}
					fmt.Fprintf(&wire, "{\"text\":%q}", fragment)
				}
				wire.WriteString("]}}]}\n\n")
			}
			body := wire.String()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			client := NewHTTPModelClient()
			var output strings.Builder
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			err := client.Stream(context.Background(), Provider{Protocol: protocol, BaseURL: server.URL, EndpointScope: EndpointPrivate}, "synthetic", CompletionRequest{Model: "test"}, func(event CompletionEvent) error { output.WriteString(event.Delta); return nil })
			runtime.ReadMemStats(&after)
			if err != nil || output.String() != strings.Repeat(fragment, count) {
				t.Fatalf("fragmented text changed: len=%d err=%v", output.Len(), err)
			}
			allocated := after.TotalAlloc - before.TotalAlloc
			t.Logf("wire=%d text=%d totalAlloc=%d", len(body), output.Len(), allocated)
			if allocated > 64<<20 {
				t.Fatalf("fragment copying exceeds 64 MiB for 128 KiB text: %d", allocated)
			}
		})
	}
}
