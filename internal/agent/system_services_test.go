package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/systemmanage"
)

func systemServiceAgentRequest(server *Server, method, path, body string, authorized bool) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if authorized {
		request.Header.Set("Authorization", "Bearer "+strings.Repeat("x", 32))
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func TestSystemServiceRoutesRejectInvalidTargetsAndUnknownFields(t *testing.T) {
	server := testServer(t)
	for _, test := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodGet, "/v1/system/services?all=true", "", http.StatusUnprocessableEntity},
		{http.MethodGet, "/v1/system/services/*.service", "", http.StatusUnprocessableEntity},
		{http.MethodGet, "/v1/system/services/a%2Fssh.service", "", http.StatusUnprocessableEntity},
		{http.MethodGet, "/v1/system/services/ssh.service/logs?lines=99999", "", http.StatusUnprocessableEntity},
		{http.MethodPost, "/v1/system/services", "", http.StatusMethodNotAllowed},
		{http.MethodPost, "/v1/system/service-actions", `{"name":"ssh.service","action":"restart","expectedResourceVersion":"` + strings.Repeat("a", 64) + `","command":"reboot"}`, http.StatusBadRequest},
		{http.MethodPost, "/v1/system/service-actions", `{"name":"--root=/tmp.service","action":"restart","expectedResourceVersion":"` + strings.Repeat("a", 64) + `"}`, http.StatusUnprocessableEntity},
		{http.MethodPost, "/v1/system/service-actions?now=true", `{}`, http.StatusBadRequest},
	} {
		response := systemServiceAgentRequest(server, test.method, test.path, test.body, true)
		if response.Code != test.status {
			t.Errorf("%s %s = %d want %d: %s", test.method, test.path, response.Code, test.status, response.Body.String())
		}
	}
	response := systemServiceAgentRequest(server, http.MethodGet, "/v1/system/services", "", false)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", response.Code)
	}
}

func TestSystemServiceRoutesReportUnsupportedAndWriteDisabled(t *testing.T) {
	server := testServer(t)
	server.systemManager = systemmanage.NewManager(systemmanage.Config{RunRoot: t.TempDir(), Enabled: false, EffectiveUID: func() int { return 0 }})
	response := systemServiceAgentRequest(server, http.MethodGet, "/v1/system/services", "", true)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"supported":false`) || !strings.Contains(response.Body.String(), `"reason":`) {
		t.Fatalf("unsupported=%d %s", response.Code, response.Body.String())
	}
	response = systemServiceAgentRequest(server, http.MethodPost, "/v1/system/service-actions", `{"name":"ssh.service","action":"stop","expectedResourceVersion":"`+strings.Repeat("a", 64)+`"}`, true)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "system_services_write_disabled") {
		t.Fatalf("disabled=%d %s", response.Code, response.Body.String())
	}
	server.serviceReadGate <- struct{}{}
	server.serviceReadGate <- struct{}{}
	response = systemServiceAgentRequest(server, http.MethodGet, "/v1/system/services", "", true)
	<-server.serviceReadGate
	<-server.serviceReadGate
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("concurrency limit status=%d", response.Code)
	}
}

func TestSystemServiceErrorStatusPreservesTimeoutConflictAndBusy(t *testing.T) {
	server := testServer(t)
	for _, test := range []struct {
		err    error
		status int
	}{
		{context.DeadlineExceeded, http.StatusGatewayTimeout},
		{systemmanage.ErrConflict, http.StatusConflict},
		{systemmanage.ErrSystemServicesBusy, http.StatusTooManyRequests},
		{systemmanage.ErrNeedsAttention, http.StatusServiceUnavailable},
	} {
		response := httptest.NewRecorder()
		server.writeSystemServiceError(response, "test", test.err)
		if response.Code != test.status {
			t.Errorf("error %v status=%d want=%d", test.err, response.Code, test.status)
		}
	}
}
