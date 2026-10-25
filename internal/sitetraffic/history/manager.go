package history

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kejilion/kejilion-panel/internal/sitetraffic"
)

type Source func(context.Context, sitetraffic.CollectInput) (sitetraffic.Batch, error)
type Manager struct {
	dir     string
	source  Source
	mu      sync.Mutex
	store   *Store
	closed  bool
	once    sync.Once
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	wake    chan struct{}
	failure bool
}

func New(dir string, source Source) *Manager {
	return &Manager{dir: dir, source: source, wake: make(chan struct{}, 1)}
}
func (m *Manager) storage() (*Store, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrUnavailable
	}
	if m.store != nil {
		return m.store, nil
	}
	value, err := Open(m.dir)
	if err != nil {
		return nil, err
	}
	m.store = value
	return value, nil
}
func (m *Manager) Start(parent context.Context) {
	m.once.Do(func() {
		ctx, cancel := context.WithCancel(parent)
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			cancel()
			return
		}
		m.cancel = cancel
		m.wg.Add(1)
		m.mu.Unlock()
		go func() {
			defer m.wg.Done()
			timer := time.NewTimer(10 * time.Second)
			defer timer.Stop()
			for {
				delay := 10 * time.Second
				if m.collect(ctx) {
					delay = time.Second
				}
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(delay)
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				case <-m.wake:
				}
			}
		}()
	})
}
func (m *Manager) collect(ctx context.Context) bool {
	// An unused dashboard does not create a database or consume Agent resources.
	m.mu.Lock()
	exists := m.store != nil
	m.mu.Unlock()
	if !exists {
		if _, err := os.Lstat(filepath.Join(m.dir, "analytics.sqlite")); err != nil {
			return false
		}
	}
	store, err := m.storage()
	if err != nil {
		return false
	}
	work, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	input, enabled, err := store.State(work)
	if err != nil || !enabled {
		return false
	}
	batch, err := m.source(work, input)
	if err == nil {
		err = store.Apply(work, batch)
	}
	m.mu.Lock()
	m.failure = err != nil && !errors.Is(err, ErrStale)
	m.mu.Unlock()
	return err == nil && batch.HasMore
}
func (m *Manager) Query(ctx context.Context, period, host string) (sitetraffic.Snapshot, error) {
	store, err := m.storage()
	if err != nil {
		return sitetraffic.Snapshot{}, err
	}
	out, err := store.Query(ctx, period, host)
	m.mu.Lock()
	failed := m.failure
	m.mu.Unlock()
	if err == nil && out.Enabled && failed {
		out.State = "source_unavailable"
		out.Warnings = append(out.Warnings, "source_unavailable")
	}
	return out, err
}
func (m *Manager) Action(ctx context.Context, action string) error {
	store, err := m.storage()
	if err != nil {
		return err
	}
	if err = store.Action(ctx, action); err != nil {
		return err
	}
	select {
	case m.wake <- struct{}{}:
	default:
	}
	return nil
}
func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	if m.cancel != nil {
		m.cancel()
	}
	store := m.store
	m.mu.Unlock()
	m.wg.Wait()
	if store != nil {
		return store.Close()
	}
	return nil
}
