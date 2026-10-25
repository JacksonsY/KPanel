package panel

import (
	"encoding/json"
	"testing"
)

func TestBackupRestoreCannotReviveCollectedController(t *testing.T) {
	// A revoked controller no longer appears after the normal tombstone GC.
	// Restoring an older same-node backup must not make its key active again.
	incoming := []byte(`{"nodeId":"same-node","controllers":[{"id":"removed-peer","state":"active","publicKey":"old-key","scope":"files,terminal"}]}`)
	current := []byte(`{"nodeId":"same-node","controllers":[]}`)
	merged, err := mergeBackupRevocations(incoming, current)
	if err != nil {
		t.Fatal(err)
	}
	var state struct{ Controllers []json.RawMessage }
	if err := json.Unmarshal(merged, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Controllers) != 0 {
		t.Fatalf("old backup revived a removed controller: %s", merged)
	}
}

func TestBackupRestoreControllerAuthorizationBoundaries(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(map[bool]string{true: "legacy", false: "v2"}[legacy], func(t *testing.T) {
			incoming := []byte(`{"nodeId":"same-node","controllers":[{"id":"surviving-peer","state":"active","publicKey":"old-key","scope":"files,terminal"},{"id":"removed-peer","state":"active","publicKey":"removed-key","scope":"files,terminal"}]}`)
			current := []byte(`{"nodeId":"same-node","controllers":[{"id":"surviving-peer","state":"active","publicKey":"current-key","scope":"summary"}]}`)
			merged, err := mergeBackupControllerAccess(incoming, current, legacy)
			if err != nil {
				t.Fatal(err)
			}
			var state struct {
				Controllers []struct{ ID, PublicKey, Scope string }
			}
			if err := json.Unmarshal(merged, &state); err != nil {
				t.Fatal(err)
			}
			if len(state.Controllers) != 1 || state.Controllers[0].ID != "surviving-peer" || state.Controllers[0].PublicKey != "current-key" || state.Controllers[0].Scope != "summary" {
				t.Fatalf("restore changed current authorization or revived a removed peer: %s", merged)
			}
			for _, destination := range [][]byte{nil, []byte(`{"nodeId":"different-node","controllers":[]}`)} {
				migrated, err := mergeBackupControllerAccess(incoming, destination, legacy)
				if err != nil || string(migrated) != string(incoming) {
					t.Fatalf("fresh identity restoration changed: %s %v", migrated, err)
				}
			}
		})
	}
}

func TestBackupRestoreRejectsAmbiguousControllerEnvelope(t *testing.T) {
	current := []byte(`{"nodeId":"same-node","controllers":[]}`)
	for _, incoming := range []string{
		`{"nodeId":"different-node","nodeid":"same-node","controllers":[{"id":"removed-peer","state":"active"}]}`,
		`{"nodeId":"same-node","controllers":[],"Controllers":[{"id":"removed-peer","state":"active"}]}`,
	} {
		for _, legacy := range []bool{false, true} {
			if merged, err := mergeBackupControllerAccess([]byte(incoming), current, legacy); err == nil {
				t.Fatalf("ambiguous authorization envelope accepted: %s", merged)
			}
		}
	}
}
