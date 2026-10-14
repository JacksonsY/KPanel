package notification

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const MaxChannels = 10

type ChannelDelivery struct {
	Message       string     `json:"message,omitempty"`
	Provider      Provider   `json:"provider"`
	Status        string     `json:"status"`
	Attempts      int        `json:"attempts"`
	LastAttemptAt *time.Time `json:"lastAttemptAt,omitempty"`
	LastErrorCode string     `json:"lastErrorCode,omitempty"`
}

// Each supported provider has one independently enabled destination. Credentials
// are stored only in the atomic 0600 state file, never returned in snapshots.
type storedChannel struct {
	Binding    string       `json:"binding,omitempty"`
	Provider   Provider     `json:"provider"`
	Enabled    bool         `json:"enabled"`
	Credential string       `json:"credential"`
	Meta       channelState `json:"meta"`
}

type ChannelUpdateInput struct {
	Provider                Provider `json:"provider"`
	Enabled                 bool     `json:"enabled"`
	Credential              string   `json:"credential,omitempty"`
	Remove                  bool     `json:"remove,omitempty"`
	ExpectedResourceVersion string   `json:"expectedResourceVersion"`
}

type ChannelActionInput struct {
	Provider                Provider `json:"provider"`
	ExpectedResourceVersion string   `json:"expectedResourceVersion"`
}

func validateChannels(state persistedState) error {
	if len(state.Channels) > MaxChannels || (!state.MultiChannel && len(state.Channels) > 0) {
		return ErrInvalidSettings
	}
	seen := map[Provider]bool{}
	for _, channel := range state.Channels {
		if channel.Binding != "" && !validHexString(channel.Binding, 64) {
			return ErrInvalidSettings
		}
		if seen[channel.Provider] || !ValidChannelCredential(channel.Provider, channel.Credential) ||
			!validTelegramStatus(channel.Meta.Status) || channel.Meta.TokenFingerprint != tokenFingerprint(channel.Credential) {
			return ErrInvalidSettings
		}
		seen[channel.Provider] = true
	}
	return nil
}

func channelsVersion(state persistedState) string {
	type binding struct {
		Binding     string
		Provider    Provider
		Enabled     bool
		Fingerprint string
		ChatID      int64
	}
	bindings := []binding{}
	for _, ch := range state.Channels {
		bindings = append(bindings, binding{channelFingerprint(ch), ch.Provider, ch.Enabled, ch.Meta.TokenFingerprint, ch.Meta.ChatID})
	}
	data, _ := json.Marshal(struct {
		Settings Settings
		Channels []binding
	}{state.Settings, bindings})
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Migration is committed together with the first channel edit. A failed save
// leaves the legacy file and state usable; merely reading settings writes nothing.
func (s *Service) configuredChannels(state persistedState) ([]storedChannel, error) {
	if state.MultiChannel {
		return state.Channels, nil
	}
	credential, present, err := s.store.credential()
	if err != nil {
		return nil, err
	}
	if !present {
		return []storedChannel{}, nil
	}
	provider, _ := DetectProvider(credential)
	return []storedChannel{{Provider: provider, Enabled: true, Credential: credential, Meta: state.Telegram}}, nil
}

func (s *Service) channelSnapshots(state persistedState) []ChannelSnapshot {
	channels, err := s.configuredChannels(state)
	if err != nil {
		return []ChannelSnapshot{}
	}
	result := make([]ChannelSnapshot, 0, len(channels))
	for _, ch := range channels {
		item := snapshotFromState(state, ch.Meta, true, ch.Provider, "").Channel
		item.Enabled = ch.Enabled
		result = append(result, item)
	}
	return result
}

func (s *Service) UpdateChannel(ctx context.Context, input ChannelUpdateInput) (Snapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	state := s.store.stateSnapshot()
	if input.ExpectedResourceVersion == "" || input.ExpectedResourceVersion != state.ResourceVersion {
		return Snapshot{}, ErrConflict
	}
	if !validProvider(input.Provider) {
		return Snapshot{}, ErrUnsupportedProvider
	}
	channels, err := s.configuredChannels(state)
	if errors.Is(err, ErrInvalidCredential) && strings.TrimSpace(input.Credential) != "" && !input.Remove {
		channels = []storedChannel{}
		err = nil
	}
	if err != nil {
		return Snapshot{}, &Error{Code: "credential_file_unavailable", Cause: err}
	}
	index := -1
	for i := range channels {
		if channels[i].Provider == input.Provider {
			index = i
			break
		}
	}
	if input.Remove {
		if index >= 0 {
			channels = append(channels[:index], channels[index+1:]...)
		}
	} else {
		ch := storedChannel{Provider: input.Provider}
		if index >= 0 {
			ch = channels[index]
		}
		credential := strings.TrimSpace(input.Credential)
		delivered := false
		if credential != "" {
			if !ValidChannelCredential(input.Provider, credential) {
				return Snapshot{}, ErrInvalidCredential
			}
			meta, sent, err := s.connectChannel(ctx, input.Provider, credential, state.Settings.Locale)
			if err != nil {
				return Snapshot{}, err
			}
			ch.Credential, ch.Meta = credential, meta
			delivered = sent
		}
		if ch.Credential == "" {
			return Snapshot{}, ErrCredentialRequired
		}
		ch.Enabled = input.Enabled
		if credential != "" || (input.Enabled && (index < 0 || !channels[index].Enabled)) {
			ch.Binding = tokenFingerprint(ch.Credential + rand.Text())
		}
		if index < 0 {
			channels = append(channels, ch)
		} else {
			channels[index] = ch
		}
		state.MultiChannel = true
		state.Channels = channels
		state.ResourceVersion = channelsVersion(state)
		state.UpdatedAt = s.now().UTC()
		if err := s.store.commitState(state); err != nil {
			return Snapshot{}, channelStoreError("state_store_unavailable", delivered, err)
		}
		return s.snapshot(ctx), nil
	}
	state.MultiChannel = true
	state.Channels = channels
	state.ResourceVersion = channelsVersion(state)
	state.UpdatedAt = s.now().UTC()
	if err := s.store.commitState(state); err != nil {
		return Snapshot{}, channelStoreError("state_store_unavailable", false, err)
	}
	return s.snapshot(ctx), nil
}

func (s *Service) ChannelAction(ctx context.Context, input ChannelActionInput, discover bool) (Snapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	state := s.store.stateSnapshot()
	if input.ExpectedResourceVersion == "" || input.ExpectedResourceVersion != state.ResourceVersion {
		return Snapshot{}, ErrConflict
	}
	channels, err := s.configuredChannels(state)
	if err != nil {
		return Snapshot{}, &Error{Code: "credential_file_unavailable", Cause: err}
	}
	index := -1
	for i := range channels {
		if channels[i].Provider == input.Provider {
			index = i
			break
		}
	}
	if index < 0 {
		return Snapshot{}, ErrChannelNotConfigured
	}
	ch := &channels[index]
	if discover {
		if ch.Provider != ProviderTelegram {
			return Snapshot{}, ErrDiscoverUnsupported
		}
		ch.Meta, _, err = s.connectChannel(ctx, ch.Provider, ch.Credential, state.Settings.Locale)
		if err == nil {
			ch.Binding = tokenFingerprint(ch.Credential + rand.Text())
		}
	} else {
		if !ch.Meta.hasDestination() {
			return Snapshot{}, ErrChannelNotReady
		}
		err = s.sendChannel(ctx, ch.Provider, ch.Credential, ch.Meta, testMessage(s.displayTime(ctx, s.now()), state.Settings.Locale, ch.Provider))
	}
	if err != nil {
		return Snapshot{}, err
	}
	ch.Meta.Status = ChannelReady
	ch.Meta.LastCheckedAt = timePtr(s.now().UTC())
	ch.Meta.LastSuccessAt = timePtr(s.now().UTC())
	ch.Meta.LastErrorCode = ""
	state.MultiChannel = true
	state.Channels = channels
	state.ResourceVersion = channelsVersion(state)
	if err := s.store.commitState(state); err != nil {
		return Snapshot{}, channelStoreError("state_store_unavailable", !discover, err)
	}
	return s.snapshot(ctx), nil
}

func (s *Service) configureMultiSettings(ctx context.Context, state persistedState, input UpdateInput, rules Rules, locale string) (Snapshot, error) {
	if input.ChannelCredential != "" || input.TelegramBotToken != "" {
		return Snapshot{}, ErrProviderMismatch
	}
	state.Settings = Settings{Enabled: input.Enabled, Locale: locale, Rules: rules}
	state.ResourceVersion = channelsVersion(state)
	state.UpdatedAt = s.now().UTC()
	if err := s.store.commitState(state); err != nil {
		return Snapshot{}, channelStoreError("state_store_unavailable", false, err)
	}
	return s.snapshot(ctx), nil
}

func multiCanDeliver(state persistedState) bool {
	if !state.Settings.Enabled {
		return false
	}
	for _, ch := range state.Channels {
		if ch.Enabled {
			return true
		}
	}
	return false
}

func prepareChannelDeliveries(event *storedEvent, state persistedState) {
	if !state.MultiChannel {
		return
	}
	event.Provider = ""
	event.ChannelFingerprint = ""
	event.Delivery = "local_only"
	if !state.Settings.Enabled {
		return
	}
	event.ChannelFingerprints = map[Provider]string{}
	for _, ch := range state.Channels {
		if !ch.Enabled {
			continue
		}
		event.Deliveries = append(event.Deliveries, ChannelDelivery{Provider: ch.Provider, Status: "pending"})
		event.ChannelFingerprints[ch.Provider] = channelFingerprint(ch)
	}
	if len(event.Deliveries) > 0 {
		event.Delivery = "pending"
	}
}

func channelFingerprint(ch storedChannel) string {
	if ch.Binding != "" {
		return ch.Binding
	}
	return tokenFingerprint(ch.Credential)
}

func aggregateDelivery(event *storedEvent) {
	if len(event.Deliveries) == 0 {
		return
	}
	counts := map[string]int{}
	event.Attempts = 0
	event.LastAttemptAt = nil
	event.LastErrorCode = ""
	for _, d := range event.Deliveries {
		counts[d.Status]++
		event.Attempts += d.Attempts
		if d.LastAttemptAt != nil && (event.LastAttemptAt == nil || d.LastAttemptAt.After(*event.LastAttemptAt)) {
			event.LastAttemptAt = cloneTime(d.LastAttemptAt)
		}
		if d.LastErrorCode != "" {
			event.LastErrorCode = d.LastErrorCode
		}
	}
	switch {
	case counts["sent"] == len(event.Deliveries):
		event.Delivery = "sent"
	case counts["sent"] > 0:
		event.Delivery = "partial"
	case counts["failed"] > 0:
		event.Delivery = "failed"
	case counts["pending"] > 0:
		event.Delivery = "pending"
	default:
		event.Delivery = "cancelled"
	}
}

func validateChannelDeliveries(event storedEvent) error {
	if len(event.CheckKey) > 160 || len(event.CheckRevision) > 80 || (event.CheckDeliveryID != 0 && (event.Rule != "service-checks" || event.CheckKey == "" || event.CheckRevision == "")) {
		return ErrInvalidSettings
	}
	if len(event.Deliveries) > MaxChannels || len(event.ChannelFingerprints) > MaxChannels {
		return errors.New("too many channel deliveries")
	}
	seen := map[Provider]bool{}
	for _, d := range event.Deliveries {
		if len(d.Message) > 4096 {
			return ErrInvalidSettings
		}
		if seen[d.Provider] || !validProvider(d.Provider) || !validDelivery(d.Status) || d.Status == "partial" ||
			d.Attempts < 0 || d.Attempts > 300 || len(d.LastErrorCode) > 80 || !validHexString(event.ChannelFingerprints[d.Provider], 64) {
			return errors.New("invalid channel delivery")
		}
		seen[d.Provider] = true
	}
	return nil
}
