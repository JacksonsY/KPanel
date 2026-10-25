package agent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/kejilion/kejilion-panel/internal/contract"
	"github.com/kejilion/kejilion-panel/internal/virtualmachine"
)

func (s *Server) virtualMachines(w http.ResponseWriter, r *http.Request) {
	requestID := requestIDFrom(w)
	if r.URL.RawQuery != "" || r.URL.RawPath != "" {
		writeProblem(w, requestID, 400, "vm_invalid_request", "Invalid virtual machine request", "")
		return
	}
	if r.URL.Path == "/v1/virtual-machines" {
		s.requireMethod(w, r, requestID, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
			result, err := s.virtualMachineService.Snapshot(r.Context())
			if err != nil {
				s.vmError(w, requestID, err)
				return
			}
			writeJSON(w, 200, result)
		})
		return
	}
	if r.URL.Path == "/v1/virtual-machines/actions" {
		s.requireMethod(w, r, requestID, http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
			var input contract.VMRequest
			if err := decodeJSON(w, r, &input); err != nil {
				writeProblem(w, requestID, 400, "vm_invalid_request", "Invalid virtual machine request", "")
				return
			}
			result, err := s.virtualMachineService.Start(r.Context(), input)
			if err != nil {
				s.vmError(w, requestID, err)
				return
			}
			writeJSON(w, 202, result)
		})
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/virtual-machines/"), "/console")
	if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/console") || !contract.ValidVMID(id) {
		writeProblem(w, requestID, 404, "not_found", "Not found", "")
		return
	}
	select {
	case s.vmConsoleGate <- struct{}{}:
		defer func() { <-s.vmConsoleGate }()
	default:
		s.vmError(w, requestID, virtualmachine.ErrBusy)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Hour)
	defer cancel()
	conn, err := s.virtualMachineService.Console(ctx, id)
	if err != nil {
		s.vmError(w, requestID, err)
		return
	}
	defer conn.Close()
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	defer ws.CloseNow()
	ws.SetReadLimit(1 << 20)
	pipe := websocket.NetConn(ctx, ws, websocket.MessageBinary)
	done := make(chan struct{}, 1)
	go func() { _, _ = io.Copy(conn, pipe); _ = conn.Close(); done <- struct{}{} }()
	_, _ = io.Copy(pipe, conn)
	cancel()
	_ = conn.Close()
	_ = pipe.Close()
	<-done
}

func (s *Server) vmError(w http.ResponseWriter, id string, err error) {
	status, code := http.StatusServiceUnavailable, "vm_unavailable"
	switch {
	case errors.Is(err, virtualmachine.ErrInvalid):
		status, code = 422, "vm_invalid_request"
	case errors.Is(err, virtualmachine.ErrConflict):
		status, code = 409, "vm_conflict"
	case errors.Is(err, virtualmachine.ErrBusy):
		status, code = 429, "vm_busy"
	case errors.Is(err, virtualmachine.ErrDisabled):
		status, code = 403, "vm_write_disabled"
	case errors.Is(err, virtualmachine.ErrUnsupported):
		code = "vm_unsupported"
	}
	writeProblem(w, id, status, code, "Virtual machine operation unavailable", safeDetail(err))
}
