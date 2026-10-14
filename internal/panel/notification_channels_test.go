package panel

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/notification"
)

func TestNotificationChannelRoutesRequireSessionCSRFAndVersion(t *testing.T) {
	server, tokenPath := newTestServer(t)
	for _, path := range []string{"/channels", "/channels/test", "/channels/discover"} {
		method := http.MethodPost
		if path == "/channels" {
			method = http.MethodPut
		}
		if res := performRequest(server, method, clusterNotificationsPath+path, []byte(`{}`), nil); res.Code != http.StatusUnauthorized && res.Code != http.StatusForbidden {
			t.Fatalf("unauthenticated %s: %d", path, res.Code)
		}
	}
	session, csrf := bootstrapCookies(t, server, tokenPath)
	snapshot := server.notifications.Snapshot()
	body, _ := json.Marshal(notification.ChannelUpdateInput{Provider: notification.ProviderNtfy, Remove: true, ExpectedResourceVersion: snapshot.ResourceVersion})
	res := authenticatedRequest(server, http.MethodPut, clusterNotificationsPath+"/channels", body, session, csrf, map[string]string{"Origin": "http://panel.test", "Content-Type": "application/json"})
	if res.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF: %d", res.Code)
	}
	headers := map[string]string{"Origin": "http://panel.test", "Content-Type": "application/json", "X-CSRF-Token": csrf.Value}
	res = authenticatedRequest(server, http.MethodPut, clusterNotificationsPath+"/channels", body, session, csrf, headers)
	if res.Code != http.StatusOK {
		t.Fatalf("channel update: %d %s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "credential") || strings.Contains(res.Body.String(), "fingerprint") {
		t.Fatal("snapshot contains protected data")
	}
	res = authenticatedRequest(server, http.MethodPut, clusterNotificationsPath+"/channels", body, session, csrf, headers)
	if res.Code != http.StatusConflict {
		t.Fatalf("stale update: %d %s", res.Code, res.Body.String())
	}
}
