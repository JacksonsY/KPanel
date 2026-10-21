package panel

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/backup"
	"github.com/kejilion/kejilion-panel/internal/backupremote"
)

func TestOneDriveAuthorizationRequiresSessionOriginCSRFAndRevision(t *testing.T) {
	s, token := newTestServer(t)
	session, csrf := bootstrapCookies(t, s, token)
	request := func(path, revision string, authorized bool) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"revision": revision, "storage": map[string]string{"name": "OneDrive", "clientId": "invalid-client"}})
		r := httptest.NewRequest("POST", "/api/v1/backups/"+path, bytes.NewReader(body))
		r.Host = "panel.test"
		r.Header.Set("Content-Type", "application/json")
		if authorized {
			r.AddCookie(session)
			r.AddCookie(csrf)
			r.Header.Set("Origin", "http://panel.test")
			r.Header.Set("X-CSRF-Token", csrf.Value)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	if w := request("onedrive-auth", s.backupRemote.Snapshot().Revision, false); w.Code != 401 && w.Code != 403 {
		t.Fatal("unauthorized start", w.Code)
	}
	if w := request("onedrive-auth", backup.NewID(), true); w.Code != 409 {
		t.Fatal("stale configuration", w.Code, w.Body.String())
	}
	if w := request("onedrive-auth", s.backupRemote.Snapshot().Revision, true); w.Code != 400 {
		t.Fatal("invalid application ID", w.Code, w.Body.String())
	}
	if w := request("onedrive-auth/"+backup.NewID(), "", true); w.Code != 400 {
		t.Fatal("unknown authorization", w.Code, w.Body.String())
	}
}

func TestOneDriveClientConstructionAndPublicSettings(t *testing.T) {
	s, _ := newTestServer(t)
	storage := backupremote.Storage{Name: "OneDrive", Kind: "onedrive", Endpoint: "https://graph.microsoft.com/v1.0", ClientID: "11111111-2222-3333-4444-555555555555", TenantID: "common", DriveID: "fixture", Secret: "refresh"}
	if err := s.backupRemote.PutStorage(s.backupRemote.Snapshot().Revision, storage); err != nil {
		t.Fatal(err)
	}
	storage, err := s.backupRemote.Storage(s.backupRemote.Snapshot().Storages[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	client, err := s.newBackupRemoteClient(storage)
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	// Settings expose only the presence of the encrypted credential.
	public := s.backupRemote.Snapshot().Storages[0]
	if public.Secret != "" || !public.HasSecret {
		t.Fatal("credential disclosure")
	}
}
