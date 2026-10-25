package panel

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPManagedScriptCredentials(t *testing.T) {
	for _, data := range []any{
		map[string]any{"output": "账号: admin  密码: synthetic-password", "state": "ready"},
		map[string]any{"FRP面板密码": "synthetic-password", "state": "ready"},
		map[string]any{"logs": []string{"数据库密码: 'synthetic-password", "synthetic-continuation'", "status: ready"}},
		map[string]any{"dataBase64": base64.StdEncoding.EncodeToString([]byte("FRP面板密码: synthetic-password\nstatus: ready")), "nextOffset": 120.0},
	} {
		body, _ := json.Marshal(data)
		output, err := mcpManagedOutput(body, 0, 50)
		if err != nil {
			t.Fatal(err)
		}
		safe, _ := json.Marshal(output)
		if strings.Contains(string(safe), "synthetic-") || strings.Contains(string(safe), "dataBase64") || !strings.Contains(string(safe), "ready") {
			t.Fatalf("unsafe or unusable script projection: %s", safe)
		}
	}
}
