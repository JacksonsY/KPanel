package panel

import (
	"testing"
	"time"
)

func TestIndependentPendingPolicyCloseSurvivesIdlePruning(t *testing.T) {
	s, _ := newTestServer(t)
	s.terminalSessions["pending"] = panelTerminalSession{ID: "pending", HostID: "unreachable", BackendSessionID: "known-backend", CloseRequested: true, UpdatedAt: time.Now().Add(-time.Hour)}
	s.pruneTerminalSessions(time.Now().Add(-panelTerminalIdleTTL))
	if _, ok := s.terminalSessions["pending"]; !ok {
		t.Fatal("idle pruning discarded remote identity with unconfirmed policy close before backend cleanup")
	}
}
