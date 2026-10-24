package panel

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/kejilion/kejilion-panel/internal/cluster"
	"github.com/kejilion/kejilion-panel/internal/terminal"
	"github.com/kejilion/kejilion-panel/internal/terminalbatch"
)

type batchTerminalDriver struct{ server *Server }

func (d batchTerminalDriver) Open(ctx context.Context, user string, host terminalbatch.Host) (terminalbatch.Session, error) {
	s := d.server
	if !s.reserveTerminalOpen(user) {
		return terminalbatch.Session{}, terminal.ErrLimit
	}
	defer s.releaseTerminalOpen(user)
	publicID, err := randomTerminalSessionID()
	if err != nil {
		return terminalbatch.Session{}, err
	}
	owner := "panel:" + user
	var opened cluster.TerminalOpenResponse
	if host.ID == cluster.LocalHostID {
		snapshot, openErr := (clusterTerminalSource{agent: s.agent}).Open(ctx, owner, 30, 120)
		err = openErr
		opened = cluster.TerminalOpenResponse{SessionID: snapshot.ID, Offset: snapshot.Offset}
	} else {
		// TerminalOpen revalidates the current pairing and scope for every queued host.
		opened, err = s.cluster.TerminalOpen(ctx, host.ID, cluster.TerminalOpenRequest{Rows: 30, Columns: 120})
	}
	if err != nil {
		s.rememberRemoteTerminalOpen(user, host.ID, opened)
		return terminalbatch.Session{}, err
	}
	now := time.Now().UTC()
	item := panelTerminalSession{ID: publicID, BackendSessionID: opened.SessionID, HostID: host.ID, UserID: user, Owner: owner, CreatedAt: now, UpdatedAt: now, Purpose: "batch"}
	if !s.registerTerminalSession(item) {
		return terminalbatch.Session{}, cluster.ErrRemoteAccessDisabled
	}
	return terminalbatch.Session{ID: publicID, Offset: opened.Offset}, nil
}

func (d batchTerminalDriver) session(id string) (panelTerminalSession, error) {
	s := d.server
	s.terminalMu.Lock()
	defer s.terminalMu.Unlock()
	item, ok := s.terminalSessions[id]
	if !ok || item.Purpose != "batch" {
		return panelTerminalSession{}, terminal.ErrNotFound
	}
	item.UpdatedAt = time.Now().UTC()
	s.terminalSessions[id] = item
	return item, nil
}
func (d batchTerminalDriver) Input(ctx context.Context, session terminalbatch.Session, data string) error {
	item, err := d.session(session.ID)
	if err != nil {
		return err
	}
	return d.server.inputTerminalBackend(ctx, item, []byte(data))
}
func (d batchTerminalDriver) Output(ctx context.Context, session terminalbatch.Session, offset int64) (terminal.Output, error) {
	item, err := d.session(session.ID)
	if err != nil {
		return terminal.Output{}, err
	}
	// Keep the reservation until Close has confirmed actual target cleanup.
	return d.server.outputTerminalBackend(ctx, item, offset, time.Second)
}
func (d batchTerminalDriver) Close(ctx context.Context, session terminalbatch.Session) error {
	item, err := d.session(session.ID)
	if err != nil {
		return err
	}
	d.server.terminalMu.Lock()
	item.CloseRequested = true
	d.server.terminalSessions[item.ID] = item
	d.server.terminalMu.Unlock()
	err = d.server.closeTerminalBackend(ctx, item)
	if err == nil || errors.Is(err, terminal.ErrClosed) || errors.Is(err, terminal.ErrNotFound) {
		d.server.deleteTerminalSession(item.ID)
		return nil
	}
	return err
}

func (s *Server) handleTerminalBatch(w http.ResponseWriter, r *http.Request) {
	_, session, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet && (!s.checkOrigin(w, r) || !s.checkCSRF(w, r, session)) {
		return
	}
	if r.URL.RawQuery != "" || r.URL.RawPath != "" {
		s.writeProblem(w, r, 400, "invalid_batch_request", "Invalid batch request", "")
		return
	}
	if s.terminalBatches == nil {
		s.writeProblem(w, r, 503, "batch_unavailable", "Batch execution unavailable", "")
		return
	}
	const path = "/api/v1/terminal-batches"
	if r.URL.Path == path {
		switch r.Method {
		case http.MethodGet:
			items, warning, available := s.terminalBatches.List(session.User.ID)
			// List polling carries metadata only; output is fetched for one selected run.
			for n := range items {
				for i := range items[n].Results {
					items[n].Results[i].Output = ""
				}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"items": items, "warning": warning, "available": available})
		case http.MethodPost:
			var input struct {
				ID      string   `json:"id"`
				Command string   `json:"command"`
				HostIDs []string `json:"hostIds"`
			}
			if err := s.decodeJSON(w, r, &input); err != nil {
				return
			}
			if len(input.HostIDs) == 0 || len(input.HostIDs) > terminalbatch.MaxHosts {
				s.writeBatchError(w, r, terminalbatch.ErrInvalid)
				return
			}
			hosts := make([]terminalbatch.Host, 0, len(input.HostIDs))
			accessLifetime := s.cluster.RemoteOperationLifetime("terminal")
			for _, id := range input.HostIDs {
				host := cluster.Host{ID: cluster.LocalHostID, Name: "Local", IsLocal: true, TerminalAvailable: true, TerminalShell: "posix"}
				if id != cluster.LocalHostID {
					var err error
					host, err = s.cluster.Host(r.Context(), id)
					if err != nil || !host.TerminalAvailable {
						s.writeProblem(w, r, 409, "terminal_unavailable", "Target terminal unavailable", "")
						return
					}
				}
				shell := host.TerminalShell
				if shell == "" {
					shell = "posix"
				}
				target := terminalbatch.Host{ID: host.ID, Name: host.Name, Shell: shell}
				if !host.IsLocal {
					target.Context = accessLifetime
				}
				hosts = append(hosts, target)
			}
			job, err := s.terminalBatches.Create(session.User.ID, input.ID, input.Command, hosts)
			if err != nil {
				s.writeBatchError(w, r, err)
				return
			}
			_ = s.audit(r, session.User.ID, "terminal.batch.start", "terminal_batch", job.ID, "success", nil)
			s.writeJSON(w, http.StatusAccepted, job)
		default:
			s.writeProblem(w, r, 405, "method_not_allowed", "Method not allowed", "")
		}
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, path+"/")
	parts := strings.Split(rest, "/")
	if len(parts) == 1 && parts[0] != "" && r.Method == http.MethodGet {
		job, err := s.terminalBatches.Get(session.User.ID, parts[0])
		if err != nil {
			s.writeBatchError(w, r, err)
			return
		}
		s.writeJSON(w, 200, job)
		return
	}
	if len(parts) != 2 || parts[0] == "" {
		s.writeBatchError(w, r, terminalbatch.ErrNotFound)
		return
	}
	var err error
	action := ""
	if parts[1] == "stop" && r.Method == http.MethodPost {
		err = s.terminalBatches.Stop(session.User.ID, parts[0])
		action = "stop"
	} else if parts[1] == "record" && r.Method == http.MethodDelete {
		err = s.terminalBatches.Remove(session.User.ID, parts[0])
		action = "remove"
	} else {
		s.writeProblem(w, r, 405, "method_not_allowed", "Method not allowed", "")
		return
	}
	if err != nil {
		s.writeBatchError(w, r, err)
		return
	}
	_ = s.audit(r, session.User.ID, "terminal.batch."+action, "terminal_batch", parts[0], "success", nil)
	s.writeJSON(w, 200, map[string]bool{"accepted": true})
}

func (s *Server) writeBatchError(w http.ResponseWriter, r *http.Request, err error) {
	status, code := 503, "batch_unavailable"
	switch {
	case errors.Is(err, terminalbatch.ErrInvalid):
		status, code = 400, "invalid_batch_request"
	case errors.Is(err, terminalbatch.ErrBusy):
		status, code = 409, "batch_busy"
	case errors.Is(err, terminalbatch.ErrNotFound):
		status, code = 404, "batch_not_found"
	}
	s.writeProblem(w, r, status, code, "Batch request failed", "")
}
