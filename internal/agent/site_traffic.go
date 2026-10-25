package agent

import (
	"context"
	"errors"
	"github.com/kejilion/kejilion-panel/internal/sitetraffic"
	"net/http"
	"time"
)

func (s *Server) siteTrafficCollect(w http.ResponseWriter, r *http.Request, requestID string) {
	if r.URL.RawPath != "" || r.URL.RawQuery != "" {
		writeProblem(w, requestID, http.StatusBadRequest, "invalid_traffic_request", "流量采集请求无效", "")
		return
	}
	var input sitetraffic.CollectInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeProblem(w, requestID, http.StatusBadRequest, "invalid_traffic_request", "流量采集请求无效", "")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	batch, err := s.siteTraffic.Collect(ctx, input)
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, sitetraffic.ErrInput) {
			status = http.StatusUnprocessableEntity
		}
		if errors.Is(err, sitetraffic.ErrBusy) {
			status = http.StatusTooManyRequests
		}
		writeProblem(w, requestID, status, "site_traffic_unavailable", "网站日志暂时无法采集", "")
		return
	}
	writeJSON(w, http.StatusOK, batch)
}
