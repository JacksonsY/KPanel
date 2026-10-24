package notification

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

type checkChannelTarget struct {
	Provider    Provider `json:"provider"`
	Fingerprint string   `json:"fingerprint"`
}

// Move service-check intents to the same bounded durable history used by host
// alerts. The source ID makes the two-file handoff idempotent after a crash.
func (s *CheckAlerts) enqueueChannelHistory(ctx context.Context) error {
	// Caller holds parent.opMu through the handoff and actual sends.
	state := s.parent.store.stateSnapshot()
	if !state.Settings.Enabled || !state.Settings.Rules.ServiceChecksEnabled {
		return nil
	}
	fetch, cancel := context.WithTimeout(ctx, channelSendTimeout)
	hosts := s.parent.hosts.Hosts(fetch)
	cancel()
	present := map[string]cluster.Host{}
	for _, host := range hosts.Items {
		present[host.ID] = host
	}
	now := s.parent.now()
	s.mu.Lock()
	next := cloneCheckDisk(s.state)
	history, err := s.parent.history.snapshot()
	if err != nil {
		s.mu.Unlock()
		return historyError(err)
	}
	keys := []string{}
	for key := range next.Incidents {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	count := 0
	for _, key := range keys {
		item := next.Incidents[key]
		if item.Pending == nil || count >= maxMessagesPerEvaluation {
			continue
		}
		pending := item.Pending
		host, exists := present[item.HostID]
		if !exists || (host.State != cluster.HostOnline && host.State != cluster.HostDegraded) || !freshForCheckDelivery(key, item, host, now) {
			continue
		}
		found := false
		for _, event := range history.Events {
			if event.CheckDeliveryID == pending.ID && event.CheckKey == key {
				found = true
				break
			}
		}
		if !found {
			history.Sequence++
			kind := "alert"
			if pending.Kind == "recovery" || pending.Kind == "resolved" || pending.Kind == "resumed" {
				kind = "recovery"
			}
			event := storedEvent{Event: Event{ID: fmt.Sprint(history.Sequence), HostID: item.HostID, HostName: safeMessageText(item.HostName), IsLocal: item.HostID == cluster.LocalHostID, CreatedAt: pending.CreatedAt.UTC(), Rule: "service-checks", Kind: kind, Message: serviceCheckMessage(item, *pending, s.parent.displayTime(ctx, now), state.Settings.Locale), Delivery: "local_only"}, CheckDeliveryID: pending.ID, CheckKey: key, CheckRevision: item.Revision, ChannelFingerprints: map[Provider]string{}}
			if event.HostName == "" {
				event.HostName = item.HostID
			}
			targets := pending.Targets
			if !pending.MultiChannel && len(targets) == 0 {
				event.Delivery = "cancelled"
				event.LastErrorCode = "channel_changed"
			}
			for _, target := range targets {
				event.Deliveries = append(event.Deliveries, ChannelDelivery{Provider: target.Provider, Status: "pending"})
				event.ChannelFingerprints[target.Provider] = target.Fingerprint
			}
			if pending.Kind == "recovery" || pending.Kind == "resolved" {
				coalesceCheckRecovery(&history, &event, item, *pending, s.parent.displayTime(ctx, now), state.Settings.Locale)
			}
			aggregateDelivery(&event)
			history.Events = append(history.Events, event)
		}
		item.Delivered = item.Active
		item.LastSent = now
		item.Pending = nil
		next.Incidents[key] = item
		count++
	}
	pruneHistory(&history, now)
	if err := s.parent.history.commit(history); err != nil {
		s.mu.Unlock()
		return historyError(err)
	}
	if err := s.commit(next); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	return s.parent.deliverChannels(ctx, state, hosts.Items, now)
}

func checkEventFresh(event storedEvent, host cluster.Host, now time.Time) (bool, bool) {
	if event.CheckRevision == "sampler" {
		return true, false
	}
	if host.LastSnapshot == nil || host.LastSnapshot.Telemetry.ServiceChecks == nil || !host.LastSnapshot.Telemetry.ServiceChecks.Available {
		return false, false
	}
	for _, sample := range host.LastSnapshot.Telemetry.ServiceChecks.Items {
		if event.CheckKey == host.ID+":"+sample.ID {
			if event.CheckRevision != sample.Revision {
				return false, true
			}
			return freshForCheckDelivery(event.CheckKey, checkIncident{Revision: event.CheckRevision}, host, now), false
		}
	}
	return false, true
}

func coalesceCheckRecovery(history *historyState, event *storedEvent, item checkIncident, pending checkDelivery, now time.Time, locale string) {
	sent := map[Provider]bool{}
	for i := len(history.Events) - 1; i >= 0; i-- {
		previous := &history.Events[i]
		if previous.CheckKey != event.CheckKey || previous.CheckRevision != event.CheckRevision {
			continue
		}
		if previous.Kind == "recovery" {
			break
		}
		for j := range previous.Deliveries {
			d := &previous.Deliveries[j]
			if previous.ChannelFingerprints[d.Provider] == event.ChannelFingerprints[d.Provider] && d.Status == "sent" {
				sent[d.Provider] = true
			}
			if d.Status == "pending" || d.Status == "failed" {
				d.Status = "cancelled"
				d.LastErrorCode = "check_recovered"
			}
		}
		aggregateDelivery(previous)
	}
	for i := range event.Deliveries {
		d := &event.Deliveries[i]
		if !sent[d.Provider] {
			resolved := pending
			resolved.Kind = "resolved"
			d.Message = serviceCheckMessage(item, resolved, now, locale)
		}
	}
}

func checkEventSuperseded(event storedEvent, host cluster.Host) bool {
	if host.LastSnapshot == nil || host.LastSnapshot.Telemetry.ServiceChecks == nil {
		return false
	}
	for _, sample := range host.LastSnapshot.Telemetry.ServiceChecks.Items {
		if event.CheckKey == host.ID+":"+sample.ID && event.CheckRevision == sample.Revision {
			return (event.Kind == "alert" && sample.State == "up" && sample.Successes >= 2) || (event.Kind == "recovery" && sample.State == "down" && sample.Failures >= 3 && sample.CheckedAt.After(event.CreatedAt))
		}
	}
	return false
}
