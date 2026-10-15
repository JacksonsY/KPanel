package cluster

import (
	"context"
	"github.com/kejilion/kejilion-panel/internal/terminal"
	"time"
)

// RetainedTerminalBackend is optional: old nodes must reject rather than silently downgrade.
type RetainedTerminalBackend interface {
	OpenRetained(context.Context, string, uint16, uint16) (terminal.Snapshot, error)
}

func openTerminalBackend(ctx context.Context, b TerminalBackend, owner string, rows, cols uint16, retained bool) (terminal.Snapshot, error) {
	if !retained {
		return b.Open(ctx, owner, rows, cols)
	}
	r, ok := b.(RetainedTerminalBackend)
	if !ok {
		return terminal.Snapshot{}, ErrTerminalUnavailable
	}
	snapshot, err := r.OpenRetained(ctx, owner, rows, cols)
	if err == nil && !snapshot.Retained {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = b.Close(cleanup, owner, snapshot.ID)
		return terminal.Snapshot{}, ErrTerminalUnavailable
	}
	return snapshot, err
}
