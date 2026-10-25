package cluster

import (
	"context"
	"encoding/json"
	"slices"
)

const maxRemoteHostPreferences = 4096

type remoteHostPreference struct {
	HostID          string `json:"hostId"`
	TerminalEnabled *bool  `json:"terminalEnabled"`
	FilesEnabled    *bool  `json:"filesEnabled"`
}

type remoteHostEpochs struct {
	terminal     context.Context
	stopTerminal context.CancelFunc
	files        context.Context
	stopFiles    context.CancelFunc
}

type HostRemoteAccess struct {
	RemoteAccess
	HostID                   string `json:"hostId"`
	GlobalTerminalEnabled    bool   `json:"globalTerminalEnabled"`
	GlobalFilesEnabled       bool   `json:"globalFilesEnabled"`
	EffectiveTerminalEnabled bool   `json:"effectiveTerminalEnabled"`
	EffectiveFilesEnabled    bool   `json:"effectiveFilesEnabled"`
}

// File transfer descriptors use the peer's node identity, while center settings
// use the local pairing record ID. Reverse-only peers have no local Host record
// and remain governed by the global switch plus their existing route grant.
func (s *Service) FileSourceHostID(nodeID string) (string, error) {
	if !validID(nodeID) {
		return "", ErrNotFound
	}
	id := ""
	if s.storeV2 != nil {
		for _, record := range s.storeV2.Hosts() {
			if record.RemoteNodeID == nodeID {
				if id != "" {
					return "", ErrNotFound
				}
				id = record.ID
			}
		}
	}
	if s.light != nil {
		if _, err := s.light.Host(nodeID); err == nil {
			if id != "" {
				return "", ErrNotFound
			}
			id = nodeID
		}
	}
	if id == "" {
		id = nodeID
	}
	return id, nil
}

func validRemoteHostPreferences(items []remoteHostPreference) bool {
	if len(items) > maxRemoteHostPreferences {
		return false
	}
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if !validID(item.HostID) || seen[item.HostID] || item.TerminalEnabled == nil || item.FilesEnabled == nil {
			return false
		}
		seen[item.HostID] = true
	}
	return true
}

func (c *remoteAccessControl) hostPreferences() []remoteHostPreference {
	items := make([]remoteHostPreference, 0, len(c.hosts))
	for _, item := range c.hosts {
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b remoteHostPreference) int {
		if a.HostID < b.HostID {
			return -1
		}
		if a.HostID > b.HostID {
			return 1
		}
		return 0
	})
	return items
}

func (c *remoteAccessControl) hostValue(id string) HostRemoteAccess {
	value := HostRemoteAccess{RemoteAccess: c.value, HostID: id, GlobalTerminalEnabled: c.value.TerminalEnabled, GlobalFilesEnabled: c.value.FilesEnabled}
	value.TerminalEnabled, value.FilesEnabled = true, true
	if preference, ok := c.hosts[id]; ok {
		value.TerminalEnabled, value.FilesEnabled = *preference.TerminalEnabled, *preference.FilesEnabled
	}
	value.EffectiveTerminalEnabled = value.Available && value.GlobalTerminalEnabled && value.TerminalEnabled
	value.EffectiveFilesEnabled = value.Available && value.GlobalFilesEnabled && value.FilesEnabled
	return value
}

func (s *Service) HostRemoteAccess(id string) HostRemoteAccess {
	if s.remoteAccess == nil {
		return HostRemoteAccess{RemoteAccess: s.RemoteAccess(), HostID: id, GlobalTerminalEnabled: true, GlobalFilesEnabled: true, EffectiveTerminalEnabled: true, EffectiveFilesEnabled: true}
	}
	c := s.remoteAccess
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hostValue(id)
}

func (c *remoteAccessControl) newHostEpoch(id, domain string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	value := c.hostValue(id)
	if domain == "terminal" && !value.EffectiveTerminalEnabled || domain == "files" && !value.EffectiveFilesEnabled {
		cancel()
	}
	return ctx, cancel
}

// All callers hold mu. Cache lifetimes so queued work keeps its original epoch;
// the bound also prevents arbitrary target IDs from growing memory indefinitely.
func (c *remoteAccessControl) hostEpoch(id, domain string) context.Context {
	epochs := c.hostEpochs[id]
	if epochs == nil {
		if len(id) > 128 || len(c.hostEpochs) >= maxRemoteHostPreferences {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}
		epochs = &remoteHostEpochs{}
		epochs.terminal, epochs.stopTerminal = c.newHostEpoch(id, "terminal")
		epochs.files, epochs.stopFiles = c.newHostEpoch(id, "files")
		c.hostEpochs[id] = epochs
	}
	if domain == "terminal" {
		return epochs.terminal
	}
	return epochs.files
}

func (s *Service) UpdateHostRemoteAccess(id, version string, terminalEnabled, filesEnabled bool) (HostRemoteAccess, error) {
	if !validID(id) {
		return HostRemoteAccess{}, ErrNotFound
	}
	c := s.remoteAccess
	if c == nil {
		return HostRemoteAccess{}, ErrRemoteAccessStorage
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	before := c.hostValue(id)
	if !c.value.Available {
		return before, ErrRemoteAccessStorage
	}
	if version == "" || version != c.value.ResourceVersion {
		return before, ErrConflict
	}
	if before.TerminalEnabled == terminalEnabled && before.FilesEnabled == filesEnabled {
		return before, nil
	}
	if _, ok := c.hosts[id]; !ok && len(c.hosts) >= maxRemoteHostPreferences {
		return before, ErrRemoteAccessStorage
	}
	revision, err := randomHex(16)
	if err != nil {
		return before, ErrRemoteAccessStorage
	}
	items := c.hostPreferences()
	items = slices.DeleteFunc(items, func(item remoteHostPreference) bool { return item.HostID == id })
	if !terminalEnabled || !filesEnabled {
		items = append(items, remoteHostPreference{HostID: id, TerminalEnabled: &terminalEnabled, FilesEnabled: &filesEnabled})
	}
	globalTerminal, globalFiles := c.value.TerminalEnabled, c.value.FilesEnabled
	content, err := json.Marshal(remoteAccessDisk{SchemaVersion: 1, TerminalEnabled: &globalTerminal, FilesEnabled: &globalFiles, Revision: revision, Hosts: items})
	if err != nil || atomicWriteFileV2(c.path, content, 0o600, true, c.ops) != nil {
		return before, ErrRemoteAccessStorage
	}
	delete(c.hosts, id)
	if !terminalEnabled || !filesEnabled {
		c.hosts[id] = remoteHostPreference{HostID: id, TerminalEnabled: &terminalEnabled, FilesEnabled: &filesEnabled}
	}
	c.value.ResourceVersion = revision
	if epochs := c.hostEpochs[id]; epochs != nil {
		if before.TerminalEnabled != terminalEnabled {
			epochs.stopTerminal()
			epochs.terminal, epochs.stopTerminal = c.newHostEpoch(id, "terminal")
		}
		if before.FilesEnabled != filesEnabled {
			epochs.stopFiles()
			epochs.files, epochs.stopFiles = c.newHostEpoch(id, "files")
		}
	}
	return c.hostValue(id), nil
}
