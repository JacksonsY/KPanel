package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBlindFragmentAndShellBoundaries(t *testing.T) {
	cases := []struct{ name, input, known string }{
		{"fragment-assignment", `API_TOKEN=prefix["OPAQUE_BODY"]suffix`, ""},
		{"fragment-inline-empty-assignment", `cmd API_TOKEN=["OPAQUE_BODY"]`, ""},
		{"fragment-spaced-flag", `cmd --password ["OPAQUE_BODY"]`, ""},
		{"fragment-attached-mysql", `mysql -p["OPAQUE_BODY"]`, ""},
		{"fragment-object-flag", `cmd --password {"value":"OPAQUE_BODY"}`, ""},
		{"fragment-command-suffix", `mysql --init-command='[1]' -pOPAQUE_BODY database`, ""},
		{"fragment-url", `https://example.test/?token=["OPAQUE_BODY"]`, ""},
		{"fragment-known", `pre["OPAQUE_BODY"]post`, `pre["OPAQUE_BODY"]post`},
		{"pending-eof", "API_TO\\\nKEN=OPAQUE_BODY\\", ""},
		{"ansi-quote", "API_TOKEN=$'first\\'\nOPAQUE_BODY\nlast'\nordinary-visible", ""},
		{"parameter-expansion", "API_TOKEN=${UNSET:-first\nOPAQUE_BODY\nlast}\nordinary-visible", ""},
		{"heredoc", "API_TOKEN=$(cat <<'EOF'\n)\nOPAQUE_BODY\nEOF\n)\nordinary-visible", ""},
		{"fragment-quote", "API_TOKEN=\"first[1]\nOPAQUE_BODY\nlast\"\nordinary-visible", ""},
		{"fragment-joined-identifier", "API_TO\\\nKEN=[\"OPAQUE_BODY\"]\\", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got string
			if c.known != "" {
				got = TextWithSecrets(c.input, c.known)
			} else {
				got = Text(c.input)
			}
			if strings.Contains(got, "OPAQUE_BODY") {
				t.Fatalf("credential remained: %s", got)
			}
			if c.known == "" {
				records := strings.Split(c.input, "\n")
				envelope, _ := json.Marshal(map[string]string{"content": c.input, "safe": "ordinary-visible"})
				array, _ := json.Marshal(records)
				for _, safe := range []string{strings.Join(Records(records), "\n"), strings.Join(Lines([]byte(c.input), -1, 4096), "\n"), string(mustProjectJSON(t, envelope)), string(mustProjectJSON(t, array))} {
					if strings.Contains(safe, "OPAQUE_BODY") {
						t.Errorf("alternate entry point retained credential: %s", safe)
					}
				}
			}
			if c.name == "ansi-quote" && !strings.Contains(got, "ordinary-visible") {
				t.Fatal("closed ANSI-C quote suppressed subsequent ordinary record")
			}
		})
	}
}
