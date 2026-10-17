package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/backup"
	"github.com/kejilion/kejilion-panel/internal/hostbackup"
)

// backupReviewAgent answers the host-backup calls a restore makes. A nil
// review stands for an Agent from before import reviews existed.
type backupReviewAgent struct {
	stubAgent
	mu       sync.Mutex
	review   *hostbackup.ImportReview
	restores [][]byte
}

func (a *backupReviewAgent) Do(_ context.Context, method, path, _, _ string, body []byte) (AgentResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	reply := func(status int, value any) (AgentResponse, error) {
		data, _ := json.Marshal(value)
		return AgentResponse{StatusCode: status, ContentType: "application/json", Body: data}, nil
	}
	switch {
	case method == "GET" && strings.HasSuffix(path, "/review"):
		if a.review == nil {
			return reply(400, map[string]string{"code": "backup_unavailable"})
		}
		return reply(200, a.review)
	case method == "POST" && path == "/v1/backups":
		a.restores = append(a.restores, append([]byte(nil), body...))
		return reply(202, backup.Record{ID: strings.Repeat("b", 32), Action: "restore", Status: "running", Modules: []string{"apps"}})
	case method == "GET" && path == "/v1/backups/"+strings.Repeat("b", 32):
		return reply(200, backup.Record{ID: strings.Repeat("b", 32), Action: "restore", Status: "completed", Modules: []string{"apps"}, CompletedModules: []string{"apps"}})
	}
	return reply(400, map[string]string{"code": "backup_unavailable"})
}

func (a *backupReviewAgent) restoreBodies() [][]byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([][]byte(nil), a.restores...)
}

func TestBackupRestoreConfirmsForeignHostConfiguration(t *testing.T) {
	foreign := &hostbackup.ImportReview{Version: 1, Modules: []hostbackup.ModuleReview{{Module: "apps", Origin: hostbackup.OriginForeign, Containers: []hostbackup.ContainerReview{{Name: "app", Image: "example/app", Privileged: true}}}}}
	local := &hostbackup.ImportReview{Version: 1, Modules: []hostbackup.ModuleReview{{Module: "apps", Origin: hostbackup.OriginLocal}}}
	for _, item := range []struct {
		name           string
		review         *hostbackup.ImportReview
		needsConfirm   bool
		forwardConfirm bool
	}{
		{"foreign", foreign, true, true},
		{"local", local, false, false},
		{"agent without reviews", nil, true, false},
		{"legacy import without details", &hostbackup.ImportReview{Version: 1, Modules: []hostbackup.ModuleReview{{Module: "apps", Origin: hostbackup.OriginForeign, Unreviewed: true}}}, true, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			s, token := newTestServer(t)
			session, csrf := bootstrapCookies(t, s, token)
			agent := &backupReviewAgent{review: item.review}
			s.agent = agent
			source, err := s.backups.Reserve("import", []string{"apps"})
			if err != nil {
				t.Fatal(err)
			}
			if err := s.backups.Run(source.ID, func(context.Context, string) error {
				return s.backups.Update(source.ID, func(r *backup.Record) {
					r.AgentID = strings.Repeat("a", 32)
					r.TargetRevision = "host-only"
					r.AgentRevision = "agent-revision"
				})
			}); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if r, _ := s.backups.Get(source.ID); r.Status == "ready" {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			call := func(method, path string, body []byte) *httptest.ResponseRecorder {
				r := httptest.NewRequest(method, path, bytes.NewReader(body))
				r.Host = "panel.test"
				r.AddCookie(session)
				r.AddCookie(csrf)
				r.Header.Set("Origin", "http://panel.test")
				r.Header.Set("X-CSRF-Token", csrf.Value)
				r.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				s.ServeHTTP(w, r)
				return w
			}
			w := call("GET", "/api/v1/backups/"+source.ID+"/review", nil)
			var review backupReviewResult
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &review) != nil || review.Available != (item.review != nil) {
				t.Fatal(w.Code, w.Body.String())
			}
			restore := func(confirm bool) *httptest.ResponseRecorder {
				body, _ := json.Marshal(backupRequest{Modules: []string{"apps"}, Revision: "host-only", ConfirmForeign: confirm})
				return call("POST", "/api/v1/backups/"+source.ID+"/restore", body)
			}
			if item.review == nil || !item.review.Complete([]string{"apps"}) {
				for _, confirm := range []bool{false, true} {
					w = restore(confirm)
					if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "backup_review_unavailable") {
						t.Fatal("missing review accepted:", w.Code, w.Body.String())
					}
				}
				if len(agent.restoreBodies()) != 0 {
					t.Fatal("restore without configuration details reached the Agent")
				}
				return
			}
			if item.needsConfirm {
				w = restore(false)
				if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "backup_foreign_confirmation_required") {
					t.Fatal("restore without confirmation accepted:", w.Code, w.Body.String())
				}
				if len(agent.restoreBodies()) != 0 {
					t.Fatal("unconfirmed restore reached the Agent")
				}
			}
			w = restore(item.needsConfirm)
			var job backup.Record
			if w.Code != http.StatusAccepted || json.Unmarshal(w.Body.Bytes(), &job) != nil {
				t.Fatal(w.Code, w.Body.String())
			}
			deadline = time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) && len(agent.restoreBodies()) == 0 {
				time.Sleep(10 * time.Millisecond)
			}
			bodies := agent.restoreBodies()
			if len(bodies) != 1 {
				t.Fatal("restore did not reach the Agent")
			}
			var sent hostbackup.Request
			if err := backup.Decode(bodies[0], &sent); err != nil || sent.ConfirmForeign != item.forwardConfirm {
				t.Fatalf("Agent restore request = %s", bodies[0])
			}
			if !item.forwardConfirm && strings.Contains(string(bodies[0]), "confirmForeign") {
				t.Fatalf("Agent received a field it may not know: %s", bodies[0])
			}
		})
	}
}
