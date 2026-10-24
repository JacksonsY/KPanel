package panel

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/kejilion/kejilion-panel/internal/cluster"
	"github.com/kejilion/kejilion-panel/internal/terminal"
)

func (s *Server) handleClusterRemoteAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if _, _, ok := s.requireSession(w, r); !ok {
			return
		}
		s.writeJSON(w, http.StatusOK, s.cluster.RemoteAccess())
		return
	}
	if r.Method != http.MethodPut {
		s.writeProblem(w, r, 405, "method_not_allowed", "Method not allowed", "")
		return
	}
	session, ok := s.requireClusterMutation(w, r)
	if !ok {
		return
	}
	var input struct {
		TerminalEnabled         *bool  `json:"terminalEnabled"`
		FilesEnabled            *bool  `json:"filesEnabled"`
		ExpectedResourceVersion string `json:"expectedResourceVersion"`
	}
	if err := s.decodeJSON(w, r, &input); err != nil {
		return
	}
	if input.TerminalEnabled == nil || input.FilesEnabled == nil {
		s.writeProblem(w, r, 422, "remote_access_invalid", "Remote access settings are invalid", "")
		return
	}
	change := map[string]any{"terminalEnabled": *input.TerminalEnabled, "filesEnabled": *input.FilesEnabled}
	if err := s.audit(r, session.User.ID, "cluster.remote-access.update", "cluster-settings", "remote-access", "intent", change); err != nil {
		s.writeProblem(w, r, 503, "audit_unavailable", "Audit storage unavailable", "")
		return
	}
	// Registration uses this same lock and rechecks the policy. An open that
	// raced this update can never escape the close-requested set.
	s.terminalMu.Lock()
	value, err := s.cluster.UpdateRemoteAccess(input.ExpectedResourceVersion, *input.TerminalEnabled, *input.FilesEnabled)
	if err == nil && !value.TerminalEnabled {
		for id, item := range s.terminalSessions {
			if item.HostID == cluster.LocalHostID {
				continue
			}
			item.CloseRequested = true
			if item.InputConnection != nil {
				item.InputConnection.cancel()
			}
			s.terminalSessions[id] = item
		}
	}
	s.terminalMu.Unlock()
	if err != nil {
		_ = s.audit(r, session.User.ID, "cluster.remote-access.update", "cluster-settings", "remote-access", "failure", change)
		if errors.Is(err, cluster.ErrConflict) {
			s.writeProblem(w, r, 409, "remote_access_changed", "Remote access settings changed", "")
		} else {
			s.writeProblem(w, r, 503, "remote_access_storage_unavailable", "Remote access settings unavailable", "")
		}
		return
	}
	pending := 0
	if !value.TerminalEnabled {
		pending = s.closeDisabledRemoteTerminals(r.Context())
	}
	_ = s.audit(r, session.User.ID, "cluster.remote-access.update", "cluster-settings", "remote-access", "success", change)
	s.writeJSON(w, 200, struct {
		cluster.RemoteAccess
		PendingTerminalCleanup int `json:"pendingTerminalCleanup"`
	}{value, pending})
}

// Never lose identities on failed closure. A later save/explicit close retries
// them, while the admission policy already rejects input and reconnection.
func (s *Server) closeDisabledRemoteTerminals(parent context.Context) int {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	s.terminalMu.Lock()
	items := make([]panelTerminalSession, 0)
	for _, item := range s.terminalSessions {
		if item.HostID != cluster.LocalHostID && item.CloseRequested {
			items = append(items, item)
		}
	}
	s.terminalMu.Unlock()
	queue := make(chan panelTerminalSession, len(items))
	for _, item := range items {
		queue <- item
	}
	close(queue)
	var workers sync.WaitGroup
	for n := 0; n < min(4, len(items)); n++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range queue {
				if ctx.Err() != nil {
					return
				}
				err := s.closeTerminalBackend(ctx, item)
				if err == nil || errors.Is(err, terminal.ErrClosed) || errors.Is(err, terminal.ErrNotFound) {
					s.deleteTerminalSession(item.ID)
				}
			}
		}()
	}
	workers.Wait()
	s.terminalMu.Lock()
	defer s.terminalMu.Unlock()
	pending := 0
	for _, item := range s.terminalSessions {
		if item.HostID != cluster.LocalHostID && item.CloseRequested {
			pending++
		}
	}
	return pending
}

func (s *Server) registerTerminalSession(item panelTerminalSession) bool {
	s.terminalMu.Lock()
	denied := item.HostID != cluster.LocalHostID && !s.cluster.RemoteAccess().TerminalEnabled
	if denied {
		item.CloseRequested = true
	}
	s.terminalSessions[item.ID] = item
	s.terminalMu.Unlock()
	if denied {
		_ = s.closeDisabledRemoteTerminals(context.Background())
	}
	return !denied
}
