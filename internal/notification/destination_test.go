package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHistoryRetryDoesNotFollowVerifiedTelegramRecipient(t *testing.T) {
	for _, multi := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "multi"}[multi], func(t *testing.T) {
			s, _, clock := localHistoryService(t)
			command, chat := "/start", int64(91)
			sends := []int64{}
			var apiMu sync.Mutex
			setProof := func(value string, recipient int64) {
				apiMu.Lock()
				command, chat = value, recipient
				apiMu.Unlock()
			}
			sentChats := func() []int64 {
				apiMu.Lock()
				defer apiMu.Unlock()
				return append([]int64(nil), sends...)
			}
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiMu.Lock()
				defer apiMu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/getMe"):
					_, _ = w.Write([]byte(`{"ok":true,"result":{"id":42,"is_bot":true,"username":"kpanel_bot"}}`))
				case strings.HasSuffix(r.URL.Path, "/getUpdates"):
					_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []any{map[string]any{"update_id": 1, "message": map[string]any{"text": command, "chat": map[string]any{"id": chat, "type": "private"}}}}})
				default:
					var body struct {
						ChatID int64 `json:"chat_id"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					sends = append(sends, body.ChatID)
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte(`{"ok":false,"error_code":503,"description":"unavailable"}`))
				}
			}))
			defer api.Close()
			s.telegram = newTelegramClientForTest(api.URL, api.Client())
			ctx := WithBindingSession(context.Background(), "outbox-owner")
			if multi {
				if _, err := s.UpdateChannel(ctx, ChannelUpdateInput{Provider: ProviderTelegram, Enabled: true, Credential: testBotToken, ExpectedResourceVersion: s.Snapshot().ResourceVersion}); err != nil {
					t.Fatal(err)
				}
			} else if _, err := s.Configure(ctx, UpdateInput{TelegramBotToken: testBotToken, Rules: s.Snapshot().Rules, ExpectedResourceVersion: s.Snapshot().ResourceVersion}); err != nil {
				t.Fatal(err)
			}
			discover := func() {
				t.Helper()
				var err error
				if multi {
					_, err = s.ChannelAction(ctx, ChannelActionInput{Provider: ProviderTelegram, ExpectedResourceVersion: s.Snapshot().ResourceVersion}, true)
				} else {
					_, err = s.Discover(ctx, s.Snapshot().ResourceVersion)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			setProof(s.SnapshotForSession(ctx).TelegramBinding.Command, 91)
			discover()
			if _, err := s.Configure(ctx, UpdateInput{Enabled: true, Rules: s.Snapshot().Rules, ExpectedResourceVersion: s.Snapshot().ResourceVersion}); err != nil {
				t.Fatal(err)
			}
			tickHistory(t, s, clock, 3)
			page, _ := s.History(HistoryQuery{Rule: "cpu"})
			if calls := sentChats(); len(page.Items) != 1 || page.Items[0].Delivery != "failed" || len(calls) != 1 || calls[0] != 91 {
				t.Fatalf("original attempt missing: %+v, calls=%v", page.Items, calls)
			}
			setProof("/start", 999)
			discover() // A consumed proof cannot select the new recipient.
			setProof(s.SnapshotForSession(ctx).TelegramBinding.Command, 999)
			discover()
			channels, err := s.configuredChannels(s.store.stateSnapshot())
			if err != nil || channels[0].Meta.ChatID != 999 {
				t.Fatal("new session proof did not bind the new recipient")
			}
			clock.Advance(5 * time.Minute)
			tickHistory(t, s, clock, 1)
			page, _ = s.History(HistoryQuery{Rule: "cpu"})
			if calls := sentChats(); len(calls) != 1 || page.Items[0].Delivery != "cancelled" || page.Items[0].LastErrorCode != "channel_changed" {
				t.Fatalf("old event followed recipient: %+v, calls=%v", page.Items, calls)
			}
		})
	}
}

func TestLegacyHistoryUnknownDestinationCancelsDuringMigration(t *testing.T) {
	for _, migrate := range []bool{false, true} {
		for _, oldFingerprint := range []string{"", tokenFingerprint(testBotToken)} {
			label := "unbound"
			if oldFingerprint != "" {
				label = "token-only"
			}
			t.Run(map[bool]string{false: "legacy", true: "migrate"}[migrate]+"/"+label, func(t *testing.T) {
				s, _, clock := localHistoryService(t)
				tg := &notificationTestTelegram{sendErr: ErrTelegramUnavailable}
				s.telegram = tg
				if _, err := s.Configure(context.Background(), UpdateInput{Enabled: true, TelegramBotToken: testBotToken, Rules: s.Snapshot().Rules, ExpectedResourceVersion: s.Snapshot().ResourceVersion}); err != nil {
					t.Fatal(err)
				}
				tickHistory(t, s, clock, 3)
				history, _ := s.history.snapshot()
				if len(history.Events) != 1 {
					t.Fatal("missing failed event")
				}
				history.Events[0].ChannelFingerprint = oldFingerprint
				if err := s.history.commit(history); err != nil {
					t.Fatal(err)
				}
				tg.sendErr = nil
				if migrate {
					s.shoutrrr = &channelTestSender{counts: map[Provider]int{}, fail: map[Provider]bool{}}
					if _, err := s.UpdateChannel(context.Background(), ChannelUpdateInput{Provider: ProviderNtfy, Enabled: true, Credential: "ntfy://:@ntfy.example/ops", ExpectedResourceVersion: s.Snapshot().ResourceVersion}); err != nil {
						t.Fatal(err)
					}
				}
				clock.Advance(5 * time.Minute)
				tickHistory(t, s, clock, 1)
				page, _ := s.History(HistoryQuery{Rule: "cpu"})
				if tg.messageCount() != 0 || page.Items[0].Delivery != "cancelled" || page.Items[0].LastErrorCode != "channel_changed" {
					t.Fatalf("unknown legacy target authorized delivery: %+v", page.Items)
				}
			})
		}
	}
}

func TestCheckOutboxPinsDestinationAcrossRestartAndMigration(t *testing.T) {
	for _, mode := range []string{"retry", "replace", "reverify", "unbound", "token-only", "migrate-bound", "migrate-unbound"} {
		t.Run(mode, func(t *testing.T) {
			checks, host, tg, clock := checkAlertFixture(t)
			setCheckObservation(host, clock.Now(), 3, "down", 3)
			evaluateChecks(t, checks)
			key := host.host.ID + ":health"
			next := cloneCheckDisk(checks.state)
			pending := next.Incidents[key].Pending
			if pending == nil || len(pending.Targets) != 1 {
				t.Fatal("queue did not persist a destination")
			}
			if mode == "unbound" || mode == "migrate-unbound" {
				pending.Targets = nil
			} else if mode == "token-only" {
				pending.Targets[0].Fingerprint = tokenFingerprint(testBotToken)
			}
			if err := checks.commit(next); err != nil {
				t.Fatal(err)
			}
			var err error
			checks, err = NewCheckAlerts(checks.parent)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "replace" || mode == "reverify" {
				credential := testBotToken
				if mode == "replace" {
					credential += "b"
				}
				if _, err := checks.parent.Configure(context.Background(), UpdateInput{Enabled: true, TelegramBotToken: credential, Rules: checks.parent.Snapshot().Rules, ExpectedResourceVersion: checks.parent.Snapshot().ResourceVersion}); err != nil {
					t.Fatal(err)
				}
			}
			sender := &channelTestSender{counts: map[Provider]int{}, fail: map[Provider]bool{}}
			if strings.HasPrefix(mode, "migrate-") {
				checks.parent.shoutrrr = sender
				if _, err := checks.parent.UpdateChannel(context.Background(), ChannelUpdateInput{Provider: ProviderNtfy, Enabled: true, Credential: "ntfy://:@ntfy.example/ops", ExpectedResourceVersion: checks.parent.Snapshot().ResourceVersion}); err != nil {
					t.Fatal(err)
				}
			}
			deliverChecks(t, checks)
			want := 0
			if mode == "retry" || mode == "migrate-bound" {
				want = 1
			}
			if tg.messageCount() != want || checks.state.Incidents[key].Pending != nil {
				t.Fatalf("unexpected delivery after %s: sends=%d pending=%+v", mode, tg.messageCount(), checks.state.Incidents[key].Pending)
			}
			if strings.HasPrefix(mode, "migrate-") && sender.count(ProviderNtfy) != 1 {
				t.Fatal("new channel received a pre-configuration incident")
			}
		})
	}
}

type blockedDestinationTelegram struct {
	notificationTestTelegram
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockedDestinationTelegram) SendMessage(ctx context.Context, credential string, chat int64, message string) error {
	b.once.Do(func() { close(b.entered) })
	select {
	case <-b.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return b.notificationTestTelegram.SendMessage(ctx, credential, chat, message)
}

func TestCheckSendSerializesConfigurationThroughNetworkCall(t *testing.T) {
	checks, host, _, clock := checkAlertFixture(t)
	setCheckObservation(host, clock.Now(), 3, "down", 3)
	evaluateChecks(t, checks)
	blocking := &blockedDestinationTelegram{entered: make(chan struct{}), release: make(chan struct{})}
	checks.parent.telegram = blocking
	sent := make(chan error, 1)
	go func() { sent <- checks.deliver(context.Background()) }()
	select {
	case <-blocking.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("send did not begin")
	}
	snapshot := checks.parent.Snapshot()
	configured, started := make(chan error, 1), make(chan struct{})
	go func() {
		close(started)
		_, err := checks.parent.Configure(context.Background(), UpdateInput{Enabled: true, TelegramBotToken: testBotToken + "b", Rules: snapshot.Rules, ExpectedResourceVersion: snapshot.ResourceVersion})
		configured <- err
	}()
	<-started
	select {
	case err := <-configured:
		close(blocking.release)
		t.Fatalf("configuration changed while old destination was in flight: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(blocking.release)
	for _, done := range []chan error{sent, configured} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("send or configuration did not finish")
		}
	}
	if blocking.messageCount() != 1 {
		t.Fatal("in-flight delivery was replayed")
	}
}
