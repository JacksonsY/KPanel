package agent

import (
	"context"
	"errors"
	"github.com/kejilion/kejilion-panel/internal/browsercomponent"
	"github.com/kejilion/kejilion-panel/internal/contract"
	"net/http"
	"time"
)

func (s *Server) browserComponentRequest(w http.ResponseWriter, r *http.Request) {
	id := requestIDFrom(w)
	if r.URL.RawPath != "" || r.URL.RawQuery != "" {
		writeProblem(w, id, 400, "browser_component_invalid", "Invalid component request", "")
		return
	}
	if s.browserComponent == nil {
		writeProblem(w, id, 503, "browser_component_unsupported", "Browser component management unavailable", "")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	if r.URL.Path == "/v1/browser/component" {
		s.requireMethod(w, r, id, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
			v, err := s.browserComponent.Snapshot(ctx)
			if err != nil {
				s.browserComponentError(w, id, err)
				return
			}
			writeJSON(w, 200, v)
		})
		return
	}
	s.requireMethod(w, r, id, http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		var input contract.BrowserComponentRequest
		if err := decodeJSON(w, r, &input); err != nil {
			writeProblem(w, id, 400, "browser_component_invalid", "Invalid component request", "")
			return
		}
		v, err := s.browserComponent.Start(ctx, input)
		if err != nil {
			s.browserComponentError(w, id, err)
			return
		}
		writeJSON(w, 202, v)
	})
}
func (s *Server) browserComponentError(w http.ResponseWriter, id string, err error) {
	status, code := 503, "browser_component_unavailable"
	switch {
	case errors.Is(err, browsercomponent.ErrInvalid):
		status, code = 422, "browser_component_invalid"
	case errors.Is(err, browsercomponent.ErrConflict):
		status, code = 409, "browser_component_changed"
	case errors.Is(err, browsercomponent.ErrBusy):
		status, code = 409, "browser_component_busy"
	case errors.Is(err, browsercomponent.ErrUnsupported):
		code = "browser_component_unsupported"
	}
	writeProblem(w, id, status, code, "Browser component operation failed; refresh component status", "")
}
