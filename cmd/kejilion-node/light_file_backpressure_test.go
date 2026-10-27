package main

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

func backpressureControl(ctx context.Context, handler http.Handler) *lightFileControl {
	return &lightFileControl{ctx: ctx, handler: handler,
		sessions: make(map[string]*lightNodeFileSession), processed: make(map[string]cluster.FileRelayEvent)}
}

func TestLightFileUnreadUploadDoesNotBlockControl(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		control := backpressureControl(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer control.resetSessions()
		if err := control.startRequest(cluster.FileRelayCommand{ID: "request", RequestID: "upload", Method: "POST", Path: "/v1/files/upload", BodyLength: 1}); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			done <- control.executeCommand(cluster.FileRelayCommand{ID: "body", RequestID: "upload", Kind: "body", Data: []byte("x"), Final: true})
		}()
		synctest.Wait()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		default:
			control.resetSessions()
			<-done
			t.Fatal("unread upload blocks the control loop and cancellation")
		}
		if err := control.executeCommand(cluster.FileRelayCommand{ID: "cancel", RequestID: "upload", Kind: "cancel"}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if len(control.sessions) != 0 {
			t.Fatal("cancel retained the upload slot")
		}
	})
}

func TestLightFileCanceledResponseWriteReportsFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		session := &lightNodeFileSession{ctx: ctx, cancel: cancel, events: make(chan cluster.FileRelayEvent, lightFileEventLimit)}
		writer := &lightFileResponseWriter{session: session, headers: make(http.Header)}
		data := []byte(strings.Repeat("x", lightFileEventChunkBytes*(lightFileEventLimit+2)))
		done := make(chan struct{})
		var count int
		var err error
		go func() { count, err = writer.Write(data); close(done) }()
		synctest.Wait()
		if len(session.events) != lightFileEventLimit {
			t.Fatal("producer bypassed bounded response queue")
		}
		cancel()
		<-done
		if err == nil || count >= len(data) {
			t.Fatalf("canceled write reported success: bytes=%d error=%v", count, err)
		}
	})
}

func TestLightFileParentCancellationUnblocksRequestBody(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		finished := make(chan struct{})
		control := backpressureControl(ctx, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer close(finished)
			_, _ = io.Copy(io.Discard, r.Body)
		}))
		defer control.resetSessions()
		if err := control.startRequest(cluster.FileRelayCommand{ID: "request", RequestID: "upload", Method: "POST", Path: "/v1/files/upload", BodyLength: -1}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		cancel()
		synctest.Wait()
		select {
		case <-finished:
		default:
			control.resetSessions()
			<-finished
			t.Fatal("parent cancellation did not unblock handler body read")
		}
	})
}

func TestLightFileSlowUploadAcknowledgesOnceAndKeepsPolling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		read := make(chan struct{})
		payload := make(chan string, 1)
		control := backpressureControl(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-read
			data, _ := io.ReadAll(r.Body)
			payload <- string(data)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer control.resetSessions()
		if err := control.startRequest(cluster.FileRelayCommand{ID: "request", RequestID: "upload", Method: "POST", Path: "/v1/files/upload", BodyLength: 2}); err != nil {
			t.Fatal(err)
		}
		control.acceptRelayResponse(cluster.FileRelayPollResponse{Epoch: "old"})
		body := cluster.FileRelayCommand{ID: "body", RequestID: "upload", Kind: "body", Data: []byte("a")}
		if err := control.executeCommand(body); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		for i := 0; i < 130; i++ {
			if err := control.executeCommand(body); err != nil {
				t.Fatal(err)
			}
		}
		if err := control.executeCommand(cluster.FileRelayCommand{ID: "overflow", RequestID: "upload", Kind: "body", Data: []byte("b")}); err == nil {
			t.Fatal("second in-flight block was accepted")
		}
		events, _ := control.collectEvents()
		if len(events) != 0 || len(control.processed) != 1 {
			t.Fatalf("acknowledged unread bytes: %v", events)
		}
		close(read)
		synctest.Wait()
		events, _ = control.collectEvents()
		if len(events) != 1 || events[0].CommandID != body.ID || events[0].Kind != "accepted" {
			t.Fatalf("missing consumed-body receipt: %v", events)
		}
		// A transport failure must preserve the exact pending acknowledgement.
		retry, _ := control.collectEvents()
		if !reflect.DeepEqual(retry, events) {
			t.Fatal("poll retry changed its acknowledgement")
		}
		control.acceptRelayResponse(cluster.FileRelayPollResponse{Epoch: "old"})
		if err := control.executeCommand(body); err != nil {
			t.Fatal(err)
		}
		control.acceptRelayResponse(cluster.FileRelayPollResponse{Epoch: "old"})
		if err := control.executeCommand(cluster.FileRelayCommand{ID: "final", RequestID: "upload", Kind: "body", Offset: 1, Data: []byte("b"), Final: true}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if got := <-payload; got != "ab" {
			t.Fatalf("upload repeated or reordered bytes: %q", got)
		}
		control.collectEvents()
		if len(control.sessions) != 0 {
			t.Fatal("completed upload retained a slot")
		}
	})
}

func TestLightFileEpochResetReleasesBlockedBodyWorker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		control := backpressureControl(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer control.resetSessions()
		control.acceptRelayResponse(cluster.FileRelayPollResponse{Epoch: "old"})
		if err := control.startRequest(cluster.FileRelayCommand{ID: "request", RequestID: "upload", Method: "POST", Path: "/v1/files/upload", BodyLength: -1}); err != nil {
			t.Fatal(err)
		}
		session := control.sessions["upload"]
		if err := control.executeCommand(cluster.FileRelayCommand{ID: "body", RequestID: "upload", Kind: "body", Data: []byte("x")}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		control.acceptRelayResponse(cluster.FileRelayPollResponse{Epoch: "new"})
		synctest.Wait()
		if len(control.sessions) != 0 || len(control.processed) != 0 || len(control.pendingEvents) != 0 {
			t.Fatal("new center inherited old relay state")
		}
		select {
		case result := <-session.bodyResult:
			if result.err == nil {
				t.Fatal("reset body worker reported successful consumption")
			}
		default:
			t.Fatal("epoch reset leaked blocked body worker")
		}
	})
}
