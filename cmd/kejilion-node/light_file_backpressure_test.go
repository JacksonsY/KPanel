package main

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

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

func TestLightFileAbandonedUploadExpiresAndReleasesSlot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		finished := make(chan struct{})
		control := backpressureControl(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer close(finished)
			_, _ = io.Copy(io.Discard, r.Body)
		}))
		defer control.resetSessions()
		if err := control.startRequest(cluster.FileRelayCommand{ID: "request", RequestID: "upload", Method: "POST", Path: "/v1/files/upload", BodyLength: -1}); err != nil {
			t.Fatal(err)
		}
		control.acceptRelayResponse(cluster.FileRelayPollResponse{Epoch: "old"})
		time.Sleep(2*time.Hour + time.Second)
		synctest.Wait()
		select {
		case <-finished:
		default:
			control.resetSessions()
			<-finished
			t.Fatal("abandoned session retained a blocked handler past its lifetime")
		}
		control.collectEvents()
		if len(control.sessions) != 0 {
			t.Fatal("expired session retained its slot after polling resumed")
		}
	})
}

func TestLightFileSelectiveReceiptsPreserveBlockedStreamAndRetry(t *testing.T) {
	control := backpressureControl(context.Background(), nil)
	control.flowMode = flowProbe
	defer control.resetSessions()
	idA, idB := strings.Repeat("a", 32), strings.Repeat("b", 32)
	for _, id := range []string{idA, idB} {
		ctx, cancel := context.WithCancel(context.Background())
		s := &lightNodeFileSession{requestID: id, ctx: ctx, cancel: cancel, events: make(chan cluster.FileRelayEvent, lightFileEventLimit)}
		s.events <- cluster.FileRelayEvent{RequestID: id, Kind: "data", Data: []byte(id[:1])}
		s.events <- cluster.FileRelayEvent{RequestID: id, Kind: "end"}
		control.sessions[id] = s
	}
	events, _ := control.collectEvents()
	if len(events) != 2 {
		t.Fatalf("expected one event per stream: %v", events)
	}
	again, _ := control.collectEvents()
	if !reflect.DeepEqual(events, again) {
		t.Fatal("unacknowledged poll batch changed on retry")
	}
	// A is blocked, B is accepted. A must stay in front of its end event.
	response := cluster.FileRelayPollResponse{Epoch: "epoch", Events: []cluster.FileRelayEvent{cluster.FileRelayFlowMarker(), cluster.FileRelayFlowReceipt(1, idB)}}
	control.acceptRelayResponse(response)
	if control.flowMode != flowSelective || len(control.sessions[idA].pending) == 0 || control.sessions[idA].pending[0].Kind != "data" {
		t.Fatal("blocked data was lost or overtaken")
	}
	if len(control.sessions[idB].pending) != 1 || control.sessions[idB].pending[0].Kind != "end" {
		t.Fatal("accepted stream did not advance")
	}
	events, _ = control.collectEvents()
	if len(events) != 2 || events[0].RequestID != idA || events[0].Kind != "data" || events[1].RequestID != idB || events[1].Kind != "end" {
		t.Fatalf("retry fairness/order: %v", events)
	}
	control.acceptRelayResponse(cluster.FileRelayPollResponse{Epoch: "epoch", Events: []cluster.FileRelayEvent{cluster.FileRelayFlowMarker(), cluster.FileRelayFlowReceipt(0, idA), cluster.FileRelayFlowReceipt(1, idB)}})
	if control.sessions[idB] != nil || control.sessions[idA] == nil {
		t.Fatal("end did not reclaim only the completed session")
	}
	events, _ = control.collectEvents()
	if len(events) != 1 || events[0].Kind != "end" {
		t.Fatalf("remaining stream end = %v", events)
	}
}

func TestLightFileFlowNegotiationFallsBackToOldCenter(t *testing.T) {
	control := backpressureControl(context.Background(), nil)
	control.flowMode = flowProbe
	id := strings.Repeat("a", 32)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	control.sessions[id] = &lightNodeFileSession{requestID: id, ctx: ctx, cancel: cancel, events: make(chan cluster.FileRelayEvent, 1)}
	control.sessions[id].events <- cluster.FileRelayEvent{RequestID: id, Kind: "end"}
	events, _ := control.collectEvents()
	if len(control.wireEvents(events)) != 2 || control.sessions[id] == nil {
		t.Fatal("capability probe did not preserve in-flight session")
	}
	control.acceptRelayResponse(cluster.FileRelayPollResponse{Epoch: "old-center"})
	if control.flowMode != flowLegacy || len(control.sessions) != 0 || len(control.pendingEvents) != 0 {
		t.Fatal("old center did not use legacy acknowledgement")
	}
	if len(control.wireEvents(nil)) != 0 {
		t.Fatal("old center remains under unnecessary probe polling")
	}
}

func TestLightFileSelectiveRetriesMultipleChunksInOrder(t *testing.T) {
	control := backpressureControl(context.Background(), nil)
	control.flowMode = flowSelective
	defer control.resetSessions()
	id := strings.Repeat("a", 32)
	ctx, cancel := context.WithCancel(context.Background())
	s := &lightNodeFileSession{requestID: id, ctx: ctx, cancel: cancel, events: make(chan cluster.FileRelayEvent, 3)}
	for i := 0; i < 3; i++ {
		s.events <- cluster.FileRelayEvent{RequestID: id, Kind: "data", Offset: int64(i), Data: []byte{byte('a' + i)}}
	}
	control.sessions[id] = s
	events, _ := control.collectEvents()
	if len(events) != 2 {
		t.Fatalf("single stream lost batching: %v", events)
	}
	// Even a malformed receipt for the later event cannot overtake a missing
	// receipt for the first event.
	control.acceptRelayResponse(cluster.FileRelayPollResponse{Events: []cluster.FileRelayEvent{cluster.FileRelayFlowMarker(), cluster.FileRelayFlowReceipt(1, id)}})
	events, _ = control.collectEvents()
	if len(events) != 2 || events[0].Offset != 0 || events[1].Offset != 1 {
		t.Fatalf("retry reordered data: %v", events)
	}
}
