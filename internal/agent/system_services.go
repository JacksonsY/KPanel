package agent

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
	"github.com/kejilion/kejilion-panel/internal/systemmanage"
)

func (s *Server) systemServices(w http.ResponseWriter, r *http.Request) {
	requestID := requestIDFrom(w)
	name, logs, valid := contract.ParseSystemServicePath(r.URL.Path, "/v1/system/services")
	if r.URL.RawQuery != "" || (r.URL.Path != "/v1/system/services" && !valid) {
		writeProblem(w, requestID, http.StatusUnprocessableEntity, "invalid_system_service_query", "服务查询参数无效", "")
		return
	}
	select {
	case s.serviceReadGate <- struct{}{}:
		defer func() { <-s.serviceReadGate }()
	default:
		writeProblem(w, requestID, http.StatusTooManyRequests, "system_services_busy", "服务查询正在执行，请稍后重试", "")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	var result any
	var err error
	if r.URL.Path == "/v1/system/services" {
		result, err = s.systemManager.SystemServices(ctx)
	} else if logs {
		result, err = s.systemManager.SystemServiceLogs(ctx, name)
	} else {
		result, err = s.systemManager.SystemService(ctx, name)
	}
	if err != nil {
		s.writeSystemServiceError(w, requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) systemServiceAction(w http.ResponseWriter, r *http.Request) {
	requestID := requestIDFrom(w)
	if r.URL.RawPath != "" || r.URL.RawQuery != "" {
		writeProblem(w, requestID, http.StatusBadRequest, "invalid_system_service_action", "服务操作 URL 无效", "")
		return
	}
	var input contract.SystemServiceActionRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeProblem(w, requestID, http.StatusBadRequest, "invalid_request", "请求格式无效", "")
		return
	}
	if field, detail := contract.ValidateSystemServiceAction(&input); field != "" {
		writeProblem(w, requestID, http.StatusUnprocessableEntity, "invalid_system_service_action", "服务操作参数无效", field+": "+detail)
		return
	}
	result, err := s.systemManager.ExecuteSystemServiceAction(r.Context(), input)
	if err != nil {
		s.writeSystemServiceError(w, requestID, err)
		return
	}
	status := http.StatusOK
	if result.Accepted {
		status = http.StatusAccepted
	}
	writeJSON(w, status, result)
}

func (s *Server) writeSystemServiceError(w http.ResponseWriter, requestID string, err error) {
	status, code, title := http.StatusServiceUnavailable, "system_services_unavailable", "服务操作结果无法确认，请刷新后核对状态"
	switch {
	case errors.Is(err, systemmanage.ErrInvalidInput):
		status, code, title = http.StatusUnprocessableEntity, "invalid_system_service", "服务名称、参数或状态无效"
	case errors.Is(err, systemmanage.ErrDisabled):
		status, code, title = http.StatusForbidden, "system_services_write_disabled", "宿主机系统写入未启用"
	case errors.Is(err, systemmanage.ErrSystemServicesBusy):
		status, code, title = http.StatusTooManyRequests, "system_services_busy", "另一项服务操作正在执行"
	case errors.Is(err, systemmanage.ErrConflict):
		status, code, title = http.StatusConflict, "system_service_conflict", "服务状态已变化，请刷新后重试"
	case errors.Is(err, context.DeadlineExceeded):
		status, code, title = http.StatusGatewayTimeout, "system_services_timeout", "服务操作超时，执行结果未确认，请刷新状态"
	case errors.Is(err, systemmanage.ErrUnsupported):
		code, title = "system_services_unsupported", "当前主机不支持该服务操作"
	case errors.Is(err, systemmanage.ErrNeedsAttention):
		code, title = "system_services_needs_attention", "服务操作结果需要核对，请刷新状态"
	}
	writeProblem(w, requestID, status, code, title, safeDetail(err))
}
