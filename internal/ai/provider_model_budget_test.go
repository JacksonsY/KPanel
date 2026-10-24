package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderModelCountIsBoundedBeforeExpansion(t *testing.T) {
	for _, field := range []string{"data", "models", "combined"} {
		t.Run(field, func(t *testing.T) {
			body := fmt.Sprintf(`{"%s":[%s{}]}`, field, strings.Repeat(`{},`, maxDiscoveredModels))
			if field == "combined" {
				body = fmt.Sprintf(`{"data":[%s{}],"models":[%s{}]}`, strings.Repeat(`{},`, 3000), strings.Repeat(`{},`, 3000))
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			provider := Provider{Protocol: ProtocolOpenAICompatible, BaseURL: server.URL, EndpointScope: EndpointPrivate}
			if models, err := NewHTTPModelClient().Models(context.Background(), provider, "synthetic"); err == nil || !strings.Contains(err.Error(), "model count budget") || len(models) != 0 {
				t.Fatalf("excess models expanded: count=%d err=%v", len(models), err)
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"data":[%s{}]}`, strings.Repeat(`{},`, maxDiscoveredModels-1))
	}))
	defer server.Close()
	models, err := NewHTTPModelClient().Models(context.Background(), Provider{Protocol: ProtocolOpenAICompatible, BaseURL: server.URL, EndpointScope: EndpointPrivate}, "synthetic")
	if err != nil || len(models) != maxDiscoveredModels {
		t.Fatalf("valid model boundary: count=%d err=%v", len(models), err)
	}
}
