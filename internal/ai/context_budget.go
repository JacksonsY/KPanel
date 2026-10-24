package ai

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

const (
	maxContextPrivateBytes       = 4 << 20
	maxContextTextBytes          = 2 << 20
	maxToolReadStateBytes        = maxProviderNativeBytes + MaxToolResultBytes + 1024
	maxToolArgumentsPreviewBytes = 4096 + len("\n[TRUNCATED]")
	maxToolResultPreviewBytes    = MaxToolResultBytes + len("\n[TRUNCATED]")
)

var ErrContextPrivateLimit = errors.New("AI 工具私有上下文超过一次处理预算；原记录已保留，请新建会话分批处理")

func contextMessageAfter(left, right Message) bool {
	return left.CreatedAt.After(right.CreatedAt) || (left.CreatedAt.Equal(right.CreatedAt) && left.ID > right.ID)
}

// Runs can interleave in (created_at,id) order within one millisecond. Expand
// a summary prefix to the last ordered message of every run it touches.
func expandContextBoundary(ctx context.Context, tx *sql.Tx, sessionID string, batch []Message, boundary Message) (Message, error) {
	seen := map[string]bool{}
	args, marks := []any{sessionID}, []string{}
	for _, message := range batch {
		if message.RunID != "" && !seen[message.RunID] {
			seen[message.RunID] = true
			args, marks = append(args, message.RunID), append(marks, "?")
		}
	}
	if len(marks) == 0 {
		return boundary, nil
	}
	var last Message
	var created int64
	if err := tx.QueryRowContext(ctx, `SELECT id,created_at FROM messages WHERE session_id=? AND run_id IN (`+strings.Join(marks, ",")+`) ORDER BY created_at DESC,id DESC LIMIT 1`, args...).Scan(&last.ID, &created); err != nil {
		return Message{}, err
	}
	last.CreatedAt = fromMillis(created)
	if contextMessageAfter(last, boundary) {
		boundary = last
	}
	return boundary, nil
}

func contextRunSpans(runIDs []string) []int {
	last := map[string]int{}
	for i, id := range runIDs {
		if id != "" {
			last[id] = i
		}
	}
	spans := []int{0}
	for start := 0; start < len(runIDs); {
		end := start + 1
		for i := start; i < end; i++ {
			if runIDs[i] != "" {
				end = max(end, last[runIDs[i]]+1)
			}
		}
		spans = append(spans, end)
		start = end
	}
	return spans
}

// Inspect lengths inside the same read transaction before copying message or
// ciphertext bodies through the SQLite driver. Compaction discards only whole
// old run groups; the current run is never silently stripped of required native
// context. Legacy oversized rows therefore fail closed if still required.
func planContextBudget(ctx context.Context, tx *sql.Tx, sessionID, cursor, currentRunID string) (int, map[string]int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT m.id,m.run_id,length(CAST(m.content AS BLOB)),
		COALESCE((SELECT length(t.private_state) FROM tool_calls t WHERE t.id=m.tool_call_id AND t.run_id=m.run_id AND t.session_id=m.session_id),0)
		FROM messages m WHERE m.session_id=? AND
		(?='' OR (m.created_at,m.id)>(SELECT created_at,id FROM messages WHERE id=? AND session_id=?))
		ORDER BY m.created_at,m.id LIMIT 200`, sessionID, cursor, cursor, sessionID)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	type metadata struct {
		id, runID     string
		text, private int
	}
	items := []metadata{}
	charges := map[string]int{}
	for rows.Next() {
		var item metadata
		if err := rows.Scan(&item.id, &item.runID, &item.text, &item.private); err != nil {
			return 0, nil, err
		}
		items = append(items, item)
		charges[item.id] = item.private
	}
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}
	runIDs := make([]string, len(items))
	for i, item := range items {
		runIDs[i] = item.runID
	}
	spans := contextRunSpans(runIDs)
	text, private, kept := 0, 0, 0
	for group := len(spans) - 1; group > 0; group-- {
		start, end := spans[group-1], spans[group]
		groupText, groupPrivate, oversize, current := 0, 0, false, false
		for _, item := range items[start:end] {
			groupText += item.text
			groupPrivate += item.private
			oversize = oversize || item.private > maxToolReadStateBytes
			current = current || (currentRunID != "" && item.runID == currentRunID)
		}
		if oversize || groupText > maxContextTextBytes-text || groupPrivate > maxContextPrivateBytes-private {
			if kept == 0 || current {
				return 0, nil, ErrContextPrivateLimit
			}
			return end, charges, nil
		}
		text += groupText
		private += groupPrivate
		kept += end - start
	}
	return 0, charges, nil
}
