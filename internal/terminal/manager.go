package terminal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kejilion/kejilion-panel/internal/hostpty"
)

const (
	DefaultBufferBytes      = 1 << 20
	DefaultMaxSessions      = 16
	DefaultMaxOwnerSessions = 4
	DefaultIdleTimeout      = 30 * time.Minute
	DefaultLifetime         = 8 * time.Hour
	MaxInputBytes           = 16 << 10
	MaxOutputBytes          = 64 << 10
	maxFinishedSessions     = 64
	maxFinishedBufferBytes  = 16 << 20
	finishedRetention       = 5 * time.Minute
)

var (
	ErrNotFound       = errors.New("terminal session not found")
	ErrLimit          = errors.New("terminal session limit reached")
	ErrClosed         = errors.New("terminal session is closed")
	ErrOffset         = errors.New("terminal output offset is invalid")
	ErrCleanupPending = errors.New("terminal process cleanup is pending")
)

type Process interface {
	io.ReadWriteCloser
	Wait() error
	Kill() error
	Resize(rows, columns uint16) error
}

type Starter func(rows, columns uint16) (Process, error)

type Config struct {
	Starter          Starter
	ParentUnit       string
	BufferBytes      int
	MaxSessions      int
	MaxOwnerSessions int
	IdleTimeout      time.Duration
	Lifetime         time.Duration
	Now              func() time.Time
}

type Manager struct {
	mu       sync.RWMutex
	config   Config
	sessions map[string]*session
	stop     chan struct{}
	stopOnce sync.Once
	closed   bool
	// Admission includes queued and spawning processes. Owner epochs exist only
	// while admissions are pending, bounded by the ordinary session limits.
	opening       int
	openingOwners map[string]*ownerAdmission
	openSpawnMu   sync.Mutex
}

type ownerAdmission struct {
	pending int
	epoch   uint64
}

type session struct {
	inputGate   chan struct{}
	inputState  reliableInputState
	closeMu     sync.Mutex
	mu          sync.Mutex
	id          string
	retained    bool
	owner       string
	process     Process
	buffer      []byte
	base        int64
	next        int64
	notify      chan struct{}
	createdAt   time.Time
	updatedAt   time.Time
	finishedAt  time.Time
	exitedAt    *time.Time
	exitError   string
	closed      bool
	terminated  bool
	closeFailed bool
}

type Snapshot struct {
	Retained  bool       `json:"retained,omitempty"`
	ID        string     `json:"id"`
	Offset    int64      `json:"offset"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	ExitedAt  *time.Time `json:"exitedAt,omitempty"`
	ExitError string     `json:"exitError,omitempty"`
	Closed    bool       `json:"closed"`
}

type Output struct {
	Data       []byte     `json:"data"`
	Offset     int64      `json:"offset"`
	NextOffset int64      `json:"nextOffset"`
	Truncated  bool       `json:"truncated"`
	ExitedAt   *time.Time `json:"exitedAt,omitempty"`
	ExitError  string     `json:"exitError,omitempty"`
	Closed     bool       `json:"closed"`
}

func New(config Config) *Manager {
	if config.Starter == nil {
		parentUnit := config.ParentUnit
		config.Starter = func(rows, columns uint16) (Process, error) {
			return starterWithParent(rows, columns, parentUnit)
		}
	}
	if config.BufferBytes <= 0 {
		config.BufferBytes = DefaultBufferBytes
	}
	if config.MaxSessions <= 0 {
		config.MaxSessions = DefaultMaxSessions
	}
	if config.MaxOwnerSessions <= 0 {
		config.MaxOwnerSessions = DefaultMaxOwnerSessions
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = DefaultIdleTimeout
	}
	if config.Lifetime <= 0 {
		config.Lifetime = DefaultLifetime
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	manager := &Manager{config: config, sessions: make(map[string]*session), stop: make(chan struct{})}
	go manager.reapLoop()
	return manager
}

func starterWithParent(rows, columns uint16, parentUnit string) (Process, error) {
	if process, err, handled := platformStarter(rows, columns); handled {
		return process, err
	}
	shell := "/bin/bash"
	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/sh"
	}
	directory := "/"
	if root, err := os.Open("/root"); err == nil {
		if info, statErr := root.Stat(); statErr == nil && info.IsDir() {
			directory = "/root"
		}
		_ = root.Close()
	}
	environment := terminalEnvironment(shell)
	if os.Getenv("INVOCATION_ID") != "" {
		systemdRun, runErr := trustedTerminalExecutable("systemd-run")
		systemctl, controlErr := trustedTerminalExecutable("systemctl")
		if runErr == nil && controlErr == nil {
			identity, err := randomID()
			if err != nil {
				return nil, err
			}
			unit := "kpanel-terminal-" + identity
			command := transientTerminalCommandWithParent(systemdRun, shell, directory, unit, environment, parentUnit)
			process, err := hostpty.Start(command, rows, columns)
			if err != nil {
				return nil, err
			}
			return &transientTerminalProcess{Process: process, systemctl: systemctl, unit: unit}, nil
		}
	}
	command := exec.Command(shell, "-l")
	command.Dir = directory
	command.Env = environment
	return hostpty.Start(command, rows, columns)
}

func terminalEnvironment(shell string) []string {
	return []string{
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"HOME=/root",
		"USER=root",
		"LOGNAME=root",
		"SHELL=" + shell,
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"LANG=C.UTF-8",
	}
}

func transientTerminalCommand(systemdRun, shell, directory, unit string, environment []string) *exec.Cmd {
	return transientTerminalCommandWithParent(systemdRun, shell, directory, unit, environment, "kejilion-agent.service")
}

func transientTerminalCommandWithParent(systemdRun, shell, directory, unit string, environment []string, parentUnit string) *exec.Cmd {
	arguments := []string{
		"--quiet",
		"--wait",
		"--collect",
		"--pty",
		"--unit=" + unit,
		"--property=Type=exec",
		"--property=User=root",
		"--property=Group=root",
		"--property=WorkingDirectory=" + directory,
		"--property=UMask=0077",
		"--property=NoNewPrivileges=no",
		"--property=ProtectSystem=no",
		"--property=ProtectHome=no",
		"--property=KillMode=mixed",
		"--property=SendSIGHUP=yes",
		"--property=RuntimeMaxSec=8h",
	}
	if strings.TrimSpace(parentUnit) != "" {
		arguments = append(arguments,
			"--property=PartOf="+strings.TrimSpace(parentUnit),
			"--property=After="+strings.TrimSpace(parentUnit),
		)
	}
	for _, value := range environment {
		arguments = append(arguments, "--setenv="+value)
	}
	arguments = append(arguments, "--", shell, "-l")
	command := exec.Command(systemdRun, arguments...)
	command.Dir = directory
	command.Env = environment
	return command
}

func trustedTerminalExecutable(name string) (string, error) {
	for _, directory := range []string{"/usr/bin", "/bin"} {
		candidate := directory + "/" + name
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o022 == 0 && terminalExecutableOwnerTrusted(info) {
			return candidate, nil
		}
	}
	return "", errors.New("trusted " + name + " was not found")
}

type transientTerminalProcess struct {
	Process
	systemctl string
	unit      string
	stopMu    sync.Mutex
	stopped   bool
}

func (process *transientTerminalProcess) stopUnit() error {
	process.stopMu.Lock()
	defer process.stopMu.Unlock()
	if process.stopped {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, process.systemctl, "stop", process.unit).Run(); err != nil {
		// --collect can unload the unit before a retry reaches systemd. Only
		// an authoritative not-found state makes that failure idempotent.
		state, stateErr := exec.CommandContext(ctx, process.systemctl, "show", process.unit, "--property=LoadState", "--value").Output()
		if stateErr != nil || strings.TrimSpace(string(state)) != "not-found" {
			return err
		}
	}
	process.stopped = true
	return nil
}

func (process *transientTerminalProcess) Close() error {
	if err := process.stopUnit(); err != nil {
		return err
	}
	return process.Process.Close()
}

func (process *transientTerminalProcess) Kill() error {
	if err := process.stopUnit(); err != nil {
		return err
	}
	return process.Process.Kill()
}

func (m *Manager) Open(owner string, rows, columns uint16) (Snapshot, error) {
	return m.OpenWithStarter(owner, rows, columns, m.config.Starter)
}

// OpenWithStarter uses the same ownership, quotas and lifecycle for a fixed
// backend-selected terminal process. The starter is never supplied over HTTP.
func (m *Manager) OpenWithStarter(owner string, rows, columns uint16, start Starter) (Snapshot, error) {
	return m.open(context.Background(), owner, rows, columns, start, false)
}

// OpenContext propagates cancellation through admission and final registration.
func (m *Manager) OpenContext(ctx context.Context, owner string, rows, columns uint16) (Snapshot, error) {
	return m.open(ctx, owner, rows, columns, m.config.Starter, false)
}

// OpenRetained keeps an authenticated session across client disconnects, bounded by Lifetime.
func (m *Manager) OpenRetained(owner string, rows, columns uint16) (Snapshot, error) {
	return m.OpenRetainedContext(context.Background(), owner, rows, columns)
}

func (m *Manager) OpenRetainedContext(ctx context.Context, owner string, rows, columns uint16) (Snapshot, error) {
	return m.open(ctx, owner, rows, columns, m.config.Starter, true)
}

func (m *Manager) reserveOpen(owner string) (*ownerAdmission, uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, 0, ErrClosed
	}
	m.trimFinishedLocked(m.config.Now().UTC())
	active, owned := 0, 0
	for _, item := range m.sessions {
		if item.isActive() {
			active++
			if item.owner == owner {
				owned++
			}
		}
	}
	state := m.openingOwners[owner]
	pending := 0
	if state != nil {
		pending = state.pending
	}
	if active+m.opening >= m.config.MaxSessions || owned+pending >= m.config.MaxOwnerSessions {
		return nil, 0, ErrLimit
	}
	if state == nil {
		if m.openingOwners == nil {
			m.openingOwners = make(map[string]*ownerAdmission)
		}
		state = &ownerAdmission{}
		m.openingOwners[owner] = state
	}
	state.pending++
	m.opening++
	return state, state.epoch, nil
}

func (m *Manager) releaseOpenLocked(owner string, state *ownerAdmission) {
	state.pending--
	m.opening--
	if state.pending == 0 {
		delete(m.openingOwners, owner)
	}
}

func (m *Manager) open(ctx context.Context, owner string, rows, columns uint16, start Starter, retained bool) (Snapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if start == nil {
		return Snapshot{}, errors.New("terminal starter is required")
	}
	owner = strings.TrimSpace(owner)
	if owner == "" || rows == 0 || columns == 0 || rows > 500 || columns > 1000 {
		return Snapshot{}, errors.New("invalid terminal session request")
	}
	state, epoch, err := m.reserveOpen(owner)
	if err != nil {
		return Snapshot{}, err
	}
	reserved := true
	defer func() {
		if reserved {
			m.mu.Lock()
			m.releaseOpenLocked(owner, state)
			m.mu.Unlock()
		}
	}()
	m.openSpawnMu.Lock()
	defer m.openSpawnMu.Unlock()
	m.mu.RLock()
	invalid := m.closed || state.epoch != epoch
	m.mu.RUnlock()
	if invalid {
		return Snapshot{}, ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	id, err := randomID()
	if err != nil {
		return Snapshot{}, err
	}
	// The spawn (fork + systemd-run) can take tens of milliseconds; run it
	// without mu so every other session's Input/Output/Resize stays live.
	// The reservation keeps both quota and backup busy-visibility intact.
	process, spawnErr := start(rows, columns)
	now := m.config.Now().UTC()
	item := &session{retained: retained, id: id, owner: owner, process: process, notify: make(chan struct{}), inputGate: make(chan struct{}, 1), createdAt: now, updatedAt: now}
	m.mu.Lock()
	if spawnErr == nil {
		spawnErr = ctx.Err()
	}
	if spawnErr == nil && (m.closed || state.epoch != epoch) {
		spawnErr = ErrClosed
	}
	m.releaseOpenLocked(owner, state)
	reserved = false
	if process == nil {
		m.mu.Unlock()
		if spawnErr == nil {
			spawnErr = errors.New("terminal starter returned no process")
		}
		return Snapshot{}, spawnErr
	}
	m.sessions[id] = item
	m.mu.Unlock()
	// Every started process needs one Wait, including failed or revoked opens.
	// Close terminates its scope and PTY; capture owns reaping the child.
	go m.capture(item)
	if spawnErr != nil {
		// A started process whose cleanup fails still occupies the same owner
		// and global capacity. The reaper retries it even though Open failed.
		_ = m.Close(owner, id)
		return Snapshot{}, spawnErr
	}
	return item.snapshot(), nil
}

// Busy includes shells whose output has ended but whose process scope has not
// been explicitly closed. A spawn starts outside mu, so it is counted while in
// flight; it cannot escape the host backup admission check by being between
// creation and registration.
func (m *Manager) Busy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.opening != 0 {
		return true
	}
	for _, item := range m.sessions {
		item.mu.Lock()
		open := !item.closed
		item.mu.Unlock()
		if open {
			return true
		}
	}
	return false
}

func (m *Manager) capture(item *session) {
	buffer := make([]byte, 32<<10)
	var readErr error
	for {
		read, err := item.process.Read(buffer)
		if read > 0 {
			item.append(buffer[:read], m.config.BufferBytes, m.config.Now().UTC())
		}
		if err != nil {
			if !hostpty.IsEnd(err) && !errors.Is(err, os.ErrClosed) {
				readErr = err
			}
			break
		}
	}
	err := item.process.Wait()
	if errors.Is(err, ErrCleanupPending) {
		item.mu.Lock()
		pending := !item.closed
		if pending {
			item.closeFailed = true
		}
		item.mu.Unlock()
		if pending {
			item.append([]byte("\r\nTerminal disconnected; process cleanup will be retried.\r\n"), m.config.BufferBytes, m.config.Now().UTC())
		}
		return
	}
	if err == nil {
		err = readErr
	}
	item.setExit(err, m.config.Now().UTC())
	// EOF/Wait does not close the PTY or prove that the shell's process group
	// is gone. Keep ownership and capacity until Close confirms cleanup.
	_ = m.Close(item.owner, item.id)
}

func (m *Manager) Output(ctx context.Context, owner, id string, offset int64, wait time.Duration) (Output, error) {
	item, err := m.lookup(owner, id)
	if err != nil {
		return Output{}, err
	}
	if wait < 0 || wait > 1500*time.Millisecond {
		return Output{}, errors.New("invalid terminal wait duration")
	}
	for {
		output, notify, ready, err := item.output(offset, MaxOutputBytes, m.config.Now().UTC())
		if err != nil || ready || wait == 0 {
			return output, err
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return Output{}, ctx.Err()
		case <-notify:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			return item.currentOutput(offset, MaxOutputBytes, m.config.Now().UTC())
		}
	}
}

func (m *Manager) Input(owner, id string, data []byte) error {
	return m.InputContext(context.Background(), owner, id, data)
}

func (m *Manager) InputContext(ctx context.Context, owner, id string, data []byte) error {
	if len(data) == 0 || len(data) > MaxInputBytes {
		return errors.New("invalid terminal input size")
	}
	item, err := m.lookup(owner, id)
	if err != nil {
		return err
	}
	if err := item.lockInput(ctx); err != nil {
		return err
	}
	defer func() { <-item.inputGate }()
	item.mu.Lock()
	if item.closeFailed && !item.closed {
		item.mu.Unlock()
		return ErrCleanupPending
	}
	if item.closed || item.exitedAt != nil {
		item.mu.Unlock()
		return ErrClosed
	}
	if item.inputState.stream != "" {
		item.mu.Unlock()
		return ErrInputSequence
	}
	item.mu.Unlock()
	n, err := item.process.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		item.mu.Lock()
		item.updatedAt = m.config.Now().UTC()
		item.mu.Unlock()
	}
	return err
}

func (item *session) lockInput(ctx context.Context) error {
	select {
	case item.inputGate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-item.inputGate
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) Resize(owner, id string, rows, columns uint16) error {
	if rows == 0 || columns == 0 || rows > 500 || columns > 1000 {
		return errors.New("invalid terminal dimensions")
	}
	item, err := m.lookup(owner, id)
	if err != nil {
		return err
	}
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.closeFailed && !item.closed {
		return ErrCleanupPending
	}
	if item.closed || item.exitedAt != nil {
		return ErrClosed
	}
	if err := item.process.Resize(rows, columns); err != nil {
		return err
	}
	item.updatedAt = m.config.Now().UTC()
	return nil
}

func (m *Manager) Close(owner, id string) error {
	item, err := m.lookup(owner, id)
	if err != nil {
		return err
	}
	item.closeMu.Lock()
	defer item.closeMu.Unlock()
	item.mu.Lock()
	if item.closed {
		item.mu.Unlock()
		return nil
	}
	// Retain ownership and capacity while stop is pending or has failed. Do not
	// hold the output/state lock across systemd I/O: that can stall all lookups
	// through Open's manager lock as well as the terminal's output capture.
	item.closeFailed = true
	item.mu.Unlock()
	if !item.terminated {
		if err := item.process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		item.terminated = true
	}
	if err := item.process.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return err
	}
	item.mu.Lock()
	item.closeFailed = false
	item.closed = true
	item.updatedAt = m.config.Now().UTC()
	if item.finishedAt.IsZero() {
		item.finishedAt = item.updatedAt
	}
	close(item.notify)
	item.notify = make(chan struct{})
	item.mu.Unlock()
	m.trimFinished()
	return nil
}

// CloseOwner terminates both relay and stream sessions when their controller epoch changes.
// Failed cleanup stays registered for the normal retry and quota accounting.
func (m *Manager) CloseOwner(owner string) {
	owner = strings.TrimSpace(owner)
	m.mu.Lock()
	if state := m.openingOwners[owner]; state != nil {
		state.epoch++
	}
	ids := make([]string, 0)
	for id, item := range m.sessions {
		if item.owner == owner {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Close(owner, id)
	}
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	m.closed = true
	items := make([]*session, 0, len(m.sessions))
	for _, item := range m.sessions {
		items = append(items, item)
	}
	m.mu.Unlock()
	m.stopOnce.Do(func() { close(m.stop) })
	for _, item := range items {
		_ = m.Close(item.owner, item.id)
	}
}

func (m *Manager) reapLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.reap(m.config.Now().UTC())
		case <-m.stop:
			return
		}
	}
}

func (m *Manager) reap(now time.Time) {
	type staleSession struct{ owner, id string }
	stale := make([]staleSession, 0)
	m.mu.Lock()
	m.trimFinishedLocked(now)
	for id, item := range m.sessions {
		item.mu.Lock()
		inactive := (!item.retained && now.Sub(item.updatedAt) >= m.config.IdleTimeout) || now.Sub(item.createdAt) >= m.config.Lifetime
		if !item.closed && (inactive || item.closeFailed || item.exitedAt != nil) {
			stale = append(stale, staleSession{owner: item.owner, id: id})
		}
		item.mu.Unlock()
	}
	m.mu.Unlock()
	for _, item := range stale {
		_ = m.Close(item.owner, item.id)
	}
}

func (m *Manager) trimFinished() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trimFinishedLocked(m.config.Now().UTC())
}

// Completed output is a bounded cache. Reads cannot renew its retention, and
// pending cleanup remains owned and counted until termination is confirmed.
func (m *Manager) trimFinishedLocked(now time.Time) {
	type entry struct {
		id    string
		at    time.Time
		bytes int
	}
	finished := make([]entry, 0)
	total := 0
	for id, item := range m.sessions {
		item.mu.Lock()
		done := item.closed && !item.closeFailed
		at, size := item.finishedAt, cap(item.buffer)
		item.mu.Unlock()
		if !done {
			continue
		}
		if !at.IsZero() && now.Sub(at) >= finishedRetention {
			delete(m.sessions, id)
			continue
		}
		finished = append(finished, entry{id, at, size})
		total += size
	}
	sort.Slice(finished, func(i, j int) bool {
		if finished[i].at.Equal(finished[j].at) {
			return finished[i].id < finished[j].id
		}
		return finished[i].at.Before(finished[j].at)
	})
	for len(finished) > maxFinishedSessions || total > maxFinishedBufferBytes {
		oldest := finished[0]
		delete(m.sessions, oldest.id)
		total -= oldest.bytes
		finished = finished[1:]
	}
}

// IsRetained only reports sessions owned by the authenticated caller.
func (m *Manager) IsRetained(owner, id string) bool {
	item, err := m.lookup(owner, id)
	return err == nil && item.retained
}

func (m *Manager) lookup(owner, id string) (*session, error) {
	m.mu.RLock()
	item := m.sessions[id]
	m.mu.RUnlock()
	if item == nil || item.owner != strings.TrimSpace(owner) {
		return nil, ErrNotFound
	}
	return item, nil
}

func (item *session) isActive() bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	return !item.closed
}

func (item *session) snapshot() Snapshot {
	item.mu.Lock()
	defer item.mu.Unlock()
	// A new reader must receive output already captured during Open, including
	// the first prompt. Starting at zero also preserves truncation reporting.
	return Snapshot{Retained: item.retained, ID: item.id, Offset: 0, CreatedAt: item.createdAt, UpdatedAt: item.updatedAt, ExitedAt: item.exitedAt, ExitError: item.exitError, Closed: item.closed}
}

func (item *session) append(data []byte, limit int, now time.Time) {
	item.mu.Lock()
	defer item.mu.Unlock()
	item.buffer = append(item.buffer, data...)
	item.next += int64(len(data))
	if len(item.buffer) > limit {
		drop := len(item.buffer) - limit
		copy(item.buffer, item.buffer[drop:])
		item.buffer = item.buffer[:limit]
		item.base += int64(drop)
	}
	item.updatedAt = now
	close(item.notify)
	item.notify = make(chan struct{})
}

func (item *session) setExit(err error, now time.Time) {
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.exitedAt != nil {
		return
	}
	item.exitedAt = &now
	if item.finishedAt.IsZero() {
		item.finishedAt = now
	}
	item.updatedAt = now
	if err != nil {
		item.exitError = err.Error()
	}
	close(item.notify)
	item.notify = make(chan struct{})
}

func (item *session) output(offset int64, limit int, now time.Time) (Output, <-chan struct{}, bool, error) {
	item.mu.Lock()
	defer item.mu.Unlock()
	if offset < 0 || offset > item.next {
		return Output{}, nil, false, ErrOffset
	}
	truncated := offset < item.base
	if truncated {
		offset = item.base
	}
	start := int(offset - item.base)
	available := len(item.buffer) - start
	if available > limit {
		available = limit
	}
	data := append([]byte(nil), item.buffer[start:start+available]...)
	item.updatedAt = now
	result := Output{Data: data, Offset: offset, NextOffset: offset + int64(len(data)), Truncated: truncated, ExitedAt: item.exitedAt, ExitError: item.exitError, Closed: item.closed}
	return result, item.notify, len(data) > 0 || item.closed || item.exitedAt != nil, nil
}

func (item *session) currentOutput(offset int64, limit int, now time.Time) (Output, error) {
	output, _, _, err := item.output(offset, limit, now)
	return output, err
}

func randomID() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
