package panel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

func (s *Server) handleSystemServiceRead(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireSession(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		s.writeProblem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	name, logs, valid := contract.ParseSystemServicePath(r.URL.Path, "/api/v1/system/services")
	if r.URL.RawQuery != "" || (r.URL.Path != "/api/v1/system/services" && !valid) {
		s.writeProblem(w, r, http.StatusNotFound, "route_not_found", "Route not found", "")
		return
	}
	agentPath := "/v1/system/services"
	if valid {
		agentPath += "/" + url.PathEscape(name)
		if logs {
			agentPath += "/logs"
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 14*time.Second)
	defer cancel()
	response, err := s.hostOps.Get(ctx, agentPath, "", requestID(r))
	if err != nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "agent_unavailable", "Agent unavailable", "")
		return
	}
	s.writeAgentResponse(w, r, response)
}

func (s *Server) handleSystemServiceAction(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawPath != "" || r.URL.RawQuery != "" {
		s.writeProblem(w, r, http.StatusNotFound, "route_not_found", "Route not found", "")
		return
	}
	if !s.checkOrigin(w, r) {
		return
	}
	_, session, ok := s.requireSession(w, r)
	if !ok || !s.checkCSRF(w, r, session) {
		return
	}
	var input contract.SystemServiceActionRequest
	if err := s.decodeJSON(w, r, &input); err != nil {
		return
	}
	if field, detail := contract.ValidateSystemServiceAction(&input); field != "" {
		s.writeValidationProblem(w, r, field, detail)
		return
	}
	body, err := json.Marshal(input)
	if err != nil {
		s.writeProblem(w, r, http.StatusInternalServerError, "request_encoding_failed", "Request encoding failed", "")
		return
	}
	action := "system.services." + input.Action
	change := map[string]any{"name": input.Name, "action": input.Action}
	if err := s.audit(r, session.User.ID, action, "system-service", input.Name, "intent", change); err != nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "audit_unavailable", "Audit storage unavailable", "")
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 18*time.Second)
	defer cancel()
	response, err := s.hostOps.Do(ctx, http.MethodPost, "/v1/system/service-actions", "", requestID(r), body)
	if err != nil {
		_ = s.audit(r, session.User.ID, action, "system-service", input.Name, "unknown", change)
		s.writeProblem(w, r, http.StatusServiceUnavailable, "agent_unavailable", "服务操作结果无法确认，请刷新后核对状态", "")
		return
	}
	result := "failure"
	if response.StatusCode >= http.StatusInternalServerError {
		result = "unknown"
	}
	if response.StatusCode == http.StatusAccepted {
		result = "accepted"
	} else if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		result = "success"
	}
	_ = s.audit(r, session.User.ID, action, "system-service", input.Name, result, change)
	s.writeAgentResponse(w, r, response)
}
