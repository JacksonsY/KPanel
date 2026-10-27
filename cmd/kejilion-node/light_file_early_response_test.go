package main

import (
	"context"
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

func TestLightFileEarlyResponseDoesNotBlockBodyCommand(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		control := backpressureControl(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer control.resetSessions()
		if err := control.startRequest(cluster.FileRelayCommand{ID: "start", RequestID: "request", Method: http.MethodPost, Path: "/v1/files/entry", BodyLength: 1}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		session := control.sessions["request"]
		if err := control.writeRequestBody(cluster.FileRelayCommand{ID: "body", RequestID: "request", Data: []byte("x")}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		select {
		case result := <-session.bodyResult:
			if result.err == nil {
				t.Fatal("closed body was consumed after handler returned")
			}
			session.bodyResult <- result
		default:
			t.Fatal("body worker remained blocked after early response")
		}
		events, _ := control.collectEvents()
		var status int
		var ended bool
		for _, event := range events {
			if event.Kind == "response" {
				status = event.Status
			}
			if event.Kind == "end" {
				ended = true
			}
			if event.Kind == "error" {
				t.Fatalf("pipe failure replaced early response: %v", event)
			}
		}
		if status != http.StatusBadRequest || !ended || len(control.sessions) != 0 {
			t.Fatalf("early response incomplete: %v", events)
		}
	})
}
