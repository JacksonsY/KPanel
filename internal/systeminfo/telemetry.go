package systeminfo

import (
	"context"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

// Leave room for Panel's local summary budget (4s), including optional checks.
const telemetryCollectionTimeout = 2 * time.Second

type telemetryCollection struct {
	done  chan struct{}
	ctx   context.Context
	value contract.SystemSummary
	err   error
}

// CollectTelemetry excludes management configuration from the monitoring path.
// One worker per Collector bounds even an uninterruptible filesystem read: its
// slot is held until it actually exits, while callers stop at the deadline.
// Timed-out work never becomes a fresh heartbeat or starts replacement workers.
func (c *Collector) CollectTelemetry(ctx context.Context) (contract.SystemSummary, error) {
	c.prepareDefaults()
	return c.collectTelemetry(ctx, telemetryCollectionTimeout, func(ctx context.Context) (contract.SystemSummary, error) {
		value, err := c.collectRuntime(ctx)
		if c.PublicNetworkLookupEnabled {
			value.PublicNetwork = c.readPublicNetwork(ctx)
		}
		return value, err
	})
}

func (c *Collector) collectTelemetry(ctx context.Context, budget time.Duration, read func(context.Context) (contract.SystemSummary, error)) (contract.SystemSummary, error) {
	if err := ctx.Err(); err != nil {
		return contract.SystemSummary{}, err
	}
	c.telemetryMu.Lock()
	pending := c.telemetryPending
	if pending == nil {
		collectionCtx, cancel := context.WithTimeout(context.Background(), budget)
		pending = &telemetryCollection{done: make(chan struct{}), ctx: collectionCtx}
		c.telemetryPending = pending
		go func() {
			defer cancel()
			pending.value, pending.err = read(collectionCtx)
			if err := collectionCtx.Err(); err != nil {
				pending.value, pending.err = contract.SystemSummary{}, err
			}
			c.telemetryMu.Lock()
			c.telemetryPending = nil
			close(pending.done)
			c.telemetryMu.Unlock()
		}()
	}
	c.telemetryMu.Unlock()
	// Prefer the completed result over the worker's deferred cancellation.
	result := func() (contract.SystemSummary, error) {
		value := pending.value
		value.OSLike = append([]string(nil), value.OSLike...)
		value.Disks = append([]contract.DiskSummary(nil), value.Disks...)
		if value.PublicNetwork.UpdatedAt != nil {
			updated := *value.PublicNetwork.UpdatedAt
			value.PublicNetwork.UpdatedAt = &updated
		}
		return value, pending.err
	}
	select {
	case <-pending.done:
		return result()
	case <-ctx.Done():
		return contract.SystemSummary{}, ctx.Err()
	case <-pending.ctx.Done():
		select {
		case <-pending.done:
			return result()
		default:
			return contract.SystemSummary{}, pending.ctx.Err()
		}
	}
}
