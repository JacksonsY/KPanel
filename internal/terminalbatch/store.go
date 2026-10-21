// Package terminalbatch schedules bounded terminal batches independently of browsers.
// Only operation metadata is persisted; PTY output and executable commands stay in memory.
package terminalbatch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/kejilion/kejilion-panel/internal/terminal"
)

const (
	maxOutputJobs   = 10 // Metadata outlives the bounded in-memory PTY output window.
	MaxHosts        = 64
	MaxCommandBytes = 16 << 10
	MaxOutputBytes  = 16 << 10
	maxFileBytes    = 2 << 20 // Bound storage by bytes, without evicting unexpired batches by count.
	retention       = 24 * time.Hour
)

var (
	ErrUnavailable = errors.New("batch history unavailable")
	ErrBusy        = errors.New("a batch is already active")
	ErrInvalid     = errors.New("invalid batch request")
	ErrNotFound    = errors.New("batch not found")
)

type Host struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Shell string `json:"shell"`
}

type Result struct {
	Host            Host   `json:"host"`
	State           string `json:"state"`
	Error           string `json:"error,omitempty"`
	Output          string `json:"output,omitempty"`
	Truncated       bool   `json:"truncated"`
	OutputAvailable bool   `json:"outputAvailable"`
}

type Job struct {
	ID        string    `json:"id"`
	UserID    string    `json:"-"`
	Command   string    `json:"command"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Results   []Result  `json:"results"`
}

type persistedJob struct {
	Job
	Owner string `json:"owner"`
}

type diskState struct {
	Version int            `json:"version"`
	Jobs    []persistedJob `json:"jobs"`
}

type Session struct {
	ID     string
	Offset int64
}

// Driver preserves the existing authenticated terminal transport and shared quotas.
// Open may be retried ONLY when it returns terminal.ErrLimit (no PTY was created).
type Driver interface {
	Open(context.Context, string, Host) (Session, error)
	Input(context.Context, Session, string) error
	Output(context.Context, Session, int64) (terminal.Output, error)
	Close(context.Context, Session) error
}

type Options struct {
	Timeout        time.Duration
	RetryDelay     time.Duration
	CleanupTimeout time.Duration
}

type Manager struct {
	mu        sync.Mutex
	path      string
	jobs      []Job
	driver    Driver
	active    map[string]context.CancelFunc
	cleanup   map[string]map[int]Session
	closed    bool
	available bool
	warning   string
	wg        sync.WaitGroup
	options   Options
	write     func(string, []byte) error
}

func Open(root string, driver Driver, options Options) (*Manager, error) {
	if !filepath.IsAbs(root) {
		return nil, ErrInvalid
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalid
	}
	if err := os.Chmod(root, 0700); err != nil {
		return nil, err
	}
	if options.Timeout <= 0 {
		options.Timeout = 4 * time.Hour
	}
	if options.CleanupTimeout <= 0 {
		options.CleanupTimeout = 30 * time.Second
	}
	if options.RetryDelay <= 0 {
		options.RetryDelay = time.Second
	}
	m := &Manager{path: filepath.Join(root, "jobs.json"), driver: driver, active: map[string]context.CancelFunc{}, cleanup: map[string]map[int]Session{}, available: true, options: options, write: atomicWrite}
	info, err = os.Lstat(m.path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	// Never overwrite an unknown/corrupt index or follow a link.
	if !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		m.available = false
		return m, nil
	}
	f, err := os.Open(m.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var disk diskState
	dec := json.NewDecoder(io.LimitReader(f, maxFileBytes+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&disk); err != nil {
		m.available = false
		return m, nil
	}
	var extra any
	if dec.Decode(&extra) != io.EOF || disk.Version != 1 {
		m.available = false
		return m, nil
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, saved := range disk.Jobs {
		j := saved.Job
		j.UserID = saved.Owner
		if !validID(j.ID) || seen[j.ID] || j.UserID == "" || len(j.UserID) > 256 || len(j.Command) > MaxCommandBytes || len(j.Results) == 0 || len(j.Results) > MaxHosts || !validJobState(j.State) || j.CreatedAt.IsZero() || j.UpdatedAt.IsZero() {
			m.available = false
			m.jobs = nil
			return m, nil
		}
		seen[j.ID] = true
		for i := range j.Results {
			r := &j.Results[i]
			if r.Host.ID == "" || len(r.Host.ID) > 128 || len(r.Host.Name) > 256 || len(r.Error) > 256 || !validResultState(r.State) || r.Output != "" {
				m.available = false
				m.jobs = nil
				return m, nil
			}
			r.OutputAvailable = false
			if !terminalState(r.State) {
				r.State = "interrupted"
				r.Error = "panel_restarted"
			}
		}
		if j.State == "running" {
			j.State = "interrupted"
			j.UpdatedAt = time.Now().UTC()
		}
		if time.Since(j.UpdatedAt) <= retention {
			m.jobs = append(m.jobs, j)
		}
	}
	if err := os.Chmod(m.path, 0600); err != nil {
		return nil, err
	}
	// Persist interruption before accepting any new work. Never replay saved commands.
	if err := m.persistLocked(); err != nil {
		m.available = false
	}
	return m, nil
}

func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func terminalState(s string) bool {
	return s == "succeeded" || s == "failed" || s == "timed_out" || s == "stopped" || s == "interrupted"
}
func validResultState(s string) bool {
	return terminalState(s) || s == "queued" || s == "connecting" || s == "running" || s == "closing"
}
func validJobState(s string) bool {
	return s == "running" || s == "completed" || s == "stopped" || s == "interrupted"
}

func (m *Manager) Create(user, id, command string, hosts []Host) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.available || m.closed {
		return Job{}, ErrUnavailable
	}
	// Idempotency is checked before admission, including completed requests.
	for _, j := range m.jobs {
		if j.ID == id {
			if j.UserID != user {
				return Job{}, ErrInvalid
			}
			return clone(j), nil
		}
	}
	if !validID(id) || user == "" || strings.TrimSpace(command) == "" || len(command) > MaxCommandBytes || !utf8.ValidString(command) || strings.ContainsRune(command, 0) || len(hosts) == 0 || len(hosts) > MaxHosts {
		return Job{}, ErrInvalid
	}
	seen := map[string]bool{}
	for _, h := range hosts {
		if h.ID == "" || len(h.ID) > 128 || len(h.Name) > 256 || seen[h.ID] || h.Shell != "posix" && h.Shell != "powershell" {
			return Job{}, ErrInvalid
		}
		seen[h.ID] = true
	}
	if len(m.active) > 0 {
		return Job{}, ErrBusy
	}
	now := time.Now().UTC()
	summary := commandSummary(command)
	j := Job{ID: id, UserID: user, Command: summary, State: "running", CreatedAt: now, UpdatedAt: now, Results: make([]Result, len(hosts))}
	for i, h := range hosts {
		j.Results[i] = Result{Host: h, State: "queued", OutputAvailable: true}
	}
	previous := m.jobs
	m.jobs = append([]Job{j}, m.jobs...)
	m.pruneLocked(now)
	if err := m.persistLocked(); err != nil {
		m.jobs = previous
		return Job{}, ErrUnavailable
	}
	// Release only old output after durable admission; retain every job and host
	// record. Admission is serialized and only one batch may produce output.
	for n := maxOutputJobs; n < len(m.jobs); n++ {
		for i := range m.jobs[n].Results {
			m.jobs[n].Results[i].Output = ""
			m.jobs[n].Results[i].OutputAvailable = false
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), m.options.Timeout)
	m.active[id] = cancel
	m.wg.Add(1)
	go m.run(ctx, id, user, command)
	return clone(j), nil
}

var summaryExecutable = regexp.MustCompile(`^[A-Za-z0-9_./-]{1,64}$`)

// A history label is deliberately a command name, not a best-effort copy of
// arguments: short flags, positional credentials and custom formats are not
// reliably redactable. Only fixed, non-secret k action names may be included.
func commandSummary(command string) string {
	if strings.HasPrefix(command, " ") {
		return "[excluded from command history]"
	}
	fields := strings.Fields(command)
	if len(fields) == 0 || !summaryExecutable.MatchString(fields[0]) {
		return "[custom command]"
	}
	name := fields[0]
	if len(fields) == 1 {
		return name
	}
	if name == "k" {
		switch fields[1] {
		case "更新", "update", "web", "docker", "清理", "系统信息":
			if len(fields) == 2 {
				return name + " " + fields[1]
			}
			return name + " " + fields[1] + " …"
		}
	}
	return name + " …"
}

func clone(j Job) Job { j.Results = append([]Result(nil), j.Results...); return j }

func (m *Manager) List(user string) ([]Job, string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	jobs := make([]Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		if j.UserID == user && (j.State == "running" || len(m.cleanup[j.ID]) > 0 || time.Since(j.UpdatedAt) <= retention) {
			jobs = append(jobs, clone(j))
		}
	}
	return jobs, m.warning, m.available
}

func (m *Manager) Get(user, id string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.ID == id && j.UserID == user && (j.State == "running" || len(m.cleanup[id]) > 0 || time.Since(j.UpdatedAt) <= retention) {
			return clone(j), nil
		}
	}
	return Job{}, ErrNotFound
}

func (m *Manager) Stop(user, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.ID == id && j.UserID == user {
			if cancel := m.active[id]; cancel != nil {
				// Repeated cleanup retries are idempotent and never cancel each other.
				if j.State == "running" {
					cancel()
				}
			} else if len(m.cleanup[id]) > 0 && !m.closed {
				ctx, cancel := context.WithTimeout(context.Background(), m.options.CleanupTimeout)
				m.active[id] = cancel
				m.wg.Add(1)
				go m.retryCleanup(ctx, id)
			}
			return nil
		}
	}
	return ErrNotFound
}

func (m *Manager) Remove(user, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, j := range m.jobs {
		if j.ID == id && j.UserID == user {
			if m.active[id] != nil || len(m.cleanup[id]) > 0 {
				return ErrBusy
			}
			previous := append([]Job(nil), m.jobs...)
			m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
			if err := m.persistLocked(); err != nil {
				m.jobs = previous
				return ErrUnavailable
			}
			return nil
		}
	}
	return ErrNotFound
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	for _, cancel := range m.active {
		cancel()
	}
	m.mu.Unlock()
	m.wg.Wait()
}

func (m *Manager) pruneLocked(now time.Time) {
	kept := m.jobs[:0]
	for _, j := range m.jobs {
		if j.State == "running" || len(m.cleanup[j.ID]) > 0 || now.Sub(j.UpdatedAt) <= retention {
			kept = append(kept, j)
		}
	}
	m.jobs = kept
}

func (m *Manager) persistLocked() error {
	disk := diskState{Version: 1, Jobs: make([]persistedJob, 0, len(m.jobs))}
	for _, j := range m.jobs {
		j = clone(j)
		for i := range j.Results {
			j.Results[i].Output = ""
			j.Results[i].OutputAvailable = false
		}
		disk.Jobs = append(disk.Jobs, persistedJob{Job: j, Owner: j.UserID})
	}
	data, err := json.Marshal(disk)
	if err == nil && len(data) > maxFileBytes {
		err = ErrInvalid
	}
	if err == nil {
		err = m.write(m.path, data)
	}
	if err != nil {
		m.warning = "batch_history_write_failed"
	} else {
		m.warning = ""
	}
	return err
}

func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".batch-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func tail(s string, n int) string {
	if len(s) <= n {
		return strings.ToValidUTF8(s, "")
	}
	s = s[len(s)-n:]
	for len(s) > 0 && !utf8.RuneStart(s[0]) {
		s = s[1:]
	}
	return strings.ToValidUTF8(s, "")
}
