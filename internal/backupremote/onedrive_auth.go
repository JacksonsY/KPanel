package backupremote

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

type DeviceAuthorization struct {
	ID       string `json:"id"`
	Code     string `json:"userCode"`
	URL      string `json:"verificationUri"`
	Expires  int    `json:"expiresIn"`
	Interval int    `json:"interval"`
	Pending  bool   `json:"pending,omitempty"`
}
type deviceSession struct {
	storage  Storage
	revision string
	code     string
	expires  time.Time
	next     time.Time
	interval time.Duration
}

// At most eight short-lived authorizations. Device/refresh/access tokens never
// leave the server; only the user-facing code and an opaque handle are returned.
type OneDriveAuth struct {
	mu        sync.Mutex
	sessions  map[string]deviceSession
	transport http.RoundTripper // package-private test seam
}

func (a *OneDriveAuth) Start(ctx context.Context, revision string, storage Storage) (DeviceAuthorization, error) {
	storage.Kind = "onedrive"
	storage.Endpoint = graphEndpoint
	storage.Secret = "pending"
	storage.DriveID = "pending"
	storage.HasSecret = false
	if storage.TenantID == "" {
		storage.TenantID = "common"
	}
	if storage.ID == "" {
		storage.ID = backup.NewID()
	}
	if !backup.ValidID(revision) || storage.Validate() != nil {
		return DeviceAuthorization{}, ErrInvalid
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sessions == nil {
		a.sessions = map[string]deviceSession{}
	}
	for id, s := range a.sessions {
		if time.Now().After(s.expires) {
			delete(a.sessions, id)
		}
	}
	if len(a.sessions) >= MaxStorages {
		return DeviceAuthorization{}, ErrLimit
	}
	client, err := newClient(storage, a.transport)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	defer client.Close()
	var response struct {
		Code     string `json:"device_code"`
		User     string `json:"user_code"`
		URL      string `json:"verification_uri"`
		Expires  int    `json:"expires_in"`
		Interval int    `json:"interval"`
	}
	status, err := microsoftForm(ctx, client.oneDrive.http, storage.TenantID, "devicecode", url.Values{"client_id": {storage.ClientID}, "scope": {microsoftScope}}, &response)
	if err != nil || status != 200 || response.Code == "" || len(response.Code) > 8192 || response.User == "" || len(response.User) > 64 || controls(response.User) || response.Expires < 1 || response.Expires > 1800 || response.Interval > 60 {
		return DeviceAuthorization{}, ErrUnavailable
	}
	u, e := url.Parse(response.URL)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Host != "microsoft.com" && u.Host != "www.microsoft.com" && u.Host != "login.microsoftonline.com" {
		return DeviceAuthorization{}, ErrUnavailable
	}
	interval := max(response.Interval, 5)
	id := backup.NewID()
	a.sessions[id] = deviceSession{storage: storage, revision: revision, code: response.Code, expires: time.Now().Add(time.Duration(response.Expires) * time.Second), next: time.Now().Add(time.Duration(interval) * time.Second), interval: time.Duration(interval) * time.Second}
	return DeviceAuthorization{ID: id, Code: response.User, URL: response.URL, Expires: response.Expires, Interval: interval}, nil
}

func (a *OneDriveAuth) Poll(ctx context.Context, id string) (Storage, string, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[id]
	if !ok || time.Now().After(s.expires) {
		delete(a.sessions, id)
		return Storage{}, "", false, ErrInvalid
	}
	if time.Now().Before(s.next) {
		return Storage{}, "", true, nil
	}
	s.next = time.Now().Add(s.interval)
	a.sessions[id] = s
	client, err := newClient(s.storage, a.transport)
	if err != nil {
		return Storage{}, "", false, err
	}
	defer client.Close()
	var token microsoftToken
	status, err := microsoftForm(ctx, client.oneDrive.http, s.storage.TenantID, "token", url.Values{"client_id": {s.storage.ClientID}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "device_code": {s.code}}, &token)
	if err != nil {
		return Storage{}, "", false, err
	}
	if status == 400 && (token.Error == "authorization_pending" || token.Error == "slow_down") {
		if token.Error == "slow_down" {
			s.interval = min(s.interval+5*time.Second, time.Minute)
			s.next = time.Now().Add(s.interval)
			a.sessions[id] = s
		}
		return Storage{}, "", true, nil
	}
	delete(a.sessions, id)
	if status != 200 || token.Access == "" || len(token.Access) > 16384 || controls(token.Access) || token.Refresh == "" || len(token.Refresh) > 4096 || controls(token.Refresh) || token.Expires <= 60 {
		return Storage{}, "", false, ErrUnavailable
	}
	client.oneDrive.token = token.Access
	client.oneDrive.expires = time.Now().Add(time.Duration(token.Expires-60) * time.Second)
	root, _, err := client.oneDrive.metadata(ctx, graphEndpoint+"/me/drive/special/approot")
	if err != nil || root.Folder == nil || !validDriveID(root.Parent.Drive) {
		return Storage{}, "", false, ErrUnavailable
	}
	s.storage.DriveID = root.Parent.Drive
	s.storage.Secret = token.Refresh
	return s.storage, s.revision, false, nil
}

func (a *OneDriveAuth) Cancel(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, id)
}
