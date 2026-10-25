package agent

import (
	"net/http"
	"testing"
)

func TestVMAgentBoundaryRejectsUnauthenticatedAndMalformedRequests(t *testing.T) {
	s := testServer(t)
	if r := systemServiceAgentRequest(s, http.MethodGet, "/v1/virtual-machines", "", false); r.Code != 401 {
		t.Fatal(r.Code)
	}
	for _, test := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/v1/virtual-machines", "", 405},
		{http.MethodGet, "/v1/virtual-machines?uri=qemu+ssh://evil/system", "", 400},
		{http.MethodGet, "/v1/virtual-machines/not-a-uuid/console", "", 404},
		{http.MethodPost, "/v1/virtual-machines/actions", `{"action":"install","requestId":"11111111-1111-1111-1111-111111111111","uri":"test:///default"}`, 400},
		{http.MethodPost, "/v1/virtual-machines/actions", `{"action":"install","requestId":"11111111-1111-1111-1111-111111111111"}`, 403},
	} {
		r := systemServiceAgentRequest(s, test.method, test.path, test.body, true)
		if r.Code != test.status {
			t.Fatalf("%s %s = %d, want %d: %s", test.method, test.path, r.Code, test.status, r.Body.String())
		}
	}
}
