package hostbackup

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode"
)

func TestCanonicalKeysUnicodeFold(t *testing.T) {
	for _, pair := range [][2]string{{"Key", "Key"}, {"Source", "ſource"}, {"Σ", "ς"}, {"σ", "Σ"}, {"İ", "i"}, {"ı", "I"}, {"ß", "SS"}, {"ä", "Ä"}} {
		m := map[string]json.RawMessage{pair[0]: json.RawMessage(`true`), pair[1]: json.RawMessage(`false`)}
		if got, want := canonicalKeys(m), !strings.EqualFold(pair[0], pair[1]); got != want {
			t.Fatalf("%q canonicalKeys=%t, want %t", pair, got, want)
		}
		if canonicalKeys(map[string]json.RawMessage{pair[1]: nil}, pair[0]) == strings.EqualFold(pair[0], pair[1]) {
			t.Fatalf("interpreted spelling %q accepted incorrectly", pair)
		}
	}
	if !canonicalKeys(map[string]json.RawMessage{"Source": nil}, "Source") {
		t.Fatal("canonical interpreted spelling rejected")
	}
}

func TestFoldJSONKeyMatchesUnicodeSimpleFold(t *testing.T) {
	// Exercise every nontrivial fold cycle once per rune, a fixed linear
	// Unicode-table regression rather than an all-pairs CPU stress test.
	for r := rune(0); r <= unicode.MaxRune; r++ {
		next := unicode.SimpleFold(r)
		if next != r && foldJSONKey(string(r)) != foldJSONKey(string(next)) {
			t.Fatalf("fold mismatch U+%04X / U+%04X", r, next)
		}
	}
}

func BenchmarkCanonicalKeys(b *testing.B) {
	for _, count := range []int{1000, 4000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			m := make(map[string]json.RawMessage, count)
			for n := range count {
				m[fmt.Sprintf("Field%06d", n)] = nil
			}
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				if !canonicalKeys(m, "Binds", "Mounts", "NetworkMode") {
					b.Fatal("distinct field rejected")
				}
			}
		})
	}
}
