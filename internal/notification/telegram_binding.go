package notification

import (
	"context"
	"crypto/rand"
	"sync"
	"time"
)

type bindingSessionKey struct{}
type telegramProofKey struct{}

// WithBindingSession uses the authenticated session hash, never a browser
// supplied account identifier. Challenges cannot move between admin sessions.
func WithBindingSession(ctx context.Context, session string) context.Context {
	return context.WithValue(ctx, bindingSessionKey{}, session)
}

type TelegramBinding struct {
	Command   string    `json:"command"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type pendingTelegramBinding struct {
	TelegramBinding
	Session     string
	Fingerprint string
}

// Only one Telegram channel exists. A fresh credential save replaces this
// bounded in-memory challenge; process restart cancels it without undoing a
// previously verified persisted destination.
type telegramBindingState struct {
	mu      sync.Mutex
	pending *pendingTelegramBinding
}

func (s *Service) bindingContext(ctx context.Context, credential string, reset bool) context.Context {
	s.binding.mu.Lock()
	defer s.binding.mu.Unlock()
	session, _ := ctx.Value(bindingSessionKey{}).(string)
	fingerprint := tokenFingerprint(credential)
	pending := s.binding.pending
	if reset || pending == nil || !pending.ExpiresAt.After(s.now()) || pending.Fingerprint != fingerprint || pending.Session != session {
		pending = &pendingTelegramBinding{
			TelegramBinding: TelegramBinding{Command: "/start " + rand.Text(), ExpiresAt: s.now().Add(5 * time.Minute)},
			Session:         session, Fingerprint: fingerprint,
		}
		s.binding.pending = pending
	}
	return context.WithValue(ctx, telegramProofKey{}, pending.Command)
}

func (s *Service) consumeBinding(ctx context.Context) bool {
	s.binding.mu.Lock()
	defer s.binding.mu.Unlock()
	session, _ := ctx.Value(bindingSessionKey{}).(string)
	if s.binding.pending != nil && s.binding.pending.Command == ctx.Value(telegramProofKey{}) && s.binding.pending.Session == session && s.binding.pending.ExpiresAt.After(s.now()) {
		s.binding.pending = nil
		return true
	}
	return false
}

func (s *Service) cancelBinding() {
	s.binding.mu.Lock()
	defer s.binding.mu.Unlock()
	s.binding.pending = nil
}

func (s *Service) bindingSnapshot(ctx context.Context) *TelegramBinding {
	s.binding.mu.Lock()
	defer s.binding.mu.Unlock()
	session, _ := ctx.Value(bindingSessionKey{}).(string)
	if pending := s.binding.pending; pending != nil && pending.Session == session && pending.ExpiresAt.After(s.now()) {
		copy := pending.TelegramBinding
		return &copy
	}
	return nil
}

func (s *Service) SnapshotForSession(ctx context.Context) Snapshot { return s.snapshot(ctx) }
