package panel

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAgentReadGroupCoalescesIdenticalInflightReads(t *testing.T) {
	var group agentReadGroup
	var calls atomic.Int32
	start := make(chan struct{})
	const readers = 12
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(readers)
	done.Add(readers)

	for range readers {
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			response, err, _ := group.do(context.Background(), "/v1/docker/containers?", func(context.Context) (AgentResponse, error) {
				calls.Add(1)
				time.Sleep(40 * time.Millisecond)
				return AgentResponse{StatusCode: 200, Body: []byte("ok")}, nil
			})
			if err != nil || string(response.Body) != "ok" {
				t.Errorf("unexpected shared response: %#v, %v", response, err)
			}
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	if got := calls.Load(); got != 1 {
		t.Fatalf("identical in-flight reads executed %d times; want 1", got)
	}
}

func TestAgentReadGroupDoesNotCacheCompletedReads(t *testing.T) {
	var group agentReadGroup
	var calls atomic.Int32
	read := func(context.Context) (AgentResponse, error) {
		calls.Add(1)
		return AgentResponse{StatusCode: 200}, nil
	}
	for range 2 {
		if _, err, _ := group.do(context.Background(), "/v1/system/summary?", read); err != nil {
			t.Fatal(err)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("completed reads were cached: calls=%d", got)
	}
}
