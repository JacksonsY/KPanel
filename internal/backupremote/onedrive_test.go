package backupremote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type graphTransport func(*http.Request) (*http.Response, error)

func (f graphTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func graphResponse(status int, v any) (*http.Response, error) {
	b, _ := json.Marshal(v)
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(b))}, nil
}
func oneDriveStorage() Storage {
	return Storage{ID: "0123456789abcdef0123456789abcdef", Name: "OneDrive", Kind: "onedrive", Endpoint: graphEndpoint, ClientID: "11111111-2222-3333-4444-555555555555", TenantID: "common", DriveID: "drive-fixture", Secret: "refresh-fixture"}
}

func TestOneDriveProtocolRoundtripAndCredentialIsolation(t *testing.T) {
	payload := bytes.Repeat([]byte("backup"), oneDriveChunk/6+123)
	var uploaded []byte
	chunks := 0
	rotations := 0
	transport := graphTransport(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "login.microsoftonline.com":
			if r.Header.Get("Authorization") != "" {
				t.Fatal("bearer forwarded to identity service")
			}
			_ = r.ParseForm()
			if r.Form.Get("refresh_token") != "refresh-fixture" || r.Form.Get("scope") != microsoftScope {
				t.Fatal("refresh request")
			}
			return graphResponse(200, map[string]any{"access_token": "access-fixture", "refresh_token": "rotated-fixture", "expires_in": 3600})
		case "fixture.up.1drv.com":
			if r.Header.Get("Authorization") != "" {
				t.Fatal("bearer forwarded to transfer service")
			}
			if r.Method == "PUT" {
				b, _ := io.ReadAll(r.Body)
				if r.Header.Get("Content-Range") != fmt.Sprintf("bytes %d-%d/%d", len(uploaded), len(uploaded)+len(b)-1, len(payload)) {
					t.Fatal("content range")
				}
				if int64(len(b)) != r.ContentLength {
					t.Fatal("content length")
				}
				uploaded = append(uploaded, b...)
				chunks++
				if len(uploaded) < len(payload) {
					if len(b)%327680 != 0 {
						t.Fatal("chunk alignment")
					}
					return graphResponse(202, map[string]any{"nextExpectedRanges": []string{fmt.Sprintf("%d-", len(uploaded))}})
				}
				return graphResponse(201, map[string]any{"size": len(uploaded), "file": map[string]any{}})
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(uploaded))}, nil
		case "graph.microsoft.com":
			if r.Header.Get("Authorization") != "Bearer access-fixture" {
				t.Fatal("graph authorization")
			}
			if strings.HasSuffix(r.URL.Path, "/special/approot") {
				return graphResponse(200, map[string]any{"id": "root", "folder": map[string]any{}, "parentReference": map[string]any{"driveId": "drive-fixture"}})
			}
			if strings.HasSuffix(r.URL.Path, "/createUploadSession") {
				return graphResponse(200, map[string]any{"uploadUrl": "https://fixture.up.1drv.com/upload"})
			}
			if strings.HasSuffix(r.URL.Path, "/children") {
				return graphResponse(200, map[string]any{"value": []any{map[string]any{"name": "test.kpb", "size": len(uploaded), "file": map[string]any{}}}})
			}
			if r.Method == "DELETE" {
				return graphResponse(204, nil)
			}
			return graphResponse(200, map[string]any{"name": "test.kpb", "size": len(uploaded), "file": map[string]any{}, "@microsoft.graph.downloadUrl": "https://fixture.up.1drv.com/download"})
		default:
			t.Fatal("unexpected host", r.URL.Host)
		}
		return nil, ErrUnavailable
	})
	c, err := newClient(oneDriveStorage(), transport)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetOneDriveTokenSaver(func(old, next string) error {
		if old != "refresh-fixture" || next != "rotated-fixture" {
			t.Fatal("rotation")
		}
		rotations++
		return nil
	})
	f, err := os.CreateTemp(t.TempDir(), "package")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, _ = f.Write(payload)
	_, _ = f.Seek(0, 0)
	ctx := context.Background()
	if err := c.Upload(ctx, "test.kpb", f, int64(len(payload))); err != nil {
		t.Fatal(err)
	}
	if chunks != 2 || rotations != 1 || !bytes.Equal(uploaded, payload) {
		t.Fatal("upload/rotation mismatch", chunks, rotations)
	}
	items, err := c.List(ctx)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	target := filepath.Join(t.TempDir(), "restored.kpb")
	n, err := c.Download(ctx, "test.kpb", target)
	if err != nil || n != int64(len(payload)) {
		t.Fatal(n, err)
	}
	got, _ := os.ReadFile(target)
	if !bytes.Equal(got, payload) {
		t.Fatal("download corrupted")
	}
	if err := c.Delete(ctx, "test.kpb"); err != nil {
		t.Fatal(err)
	}
}

func TestOneDriveDestinationAndURLBoundaries(t *testing.T) {
	for _, raw := range []string{"http://fixture.up.1drv.com/u", "https://1drv.com.evil.test/u", "https://evil.test/u", "https://user@fixture.up.1drv.com/u", "https://fixture.up.1drv.com:444/u", "https://127.0.0.1/u"} {
		if microsoftURL(raw, true) {
			t.Fatal("accepted transfer URL", raw)
		}
	}
	if microsoftURL("https://graph.microsoft.com.evil.test/v1.0/me", false) {
		t.Fatal("graph host escape")
	}
	s := oneDriveStorage()
	original := s.Fingerprint()
	s.Secret = "rotate"
	if s.Fingerprint() != original {
		t.Fatal("credential changed destination")
	}
	s.DriveID = "another"
	if s.Fingerprint() == original {
		t.Fatal("account switch did not change destination")
	}
	for _, prefix := range []string{"../backup", "bad:name", "trailing.", "a//b", "bad|name"} {
		s := oneDriveStorage()
		s.Prefix = prefix
		if s.Validate() == nil {
			t.Fatal("accepted prefix", prefix)
		}
	}
	public := oneDriveStorage().Public()
	if public.Secret != "" || !public.HasSecret {
		t.Fatal("public credential leak")
	}
}

func TestOneDriveRotationPersistsAndRejectsStaleCredentials(t *testing.T) {
	root := t.TempDir()
	store, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	s := oneDriveStorage()
	s.ID = ""
	if err := store.PutStorage(store.Snapshot().Revision, s); err != nil {
		t.Fatal(err)
	}
	s, err = store.Storage(store.Snapshot().Storages[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	revision := store.Snapshot().Revision
	if err := store.RotateOneDriveToken(s, s.Secret, "rotated"); err != nil {
		t.Fatal(err)
	}
	if store.Snapshot().Revision != revision {
		t.Fatal("rotation changed config revision")
	}
	if err := store.RotateOneDriveToken(s, s.Secret, "stale"); err != ErrConflict {
		t.Fatal(err)
	}
	reopened, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	v, err := reopened.Storage(s.ID)
	if err != nil || v.Secret != "rotated" {
		t.Fatal("rotation lost on restart", err)
	}
	onDisk, _ := os.ReadFile(filepath.Join(root, "settings.enc"))
	if bytes.Contains(onDisk, []byte("rotated")) {
		t.Fatal("plaintext credential")
	}
	v.ClientID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	v.Secret = ""
	if store.PutStorage(revision, v) != ErrInvalid {
		t.Fatal("reused secret across app registrations")
	}
}

func TestOneDriveDeviceAuthorizationPendingSlowdownAndCompletion(t *testing.T) {
	var a OneDriveAuth
	polls := 0
	a.transport = graphTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/devicecode") {
			return graphResponse(200, map[string]any{"device_code": "server-only-code", "user_code": "ABCD-EFGH", "verification_uri": "https://microsoft.com/devicelogin", "expires_in": 900, "interval": 5})
		}
		if strings.HasSuffix(r.URL.Path, "/token") {
			polls++
			if polls == 1 {
				return graphResponse(400, map[string]any{"error": "slow_down"})
			}
			return graphResponse(200, map[string]any{"access_token": "access", "refresh_token": "refresh", "expires_in": 3600})
		}
		return graphResponse(200, map[string]any{"id": "root", "folder": map[string]any{}, "parentReference": map[string]any{"driveId": "drive-fixture"}})
	})
	s := oneDriveStorage()
	s.ID = ""
	auth, err := a.Start(context.Background(), "0123456789abcdef0123456789abcdef", s)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(auth)
	if bytes.Contains(b, []byte("server-only-code")) {
		t.Fatal("device code exposed")
	}
	_, _, pending, err := a.Poll(context.Background(), auth.ID)
	if err != nil || !pending || polls != 0 {
		t.Fatal("interval not honored")
	}
	session := a.sessions[auth.ID]
	session.next = time.Now().Add(-time.Second)
	a.sessions[auth.ID] = session
	_, _, pending, err = a.Poll(context.Background(), auth.ID)
	if err != nil || !pending || a.sessions[auth.ID].interval != 10*time.Second {
		t.Fatal("slow_down")
	}
	session = a.sessions[auth.ID]
	session.next = time.Now().Add(-time.Second)
	a.sessions[auth.ID] = session
	storage, _, pending, err := a.Poll(context.Background(), auth.ID)
	if err != nil || pending || storage.Secret != "refresh" || storage.DriveID != "drive-fixture" {
		t.Fatal(storage.Public(), err)
	}
	if _, _, _, err := a.Poll(context.Background(), auth.ID); err != ErrInvalid {
		t.Fatal("replayed completed authorization")
	}
	auth, err = a.Start(context.Background(), "0123456789abcdef0123456789abcdef", s)
	if err != nil {
		t.Fatal(err)
	}
	a.Cancel(auth.ID)
	if _, _, _, err := a.Poll(context.Background(), auth.ID); err != ErrInvalid {
		t.Fatal("cancelled authorization reused")
	}
}

func TestOneDriveRejectsForeignPaginationAndFailedRotation(t *testing.T) {
	requests := 0
	c, err := newClient(oneDriveStorage(), graphTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.Host != "graph.microsoft.com" {
			t.Fatal("credential leaked via pagination")
		}
		return graphResponse(200, map[string]any{"value": []any{}, "@odata.nextLink": "https://evil.test/v1.0/page"})
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.oneDrive.folder = "root"
	c.oneDrive.token = "token"
	c.oneDrive.expires = time.Now().Add(time.Hour)
	if _, err := c.List(context.Background()); err != ErrUnavailable || requests != 1 {
		t.Fatal("foreign pagination", err, requests)
	}
	c.oneDrive.token = ""
	c.oneDrive.http.Transport = graphTransport(func(*http.Request) (*http.Response, error) {
		return graphResponse(200, map[string]any{"access_token": "token", "refresh_token": "rotated", "expires_in": 3600})
	})
	c.SetOneDriveTokenSaver(func(string, string) error { return ErrConflict })
	if _, err := c.oneDrive.access(context.Background()); err != ErrConflict || c.oneDrive.token != "" {
		t.Fatal("accepted nondurable rotation", err)
	}
}

func TestOneDriveFailedUploadCancelsSessionWithoutDeletingLocalPackage(t *testing.T) {
	deleted := false
	c, err := newClient(oneDriveStorage(), graphTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == "POST" {
			return graphResponse(200, map[string]any{"uploadUrl": "https://fixture.up.1drv.com/upload"})
		}
		if r.Method == "DELETE" {
			deleted = true
			return graphResponse(204, nil)
		}
		return graphResponse(503, map[string]any{"error": "unavailable"})
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.oneDrive.folder = "folder"
	c.oneDrive.token = "token"
	c.oneDrive.expires = time.Now().Add(time.Hour)
	f, err := os.CreateTemp(t.TempDir(), "package")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, _ = f.WriteString("package")
	_, _ = f.Seek(0, 0)
	if err := c.Upload(context.Background(), "test.kpb", f, 7); err != ErrUnavailable {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("failed upload session was not cancelled")
	}
	if info, err := f.Stat(); err != nil || info.Size() != 7 {
		t.Fatal("local package lost")
	}
}

func TestOneDriveRejectsExpiredAuthorizationAndAnotherDrive(t *testing.T) {
	a := OneDriveAuth{sessions: map[string]deviceSession{"expired": {expires: time.Now().Add(-time.Second)}}}
	if _, _, _, err := a.Poll(context.Background(), "expired"); err != ErrInvalid || len(a.sessions) != 0 {
		t.Fatal("expired authorization accepted")
	}
	c, err := newClient(oneDriveStorage(), graphTransport(func(*http.Request) (*http.Response, error) {
		return graphResponse(200, map[string]any{"id": "root", "folder": map[string]any{}, "parentReference": map[string]any{"driveId": "another-drive"}})
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.oneDrive.token = "token"
	c.oneDrive.expires = time.Now().Add(time.Hour)
	if _, err := c.List(context.Background()); err != ErrUnavailable {
		t.Fatal("silently switched destination", err)
	}
}
