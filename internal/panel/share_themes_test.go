package panel

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/scenepacks"
)

func TestShareThemeUploadHTTP(t *testing.T) {
	s, tokenPath := newTestServer(t)
	s.shareThemes.Close()
	s.shareThemes = scenepacks.OpenShareThemes(t.TempDir(), func(context.Context, string, int64) ([]byte, error) { return nil, scenepacks.ErrSource })
	session, csrf := bootstrapCookies(t, s, tokenPath)
	list, err := s.shareThemes.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	for _, name := range []string{"manifest.json", "index.html", "theme.css", "theme.js"} {
		body, err := os.ReadFile(filepath.Join("../../share-themes/minimal/dist", name))
		if err != nil {
			t.Fatal(err)
		}
		f, _ := z.Create(name)
		_, _ = f.Write(body)
	}
	_ = z.Close()
	var body bytes.Buffer
	m := multipart.NewWriter(&body)
	_ = m.WriteField("expectedResourceVersion", list.ResourceVersion)
	f, _ := m.CreateFormFile("file", "theme.zip")
	_, _ = f.Write(archive.Bytes())
	_ = m.Close()
	headers := map[string]string{"Origin": "http://panel.test", "Content-Type": m.FormDataContentType(), "X-CSRF-Token": csrf.Value}
	url := shareThemesPath + "/upload"
	if r := performRequest(s, "POST", url, body.Bytes(), headers); r.Code != 401 {
		t.Fatal("anonymous upload", r.Code)
	}
	for _, denied := range []map[string]string{
		{"Origin": "https://evil.example", "Content-Type": m.FormDataContentType(), "X-CSRF-Token": csrf.Value},
		{"Origin": "http://panel.test", "Content-Type": m.FormDataContentType()},
	} {
		if r := authenticatedRequest(s, "POST", url, body.Bytes(), session, csrf, denied); r.Code != 403 {
			t.Fatal("upload boundary", r.Code)
		}
	}
	r := authenticatedRequest(s, "POST", url, body.Bytes(), session, csrf, headers)
	var installed scenepacks.View
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &installed) != nil || !installed.Uploaded || installed.FileBase == nil {
		t.Fatal("upload", r.Code, r.Body.String())
	}
	if s.shareThemes.ActiveTheme() != nil {
		t.Fatal("upload unexpectedly selected theme")
	}
	r = performRequest(s, "GET", *installed.FileBase+"index.html", nil, nil)
	if r.Code != 200 || !strings.Contains(r.Header().Get("Content-Security-Policy"), "sandbox allow-scripts") || strings.Contains(r.Header().Get("Content-Security-Policy"), "allow-same-origin") {
		t.Fatal("upload sandbox", r.Code)
	}
	if r := authenticatedRequest(s, "POST", url, body.Bytes(), session, csrf, headers); r.Code != 409 {
		t.Fatal("stale upload", r.Code)
	}
	if r := authenticatedRequest(s, "POST", scenePacksPath+"/upload", body.Bytes(), session, csrf, headers); r.Code != 405 {
		t.Fatal("scene upload enabled", r.Code)
	}
}

func TestShareThemeHTTPBoundaryAndPublicLifecycle(t *testing.T) {
	s, tokenPath := newTestServer(t)
	s.shareThemes.Close()
	s.shareThemes = scenepacks.OpenShareThemes(t.TempDir(), func(_ context.Context, address string, _ int64) ([]byte, error) {
		const prefix = "https://raw.githubusercontent.com/kejilion/KPanel/main/share-themes/"
		if !strings.HasPrefix(address, prefix) {
			return nil, scenepacks.ErrSource
		}
		name := strings.TrimPrefix(address, prefix)
		if !filepath.IsLocal(name) || strings.Contains(name, "..") {
			return nil, scenepacks.ErrSource
		}
		return os.ReadFile(filepath.Join("../../share-themes", name))
	})
	session, csrf := bootstrapCookies(t, s, tokenPath)
	headers := map[string]string{"Origin": "http://panel.test", "Content-Type": "application/json", "X-CSRF-Token": csrf.Value}
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		return authenticatedRequest(s, method, path, data, session, csrf, headers)
	}
	if r := performRequest(s, "GET", shareThemesPath, nil, nil); r.Code != http.StatusUnauthorized {
		t.Fatal("public catalog", r.Code)
	}
	var list scenepacks.List
	readList := func() {
		r := call("GET", shareThemesPath, nil)
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &list) != nil {
			t.Fatal("catalog", r.Code, r.Body.String())
		}
	}
	readList()
	p := list.Packs[0]
	input := map[string]string{"expectedResourceVersion": p.ResourceVersion}
	body, _ := json.Marshal(input)
	for _, denied := range []map[string]string{
		{"Origin": "https://evil.example", "Content-Type": "application/json", "X-CSRF-Token": csrf.Value},
		{"Origin": "http://panel.test", "Content-Type": "application/json"},
	} {
		if r := authenticatedRequest(s, "POST", shareThemesPath+"/"+p.ID+"/install", body, session, csrf, denied); r.Code != 403 {
			t.Fatal("write boundary", r.Code)
		}
	}
	if r := call("PUT", shareThemesPath+"/source", map[string]string{"source": "http://127.0.0.1"}); r.Code != 400 {
		t.Fatal("arbitrary download source")
	}
	r := call("POST", shareThemesPath+"/"+p.ID+"/install", input)
	if r.Code != 200 {
		t.Fatal("install", r.Code, r.Body.String())
	}
	var installed scenepacks.View
	if json.Unmarshal(r.Body.Bytes(), &installed) != nil || installed.FileBase == nil {
		t.Fatal("missing file base")
	}
	fileURL := *installed.FileBase + "index.html"
	r = performRequest(s, "GET", fileURL, nil, nil)
	if r.Code != 200 {
		t.Fatal("anonymous artwork", r.Code)
	}
	csp := r.Header().Get("Content-Security-Policy")
	for _, value := range []string{"sandbox allow-scripts", "default-src 'none'", "frame-ancestors 'self'", "form-action 'none'", *installed.FileBase} {
		if !strings.Contains(csp, value) {
			t.Fatal("CSP missing", value, csp)
		}
	}
	if strings.Contains(csp, "allow-same-origin") || r.Header().Get("Referrer-Policy") != "no-referrer" || !securityEntrancePublicPath(fileURL) {
		t.Fatal("theme isolation/entrance")
	}
	readList()
	selection := map[string]string{"selected": p.ID, "expectedResourceVersion": list.ResourceVersion}
	if r := call("PUT", shareThemesPath+"/selection", selection); r.Code != 200 {
		t.Fatal("apply", r.Code, r.Body.String())
	}
	if r := call("PUT", shareThemesPath+"/selection", selection); r.Code != 409 {
		t.Fatal("stale apply", r.Code)
	}
	settings, version := s.store.ClusterShare()
	settings.Enabled = true
	settings.Token = strings.Repeat("a", 64)
	if err := s.store.ReplaceClusterShare(version, settings); err != nil {
		t.Fatal(err)
	}
	publicURL := clusterShareAPIPrefix + settings.Token
	r = performRequest(s, "GET", publicURL, nil, nil)
	var snapshot publicClusterShareSnapshot
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &snapshot) != nil || snapshot.Theme == nil || snapshot.Theme.ID != p.ID {
		t.Fatal("public theme", r.Code, r.Body.String())
	}
	for _, forbidden := range []string{"resourceVersion", "installedVersion", settings.Token, "csrf"} {
		if strings.Contains(r.Body.String(), forbidden) {
			t.Fatal("private data leaked", forbidden)
		}
	}
	if r := call("DELETE", shareThemesPath+"/"+p.ID, map[string]string{"expectedResourceVersion": installed.ResourceVersion}); r.Code != 204 {
		t.Fatal("delete", r.Code)
	}
	r = performRequest(s, "GET", publicURL, nil, nil)
	if strings.Contains(r.Body.String(), `"theme"`) {
		t.Fatal("cached snapshot kept deleted theme")
	}
	if r := performRequest(s, "GET", fileURL, nil, nil); r.Code != 404 {
		t.Fatal("deleted file still public")
	}
	settings, version = s.store.ClusterShare()
	settings.Enabled = false
	if err := s.store.ReplaceClusterShare(version, settings); err != nil {
		t.Fatal(err)
	}
	if r := performRequest(s, "GET", publicURL, nil, nil); r.Code != 404 {
		t.Fatal("closed sharing still readable")
	}
}
