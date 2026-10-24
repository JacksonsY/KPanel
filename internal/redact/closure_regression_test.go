package redact

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestClosureShellLogicalRecords(t *testing.T) {
	for _, body := range []string{
		"API_TOKEN=\"$(printf \"%s\" \"first\nNESTED_BODY\nlast\")\"",
		"API_TOKEN=\"$(printf '%s' \"$(printf \"%s\" \"first\nNESTED_BODY\nlast\")\")\"",
		"API_TO\\\nKEN=SPLIT_BODY",
		"export API_\\\nTO\\\nKEN=\"first\nSPLIT_BODY\nlast\"",
		"curl --api-to\\\nken \"first\nSPLIT_BODY\nlast\"",
	} {
		records := strings.Split("initial-visible\n"+body+"\nsafe: final-visible", "\n")
		for _, secret := range []string{"NESTED_BODY", "SPLIT_BODY"} {
			assertNormalFormProjections(t, records, secret)
			encoded := make([]string, len(records))
			for i, line := range records {
				raw, _ := json.Marshal(map[string]any{"line": line, "time": "ordinary-metadata", "exitCode": 0})
				encoded[i] = string(raw)
			}
			assertNormalFormProjections(t, encoded, secret)
		}
	}
}

func TestClosureMySQLRepeatedCredentials(t *testing.T) {
	for _, command := range []string{
		`mysql -pDECOY_BODY -pACTUAL_BODY -P 3306 database`,
		`mysqldump -p DECOY_BODY -p 'ACTUAL_BODY with spaces' database`,
		`mariadb -pDECOY_BODY -p "ACTUAL_BODY" database`,
	} {
		records := []string{"initial-visible", command, "safe: final-visible"}
		assertNormalFormProjections(t, records, "ACTUAL_BODY")
		if safe := Text(command); strings.Contains(safe, "DECOY_BODY") || !strings.Contains(safe, "database") {
			t.Errorf("command usability or credential masking changed: %s", safe)
		}
	}
}

func TestClosureEmbeddedJSONAndCredentialKeys(t *testing.T) {
	for _, line := range []string{
		`INFO {"auth":"AUTH_BODY","safe":true,"count":9007199254740993}`,
		`INFO {"passw\u006frd":"AUTH_BODY","safe":true,"count":9007199254740993} trailer`,
		`INFO "{\"auth\":\"AUTH_BODY\",\"safe\":true,\"count\":9007199254740993}"`,
		`{"password=AUTH_BODY":"ordinary-value","safe":true,"count":9007199254740993}`,
	} {
		assertNormalFormProjections(t, []string{"initial-visible", line, "final-visible"}, "AUTH_BODY")
		if safe := Text(line); !strings.Contains(safe, "safe") || !strings.Contains(safe, "true") || !strings.Contains(safe, "9007199254740993") {
			t.Errorf("ordinary structured fields lost: %s", safe)
		}
	}
}

func TestClosureKnownIdentityIsSimultaneousAndNormalized(t *testing.T) {
	for _, secrets := range [][]string{{"short", "short/LONG-OPAQUE-TAIL"}, {"short/LONG-OPAQUE-TAIL", "short"}, {"opaque-first\r\nopaque-second"}} {
		key := "short/LONG-OPAQUE-TAIL"
		if len(secrets) == 1 {
			key = secrets[0]
		}
		for _, input := range []string{"before " + key + " after", `{"content":` + quoted("before "+key+" after") + `,"safe":true}`} {
			safe := TextWithSecrets(input, secrets...)
			for _, fragment := range []string{"LONG-OPAQUE-TAIL", "opaque-first", "opaque-second"} {
				if strings.Contains(safe, fragment) {
					t.Errorf("identity fragment retained: %s", safe)
				}
			}
			if !strings.Contains(safe, "before") || !strings.Contains(safe, "after") {
				t.Errorf("identity matching lost safe neighbors: %s", safe)
			}
		}
	}
}

func TestClosureKnownIdentityUsesDecodedChannelsBeforeHeuristics(t *testing.T) {
	for _, fixture := range [][2]string{
		{"before abcde after", "before [REDACTED] after"},
		{`{"line":"before abcde after","safe":true}`, `{"line":"before [REDACTED] after","safe":true}`},
	} {
		for _, secrets := range [][]string{{"abc", "cde"}, {"cde", "abc"}} {
			if safe := TextWithSecrets(fixture[0], secrets...); safe != fixture[1] {
				t.Fatalf("overlapping identity was only partly masked: %s", safe)
			}
		}
	}
	for _, input := range []string{
		"before opaque-first\r\nopaque-second after",
		"{\"line\":\"before opaque-first\"}\n{\"line\":\"opaque-second after\"}",
		`{"content":"{\"line\":\"before opaque-first\"}\n{\"line\":\"opaque-second after\"}","safe":true}`,
	} {
		safe := TextWithSecrets(input, "opaque-first", "opaque-first\r\nopaque-second")
		if strings.Contains(safe, "opaque-first") || strings.Contains(safe, "opaque-second") || !strings.Contains(safe, "before") || !strings.Contains(safe, "after") {
			t.Fatalf("normalized semantic identity did not cover the whole credential: %s", safe)
		}
	}
	if safe := TextWithSecrets("before opaque/password=value after", "opaque", "opaque/password=value"); strings.Contains(safe, "value") || strings.Contains(safe, "password") || !strings.Contains(safe, "before") || !strings.Contains(safe, "after") {
		t.Fatalf("field heuristic broke a known identity: %s", safe)
	}
	if safe := TextWithSecrets(`{"result":{"a":"OPAQUE_BODY\nSECOND_BODY","token_type":"x"},"safe":true}`, `{"a":"OPAQUE_BODY\r\nSECOND_BODY","token_type":"x"}`); strings.Contains(safe, "OPAQUE_BODY") || strings.Contains(safe, "SECOND_BODY") || !strings.Contains(safe, `"safe":true`) {
		t.Fatalf("structured identity was not normalized before field policy: %s", safe)
	}
}

func TestClosureOrdinaryShapesAndSourceBoundaries(t *testing.T) {
	for _, input := range []string{
		`echo "${HOME}"`, `printf '%s' '{ordinary}'`, `mysql -P 3306 -u panel database`,
		"ordinary\\\nvalue", "'literal\\\nnext'", "[INFO] ordinary-visible",
	} {
		if safe := Text(input); safe != input {
			t.Errorf("ordinary source changed: %q -> %q", input, safe)
		}
	}
	for _, content := range []string{
		"API_TOKEN=\"$(printf '%s' \"OPEN_FRAGMENT\nMIDDLE_FRAGMENT\nCLOSE_FRAGMENT\")\"\nordinary-visible",
		"API_TOKEN=$(printf x\n# ) \" misleading comment\nprintf 'MIDDLE_FRAGMENT'\n)\nordinary-visible",
	} {
		safe := Text(content)
		for _, fragment := range []string{"OPEN_FRAGMENT", "MIDDLE_FRAGMENT", "CLOSE_FRAGMENT"} {
			if strings.Contains(safe, fragment) {
				t.Errorf("sensitive continuation retained: %s", safe)
			}
		}
		if !strings.Contains(safe, "ordinary-visible") {
			t.Errorf("safe boundary did not recover: %s", safe)
		}
	}
	for _, key := range []string{"PRIVATE", "-----BEGIN PRIVATE KEY-----\nfirst", "PRIVATE\nfirst"} {
		safe := TextWithSecrets("initial-visible\n-----BEGIN PRIVATE KEY-----\nfirst\nHIDDEN_BODY\n-----END PRIVATE KEY-----\nfinal-visible", key)
		if strings.Contains(safe, "HIDDEN_BODY") || !strings.Contains(safe, "final-visible") {
			t.Fatalf("identity matching consumed source markers: %s", safe)
		}
	}
}

func TestClosureEmbeddedJSONRetainsTypesAndBounds(t *testing.T) {
	body := `{"auth":"PRIVATE_VALUE","safe":[true,null,9007199254740993],"note":"ordinary"}`
	for depth := 0; depth < 4; depth++ {
		input := "INFO " + body + " trailer"
		safe := Text(input)
		if strings.Contains(safe, "PRIVATE_VALUE") || !strings.Contains(safe, "ordinary") || !strings.Contains(safe, "9007199254740993") || !strings.HasPrefix(safe, "INFO ") || !strings.HasSuffix(safe, " trailer") {
			t.Fatalf("embedded structure changed: %s", safe)
		}
		if again := Text(safe); safe != again {
			t.Fatalf("embedded projection is not idempotent: %s -> %s", safe, again)
		}
		body = quoted(body)
	}
	for _, malformed := range []string{`INFO {"passw\u006frd":"PRIVATE_VALUE"`, `INFO "{\"auth\":\"PRIVATE_VALUE\"`} {
		if safe := Text(malformed); strings.Contains(safe, "PRIVATE_VALUE") {
			t.Errorf("malformed embedded JSON leaked: %s", safe)
		}
	}
	if safe := Text("INFO " + strings.Repeat("[", maxProjectionDepth+1) + `"PRIVATE_VALUE"` + strings.Repeat("]", maxProjectionDepth+1)); safe != projectionLimitReplacement {
		t.Fatalf("embedded depth budget escaped: %s", safe)
	}
}

func quoted(value string) string { raw, _ := json.Marshal(value); return string(raw) }

func TestClosureCombinedConsumerShapes(t *testing.T) {
	// These are AI content/tool preview and backup HostConfig shapes. Keep the
	// execution source and host-review facts separate from their display view.
	content := "initial-visible\nAPI_TO\\\nKEN=\"$(printf \"%s\" \"first\nCOMBINED_BODY\nlast\")\"\nINFO {\"auth\":\"COMBINED_BODY\",\"safe\":true}\nmysql -pDECOY -pCOMBINED_BODY database\nfinal-visible"
	input := map[string]any{"content": content, "HostConfig": map[string]any{"Privileged": true, "NetworkMode": "host", "CapAdd": []string{"SYS_ADMIN"}}, "count": json.Number("9007199254740993"), "safe": "ordinary"}
	raw, _ := json.Marshal(input)
	copyRaw := append([]byte(nil), raw...)
	safe := mustProjectJSON(t, raw)
	if !reflect.DeepEqual(raw, copyRaw) || input["content"] != content {
		t.Fatal("projection changed execution input")
	}
	if strings.Contains(string(safe), "COMBINED_BODY") || !strings.Contains(string(safe), "final-visible") || !strings.Contains(string(safe), `"Privileged":true`) || !strings.Contains(string(safe), `"NetworkMode":"host"`) || !strings.Contains(string(safe), "SYS_ADMIN") {
		t.Fatalf("consumer view lost safety or independent facts: %s", safe)
	}
	if again := mustProjectJSON(t, safe); string(again) != string(safe) {
		t.Fatalf("projection is not idempotent:\n%s\n%s", safe, again)
	}
}
