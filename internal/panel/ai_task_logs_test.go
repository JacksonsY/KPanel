package panel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/ai"
	"github.com/kejilion/kejilion-panel/internal/mcpaccess"
)

func TestTaskLogsWithoutContextCannotReachNativeMCPOrHistory(t *testing.T) {
	s, _ := newTestServerWithPublicURL(t, "https://panel.test")
	tool := &panelAITools{server: s}
	// An owner tail can begin inside a quoted password; its bare body cannot
	// be repaired by another text-regex pass over the already-cut response.
	raw := []byte(`{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","status":"succeeded","logs":["synthetic-tail-without-prefix'"],"data":{"text":"synthetic-window","nextOffset":100}}`)
	native, err := tool.finish(ai.ToolExecutionContext{}, "host_job_input", "app", "test", nil, AgentResponse{StatusCode: 200, Body: raw}, nil)
	if err != nil {
		t.Fatal(err)
	}
	mcp, err := mcpManagedOutput(raw, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	mcpBody, _ := json.Marshal(mcp)
	history, _ := json.Marshal(mcpOperationView(mcpaccess.Operation{State: "succeeded", Result: raw}))
	for _, display := range []string{native, string(mcpBody), string(history)} {
		if strings.Contains(display, "synthetic-") || !strings.Contains(display, "succeeded") || !strings.Contains(display, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
			t.Fatal(display)
		}
	}
	qualified := []byte(`{"projection":"model-v1","id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","logs":["status: ready"]}`)
	safe, err := mcpDisplayJSON(qualified)
	if err != nil || !strings.Contains(string(safe), "status: ready") {
		t.Fatalf("qualified logs lost: %s %v", safe, err)
	}
}
