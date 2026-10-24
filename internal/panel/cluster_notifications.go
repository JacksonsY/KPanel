package panel

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kejilion/kejilion-panel/internal/notification"
)

const clusterNotificationsPath = "/api/v1/cluster/notifications"

func (s *Server) handleClusterNotifications(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawPath == "" && r.URL.RawQuery == "" && ((r.Method == http.MethodPut && r.URL.Path == clusterNotificationsPath+"/channels") || (r.Method == http.MethodPost && (r.URL.Path == clusterNotificationsPath+"/channels/test" || r.URL.Path == clusterNotificationsPath+"/channels/discover"))) {
		s.handleNotificationChannel(w, r)
		return
	}
	if r.URL.Path == clusterNotificationsPath+"/history" && r.Method == http.MethodGet && r.URL.RawPath == "" {
		s.handleNotificationHistory(w, r)
		return
	}
	if r.URL.RawPath != "" || r.URL.RawQuery != "" {
		s.writeProblem(w, r, http.StatusBadRequest, "invalid_cluster_notification_request", "Invalid cluster notification request", "")
		return
	}
	switch {
	case r.URL.Path == clusterNotificationsPath && r.Method == http.MethodGet:
		if _, _, ok := s.requireSession(w, r); !ok {
			return
		}
		if s.notifications == nil {
			s.writeProblem(w, r, http.StatusServiceUnavailable, "cluster_notifications_unavailable", "Cluster notifications unavailable", "")
			return
		}
		s.writeJSON(w, http.StatusOK, s.notifications.SnapshotForSession(r.Context()))
	case r.URL.Path == clusterNotificationsPath && r.Method == http.MethodPut:
		s.handleClusterNotificationsUpdate(w, r)
	case r.URL.Path == clusterNotificationsPath+"/discover" && r.Method == http.MethodPost:
		s.handleClusterNotificationsDiscover(w, r)
	case r.URL.Path == clusterNotificationsPath+"/test" && r.Method == http.MethodPost:
		s.handleClusterNotificationsTest(w, r)
	default:
		s.writeProblem(w, r, http.StatusNotFound, "route_not_found", "Route not found", "")
	}
}

func (s *Server) handleNotificationHistory(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireSession(w, r); !ok {
		return
	}
	query, err := parseNotificationHistoryQuery(r.URL.RawQuery)
	if err != nil {
		s.writeProblem(w, r, http.StatusBadRequest, "invalid_notification_history_query", "Invalid notification history filters", "")
		return
	}
	if s.notifications == nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "cluster_notifications_unavailable", "Cluster notifications unavailable", "")
		return
	}
	page, err := s.notifications.History(query)
	if err != nil {
		s.writeNotificationError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, page)
}

func parseNotificationHistoryQuery(raw string) (notification.HistoryQuery, error) {
	query := notification.HistoryQuery{}
	if len(raw) > 4096 {
		return query, errors.New("query too long")
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return query, err
	}
	for key, value := range values {
		if len(value) != 1 {
			return query, errors.New("duplicate query parameter")
		}
		switch key {
		case "host":
			query.HostID = value[0]
		case "rule":
			query.Rule = value[0]
		case "kind":
			query.Kind = value[0]
		case "delivery":
			query.Delivery = value[0]
		case "search":
			query.Search = value[0]
		case "since":
			query.Since, err = time.Parse(time.RFC3339, value[0])
		case "until":
			query.Until, err = time.Parse(time.RFC3339, value[0])
		case "cursor":
			query.Before, err = strconv.ParseUint(value[0], 10, 64)
			if query.Before == 0 {
				err = errors.New("invalid cursor")
			}
		case "limit":
			query.Limit, err = strconv.Atoi(value[0])
			if query.Limit < 1 {
				err = errors.New("invalid limit")
			}
		default:
			return query, errors.New("unknown query parameter")
		}
		if err != nil {
			return query, err
		}
	}
	return query, query.Validate()
}

func (s *Server) handleClusterNotificationsUpdate(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireClusterMutation(w, r)
	if !ok {
		return
	}
	var input notification.UpdateInput
	if err := s.decodeJSON(w, r, &input); err != nil {
		return
	}
	change := map[string]any{
		"enabled": input.Enabled, "rules": input.Rules, "provider": auditNotificationProvider(input.Provider),
		"credentialProvided": strings.TrimSpace(input.ChannelCredential) != "" || strings.TrimSpace(input.TelegramBotToken) != "",
	}
	target := auditNotificationProvider(input.Provider)
	if err := s.audit(r, session.User.ID, "cluster.notifications.update", "cluster-notifications", target, "intent", change); err != nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "audit_unavailable", "Audit storage unavailable", "")
		return
	}
	if s.notifications == nil {
		_ = s.audit(r, session.User.ID, "cluster.notifications.update", "cluster-notifications", target, "failure", change)
		s.writeProblem(w, r, http.StatusServiceUnavailable, "cluster_notifications_unavailable", "Cluster notifications unavailable", "")
		return
	}
	snapshot, err := s.notifications.Configure(r.Context(), input)
	if err != nil {
		_ = s.audit(r, session.User.ID, "cluster.notifications.update", "cluster-notifications", target, "failure", change)
		s.writeNotificationError(w, r, err)
		return
	}
	_ = s.audit(r, session.User.ID, "cluster.notifications.update", "cluster-notifications", target, "success", change)
	s.writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleClusterNotificationsDiscover(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireClusterMutation(w, r)
	if !ok {
		return
	}
	var input struct {
		ExpectedResourceVersion string `json:"expectedResourceVersion"`
	}
	if err := s.decodeJSON(w, r, &input); err != nil {
		return
	}
	change := map[string]any{"expectedResourceVersion": input.ExpectedResourceVersion}
	if err := s.audit(r, session.User.ID, "cluster.notifications.discover", "cluster-notifications", "telegram", "intent", change); err != nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "audit_unavailable", "Audit storage unavailable", "")
		return
	}
	if s.notifications == nil {
		_ = s.audit(r, session.User.ID, "cluster.notifications.discover", "cluster-notifications", "telegram", "failure", change)
		s.writeProblem(w, r, http.StatusServiceUnavailable, "cluster_notifications_unavailable", "Cluster notifications unavailable", "")
		return
	}
	snapshot, err := s.notifications.Discover(r.Context(), input.ExpectedResourceVersion)
	if err != nil {
		_ = s.audit(r, session.User.ID, "cluster.notifications.discover", "cluster-notifications", "telegram", "failure", change)
		s.writeNotificationError(w, r, err)
		return
	}
	_ = s.audit(r, session.User.ID, "cluster.notifications.discover", "cluster-notifications", "telegram", "success", change)
	s.writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleClusterNotificationsTest(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireClusterMutation(w, r)
	if !ok {
		return
	}
	var input struct{}
	if err := s.decodeJSON(w, r, &input); err != nil {
		return
	}
	if err := s.audit(r, session.User.ID, "cluster.notifications.test", "cluster-notifications", "channel", "intent", nil); err != nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "audit_unavailable", "Audit storage unavailable", "")
		return
	}
	if s.notifications == nil {
		_ = s.audit(r, session.User.ID, "cluster.notifications.test", "cluster-notifications", "channel", "failure", nil)
		s.writeProblem(w, r, http.StatusServiceUnavailable, "cluster_notifications_unavailable", "Cluster notifications unavailable", "")
		return
	}
	snapshot, err := s.notifications.Test(r.Context())
	if err != nil {
		_ = s.audit(r, session.User.ID, "cluster.notifications.test", "cluster-notifications", "channel", "failure", nil)
		s.writeNotificationError(w, r, err)
		return
	}
	_ = s.audit(r, session.User.ID, "cluster.notifications.test", "cluster-notifications", "channel", "success", nil)
	s.writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) writeNotificationError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusUnprocessableEntity
	code := "cluster_notifications_failed"
	title := "Cluster notifications failed"
	var validation *notification.ValidationError
	if errors.As(err, &validation) {
		s.writeValidationProblem(w, r, validation.Field, validation.Message)
		return
	}
	switch {
	case errors.Is(err, notification.ErrConflict):
		status, code, title = http.StatusConflict, "cluster_notifications_changed", "Cluster notification settings changed"
	case errors.Is(err, notification.ErrTokenRequired):
		code, title = "cluster_notifications_token_required", "Telegram bot token is required"
	case errors.Is(err, notification.ErrCredentialRequired):
		code, title = "cluster_notifications_credential_required", "Notification channel credential is required"
	case errors.Is(err, notification.ErrNotConfigured):
		code, title = "cluster_notifications_not_configured", "Telegram notifications are not configured"
	case errors.Is(err, notification.ErrChannelNotConfigured):
		code, title = "cluster_notifications_channel_not_configured", "Notification channel is not configured"
	case errors.Is(err, notification.ErrNotReady):
		code, title = "cluster_notifications_not_ready", "Telegram notifications are not ready"
	case errors.Is(err, notification.ErrChannelNotReady):
		code, title = "cluster_notifications_channel_not_ready", "Notification channel is not ready"
	case errors.Is(err, notification.ErrProviderMismatch):
		code, title = "cluster_notifications_provider_mismatch", "Notification credential does not match the selected channel"
	case errors.Is(err, notification.ErrUnsupportedProvider):
		code, title = "cluster_notifications_unsupported_provider", "Notification channel is unsupported"
	case errors.Is(err, notification.ErrDiscoverUnsupported):
		code, title = "cluster_notifications_discover_unsupported", "Notification channel does not support chat discovery"
	case errors.Is(err, notification.ErrInvalidCredential):
		code, title = "cluster_notifications_invalid_credential", "Notification channel credential is invalid"
	case errors.Is(err, notification.ErrChatNotFound):
		code, title = "cluster_notifications_chat_not_found", "Telegram private chat was not found"
	case errors.Is(err, notification.ErrTelegramInvalidToken):
		code, title = "cluster_notifications_invalid_token", "Telegram bot token is invalid"
	case errors.Is(err, notification.ErrTelegramWebhookActive):
		code, title = "cluster_notifications_webhook_active", "Telegram webhook is active"
	case errors.Is(err, notification.ErrTelegramUnavailable):
		status, code, title = http.StatusBadGateway, "cluster_notifications_telegram_unavailable", "Telegram Bot API is unavailable"
	case errors.Is(err, notification.ErrChannelUnavailable):
		status, code, title = http.StatusBadGateway, "cluster_notifications_channel_unavailable", "Notification channel is unavailable"
	default:
		var typed *notification.Error
		if errors.As(err, &typed) {
			switch typed.Code {
			case "credential_store_unavailable_after_delivery", "credential_rollback_failed_after_delivery",
				"state_store_unavailable_after_delivery":
				status, code, title = http.StatusServiceUnavailable, "cluster_notifications_save_failed_after_delivery", "Validation message delivered but notification settings were not saved"
			case "history_store_unavailable", "credential_store_unavailable", "credential_rollback_failed", "state_store_unavailable", "credential_file_unavailable",
				"token_store_unavailable", "token_file_unavailable":
				status, code, title = http.StatusServiceUnavailable, "cluster_notifications_unavailable", "Cluster notifications unavailable"
			case "invalid_response", "api_error", "rate_limited", "unavailable":
				status, code, title = http.StatusBadGateway, "cluster_notifications_channel_unavailable", "Notification channel is unavailable"
			}
		}
	}
	s.writeProblem(w, r, status, code, title, "")
}

func auditNotificationProvider(provider notification.Provider) string {
	switch provider {
	case notification.ProviderTelegram, notification.ProviderFeishu, notification.ProviderDingTalk, notification.ProviderWeCom, notification.ProviderSMTP, notification.ProviderBark, notification.ProviderNtfy, notification.ProviderGotify, notification.ProviderDiscord, notification.ProviderSlack:
		return string(provider)
	case "":
		return "unchanged"
	default:
		return "unknown"
	}
}
