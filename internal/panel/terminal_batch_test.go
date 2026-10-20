package panel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/terminal"
	"github.com/kejilion/kejilion-panel/internal/terminalbatch"
)

func TestBatchAPIRequiresSessionOriginCSRFAndValidTargets(t *testing.T) {
	server, tokenPath := newTestServer(t)
	path := "/api/v1/terminal-batches"
	if response := performRequest(server, http.MethodGet, path, nil, nil); response.Code != 401 {
		t.Fatal(response.Code)
	}
	cookie, csrf := bootstrapCookies(t, server, tokenPath)
	body := []byte(`{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","command":"true","hostIds":["local"]}`)
	headers := map[string]string{"Content-Type": "application/json", "Origin": "http://panel.test", "X-CSRF-Token": csrf.Value}
	for _, missing := range []string{"Origin", "X-CSRF-Token"} {
		h := map[string]string{}
		for k, v := range headers {
			if k != missing {
				h[k] = v
			}
		}
		response := authenticatedRequest(server, http.MethodPost, path, body, cookie, csrf, h)
		if response.Code != 403 {
			t.Fatalf("missing %s: %d", missing, response.Code)
		}
	}
	invalid := authenticatedRequest(server, http.MethodPost, path, []byte(`{"id":"bad","command":"true","hostIds":["local"]}`), cookie, csrf, headers)
	if invalid.Code != 400 {
		t.Fatalf("invalid ID: %d %s", invalid.Code, invalid.Body.String())
	}
	server.agent = &terminalAgentStub{}
	created := authenticatedRequest(server, http.MethodPost, path, body, cookie, csrf, headers)
	if created.Code != 202 {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var j terminalbatch.Job
	if err := json.Unmarshal(created.Body.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	listed := authenticatedRequest(server, http.MethodGet, path, nil, cookie, csrf, nil)
	if listed.Code != 200 || !strings.Contains(listed.Body.String(), j.ID) || strings.Contains(listed.Body.String(), `"owner"`) {
		t.Fatalf("list: %s", listed.Body.String())
	}
	stop := authenticatedRequest(server, http.MethodPost, path+"/"+j.ID+"/stop", []byte(`{}`), cookie, csrf, headers)
	if stop.Code != 200 {
		t.Fatalf("stop: %d", stop.Code)
	}
}

type batchCloseAgent struct {
	terminalAgentStub
	fail bool
}

func (a *batchCloseAgent) Do(ctx context.Context, method, path, query, request string, body []byte) (AgentResponse, error) {
	if strings.HasSuffix(path, "/close") && a.fail {
		return AgentResponse{StatusCode: 503, Body: []byte(`{}`)}, nil
	}
	return a.terminalAgentStub.Do(ctx, method, path, query, request, body)
}

func TestBatchDriverSharesQuotaAndRetainsFailedCloseIdentity(t *testing.T) {
	s, _ := newTestServer(t)
	a := &batchCloseAgent{fail: true}
	s.agent = a
	d := batchTerminalDriver{s}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		s.terminalSessions[string(rune('a'+i))] = panelTerminalSession{ID: string(rune('a' + i)), UserID: "user", Purpose: "test"}
	}
	session, err := d.Open(ctx, "user", terminalbatch.Host{ID: "local", Shell: "posix"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Open(ctx, "user", terminalbatch.Host{ID: "local", Shell: "posix"}); !errors.Is(err, terminal.ErrLimit) {
		t.Fatal("retained sessions ignored")
	}
	if err = d.Close(ctx, session); err == nil {
		t.Fatal("failed close treated as success")
	}
	if _, ok := s.terminalSessions[session.ID]; !ok {
		t.Fatal("failed-close identity removed")
	}
	if _, err = d.Open(ctx, "user", terminalbatch.Host{ID: "local", Shell: "posix"}); !errors.Is(err, terminal.ErrLimit) {
		t.Fatal("failed close released quota")
	}
	a.fail = false
	if err = d.Close(ctx, session); err != nil {
		t.Fatal(err)
	}
	s.terminalMu.Lock()
	_, exists := s.terminalSessions[session.ID]
	s.terminalMu.Unlock()
	if exists {
		t.Fatal("confirmed close kept quota")
	}
	// Avoid asking the Agent to close test-only identities during teardown.
	s.terminalMu.Lock()
	clear(s.terminalSessions)
	s.terminalMu.Unlock()
}
