package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodedJSONLMarkersKeepPrivateKeyRecordState(t *testing.T) {
	for _, marker := range []string{`PRIVA\u0054E`, `PRIVA\u001b[0mTE`} {
		records := []string{`{"line":"initial-visible"}`, `{"line":"-----BEGIN ` + marker + ` KEY-----"}`, `{"line":"SYNTHETIC_BODY"}`, `{"line":"-----END ` + marker + ` KEY-----"}`, `{"line":"final-visible"}`}
		assertRecordBoundaryProjections(t, strings.Join(records, "\n"))
	}
}

func TestMultilineConfigurationCredentialsKeepTheirBoundary(t *testing.T) {
	for _, body := range []string{
		"password: |\n  SYNTHETIC_BODY\nsafe: final-visible",
		"password: >- # folded\n  SYNTHETIC_BODY\nsafe: final-visible",
		"db:\n  password:\n    - SYNTHETIC_BODY\n  safe: final-visible",
		"PASSWORD='first\nSYNTHETIC_BODY\nlast'\nSAFE=final-visible",
		"PASSWORD=\"first\\\"part\nSYNTHETIC_BODY\nlast\"\nSAFE=final-visible",
	} {
		assertRecordBoundaryProjections(t, "initial-visible\n"+body)
	}
}

func assertRecordBoundaryProjections(t *testing.T, body string) {
	t.Helper()
	envelope, _ := json.Marshal(map[string]string{"content": body})
	projected, err := JSON(envelope)
	if err != nil {
		t.Fatal(err)
	}
	for _, safe := range []string{Text(body), strings.Join(Records(strings.Split(body, "\n")), "\n"), strings.Join(Lines([]byte(body), 100, 4096), "\n"), string(projected)} {
		if strings.Contains(safe, "SYNTHETIC_BODY") || !strings.Contains(safe, "initial-visible") || !strings.Contains(safe, "final-visible") {
			t.Fatalf("record boundary lost: %s", safe)
		}
	}
}
