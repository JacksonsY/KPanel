package panel

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/ai"
	"github.com/kejilion/kejilion-panel/internal/auth"
)

type aiSnapshotRecorder struct {
	*httptest.ResponseRecorder
	cancel context.CancelFunc
}

type aiDeadlineRecorder struct {
	*httptest.ResponseRecorder
	deadlines int
}

func (w *aiDeadlineRecorder) SetWriteDeadline(time.Time) error {
	w.deadlines++
	return nil
}

func (w *aiSnapshotRecorder) Flush() {
	w.ResponseRecorder.Flush()
	w.cancel()
}

func TestAIHistoryAndStreamAttachmentMetadata(t *testing.T) {
	server, tokenPath := newTestServer(t)
	sessionCookie, _ := bootstrapCookies(t, server, tokenPath)
	authSession, err := server.auth.Authenticate(sessionCookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.EnableAI(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	session, err := server.ai.Store.CreateSession(ctx, ai.Session{UserID: authSession.User.ID, ProviderID: "provider", ModelID: "model"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := server.ai.Store.CreateRun(ctx, ai.Run{SessionID: session.ID, UserID: authSession.User.ID, ProviderID: "provider", ModelID: "model"})
	if err != nil {
		t.Fatal(err)
	}
	message, err := server.ai.Store.AddMessage(ctx, ai.Message{SessionID: session.ID, RunID: run.ID, Role: ai.RoleUser, Content: "read this",
		Attachments: []ai.Attachment{{Name: "note.txt", MimeType: "text/plain", Kind: "text", Data: []byte("hello")}}})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.aiMessages(response, httptest.NewRequest(http.MethodGet, "/", nil), authSession.User.ID, session.ID)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"size":5`) || strings.Contains(response.Body.String(), "aGVsbG8=") {
		t.Fatalf("history=%d %s", response.Code, response.Body.String())
	}
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream := &aiSnapshotRecorder{ResponseRecorder: httptest.NewRecorder(), cancel: cancel}
	request := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(streamCtx)
	request.AddCookie(sessionCookie)
	server.aiRunEvents(stream, request, authSession, run.ID)
	if stream.Code != http.StatusOK || !strings.Contains(stream.Body.String(), "event: run.snapshot") || !strings.Contains(stream.Body.String(), `"size":5`) || strings.Contains(stream.Body.String(), "aGVsbG8=") {
		t.Fatalf("stream=%d %s", stream.Code, stream.Body.String())
	}
	db, err := sql.Open("sqlite", filepath.Join(server.config.DataDir, "ai.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "UPDATE messages SET attachments_json=? WHERE id=?", []byte("{"), message.ID); err != nil {
		t.Fatal(err)
	}
	for _, streamRequest := range []bool{false, true} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.AddCookie(sessionCookie)
		if streamRequest {
			server.aiRunEvents(response, request, authSession, run.ID)
		} else {
			server.aiMessages(response, request, authSession.User.ID, session.ID)
		}
		if response.Code != http.StatusUnprocessableEntity || strings.Contains(response.Body.String(), "run.snapshot") || !strings.Contains(response.Body.String(), "ai_request_failed") {
			t.Fatalf("corrupt stream=%v: %d %s", streamRequest, response.Code, response.Body.String())
		}
	}
}

type aiStreamFixture struct {
	server  *Server
	cookie  *http.Cookie
	csrf    *http.Cookie
	session auth.Session
	run     ai.Run
}

func newAIStreamFixture(t *testing.T) aiStreamFixture {
	t.Helper()
	server, tokenPath := newTestServer(t)
	if err := server.EnableAI(); err != nil {
		t.Fatal(err)
	}
	cookie, csrf := bootstrapCookies(t, server, tokenPath)
	session, err := server.auth.Authenticate(cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := server.ai.Store.CreateSession(context.Background(), ai.Session{UserID: session.User.ID, ProviderID: "provider", ModelID: "model"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := server.ai.Store.CreateRun(context.Background(), ai.Run{SessionID: conversation.ID, UserID: session.User.ID, ProviderID: "provider", ModelID: "model"})
	if err != nil {
		t.Fatal(err)
	}
	return aiStreamFixture{server: server, cookie: cookie, csrf: csrf, session: session, run: run}
}

func TestAIStreamRejectsOversizedToolStateSnapshot(t *testing.T) {
	fixture := newAIStreamFixture(t)
	native := json.RawMessage(`{"text":"` + strings.Repeat("x", 900<<10) + `"}`)
	for i := 0; i < 6; i++ {
		_, err := fixture.server.ai.Store.SaveToolCall(context.Background(), ai.ToolCall{ID: string(rune('a' + i)), RunID: fixture.run.ID, SessionID: fixture.run.SessionID, Name: "host_read", Arguments: json.RawMessage(`{}`), ProviderData: native, Status: ai.ToolCompleted})
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	response := &aiSnapshotRecorder{ResponseRecorder: httptest.NewRecorder(), cancel: cancel}
	request := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	request.AddCookie(fixture.cookie)
	fixture.server.aiRunEvents(response, request, fixture.session, fixture.run.ID)
	if response.Code != http.StatusUnprocessableEntity || strings.Contains(response.Body.String(), "run.snapshot") || strings.Contains(response.Header().Get("Content-Type"), "event-stream") {
		t.Fatalf("oversized snapshot did not fail before streaming: %d %s", response.Code, response.Body.String())
	}
}

func openAIHTTPStream(t *testing.T, fixture aiStreamFixture) (<-chan string, <-chan struct{}, context.CancelFunc) {
	t.Helper()
	handlerDone := make(chan struct{})
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		fixture.server.ServeHTTP(w, r)
	}))
	t.Cleanup(httpServer.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, httpServer.URL+"/api/v1/ai/runs/"+fixture.run.ID+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = "panel.test"
	request.AddCookie(fixture.cookie)
	response, err := httpServer.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/event-stream" {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("stream status=%d body=%s", response.StatusCode, body)
	}
	lines := make(chan string, 32)
	go func() {
		defer close(lines)
		reader := bufio.NewScanner(response.Body)
		for reader.Scan() {
			select {
			case lines <- reader.Text():
			case <-ctx.Done():
				return
			}
		}
	}()
	return lines, handlerDone, cancel
}

func waitAIStreamReady(t *testing.T, fixture aiStreamFixture, lines <-chan string) {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	snapshot := false
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("stream closed before normal output")
			}
			snapshot = snapshot || line == "event: run.snapshot"
			if strings.Contains(line, "normal-output-marker") {
				if !snapshot {
					t.Fatal("normal output preceded snapshot")
				}
				return
			}
		case <-tick.C:
			fixture.server.ai.Events.Publish(ai.RunEvent{RunID: fixture.run.ID, Type: "message.delta", Data: "normal-output-marker"})
		case <-deadline.C:
			t.Fatal("normal stream output did not arrive")
		}
	}
}

func assertAIStreamClosed(t *testing.T, lines <-chan string, handlerDone <-chan struct{}, timeout time.Duration) {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				select {
				case <-handlerDone:
					return
				case <-deadline.C:
					t.Fatal("stream body closed but handler did not exit")
				}
			}
			if strings.Contains(line, "revoked-output-marker") || strings.HasPrefix(line, ": heartbeat") {
				t.Fatalf("revoked stream received output: %s", line)
			}
		case <-deadline.C:
			t.Fatal("stream did not close within its bound")
		}
	}
}

func TestAIStreamStopsAfterSessionRevocation(t *testing.T) {
	for _, action := range []string{"logout", "password", "transport"} {
		t.Run(action, func(t *testing.T) {
			fixture := newAIStreamFixture(t)
			lines, handlerDone, _ := openAIHTTPStream(t, fixture)
			waitAIStreamReady(t, fixture, lines)
			headers := map[string]string{"Origin": "http://panel.test", "X-CSRF-Token": fixture.csrf.Value, "Content-Type": "application/json"}
			switch action {
			case "logout":
				response := authenticatedRequest(fixture.server, http.MethodPost, "/api/v1/auth/logout", nil, fixture.cookie, fixture.csrf, headers)
				if response.Code != http.StatusOK {
					t.Fatalf("logout=%d %s", response.Code, response.Body.String())
				}
			case "password":
				response := authenticatedRequest(fixture.server, http.MethodPut, "/api/v1/settings/password", []byte(`{"currentPassword":"a-strong-password-1","newPassword":"another-strong-password-2"}`), fixture.cookie, fixture.csrf, headers)
				if response.Code != http.StatusOK {
					t.Fatalf("password change=%d %s", response.Code, response.Body.String())
				}
			case "transport":
				if err := fixture.server.auth.ChangeTransportBoundary(func() error { return nil }); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := fixture.server.auth.Authenticate(fixture.cookie.Value); !errors.Is(err, auth.ErrInvalidSession) {
				t.Fatalf("session still valid after %s: %v", action, err)
			}
			fixture.server.ai.Events.Publish(ai.RunEvent{RunID: fixture.run.ID, Type: "message.delta", Data: "revoked-output-marker"})
			fixture.server.ai.Events.Publish(ai.RunEvent{RunID: fixture.run.ID, Type: "tool.updated", Data: "revoked-output-marker"})
			assertAIStreamClosed(t, lines, handlerDone, 2*time.Second)
		})
	}
}

func TestAIStreamIdleRevocationStopsAtHeartbeat(t *testing.T) {
	fixture := newAIStreamFixture(t)
	lines, handlerDone, _ := openAIHTTPStream(t, fixture)
	if err := fixture.server.auth.Logout(fixture.cookie.Value); err != nil {
		t.Fatal(err)
	}
	assertAIStreamClosed(t, lines, handlerDone, 18*time.Second)
}

func TestAIStreamContextCancelExits(t *testing.T) {
	fixture := newAIStreamFixture(t)
	lines, handlerDone, cancel := openAIHTTPStream(t, fixture)
	waitAIStreamReady(t, fixture, lines)
	cancel()
	assertAIStreamClosed(t, lines, handlerDone, 2*time.Second)
}

func TestAIStreamSnapshotRequiresOriginalLiveSession(t *testing.T) {
	for _, action := range []string{"revoked", "different-token"} {
		t.Run(action, func(t *testing.T) {
			fixture := newAIStreamFixture(t)
			cookie := fixture.cookie
			if action == "revoked" {
				if err := fixture.server.auth.Logout(cookie.Value); err != nil {
					t.Fatal(err)
				}
			} else {
				credentials, err := fixture.server.auth.Login("192.0.2.1", "admin", "a-strong-password-1", "")
				if err != nil {
					t.Fatal(err)
				}
				cookie = &http.Cookie{Name: cookie.Name, Value: credentials.Token}
			}
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.AddCookie(cookie)
			response := &aiDeadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
			fixture.server.aiRunEvents(response, request, fixture.session, fixture.run.ID)
			if response.Body.Len() != 0 || response.deadlines != 0 {
				t.Fatalf("invalid original session received snapshot/deadline: body=%s deadlines=%d", response.Body.String(), response.deadlines)
			}
		})
	}
}

func TestAIMessageAcceptsMultipartAttachmentsWithoutBase64JSON(t *testing.T) {
	server, _ := newTestServer(t)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("content", "分析图片"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("attachments", "screen.png")
	if err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	if _, err := part.Write(png); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ai/sessions/test/messages", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	content, attachments, err := server.decodeAIMessage(response, request)
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("multipart decode status=%d body=%s err=%v", response.Code, response.Body.String(), err)
	}
	if content != "分析图片" || len(attachments) != 1 || attachments[0].Name != "screen.png" || !bytes.Equal(attachments[0].Data, png) {
		t.Fatalf("content=%q attachments=%#v", content, attachments)
	}
}

func TestAIMessageRejectsOversizedMultipartAttachmentInsteadOfTruncating(t *testing.T) {
	server, _ := newTestServer(t)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("attachments", "large.png")
	if err != nil {
		t.Fatal(err)
	}
	oversized := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, make([]byte, maxAIAttachmentBytes)...)
	if _, err := part.Write(oversized); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ai/sessions/test/messages", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	_, attachments, err := server.decodeAIMessage(response, request)
	if !errors.Is(err, errAIAttachmentTooLarge) || response.Code != http.StatusRequestEntityTooLarge || attachments != nil {
		t.Fatalf("oversized attachment status=%d err=%v attachments=%d", response.Code, err, len(attachments))
	}
}

func TestAIProviderModelSessionCRUD(t *testing.T) {
	server, tokenPath := newTestServer(t)
	if err := server.EnableAI(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	sessionCookie, csrfCookie := bootstrapCookies(t, server, tokenPath)
	headers := map[string]string{"Content-Type": "application/json", "Origin": "http://panel.test", "X-CSRF-Token": csrfCookie.Value}
	create := authenticatedRequest(server, http.MethodPost, "/api/v1/ai/providers", []byte(`{"name":"Mock","protocol":"openai_compatible","baseUrl":"http://127.0.0.1:11434/v1","endpointScope":"private","privateConfirmed":true,"apiKey":"secret-1234","enabled":true}`), sessionCookie, csrfCookie, headers)
	if create.Code != http.StatusCreated {
		t.Fatalf("provider create=%d %s", create.Code, create.Body.String())
	}
	var provider ai.Provider
	if err := json.Unmarshal(create.Body.Bytes(), &provider); err != nil {
		t.Fatal(err)
	}
	if provider.ID == "" || !provider.APIKeySet || provider.APIKeyHint != "1234" {
		t.Fatalf("provider=%#v", provider)
	}
	if strings.Contains(create.Body.String(), "secret-1234") {
		t.Fatal("response exposed API key")
	}
	modelBody := []byte(`{"modelId":"mock-model","displayName":"Mock Model","contextWindow":8192,"toolCalling":true,"enabled":true}`)
	modelsResponse := authenticatedRequest(server, http.MethodPost, "/api/v1/ai/providers/"+provider.ID+"/models", modelBody, sessionCookie, csrfCookie, headers)
	if modelsResponse.Code != http.StatusCreated {
		t.Fatalf("model add=%d %s", modelsResponse.Code, modelsResponse.Body.String())
	}
	var models []ai.Model
	if err := json.Unmarshal(modelsResponse.Body.Bytes(), &models); err != nil || len(models) != 1 {
		t.Fatalf("models=%#v err=%v", models, err)
	}
	if !models[0].Vision {
		t.Fatalf("manually added model did not default to vision: %#v", models[0])
	}
	sessionResponse := authenticatedRequest(server, http.MethodPost, "/api/v1/ai/sessions", []byte(`{"providerId":"`+provider.ID+`","modelId":"`+models[0].ID+`"}`), sessionCookie, csrfCookie, headers)
	if sessionResponse.Code != http.StatusCreated {
		t.Fatalf("session create=%d %s", sessionResponse.Code, sessionResponse.Body.String())
	}
	var session ai.Session
	if err := json.Unmarshal(sessionResponse.Body.Bytes(), &session); err != nil || session.ApprovalMode != ai.ApprovalManual {
		t.Fatalf("session approval default=%#v err=%v", session, err)
	}
	modeResponse := authenticatedRequest(server, http.MethodPatch, "/api/v1/ai/sessions/"+session.ID, []byte(`{"approvalMode":"auto"}`), sessionCookie, csrfCookie, headers)
	if modeResponse.Code != http.StatusOK || json.Unmarshal(modeResponse.Body.Bytes(), &session) != nil || session.ApprovalMode != ai.ApprovalAuto {
		t.Fatalf("session approval update=%d %s", modeResponse.Code, modeResponse.Body.String())
	}
	list := authenticatedRequest(server, http.MethodGet, "/api/v1/ai/sessions", nil, sessionCookie, csrfCookie, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("session list=%d %s", list.Code, list.Body.String())
	}
	run, err := server.ai.Store.CreateRun(context.Background(), ai.Run{SessionID: session.ID, UserID: session.UserID, ProviderID: provider.ID, ProviderName: provider.Name, ModelID: models[0].ID, ModelName: models[0].DisplayName})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.ai.Store.SaveToolCall(context.Background(), ai.ToolCall{ID: "persisted-call", RunID: run.ID, SessionID: session.ID, Name: "host_system_summary", Arguments: json.RawMessage(`{}`), Status: ai.ToolCompleted}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.ai.Store.AddMessage(context.Background(), ai.Message{SessionID: session.ID, RunID: run.ID, Role: ai.RoleAssistant, Content: "healthy"}); err != nil {
		t.Fatal(err)
	}
	messagesResponse := authenticatedRequest(server, http.MethodGet, "/api/v1/ai/sessions/"+session.ID+"/messages", nil, sessionCookie, csrfCookie, nil)
	var messagePage struct {
		Items     []ai.Message  `json:"items"`
		ToolCalls []ai.ToolCall `json:"toolCalls"`
	}
	if messagesResponse.Code != http.StatusOK || json.Unmarshal(messagesResponse.Body.Bytes(), &messagePage) != nil || len(messagePage.Items) != 1 || len(messagePage.ToolCalls) != 1 || messagePage.ToolCalls[0].ID != "persisted-call" {
		t.Fatalf("message process history=%d %s", messagesResponse.Code, messagesResponse.Body.String())
	}
	deleteResponse := authenticatedRequest(server, http.MethodDelete, "/api/v1/ai/sessions/"+session.ID, nil, sessionCookie, csrfCookie, headers)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("active session delete=%d %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if _, err := server.ai.Store.Session(context.Background(), session.UserID, session.ID); !errors.Is(err, ai.ErrNotFound) {
		t.Fatalf("deleted active session lookup error=%v", err)
	}
}

func TestAIToolMapsAgentVersionConflictForReplanning(t *testing.T) {
	server, _ := newTestServer(t)
	server.agent = &stubAgent{response: AgentResponse{StatusCode: http.StatusConflict, ContentType: "application/json", Body: []byte(`{"code":"resource_version_conflict"}`)}}
	tools := &panelAITools{server: server}
	execution := ai.ToolExecutionContext{UserID: "admin", SessionID: "ses", RunID: "run", ToolCallID: "call"}
	_, err := tools.Execute(context.Background(), execution, "host_system_summary", json.RawMessage(`{}`))
	if !errors.Is(err, ai.ErrToolConflict) {
		t.Fatalf("Agent conflict was not typed for replanning: %v", err)
	}
}

func TestAIToolMapsSafeAgentBusinessRejectionForReplanning(t *testing.T) {
	server, _ := newTestServer(t)
	server.agent = &stubAgent{response: AgentResponse{StatusCode: http.StatusUnprocessableEntity, ContentType: "application/json", Body: []byte(`{"code":"file_symlink_rejected","detail":"sensitive server detail","requestId":"req-safe"}`)}}
	tools := &panelAITools{server: server}
	execution := ai.ToolExecutionContext{UserID: "admin", SessionID: "ses", RunID: "run", ToolCallID: "call"}
	_, err := tools.Execute(context.Background(), execution, "host_system_summary", json.RawMessage(`{}`))
	if !errors.Is(err, ai.ErrToolRejected) {
		t.Fatalf("Agent business rejection was not typed for replanning: %v", err)
	}
	var rejection *ai.ToolRejectedError
	if !errors.As(err, &rejection) || rejection.StatusCode != http.StatusUnprocessableEntity || rejection.Code != "file_symlink_rejected" || rejection.RequestID != "req-safe" {
		t.Fatalf("typed rejection=%#v err=%v", rejection, err)
	}
	if strings.Contains(err.Error(), "sensitive server detail") {
		t.Fatalf("Agent detail leaked through rejection: %v", err)
	}

	server.agent = &stubAgent{response: AgentResponse{StatusCode: http.StatusUnauthorized, ContentType: "application/json", Body: []byte(`{"code":"unauthorized"}`)}}
	_, err = tools.Execute(context.Background(), execution, "host_system_summary", json.RawMessage(`{}`))
	if errors.Is(err, ai.ErrToolRejected) {
		t.Fatalf("Agent authentication failure was treated as model-recoverable: %v", err)
	}
}

func TestAIToolUsesFixedAgentPathAndAudits(t *testing.T) {
	server, _ := newTestServer(t)
	agent := &stubAgent{response: AgentResponse{StatusCode: 200, ContentType: "application/json", Body: []byte(`{"ok":true}`)}}
	server.agent = agent
	tools := &panelAITools{server: server}
	execution := ai.ToolExecutionContext{UserID: "admin", SessionID: "ses-1", RunID: "run-1", ToolCallID: "call-1"}
	result, err := tools.Execute(context.Background(), execution, "host_system_summary", json.RawMessage(`{"reason":"inspect resources"}`))
	if err != nil || result != "{\"ok\":true}" {
		t.Fatalf("result=%q err=%v", result, err)
	}
	calls := agent.snapshotCalls()
	if len(calls) != 1 || calls[0].path != "/v1/system/summary" || calls[0].method != http.MethodGet {
		t.Fatalf("calls=%#v", calls)
	}
	if _, err := tools.Execute(context.Background(), execution, "host_system_summary", json.RawMessage(`{"unknown":true}`)); !errors.Is(err, ai.ErrToolArguments) || len(agent.snapshotCalls()) != 1 {
		t.Fatalf("unknown read arguments were not rejected before Agent: %v", err)
	}
	audits, _, _ := server.store.ListAudit(10, "")
	if len(audits) == 0 || audits[0].Change["sessionId"] != "ses-1" || audits[0].Change["runId"] != "run-1" || audits[0].Change["toolCallId"] != "call-1" {
		t.Fatalf("AI audit correlation missing: %#v", audits)
	}
	_, err = tools.Execute(context.Background(), execution, "host_docker_container_action", json.RawMessage(`{"containerId":"bad","action":"remove","resourceVersion":"bad"}`))
	if err == nil || len(agent.snapshotCalls()) != 1 {
		t.Fatal("invalid write reached Agent")
	}
}

func TestAIToolReasonMetadataIsStrippedBeforeNginxReloadAndAudit(t *testing.T) {
	server, _ := newTestServer(t)
	agent := &stubAgent{response: AgentResponse{StatusCode: 200, ContentType: "application/json", Body: []byte(`{"ok":true}`)}}
	server.agent = agent
	tools := &panelAITools{server: server}
	execution := ai.ToolExecutionContext{UserID: "admin", SessionID: "ses", RunID: "run", ToolCallID: "call"}
	if _, err := tools.Execute(context.Background(), execution, "host_nginx_reload", json.RawMessage(`{"reason":"configuration validated"}`)); err != nil {
		t.Fatal(err)
	}
	calls := agent.snapshotCalls()
	if len(calls) != 1 || calls[0].path != "/v1/nginx/reload" || calls[0].method != http.MethodPost || string(calls[0].body) != `{}` {
		t.Fatalf("nginx reload Agent call=%#v", calls)
	}
	audits, _, _ := server.store.ListAudit(10, "")
	encoded, _ := json.Marshal(audits)
	if strings.Contains(string(encoded), "configuration validated") || strings.Contains(string(encoded), `"reason"`) {
		t.Fatalf("reason metadata reached audit: %s", encoded)
	}
}

func TestAIFileToolsUseBoundedAgentRoutesAndRedactContent(t *testing.T) {
	server, _ := newTestServer(t)
	agent := &stubAgent{response: AgentResponse{StatusCode: 200, ContentType: "application/json", Body: []byte(`{"ok":true}`)}}
	server.agent = agent
	tools := &panelAITools{server: server}
	execution := ai.ToolExecutionContext{UserID: "admin", SessionID: "ses", RunID: "run", ToolCallID: "call", Approved: true}
	if _, err := tools.Execute(context.Background(), execution, "host_file_read", json.RawMessage(`{"path":"/tmp/kpanel-ai-test.txt"}`)); err != nil {
		t.Fatal(err)
	}
	version := "sha256:" + strings.Repeat("a", 64)
	arguments := json.RawMessage(`{"path":"/tmp/kpanel-ai-test.txt","content":"sensitive-value","expectedResourceVersion":"` + version + `"}`)
	if _, err := tools.Execute(context.Background(), execution, "host_file_write", arguments); err != nil {
		t.Fatal(err)
	}
	calls := agent.snapshotCalls()
	if len(calls) != 2 || calls[0].path != "/v1/files/text" || calls[0].rawQuery != "path=%2Ftmp%2Fkpanel-ai-test.txt" ||
		calls[1].path != "/v1/files/content" || calls[1].method != http.MethodPut || calls[1].rawQuery != "path=%2Ftmp%2Fkpanel-ai-test.txt" {
		t.Fatalf("file tool calls=%#v", calls)
	}
	audits, _, _ := server.store.ListAudit(20, "")
	for _, event := range audits {
		encoded, _ := json.Marshal(event.Change)
		if strings.Contains(string(encoded), "sensitive-value") {
			t.Fatalf("file content leaked into audit: %s", encoded)
		}
	}
}

func TestAIToolAgentProblemIsSafeAndTraceable(t *testing.T) {
	server, _ := newTestServer(t)
	server.agent = &stubAgent{response: AgentResponse{StatusCode: http.StatusBadRequest, ContentType: "application/problem+json", Body: []byte(`{"title":"invalid","status":400,"code":"invalid_request","detail":"secret payload","requestId":"req-123"}`)}}
	tools := &panelAITools{server: server}
	_, err := tools.Execute(context.Background(), ai.ToolExecutionContext{UserID: "admin"}, "host_system_summary", json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), "invalid_request") || !strings.Contains(err.Error(), "req-123") || strings.Contains(err.Error(), "secret payload") || strings.Contains(err.Error(), "{\"") {
		t.Fatalf("unsafe or untraceable Agent error: %v", err)
	}
}
