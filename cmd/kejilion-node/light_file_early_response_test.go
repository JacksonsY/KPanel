package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

func TestLightFileEarlyResponseDoesNotBlockBodyCommand(t *testing.T) {
	returned := make(chan struct{})
	control := &lightFileControl{
		ctx: context.Background(), sessions: make(map[string]*lightNodeFileSession),
		processed: make(map[string]cluster.FileRelayEvent),
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			close(returned)
		}),
	}
	defer control.resetSessions()
	if err := control.startRequest(cluster.FileRelayCommand{ID: "start", RequestID: "request", Method: http.MethodPost, Path: "/v1/files/entry", BodyLength: 1}); err != nil {
		t.Fatal(err)
	}
	<-returned
	done := make(chan error, 1)
	go func() {
		done <- control.writeRequestBody(cluster.FileRelayCommand{ID: "body", RequestID: "request", Data: []byte("x")})
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("body write after handler return unexpectedly succeeded")
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("body write blocked after handler returned")
	}
}
