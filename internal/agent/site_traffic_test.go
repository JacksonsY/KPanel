package agent

import (
	"encoding/json"
	"fmt"
	"github.com/kejilion/kejilion-panel/internal/sitetraffic"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSiteTrafficCollectorHTTP(t *testing.T) {
	s := testServer(t)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "log/nginx"), 0700)
	s.siteTraffic = sitetraffic.NewReader(root)
	line := fmt.Sprintf("81.2.69.160 - - [%s] \"GET /account?token=private HTTP/1.1\" 403 12 \"-\" \"agent\"\n", time.Now().Format("02/Jan/2006:15:04:05 -0700"))
	os.WriteFile(filepath.Join(root, "log/nginx/access.log"), []byte(line), 0600)
	call := func(method, path, body string, auth bool) (int, string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if auth {
			req.Header.Set("Authorization", "Bearer "+strings.Repeat("x", 32))
		}
		out := httptest.NewRecorder()
		s.ServeHTTP(out, req)
		return out.Code, out.Body.String()
	}
	input := sitetraffic.CollectInput{Salt: strings.Repeat("ab", 32)}
	raw, _ := json.Marshal(input)
	if code, _ := call("POST", "/v1/site-traffic/collect", string(raw), false); code != 401 {
		t.Fatalf("anonymous %d", code)
	}
	if code, _ := call("GET", "/v1/site-traffic/collect", "", true); code != 405 {
		t.Fatalf("method %d", code)
	}
	if code, _ := call("POST", "/v1/site-traffic/collect?path=/etc/passwd", string(raw), true); code != 400 {
		t.Fatalf("query %d", code)
	}
	if code, _ := call("POST", "/v1/site-traffic/collect", `{"salt":"bad"}`, true); code != 422 {
		t.Fatalf("salt %d", code)
	}
	code, body := call("POST", "/v1/site-traffic/collect", string(raw), true)
	var batch sitetraffic.Batch
	if code != 200 || json.Unmarshal([]byte(body), &batch) != nil || len(batch.Records) != 1 || batch.Records[0].Kind != "access" || batch.WAFSource {
		t.Fatalf("collect %d %s", code, body)
	}
	if strings.Contains(body, "81.2.69.160") || strings.Contains(body, "private") {
		t.Fatal("unredacted source")
	}
	input.Cursors = batch.Cursors
	raw, _ = json.Marshal(input)
	_, body = call("POST", "/v1/site-traffic/collect", string(raw), true)
	json.Unmarshal([]byte(body), &batch)
	if len(batch.Records) != 0 {
		t.Fatal("replayed")
	}
}
