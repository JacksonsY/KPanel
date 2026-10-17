package panel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditRedactsStructuredEnvironmentValues(t *testing.T) {
	summary := safeArgumentSummary(json.RawMessage(`{"environment":[{"name":"MYSQL_ROOT_PASSWORD","value":"hidden-value"},{"name":"CUSTOM_CREDENTIAL","value":"other-value"}]}`))
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "hidden-value") || strings.Contains(string(encoded), "other-value") {
		t.Fatalf("environment values leaked: %s", encoded)
	}
	if !strings.Contains(string(encoded), "MYSQL_ROOT_PASSWORD") {
		t.Fatal("variable name lost")
	}
}
