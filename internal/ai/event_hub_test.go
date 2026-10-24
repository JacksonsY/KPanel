package ai

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestEventHubOverflowClosesSubscriber(t *testing.T) {
	h := NewEventHub()
	ch, stop := h.Subscribe("run")
	defer stop()
	for i := 0; i < 33; i++ {
		h.Publish(RunEvent{RunID: "run", Type: "message.delta", Data: map[string]string{"delta": "x"}})
	}
	h.Publish(RunEvent{RunID: "run", Type: "run.completed"})
	for i := 0; i < 34; i++ {
		select {
		case _, ok := <-ch:
			if !ok {
				stop() // Must remain safe after overflow and repeated unsubscribe.
				return
			}
		default:
			t.Fatal("slow subscriber silently lost events without reconnect signal")
		}
	}
	t.Fatal("slow subscriber was not closed")
}

func TestEventHubAtomicLiveBoundaryAndCleanup(t *testing.T) {
	h := NewEventHub()
	m := Message{ID: "draft", RunID: "run", SessionID: "session", Role: RoleAssistant}
	if err := h.beginMessage(m); err != nil {
		t.Fatal(err)
	}
	if err := h.publishDelta("run", "draft", "prefix", "prefix"); err != nil {
		t.Fatal(err)
	}
	inside, release, published := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	done := make(chan struct{})
	go func() {
		<-inside
		close(done)
		published <- h.publishDelta("run", "draft", "prefix-tail", "-tail")
	}()
	ch, stop, err := h.SnapshotSubscribe("run", func(live *Message) error {
		if live == nil || live.Content != "prefix" {
			t.Errorf("snapshot=%+v", live)
		}
		close(inside)
		<-done // Publisher is attempting the locked boundary.
		select {
		case <-published:
			t.Error("publisher crossed snapshot lock")
		default:
		}
		close(release)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	<-release
	if err := <-published; err != nil {
		t.Fatal(err)
	}
	if e := <-ch; e.Type != "message.delta" || e.Data.(map[string]string)["delta"] != "-tail" {
		t.Fatalf("event=%+v", e)
	}
	h.Publish(RunEvent{RunID: "run", Type: "message.completed", Data: m})
	if len(h.live) != 0 {
		t.Fatal("completed draft leaked")
	}
	stop()
	stop()
	if len(h.subscribers) != 0 {
		t.Fatal("unsubscribe map leaked")
	}
	_, _, err = h.SnapshotSubscribe("run", func(*Message) error { return errors.New("failed read") })
	if err == nil || len(h.subscribers) != 0 {
		t.Fatal("failed read created subscription")
	}
}

func TestProjectLiveMessageBudgetAndOwnership(t *testing.T) {
	run := Run{ID: "run", SessionID: "session", ProviderID: "provider", ModelID: "model"}
	live := Message{ID: "draft", RunID: run.ID, SessionID: run.SessionID, ProviderID: run.ProviderID, ModelID: run.ModelID, Role: RoleAssistant, Content: strings.Repeat("x", MaxAssistantBytes), CreatedAt: time.Unix(3, 0)}
	history := []Message{
		{ID: "old", Content: strings.Repeat("a", MaxAssistantBytes), CreatedAt: time.Unix(1, 0)},
		{ID: "recent", Content: strings.Repeat("b", MaxAssistantBytes-1), CreatedAt: time.Unix(2, 0)},
		{ID: "draft", Content: "x", CreatedAt: live.CreatedAt},
	}
	items, id, err := ProjectLiveMessage(run, history, &live)
	if err != nil || id != live.ID || len(items) != 2 || items[0].ID != "recent" || items[1].Content != live.Content {
		t.Fatalf("projection items=%d id=%s error=%v", len(items), id, err)
	}
	bytes := 0
	for _, item := range items {
		bytes += len(item.Content)
	}
	if bytes > 2<<20 {
		t.Fatal("live escaped history budget")
	}
	live.SessionID = "other"
	if _, _, err := ProjectLiveMessage(run, history, &live); err == nil {
		t.Fatal("cross-session draft accepted")
	}
}

// Fake client advances only when the test releases a finite command. It crosses
// the real known-credential boundary and writes real SQLite checkpoints.
func TestRuntimeLiveSnapshotBetweenCheckpoints(t *testing.T) {
	s, providers, p, m := runtimeFixture(t)
	defer s.Close()
	session, run := checkpointRun(t, s, p, m)
	hub := NewEventHub()
	commands := make(chan string)
	accepted := make(chan error, 1)
	client := checkpointClient{stream: func(ctx context.Context, _ CompletionRequest, emit func(CompletionEvent) error) error {
		for {
			select {
			case delta := <-commands:
				if delta == "done" {
					return emit(CompletionEvent{Done: true})
				}
				err := emit(CompletionEvent{Delta: delta})
				accepted <- err
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}}
	r, err := NewNativeRuntime(s, providers, client, &fakeTools{readOnly: true}, hub)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- r.Run(ctx, run.ID) }()
	for _, delta := range []string{strings.Repeat("x", 1024), "tail-test-key-visible"} {
		select {
		case commands <- delta:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		if err := <-accepted; err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.ConversationMessageMetadataPage(ctx, session.ID, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	durable := ""
	for _, message := range page.Items {
		if message.Role == RoleAssistant {
			durable = message.Content
		}
	}
	if len(durable) != 1024 {
		t.Fatalf("unexpected extra SQL checkpoint: %d", len(durable))
	}
	var draft Message
	ch, stop, err := hub.SnapshotSubscribe(run.ID, func(live *Message) error {
		if live == nil || !strings.Contains(live.Content, "[REDACTED]") || strings.Contains(live.Content, "test-key") || !strings.Contains(live.Content, "visible") {
			t.Errorf("missing or unsafe live prefix: %+v", live)
			return errors.New("live mismatch")
		}
		draft = *live
		items, id, err := ProjectLiveMessage(run, page.Items, live)
		if err != nil || id != draft.ID || items[len(items)-1].ID != draft.ID || items[len(items)-1].Content != draft.Content {
			t.Error("draft identity changed")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	commands <- "done"
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	completed := false
	for len(ch) > 0 {
		e := <-ch
		if e.Type == "message.completed" {
			message := e.Data.(Message)
			completed = message.ID == draft.ID && message.Content == draft.Content
		}
	}
	if !completed || len(hub.live) != 0 {
		t.Fatal("final message identity or cleanup failed")
	}
	// A fresh process has no live state and restores only durable text.
	_, stopFresh, err := NewEventHub().SnapshotSubscribe(run.ID, func(live *Message) error {
		if live != nil {
			t.Error("restart retained live data")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	stopFresh()
}

func TestEventHubLiveCapacityAndUnknownState(t *testing.T) {
	h := NewEventHub()
	for _, id := range []string{"a", "b"} {
		if err := h.beginMessage(Message{ID: id, RunID: id}); err != nil {
			t.Fatal(err)
		}
		if err := h.publishDelta(id, id, strings.Repeat("x", MaxAssistantBytes), strings.Repeat("x", MaxAssistantBytes)); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.beginMessage(Message{ID: "c", RunID: "c"}); err == nil {
		t.Fatal("third active prefix accepted")
	}
	if err := h.publishDelta("a", "a", strings.Repeat("x", MaxAssistantBytes+1), "x"); err == nil {
		t.Fatal("oversized live prefix accepted")
	}
	h.Publish(RunEvent{RunID: "a", Type: "run.cancelled"})
	h.Publish(RunEvent{RunID: "b", Type: "run.failed"})
	if len(h.live) != 0 {
		t.Fatal("terminal live prefix leaked")
	}
	h.invalidateSnapshots()
	if _, _, err := h.SnapshotSubscribe("a", func(*Message) error { t.Error("unknown snapshot was read"); return nil }); err == nil {
		t.Fatal("unknown terminal storage state accepted")
	}
}

func TestRuntimeTwoActiveLivePrefixesAndCancellationFlush(t *testing.T) {
	s, providers, p, m := runtimeFixture(t)
	defer s.Close()
	hub := NewEventHub()
	accepted := make(chan struct{}, 2)
	stopped := make(chan struct{}, 2)
	client := checkpointClient{stream: func(ctx context.Context, _ CompletionRequest, emit func(CompletionEvent) error) error {
		if err := emit(CompletionEvent{Delta: strings.Repeat("x", 1024)}); err != nil {
			return err
		}
		if err := emit(CompletionEvent{Delta: strings.Repeat("x", MaxAssistantBytes-1024)}); err != nil {
			return err
		}
		accepted <- struct{}{}
		<-ctx.Done()
		stopped <- struct{}{}
		return ctx.Err()
	}}
	r, err := NewNativeRuntime(s, providers, client, &fakeTools{readOnly: true}, hub)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	runs := make([]Run, 0, 2)
	finished := make(chan error, 2)
	for i := 0; i < 2; i++ {
		_, run := checkpointRun(t, s, p, m)
		runs = append(runs, run)
		go func() { finished <- r.Run(ctx, run.ID) }()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-accepted:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	runtime.ReadMemStats(&after)
	hub.mu.Lock()
	bytes := 0
	for _, message := range hub.live {
		bytes += len(message.Content)
	}
	active := len(hub.live)
	hub.mu.Unlock()
	t.Logf("active_runs=%d live_prefix_bytes=%d heap_alloc_delta=%d total_alloc_delta=%d", active, bytes, int64(after.HeapAlloc)-int64(before.HeapAlloc), after.TotalAlloc-before.TotalAlloc)
	if active != 2 || bytes != 2*MaxAssistantBytes {
		t.Fatal("two-worker live budget changed")
	}
	// Hold the only DB connection so cancellation cannot finish its final save.
	// A terminal publication must not clear the accepted live prefix early.
	s.db.SetMaxOpenConns(1)
	conn, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range runs {
		if err := r.Cancel(ctx, run.ID); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		select {
		case <-stopped:
		case <-ctx.Done():
			conn.Close()
			t.Fatal(ctx.Err())
		}
	}
	_, stop, err := hub.SnapshotSubscribe(runs[0].ID, func(live *Message) error {
		if live == nil || len(live.Content) != MaxAssistantBytes {
			t.Error("cancel published terminal before flush")
		}
		return nil
	})
	if err != nil {
		conn.Close()
		t.Fatal(err)
	}
	stop()
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := <-finished; !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel error=%v", err)
		}
	}
	if len(hub.live) != 0 {
		t.Fatal("cancelled live prefixes leaked")
	}
	for _, run := range runs {
		loaded, err := s.RunByID(ctx, run.ID)
		if err != nil || loaded.Status != RunCancelled {
			t.Fatalf("cancel status=%v error=%v", loaded.Status, err)
		}
	}
}

func TestEventHubSnapshotReadDeadlineReleasesPublishers(t *testing.T) {
	s, _, _, _ := runtimeFixture(t)
	defer s.Close()
	s.db.SetMaxOpenConns(1)
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	h := NewEventHub()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err = h.SnapshotSubscribe("run", func(*Message) error { _, err := s.RunByID(ctx, "run"); return err })
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("read deadline=%v elapsed=%s", err, time.Since(start))
	}
	done := make(chan struct{})
	go func() { h.Publish(RunEvent{RunID: "run", Type: "run.failed"}); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("snapshot error retained hub lock")
	}
	if len(h.subscribers) != 0 {
		t.Fatal("failed snapshot subscription leaked")
	}
}

func TestConversationSnapshotRefreshesAfterMetadataPreload(t *testing.T) {
	s, _, p, m := runtimeFixture(t)
	defer s.Close()
	ctx := context.Background()
	session, run := checkpointRun(t, s, p, m)
	draft, err := s.AddMessage(ctx, Message{SessionID: session.ID, RunID: run.ID, Role: RoleAssistant, Content: "checkpoint"})
	if err != nil {
		t.Fatal(err)
	}
	cached, err := s.ConversationMessageMetadataPage(ctx, session.ID, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	draft.Content = "complete-after-preload"
	if _, err := s.AddMessage(ctx, draft); err != nil {
		t.Fatal(err)
	}
	user, err := s.AddMessage(ctx, Message{SessionID: session.ID, Role: RoleUser, Content: "late-input", Attachments: []Attachment{{Name: "late.txt", MimeType: "text/plain", Kind: "text", Data: []byte("late")}}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.ConversationSnapshotMessagePage(ctx, session.ID, cached.Items)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]Message{}
	for _, message := range page.Items {
		if _, duplicate := seen[message.ID]; duplicate {
			t.Fatal("snapshot duplicated an ID")
		}
		seen[message.ID] = message
	}
	if seen[draft.ID].Content != draft.Content || len(seen[user.ID].Attachments) != 1 || seen[user.ID].Attachments[0].Size != 4 || seen[user.ID].Attachments[0].Data != nil {
		t.Fatal("preload hid newer text or attachment metadata")
	}
}
