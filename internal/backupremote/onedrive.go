package backupremote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

const graphEndpoint = "https://graph.microsoft.com/v1.0"
const microsoftScope = "offline_access https://graph.microsoft.com/Files.ReadWrite.AppFolder"
const oneDriveChunk = 10 * 1024 * 1024 // 32 * 320 KiB; below Graph's 60 MiB limit.

func validMicrosoftID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
func validTenant(s string) bool {
	return s == "common" || s == "consumers" || s == "organizations" || validMicrosoftID(s)
}
func validDriveID(s string) bool {
	return s != "" && len(s) <= 256 && !controls(s) && !strings.ContainsAny(s, "/\\%?#:")
}

// Graph credentials are sent only to Graph. Preauthenticated upload/download
// URLs come from Graph and receive no bearer token. No redirects are followed.
func microsoftURL(raw string, transfer bool) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.Fragment != "" || u.Port() != "" && u.Port() != "443" || controls(raw) || strings.Contains(raw, "\\") {
		return false
	}
	if !transfer {
		return u.Host == "graph.microsoft.com" && strings.HasPrefix(u.Path, "/v1.0/")
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range []string{"1drv.com", "onedrive.com", "sharepoint.com", "storage.live.com", "microsoftpersonalcontent.com"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

type oneDriveClient struct {
	storage Storage
	http    *http.Client
	mu      sync.Mutex
	token   string
	expires time.Time
	folder  string
	rotate  func(string, string) error
}

func newOneDriveClient(s Storage, transport http.RoundTripper) *oneDriveClient {
	return &oneDriveClient{storage: s, http: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrUnavailable }}}
}

// Persist refresh-token rotation before accepting the refreshed access token.
func (c *Client) SetOneDriveTokenSaver(save func(string, string) error) {
	if c.oneDrive != nil {
		c.oneDrive.rotate = save
	}
}

type microsoftToken struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
	Expires int    `json:"expires_in"`
	Error   string `json:"error"`
}

func microsoftForm(ctx context.Context, client *http.Client, tenant, action string, values url.Values, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", "https://login.microsoftonline.com/"+tenant+"/oauth2/v2.0/"+action, strings.NewReader(values.Encode()))
	if err != nil {
		return 0, ErrInvalid
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return 0, ErrUnavailable
	}
	defer res.Body.Close()
	return res.StatusCode, decodeMicrosoft(res.Body, out)
}
func decodeMicrosoft(body io.Reader, out any) error {
	b, err := io.ReadAll(io.LimitReader(body, (1<<20)+1))
	if err != nil || len(b) > 1<<20 || json.Unmarshal(b, out) != nil {
		return ErrUnavailable
	}
	return nil
}

func (c *oneDriveClient) access(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expires) {
		return c.token, nil
	}
	var token microsoftToken
	status, err := microsoftForm(ctx, c.http, c.storage.TenantID, "token", url.Values{"client_id": {c.storage.ClientID}, "grant_type": {"refresh_token"}, "refresh_token": {c.storage.Secret}, "scope": {microsoftScope}}, &token)
	if err != nil || status != 200 || token.Access == "" || token.Expires <= 60 || len(token.Access) > 16384 || controls(token.Access) {
		return "", ErrUnavailable
	}
	if token.Refresh != "" && token.Refresh != c.storage.Secret {
		if len(token.Refresh) > 4096 || controls(token.Refresh) || c.rotate == nil {
			return "", ErrUnavailable
		}
		if err := c.rotate(c.storage.Secret, token.Refresh); err != nil {
			return "", err
		}
		c.storage.Secret = token.Refresh
	}
	c.token = token.Access
	c.expires = time.Now().Add(time.Duration(token.Expires-60) * time.Second)
	return c.token, nil
}

func (c *oneDriveClient) request(ctx context.Context, method, raw string, body io.Reader, size int64, transfer bool) (*http.Response, error) {
	if !microsoftURL(raw, transfer) {
		return nil, ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, method, raw, body)
	if err != nil {
		return nil, ErrUnavailable
	}
	req.ContentLength = size
	if !transfer {
		token, err := c.access(ctx)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, ErrUnavailable
	}
	return res, nil
}

type driveItem struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Size     int64            `json:"size"`
	Modified time.Time        `json:"lastModifiedDateTime"`
	Folder   *json.RawMessage `json:"folder"`
	File     *json.RawMessage `json:"file"`
	Download string           `json:"@microsoft.graph.downloadUrl"`
	Parent   struct {
		Drive string `json:"driveId"`
	} `json:"parentReference"`
}

func (v driveItem) object() Object { return Object{Key: v.Name, Size: v.Size, Modified: v.Modified} }
func (c *oneDriveClient) metadata(ctx context.Context, raw string) (driveItem, int, error) {
	res, err := c.request(ctx, "GET", raw, nil, 0, false)
	if err != nil {
		return driveItem{}, 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return driveItem{}, res.StatusCode, ErrUnavailable
	}
	var item driveItem
	err = decodeMicrosoft(res.Body, &item)
	return item, res.StatusCode, err
}
func itemPath(id string) string { return graphEndpoint + "/me/drive/items/" + url.PathEscape(id) }

func (c *oneDriveClient) root(ctx context.Context) (string, error) {
	if c.folder != "" {
		return c.folder, nil
	}
	item, _, err := c.metadata(ctx, graphEndpoint+"/me/drive/special/approot")
	if err != nil || item.ID == "" || item.Folder == nil || item.Parent.Drive != c.storage.DriveID {
		return "", ErrUnavailable
	}
	for _, part := range strings.Split(c.storage.Prefix, "/") {
		if part == "" {
			continue
		}
		raw := itemPath(item.ID) + ":/" + url.PathEscape(part)
		next, status, err := c.metadata(ctx, raw)
		if status == 404 {
			b, _ := json.Marshal(map[string]any{"name": part, "folder": map[string]any{}, "@microsoft.graph.conflictBehavior": "fail"})
			res, e := c.request(ctx, "POST", itemPath(item.ID)+"/children", bytes.NewReader(b), int64(len(b)), false)
			if e != nil {
				return "", e
			}
			status = res.StatusCode
			if status == 201 {
				err = decodeMicrosoft(res.Body, &next)
			}
			res.Body.Close()
			if status == 409 {
				next, _, err = c.metadata(ctx, raw)
			} else if status != 201 {
				err = ErrUnavailable
			}
		}
		if err != nil || next.ID == "" || next.Folder == nil {
			return "", ErrUnavailable
		}
		item = next
	}
	c.folder = item.ID
	return item.ID, nil
}
func (c *oneDriveClient) objectURL(ctx context.Context, name string) (string, error) {
	id, err := c.root(ctx)
	if err != nil {
		return "", err
	}
	return itemPath(id) + ":/" + url.PathEscape(name), nil
}
func (c *oneDriveClient) stat(ctx context.Context, name string) (Object, error) {
	raw, err := c.objectURL(ctx, name)
	if err != nil {
		return Object{}, err
	}
	v, _, err := c.metadata(ctx, raw)
	if err != nil || v.File == nil {
		return Object{}, ErrUnavailable
	}
	return v.object(), nil
}
func (c *oneDriveClient) get(ctx context.Context, name string) (io.ReadCloser, error) {
	raw, err := c.objectURL(ctx, name)
	if err != nil {
		return nil, err
	}
	v, _, err := c.metadata(ctx, raw)
	if err != nil || v.File == nil {
		return nil, ErrUnavailable
	}
	res, err := c.request(ctx, "GET", v.Download, nil, 0, true)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 || res.Header.Get("Content-Encoding") != "" && res.Header.Get("Content-Encoding") != "identity" {
		res.Body.Close()
		return nil, ErrUnavailable
	}
	return res.Body, nil
}
func (c *oneDriveClient) remove(ctx context.Context, name string) error {
	raw, err := c.objectURL(ctx, name)
	if err != nil {
		return err
	}
	res, err := c.request(ctx, "DELETE", raw, nil, 0, false)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 204 && res.StatusCode != 404 {
		return ErrUnavailable
	}
	return nil
}
func (c *oneDriveClient) list(ctx context.Context) ([]Object, error) {
	id, err := c.root(ctx)
	if err != nil {
		return nil, err
	}
	next := itemPath(id) + "/children?$top=200"
	result := []Object{}
	count := 0
	for pages := 0; next != ""; pages++ {
		if pages >= MaxObjects {
			return nil, ErrLimit
		}
		res, err := c.request(ctx, "GET", next, nil, 0, false)
		if err != nil {
			return nil, err
		}
		var page struct {
			Items []driveItem `json:"value"`
			Next  string      `json:"@odata.nextLink"`
		}
		err = decodeMicrosoft(res.Body, &page)
		res.Body.Close()
		if err != nil || res.StatusCode != 200 {
			return nil, ErrUnavailable
		}
		count += len(page.Items)
		if count > MaxObjects {
			return nil, ErrLimit
		}
		for _, item := range page.Items {
			if item.File != nil && ValidObject(item.Name) && item.Size > 0 && item.Size <= backup.MaxEncryptedBytes {
				result = append(result, item.object())
			}
		}
		next = page.Next
	}
	return result, nil
}
func (c *oneDriveClient) put(ctx context.Context, name string, reader io.Reader, size int64) (result error) {
	raw, err := c.objectURL(ctx, name)
	if err != nil {
		return err
	}
	b := []byte(`{"item":{"@microsoft.graph.conflictBehavior":"replace"}}`)
	res, err := c.request(ctx, "POST", raw+":/createUploadSession", bytes.NewReader(b), int64(len(b)), false)
	if err != nil {
		return err
	}
	var session struct {
		URL string `json:"uploadUrl"`
	}
	err = decodeMicrosoft(res.Body, &session)
	res.Body.Close()
	if err != nil || res.StatusCode != 200 || !microsoftURL(session.URL, true) {
		return ErrUnavailable
	}
	defer func() {
		if result == nil {
			return
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if res, e := c.request(cleanup, "DELETE", session.URL, nil, 0, true); e == nil {
			res.Body.Close()
		}
	}()
	buffer := make([]byte, oneDriveChunk)
	for offset := int64(0); offset < size; {
		length := min(int64(len(buffer)), size-offset)
		if _, err := io.ReadFull(reader, buffer[:length]); err != nil {
			return ErrUnavailable
		}
		req, err := http.NewRequestWithContext(ctx, "PUT", session.URL, bytes.NewReader(buffer[:length]))
		if err != nil {
			return ErrUnavailable
		}
		req.ContentLength = length
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, offset+length-1, size))
		res, err := c.http.Do(req)
		if err != nil {
			return ErrUnavailable
		}
		if offset+length == size {
			var item driveItem
			err = decodeMicrosoft(res.Body, &item)
			res.Body.Close()
			if err != nil || res.StatusCode != 200 && res.StatusCode != 201 || item.Size != size || item.File == nil {
				return ErrUnavailable
			}
		} else {
			var state struct {
				Ranges []string `json:"nextExpectedRanges"`
			}
			err = decodeMicrosoft(res.Body, &state)
			res.Body.Close()
			if err != nil || res.StatusCode != 202 || len(state.Ranges) != 1 || state.Ranges[0] != fmt.Sprintf("%d-", offset+length) {
				return ErrUnavailable
			}
		}
		offset += length
	}
	return nil
}
