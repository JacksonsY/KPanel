package ai

import (
	"errors"
	"sort"
	"sync"
	"time"
)

const maxLiveMessages = 2 // Same global upper bound as NativeRuntime workers.

// EventHub retains only the current public assistant prefix, never provider
// state or an event replay log. Prefix strings alias the runtime's append-only
// builder; old snapshots remain immutable when the runtime appends more text.
type EventHub struct {
	mu          sync.Mutex
	subscribers map[string]map[chan RunEvent]struct{}
	live        map[string]Message
	unavailable bool
}

func NewEventHub() *EventHub {
	return &EventHub{subscribers: make(map[string]map[chan RunEvent]struct{}), live: make(map[string]Message)}
}

func (h *EventHub) subscribeLocked(runID string) (<-chan RunEvent, func()) {
	channel := make(chan RunEvent, 32)
	if h.subscribers[runID] == nil {
		h.subscribers[runID] = make(map[chan RunEvent]struct{})
	}
	h.subscribers[runID][channel] = struct{}{}
	return channel, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.removeLocked(runID, channel)
	}
}

func (h *EventHub) removeLocked(runID string, channel chan RunEvent) {
	if _, ok := h.subscribers[runID][channel]; ok {
		delete(h.subscribers[runID], channel)
		close(channel)
		if len(h.subscribers[runID]) == 0 {
			delete(h.subscribers, runID)
		}
	}
}

func (h *EventHub) Subscribe(runID string) (<-chan RunEvent, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.subscribeLocked(runID)
}

// SnapshotSubscribe establishes a cut through the public stream. The callback
// must do only bounded local reads (the HTTP caller uses a two-second context),
// never network writes, Publish, or acquire a runtime lock. Store operations do
// not call the hub while holding a transaction, so there is no reverse lock edge.
// On callback failure no subscription is created.
func (h *EventHub) SnapshotSubscribe(runID string, snapshot func(*Message) error) (<-chan RunEvent, func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.unavailable {
		return nil, nil, errors.New("AI live state unavailable until restart")
	}
	var current *Message
	if message, ok := h.live[runID]; ok && message.Content != "" {
		current = &message
	}
	if err := snapshot(current); err != nil {
		return nil, nil, err
	}
	channel, stop := h.subscribeLocked(runID)
	return channel, stop, nil
}

func (h *EventHub) beginMessage(message Message) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.live[message.RunID]; exists || len(h.live) >= maxLiveMessages || h.unavailable {
		return errors.New("live assistant message capacity unavailable")
	}
	h.live[message.RunID] = message
	return nil
}

// Only NativeRuntime calls this after streamPublicCompletion has filtered the
// provider credential. No raw provider/native/tool payload reaches live state.
func (h *EventHub) publishDelta(runID, messageID, content, delta string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	message, ok := h.live[runID]
	if !ok || message.ID != messageID || len(content) > MaxAssistantBytes || len(content) != len(message.Content)+len(delta) {
		return errors.New("invalid live assistant message boundary")
	}
	message.Content = content
	h.live[runID] = message
	h.publishLocked(RunEvent{Type: "message.delta", RunID: runID, Data: map[string]string{"delta": delta}})
	return nil
}

func (h *EventHub) clearLive(runID string) {
	h.mu.Lock()
	delete(h.live, runID)
	h.mu.Unlock()
}

// If even a terminal status cannot be saved, durable reconnect is unknown.
// Retaining unbounded terminal tombstones is not safe; fail closed until restart
// instead, when store recovery marks unfinished durable runs interrupted.
func (h *EventHub) invalidateSnapshots() {
	h.mu.Lock()
	h.unavailable = true
	for runID, channels := range h.subscribers {
		for channel := range channels {
			h.removeLocked(runID, channel)
		}
	}
	h.mu.Unlock()
}

func (h *EventHub) snapshotsUnavailable() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.unavailable
}

// ProjectLiveMessage uses the history page's existing UTF-8 budget, replacing
// the durable draft by ID. Keep the complete current draft and the newest whole
// history messages; never append another 1 MiB outside the page's 2 MiB budget.
func ProjectLiveMessage(run Run, messages []Message, live *Message) ([]Message, string, error) {
	if live == nil {
		return messages, "", nil
	}
	if live.ID == "" || live.RunID != run.ID || live.SessionID != run.SessionID || live.ProviderID != run.ProviderID || live.ProviderName != run.ProviderName || live.ModelID != run.ModelID || live.ModelName != run.ModelName || live.Role != RoleAssistant || live.ToolCallID != "" || len(live.Content) > MaxAssistantBytes || len(live.Attachments) != 0 {
		return nil, "", errors.New("live assistant message ownership mismatch")
	}
	items := make([]Message, 0, len(messages)+1)
	for _, message := range messages {
		if message.ID != live.ID {
			items = append(items, message)
		}
	}
	items = append(items, *live)
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	bytes, count := len(live.Content), 1
	keep := make([]bool, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].ID == live.ID {
			keep[i] = true
			continue
		}
		if bytes+len(items[i].Content) > 2<<20 || count >= 50 {
			break
		}
		keep[i] = true
		bytes += len(items[i].Content)
		count++
	}
	result := make([]Message, 0, count)
	for i, item := range items {
		if keep[i] || item.ID == live.ID {
			result = append(result, item)
		}
	}
	return result, live.ID, nil
}

func (h *EventHub) Publish(event RunEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if event.Type == "message.completed" {
		if message, ok := event.Data.(Message); ok && message.ID == h.live[event.RunID].ID {
			delete(h.live, event.RunID)
		}
	}
	if event.Type == "run.completed" || event.Type == "run.failed" || event.Type == "run.cancelled" {
		delete(h.live, event.RunID)
	}
	h.publishLocked(event)
}

func (h *EventHub) publishLocked(event RunEvent) {
	event.At = time.Now().UTC()
	for channel := range h.subscribers[event.RunID] {
		select {
		case channel <- event:
		default:
			// Closing, rather than silently dropping a delta/terminal event,
			// forces native EventSource to reconnect and fetch a current snapshot.
			h.removeLocked(event.RunID, channel)
		}
	}
}
