package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type channelTestSender struct {
	messages map[Provider][]string
	mu       sync.Mutex
	counts   map[Provider]int
	fail     map[Provider]bool
	entered  chan Provider
	release  chan struct{}
}

func TestChannelStatePreservesLegacyJSON(t *testing.T) {
	const legacy = `{"tokenFingerprint":"saved","hasChat":true,"chatId":123,"status":"ready"}`
	var state channelState
	if err := json.Unmarshal([]byte(legacy), &state); err != nil {
		t.Fatal(err)
	}
	if !state.hasDestination() || state.ChatID != 123 || state.Status != ChannelReady {
		t.Fatal("legacy destination state was not retained")
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var restored telegramState
	if err := json.Unmarshal(data, &restored); err != nil || !restored.HasChat || restored.Status != TelegramReady {
		t.Fatal("generic channel state changed the legacy JSON contract")
	}
}

func (s *channelTestSender) SendMessage(ctx context.Context, provider Provider, credential, message string) error {
	s.mu.Lock()
	s.counts[provider]++
	fail := s.fail[provider]
	if !fail {
		if s.messages == nil {
			s.messages = map[Provider][]string{}
		}
		s.messages[provider] = append(s.messages[provider], message)
	}
	s.mu.Unlock()
	if s.entered != nil {
		s.entered <- provider
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if fail {
		return ErrChannelUnavailable
	}
	return nil
}
func (s *channelTestSender) count(provider Provider) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counts[provider]
}

func setupMultiChannels(t *testing.T) (*Service, *notificationHostSource, *notificationTestClock, *channelTestSender) {
	t.Helper()
	s, source, clock := localHistoryService(t)
	sender := &channelTestSender{counts: map[Provider]int{}, fail: map[Provider]bool{}}
	s.shoutrrr = sender
	for provider, credential := range map[Provider]string{ProviderNtfy: "ntfy://:@ntfy.example/ops", ProviderGotify: "gotify://gotify.example/A12345678901234"} {
		if _, err := s.UpdateChannel(context.Background(), ChannelUpdateInput{Provider: provider, Credential: credential, Enabled: true, ExpectedResourceVersion: s.Snapshot().ResourceVersion}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Configure(context.Background(), UpdateInput{Enabled: true, Rules: s.Snapshot().Rules, ExpectedResourceVersion: s.Snapshot().ResourceVersion}); err != nil {
		t.Fatal(err)
	}
	return s, source, clock, sender
}

func TestMultiChannelPartialRetryAndRestart(t *testing.T) {
	s, source, clock, sender := setupMultiChannels(t)
	sender.fail[ProviderGotify] = true
	tickHistory(t, s, clock, 3)
	page, err := s.History(HistoryQuery{Rule: "cpu"})
	if err != nil || len(page.Items) != 1 || page.Items[0].Delivery != "partial" || len(page.Items[0].Deliveries) != 2 {
		t.Fatalf("history=%+v err=%v", page, err)
	}
	if sender.count(ProviderNtfy) != 2 || sender.count(ProviderGotify) != 2 {
		t.Fatal("each channel must get validation and one alert")
	}
	restarted, err := NewService(Config{DataDir: filepath.Dir(s.store.directory), Hosts: source, Now: clock.Now, Shoutrrr: sender})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	sender.fail[ProviderGotify] = false
	clock.Advance(5 * time.Minute)
	tickHistory(t, restarted, clock, 1)
	page, _ = restarted.History(HistoryQuery{Rule: "cpu"})
	if page.Items[0].Delivery != "sent" || sender.count(ProviderNtfy) != 2 || sender.count(ProviderGotify) != 3 {
		t.Fatalf("retry duplicated successful channel: %+v", page.Items)
	}
	data, _ := json.Marshal(restarted.Snapshot())
	if strings.Contains(string(data), "abcdefgh") || strings.Contains(string(data), "ntfy.example") || strings.Contains(string(data), "fingerprint") {
		t.Fatalf("snapshot exposed secrets: %s", data)
	}
}

func TestMultiChannelSendsConcurrently(t *testing.T) {
	s, _, clock, sender := setupMultiChannels(t)
	sender.entered = make(chan Provider, 2)
	sender.release = make(chan struct{})
	done := make(chan error, 1)
	go func() {
		for range 3 {
			if err := s.evaluate(context.Background()); err != nil {
				done <- err
				return
			}
			clock.Advance(30 * time.Second)
		}
		done <- nil
	}()
	defer close(sender.release)
	for range 2 {
		select {
		case <-sender.entered:
		case <-time.After(time.Second):
			t.Fatal("one channel blocked the other")
		}
	}
	sender.release <- struct{}{}
	sender.release <- struct{}{}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestMultiChannelMigrationPreservesLegacyAndPendingBinding(t *testing.T) {
	s, _, clock := localHistoryService(t)
	tg := &notificationTestTelegram{}
	s.telegram = tg
	if _, err := s.Configure(context.Background(), UpdateInput{Enabled: true, Rules: s.Snapshot().Rules, TelegramBotToken: testBotToken, ExpectedResourceVersion: s.Snapshot().ResourceVersion}); err != nil {
		t.Fatal(err)
	}
	tg.sendErr = ErrTelegramUnavailable
	tickHistory(t, s, clock, 3)
	sender := &channelTestSender{counts: map[Provider]int{}, fail: map[Provider]bool{}}
	s.shoutrrr = sender
	before := s.Snapshot().ResourceVersion
	if _, err := s.UpdateChannel(context.Background(), ChannelUpdateInput{Provider: ProviderNtfy, Enabled: true, Credential: "ntfy://:@ntfy.example/ops", ExpectedResourceVersion: before}); err != nil {
		t.Fatal(err)
	}
	if len(s.Snapshot().Channels) != 2 {
		t.Fatal("legacy channel lost")
	}
	if _, err := s.UpdateChannel(context.Background(), ChannelUpdateInput{Provider: ProviderNtfy, Enabled: false, ExpectedResourceVersion: before}); !errors.Is(err, ErrConflict) {
		t.Fatal("stale settings accepted")
	}
	tg.sendErr = nil
	clock.Advance(5 * time.Minute)
	tickHistory(t, s, clock, 1)
	if tg.messageCount() != 1 || sender.count(ProviderNtfy) != 1 {
		t.Fatal("old pending alert was sent to newly configured recipient")
	}
}

func TestMultiChannelDisableAndChangeCancelOnlyTheirPendingDelivery(t *testing.T) {
	for _, mode := range []string{"disable", "remove", "replace"} {
		t.Run(mode, func(t *testing.T) {
			s, _, clock, sender := setupMultiChannels(t)
			sender.fail[ProviderGotify] = true
			tickHistory(t, s, clock, 3)
			sender.fail[ProviderGotify] = false
			input := ChannelUpdateInput{Provider: ProviderGotify, ExpectedResourceVersion: s.Snapshot().ResourceVersion}
			if mode == "remove" {
				input.Remove = true
			}
			if mode == "replace" {
				input.Enabled = true
				input.Credential = "gotify://gotify.example/A23456789012345"
			}
			if _, err := s.UpdateChannel(context.Background(), input); err != nil {
				t.Fatal(err)
			}
			before := sender.count(ProviderGotify)
			clock.Advance(5 * time.Minute)
			tickHistory(t, s, clock, 1)
			page, _ := s.History(HistoryQuery{Rule: "cpu"})
			if sender.count(ProviderGotify) != before || sender.count(ProviderNtfy) != 2 {
				t.Fatal("cancelled or successful delivery retried")
			}
			statuses := map[Provider]string{}
			for _, d := range page.Items[0].Deliveries {
				statuses[d.Provider] = d.Status
			}
			if statuses[ProviderGotify] != "cancelled" || statuses[ProviderNtfy] != "sent" {
				t.Fatalf("statuses=%v", statuses)
			}
		})
	}
}

func TestMultiChannelFailedStateWritePreservesConfiguration(t *testing.T) {
	s, _, _, sender := setupMultiChannels(t)
	before := s.Snapshot()
	original := s.store.statePath
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.store.statePath = filepath.Join(blocked, "state")
	_, err := s.UpdateChannel(context.Background(), ChannelUpdateInput{Provider: ProviderGotify, Credential: "gotify://gotify.example/A23456789012345", Enabled: true, ExpectedResourceVersion: before.ResourceVersion})
	s.store.statePath = original
	if err == nil || !strings.Contains(err.Error(), "after_delivery") || s.Snapshot().ResourceVersion != before.ResourceVersion || sender.count(ProviderGotify) != 2 {
		t.Fatalf("save failure=%v", err)
	}
}

func TestMultiChannelServiceChecksUseIndependentDurableDelivery(t *testing.T) {
	checks, host, tg, clock := checkAlertFixture(t)
	sender := &channelTestSender{counts: map[Provider]int{}, fail: map[Provider]bool{}}
	checks.parent.shoutrrr = sender
	_, err := checks.parent.UpdateChannel(context.Background(), ChannelUpdateInput{Provider: ProviderNtfy, Enabled: true, Credential: "ntfy://:@ntfy.example/ops", ExpectedResourceVersion: checks.parent.Snapshot().ResourceVersion})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		setCheckObservation(host, clock.Now(), uint64(i), "down", i)
		evaluateChecks(t, checks)
		clock.Advance(30 * time.Second)
	}
	tg.sendErr = ErrTelegramUnavailable
	deliverChecks(t, checks)
	page, err := checks.parent.History(HistoryQuery{Rule: "service-checks"})
	if err != nil || len(page.Items) != 1 || page.Items[0].Delivery != "partial" {
		t.Fatalf("checks history=%+v err=%v", page, err)
	}
	if sender.count(ProviderNtfy) != 2 {
		t.Fatal("missing service-check notification")
	}
	clock.Advance(5 * time.Minute)
	setCheckObservation(host, clock.Now(), 4, "down", 4)
	tg.sendErr = nil
	deliverChecks(t, checks)
	page, _ = checks.parent.History(HistoryQuery{Rule: "service-checks"})
	if len(page.Items) != 1 || page.Items[0].Delivery != "sent" || sender.count(ProviderNtfy) != 2 {
		t.Fatal("service-check retry duplicated a successful delivery")
	}
}

func TestMultiChannelServiceCheckRecoveryCoalescesEachDestination(t *testing.T) {
	for _, allFailed := range []bool{true, false} {
		t.Run(fmt.Sprint(allFailed), func(t *testing.T) {
			checks, host, tg, clock := checkAlertFixture(t)
			sender := &channelTestSender{counts: map[Provider]int{}, fail: map[Provider]bool{}}
			checks.parent.shoutrrr = sender
			if _, err := checks.parent.UpdateChannel(context.Background(), ChannelUpdateInput{Provider: ProviderNtfy, Enabled: true, Credential: "ntfy://:@ntfy.example/ops", ExpectedResourceVersion: checks.parent.Snapshot().ResourceVersion}); err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= 3; i++ {
				setCheckObservation(host, clock.Now(), uint64(i), "down", i)
				evaluateChecks(t, checks)
				clock.Advance(30 * time.Second)
			}
			if allFailed {
				tg.sendErr = ErrTelegramUnavailable
			}
			sender.fail[ProviderNtfy] = true
			deliverChecks(t, checks)
			tg.sendErr = nil
			sender.fail[ProviderNtfy] = false
			setCheckObservation(host, clock.Now(), 4, "up", 2)
			evaluateChecks(t, checks)
			deliverChecks(t, checks)
			page, _ := checks.parent.History(HistoryQuery{Rule: "service-checks"})
			if len(page.Items) != 2 || page.Items[0].Kind != "recovery" || page.Items[0].Delivery != "sent" {
				t.Fatalf("history=%+v", page.Items)
			}
			ntfyMessages := sender.messages[ProviderNtfy]
			if !strings.Contains(ntfyMessages[len(ntfyMessages)-1], "曾异常") {
				t.Fatal("undelivered channel got a bare recovery")
			}
			tgMessages := tg.messagesSnapshot()
			if allFailed && !strings.Contains(tgMessages[len(tgMessages)-1], "曾异常") {
				t.Fatal("undelivered Telegram got a bare recovery")
			}
			if !allFailed && !strings.Contains(tgMessages[len(tgMessages)-1], "服务恢复") {
				t.Fatal("successful Telegram lost its recovery")
			}
			before := tg.messageCount()
			ntfyBefore := sender.count(ProviderNtfy)
			clock.Advance(5 * time.Minute)
			setCheckObservation(host, clock.Now(), 5, "up", 3)
			deliverChecks(t, checks)
			if tg.messageCount() != before || sender.count(ProviderNtfy) != ntfyBefore {
				t.Fatal("obsolete failure delivered after recovery")
			}
		})
	}
}
