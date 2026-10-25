package panel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/cluster"
	"github.com/kejilion/kejilion-panel/internal/mcpaccess"
)

func TestMCPRemoteReceiptAndHistoricalViewProjectCredentials(t *testing.T) {
	raw := json.RawMessage(`{"state":"succeeded","operationId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","password":"synthetic-peer-secret","日志":["访问密码: 'synthetic-first","synthetic-second'","status: ready"]}`)
	state, receipt, err := mcpRemoteOperationResult(cluster.ManagedResponse{Authorized: true, Supported: true, Data: raw})
	if err != nil || state != "succeeded" {
		t.Fatalf("%s: %v", state, err)
	}
	view, _ := json.Marshal(mcpOperationView(mcpaccess.Operation{State: "succeeded", Result: raw}))
	for _, output := range [][]byte{receipt, view} {
		if strings.Contains(string(output), "synthetic-") || !strings.Contains(string(output), "ready") {
			t.Fatal(string(output))
		}
	}
}
