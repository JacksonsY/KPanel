package ai

import "time"

const (
	maxDraftCheckpoints     = 8
	maxDraftPayloadBytes    = 2 << 20
	draftCheckpointInterval = 2 * time.Second
)

// A model step owns one admission budget across every transport retry. Count
// and cumulative full-prefix bytes are independent of provider chunk size or
// stream duration. The final flush has its own single MaxAssistantBytes slot.
type draftCheckpoints struct {
	count, bytes int
	last         time.Time
}

func (d *draftCheckpoints) admit(size int, now time.Time) bool {
	if size <= 0 || size > MaxAssistantBytes || d.count >= maxDraftCheckpoints || size > maxDraftPayloadBytes-d.bytes {
		return false
	}
	if now.Sub(d.last) < draftCheckpointInterval && (d.count > 0 || size < 1024) {
		return false
	}
	d.count++
	d.bytes += size
	d.last = now
	return true
}
