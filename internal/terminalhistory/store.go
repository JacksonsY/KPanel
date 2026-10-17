// Package terminalhistory keeps the single panel-wide list of commands that
// administrators ran from the terminal page. Commands are kept newest first,
// deduplicated by text, and never written to audit or panel backups.
package terminalhistory

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	SchemaVersion   = 1
	MaxEntries      = 200
	MaxCommandBytes = 8 << 10
	MaxAppendItems  = 32
	// One append carries at most 32 full-size commands plus JSON framing.
	MaxRequestBytes    = 320 << 10
	maxPersistedBytes  = 4 << 20
	unavailableWarning = "terminal_history_unavailable"
)

var (
	ErrUnavailable   = errors.New("terminal history is unavailable")
	errInvalidOnDisk = errors.New("terminal history data is invalid")
	hostIDPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

// Entry is one distinct command. HostID names the host it last ran on; an
// empty HostID means it last ran as a multi-host batch command.
type Entry struct {
	Command string `json:"command"`
	HostID  string `json:"hostId"`
	At      int64  `json:"at"`
}

type Snapshot struct {
	SchemaVersion int     `json:"schemaVersion"`
	Available     bool    `json:"available"`
	Warning       string  `json:"warning,omitempty"`
	Items         []Entry `json:"items"`
}

type AppendItem struct {
	Command string `json:"command"`
	HostID  string `json:"hostId"`
}

// AppendInput lists commands oldest first, in the order they ran.
type AppendInput struct {
	Items []AppendItem `json:"items"`
}

type RemoveInput struct {
	Command string `json:"command"`
}

type ValidationError struct {
	Field  string
	Detail string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Detail
}

type persistedState struct {
	SchemaVersion int     `json:"schemaVersion"`
	Items         []Entry `json:"items"`
}

type atomicWriter func(directory, target string, data []byte) error

type Store struct {
	mu          sync.RWMutex
	root        string
	path        string
	state       persistedState
	available   bool
	writeAtomic atomicWriter
}

func Open(root string) (*Store, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "." || !filepath.IsAbs(root) {
		return nil, errors.New("terminal history root must be absolute")
	}
	if err := ensurePrivateDirectory(root); err != nil {
		return nil, fmt.Errorf("initialize terminal history directory: %w", err)
	}
	store := &Store{
		root:        root,
		path:        filepath.Join(root, "history.json"),
		state:       emptyState(),
		available:   true,
		writeAtomic: writeAtomicPrivateFile,
	}
	state, err := readState(store.path)
	switch {
	case err == nil:
		store.state = state
		if chmodErr := os.Chmod(store.path, 0o600); chmodErr != nil {
			return nil, fmt.Errorf("protect terminal history: %w", chmodErr)
		}
	case errors.Is(err, os.ErrNotExist):
		if err := store.persistLocked(store.state); err != nil {
			return nil, err
		}
	case errors.Is(err, errInvalidOnDisk):
		// Keep the unreadable file for inspection; Clear starts a fresh list.
		store.available = false
	default:
		return nil, err
	}
	return store, nil
}

func ValidHostID(value string) bool {
	return value == "" || hostIDPattern.MatchString(value)
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked()
}

// Append moves each command to the top of the list, refreshing its host and
// time, and keeps the newest MaxEntries. The whole request is rejected when
// any item is invalid.
func (s *Store) Append(input AppendInput, now time.Time) (Snapshot, error) {
	if len(input.Items) == 0 || len(input.Items) > MaxAppendItems {
		return Snapshot{}, &ValidationError{Field: "items", Detail: "between 1 and 32 commands are required"}
	}
	for _, item := range input.Items {
		if err := validateCommand(item.Command); err != nil {
			return Snapshot{}, err
		}
		if !ValidHostID(item.HostID) {
			return Snapshot{}, &ValidationError{Field: "items.hostId", Detail: "host ID is invalid"}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.available {
		return Snapshot{}, ErrUnavailable
	}
	at := now.UnixMilli()
	items := cloneEntries(s.state.Items)
	for _, item := range input.Items {
		items = withoutCommand(items, item.Command)
		items = append([]Entry{{Command: item.Command, HostID: item.HostID, At: at}}, items...)
	}
	if len(items) > MaxEntries {
		items = items[:MaxEntries]
	}
	next := persistedState{SchemaVersion: SchemaVersion, Items: items}
	if err := s.persistLocked(next); err != nil {
		return Snapshot{}, err
	}
	s.state = next
	return s.snapshotLocked(), nil
}

// Remove deletes one command; removing a command that is already gone succeeds.
func (s *Store) Remove(input RemoveInput) (Snapshot, bool, error) {
	if err := validateCommand(input.Command); err != nil {
		return Snapshot{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.available {
		return Snapshot{}, false, ErrUnavailable
	}
	items := withoutCommand(cloneEntries(s.state.Items), input.Command)
	if len(items) == len(s.state.Items) {
		return s.snapshotLocked(), false, nil
	}
	next := persistedState{SchemaVersion: SchemaVersion, Items: items}
	if err := s.persistLocked(next); err != nil {
		return Snapshot{}, false, err
	}
	s.state = next
	return s.snapshotLocked(), true, nil
}

// Clear empties the list. It also replaces an unreadable file, so clearing is
// the recovery path when the stored history is damaged.
func (s *Store) Clear() (Snapshot, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleared := 0
	if s.available {
		cleared = len(s.state.Items)
	}
	next := emptyState()
	if err := s.persistLocked(next); err != nil {
		return Snapshot{}, 0, err
	}
	s.state = next
	s.available = true
	return s.snapshotLocked(), cleared, nil
}

func (s *Store) snapshotLocked() Snapshot {
	if !s.available {
		return Snapshot{SchemaVersion: SchemaVersion, Available: false, Warning: unavailableWarning, Items: []Entry{}}
	}
	return Snapshot{SchemaVersion: SchemaVersion, Available: true, Items: cloneEntries(s.state.Items)}
}

func (s *Store) persistLocked(state persistedState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode terminal history: %w", err)
	}
	data = append(data, '\n')
	if len(data) > maxPersistedBytes {
		return &ValidationError{Field: "items", Detail: "terminal history exceeds the storage limit"}
	}
	if err := s.writeAtomic(s.root, s.path, data); err != nil {
		return fmt.Errorf("persist terminal history: %w", err)
	}
	return nil
}

func emptyState() persistedState {
	return persistedState{SchemaVersion: SchemaVersion, Items: []Entry{}}
}

func withoutCommand(items []Entry, command string) []Entry {
	kept := items[:0]
	for _, item := range items {
		if item.Command != command {
			kept = append(kept, item)
		}
	}
	return kept
}

func readState(path string) (persistedState, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return persistedState{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return persistedState{}, errors.New("terminal history must be a regular file")
	}
	if info.Size() <= 0 || info.Size() > maxPersistedBytes {
		return persistedState{}, errInvalidOnDisk
	}
	file, err := os.Open(path)
	if err != nil {
		return persistedState{}, fmt.Errorf("open terminal history: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPersistedBytes+1))
	if err != nil {
		return persistedState{}, fmt.Errorf("read terminal history: %w", err)
	}
	if len(data) == 0 || len(data) > maxPersistedBytes {
		return persistedState{}, errInvalidOnDisk
	}
	var state persistedState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return persistedState{}, errInvalidOnDisk
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return persistedState{}, errInvalidOnDisk
	}
	if err := validateState(state); err != nil {
		return persistedState{}, errInvalidOnDisk
	}
	state.Items = cloneEntries(state.Items)
	return state, nil
}

func validateState(state persistedState) error {
	if state.SchemaVersion != SchemaVersion || state.Items == nil || len(state.Items) > MaxEntries {
		return errInvalidOnDisk
	}
	seen := make(map[string]bool, len(state.Items))
	for _, item := range state.Items {
		if validateCommand(item.Command) != nil || !ValidHostID(item.HostID) || item.At <= 0 || seen[item.Command] {
			return errInvalidOnDisk
		}
		seen[item.Command] = true
	}
	return nil
}

func validateCommand(command string) error {
	if command == "" || strings.TrimSpace(command) != command || !utf8.ValidString(command) ||
		len(command) > MaxCommandBytes || hasControl(command) {
		return &ValidationError{Field: "command", Detail: "command must contain 1 to 8192 bytes without unsupported controls or outer whitespace"}
	}
	return nil
}

func hasControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\t' {
			return true
		}
	}
	return false
}

func cloneEntries(source []Entry) []Entry {
	cloned := make([]Entry, len(source))
	copy(cloned, source)
	return cloned
}

func ensurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("path must be a non-symlink directory")
	}
	return os.Chmod(path, 0o700)
}

func writeAtomicPrivateFile(directory, target string, data []byte) error {
	file, err := os.CreateTemp(directory, ".terminal-history-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, target); err == nil {
		_ = syncDirectory(directory)
		return nil
	} else if runtime.GOOS != "windows" {
		return err
	}

	backup := target + ".previous"
	_ = os.Remove(backup)
	if err := os.Rename(target, backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(temporary, target); err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	_ = os.Remove(backup)
	_ = syncDirectory(directory)
	return nil
}

func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
