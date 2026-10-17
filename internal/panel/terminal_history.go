package panel

import (
	"errors"
	"net/http"
	"time"

	"github.com/kejilion/kejilion-panel/internal/terminalhistory"
)

const (
	terminalHistoryPath       = "/api/v1/terminal-history"
	terminalHistoryRemovePath = "/api/v1/terminal-history/remove"
)

// handleTerminalHistory serves the shared command history. Appends are not
// audited: they follow every command and the audit log never holds command
// text; clearing and removing record only counts.
func (s *Server) handleTerminalHistory(w http.ResponseWriter, r *http.Request) {
	allowed := http.MethodGet + ", " + http.MethodPost + ", " + http.MethodDelete
	remove := r.URL.Path == terminalHistoryRemovePath
	if remove {
		allowed = http.MethodPost
	}
	if (remove && r.Method != http.MethodPost) ||
		(!remove && r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodDelete) {
		w.Header().Set("Allow", allowed)
		s.writeProblem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	if r.URL.RawPath != "" || r.URL.RawQuery != "" {
		s.writeProblem(w, r, http.StatusBadRequest, "terminal_history_request_invalid", "Terminal history request is invalid", "")
		return
	}
	if r.Method != http.MethodGet && !s.checkOrigin(w, r) {
		return
	}
	_, session, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if s.terminalHistory == nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "terminal_history_unavailable", "Terminal history unavailable", "")
		return
	}
	if r.Method == http.MethodGet {
		s.writeJSON(w, http.StatusOK, s.terminalHistory.Snapshot())
		return
	}
	if !s.checkCSRF(w, r, session) {
		return
	}

	switch {
	case remove:
		var input terminalhistory.RemoveInput
		if err := decodeLimitedJSON(w, r, terminalhistory.MaxRequestBytes, &input); err != nil {
			return
		}
		if !s.auditTerminalHistoryIntent(w, r, session.User.ID, "terminal.history.remove") {
			return
		}
		snapshot, removed, err := s.terminalHistory.Remove(input)
		if err != nil {
			_ = s.audit(r, session.User.ID, "terminal.history.remove", "terminal-history", "collection", "failure", nil)
			s.writeTerminalHistoryError(w, r, err)
			return
		}
		removedCommands := 0
		if removed {
			removedCommands = 1
		}
		_ = s.audit(r, session.User.ID, "terminal.history.remove", "terminal-history", "collection", "success", map[string]any{"removedCommands": removedCommands})
		s.writeJSON(w, http.StatusOK, snapshot)
	case r.Method == http.MethodDelete:
		if !s.auditTerminalHistoryIntent(w, r, session.User.ID, "terminal.history.clear") {
			return
		}
		snapshot, cleared, err := s.terminalHistory.Clear()
		if err != nil {
			_ = s.audit(r, session.User.ID, "terminal.history.clear", "terminal-history", "collection", "failure", nil)
			s.writeTerminalHistoryError(w, r, err)
			return
		}
		_ = s.audit(r, session.User.ID, "terminal.history.clear", "terminal-history", "collection", "success", map[string]any{"removedCommands": cleared})
		s.writeJSON(w, http.StatusOK, snapshot)
	default:
		var input terminalhistory.AppendInput
		if err := decodeLimitedJSON(w, r, terminalhistory.MaxRequestBytes, &input); err != nil {
			return
		}
		snapshot, err := s.terminalHistory.Append(input, time.Now())
		if err != nil {
			s.writeTerminalHistoryError(w, r, err)
			return
		}
		s.writeJSON(w, http.StatusOK, snapshot)
	}
}

func (s *Server) auditTerminalHistoryIntent(w http.ResponseWriter, r *http.Request, actorID, action string) bool {
	if err := s.audit(r, actorID, action, "terminal-history", "collection", "intent", nil); err != nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "audit_unavailable", "Audit storage unavailable", "")
		return false
	}
	return true
}

func (s *Server) writeTerminalHistoryError(w http.ResponseWriter, r *http.Request, err error) {
	var validation *terminalhistory.ValidationError
	switch {
	case errors.As(err, &validation):
		s.writeValidationProblem(w, r, validation.Field, validation.Detail)
	case errors.Is(err, terminalhistory.ErrUnavailable):
		s.writeProblem(w, r, http.StatusServiceUnavailable, "terminal_history_unavailable", "Terminal history unavailable", "")
	default:
		s.writeProblem(w, r, http.StatusInternalServerError, "terminal_history_operation_failed", "Terminal history operation failed", "")
	}
}
