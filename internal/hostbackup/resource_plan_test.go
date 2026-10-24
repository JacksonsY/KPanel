package hostbackup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

func restoreFailureCause(err error) error {
	var failure *backup.Failure
	if errors.As(err, &failure) {
		return failure.Err
	}
	return err
}

func volumePayload(t *testing.T) (*Engine, *fixtureDocker, Payload) {
	t.Helper()
	e, d, p := hostFixture(t)
	name := "planned-data"
	mount := "/var/lib/docker/volumes/" + name + "/_data"
	p.Volumes = map[string]Volume{name: {Name: name, Driver: "local", Mountpoint: mount, Labels: map[string]string{"application": "sample"}}}
	p.Containers[0].Mounts = append(p.Containers[0].Mounts, Mount{Type: "volume", Name: name, Source: mount, Destination: "/volume"})
	p.Containers[0].HostConfig["Binds"] = rawJSON([]string{"/home/app:/data:rw", name + ":/volume:rw"})
	p.Roots = append(p.Roots, Root{ID: resourceRootID(mount), Path: mount, Module: "apps", Directory: true})
	if err := os.MkdirAll(e.host(mount), 0700); err != nil {
		t.Fatal(err)
	}
	return e, d, p
}

func TestPayloadResourceGraphRejectsHiddenResources(t *testing.T) {
	e, _, p := hostFixture(t)
	p.Volumes = map[string]Volume{}
	for n := 0; n < 600; n++ {
		name := "orphan-" + strconv.Itoa(n)
		p.Volumes[name] = Volume{Name: name, Driver: "local", Mountpoint: "/tmp/" + name}
	}
	if err := e.validatePayload(p); err == nil {
		t.Fatal("isolated volumes accepted")
	}
	e, _, p = volumePayload(t)
	if err := e.validatePayload(p); err != nil {
		t.Fatal(err)
	}
	p.Networks = map[string]json.RawMessage{"hidden": rawJSON(map[string]string{"Name": "hidden"})}
	if err := e.validatePayload(p); err == nil {
		t.Fatal("isolated network accepted")
	}
}

func TestVolumeReviewUsesExecutionConfiguration(t *testing.T) {
	_, _, p := volumePayload(t)
	v := p.Volumes["planned-data"]
	v.Labels["io.kpanel.restore.job"] = "forged"
	p.Volumes[v.Name] = v
	review := moduleReview(p, payloadFacts{}, nil)
	if len(review.Volumes) != 1 {
		t.Fatal(review)
	}
	expected, _ := json.Marshal(volumeCreateBody(v))
	if string(review.Volumes[0].Config) != string(expected) || strings.Contains(string(expected), "forged") || strings.Contains(string(expected), "Mountpoint") {
		t.Fatal(string(review.Volumes[0].Config))
	}
}

func TestVolumeIntentPrecedesCreateAndLostResponseRollsBack(t *testing.T) {
	e, d, p := volumePayload(t)
	id := backup.NewID()
	dir := filepath.Join(t.TempDir(), id)
	if err := backup.PrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(dir, "restore-journal.json")
	j := restoreJournal{ID: id, Phase: "preparing"}
	e.Client.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == "POST" && r.URL.Path == "/volumes/create" {
			var saved restoreJournal
			data, err := os.ReadFile(journalPath)
			if err != nil || json.Unmarshal(data, &saved) != nil || len(saved.Volumes) != 1 {
				t.Fatal("volume creation preceded persistent intent", err)
			}
			response, err := d.request(r)
			if err != nil {
				return nil, err
			}
			response.Body.Close()
			return nil, errors.New("response lost")
		}
		return d.request(r)
	})
	if err := e.prepareVolumes(context.Background(), &p, &j, journalPath); err == nil {
		t.Fatal("lost response accepted")
	}
	if len(d.volumes) != 1 {
		t.Fatal("fixture did not create volume")
	}
	// Decode from disk to exercise crash recovery, not only in-memory rollback.
	var saved restoreJournal
	data, _ := os.ReadFile(journalPath)
	if json.Unmarshal(data, &saved) != nil {
		t.Fatal("journal")
	}
	for n := 0; n < 2; n++ {
		if err := e.Recover(context.Background(), id, dir); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.volumes) != 0 {
		t.Fatal("owned volume leaked")
	}
}

func TestVolumeRollbackDoesNotDeleteExistingOrReplaced(t *testing.T) {
	e, d, p := volumePayload(t)
	v := p.Volumes["planned-data"]
	d.volumes = map[string]Volume{v.Name: v}
	j := restoreJournal{ID: backup.NewID(), Phase: "preparing"}
	if err := e.prepareVolumes(context.Background(), &p, &j, filepath.Join(t.TempDir(), "restore-journal.json")); err != nil {
		t.Fatal(err)
	}
	if len(j.Volumes) != 0 {
		t.Fatal("existing volume claimed")
	}
	if err := e.rollback(context.Background(), &j); err != nil {
		t.Fatal(err)
	}
	j.Volumes = []restoreVolume{{Name: v.Name, Token: backup.NewID()}}
	if err := e.rollback(context.Background(), &j); err == nil {
		t.Fatal("changed ownership was claimed recovered")
	}
	if len(d.volumes) != 1 {
		t.Fatal("unrelated volume deleted")
	}
}

func TestLegacyForeignReviewRequiresReimport(t *testing.T) {
	dir := t.TempDir()
	body := rawJSON(ImportReview{Version: 1, Modules: []ModuleReview{{Module: "apps", Origin: OriginForeign}}})
	if err := os.WriteFile(filepath.Join(dir, reviewFile), body, 0600); err != nil {
		t.Fatal(err)
	}
	r, err := loadReview(dir, []string{"apps"})
	if err != nil || r.Complete([]string{"apps"}) {
		t.Fatal("old review silently accepted", r, err)
	}
}

func resourceRootID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:16])
}

func TestImageDeclaredVolumeCannotEscapePlan(t *testing.T) {
	e, d, p := hostFixture(t)
	e.Client.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.Path, "/images/") {
			body := rawJSON(map[string]any{"Id": p.Containers[0].ImageID, "Config": map[string]any{"Volumes": map[string]any{"/hidden": map[string]any{}}}})
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
		}
		return d.request(r)
	})
	if err := e.prepareImages(context.Background(), p.Containers); err == nil {
		t.Fatal("image-created anonymous volume accepted")
	}
	p.Containers[0].Mounts = append(p.Containers[0].Mounts, Mount{Type: "bind", Source: "/home/app", Destination: "/hidden"})
	if err := e.prepareImages(context.Background(), p.Containers); err != nil {
		t.Fatal("explicit reviewed mount rejected", err)
	}
}

func TestVolumeInspectFailureDoesNotCreate(t *testing.T) {
	e, d, p := volumePayload(t)
	e.Client.Transport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.Path, "/volumes/") {
			return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("unavailable")), Header: make(http.Header)}, nil
		}
		return d.request(r)
	})
	j := restoreJournal{ID: backup.NewID(), Phase: "preparing"}
	if err := e.prepareVolumes(context.Background(), &p, &j, filepath.Join(t.TempDir(), "restore-journal.json")); err == nil {
		t.Fatal("unavailable daemon accepted")
	}
	if len(j.Volumes) != 0 || len(d.volumes) != 0 {
		t.Fatal("failed inspection created/claimed a volume")
	}
}

func TestVolumeDataMustBeSelectedBeforePreparation(t *testing.T) {
	e, d, p := volumePayload(t)
	p.Roots = p.Roots[:1]
	dir := t.TempDir()
	if err := e.writePayload(context.Background(), filepath.Join(dir, "apps.payload"), p); err != nil {
		t.Fatal(err)
	}
	if err := e.Restore(context.Background(), backup.NewID(), dir, []string{"apps"}, true); err == nil {
		t.Fatal("incomplete selection accepted")
	}
	for _, call := range d.calls {
		if strings.HasPrefix(call, "POST ") {
			t.Fatal("incomplete plan caused side effect", call)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "restore-journal.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("incomplete plan advanced to transaction", err)
	}
}

func TestPreparationFailureDoesNotTouchPreexistingRecoveryPaths(t *testing.T) {
	e, _, p := hostFixture(t)
	id := backup.NewID()
	directory := t.TempDir()
	if err := e.writePayload(context.Background(), filepath.Join(directory, "apps.payload"), p); err != nil {
		t.Fatal(err)
	}
	next := e.host("/home/app") + ".kpanel-next-" + id
	if err := os.WriteFile(next, []byte("must survive"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.Restore(context.Background(), id, directory, []string{"apps"}, true); err == nil {
		t.Fatal("preexisting recovery path accepted")
	}
	data, err := os.ReadFile(next)
	if err != nil || string(data) != "must survive" {
		t.Fatal("preflight rollback touched unrelated path", err)
	}
}
