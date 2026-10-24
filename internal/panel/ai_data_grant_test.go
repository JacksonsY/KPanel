package panel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/ai"
)

func TestAIFileDataGrantRequiresHumanDecisionBeforeAgentRead(t *testing.T) {
	server, _ := newTestServer(t)
	agent := &stubAgent{response: AgentResponse{StatusCode: 200, Body: []byte(`{"content":"{\"auths\":{\"registry\":{\"auth\":\"dXNlcjpwYXNz\"}},\"label\":\"ordinary\"}","path":"/root/.docker/config.json"}`)}}
	server.agent = agent
	tools := &panelAITools{server: server}
	args := json.RawMessage(`{"path":"/root/.docker/config.json"}`)
	if !tools.RequiresApproval(context.Background(), "host_file_read", args) {
		t.Fatal("administrator file access delegated to model")
	}
	execution := ai.ToolExecutionContext{UserID: "admin", SessionID: "session", RunID: "run", ToolCallID: "call"}
	if _, err := tools.Execute(context.Background(), execution, "host_file_read", args); !errors.Is(err, ai.ErrToolRejected) || len(agent.snapshotCalls()) != 0 {
		t.Fatalf("read before approval: %v", err)
	}
	if _, err := tools.Execute(context.Background(), execution, "host_file_read", json.RawMessage(`{"path":"/root/.docker/config.json","approved":true}`)); !errors.Is(err, ai.ErrToolArguments) || len(agent.snapshotCalls()) != 0 {
		t.Fatalf("model expanded grant: %v", err)
	}
	execution.Approved = true
	result, err := tools.Execute(context.Background(), execution, "host_file_read", args)
	if err != nil || strings.Contains(result, "dXNlcjpwYXNz") || !strings.Contains(result, "ordinary") || len(agent.snapshotCalls()) != 1 {
		t.Fatalf("approved projection: %s %v", result, err)
	}
}

func TestAIToolProjectionDecodesContentBeforeRedaction(t *testing.T) {
	server, _ := newTestServer(t)
	body, _ := json.Marshal(map[string]any{"content": "{\"passw\\u006frd\":\"escaped-secret\",\"environment\":[{\"name\":\"CUSTOM\",\"value\":\"environment-secret\"}],\"note\":\"normal\\nlog\"}", "auth": "opaque-auth", "version": "sha256:test"})
	agent := &stubAgent{response: AgentResponse{StatusCode: 200, Body: body}}
	server.agent = agent
	result, err := (&panelAITools{server: server}).Execute(context.Background(), ai.ToolExecutionContext{}, "host_system_summary", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"escaped-secret", "environment-secret", "opaque-auth"} {
		if strings.Contains(result, secret) {
			t.Fatalf("projection leaked %s: %s", secret, result)
		}
	}
	if !strings.Contains(result, "normal") || !strings.Contains(result, "sha256:test") {
		t.Fatalf("ordinary fields lost: %s", result)
	}
}

func TestAIFileDisclosureIsAuthorizedIndependentlyOfContentSyntax(t *testing.T) {
	server, _ := newTestServer(t)
	agent := &stubAgent{response: AgentResponse{StatusCode: 200, Body: []byte(`{"content":"ordinary file"}`)}}
	server.agent = agent
	tools := &panelAITools{server: server}
	for _, file := range []string{"/home/web/html/site/wp-config.php", "/home/web/html/site/index.html", "/var/log/app.log"} {
		args, _ := json.Marshal(map[string]string{"path": file})
		for _, tool := range []string{"host_file_read", "host_file_tail"} {
			if !tools.RequiresApproval(context.Background(), tool, args) {
				t.Errorf("file content implicitly disclosed by directory classification: %s", file)
			}
			if _, err := tools.Execute(context.Background(), ai.ToolExecutionContext{}, tool, args); !errors.Is(err, ai.ErrToolRejected) {
				t.Errorf("unapproved content read reached Agent: %s %v", file, err)
			}
		}
	}
	if len(agent.snapshotCalls()) != 0 {
		t.Fatal("unapproved content was read before the human data-sharing decision")
	}
	if tools.RequiresApproval(context.Background(), "host_file_list", json.RawMessage(`{"path":"/home/web"}`)) {
		t.Fatal("metadata listing unnecessarily requires file-content consent")
	}
}

func TestAIActualAuditSinkPreservesEnvironmentNamesAndHidesValues(t *testing.T) {
	server, _ := newTestServer(t)
	agent := &stubAgent{response: AgentResponse{StatusCode: 200, Body: []byte(`{"ok":true}`)}}
	server.agent = agent
	args := json.RawMessage(`{"action":"container_create","image":"alpine","environment":[{"name":"PASSWORD","value":"audit-synthetic-secret"},{"name":"CUSTOM_VALUE","value":"other-synthetic-secret"}]}`)
	_, err := (&panelAITools{server: server}).Execute(context.Background(), ai.ToolExecutionContext{UserID: "admin", RunID: "run", SessionID: "session", ToolCallID: "call"}, "host_docker_task", args)
	if err != nil {
		t.Fatal(err)
	}
	events, _, err := server.store.ListAudit(20, "")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(events)
	if strings.Contains(string(encoded), "audit-synthetic-secret") || strings.Contains(string(encoded), "other-synthetic-secret") || !strings.Contains(string(encoded), "PASSWORD") || !strings.Contains(string(encoded), "CUSTOM_VALUE") {
		t.Fatalf("audit projection lost semantics or leaked values: %s", encoded)
	}
	if len(agent.snapshotCalls()) != 1 || !strings.Contains(string(agent.snapshotCalls()[0].body), "audit-synthetic-secret") {
		t.Fatal("audit projection changed execution arguments")
	}
}
