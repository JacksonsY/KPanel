package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTelegramBindingRequiresSessionChallengeAndPreservesExistingDestination(t *testing.T) {
	clock := &notificationTestClock{now: time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)}
	command := "/start"
	proofChat := int64(91)
	sends := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":42,"is_bot":true,"username":"kpanel_bot"}}`))
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []any{
				map[string]any{"update_id": 1, "message": map[string]any{"text": command, "chat": map[string]any{"id": proofChat, "type": "private"}}},
				map[string]any{"update_id": 2, "message": map[string]any{"text": "/start", "chat": map[string]any{"id": 666, "type": "private"}}},
			}})
		default:
			sends++
			t.Error("binding must never send a message")
		}
	}))
	defer api.Close()
	s, err := NewService(Config{DataDir: t.TempDir(), Hosts: newNotificationTestHost(clock.Now()), Now: clock.Now, Telegram: newTelegramClientForTest(api.URL, api.Client())})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := WithBindingSession(context.Background(), "session-a")
	other := WithBindingSession(context.Background(), "session-b")
	snap, err := s.UpdateChannel(ctx, ChannelUpdateInput{Provider: ProviderTelegram, Credential: testBotToken, Enabled: true, ExpectedResourceVersion: s.Snapshot().ResourceVersion})
	if err != nil || snap.Telegram.Ready || snap.TelegramBinding == nil {
		t.Fatalf("initial save must wait for proof: %+v %v", snap, err)
	}
	if s.SnapshotForSession(other).TelegramBinding != nil {
		t.Fatal("challenge leaked to another admin session")
	}
	first := snap.TelegramBinding.Command
	command = first
	// Expiry replaces the challenge before reading old Telegram updates.
	clock.now = clock.now.Add(6 * time.Minute)
	snap, err = s.ChannelAction(ctx, ChannelActionInput{Provider: ProviderTelegram, ExpectedResourceVersion: snap.ResourceVersion}, true)
	if err != nil || snap.Telegram.Ready || snap.TelegramBinding == nil || snap.TelegramBinding.Command == first {
		t.Fatalf("expired proof accepted: %+v %v", snap, err)
	}
	command = snap.TelegramBinding.Command
	snap, err = s.ChannelAction(ctx, ChannelActionInput{Provider: ProviderTelegram, ExpectedResourceVersion: snap.ResourceVersion}, true)
	if err != nil || !snap.Telegram.Ready || snap.TelegramBinding != nil {
		t.Fatalf("valid proof not bound: %+v %v", snap, err)
	}
	channels, _ := s.configuredChannels(s.store.stateSnapshot())
	if channels[0].Meta.ChatID != 91 {
		t.Fatal("unrelated latest /start stole the destination")
	}
	proofChat = 999
	snap, err = s.ChannelAction(ctx, ChannelActionInput{Provider: ProviderTelegram, ExpectedResourceVersion: snap.ResourceVersion}, true)
	channels, _ = s.configuredChannels(s.store.stateSnapshot())
	if err != nil || channels[0].Meta.ChatID != 91 || !snap.Telegram.Ready || snap.TelegramBinding == nil {
		t.Fatal("consumed proof reused or existing binding lost")
	}
	old := snap.TelegramBinding.Command
	snap, err = s.UpdateChannel(ctx, ChannelUpdateInput{Provider: ProviderTelegram, Credential: testBotToken, Enabled: true, ExpectedResourceVersion: snap.ResourceVersion})
	if err != nil || snap.TelegramBinding == nil || snap.TelegramBinding.Command == old || snap.Telegram.Ready {
		t.Fatal("credential resave must revoke old challenge and destination until fresh proof")
	}
	if sends != 0 {
		t.Fatal("unexpected message sends")
	}
}

func TestTelegramProofMustRemainValidUntilConsumption(t *testing.T) {
	now := time.Now()
	s := &Service{now: func() time.Time { return now }}
	ctx := WithBindingSession(context.Background(), "session-a")
	proof := s.bindingContext(ctx, testBotToken, true)
	now = now.Add(5 * time.Minute)
	if s.consumeBinding(proof) {
		t.Fatal("proof expiring during discovery was consumed")
	}
	proof = s.bindingContext(ctx, testBotToken, true)
	other := WithBindingSession(proof, "session-b")
	if s.consumeBinding(other) {
		t.Fatal("proof transferred to another administrator session")
	}
	if !s.consumeBinding(proof) || s.consumeBinding(proof) {
		t.Fatal("proof must be consumed once")
	}
}
