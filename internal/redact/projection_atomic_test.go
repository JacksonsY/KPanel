package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestKnownLiteralsCoverCompleteStructuredRepresentations(t *testing.T) {
	for _, key := range []string{`{"a":"opaque"}`, `["opaque"]`, `{ "a" : "opaque" }`, `{"a":"<opaque>"}`} {
		var parsed any
		if err := json.Unmarshal([]byte(key), &parsed); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(map[string]any{"result": parsed, "safe": "ordinary"})
		for _, input := range []string{key, `{"result":` + key + `,"safe":"ordinary"}`, string(encoded)} {
			if safe := TextWithSecrets(input, key); strings.Contains(safe, "opaque") {
				t.Errorf("known structured literal retained: %s", safe)
			}
		}
	}
}

func TestSensitiveFieldsRetainPrivateBlockProvenance(t *testing.T) {
	for _, body := range []string{
		`{"PRIVATE_BODY":"x","password":"x"}`,
		`{"PRIVATE_BODY":"x","password":{"nested":["x",true]}}`,
		`{"PRIVATE_BODY":"x","name":"password","value":"x"}`,
	} {
		records := []string{"initial-visible", "-----BEGIN PRIVATE KEY-----", body, "-----END PRIVATE KEY-----", "safe: final-visible"}
		assertNormalFormProjections(t, records, "PRIVATE_BODY")
		for i, record := range records {
			raw, _ := json.Marshal(map[string]string{"line": record, "time": "ordinary-metadata"})
			records[i] = string(raw)
		}
		assertNormalFormProjections(t, records, "PRIVATE_BODY")
	}
	// Masking the value does not discard its source markers. Later records
	// remain covered even when BEGIN was inside a sensitive field.
	assertNormalFormProjections(t, []string{"initial-visible", `{"password":"-----BEGIN PRIVATE KEY-----"}`, `{"line":"PRIVATE_BODY"}`, `{"line":"-----END PRIVATE KEY-----"}`, "safe: final-visible"}, "PRIVATE_BODY")
	ordinary, err := JSON([]byte(`{"password":"x","safe":true}`))
	if err != nil || string(ordinary) != `{"password":"[REDACTED]","safe":true}` {
		t.Fatalf("ordinary object shape changed: %s %v", ordinary, err)
	}
}
