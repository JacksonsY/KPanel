package redact

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

func TestKnownLiteralsUseTheSharedSemanticProjection(t *testing.T) {
	key := "opaque<credential>"
	inner := `{"note":"opaque\u003ccredential\u003e","count":9007199254740993,"safe":"visible"}`
	raw, _ := json.Marshal(map[string]string{"content": inner, key: "value"})
	safe := TextWithSecrets(string(raw), key)
	if strings.Contains(safe, "credential") || !strings.Contains(safe, "visible") || !strings.Contains(safe, "9007199254740993") || !json.Valid([]byte(safe)) {
		t.Fatalf("known credential escaped semantic projection: %s", safe)
	}
	// A known literal must not prevent the common state machine from seeing
	// private-key markers before the additional literal projection runs.
	safe = TextWithSecrets("-----BEGIN PRIVATE KEY-----\nPRIVATE_BODY\n-----END PRIVATE KEY-----\nafter", "PRIVATE")
	if strings.Contains(safe, "PRIVATE_BODY") || !strings.Contains(safe, "after") {
		t.Fatalf("literal projection hid the PEM boundary: %s", safe)
	}
	if safe := TextWithSecrets(strings.Repeat("x", 8<<20), "x"); safe != projectionLimitReplacement {
		t.Fatalf("short literal expansion escaped the shared budget: len=%d", len(safe))
	}
}

func TestJSONEncodingRejectsGrowthBeforeAllocation(t *testing.T) {
	value := strings.Repeat("<", 2<<20)
	p := newProjection()
	p.budget.bytes = maxProjectionWorkBytes - (1 << 20)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := p.marshal(value)
	runtime.ReadMemStats(&after)
	if err == nil || after.TotalAlloc-before.TotalAlloc > 2<<20 {
		t.Fatalf("encoding allocated over budget before rejecting: err=%v allocated=%d", err, after.TotalAlloc-before.TotalAlloc)
	}
}

func TestKnownMultilineLiteralsAreMatchedAfterReassembly(t *testing.T) {
	key := "opaque-first\nopaque-second"
	for _, body := range []string{"before\n" + key + "\nafter", `{"content":"before\nopaque-first\nopaque-second\nafter","safe":true}`} {
		safe := TextWithSecrets(body, key)
		if strings.Contains(safe, "opaque-first") || strings.Contains(safe, "opaque-second") || !strings.Contains(safe, "before") || !strings.Contains(safe, "after") {
			t.Fatalf("multiline known secret escaped reconstructed projection: %s", safe)
		}
	}
	key = "-----BEGIN PRIVATE KEY-----\nfirst"
	if safe := TextWithSecrets(key+"\nPRIVATE_BODY\n-----END PRIVATE KEY-----\nafter", key); strings.Contains(safe, "PRIVATE_BODY") || !strings.Contains(safe, "after") {
		t.Fatalf("literal matching hid the common PEM boundary: %s", safe)
	}
}

func TestKnownLiteralsCoverScalarTerminalsWithoutChangingOrdinaryTypes(t *testing.T) {
	for _, raw := range []string{`{"result":12345678,"safe":9007199254740993}`, `{"result":[12345678],"safe":9007199254740993}`, `{"content":"{\"result\":12345678}","safe":9007199254740993}`} {
		safe := TextWithSecrets(raw, "12345678")
		if strings.Contains(safe, "12345678") || !strings.Contains(safe, `"safe":9007199254740993`) || !json.Valid([]byte(safe)) {
			t.Errorf("numeric credential escaped projection: %s", safe)
		}
	}
	for _, pair := range [][2]string{{`{"result":true}`, "true"}, {`{"result":null}`, "null"}, {`{"result":{}}`, "{}"}} {
		if safe := TextWithSecrets(pair[0], pair[1]); strings.Contains(safe, pair[1]) || !json.Valid([]byte(safe)) {
			t.Errorf("known scalar literal escaped projection: %s", safe)
		}
	}
	if safe := TextWithSecrets(`{"bool":true,"null":null,"array":[],"object":{},"number":9007199254740993}`, "unrelated-key"); safe != `{"array":[],"bool":true,"null":null,"number":9007199254740993,"object":{}}` {
		t.Errorf("ordinary scalar types changed: %s", safe)
	}
}
