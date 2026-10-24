package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfigurationQuotesUseTheirOwnLexicalSource(t *testing.T) {
	for _, quoteBody := range []string{`  "\u0027"`, `  {"note":"\u0027"}`, `  ["\u0027"]`, `  "\"\\u0027\""`} {
		records := []string{"initial-visible", "password: 'first", quoteBody, "  CONFIG_SECRET", "  last'", "safe: final-visible"}
		assertNormalFormProjections(t, records, "CONFIG_SECRET")
		// Encoding each physical line adds a lexical layer without changing the
		// YAML source layer in which the sensitive quote must be closed.
		encoded := make([]string, len(records))
		jsonl := make([]string, len(records))
		for i, record := range records {
			raw, _ := json.Marshal(record)
			encoded[i] = string(raw)
			raw, _ = json.Marshal(map[string]string{"line": record, "time": "ordinary-metadata"})
			jsonl[i] = string(raw)
		}
		assertNormalFormProjections(t, encoded, "CONFIG_SECRET")
		assertNormalFormProjections(t, jsonl, "CONFIG_SECRET")
		body := strings.Join(jsonl, "\n")
		for i := 0; i < 3; i++ {
			raw, _ := json.Marshal(map[string]string{"content": body})
			body = string(raw)
		}
		if safe := Text(body); strings.Contains(safe, "CONFIG_SECRET") || !strings.Contains(safe, "final-visible") || !strings.Contains(safe, "ordinary-metadata") {
			t.Errorf("nested lexical source changed: %s", safe)
		}
	}
}

func TestYAMLSequenceConfigurationContinuation(t *testing.T) {
	for _, entry := range []string{"- password: |", "  - password: >- # folded", "- - password:", "- password: 'first", `- password: "first`} {
		close := ""
		if strings.HasSuffix(entry, "'first") {
			close = "    last'\n"
		} else if strings.HasSuffix(entry, `"first`) {
			close = "    last\"\n"
		}
		records := strings.Split("initial-visible\n"+entry+"\n      CONFIG_SECRET\n"+close+"safe: final-visible", "\n")
		assertNormalFormProjections(t, records, "CONFIG_SECRET")
		for i, record := range records {
			raw, _ := json.Marshal(map[string]string{"line": record, "time": "ordinary-metadata"})
			records[i] = string(raw)
		}
		assertNormalFormProjections(t, records, "CONFIG_SECRET")
	}
}

func TestYAMLSequenceFieldIndentPreservesVisibleSiblings(t *testing.T) {
	for _, body := range []string{
		"- password: |\n    CONFIG_SECRET\n  safe: final-visible",
		"- - password: |\n      CONFIG_SECRET\n    safe: final-visible",
		"  - password: |\n      CONFIG_SECRET\n    safe: final-visible",
	} {
		assertNormalFormProjections(t, strings.Split("initial-visible\n"+body, "\n"), "CONFIG_SECRET")
	}
}

func TestConfigurationLiteralQuoteClosesAtSourceLayer(t *testing.T) {
	for _, close := range []string{`  "'"`, `  {"ordinary":"'"}`, `  [''] '`} {
		records := []string{"initial-visible", "password: 'first", "  CONFIG_SECRET", close, "safe: final-visible"}
		assertNormalFormProjections(t, records, "CONFIG_SECRET")
	}
}

func TestConfigurationContinuationSurvivesMixedRepresentations(t *testing.T) {
	for _, records := range [][]string{
		{"initial-visible", `"password: |"`, "  CONFIG_SECRET", "safe: final-visible"},
		{"initial-visible", `"password: |"`, `  {"ordinary":"CONFIG_SECRET"}`, "safe: final-visible"},
		{"initial-visible", `"password: 'first"`, `  {"ordinary":"\u0027"}`, "  CONFIG_SECRET", `"last'"`, "safe: final-visible"},
	} {
		assertNormalFormProjections(t, records, "CONFIG_SECRET")
	}
}

func TestEncodedRecordQuoteClosesInDecodedSourceLayer(t *testing.T) {
	// Here the sensitive opener itself was inside a JSON string record. The
	// record's decoded source is authoritative for its closer as well.
	assertNormalFormProjections(t, []string{"initial-visible", `"password: \u0027first"`, `"  CONFIG_SECRET"`, `"last\u0027"`, `"safe: final-visible"`}, "CONFIG_SECRET")
}

func TestConfigurationSuppressesStructuredValuesWithoutTextLeaves(t *testing.T) {
	for _, body := range []string{`  {"ordinary":987654321}`, `  [987654321]`, `  {"987654321":null}`, `  {"ordinary":{"987654321":[]}}`} {
		for _, opener := range []string{"password: |", `"password: |"`} {
			assertNormalFormProjections(t, []string{"initial-visible", opener, body, "safe: final-visible"}, "987654321")
		}
	}
	for _, body := range []string{`{"ordinary":987654321}`, `{"987654321":false}`, `{"987654321":[]}`, `{"987654321":{}}`} {
		assertNormalFormProjections(t, []string{"initial-visible", "-----BEGIN PRIVATE KEY-----", body, "-----END PRIVATE KEY-----", "final-visible"}, "987654321")
	}
}

func TestPrivateBodyCannotStartUnrelatedConfigurationContinuation(t *testing.T) {
	assertNormalFormProjections(t, []string{"initial-visible", "-----BEGIN PRIVATE KEY-----", "password: 'CONFIG_SECRET", "-----END PRIVATE KEY-----", "safe: final-visible"}, "CONFIG_SECRET")
}

func TestOrphanPrivateEndMasksScalarPrefix(t *testing.T) {
	for _, raw := range []string{`[987654321,"-----END PRIVATE KEY-----","final-visible"]`, `[{"987654321":[]},"-----END PRIVATE KEY-----","final-visible"]`} {
		for _, safe := range []string{Text(raw), string(mustProjectJSON(t, []byte(raw)))} {
			if strings.Contains(safe, "987654321") || !strings.Contains(safe, "final-visible") || !strings.Contains(safe, "[REDACTED") {
				t.Errorf("uncertain scalar prefix was retained: %s", safe)
			}
		}
	}
}

func TestYAMLImplicitValuesKeepCommentsAndIndentlessSequences(t *testing.T) {
	for _, body := range []string{
		"password:\n- CONFIG_SECRET\nsafe: final-visible",
		"- password:\n  - CONFIG_SECRET\n  safe: final-visible",
		"password: # comment\n  CONFIG_SECRET\nsafe: final-visible",
		"password:\n# comment\n  CONFIG_SECRET\nsafe: final-visible",
		"password:\n-\n  CONFIG_SECRET\nsafe: final-visible",
	} {
		assertNormalFormProjections(t, strings.Split("initial-visible\n"+body, "\n"), "CONFIG_SECRET")
	}
	// Explicit block scalars end at dedent, including a new sequence item.
	if safe := Text("password: |\n  CONFIG_SECRET\n- ordinary-visible"); strings.Contains(safe, "CONFIG_SECRET") || !strings.Contains(safe, "ordinary-visible") {
		t.Fatalf("explicit scalar inherited implicit sequence semantics: %s", safe)
	}
}

func TestShellSensitiveAssignmentsCarryPhysicalContinuation(t *testing.T) {
	for _, header := range []string{`export API_TOKEN=\`, `API_TOKEN=prefix\\\`, `API_TOKEN="first"\`, `echo API_TOKEN=\`, `curl --api-token \`} {
		records := []string{"initial-visible", header, `  prefix\`, "  CONFIG_SECRET", "safe: final-visible"}
		assertNormalFormProjections(t, records, "CONFIG_SECRET")
		for i, record := range records {
			raw, _ := json.Marshal(map[string]string{"line": record, "time": "ordinary-metadata"})
			records[i] = string(raw)
		}
		assertNormalFormProjections(t, records, "CONFIG_SECRET")
		body := strings.Join(records, "\n")
		for i := 0; i < 3; i++ {
			raw, _ := json.Marshal(map[string]string{"content": body})
			body = string(raw)
		}
		if safe := Text(body); strings.Contains(safe, "CONFIG_SECRET") || !strings.Contains(safe, "final-visible") || !strings.Contains(safe, "ordinary-metadata") {
			t.Errorf("nested shell source changed: %s", safe)
		}
	}
}

func TestShellContinuationEndsAtItsSourceBoundary(t *testing.T) {
	for _, header := range []string{`API_TOKEN=value\\`, "API_TOKEN=value\\ ", `export API_TOKEN= # comment\`} {
		if safe := Text(header + "\nordinary-visible"); !strings.Contains(safe, "ordinary-visible") {
			t.Errorf("non-continuation hid an adjacent record: %s", safe)
		}
	}
	for _, records := range [][]string{
		{"initial-visible", `API_TOKEN='first\`, "  CONFIG_SECRET'", "safe: final-visible"},
		{"initial-visible", `API_TOKEN="first\`, `  CONFIG_SECRET"\`, "last", "safe: final-visible"},
		{"initial-visible", `API_TOKEN='first`, `  end\'`, "safe: final-visible"},
	} {
		assertNormalFormProjections(t, records, "CONFIG_SECRET")
	}
}
