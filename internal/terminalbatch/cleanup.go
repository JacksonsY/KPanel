package terminalbatch

import (
	"context"
	"time"
)

// Cleanup belongs to the manager, so it continues without an open browser.
// Each round is bounded and retries only Close, never the user's command.
func (m *Manager) startCleanupLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	m.stopCleanup = cancel
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(m.options.CleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.mu.Lock()
				for id := range m.cleanup {
					m.startCleanupLocked(id)
				}
				m.mu.Unlock()
			}
		}
	}()
}

func (m *Manager) startCleanupLocked(id string) {
	if m.closed || m.cleaning[id] != nil || len(m.cleanup[id]) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), m.options.CleanupTimeout)
	m.cleaning[id] = cancel
	m.wg.Add(1)
	go m.retryCleanup(ctx, id)
}
