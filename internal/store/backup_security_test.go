package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestIdentityRestoreDoesNotImportCredentialCounter(t *testing.T) {
	for _, version := range []uint64{0, ^uint64(0) - 1, ^uint64(0)} {
		t.Run(stringVersion(version), func(t *testing.T) {
			s, u, _, _ := passkeyFixture(t, false)
			data, err := s.ExportIdentity()
			if err != nil {
				t.Fatal(err)
			}
			var state diskState
			if err := json.Unmarshal(data, &state); err != nil {
				t.Fatal(err)
			}
			state.Users[0].CredentialVersion = version
			state.Users[0].CredentialEpoch = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			data, _ = json.Marshal(state)
			if err := s.RestoreIdentity(data); err != nil {
				t.Fatalf("restore rejected valid historical counter: %v", err)
			}
			got, err := s.UserByUsername(u.Username)
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != u.ID || got.CredentialVersion != 1 || got.CredentialEpoch == state.Users[0].CredentialEpoch {
				t.Fatalf("imported untrusted counter %d as local %d", version, got.CredentialVersion)
			}
			if err := s.ReplaceUserPassword(got.ID, got.PasswordHash, "new-hash", time.Now()); err != nil {
				t.Fatalf("restore stranded password management: %v", err)
			}
			exported, err := s.ExportIdentity()
			if err != nil || bytes.Contains(exported, []byte("credentialEpoch")) {
				t.Fatalf("identity backup exported a local epoch: %v", err)
			}
		})
	}
}

func TestIdentityRestoreEpochRejectsEqualVersionSnapshots(t *testing.T) {
	s, _, session, key := passkeyFixture(t, true)
	s.data.Users[0].CredentialVersion = 2
	stale := cloneUser(s.data.Users[0])
	backup, err := s.ExportIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreIdentity(backup); err != nil {
		t.Fatal(err)
	}
	current, _ := s.UserByID(stale.ID)
	// Re-enroll the same authenticator, bringing the local counter back to 2.
	if err := s.PutSession(session); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceUserPasskeys(current, []Passkey{key}, session.TokenHash, time.Now()); err != nil {
		t.Fatal(err)
	}
	current, _ = s.UserByID(stale.ID)
	if current.CredentialVersion != stale.CredentialVersion || current.CredentialEpoch == stale.CredentialEpoch {
		t.Fatal("fixture did not collide counters across separate epochs")
	}
	updated, login := passkeyLogin(key, session)
	if err := s.UpdateUserPasskey(stale, updated, login); !errors.Is(err, ErrConflict) {
		t.Fatalf("old passkey snapshot survived equal counter: %v", err)
	}
	if err := s.PutLoginSession(stale, login, nil, time.Now().Add(-time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatalf("old password login survived equal counter: %v", err)
	}
	if err := s.EnableUserTOTP(stale.ID, stale.CredentialEpoch, stale.CredentialVersion, "old", time.Now(), 1, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("old enrollment survived equal counter: %v", err)
	}
	if err := s.PutSession(session); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceUserPasskeys(stale, nil, session.TokenHash, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("old management snapshot survived equal counter: %v", err)
	}
	if err := s.DisablePasskeys(stale, PasskeyOriginResourceVersion(""), session.TokenHash, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("old RP management survived equal counter: %v", err)
	}
}

func TestCredentialEpochRolloverAndRestoreRollback(t *testing.T) {
	for _, operation := range []string{"password rollover", "restore"} {
		t.Run(operation, func(t *testing.T) {
			s, u, _, _ := passkeyFixture(t, true)
			s.data.Users[0].CredentialVersion = ^uint64(0)
			if err := s.persistLocked(); err != nil {
				t.Fatal(err)
			}
			backup, _ := s.ExportIdentity()
			before := cloneDiskState(s.data)
			path := s.path
			diskBefore, _ := os.ReadFile(path)
			s.path = filepath.Join(t.TempDir(), "missing", "state.json")
			var err error
			if operation == "restore" {
				err = s.RestoreIdentity(backup)
			} else {
				err = s.ReplaceUserPassword(u.ID, u.PasswordHash, "new", time.Now())
			}
			s.path = path
			diskAfter, readErr := os.ReadFile(path)
			if err == nil || readErr != nil || !reflect.DeepEqual(before, cloneDiskState(s.data)) || !bytes.Equal(diskBefore, diskAfter) {
				t.Fatalf("failed epoch transition changed state: write=%v read=%v", err, readErr)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if err := reopened.RestoreIdentity(backup); err != nil {
				t.Fatalf("persisted legacy maximum cannot recover: %v", err)
			}
			got, _ := reopened.UserByID(u.ID)
			if got.CredentialVersion != 1 || got.CredentialEpoch == "" {
				t.Fatal("restored maximum lacks a fresh local epoch")
			}
		})
	}
}

func stringVersion(value uint64) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func TestIdentityRestoreRecoversExhaustedLocalCounterWithoutReusingOldGeneration(t *testing.T) {
	s, u, session, key := passkeyFixture(t, true)
	data, err := s.ExportIdentity()
	if err != nil {
		t.Fatal(err)
	}
	s.data.Users[0].CredentialVersion = ^uint64(0)
	if err := s.persistLocked(); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreIdentity(data); err != nil {
		t.Fatalf("exhausted local identity cannot recover: %v", err)
	}
	first, _ := s.UserByUsername(u.Username)
	if first.ID != u.ID || first.CredentialEpoch == u.CredentialEpoch || first.CredentialEpoch == "" {
		t.Fatal("restored identity reused an old credential generation")
	}
	if _, err := s.SessionByTokenHash(session.TokenHash, time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old session survived: %v", err)
	}
	if len(first.Passkeys) != 0 {
		t.Fatal("restored passkeys")
	}
	updated, login := passkeyLogin(key, session)
	if err := s.UpdateUserPasskey(u, updated, login); err == nil {
		t.Fatal("old passkey snapshot created a session")
	}
	if err := s.EnableUserTOTP(u.ID, u.CredentialEpoch, u.CredentialVersion, "stale-secret", time.Now(), 1, nil); err == nil {
		t.Fatal("old enrollment survived restore")
	}
	if err := s.ReplaceUserPassword(first.ID, first.PasswordHash, first.PasswordHash, time.Now()); err != nil {
		t.Fatal(err)
	}
	// Reimport the original old ID/version: the generation cannot be resurrected.
	if err := s.RestoreIdentity(data); err != nil {
		t.Fatal(err)
	}
	second, _ := s.UserByUsername(u.Username)
	if second.ID != u.ID || second.CredentialEpoch == first.CredentialEpoch || second.CredentialEpoch == u.CredentialEpoch {
		t.Fatal("repeated restore reused an old credential generation")
	}
	if err := s.EnableUserTOTP(u.ID, u.CredentialEpoch, u.CredentialVersion, "stale-secret", time.Now(), 1, nil); err == nil {
		t.Fatal("repeated restore resurrected an enrollment")
	}
	if err := s.RecoverUserPassword(PasswordRecovery{UserID: second.ID, ExpectedHash: second.PasswordHash, NewHash: "recovered", UpdatedAt: time.Now(), AuditEvent: AuditEvent{ID: "counter-recover", Action: "auth.password.recover"}}); err != nil {
		t.Fatalf("local recovery after restore failed: %v", err)
	}
}
