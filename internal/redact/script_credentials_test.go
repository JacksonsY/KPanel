package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestScriptChineseCredentialProjection(t *testing.T) {
	for _, input := range []string{
		"账号: admin  密码: fake-sensitive-value",
		"FRP面板密码: fake-sensitive-value",
		"\x1b[32m访问密码：fake-sensitive-value\x1b[0m",
		`{"数据库密码":"fake-sensitive-value","status":"ready"}`,
	} {
		if safe := Text(input); strings.Contains(safe, "fake-sensitive-value") {
			t.Errorf("credential reached display projection: %q", safe)
		}
	}
	// Structured and chronological records use the same label classifier;
	// quoted values may span records, but following ordinary status stays visible.
	input := []string{"服务密码: 'fake-first", "fake-second'", "status: ready"}
	safe := strings.Join(Records(input), "\n")
	if strings.Contains(safe, "fake-first") || strings.Contains(safe, "fake-second") || !strings.Contains(safe, "status: ready") {
		t.Fatal(safe)
	}
	raw, _ := json.Marshal(map[string]any{"items": input, "口令": "fake-third"})
	projected, err := JSON(raw)
	if err != nil || strings.Contains(string(projected), "fake-") {
		t.Fatalf("%s: %v", projected, err)
	}
}
