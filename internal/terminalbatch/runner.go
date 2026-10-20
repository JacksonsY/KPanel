package terminalbatch

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/kejilion/kejilion-panel/internal/terminal"
)

var ansi = regexp.MustCompile(`\x1b(?:\][^\x07]*(?:\x07|\x1b\\)|\[[0-?]*[ -/]*[@-~]|[@-_][0-?]*[ -/]*[@-~])`)
var prompt = regexp.MustCompile(`(?:^|\r?\n|\r)[^\r\n]{0,64}@[^\r\n]{0,64}:[^\r\n]{0,80}[$#] ?$|(?:^|\r?\n|\r)(?:bash|sh)-[\d.]+\$ ?$|(?:^|\r?\n|\r)[$#] ?$`)

func (m *Manager) update(id string, index int, state, code, output string, truncated bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for n := range m.jobs {
		j := &m.jobs[n]
		if j.ID != id {
			continue
		}
		r := &j.Results[index]
		changed := r.State != state || r.Error != code
		r.State = state
		r.Error = code
		if output != "" {
			r.Output = tail(output, MaxOutputBytes)
		}
		r.Truncated = r.Truncated || truncated
		j.UpdatedAt = time.Now().UTC()
		if changed {
			_ = m.persistLocked()
		}
		return
	}
}

func (m *Manager) run(ctx context.Context, id, user, command string) {
	defer m.wg.Done()
	m.mu.Lock()
	var targets []Result
	for _, j := range m.jobs {
		if j.ID == id {
			targets = append([]Result(nil), j.Results...)
		}
	}
	m.mu.Unlock()
	queue := make(chan int, len(targets))
	for i := range targets {
		queue <- i
	}
	close(queue)
	var workers sync.WaitGroup
	for w := 0; w < 4 && w < len(targets); w++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range queue {
				m.execute(ctx, id, user, command, i, targets[i].Host)
			}
		}()
	}
	workers.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	for n := range m.jobs {
		j := &m.jobs[n]
		if j.ID == id {
			j.State = "completed"
			if ctx.Err() != nil {
				j.State = "stopped"
			}
			if m.closed {
				j.State = "interrupted"
			}
			j.UpdatedAt = time.Now().UTC()
		}
	}
	if cancel := m.active[id]; cancel != nil {
		cancel()
	}
	delete(m.active, id)
	_ = m.persistLocked()
}

func wait(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (m *Manager) stopped(id string, index int, ctx context.Context) {
	state, code := "stopped", ""
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		state, code = "timed_out", "batch_timeout"
	}
	m.mu.Lock()
	closing := m.closed
	m.mu.Unlock()
	if closing {
		state, code = "interrupted", "panel_stopped"
	}
	m.update(id, index, state, code, "", false)
}

func (m *Manager) execute(ctx context.Context, id, user, command string, index int, host Host) {
	if ctx.Err() != nil {
		m.stopped(id, index, ctx)
		return
	}
	// The backend reserves from the same pool used by all retained/window sessions.
	// A definitive limit rejection is safe to retry. Unknown open/input outcomes are not.
	var session Session
	m.update(id, index, "connecting", "", "", false)
	for {
		op, cancel := context.WithTimeout(ctx, 15*time.Second)
		opened, err := m.driver.Open(op, user, host)
		cancel()
		if err == nil {
			session = opened
			break
		}
		if ctx.Err() != nil {
			m.stopped(id, index, ctx)
			return
		}
		if !errors.Is(err, terminal.ErrLimit) {
			m.update(id, index, "failed", "terminal_open_failed", "", false)
			return
		}
		m.update(id, index, "queued", "waiting_for_capacity", "", false)
		if !wait(ctx, m.options.RetryDelay) {
			m.stopped(id, index, ctx)
			return
		}
	}
	state, code := "failed", "terminal_execution_failed"
	// A worker cannot release its slot until the target confirms closure. Failed
	// closure stays visible and registered, including when a batch is stopped.
	defer func() {
		m.update(id, index, "closing", "", "", false)
		attempts := 0
		for {
			closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := m.driver.Close(closeCtx, session)
			cancel()
			if err == nil || errors.Is(err, terminal.ErrNotFound) || errors.Is(err, terminal.ErrClosed) {
				break
			}
			attempts++
			m.update(id, index, "closing", "terminal_close_failed", "", false)
			// During shutdown/stop make bounded cleanup attempts. The Panel retains the
			// failed-close identity and quota, so a later batch still cannot over-admit.
			if ctx.Err() != nil && attempts >= 2 {
				m.rememberCleanup(id, index, session)
				state, code = "failed", "terminal_close_failed"
				break
			}
			if !wait(ctx, m.options.RetryDelay) && attempts >= 2 {
				m.rememberCleanup(id, index, session)
				state, code = "failed", "terminal_close_failed"
				break
			}
		}
		if ctx.Err() != nil && code != "terminal_close_failed" {
			m.stopped(id, index, ctx)
		} else {
			m.update(id, index, state, code, "", false)
		}
	}()
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		code = "terminal_marker_failed"
		return
	}
	marker := "__KPANEL_DONE_" + hex.EncodeToString(nonce) + "_"
	wrapped := wrap(command, marker, host.Shell)
	m.update(id, index, "running", "", "", false)
	for sent := 0; sent < len(wrapped); {
		end := min(sent+terminal.MaxInputBytes, len(wrapped))
		op, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := m.driver.Input(op, session, wrapped[sent:end])
		cancel()
		if err != nil {
			code = "terminal_input_uncertain"
			return
		}
		sent = end
	}
	offset := session.Offset
	raw := ""
	failures := 0
	promptAt := time.Time{}
	exits := 0
	exitPattern := regexp.MustCompile(regexp.QuoteMeta(marker) + `(-?\d{1,10})__`)
	for ctx.Err() == nil {
		op, cancel := context.WithTimeout(ctx, 5*time.Second)
		chunk, err := m.driver.Output(op, session, offset)
		cancel()
		if err != nil {
			failures++
			if failures > 5 || errors.Is(err, terminal.ErrNotFound) {
				code = "terminal_output_failed"
				return
			}
			if !wait(ctx, m.options.RetryDelay*time.Duration(1<<uint(failures-1))) {
				return
			}
			continue
		}
		failures = 0
		raw += string(chunk.Data)
		if match := exitPattern.FindStringSubmatch(raw); len(match) == 2 {
			exitCode, parseErr := strconv.ParseInt(match[1], 10, 32)
			if parseErr == nil && (host.Shell == "powershell" || exitCode >= 0 && exitCode <= 255) {
				state, code = "succeeded", ""
				if exitCode != 0 {
					state, code = "failed", fmt.Sprintf("exit status %d", exitCode)
				}
				m.update(id, index, "running", "", display(raw, command, marker), chunk.Truncated || len(raw) > MaxOutputBytes)
				return
			}
		}
		// PTY frames may split a UTF-8 character. Retain raw bytes across frames;
		// sanitize only the display copy so a later frame can complete the rune.
		if len(raw) > MaxOutputBytes*2 {
			raw = raw[len(raw)-MaxOutputBytes*2:]
		}
		output := display(raw, command, marker)
		m.update(id, index, "running", "", output, chunk.Truncated || len(raw) > MaxOutputBytes)
		if chunk.Closed || chunk.ExitedAt != nil {
			state, code = "succeeded", ""
			if chunk.ExitError != "" {
				state, code = "failed", "terminal_exit_failed"
			}
			return
		}
		if chunk.NextOffset > offset {
			promptAt = time.Time{}
		} else if promptAt.IsZero() && prompt.MatchString(output) {
			promptAt = time.Now()
		}
		offset = chunk.NextOffset
		if !promptAt.IsZero() && time.Since(promptAt) >= 3*time.Second {
			if exits >= 3 {
				code = "terminal_completion_unknown"
				return
			}
			op, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := m.driver.Input(op, session, "exit\r")
			cancel()
			if err != nil {
				code = "terminal_input_uncertain"
				return
			}
			exits++
			promptAt = time.Time{}
		}
		if len(chunk.Data) == 0 {
			if !wait(ctx, 300*time.Millisecond) {
				return
			}
		}
	}
}

func (m *Manager) rememberCleanup(id string, index int, session Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cleanup[id] == nil {
		m.cleanup[id] = map[int]Session{}
	}
	m.cleanup[id][index] = session
}

func (m *Manager) retryCleanup(ctx context.Context, id string) {
	defer m.wg.Done()
	m.mu.Lock()
	pending := make(map[int]Session, len(m.cleanup[id]))
	for index, session := range m.cleanup[id] {
		pending[index] = session
	}
	m.mu.Unlock()
	for index, session := range pending {
		m.update(id, index, "closing", "terminal_close_failed", "", false)
		for {
			op, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := m.driver.Close(op, session)
			cancel()
			if err == nil || errors.Is(err, terminal.ErrNotFound) || errors.Is(err, terminal.ErrClosed) {
				m.mu.Lock()
				delete(m.cleanup[id], index)
				m.mu.Unlock()
				m.update(id, index, "stopped", "", "", false)
				break
			}
			if !wait(ctx, m.options.RetryDelay) {
				m.update(id, index, "failed", "terminal_close_failed", "", false)
				break
			}
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for n := range m.jobs {
		if m.jobs[n].ID == id {
			m.jobs[n].State = "stopped"
			m.jobs[n].UpdatedAt = time.Now().UTC()
		}
	}
	if cancel := m.active[id]; cancel != nil {
		cancel()
	}
	delete(m.active, id)
	if len(m.cleanup[id]) == 0 {
		delete(m.cleanup, id)
	}
	_ = m.persistLocked()
}

func wrap(command, marker, shell string) string {
	if shell == "powershell" {
		payload := "$global:LASTEXITCODE=0; try { & {\n" + command + "\n}; if (-not $?) { if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; exit 1 }; exit $LASTEXITCODE } catch { Write-Error $_; exit 1 }"
		units := utf16.Encode([]rune(payload))
		data := make([]byte, len(units)*2)
		for i, u := range units {
			data[2*i] = byte(u)
			data[2*i+1] = byte(u >> 8)
		}
		return "& (Join-Path $PSHOME $(if ($PSVersionTable.PSEdition -eq 'Core') {'pwsh.exe'} else {'powershell.exe'})) -NoProfile -NonInteractive -EncodedCommand " + base64.StdEncoding.EncodeToString(data) + "; [Console]::WriteLine((\"`n" + marker + "{0}__\" -f $LASTEXITCODE))\r"
	}
	return "{ " + strings.TrimRight(command, "\r\n") + "\n}; printf '\\n" + marker + "%s__\\n' \"$?\"\r"
}

func display(raw, command, marker string) string {
	raw = ansi.ReplaceAllString(raw, "")
	raw = strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(raw, "\n")
	kept := lines[:0]
	first := strings.SplitN(strings.TrimRight(command, "\r\n"), "\n", 2)[0]
	for _, line := range lines {
		if !strings.Contains(line, marker) {
			kept = append(kept, strings.ReplaceAll(line, "{ "+first, first))
		}
	}
	return strings.TrimRight(strings.Join(kept, "\n"), "\n")
}
