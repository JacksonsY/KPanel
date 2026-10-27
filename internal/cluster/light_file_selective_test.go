package cluster

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestLightFileSelectivePollIsolatesUnreadConsumer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		relay := newLightFileRelay(time.Now)
		defer relay.closeAll()
		item := relay.node(strings.Repeat("e", 32), true)
		id := strings.Repeat("a", 32)
		slow := &lightFileSession{id: id, responseReady: make(chan struct{}), data: make(chan []byte, 8), done: make(chan struct{}), space: make(chan struct{}, 1)}
		item.sessions[id] = slow
		for i := 0; i < 8; i++ {
			if err := slow.push(context.Background(), int64(i), []byte("x")); err != nil {
				t.Fatal(err)
			}
		}
		other := strings.Repeat("b", 32)
		command := &lightFileCommand{command: FileRelayCommand{ID: other, RequestID: other, Kind: "request", Method: "GET", ExpiresAt: time.Now().Add(time.Hour).Unix()}, done: make(chan error, 1)}
		item.pending[other] = command
		item.queued = append(item.queued, command)
		events := []FileRelayEvent{{RequestID: id, Kind: "data", Offset: 8, Data: []byte("z")}, {RequestID: id, Kind: "end"}, FileRelayFlowMarker()}
		out, err := relay.pollEvents(context.Background(), item.id, []string{id}, events)
		if err != nil || out.Command == nil || out.Command.ID != other {
			t.Fatalf("unread stream blocked another command: %#v %v", out, err)
		}
		if len(out.Events) != 1 || !IsFileRelayFlowMarker(out.Events[0]) || slow.isFinished() {
			t.Fatalf("blocked data or later end incorrectly acknowledged: %#v", out.Events)
		}
		<-slow.data
		out, err = relay.pollEvents(context.Background(), item.id, []string{id}, events)
		if err != nil || len(out.Events) != 3 || !slow.isFinished() {
			t.Fatalf("resumed stream did not receive data and end: %#v %v", out, err)
		}
		if got := <-slow.data; string(got) != "x" {
			t.Fatalf("buffer order changed: %q", got)
		}
		for len(slow.data) > 1 {
			<-slow.data
		}
		if got := <-slow.data; string(got) != "z" {
			t.Fatalf("last data chunk was lost: %q", got)
		}
	})
}
