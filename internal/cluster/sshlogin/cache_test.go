package sshlogin

import (
	"context"
	"testing"
	"time"
)

func TestCachedLoginNeverWaitsForAnExistingLogRead(t *testing.T) {
	r := NewReader(Config{})
	r.mu.Lock()
	_, _ = r.CachedLatestSSHLogin(context.Background())
	for i := 0; i < 100; i++ {
		_, _ = r.CachedLatestSSHLogin(context.Background())
	}
	r.refreshMu.Lock()
	pending := r.refreshPending
	r.refreshMu.Unlock()
	if !pending {
		r.mu.Unlock()
		t.Fatal("expected one retained background read")
	}
	r.mu.Unlock()
	deadline := time.After(4 * time.Second)
	for {
		r.refreshMu.Lock()
		done := !r.refreshPending
		after := r.refreshAfter
		r.refreshMu.Unlock()
		if done {
			if after.IsZero() {
				t.Fatal("failed reads need a cooldown too")
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("refresh did not finish")
		case <-time.After(time.Millisecond):
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = r.CachedLatestSSHLogin(context.Background())
	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()
	if r.refreshPending {
		t.Fatal("cache cooldown did not suppress a repeated scan")
	}
}
