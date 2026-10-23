package cluster

import (
	"errors"
	"strconv"
	"testing"
	"time"
)

func TestReplayGuardIsolatesPrincipalCapacityAndBoundsGlobalEntries(t *testing.T) {
	now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)
	guard := newReplayGuard(4, 2, 5*time.Minute)

	for _, nonce := range []string{"a1", "a2"} {
		if err := guard.Accept("light:node-a", nonce, now); err != nil {
			t.Fatalf("Accept(node-a, %q) error: %v", nonce, err)
		}
	}
	if err := guard.Accept("light:node-a", "a1", now); !errors.Is(err, ErrReplay) {
		t.Fatalf("duplicate at principal capacity error = %v, want ErrReplay", err)
	}
	if err := guard.Accept("light:node-a", "a3", now); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("over-quota principal error = %v, want ErrRateLimited", err)
	}

	for _, nonce := range []string{"b1", "b2"} {
		if err := guard.Accept("light:node-b", nonce, now); err != nil {
			t.Fatalf("Accept(node-b, %q) should remain available: %v", nonce, err)
		}
	}
	if err := guard.Accept("light:node-c", "c1", now); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("over-capacity global cache error = %v, want ErrRateLimited", err)
	}

	if err := guard.Accept("light:node-c", "c1", now.Add(5*time.Minute)); err != nil {
		t.Fatalf("expired entries should free global and principal capacity: %v", err)
	}
}

func TestReplayGuardProductionPrincipalLimitLeavesOtherNodesAvailable(t *testing.T) {
	now := time.Date(2026, 10, 23, 11, 0, 0, 0, time.UTC)
	guard := newReplayGuard(replayGuardTotalEntries, replayGuardEntriesPerPrincipal, 5*time.Minute)
	principal := replayLightPrincipal("node-a")

	for i := 0; i < replayGuardEntriesPerPrincipal; i++ {
		if err := guard.Accept(principal, strconv.Itoa(i), now); err != nil {
			t.Fatalf("Accept(node-a, entry %d) error before configured principal limit: %v", i, err)
		}
	}
	if err := guard.Accept(principal, "over-limit", now); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("over-limit light principal error = %v, want ErrRateLimited", err)
	}
	if err := guard.Accept(replayLightPrincipal("node-b"), "first", now); err != nil {
		t.Fatalf("a full light principal must leave another node available: %v", err)
	}
}
