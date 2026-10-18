package store

import (
	"encoding/json"
	"errors"
	"github.com/kejilion/kejilion-panel/internal/backup"
	"maps"
)

// ExportIdentity excludes sessions, passkeys, login attempts, audit history and
// live share authorizations. Restoring a backup must never reactivate a revoked
// credential. Passkeys must be enrolled again on the destination panel.
// The private cluster host order is panel-owned, non-sensitive preference state
// and is preserved independently from the public cluster share configuration.
func (s *Store) ExportIdentity() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]User, len(s.data.Users))
	copy(users, s.data.Users)
	for i := range users {
		users[i].TOTPRecoveryCodeHashes = nil
		users[i].Passkeys = nil
	}
	return json.Marshal(diskState{
		SchemaVersion:      1,
		Users:              users,
		ClusterHostOrder:   cloneClusterHostOrder(s.data.ClusterHostOrder),
		ClusterHostDetails: maps.Clone(s.data.ClusterHostDetails),
		ClusterTraffic:     maps.Clone(s.data.ClusterTraffic),
		Appearance:         cloneAppearance(s.data.Appearance),
		Preferences:        clonePreferences(s.data.Preferences),
	})
}

func ValidateIdentityBackup(data []byte) error {
	var state diskState
	if err := backup.Decode(data, &state); err != nil {
		return err
	}
	if state.SchemaVersion != 1 || len(state.Users) != 1 || len(state.Sessions) != 0 || len(state.Audit) != 0 || len(state.LoginAttempts) != 0 || len(state.FileShares) != 0 || state.ClusterShare.Enabled || state.ClusterShare.Token != "" || state.SecurityEntrance.Enabled || state.PasskeyOrigin != "" {
		return errors.New("invalid panel identity backup")
	}
	if state.ClusterHostOrder != nil {
		if err := ValidateClusterHostOrder(state.ClusterHostOrder.IDs); err != nil {
			return errors.New("invalid panel identity backup")
		}
	}
	if state.Appearance != nil && ValidateAppearance(*state.Appearance) != nil {
		return errors.New("invalid panel appearance backup")
	}
	if ValidatePreferences(state.Preferences) != nil {
		return errors.New("invalid panel preferences backup")
	}
	if validateClusterHostDetailsMap(state.ClusterHostDetails) != nil {
		return errors.New("invalid cluster host details backup")
	}
	if validateClusterTraffic(state.ClusterTraffic, state.ClusterHostDetails) != nil {
		return errors.New("invalid cluster traffic backup")
	}
	u := state.Users[0]
	if u.ID == "" || len(u.ID) > 128 || u.Role != "admin" || u.Username == "" || len(u.Username) > 128 || len(u.PasswordHash) > 1024 || len(u.PasswordHash) < 32 || len(u.TOTPRecoveryCodeHashes) != 0 || len(u.Passkeys) != 0 {
		return errors.New("invalid backup account")
	}
	if err := validateUserAvatar(u.Avatar); err != nil {
		return errors.New("invalid backup account")
	}
	return nil
}

// RestoreIdentity retains the destination entrance and audit history, while
// revoking all current login sessions and public shares.
func (s *Store) RestoreIdentity(data []byte) error {
	if err := ValidateIdentityBackup(data); err != nil {
		return err
	}
	var incoming diskState
	if err := json.Unmarshal(data, &incoming); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Restoring an older backup must not resurrect a credential snapshot.
	for i := range incoming.Users {
		for _, current := range s.data.Users {
			if current.CredentialVersion > incoming.Users[i].CredentialVersion {
				incoming.Users[i].CredentialVersion = current.CredentialVersion
			}
		}
		if err := advanceCredentialVersion(&incoming.Users[i]); err != nil {
			return err
		}
		incoming.Users[i].Passkeys = nil
	}
	previous := cloneDiskState(s.data)
	s.data.Users = incoming.Users
	s.data.Sessions = nil
	s.data.LoginAttempts = nil
	s.data.FileShares = nil
	s.data.ClusterShare = ClusterShare{}
	s.data.ClusterHostOrder = cloneClusterHostOrder(incoming.ClusterHostOrder)
	s.data.ClusterHostDetails = maps.Clone(incoming.ClusterHostDetails)
	s.data.ClusterTraffic = maps.Clone(incoming.ClusterTraffic)
	s.data.Appearance = cloneAppearance(incoming.Appearance)
	s.data.Preferences = clonePreferences(incoming.Preferences)
	if err := s.persistLocked(); err != nil {
		s.data = previous
		return err
	}
	return nil
}
