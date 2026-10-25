package panel

import (
	"encoding/json"
	"testing"
)

func TestBackupRevocationsCompareDecodedIdentities(t *testing.T) {
	current := []byte(`{"nodeId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","controllers":[{"id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","state":"revoked"}]}`)
	for _, incoming := range []string{
		`{"nodeId":"\u0061aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","controllers":[{"id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","state":"active"}]}`,
		`{"nodeId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","controllers":[{"id":"\u0062bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","state":"active"}]}`,
	} {
		merged, err := mergeBackupRevocations([]byte(incoming), current)
		if err != nil {
			t.Fatal(err)
		}
		var result struct{ Controllers []struct{ ID, State string } }
		if err := json.Unmarshal(merged, &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Controllers) != 1 || result.Controllers[0].ID != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" || result.Controllers[0].State != "revoked" {
			t.Fatal(string(merged))
		}
	}
}
