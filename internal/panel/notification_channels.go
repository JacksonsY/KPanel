package panel

import (
	"net/http"
	"strings"

	"github.com/kejilion/kejilion-panel/internal/notification"
)

func (s *Server) handleNotificationChannel(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireClusterMutation(w, r)
	if !ok {
		return
	}
	if s.notifications == nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "cluster_notifications_unavailable", "Notifications unavailable", "")
		return
	}
	var input notification.ChannelUpdateInput
	var action notification.ChannelActionInput
	if r.Method == http.MethodPut {
		if err := s.decodeJSON(w, r, &input); err != nil {
			return
		}
		action.Provider = input.Provider
		action.ExpectedResourceVersion = input.ExpectedResourceVersion
	} else if err := s.decodeJSON(w, r, &action); err != nil {
		return
	}
	operation := "cluster.notifications.channel.update"
	if r.Method == http.MethodPost {
		operation = "cluster.notifications.channel.test"
		if strings.HasSuffix(r.URL.Path, "/discover") {
			operation = "cluster.notifications.channel.discover"
		}
	}
	change := map[string]any{"provider": auditNotificationProvider(action.Provider), "enabled": input.Enabled, "remove": input.Remove, "credentialProvided": input.Credential != ""}
	target := auditNotificationProvider(action.Provider)
	if err := s.audit(r, session.User.ID, operation, "cluster-notifications", target, "intent", change); err != nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "audit_unavailable", "Audit storage unavailable", "")
		return
	}
	var snapshot notification.Snapshot
	var err error
	if r.Method == http.MethodPut {
		snapshot, err = s.notifications.UpdateChannel(r.Context(), input)
	} else {
		snapshot, err = s.notifications.ChannelAction(r.Context(), action, strings.HasSuffix(r.URL.Path, "/discover"))
	}
	if err != nil {
		_ = s.audit(r, session.User.ID, operation, "cluster-notifications", target, "failure", change)
		s.writeNotificationError(w, r, err)
		return
	}
	_ = s.audit(r, session.User.ID, operation, "cluster-notifications", target, "success", change)
	s.writeJSON(w, http.StatusOK, snapshot)
}
