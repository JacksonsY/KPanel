package notification

// A bounded durable outbox follows the global service notification rule.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kejilion/kejilion-panel/internal/cluster"
	"github.com/kejilion/kejilion-panel/internal/contract"
)

const maxCheckIncidents = (cluster.MaxHosts + 1) * (contract.MaxServiceChecks + 1)

type checkTarget struct {
	HostID   string
	CheckID  string
	Revision string
}

type checkDelivery struct {
	MultiChannel bool                 `json:"multiChannel,omitempty"`
	Targets      []checkChannelTarget `json:"targets,omitempty"`
	ID           uint64               `json:"id"`
	Kind         string               `json:"kind"`
	CreatedAt    time.Time            `json:"createdAt"`
	NextAttempt  time.Time            `json:"nextAttempt"`
	Attempts     int                  `json:"attempts"`
}
type checkIncident struct {
	HostID    string         `json:"hostId"`
	HostName  string         `json:"hostName"`
	Name      string         `json:"name"`
	Target    string         `json:"target"`
	Revision  string         `json:"revision"`
	Epoch     string         `json:"epoch"`
	Sequence  uint64         `json:"sequence"`
	CheckedAt time.Time      `json:"checkedAt"`
	Interval  int            `json:"interval"`
	Active    bool           `json:"active"`
	Delivered bool           `json:"delivered"`
	LastSent  time.Time      `json:"lastSent"`
	Pending   *checkDelivery `json:"pending,omitempty"`
}
type checkAlertDisk struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Generation    uint64                   `json:"generation"`
	Incidents     map[string]checkIncident `json:"incidents"`
}
type CheckAlerts struct {
	parent      *Service
	mu          sync.Mutex
	state       checkAlertDisk
	path        string
	lastError   string
	loggedError string
	logger      *slog.Logger
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func NewCheckAlerts(parent *Service) (*CheckAlerts, error) {
	s := &CheckAlerts{parent: parent, logger: slog.Default(), path: filepath.Join(parent.store.directory, "service-check-alerts-v1.json"),
		state: checkAlertDisk{SchemaVersion: 1, Incidents: map[string]checkIncident{}}}
	data, err := readRegularFile(s.path, maxStateBytes, false)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	// Earlier local candidates stored subscriptions here. Accept their envelope
	// without turning a narrow subscription into a fleet-wide opt-in.
	var disk struct {
		checkAlertDisk
		LegacySettings json.RawMessage `json:"settings,omitempty"`
	}
	if err = d.Decode(&disk); err != nil {
		return nil, err
	}
	s.state = disk.checkAlertDisk
	var extra any
	if d.Decode(&extra) != io.EOF || s.state.SchemaVersion != 1 || len(s.state.Incidents) > maxCheckIncidents || s.state.Incidents == nil {
		return nil, errors.New("invalid service check notification state")
	}
	for key, value := range s.state.Incidents {
		if value.Pending != nil {
			seen := map[Provider]bool{}
			if len(value.Pending.Targets) > MaxChannels || (!value.Pending.MultiChannel && len(value.Pending.Targets) > 1) {
				return nil, ErrInvalidSettings
			}
			for _, target := range value.Pending.Targets {
				if seen[target.Provider] || !validProvider(target.Provider) || !validHexString(target.Fingerprint, 64) {
					return nil, ErrInvalidSettings
				}
				seen[target.Provider] = true
			}
		}
		if len(key) > 160 || !validDisplayText(value.HostID, 80) || !validDisplayText(value.Name, 192) || len(value.Target) > 253 || len(value.HostName) > 256 || value.Pending != nil && (value.Pending.ID == 0 || value.Pending.Attempts < 0 || value.Pending.Attempts > 20 || !validCheckDelivery(value.Pending.Kind)) {
			return nil, errors.New("invalid service check incident")
		}
	}
	return s, nil
}

func validCheckDelivery(kind string) bool {
	switch kind {
	case "down", "recovery", "resolved", "repeat", "unknown", "resumed":
		return true
	}
	return false
}
func cloneCheckDisk(value checkAlertDisk) checkAlertDisk {
	items := make(map[string]checkIncident, len(value.Incidents))
	for key, item := range value.Incidents {
		if item.Pending != nil {
			copy := *item.Pending
			copy.Targets = append([]checkChannelTarget(nil), copy.Targets...)
			item.Pending = &copy
		}
		items[key] = item
	}
	value.Incidents = items
	return value
}
func (s *CheckAlerts) commit(next checkAlertDisk) error {
	if reflect.DeepEqual(next, s.state) {
		return nil
	}
	data, err := json.Marshal(next)
	if err != nil || len(data) > int(maxStateBytes) || len(next.Incidents) > maxCheckIncidents {
		s.lastError = "state_limit"
		return errors.New("service check state exceeds limit")
	}
	if err := atomicWrite(s.path, data, 0o600); err != nil {
		s.lastError = "storage_unavailable"
		return err
	}
	s.state = next
	if s.lastError == "storage_unavailable" || s.lastError == "state_limit" {
		s.lastError = ""
	}
	return nil
}
func (s *CheckAlerts) Start(parent context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.wg.Add(2)
	go func() {
		defer s.wg.Done()
		timer := time.NewTicker(30 * time.Second)
		defer timer.Stop()
		for {
			_ = s.evaluate(ctx)
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
		}
	}()
	go func() {
		defer s.wg.Done()
		timer := time.NewTicker(30 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				_ = s.deliver(ctx)
			}
		}
	}()
}
func (s *CheckAlerts) Close() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.mu.Unlock()
	s.wg.Wait()
}

// Targets are derived each round, so future hosts/checks need no settings save.
func currentCheckTargets(hosts []cluster.Host, incidents map[string]checkIncident) []checkTarget {
	targets := make([]checkTarget, 0)
	for _, host := range hosts {
		var summary *contract.ServiceCheckSummary
		if host.LastSnapshot != nil {
			summary = host.LastSnapshot.Telemetry.ServiceChecks
		}
		online := host.State == cluster.HostOnline || host.State == cluster.HostDegraded
		if online && summary != nil && summary.Available {
			for _, check := range summary.Items {
				targets = append(targets, checkTarget{HostID: host.ID, CheckID: check.ID, Revision: check.Revision})
			}
			continue
		}
		// Retain pending events until a fresh catalog can prove a target was deleted.
		found := false
		for key, item := range incidents {
			if item.HostID == host.ID && item.Revision != "sampler" {
				targets = append(targets, checkTarget{HostID: host.ID, CheckID: strings.TrimPrefix(key, host.ID+":"), Revision: item.Revision})
				found = true
			}
		}
		_, tracked := incidents[host.ID+":@sampler"]
		if !found && (tracked || summary != nil && !summary.Available) {
			targets = append(targets, checkTarget{HostID: host.ID, CheckID: "@sampler", Revision: "sampler"})
		}
	}
	return targets
}

func (s *CheckAlerts) evaluate(ctx context.Context) error {
	defer s.reportErrorChange()
	s.parent.opMu.Lock()
	defer s.parent.opMu.Unlock()
	settings := s.parent.store.stateSnapshot().Settings
	s.mu.Lock()
	if !settings.Enabled || !settings.Rules.ServiceChecksEnabled {
		next := cloneCheckDisk(s.state)
		next.Incidents = map[string]checkIncident{}
		err := s.commit(next)
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	fetch, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	hosts := s.parent.hosts.Hosts(fetch)
	now := s.parent.now()
	state := s.parent.store.stateSnapshot()
	settings = state.Settings
	channels, err := s.parent.configuredChannels(state)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneCheckDisk(s.state)
	if !settings.Enabled || !settings.Rules.ServiceChecksEnabled {
		next.Incidents = map[string]checkIncident{}
		return s.commit(next)
	}
	hostMap := map[string]cluster.Host{}
	alive := map[string]bool{}
	sampler := map[string]bool{}
	freshAt := map[string]time.Time{}
	for _, host := range hosts.Items {
		hostMap[host.ID] = host
	}
	queue := func(item *checkIncident, kind string) {
		next.Generation++
		item.Pending = &checkDelivery{ID: next.Generation, Kind: kind, CreatedAt: now, NextAttempt: now}
		item.Pending.MultiChannel = state.MultiChannel
		for _, ch := range channels {
			if ch.Enabled {
				item.Pending.Targets = append(item.Pending.Targets, checkChannelTarget{ch.Provider, channelFingerprint(ch)})
			}
		}
	}
	for _, sub := range currentCheckTargets(hosts.Items, next.Incidents) {
		key := sub.HostID + ":" + sub.CheckID
		alive[key] = true
		host, exists := hostMap[sub.HostID]
		if !exists {
			delete(next.Incidents, key)
			continue
		}
		item := next.Incidents[key]
		if item.Revision != sub.Revision {
			item = checkIncident{HostID: host.ID, Revision: sub.Revision}
		}
		// Offline hosts own their parent alert. Pause delivery; retain an unsent recovery.
		if host.State != cluster.HostOnline && host.State != cluster.HostDegraded {
			alive[host.ID+":@sampler"] = true
			if item.Name != "" {
				next.Incidents[key] = item
			}
			continue
		}
		if host.LastSnapshot == nil || host.LastSnapshot.Telemetry.ServiceChecks == nil {
			if _, supported := next.Incidents[host.ID+":@sampler"]; supported {
				sampler[host.ID] = true
			}
			if item.Name != "" {
				next.Incidents[key] = item
			}
			continue
		}
		summary := host.LastSnapshot.Telemetry.ServiceChecks
		// An unreadable checks file has no catalog. Only an available catalog
		// can authoritatively say that a monitored target was deleted.
		if !summary.Available {
			sampler[host.ID] = true
			if item.Name != "" {
				next.Incidents[key] = item
			}
			continue
		}
		var sample *contract.ServiceCheckStatus
		for i := range summary.Items {
			if summary.Items[i].ID == sub.CheckID {
				sample = &summary.Items[i]
				break
			}
		}
		if sample == nil || sample.Revision != sub.Revision {
			delete(next.Incidents, key)
			continue
		}
		item.HostName = host.Name
		item.Name = sample.Name
		item.Target = sample.Target
		if _, exists := sampler[host.ID]; !exists {
			sampler[host.ID] = false
		}
		if sample.State == "unknown" || sample.CheckedAt.IsZero() || now.Sub(sample.CheckedAt) > time.Duration(2*summary.IntervalSeconds)*time.Second+time.Minute {
			sampler[host.ID] = true
			next.Incidents[key] = item
			continue
		}
		if item.Epoch == summary.Epoch && summary.Sequence <= item.Sequence {
			if freshAt[host.ID].IsZero() || sample.CheckedAt.Before(freshAt[host.ID]) {
				freshAt[host.ID] = sample.CheckedAt
			}
			continue
		}
		// A restarted sampler cannot replay an older observation into a recovery.
		if !item.CheckedAt.IsZero() && !sample.CheckedAt.After(item.CheckedAt) {
			continue
		}
		item.Epoch = summary.Epoch
		if freshAt[host.ID].IsZero() || sample.CheckedAt.Before(freshAt[host.ID]) {
			freshAt[host.ID] = sample.CheckedAt
		}
		item.Sequence = summary.Sequence
		item.CheckedAt = sample.CheckedAt
		if sample.State == "down" && sample.Failures >= 3 {
			if !item.Active {
				item.Active = true
				item.Delivered = false
				queue(&item, "down")
			} else if item.Pending == nil && !item.Delivered {
				queue(&item, "down")
			}
		} else if sample.State == "up" && sample.Successes >= 2 && item.Active {
			item.Active = false
			if item.Delivered {
				queue(&item, "recovery")
			} else {
				queue(&item, "resolved")
			}
		}
		next.Incidents[key] = item
	}
	// One sampler alert per host, regardless of the number of configured services.
	for hostID, unavailable := range sampler {
		host := hostMap[hostID]
		key := hostID + ":@sampler"
		alive[key] = true
		item := next.Incidents[key]
		if item.Revision == "" {
			item = checkIncident{HostID: hostID, HostName: host.Name, Name: "服务检测数据", Revision: "sampler", CheckedAt: now, Interval: 300}
		}
		if host.LastSnapshot != nil && host.LastSnapshot.Telemetry.ServiceChecks != nil {
			item.Interval = host.LastSnapshot.Telemetry.ServiceChecks.IntervalSeconds
		}
		if !unavailable && !freshAt[hostID].IsZero() {
			item.CheckedAt = freshAt[hostID]
		}
		stale := unavailable && now.Sub(item.CheckedAt) > time.Duration(2*item.Interval)*time.Second+time.Minute
		if stale && !item.Active {
			item.Active = true
			item.Delivered = false
			queue(&item, "unknown")
		}
		if !unavailable && item.Active {
			item.Active = false
			if item.Delivered {
				queue(&item, "resumed")
			} else {
				item.Pending = nil
			}
		}
		next.Incidents[key] = item
	}
	for key := range next.Incidents {
		if !alive[key] {
			delete(next.Incidents, key)
		}
	}
	return s.commit(next)
}

// The operation lock serializes destination configuration with queueing and
// sending. Store/state locks are released before bounded network I/O.
func (s *CheckAlerts) deliver(ctx context.Context) error {
	defer s.reportErrorChange()
	s.parent.opMu.Lock()
	defer s.parent.opMu.Unlock()
	settings := s.parent.store.stateSnapshot()
	if settings.MultiChannel {
		return s.enqueueChannelHistory(ctx)
	}
	credential, configured, err := s.parent.store.credential()
	if err != nil || !configured || !settings.Settings.Enabled || !settings.Settings.Rules.ServiceChecksEnabled || !settings.Telegram.HasChat || settings.Telegram.TokenFingerprint != tokenFingerprint(credential) {
		return nil
	}
	provider, _ := DetectProvider(credential)
	now := s.parent.now()
	destination := legacyDestination(provider, credential, settings.Telegram)
	s.mu.Lock()
	next := cloneCheckDisk(s.state)
	for key, item := range next.Incidents {
		if pending := item.Pending; pending != nil && (pending.MultiChannel || len(pending.Targets) != 1 ||
			pending.Targets[0].Provider != provider || pending.Targets[0].Fingerprint != destination) {
			// An unbound legacy intent cannot be reconstructed from today's
			// configuration. Cancel it instead of selecting a new recipient.
			item.Pending = nil
			next.Incidents[key] = item
		}
	}
	if err := s.commit(next); err != nil {
		s.mu.Unlock()
		return err
	}
	keys := []string{}
	for key, item := range s.state.Incidents {
		if item.Pending != nil && !item.Pending.NextAttempt.After(now) {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := s.state.Incidents[keys[i]].Pending, s.state.Incidents[keys[j]].Pending
		if a.NextAttempt.Equal(b.NextAttempt) {
			return keys[i] < keys[j]
		}
		return a.NextAttempt.Before(b.NextAttempt)
	})
	s.mu.Unlock()
	if len(keys) == 0 {
		return nil
	}
	fetch, cancelFetch := context.WithTimeout(ctx, 8*time.Second)
	hosts := s.parent.hosts.Hosts(fetch)
	cancelFetch()
	hostMap := map[string]cluster.Host{}
	for _, host := range hosts.Items {
		hostMap[host.ID] = host
	}
	attempted := 0
	consumed := map[string]bool{}
	displayNow := s.parent.displayTime(ctx, now)
	for _, key := range keys {
		if consumed[key] {
			continue
		}
		if attempted >= 8 || ctx.Err() != nil {
			break
		}
		s.mu.Lock()
		item, ok := s.state.Incidents[key]
		if !ok || item.Pending == nil {
			s.mu.Unlock()
			continue
		}
		pending := *item.Pending
		s.mu.Unlock()
		latest := s.parent.store.stateSnapshot()
		if !latest.Settings.Enabled || latest.ResourceVersion != settings.ResourceVersion || latest.Telegram.TokenFingerprint != settings.Telegram.TokenFingerprint {
			return nil
		}
		host, exists := hostMap[item.HostID]
		if !exists || (host.State != cluster.HostOnline && host.State != cluster.HostDegraded) {
			continue
		}
		if !freshForCheckDelivery(key, item, host, now) {
			continue
		}
		message := serviceCheckMessage(item, pending, displayNow, settings.Settings.Locale)
		batch := map[string]uint64{key: pending.ID}
		consumed[key] = true
		// Combine events for one observer host within all providers' text limits.
		s.mu.Lock()
		for _, otherKey := range keys {
			other, exists := s.state.Incidents[otherKey]
			if consumed[otherKey] || !exists || other.HostID != item.HostID || other.Pending == nil || other.Pending.NextAttempt.After(now) || !freshForCheckDelivery(otherKey, other, host, now) {
				continue
			}
			part := serviceCheckMessage(other, *other.Pending, displayNow, settings.Settings.Locale)
			if len(message)+len(part)+2 > 1800 {
				continue
			}
			message += "\n\n" + part
			batch[otherKey] = other.Pending.ID
			consumed[otherKey] = true
		}
		s.mu.Unlock()
		attempted++
		send, cancel := context.WithTimeout(ctx, channelSendTimeout)
		err = s.parent.sendChannel(send, provider, credential, settings.Telegram, message)
		cancel()
		s.mu.Lock()
		next := cloneCheckDisk(s.state)
		for batchKey, eventID := range batch {
			current, ok := next.Incidents[batchKey]
			if ok && current.Pending != nil && current.Pending.ID == eventID {
				if err == nil {
					current.Delivered = current.Active
					current.LastSent = now
					current.Pending = nil
					s.lastError = ""
				} else {
					current.Pending.Attempts = min(20, current.Pending.Attempts+1)
					delay := min(300, 30<<(min(current.Pending.Attempts-1, 4)))
					current.Pending.NextAttempt = now.Add(time.Duration(delay+int(eventID%11)) * time.Second)
					s.lastError = "delivery_failed"
				}
				next.Incidents[batchKey] = current
			}
		}
		if commitErr := s.commit(next); commitErr != nil {
			s.mu.Unlock()
			return commitErr
		}
		s.mu.Unlock()
	}
	return nil
}

// Report only transitions, with fixed codes and no target or credential data.
// A persistent failure must remain observable without flooding the service log.
func (s *CheckAlerts) reportErrorChange() {
	s.mu.Lock()
	code, previous := s.lastError, s.loggedError
	if code == previous {
		s.mu.Unlock()
		return
	}
	s.loggedError = code
	s.mu.Unlock()
	if code != "" {
		s.logger.Warn("Service notification worker failed", "code", code)
	} else {
		s.logger.Info("Service notification worker recovered", "previousCode", previous)
	}
}

func freshForCheckDelivery(key string, item checkIncident, host cluster.Host, now time.Time) bool {
	if item.Revision == "sampler" {
		return true
	}
	if host.LastSnapshot == nil || host.LastSnapshot.Telemetry.ServiceChecks == nil {
		return false
	}
	summary := host.LastSnapshot.Telemetry.ServiceChecks
	for _, sample := range summary.Items {
		if key == host.ID+":"+sample.ID && item.Revision == sample.Revision && summary.Available && sample.State != "unknown" && !sample.CheckedAt.IsZero() && now.Sub(sample.CheckedAt) <= time.Duration(2*summary.IntervalSeconds)*time.Second+time.Minute {
			return true
		}
	}
	return false
}
func serviceCheckMessage(item checkIncident, delivery checkDelivery, now time.Time, locale string) string {
	labels := map[string]string{"down": "服务异常", "recovery": "服务恢复", "resolved": "服务曾异常，现已恢复", "repeat": "服务持续异常", "unknown": "服务检测数据中断", "resumed": "服务检测数据恢复"}
	if locale == "en-US" {
		labels = map[string]string{"down": "Service unavailable", "recovery": "Service recovered", "resolved": "Service incident resolved before delivery", "repeat": "Service still unavailable", "unknown": "Service check data unavailable", "resumed": "Service check data resumed"}
	}
	hostLabel, checkLabel, targetLabel := "观测主机", "检测", "目标"
	if locale == "en-US" {
		hostLabel, checkLabel, targetLabel = "Observer host", "Check", "Target"
	}
	if locale == "zh-TW" {
		labels = map[string]string{"down": "服務異常", "recovery": "服務恢復", "resolved": "服務曾異常，現已恢復", "repeat": "服務持續異常", "unknown": "服務檢測資料中斷", "resumed": "服務檢測資料恢復"}
		hostLabel, checkLabel, targetLabel = "觀測主機", "檢測", "目標"
	}
	return fmt.Sprintf("[KPanel] %s\n%s: %s\n%s: %s\n%s: %s\n%s\nEvent: %d", labels[delivery.Kind], hostLabel, item.HostName, checkLabel, item.Name, targetLabel, item.Target, now.Format(time.RFC3339), delivery.ID)
}
