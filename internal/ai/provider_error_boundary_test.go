package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderCredentialRedactionPrecedesTruncation(t *testing.T) {
	credential := "OpaqueCredentialABCDEFGH0123456789"
	for _, message := range []string{strings.Repeat("x", 990) + credential, strings.Repeat(" ", (32<<10)-8) + credential} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(400); fmt.Fprint(w, message) }))
		_, err := NewHTTPModelClient().Models(context.Background(), Provider{Protocol: ProtocolOpenAICompatible, BaseURL: server.URL, EndpointScope: EndpointPrivate}, credential)
		server.Close()
		if err == nil || strings.Contains(err.Error(), credential[:8]) {
			t.Fatalf("truncated provider error retained a credential prefix: %v", err)
		}
	}
}
