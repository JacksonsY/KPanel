package terminalhistory

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

var testNow = time.Date(2026, 10, 17, 8, 30, 0, 0, time.UTC)

func TestAppendKeepsDistinctCommandsNewestFirstAndSurvivesRestart(t *testing.T) {
	root := filepath.Join(t.TempDir(), "terminal-history")
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if initial := store.Snapshot(); !initial.Available || initial.SchemaVersion != SchemaVersion || initial.Items == nil || len(initial.Items) != 0 {
		t.Fatalf("unexpected initial snapshot: %#v", initial)
	}
	if runtime.GOOS != "windows" {
		assertMode(t, root, 0o700)
		assertMode(t, filepath.Join(root, "history.json"), 0o600)
	}

	if _, err := store.Append(AppendInput{Items: []AppendItem{
		{Command: "docker ps", HostID: "local"},
		{Command: "df -h", HostID: "0123456789abcdef0123456789abcdef"},
	}}, testNow); err != nil {
		t.Fatal(err)
	}
	later := testNow.Add(time.Minute)
	saved, err := store.Append(AppendInput{Items: []AppendItem{{Command: "docker ps", HostID: ""}}}, later)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Command: "docker ps", HostID: "", At: later.UnixMilli()},
		{Command: "df -h", HostID: "0123456789abcdef0123456789abcdef", At: testNow.UnixMilli()},
	}
	assertEntries(t, saved.Items, want)

	reopened, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	assertEntries(t, reopened.Snapshot().Items, want)
}

func TestAppendAndOpenRedactRecognizableCommandSecrets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "terminal-history")
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	command := "mysql -p history-secret-value"
	snapshot, err := store.Append(AppendInput{Items: []AppendItem{{Command: command, HostID: "local"}}}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "history-secret-value") || strings.Contains(snapshot.Items[0].Command, "history-secret-value") ||
		!strings.Contains(snapshot.Items[0].Command, "[REDACTED]") {
		t.Fatalf("append retained a recognizable secret: disk=%s snapshot=%#v", encoded, snapshot.Items)
	}
	if _, removed, err := store.Remove(RemoveInput{Command: command}); err != nil || !removed {
		t.Fatalf("remove of original command = removed %v, err %v", removed, err)
	}

	legacy := persistedState{SchemaVersion: SchemaVersion, Items: []Entry{{
		Command: command, HostID: "local", At: testNow.UnixMilli(),
	}}}
	encoded, err = json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = os.ReadFile(reopened.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "history-secret-value") || len(reopened.Snapshot().Items) != 1 {
		t.Fatalf("legacy history secret was not redacted on open: %s", encoded)
	}
}

func TestAppendTrimsToNewestEntries(t *testing.T) {
	store := openTestStore(t)
	for batch := 0; batch < (MaxEntries+40)/MaxAppendItems+1; batch++ {
		items := make([]AppendItem, 0, MaxAppendItems)
		for index := 0; index < MaxAppendItems; index++ {
			items = append(items, AppendItem{Command: fmt.Sprintf("echo %d", batch*MaxAppendItems+index), HostID: "local"})
		}
		if _, err := store.Append(AppendInput{Items: items}, testNow); err != nil {
			t.Fatal(err)
		}
	}
	items := store.Snapshot().Items
	last := ((MaxEntries+40)/MaxAppendItems+1)*MaxAppendItems - 1
	if len(items) != MaxEntries || items[0].Command != fmt.Sprintf("echo %d", last) ||
		items[MaxEntries-1].Command != fmt.Sprintf("echo %d", last-MaxEntries+1) {
		t.Fatalf("history was not trimmed to the newest entries: len=%d first=%q last=%q", len(items), items[0].Command, items[len(items)-1].Command)
	}
}

func TestAppendRejectsInvalidRequestsWithoutPartialWrites(t *testing.T) {
	store := openTestStore(t)
	for _, test := range []struct {
		name  string
		input AppendInput
		field string
	}{
		{name: "empty", input: AppendInput{}, field: "items"},
		{name: "too many", input: AppendInput{Items: make([]AppendItem, MaxAppendItems+1)}, field: "items"},
		{name: "blank", input: AppendInput{Items: []AppendItem{{Command: "   "}}}, field: "command"},
		{name: "outer whitespace", input: AppendInput{Items: []AppendItem{{Command: " ls"}}}, field: "command"},
		{name: "terminal control", input: AppendInput{Items: []AppendItem{{Command: "echo ok\x1b[2J"}}}, field: "command"},
		{name: "oversized", input: AppendInput{Items: []AppendItem{{Command: strings.Repeat("x", MaxCommandBytes+1)}}}, field: "command"},
		{name: "invalid utf8", input: AppendInput{Items: []AppendItem{{Command: "echo \xff"}}}, field: "command"},
		{name: "host", input: AppendInput{Items: []AppendItem{{Command: "uptime", HostID: "../etc"}}}, field: "items.hostId"},
		{name: "valid then invalid", input: AppendInput{Items: []AppendItem{{Command: "uptime", HostID: "local"}, {Command: "echo\x00"}}}, field: "command"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.Append(test.input, testNow); validationField(err) != test.field {
				t.Fatalf("error = %v, want field %s", err, test.field)
			}
			if items := store.Snapshot().Items; len(items) != 0 {
				t.Fatalf("invalid append changed history: %#v", items)
			}
		})
	}
	if _, err := store.Append(AppendInput{Items: []AppendItem{{Command: "for i in 1 2\ndo\techo $i\ndone", HostID: ""}}}, testNow); err != nil {
		t.Fatalf("multi-line batch command was rejected: %v", err)
	}
}

func TestRemoveAndClear(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.Append(AppendInput{Items: []AppendItem{{Command: "uptime"}, {Command: "free -m"}}}, testNow); err != nil {
		t.Fatal(err)
	}
	snapshot, removed, err := store.Remove(RemoveInput{Command: "uptime"})
	if err != nil || !removed {
		t.Fatalf("remove = %v removed=%v", err, removed)
	}
	assertEntries(t, snapshot.Items, []Entry{{Command: "free -m", At: testNow.UnixMilli()}})
	if _, removed, err := store.Remove(RemoveInput{Command: "uptime"}); err != nil || removed {
		t.Fatalf("removing a missing command = %v removed=%v, want idempotent success", err, removed)
	}
	if _, _, err := store.Remove(RemoveInput{Command: ""}); validationField(err) != "command" {
		t.Fatalf("blank remove error = %v", err)
	}
	cleared, count, err := store.Clear()
	if err != nil || count != 1 || len(cleared.Items) != 0 {
		t.Fatalf("clear = %#v count=%d err=%v", cleared, count, err)
	}
}

func TestCorruptStoreDegradesUntilCleared(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "malformed", data: []byte("{not-json")},
		{name: "unknown schema", data: []byte(`{"schemaVersion":99,"items":[]}`)},
		{name: "unknown field", data: []byte(`{"schemaVersion":1,"items":[],"extra":true}`)},
		{name: "duplicate", data: []byte(`{"schemaVersion":1,"items":[{"command":"ls","hostId":"","at":1},{"command":"ls","hostId":"","at":2}]}`)},
		{name: "missing time", data: []byte(`{"schemaVersion":1,"items":[{"command":"ls","hostId":""}]}`)},
		{name: "oversized", data: bytes.Repeat([]byte("x"), maxPersistedBytes+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "terminal-history")
			if _, err := Open(root); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "history.json")
			if err := os.WriteFile(path, test.data, 0o600); err != nil {
				t.Fatal(err)
			}
			store, err := Open(root)
			if err != nil {
				t.Fatalf("corrupt history must not fail panel initialization: %v", err)
			}
			snapshot := store.Snapshot()
			if snapshot.Available || snapshot.Warning != unavailableWarning || len(snapshot.Items) != 0 {
				t.Fatalf("unexpected degraded snapshot: %#v", snapshot)
			}
			if _, err := store.Append(AppendInput{Items: []AppendItem{{Command: "uptime"}}}, testNow); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("append error = %v, want unavailable", err)
			}
			if _, _, err := store.Remove(RemoveInput{Command: "uptime"}); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("remove error = %v, want unavailable", err)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(test.data, after) {
				t.Fatal("unavailable store overwrote the damaged history before an explicit clear")
			}
			cleared, count, err := store.Clear()
			if err != nil || count != 0 || !cleared.Available {
				t.Fatalf("clear did not recover the store: %#v count=%d err=%v", cleared, count, err)
			}
			if _, err := store.Append(AppendInput{Items: []AppendItem{{Command: "uptime"}}}, testNow); err != nil {
				t.Fatalf("append after recovery: %v", err)
			}
		})
	}
}

func TestPersistenceFailureLeavesSnapshotUnchanged(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.Append(AppendInput{Items: []AppendItem{{Command: "uptime"}}}, testNow); err != nil {
		t.Fatal(err)
	}
	store.writeAtomic = func(string, string, []byte) error { return errors.New("injected write failure") }
	if _, err := store.Append(AppendInput{Items: []AppendItem{{Command: "df -h"}}}, testNow); err == nil {
		t.Fatal("expected injected append failure")
	}
	if _, _, err := store.Remove(RemoveInput{Command: "uptime"}); err == nil {
		t.Fatal("expected injected remove failure")
	}
	if _, _, err := store.Clear(); err == nil {
		t.Fatal("expected injected clear failure")
	}
	assertEntries(t, store.Snapshot().Items, []Entry{{Command: "uptime", At: testNow.UnixMilli()}})
}

func TestConcurrentAppendsKeepEveryCommand(t *testing.T) {
	store := openTestStore(t)
	var wait sync.WaitGroup
	errs := make([]error, 16)
	for index := range errs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, errs[index] = store.Append(AppendInput{Items: []AppendItem{{Command: fmt.Sprintf("echo %d", index), HostID: "local"}}}, testNow)
		}(index)
	}
	wait.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if items := store.Snapshot().Items; len(items) != len(errs) {
		t.Fatalf("concurrent appends lost commands: %d", len(items))
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "terminal-history"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func assertEntries(t *testing.T, got, want []Entry) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("entries = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("entry %d = %#v, want %#v", index, got[index], want[index])
		}
	}
}

func validationField(err error) string {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return validation.Field
	}
	return ""
}

func assertMode(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != mode {
		t.Fatalf("%s mode = %o, want %o", path, info.Mode().Perm(), mode)
	}
}
