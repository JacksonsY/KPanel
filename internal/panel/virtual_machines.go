package panel

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/kejilion/kejilion-panel/internal/contract"
)

func (s *Server) handleVirtualMachines(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" || r.URL.RawPath != "" {
		s.writeProblem(w, r, 404, "route_not_found", "Route not found", "")
		return
	}
	token, session, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/api/v1/virtual-machines" {
		ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
		defer cancel()
		response, err := s.hostOps.Get(ctx, "/v1/virtual-machines", "", requestID(r))
		if err != nil {
			s.writeProblem(w, r, 503, "agent_unavailable", "Agent unavailable", "")
			return
		}
		s.writeAgentResponse(w, r, response)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/api/v1/virtual-machines/actions" {
		if !s.checkOrigin(w, r) || !s.checkCSRF(w, r, session) {
			return
		}
		var input contract.VMRequest
		if err := s.decodeJSON(w, r, &input); err != nil {
			return
		}
		if field, detail := contract.ValidateVMRequest(&input); field != "" {
			s.writeValidationProblem(w, r, field, detail)
			return
		}
		// Public keys, guest credentials, downloaded content and console frames
		// never enter the audit record.
		change := map[string]any{"action": input.Action, "id": input.ID, "name": input.Name, "deleteDisks": input.DeleteDisks}
		action := "virtual-machines." + input.Action
		if err := s.audit(r, session.User.ID, action, "virtual-machine", input.ID, "intent", change); err != nil {
			s.writeProblem(w, r, 503, "audit_unavailable", "Audit unavailable", "")
			return
		}
		body, _ := json.Marshal(input)
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 18*time.Second)
		defer cancel()
		response, err := s.hostOps.Do(ctx, http.MethodPost, "/v1/virtual-machines/actions", "", requestID(r), body)
		result := "unknown"
		if err == nil {
			result = "failure"
			if response.StatusCode == http.StatusAccepted {
				result = "accepted"
			}
		}
		_ = s.audit(r, session.User.ID, action, "virtual-machine", input.ID, result, change)
		if err != nil {
			s.writeProblem(w, r, 503, "vm_result_unknown", "Refresh tasks before retrying", "")
			return
		}
		s.writeAgentResponse(w, r, response)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/virtual-machines/"), "/console")
	if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/console") || !contract.ValidVMID(id) {
		s.writeProblem(w, r, 404, "route_not_found", "Route not found", "")
		return
	}
	if r.Header.Get("Origin") == "" || !s.checkOrigin(w, r) {
		return
	}
	select {
	case s.vmConsoleGate <- struct{}{}:
		defer func() { <-s.vmConsoleGate }()
	default:
		s.writeProblem(w, r, 429, "vm_busy", "Console capacity reached", "")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Hour)
	defer cancel()
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	defer ws.CloseNow()
	ws.SetReadLimit(1 << 20)
	// Authenticate the already session-bound socket with a first frame, never
	// with a URL credential. noVNC receives the channel only after this reply.
	authCtx, stop := context.WithTimeout(ctx, 5*time.Second)
	kind, payload, err := ws.Read(authCtx)
	stop()
	var hello struct {
		CSRF string `json:"csrf"`
	}
	if err != nil || kind != websocket.MessageText || len(payload) > 4096 || decodeTerminalSocketJSON(payload, &hello) != nil || !secureStringEqual(hello.CSRF, s.csrfCookieValue(r)) || s.auth.ValidateCSRF(session, hello.CSRF) != nil {
		return
	}
	if _, err = s.auth.Authenticate(token); err != nil {
		return
	}
	client, ok := s.agent.(interface {
		OpenVMConsole(context.Context, string) (*websocket.Conn, error)
	})
	if !ok {
		return
	}
	if err = s.audit(r, session.User.ID, "virtual-machines.console", "virtual-machine", id, "intent", nil); err != nil {
		return
	}
	upstream, err := client.OpenVMConsole(ctx, id)
	if err != nil {
		return
	}
	defer upstream.CloseNow()
	writeCtx, writeCancel := context.WithTimeout(ctx, 5*time.Second)
	err = ws.Write(writeCtx, websocket.MessageText, []byte(`{"ready":true}`))
	writeCancel()
	if err != nil {
		return
	}
	done := make(chan struct{}, 1)
	copyFrames := func(to, from *websocket.Conn) {
		defer cancel()
		for {
			kind, b, err := from.Read(ctx)
			if err != nil {
				return
			}
			if kind != websocket.MessageBinary {
				return
			}
			writeCtx, stop := context.WithTimeout(ctx, 10*time.Second)
			err = to.Write(writeCtx, kind, b)
			stop()
			if err != nil {
				return
			}
		}
	}
	go func() { copyFrames(upstream, ws); done <- struct{}{} }()
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, e := s.auth.Authenticate(token); e != nil || !time.Now().Before(session.ExpiresAt) {
					cancel()
					return
				}
				pingCtx, stop := context.WithTimeout(ctx, 5*time.Second)
				e := ws.Ping(pingCtx)
				stop()
				if e != nil {
					cancel()
					return
				}
			}
		}
	}()
	copyFrames(ws, upstream)
	cancel()
	<-done
}
