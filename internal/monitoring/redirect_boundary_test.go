package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestHTTPProbeMeasuresFirstResponseWithoutFollowingRedirect(t *testing.T) {
	var destinationRequests atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationRequests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL+"/private-action", http.StatusFound)
	}))
	defer source.Close()
	if _, err := probeCheck(context.Background(), nil, Check{Kind: "http", Target: source.URL}); err != nil {
		t.Fatalf("a 302 first response should be reachable: %v", err)
	}
	if got := destinationRequests.Load(); got != 0 {
		t.Fatalf("unselected redirect destination received %d requests", got)
	}
	// Explicit administrator-selected private targets remain supported.
	if _, err := probeCheck(context.Background(), nil, Check{Kind: "http", Target: destination.URL}); err != nil {
		t.Fatal(err)
	}
	if got := destinationRequests.Load(); got != 1 {
		t.Fatalf("explicit target received %d requests, want 1", got)
	}
}
