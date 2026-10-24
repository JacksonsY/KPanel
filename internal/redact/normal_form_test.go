package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalFormJSONLConfigurationContinuation(t *testing.T) {
	records := []string{`{"line":"initial-visible"}`, `{"line":"password: |"}`, `{"line":"  CONFIG_SECRET"}`, `{"line":"safe: final-visible"}`}
	assertNormalFormProjections(t, records, "CONFIG_SECRET")
}

func TestNormalFormYAMLQuotedContinuation(t *testing.T) {
	records := []string{"initial-visible", "password: 'first", "  it''s still inside", "  CONFIG_SECRET", "  last'", "safe: final-visible"}
	assertNormalFormProjections(t, records, "CONFIG_SECRET")
}

func TestNormalFormEnvironmentTupleKeys(t *testing.T) {
	for _, pair := range [][2]string{{"Name", "Value"}, {"NAME", "VALUE"}, {"na\x00me", "va\x1b[0mlue"}} {
		raw, _ := json.Marshal(map[string]string{pair[0]: "CUSTOM", pair[1]: "OPAQUE_VALUE", "safe": "visible"})
		for _, safe := range []string{Text(string(raw)), strings.Join(Records([]string{string(raw)}), "\n"), string(mustProjectJSON(t, raw))} {
			if strings.Contains(safe, "OPAQUE_VALUE") || !strings.Contains(safe, "visible") {
				t.Errorf("tuple keys bypassed projection: %s", safe)
			}
		}
	}
}

func mustProjectJSON(t *testing.T, raw []byte) []byte {
	t.Helper()
	safe, err := JSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	return safe
}

func assertNormalFormProjections(t *testing.T, records []string, secret string) {
	t.Helper()
	body := strings.Join(records, "\n")
	envelope, _ := json.Marshal(map[string]string{"content": body})
	array, _ := json.Marshal(records)
	for _, safe := range []string{Text(body), strings.Join(Records(records), "\n"), strings.Join(Lines([]byte(body), 100, 4096), "\n"), string(mustProjectJSON(t, envelope)), string(mustProjectJSON(t, array))} {
		if strings.Contains(safe, secret) || !strings.Contains(safe, "initial-visible") || !strings.Contains(safe, "final-visible") {
			t.Errorf("semantic records lost state: %s", safe)
		}
	}
}

func TestNormalFormEncodedLeavesShareOneStateMachine(t *testing.T) {
	for _, records := range [][]string{
		{`{"line":"initial-visible"}`, `{"line":"password: >- # folded"}`, `{"l\u0069ne":"  CONFIG_SECRET"}`, `{"line":"safe: final-visible"}`},
		{`{"line":"initial-visible"}`, `{"line":"PASSWORD='first"}`, `{"line":"CONFIG_SECRET"}`, `{"line":"last'"}`, `{"line":"SAFE=final-visible"}`},
		{`{"line":"initial-visible"}`, `{"line":"password: 'it''s first"}`, `{"line":"  it''''s still inside"}`, `{"line":"  CONFIG_SECRET"}`, `{"line":"last'"}`, `{"line":"safe: final-visible"}`},
		{`{"line":"initial-visible"}`, `{"line":"\u002d----BEGIN PRIVA\u001b[0mTE KEY-----"}`, `{"line":"CONFIG_SECRET"}`, `{"line":"-----END PRIVA\u0054E KEY-----"}`, `{"line":"final-visible"}`},
	} {
		assertNormalFormProjections(t, records, "CONFIG_SECRET")
		body := strings.Join(records, "\n")
		for i := 0; i < 4; i++ {
			raw, _ := json.Marshal(map[string]string{"content": body, "visible": "ordinary-field"})
			body = string(raw)
		}
		safe := Text(body)
		if strings.Contains(safe, "CONFIG_SECRET") || !strings.Contains(safe, "ordinary-field") || !strings.Contains(safe, "final-visible") {
			t.Fatalf("nested content lost semantic state: %s", safe)
		}
	}
}

func TestNormalFormPreservesJSONTypesAndRecordPositions(t *testing.T) {
	raw := []byte(`{"array":[null,true,false,9007199254740993,1.2300e+40,{"safe":"visible"}],"content":"ordinary\npassword: |\n  CONFIG_SECRET\nsafe: visible","nested":"{\"sizeBytes\":9007199254740993,\"safe\":\"visible\"}"}`)
	safe := mustProjectJSON(t, raw)
	var value struct {
		Array   []json.RawMessage `json:"array"`
		Content string            `json:"content"`
		Nested  string            `json:"nested"`
	}
	if err := json.Unmarshal(safe, &value); err != nil {
		t.Fatal(err)
	}
	for i, expected := range []string{"null", "true", "false", "9007199254740993", "1.2300e+40", `{"safe":"visible"}`} {
		if string(value.Array[i]) != expected {
			t.Fatalf("JSON value changed: %s", safe)
		}
	}
	if value.Content != "ordinary\npassword: [REDACTED]\nsafe: visible" || !json.Valid([]byte(value.Nested)) || !strings.Contains(value.Nested, "9007199254740993") {
		t.Fatalf("readable content changed: %s", safe)
	}
	records := []string{"before", `{"line":"-----BEGIN PRIVATE KEY-----"}`, `{"line":"CONFIG_SECRET"}`, `{"line":"-----END PRIVATE KEY-----"}`, "after"}
	projected := Records(records)
	if len(projected) != len(records) || projected[0] != "before" || projected[2] != "" || projected[3] != "" || projected[4] != "after" {
		t.Fatalf("record positions changed: %#v", projected)
	}
	lines := Lines([]byte(strings.Join(records, "\n")), -1, 4096)
	if len(lines) != 3 || lines[0] != "before" || lines[2] != "after" {
		t.Fatalf("suppressed lines changed: %#v", lines)
	}
}

func TestNormalFormYAMLSingleQuoteBackslashIsLiteral(t *testing.T) {
	records := []string{"initial-visible", "password: 'first", "  CONFIG_SECRET\\'", "safe: final-visible"}
	assertNormalFormProjections(t, records, "CONFIG_SECRET")
}

func TestNormalFormFieldNamesUseOneCanonicalization(t *testing.T) {
	for _, pair := range [][2]string{{"nAmE", "vAlUe"}, {"na\tme", "va\r\nlue"}, {"na\x1b[31mme", "v.a_l-uE"}} {
		raw, _ := json.Marshal(map[string]any{pair[0]: "CUSTOM", pair[1]: map[string]string{"ordinary": "OPAQUE_VALUE"}, "safe": "visible", "count": 5})
		safe := mustProjectJSON(t, raw)
		if strings.Contains(string(safe), "OPAQUE_VALUE") || !strings.Contains(string(safe), "visible") || !strings.Contains(string(safe), `"count":5`) {
			t.Fatalf("inconsistent canonical key policy: %s", safe)
		}
	}
	if safe := string(mustProjectJSON(t, []byte(`{"Name":42,"Value":"ordinary-value","safe":"visible"}`))); !strings.Contains(safe, "ordinary-value") {
		t.Fatalf("non-tuple value changed: %s", safe)
	}
}

func TestNormalFormDepthAndInvalidDataFailClosed(t *testing.T) {
	for _, depth := range []int{maxProjectionDepth, maxProjectionDepth + 1} {
		raw := strings.Repeat("[", depth) + "null" + strings.Repeat("]", depth)
		_, err := JSON([]byte(raw))
		if (err != nil) != (depth > maxProjectionDepth) {
			t.Fatalf("depth %d: %v", depth, err)
		}
		if depth > maxProjectionDepth && Text(raw) != projectionLimitReplacement {
			t.Fatal("text nesting limit did not fail closed")
		}
	}
	for _, malformed := range []string{`{"passw\u006frd":"CONFIG_SECRET"`, `["CONFIG_SECRET"`, `{"Name":"CUSTOM","Value":"CONFIG_SECRET"} trailing`} {
		if _, err := JSON([]byte(malformed)); err == nil {
			t.Fatalf("malformed JSON accepted: %s", malformed)
		}
		if safe := Text(malformed); strings.Contains(safe, "CONFIG_SECRET") {
			t.Fatalf("malformed structured text leaked: %s", safe)
		}
	}
	if safe := Text("[INFO] initial-visible\npassword=CONFIG_SECRET\n[INFO] final-visible"); strings.Contains(safe, "CONFIG_SECRET") || !strings.Contains(safe, "final-visible") {
		t.Fatalf("ordinary log prefixes changed: %s", safe)
	}
	// JSON APIs cannot create cycles. The internal depth guard also fails closed
	// if an accidental package-local cyclic value ever reaches the normalizer.
	cyclic := map[string]any{}
	cyclic["content"] = cyclic
	if _, err := newProjection().jsonValue(cyclic, 0); err == nil {
		t.Fatal("cyclic internal value was accepted")
	}
}

func TestNormalFormBudgetsAreSharedAcrossEncodedLayers(t *testing.T) {
	oversized := strings.Repeat("x", maxProjectionInputBytes+1)
	if Text(oversized) != projectionLimitReplacement {
		t.Fatal("input byte budget did not fail closed")
	}
	if _, err := JSON([]byte(oversized)); err == nil {
		t.Fatal("JSON byte budget did not fail closed")
	}
	many := strings.Repeat("\n", maxProjectionEntries)
	if lines := Lines([]byte(many), -1, 4096); len(lines) != 1 || lines[0] != projectionLimitReplacement {
		t.Fatal("physical entry budget did not fail closed")
	}
	raw := "[" + strings.Repeat("null,", maxProjectionEntries) + "null]"
	if _, err := JSON([]byte(raw)); err == nil {
		t.Fatal("JSON entry budget did not fail closed")
	}
	nested := strings.Repeat("x", 4<<20)
	for i := 0; i < 9; i++ {
		encoded, _ := json.Marshal(nested)
		nested = string(encoded)
	}
	if len(nested) > maxProjectionInputBytes {
		t.Fatal("work-budget fixture exceeds input budget")
	}
	if Text(nested) != projectionLimitReplacement {
		t.Fatal("encoded layers restarted the work budget")
	}
	if _, err := JSON([]byte(nested)); err == nil {
		t.Fatal("JSON encoded layers restarted the work budget")
	}
}

func TestNormalFormStandaloneMultilineQuoteClosesBeforeSafeSuffix(t *testing.T) {
	for _, close := range []string{`"`, `" # end`} {
		records := []string{"initial-visible", `password: "first`, "CONFIG_SECRET", close, "safe: final-visible"}
		assertNormalFormProjections(t, records, "CONFIG_SECRET")
	}
}

func TestNormalFormBudgetFailureKeepsLineOutputBounds(t *testing.T) {
	input := []byte(strings.Repeat("\n", maxProjectionEntries))
	if got := Lines(input, 0, 8); len(got) != 0 {
		t.Fatalf("zero limit changed: %#v", got)
	}
	if got := Lines(input, 1, 8); len(got) != 1 || got[0] != "[REDACTE…" {
		t.Fatalf("line bound changed on projection failure: %#v", got)
	}
}

func TestNormalFormConfigurationMetadataDoesNotEndPayloadContinuation(t *testing.T) {
	records := []string{`{"line":"initial-visible","time":"ordinary-metadata"}`, `{"line":"password: |","time":"ordinary-metadata"}`, `{"line":"  CONFIG_SECRET","time":"ordinary-metadata"}`, `{"line":"safe: final-visible","time":"ordinary-metadata"}`}
	assertNormalFormProjections(t, records, "CONFIG_SECRET")
	for _, record := range Records(records) {
		if !strings.Contains(record, "ordinary-metadata") {
			t.Fatalf("unrelated metadata was hidden: %s", record)
		}
	}
}

func TestNormalFormStructuredConfigurationBodyRetainsPhysicalIndent(t *testing.T) {
	assertNormalFormProjections(t, []string{"initial-visible", "password: |", `  {"ordinary":"CONFIG_SECRET"}`, "safe: final-visible"}, "CONFIG_SECRET")
	assertNormalFormProjections(t, []string{`{"line":"initial-visible"}`, `{"line":"password: |"}`, `{"line":"  {\"ordinary\":\"CONFIG_SECRET\"}"}`, `{"line":"safe: final-visible"}`}, "CONFIG_SECRET")
}

func TestNormalFormPEMMetadataCannotClosePayloadChannel(t *testing.T) {
	records := []string{`{"line":"initial-visible","time":"ordinary-metadata"}`, `{"line":"-----BEGIN PRIVATE KEY-----","time":"-----END PRIVATE KEY-----"}`, `{"line":"CONFIG_SECRET","time":"-----BEGIN PRIVATE KEY-----"}`, `{"line":"-----END PRIVATE KEY-----","time":"ordinary-metadata"}`, `{"line":"final-visible","time":"ordinary-metadata"}`}
	assertNormalFormProjections(t, records, "CONFIG_SECRET")
}

func TestNormalFormChannelsContinueAcrossArrayElementsAndNames(t *testing.T) {
	raw := []byte(`{"logs":[{"payload":{"line":"initial-visible"},"time":"metadata"},{"payload":{"line":"password: 'it''s first"},"time":"metadata"},{"payload":{"l\u0000ine":"CONFIG_SECRET"},"time":"metadata"},{"payload":{"line":"last'"},"time":"metadata"},{"payload":{"line":"safe: final-visible"},"time":"metadata"}]}`)
	safe := mustProjectJSON(t, raw)
	if strings.Contains(string(safe), "CONFIG_SECRET") || !strings.Contains(string(safe), "initial-visible") || !strings.Contains(string(safe), "final-visible") || !strings.Contains(string(safe), "metadata") {
		t.Fatalf("array channel continuation changed: %s", safe)
	}
}

func TestNormalFormPEMUnionCoversNestedAndUnclosedPayloads(t *testing.T) {
	for _, body := range []string{"CONFIG_SECRET", `{"ordinary":"CONFIG_SECRET"}`} {
		encoded, _ := json.Marshal(map[string]string{"line": body, "time": "-----BEGIN PRIVATE KEY-----"})
		records := []string{`{"line":"initial-visible","time":"metadata"}`, `{"line":"-----BEGIN PRIVATE KEY-----","time":"-----END PRIVATE KEY-----"}`, string(encoded), `{"line":"-----END PRIVATE KEY-----","time":"metadata"}`, `{"line":"final-visible","time":"metadata"}`}
		assertNormalFormProjections(t, records, "CONFIG_SECRET")
		records = records[:3]
		if safe := strings.Join(Records(records), "\n"); strings.Contains(safe, "CONFIG_SECRET") || !strings.Contains(safe, "initial-visible") {
			t.Fatalf("unclosed channel leaked: %s", safe)
		}
	}
	assertNormalFormProjections(t, []string{`{"initial":"initial-visible"}`, `{"begin":"-----BEGIN PRIVATE KEY-----"}`, `{"body":"CONFIG_SECRET"}`, `{"end":"-----END PRIVATE KEY-----"}`, `{"safe":"final-visible"}`}, "CONFIG_SECRET")
}
