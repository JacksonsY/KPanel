package panel

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestMCPForeignBackupReviewAndConfirmationRetainApproval(t *testing.T) {
	catalog := managedCatalog()
	review, ok := catalog["host_backup_review"]
	if !ok || !review.ReadOnly {
		t.Fatal("read-only backup review unavailable")
	}
	id := strings.Repeat("a", 32)
	request, err := review.Prepare(json.RawMessage(`{"backupId":"`+id+`"}`), nil)
	if err != nil || request.Method != http.MethodGet || request.Path != "/api/v1/backups/"+id+"/review" {
		t.Fatalf("review request: %#v %v", request, err)
	}
	restore := catalog["host_backup_restore"]
	raw := json.RawMessage(`{"backupId":"` + id + `","modules":["docker"],"revision":"current","confirmForeign":true}`)
	if restore.ReadOnly || restore.automatic(raw) {
		t.Fatal("restore no longer requires approval")
	}
	if err := restore.validate(raw); err != nil {
		t.Fatal(err)
	}
	request, err = restore.Prepare(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	var body backupRequest
	if json.Unmarshal(request.Body, &body) != nil || !body.ConfirmForeign {
		t.Fatalf("confirmation lost: %s", request.Body)
	}
}
