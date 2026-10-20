package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoginSessionCommitsBudgetsAndSessionTogether(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempts := []LoginAttempt{{Key: "ip:192.0.2.1", OccurredAt: now, Success: true}, {Key: "account:admin", OccurredAt: now, Success: true}}
	session := Session{TokenHash: "token-hash", CSRFHash: "csrf-hash", UserID: "admin", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := s.RecordLoginAttempt(LoginAttempt{Key: attempts[0].Key, OccurredAt: now.Add(-time.Second)}, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.PutLoginSession(session, attempts, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.SessionByTokenHash(session.TokenHash, now); err != nil {
		t.Fatal(err)
	}
	if count := s.FailedLoginCount(attempts[0].Key, now.Add(-time.Minute)); count != 0 {
		t.Fatalf("success did not reset budget: %d", count)
	}
	if len(s.data.LoginAttempts) != 3 {
		t.Fatalf("lost login history: %d", len(s.data.LoginAttempts))
	}
}

func TestLoginSessionRollsBackBothChangesOnPersistenceFailure(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	attempt := LoginAttempt{Key: "ip:192.0.2.1", OccurredAt: now.Add(-time.Second)}
	if err := s.RecordLoginAttempt(attempt, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	originalPath := s.path
	before, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	s.path = filepath.Join(t.TempDir(), "missing", "state.json")
	err = s.PutLoginSession(Session{TokenHash: "new-token", ExpiresAt: now.Add(time.Hour)}, []LoginAttempt{{Key: attempt.Key, OccurredAt: now, Success: true}}, now.Add(-time.Minute))
	s.path = originalPath
	if err == nil {
		t.Fatal("expected failed persistence")
	}
	if _, err := s.SessionByTokenHash("new-token", now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("failed login exposed a session: %v", err)
	}
	if s.FailedLoginCount(attempt.Key, now.Add(-time.Minute)) != 1 {
		t.Fatal("failed login cleared failure budget")
	}
	after, err := os.ReadFile(originalPath)
	if err != nil || string(before) != string(after) {
		t.Fatal("failed login changed durable state")
	}
}

func TestLoginSessionCannotBypassAttemptCapacity(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	s.data.LoginAttempts = make([]LoginAttempt, maxLoginAttempts)
	for i := range s.data.LoginAttempts {
		s.data.LoginAttempts[i] = LoginAttempt{Key: "account:other", OccurredAt: now}
	}
	err = s.PutLoginSession(Session{TokenHash: "new-token", ExpiresAt: now.Add(time.Hour)}, []LoginAttempt{{Key: "account:admin", OccurredAt: now, Success: true}}, now.Add(-time.Minute))
	if !errors.Is(err, ErrLimitReached) {
		t.Fatalf("expected capacity rejection: %v", err)
	}
	if len(s.data.LoginAttempts) != maxLoginAttempts || len(s.data.Sessions) != 0 {
		t.Fatal("capacity rejection mutated state")
	}
}
