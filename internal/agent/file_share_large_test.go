package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/contract"
	"github.com/kejilion/kejilion-panel/internal/filemanager"
)

func TestLargeFileShareContentSupportsHeadRangeAndRevalidation(t *testing.T) {
	size := int64(512<<20) + 1
	if os.Getenv("KPANEL_TEST_MAX_SHARE") == "1" {
		size = contract.MaxFileShareBytes
	}
	root := t.TempDir()
	filePath := filepath.Join(root, "large.iso")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(size); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("tail"), size-4); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	manager, err := filemanager.New(filemanager.Config{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	server := testServer(t)
	server.files = manager
	response := fileRequest(server, http.MethodGet, "/v1/files/share-entry?path=%2Flarge.iso", "")
	var entry contract.FileShareEntry
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &entry) != nil || entry.SizeBytes != size || entry.ShareVersion == "" {
		t.Fatalf("large share identity = %d %s", response.Code, response.Body.String())
	}
	headers := map[string]string{
		"If-Match":                      `"` + entry.ResourceVersion + `"`,
		contract.FileShareVersionHeader: entry.ShareVersion,
	}
	target := "/v1/files/share-content?path=%2Flarge.iso&disposition=attachment"
	head := fileRequestWithHeaders(server, http.MethodHead, target, "", headers)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != fmt.Sprint(size) {
		t.Fatalf("large HEAD = %d length=%s", head.Code, head.Header().Get("Content-Length"))
	}
	headers["Range"] = "bytes=-4"
	resumed := fileRequestWithHeaders(server, http.MethodGet, target, "", headers)
	if resumed.Code != http.StatusPartialContent || resumed.Body.String() != "tail" ||
		resumed.Header().Get("Content-Range") != fmt.Sprintf("bytes %d-%d/%d", size-4, size-1, size) {
		t.Fatalf("large resume = %d %q range=%s", resumed.Code, resumed.Body.String(), resumed.Header().Get("Content-Range"))
	}
	delete(headers, "Range")
	headers["If-None-Match"] = `"` + entry.ResourceVersion + `"`
	unchanged := fileRequestWithHeaders(server, http.MethodGet, target, "", headers)
	if unchanged.Code != http.StatusNotModified || unchanged.Body.Len() != 0 {
		t.Fatalf("large revalidation = %d", unchanged.Code)
	}
	// A same-length rewrite with restored mtime must still invalidate the share.
	file, err = os.OpenFile(filePath, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteAt([]byte("evil"), size-4)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("rewrite: %v %v", err, closeErr)
	}
	if err := os.Chtimes(filePath, entry.ModifiedAt, entry.ModifiedAt); err != nil {
		t.Fatal(err)
	}
	delete(headers, "If-None-Match")
	headers["Range"] = "bytes=-4"
	changed := fileRequestWithHeaders(server, http.MethodGet, target, "", headers)
	if changed.Code != http.StatusPreconditionFailed || changed.Body.Len() != 0 {
		t.Fatalf("rewritten large share leaked bytes: %d %q", changed.Code, changed.Body.String())
	}
	t.Logf("verified sparse file size=%d: identity, HEAD, resumed suffix, 304, rewrite rejection", size)
}
