package panel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kejilion/kejilion-panel/internal/sitetraffic"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *Server) collectSiteTraffic(ctx context.Context, input sitetraffic.CollectInput) (sitetraffic.Batch, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return sitetraffic.Batch{}, err
	}
	response, err := s.agent.Do(ctx, http.MethodPost, "/v1/site-traffic/collect", "", newRequestID(), body)
	if err != nil {
		return sitetraffic.Batch{}, err
	}
	if response.StatusCode != http.StatusOK {
		return sitetraffic.Batch{}, errors.New("traffic source unavailable")
	}
	var batch sitetraffic.Batch
	if err = json.Unmarshal(response.Body, &batch); err != nil {
		return batch, err
	}
	return batch, nil
}
func (s *Server) handleSiteTraffic(w http.ResponseWriter, r *http.Request) {
	_, session, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if r.URL.RawPath != "" {
		s.writeProblem(w, r, 400, "invalid_traffic_request", "网站监控请求无效", "")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if r.URL.Path == "/api/v1/site-traffic/actions" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			s.writeProblem(w, r, 405, "method_not_allowed", "请求方法不允许", "")
			return
		}
		if r.URL.RawQuery != "" {
			s.writeProblem(w, r, 422, "invalid_traffic_request", "网站监控请求无效", "")
			return
		}
		if !s.checkOrigin(w, r) || !s.checkCSRF(w, r, session) {
			return
		}
		var input struct {
			Action string `json:"action"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		var trailing any
		if err := decoder.Decode(&input); err != nil || decoder.Decode(&trailing) != io.EOF || (input.Action != "enable" && input.Action != "pause" && input.Action != "clear") {
			s.writeProblem(w, r, 422, "invalid_traffic_action", "监控操作无效", "")
			return
		}
		if err := s.audit(r, session.User.ID, "site.monitor."+input.Action, "site-traffic", "history", "intent", nil); err != nil {
			s.writeProblem(w, r, 503, "audit_unavailable", "Audit storage unavailable", "")
			return
		}
		if err := s.siteTraffic.Action(ctx, input.Action); err != nil {
			_ = s.audit(r, session.User.ID, "site.monitor."+input.Action, "site-traffic", "history", "failure", nil)
			s.writeProblem(w, r, 503, "site_traffic_unavailable", "统计库暂时不可用", "")
			return
		}
		_ = s.audit(r, session.User.ID, "site.monitor."+input.Action, "site-traffic", "history", "success", nil)
		s.writeJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		s.writeProblem(w, r, 405, "method_not_allowed", "请求方法不允许", "")
		return
	}
	query := r.URL.Query()
	for key, values := range query {
		if (key != "period" && key != "host") || len(values) != 1 {
			s.writeProblem(w, r, 422, "invalid_traffic_request", "监控筛选无效", "")
			return
		}
	}
	period := query.Get("period")
	if period == "" {
		period = "24h"
	}
	host := query.Get("host")
	if (period != "24h" && period != "7d" && period != "30d") || len(host) > 253 || strings.ContainsAny(host, "\x00\r\n") {
		s.writeProblem(w, r, 422, "invalid_traffic_request", "监控筛选无效", "")
		return
	}
	snapshot, err := s.siteTraffic.Query(ctx, period, host)
	if err != nil {
		s.writeProblem(w, r, 503, "site_traffic_unavailable", "网站监控暂时不可用", "")
		return
	}
	s.writeJSON(w, 200, snapshot)
}
