package notification

import (
	"context"
	"sync"
	"time"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

// Caller holds opMu while the bounded batch runs, including parallel sends.
func (s *Service) deliverChannels(ctx context.Context, state persistedState, hosts []cluster.Host, now time.Time) error {
	history, err := s.history.snapshot()
	if err != nil {
		return historyError(err)
	}
	channels := map[Provider]storedChannel{}
	for _, ch := range state.Channels {
		channels[ch.Provider] = ch
	}
	present := map[string]cluster.Host{}
	for _, host := range hosts {
		present[host.ID] = host
	}
	expiries := s.expirySnapshot()
	limits := s.trafficLimitsSnapshot()
	type attempt struct {
		eventID  string
		provider Provider
		message  string
		channel  storedChannel
		err      error
	}
	batch := []attempt{}
	for i := range history.Events {
		event := &history.Events[i]
		// Existing pending events migrate only to their original channel. New
		// destinations must never receive pre-configuration historical messages.
		if len(event.Deliveries) == 0 && (event.Delivery == "pending" || event.Delivery == "failed") {
			if !validProvider(event.Provider) || !validHexString(event.ChannelFingerprint, 64) {
				event.Delivery = "cancelled"
				event.LastErrorCode = "channel_changed"
				continue
			}
			event.Deliveries = []ChannelDelivery{{Provider: event.Provider, Status: event.Delivery, Attempts: event.Attempts, LastAttemptAt: cloneTime(event.LastAttemptAt), LastErrorCode: event.LastErrorCode}}
			event.ChannelFingerprints = map[Provider]string{event.Provider: event.ChannelFingerprint}
		}
		if len(event.Deliveries) == 0 {
			continue
		}
		host, exists := present[event.HostID]
		rules := effectiveTrafficRules(state.Settings.Rules, limits[event.HostID])
		reason := ""
		switch {
		case !state.Settings.Enabled:
			reason = "push_disabled"
		case event.Rule == serverExpiryRuleKey && expiries[event.HostID].ExpiresOn != event.ExpiryDate:
			reason = "expiry_reminder_changed"
		case !ruleEnabled(rules, event.Rule):
			reason = "rule_disabled"
		case isCumulativeTrafficRule(event.Rule) && event.TrafficThresholdGiB != trafficThreshold(rules, event.Rule):
			reason = "traffic_threshold_changed"
		case !exists:
			reason = "host_removed"
		case isCumulativeTrafficRule(event.Rule) && (host.TrafficPeriod == nil || host.TrafficPeriod.Available) && event.TrafficCycle != trafficCycleKey(host):
			reason = "traffic_cycle_changed"
		case now.Sub(event.CreatedAt) >= 24*time.Hour:
			reason = "retry_expired"
		}
		if event.Rule == "service-checks" && reason == "" {
			fresh, changed := checkEventFresh(*event, host, now)
			if changed {
				reason = "check_changed"
			} else if !fresh || (host.State != cluster.HostOnline && host.State != cluster.HostDegraded) {
				continue
			} else if checkEventSuperseded(*event, host) {
				reason = "check_state_changed"
			}
		}
		for j := range event.Deliveries {
			d := &event.Deliveries[j]
			if d.Status != "pending" && d.Status != "failed" {
				continue
			}
			ch, configured := channels[d.Provider]
			cancelReason := reason
			if cancelReason == "" {
				switch {
				case !configured:
					cancelReason = "channel_removed"
				case !ch.Enabled:
					cancelReason = "channel_disabled"
				case event.ChannelFingerprints[d.Provider] != channelFingerprint(ch):
					cancelReason = "channel_changed"
				}
			}
			if cancelReason != "" {
				d.Status = "cancelled"
				d.LastErrorCode = cancelReason
				continue
			}
			if isCumulativeTrafficRule(event.Rule) && host.TrafficPeriod != nil && !host.TrafficPeriod.Available {
				continue
			}
			if !ch.Meta.hasDestination() || len(batch) >= maxMessagesPerEvaluation || (d.LastAttemptAt != nil && now.Sub(*d.LastAttemptAt) < alertRetryInterval) {
				continue
			}
			d.Attempts++
			d.LastAttemptAt = timePtr(now)
			message := event.Message
			if d.Message != "" {
				message = d.Message
			}
			batch = append(batch, attempt{eventID: event.ID, provider: d.Provider, message: message, channel: ch})
		}
		aggregateDelivery(event)
	}
	// Attempts are durable before any network call, including parallel sends.
	if err := s.history.commit(history); err != nil {
		return historyError(err)
	}
	if len(batch) == 0 {
		return nil
	}
	history, err = s.history.snapshot()
	if err != nil {
		return historyError(err)
	}
	retained := map[string]bool{}
	for _, event := range history.Events {
		retained[event.ID] = true
	}
	var wg sync.WaitGroup
	// At most eight sends per evaluation, at most three in flight. A failing
	// destination cannot prevent the other destinations from being attempted.
	semaphore := make(chan struct{}, 3)
	groups := map[Provider][]int{}
	for i := range batch {
		if retained[batch[i].eventID] {
			groups[batch[i].provider] = append(groups[batch[i].provider], i)
		}
	}
	for _, indices := range groups {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				for _, i := range indices {
					batch[i].err = ctx.Err()
				}
				return
			}
			defer func() { <-semaphore }()
			for _, i := range indices {
				item := &batch[i]
				sendCtx, cancel := context.WithTimeout(ctx, channelSendTimeout)
				item.err = s.sendChannel(sendCtx, item.provider, item.channel.Credential, item.channel.Meta, item.message)
				cancel()
			}
		}()
	}
	wg.Wait()
	for _, item := range batch {
		if !retained[item.eventID] {
			continue
		}
		for i := range history.Events {
			event := &history.Events[i]
			if event.ID != item.eventID {
				continue
			}
			for j := range event.Deliveries {
				d := &event.Deliveries[j]
				if d.Provider != item.provider {
					continue
				}
				d.Status = "sent"
				d.LastErrorCode = ""
				if item.err != nil {
					d.Status = "failed"
					d.LastErrorCode = channelCode(item.provider, item.err)
				}
			}
			aggregateDelivery(event)
		}
		for i := range state.Channels {
			ch := &state.Channels[i]
			if ch.Provider != item.provider {
				continue
			}
			ch.Meta.LastCheckedAt = timePtr(now)
			ch.Meta.LastErrorCode = ""
			ch.Meta.Status = ChannelReady
			if item.err != nil {
				ch.Meta.Status = ChannelError
				ch.Meta.LastErrorCode = channelCode(item.provider, item.err)
			} else {
				ch.Meta.LastSuccessAt = timePtr(now)
			}
		}
	}
	if err := s.history.commit(history); err != nil {
		return historyError(err)
	}
	return s.store.commitState(state)
}
