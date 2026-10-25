package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/kejilion/kejilion-panel/internal/redact"
)

const (
	terminalDisplayVersion   = "model-v1"
	maxTerminalDisplaySource = 8 << 20
	maxTerminalDisplayReads  = 257
	maxTerminalDisplayPage   = 12 << 10
)

// A display request can inspect the retained context but cannot allocate an
// unbounded number of projections or start another PTY/process/owner task.
var terminalDisplaySlots = make(chan struct{}, 2)

type terminalSourceChunk struct {
	DataBase64 string `json:"dataBase64"`
	NextOffset int64  `json:"nextOffset"`
	InputOpen  bool   `json:"inputOpen"`
	Finished   bool   `json:"finished"`
	Truncated  bool   `json:"truncated"`
}

type terminalDisplay struct {
	Projection string `json:"projection"`
	Text       string `json:"text"`
	NextOffset int64  `json:"nextOffset"`
	InputOpen  bool   `json:"inputOpen"`
	Finished   bool   `json:"finished"`
	Truncated  bool   `json:"truncated"`
}

// projectTerminalRead uses the owner's existing bounded read contract for all
// task types. The caller's cursor is applied only to the safe display snapshot,
// never to the raw source. Missing or rotated context fails closed.
func projectTerminalRead[T any](ctx context.Context, offset int64, read func(int64) (T, error)) (terminalDisplay, error) {
	select {
	case terminalDisplaySlots <- struct{}{}:
		defer func() { <-terminalDisplaySlots }()
	default:
		return terminalDisplay{}, errors.New("terminal_projection_busy")
	}
	return buildTerminalDisplay(ctx, offset, terminalReadAdapter(read))
}

func terminalReadAdapter[T any](read func(int64) (T, error)) func(int64) (terminalSourceChunk, error) {
	return func(rawOffset int64) (terminalSourceChunk, error) {
		value, err := read(rawOffset)
		if err != nil {
			return terminalSourceChunk{}, err
		}
		body, err := json.Marshal(value)
		if err != nil || len(body) > 128<<10 {
			return terminalSourceChunk{}, errors.New("terminal_projection_invalid")
		}
		var chunk terminalSourceChunk
		if json.Unmarshal(body, &chunk) != nil {
			return chunk, errors.New("terminal_projection_invalid")
		}
		return chunk, nil
	}
}

func buildTerminalDisplay(ctx context.Context, offset int64, read func(int64) (terminalSourceChunk, error)) (terminalDisplay, error) {
	if offset < 0 {
		return terminalDisplay{}, errors.New("terminal_projection_invalid")
	}
	safe, last, err := readTerminalSnapshot(ctx, read, &terminalDisplayBudget{})
	if err != nil {
		return terminalDisplay{}, err
	}
	return terminalDisplayPage(safe, last, offset), nil
}

type terminalDisplayBudget struct{ bytes, reads int }

func readTerminalSnapshot(ctx context.Context, read func(int64) (terminalSourceChunk, error), budget *terminalDisplayBudget) (string, terminalSourceChunk, error) {
	var source []byte
	var last terminalSourceChunk
	complete := false
	for budget.reads < maxTerminalDisplayReads {
		if err := ctx.Err(); err != nil {
			return "", last, err
		}
		budget.reads++
		chunk, err := read(int64(len(source)))
		if err != nil {
			return "", last, err
		}
		// Decode only within the total source budget, including a malformed
		// oversized owner response. Individual owner chunks are <= 64 KiB.
		if len(chunk.DataBase64) > base64.StdEncoding.EncodedLen(64<<10) {
			return "", last, errors.New("terminal_projection_limit")
		}
		data, err := base64.StdEncoding.DecodeString(chunk.DataBase64)
		if err != nil || chunk.Truncated || len(data) > maxTerminalDisplaySource-budget.bytes || chunk.NextOffset != int64(len(source)+len(data)) {
			return "", last, errors.New("terminal_projection_context_unavailable")
		}
		budget.bytes += len(data)
		source = append(source, data...)
		last = chunk
		if len(data) == 0 || chunk.Finished {
			complete = true
			break
		}
	}
	if !complete {
		return "", last, errors.New("terminal_projection_limit")
	}
	if err := ctx.Err(); err != nil {
		return "", last, err
	}
	safe := redact.Text(string(source))
	if len(safe) > 16<<20 {
		return "", last, errors.New("terminal_projection_limit")
	}
	if err := ctx.Err(); err != nil {
		return "", last, err
	}
	return safe, last, nil
}

func terminalDisplayPage(safe string, last terminalSourceChunk, offset int64) terminalDisplay {
	start := min(offset, int64(len(safe)))
	for start < int64(len(safe)) && !utf8.RuneStart(safe[start]) {
		start++
	}
	end := min(start+maxTerminalDisplayPage, int64(len(safe)))
	for end > start && end < int64(len(safe)) && !utf8.RuneStart(safe[end]) {
		end--
	}
	return terminalDisplay{Projection: terminalDisplayVersion, Text: safe[start:end], NextOffset: end, InputOpen: last.InputOpen, Finished: last.Finished && end == int64(len(safe)), Truncated: end < int64(len(safe))}
}

func taskDisplayQuery(r *http.Request) bool {
	return r.URL.RawQuery == "projection=model-v1"
}

func writeTaskDisplay[T any](w http.ResponseWriter, r *http.Request, value any, read func(string, int64) (T, error)) {
	select {
	case terminalDisplaySlots <- struct{}{}:
		defer func() { <-terminalDisplaySlots }()
	default:
		writeProblem(w, requestIDFrom(w), 503, "terminal_projection_unavailable", "Task output projection unavailable", "")
		return
	}
	// Callers pass metadata copies with raw Logs cleared before marshalling.
	body, err := json.Marshal(value)
	var result map[string]any
	if err != nil || len(body) > 8<<20 || json.Unmarshal(body, &result) != nil {
		writeProblem(w, requestIDFrom(w), 503, "terminal_projection_unavailable", "Task output projection unavailable", "")
		return
	}
	budget := &terminalDisplayBudget{}
	project := func(item map[string]any) {
		id, _ := item["id"].(string)
		safe, _, err := readTerminalSnapshot(r.Context(), terminalReadAdapter(func(offset int64) (T, error) { return read(id, offset) }), budget)
		item["projection"] = terminalDisplayVersion
		if err != nil {
			item["logs"] = []string{"[REDACTED: task log context unavailable]"}
			item["logsContextUnavailable"] = true
			return
		}
		lines := strings.Split(strings.TrimSuffix(safe, "\n"), "\n")
		if len(lines) > 120 {
			lines = lines[len(lines)-120:]
		}
		tail := strings.Join(lines, "\n")
		if len(tail) > maxTerminalDisplayPage {
			start := len(tail) - maxTerminalDisplayPage
			for start < len(tail) && !utf8.RuneStart(tail[start]) {
				start++
			}
			tail = tail[start:]
		}
		item["logs"] = strings.Split(tail, "\n")
	}
	if listed, exists := result["items"]; exists {
		items, _ := listed.([]any)
		for _, item := range items {
			if object, ok := item.(map[string]any); ok {
				project(object)
			}
		}
	} else {
		project(result)
	}
	if r.Context().Err() != nil {
		return
	}
	result["projection"] = terminalDisplayVersion
	writeJSON(w, 200, result)
}

func writeTerminalDisplay[T any](w http.ResponseWriter, r *http.Request, offset int64, read func(int64) (T, error)) {
	display, err := projectTerminalRead(r.Context(), offset, read)
	if err != nil {
		// Never include owner error text or raw output in a projection failure.
		if !errors.Is(err, context.Canceled) {
			writeProblem(w, requestIDFrom(w), http.StatusServiceUnavailable, "terminal_projection_unavailable", "Task output projection unavailable", "")
		}
		return
	}
	writeJSON(w, http.StatusOK, display)
}
