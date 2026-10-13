package filemanager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"hash"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

type receiveBlockedHash struct {
	hash.Hash
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (h *receiveBlockedHash) Write(data []byte) (int, error) {
	h.once.Do(func() { close(h.started); <-h.release })
	return h.Hash.Write(data)
}

func blockedReceiveWriter(t *testing.T) (*Manager, contract.FileReceiveSession, contract.FileReceiveInput, func(), <-chan error) {
	t.Helper()
	m := newReceiveManager(t, t.TempDir())
	input := testReceiveInput(4)
	session, err := m.BeginReceive(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	m.receives.mu.Lock()
	record := m.receives.records[session.ID]
	blocked := &receiveBlockedHash{Hash: record.hasher, started: make(chan struct{}), release: make(chan struct{})}
	record.hasher = blocked
	m.receives.mu.Unlock()
	var once sync.Once
	unblock := func() { once.Do(func() { close(blocked.release) }) }
	t.Cleanup(unblock)
	result := make(chan error, 1)
	go func() {
		_, err := m.WriteReceiveChunk(context.Background(), session.ID, input.SourceKey, 0, bytes.NewBufferString("data"), receiveDigest([]byte("data")))
		result <- err
	}()
	select {
	case <-blocked.started:
	case <-time.After(2 * time.Second):
		t.Fatal("writer did not reach the post-sync hash stage")
	}
	return m, session, input, unblock, result
}

func TestReceiveSlowChunkDoesNotBlockUnrelatedSession(t *testing.T) {
	m, first, _, unblock, result := blockedReceiveWriter(t)
	// Creating another session and querying it must not wait for the slow writer.
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	other := testReceiveInput(0)
	other.Name, other.SourceKey = "other.bin", strings.Repeat("b", 64)
	session, err := m.BeginReceive(ctx, other)
	if err != nil {
		t.Fatalf("unrelated session creation blocked by chunk: %v", err)
	}
	if status, err := m.ReceiveStatus(ctx, session.ID, other.SourceKey); err != nil || status.Offset != 0 {
		t.Fatalf("unrelated status blocked: %#v %v", status, err)
	}
	// Another session's journal flush must never publish speculative bytes.
	payload, err := os.ReadFile(filepath.Join(m.root, ".kpanel-trash", "receive-sessions.json"))
	var journal struct {
		Records []receiveRecord `json:"records"`
	}
	if err != nil || json.Unmarshal(payload, &journal) != nil {
		t.Fatalf("read concurrent checkpoint: %v", err)
	}
	found := false
	for _, saved := range journal.Records {
		if saved.Session.ID == first.ID {
			found = true
			if saved.Session.Offset != 0 || saved.Session.PrefixSHA256 != first.PrefixSHA256 {
				t.Fatalf("speculative checkpoint escaped: %#v", saved.Session)
			}
		}
	}
	if !found {
		t.Fatal("in-flight record missing from durable journal")
	}
	unblock()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestReceiveSlowChunkKeepsSameSessionAbortCancellable(t *testing.T) {
	m, session, input, unblock, result := blockedReceiveWriter(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := m.AbortReceive(ctx, session.ID, input.SourceKey); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("abort must wait for the writer without ignoring cancellation: %v", err)
	}
	unblock()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if err := m.AbortReceive(context.Background(), session.ID, input.SourceKey); err != nil {
		t.Fatal(err)
	}
}

func TestReceiveCloseWaitsForInFlightChunk(t *testing.T) {
	m, _, _, unblock, result := blockedReceiveWriter(t)
	closed := make(chan error, 1)
	go func() { closed <- m.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("roots closed while a chunk was in flight: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	unblock()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
}
