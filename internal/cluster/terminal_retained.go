package cluster

import (
	"context"
	"errors"
	"time"

	"github.com/kejilion/kejilion-panel/internal/terminal"
)

var ErrTerminalRetentionUnavailable = errors.New("retained terminals are not supported")

// TerminalCapabilities are queried at the actual PTY owner, never from telemetry.
// Implementations may return both flags in one authenticated local request.
type TerminalCapabilities struct {
	Protocol string `json:"protocol"`
	Retained bool   `json:"retained,omitempty"`
}

type TerminalCapabilityBackend interface {
	TerminalCapabilities(context.Context) (TerminalCapabilities, error)
}

func terminalBackendCapabilities(ctx context.Context, b TerminalBackend) (TerminalCapabilities, error) {
	if combined, ok := b.(TerminalCapabilityBackend); ok {
		return combined.TerminalCapabilities(ctx)
	}
	var result TerminalCapabilities
	if reliable, ok := b.(SequencedTerminalBackend); ok && reliable.SupportsSequencedInput(ctx) {
		result.Protocol = terminal.InputProtocol
	}
	var err error
	result.Retained, err = supportsRetainedTerminals(ctx, b)
	return result, err
}

// RetainedTerminalBackend is optional: old nodes must reject rather than silently downgrade.
type RetainedTerminalBackend interface {
	OpenRetained(context.Context, string, uint16, uint16) (terminal.Snapshot, error)
}

// RetainedTerminalCapabilityBackend probes the actual PTY owner without creating a session.
type RetainedTerminalCapabilityBackend interface {
	SupportsRetainedTerminals(context.Context) (bool, error)
}

func supportsRetainedTerminals(ctx context.Context, b TerminalBackend) (bool, error) {
	if capability, ok := b.(RetainedTerminalCapabilityBackend); ok {
		return capability.SupportsRetainedTerminals(ctx)
	}
	_, ok := b.(RetainedTerminalBackend)
	return ok, nil
}

func openTerminalBackend(ctx context.Context, b TerminalBackend, owner string, rows, cols uint16, retained bool) (terminal.Snapshot, error) {
	if !retained {
		return b.Open(ctx, owner, rows, cols)
	}
	r, ok := b.(RetainedTerminalBackend)
	if !ok {
		return terminal.Snapshot{}, ErrTerminalRetentionUnavailable
	}
	snapshot, err := r.OpenRetained(ctx, owner, rows, cols)
	if err == nil && !snapshot.Retained {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = b.Close(cleanup, owner, snapshot.ID)
		return terminal.Snapshot{}, ErrTerminalRetentionUnavailable
	}
	return snapshot, err
}
