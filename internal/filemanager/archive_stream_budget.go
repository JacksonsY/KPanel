package filemanager

import (
	"context"
	"io"
	"math"
)

// Logical TAR entries omit extension headers consumed inside Next. Charge the
// complete expanded stream too, sharing the budget across a batch. The generous
// per-entry allowance covers path/PAX metadata without allowing it to grow
// independently of the declared object and payload budgets.
type archiveStreamReader struct {
	ctx    context.Context
	reader io.Reader
	budget *copyBudget
}

func (r *archiveStreamReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	const entryOverhead = int64(16 << 10)
	const trailerAllowance = int64(1 << 20)
	if r.budget.maxEntries < 0 || int64(r.budget.maxEntries) > (math.MaxInt64-trailerAllowance)/entryOverhead {
		return 0, ErrTooLarge
	}
	overhead := int64(r.budget.maxEntries)*entryOverhead + trailerAllowance
	if r.budget.maxBytes < 0 || r.budget.maxBytes > math.MaxInt64-overhead {
		return 0, ErrTooLarge
	}
	limit := r.budget.maxBytes + overhead
	remaining := limit - r.budget.archiveBytes
	if remaining < 0 {
		return 0, ErrTooLarge
	}
	if int64(len(p)) > remaining {
		p = p[:int(remaining)+1]
	}
	n, err := r.reader.Read(p)
	if int64(n) > remaining {
		r.budget.archiveBytes = limit
		return 0, ErrTooLarge
	}
	r.budget.archiveBytes += int64(n)
	return n, err
}
