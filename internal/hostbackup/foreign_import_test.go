package hostbackup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

func rawJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

// otherHost exports with the same filesystem and Docker fixture but its own
// Agent state, as a package made on another machine would be.
func otherHost(t *testing.T, e *Engine) *Engine {
	t.Helper()
	return &Engine{Root: e.Root, StateDir: t.TempDir(), Client: e.Client}
}

func writeFixturePayload(t *testing.T, e *Engine, p Payload) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), p.Module+".payload")
	if err := e.writePayload(context.Background(), path, p); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFacts(t *testing.T, e *Engine, path string) payloadFacts {
	t.Helper()
	_, facts, err := e.readPayload(context.Background(), path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return facts
}

func TestPayloadOriginOnlyTrustsThisAgentsExports(t *testing.T) {
	e, _, p := hostFixture(t)
	local := writeFixturePayload(t, e, p)
	if !readFacts(t, e, local).Local {
		t.Fatal("this Agent's own export was not recognised as local")
	}
	if readFacts(t, otherHost(t, e), local).Local {
		t.Fatal("another Agent accepted this Agent's export as its own")
	}
	foreign := writeFixturePayload(t, otherHost(t, e), p)
	if readFacts(t, e, foreign).Local {
		t.Fatal("another host's export was accepted as local")
	}

	original, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	forged := func(name string, data []byte) string {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	// A changed tag byte must not verify.
	edited := bytes.Clone(original)
	edited[originExtraOffset+5] ^= 0x01
	if readFacts(t, e, forged("edited.payload", edited)).Local {
		t.Fatal("a modified origin tag was accepted")
	}
	// A genuine tag moved onto different content must not verify either.
	other := p
	other.Containers = append([]Container(nil), p.Containers...)
	other.Containers[0].Config = map[string]json.RawMessage{"Image": rawJSON("example/app:other"), "Labels": rawJSON(map[string]string{})}
	moved, err := os.ReadFile(writeFixturePayload(t, otherHost(t, e), other))
	if err != nil {
		t.Fatal(err)
	}
	copy(moved[originExtraOffset:originExtraOffset+originExtraBytes], original[originExtraOffset:originExtraOffset+originExtraBytes])
	if readFacts(t, e, forged("moved.payload", moved)).Local {
		t.Fatal("an origin tag replayed onto other content was accepted")
	}
	// Packages written before origin tags existed, or with an invented
	// subfield, are foreign.
	reader, err := gzip.NewReader(bytes.NewReader(original))
	if err != nil {
		t.Fatal(err)
	}
	stream, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	for name, extra := range map[string][]byte{"untagged": nil, "invented": append(originPlaceholder()[:5], bytes.Repeat([]byte{7}, sha256.Size)...)} {
		var out bytes.Buffer
		gz := gzip.NewWriter(&out)
		gz.Extra = extra
		_, _ = gz.Write(stream)
		_ = gz.Close()
		if readFacts(t, e, forged(name+".payload", out.Bytes())).Local {
			t.Fatalf("%s payload was accepted as local", name)
		}
	}
}

func TestOriginKeyIsCreatedOnceAndKept(t *testing.T) {
	e := &Engine{StateDir: t.TempDir()}
	if key, err := e.originKey(false); err != nil || key != nil {
		t.Fatalf("reading without a key created one: %v %v", key, err)
	}
	first, err := e.originKey(true)
	if err != nil || len(first) != originKeyBytes {
		t.Fatal(first, err)
	}
	again, err := e.originKey(true)
	if err != nil || !bytes.Equal(first, again) {
		t.Fatal("origin key was replaced")
	}
	path, _ := e.originKeyPath()
	if info, err := os.Stat(path); err != nil || info.Size() != originKeyBytes {
		t.Fatal(err)
	}
}

// hostReachingPayload is the fixture application with host-reaching runtime
// settings, as a package from elsewhere could declare.
func hostReachingPayload(p Payload) Payload {
	c := p.Containers[0]
	c.HostConfig = map[string]json.RawMessage{
		"Binds":             rawJSON([]string{"/home/app:/data:rw", "/:/host:rw", "/etc:/host-etc:ro"}),
		"Privileged":        rawJSON(true),
		"CapAdd":            rawJSON([]string{"SYS_ADMIN", "NET_ADMIN"}),
		"Devices":           rawJSON([]map[string]string{{"PathOnHost": "/dev/sda", "PathInContainer": "/dev/sda", "CgroupPermissions": "rwm"}}),
		"DeviceCgroupRules": rawJSON([]string{"b *:* rwm"}),
		"PidMode":           rawJSON("host"),
		"IpcMode":           rawJSON("host"),
		"NetworkMode":       rawJSON("host"),
		"SecurityOpt":       rawJSON([]string{"apparmor=unconfined"}),
	}
	c.Mounts = []Mount{
		{Type: "bind", Source: "/home/app", Destination: "/data", RW: true},
		{Type: "bind", Source: "/", Destination: "/host", RW: true},
		{Type: "bind", Source: "/etc", Destination: "/host-etc"},
	}
	p.Containers = []Container{c}
	return p
}

func TestForeignReviewListsHostConfiguration(t *testing.T) {
	e, _, p := hostFixture(t)
	p = hostReachingPayload(p)
	p.Networks = map[string]json.RawMessage{"lan": rawJSON(map[string]any{"Driver": "macvlan", "IPAM": map[string]any{"Config": []map[string]string{{"Subnet": "192.168.1.0/24", "Gateway": "192.168.1.1"}}}, "Options": map[string]string{"parent": "eth0"}})}
	p.Containers[0].Networks = map[string]json.RawMessage{}
	path := writeFixturePayload(t, otherHost(t, e), p)
	read, facts, err := e.readPayload(context.Background(), path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	review := ImportReview{Modules: []ModuleReview{moduleReview(read, facts, []string{"/home/app"})}}
	current, err := e.Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	e.annotateReview(&review, current)
	m := review.Modules[0]
	if m.Origin != OriginForeign || !review.Foreign([]string{"apps"}) {
		t.Fatal("foreign module not marked", m.Origin)
	}
	c := m.Containers[0]
	if !c.Privileged || !slices.Equal(c.CapAdd, []string{"SYS_ADMIN", "NET_ADMIN"}) || !slices.Equal(c.HostNamespaces, []string{"network", "pid", "ipc"}) || !slices.Equal(c.SecurityOpt, []string{"apparmor=unconfined"}) || !c.Replaces {
		t.Fatalf("host settings missing from review: %+v", c)
	}
	if !slices.Equal(c.Devices, []string{"/dev/sda:/dev/sda:rwm", "cgroup b *:* rwm"}) {
		t.Fatalf("devices missing from review: %v", c.Devices)
	}
	want := []BindReview{{Source: "/home/app", Destination: "/data"}, {Source: "/", Destination: "/host", Live: true}, {Source: "/etc", Destination: "/host-etc", ReadOnly: true, Live: true}}
	if !slices.Equal(c.Binds, want) {
		t.Fatalf("binds = %+v, want %+v", c.Binds, want)
	}
	if len(m.Networks) != 1 || m.Networks[0].Driver != "macvlan" || !slices.Equal(m.Networks[0].Subnets, []string{"192.168.1.0/24 gw 192.168.1.1"}) || !slices.Equal(m.Networks[0].Options, []string{"parent=eth0"}) {
		t.Fatalf("network missing from review: %+v", m.Networks)
	}
	if len(m.Roots) != 1 || m.Roots[0].Path != "/home/app" || m.Roots[0].SystemPath || !m.Roots[0].Existing || len(m.Roots[0].UsedBy) != 1 {
		t.Fatalf("root review = %+v", m.Roots)
	}

	// A local export keeps the earlier experience: no confirmation, no list.
	local, facts, err := e.readPayload(context.Background(), writeFixturePayload(t, e, p), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if r := moduleReview(local, facts, nil); r.Origin != OriginLocal || len(r.Containers) != 0 || (ImportReview{Modules: []ModuleReview{r}}).Foreign([]string{"apps"}) {
		t.Fatalf("local module review = %+v", r)
	}
}

// Docker reads JSON field names case-insensitively; a second spelling of a
// field must not reach it past the checks and the review.
func TestPayloadRejectsShadowedDockerFields(t *testing.T) {
	e, _, p := hostFixture(t)
	if err := e.validatePayload(p); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Container){
		"host binds":     func(c *Container) { c.HostConfig["binds"] = rawJSON([]string{"/:/host"}) },
		"config body":    func(c *Container) { c.Config["hostConfig"] = rawJSON(map[string]any{"Privileged": true}) },
		"config host":    func(c *Container) { c.Config["HostConfig"] = rawJSON(map[string]any{}) },
		"image override": func(c *Container) { c.Config["image"] = rawJSON("other/image") },
		"mount source": func(c *Container) {
			c.HostConfig["Mounts"] = rawJSON([]map[string]any{{"Type": "bind", "Source": "/home/app", "source": "/", "Target": "/data"}})
			c.HostConfig["Binds"] = rawJSON([]string{})
		},
	} {
		bad := p
		c := p.Containers[0]
		c.Config = map[string]json.RawMessage{}
		c.HostConfig = map[string]json.RawMessage{}
		for k, v := range p.Containers[0].Config {
			c.Config[k] = v
		}
		for k, v := range p.Containers[0].HostConfig {
			c.HostConfig[k] = v
		}
		mutate(&c)
		bad.Containers = []Container{c}
		if err := e.validatePayload(bad); err == nil {
			t.Fatalf("%s: shadowed field accepted", name)
		}
	}
}

func TestForeignRestoreNeedsConfirmationBeforeAnyChange(t *testing.T) {
	e, d, p := hostFixture(t)
	p = hostReachingPayload(p)
	_ = os.WriteFile(e.host("/home/app/data"), []byte("new!"), 0640)
	directory := t.TempDir()
	if err := otherHost(t, e).writePayload(context.Background(), filepath.Join(directory, "apps.payload"), p); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(e.host("/home/app/data"), []byte("old!"), 0640)
	d.calls = nil
	err := e.Restore(context.Background(), backup.NewID(), directory, []string{"apps"}, false)
	var failure *backup.Failure
	if !errors.Is(err, ErrForeignUnconfirmed) || !errors.As(err, &failure) || backup.FailureCode(err) != "foreign_confirmation_required" {
		t.Fatal("unconfirmed foreign restore was not refused:", err)
	}
	if len(d.calls) != 0 {
		t.Fatal("unconfirmed foreign restore reached Docker:", d.calls)
	}
	if data, _ := os.ReadFile(e.host("/home/app/data")); string(data) != "old!" {
		t.Fatal("unconfirmed foreign restore changed data")
	}
	if _, err := os.Lstat(filepath.Join(directory, "restore-journal.json")); !os.IsNotExist(err) {
		t.Fatal("unconfirmed foreign restore wrote a journal")
	}
	if err := e.Restore(context.Background(), backup.NewID(), directory, []string{"apps"}, true); err != nil {
		t.Fatal("confirmed foreign restore failed:", err)
	}
	if data, _ := os.ReadFile(e.host("/home/app/data")); string(data) != "new!" {
		t.Fatal("confirmed foreign restore did not apply data")
	}
	// The review described what Docker was asked to run.
	for _, c := range d.containers {
		var privileged bool
		_ = json.Unmarshal(c.HostConfig["Privileged"], &privileged)
		if !privileged {
			t.Fatal("restored container differs from the reviewed configuration")
		}
	}
}

func TestLocalRestoreNeedsNoConfirmation(t *testing.T) {
	e, _, p := hostFixture(t)
	directory := t.TempDir()
	_ = os.WriteFile(e.host("/home/app/data"), []byte("new!"), 0640)
	if err := e.writePayload(context.Background(), filepath.Join(directory, "apps.payload"), p); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(e.host("/home/app/data"), []byte("old!"), 0640)
	if err := e.Restore(context.Background(), backup.NewID(), directory, []string{"apps"}, false); err != nil {
		t.Fatal("local restore needed a confirmation:", err)
	}
	if data, _ := os.ReadFile(e.host("/home/app/data")); string(data) != "new!" {
		t.Fatal("local restore did not apply data")
	}
}

// A destination container's read-only system bind is a restorable root;
// declaring a container of the same name skips the shared-writer check. The
// review must show that replacement, and only the confirmation lets it run.
func TestForeignReviewShowsReplacedReadOnlySystemBind(t *testing.T) {
	e, d, _ := hostFixture(t)
	system := "/lib/modules"
	if err := os.MkdirAll(e.host(system), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.host(system+"/version"), []byte("host"), 0600); err != nil {
		t.Fatal(err)
	}
	exporter := Container{ID: strings.Repeat("e", 64), Name: "node-exporter", Module: "docker", ImageID: "sha256:" + strings.Repeat("a", 64), Config: map[string]json.RawMessage{"Image": rawJSON("example/exporter"), "Labels": rawJSON(map[string]string{})}, HostConfig: map[string]json.RawMessage{"Binds": rawJSON([]string{system + ":/host/modules:ro"})}, Networks: map[string]json.RawMessage{}, Mounts: []Mount{{Type: "bind", Source: system, Destination: "/host/modules"}}}
	running := &fixtureContainer{ID: exporter.ID, Name: "/node-exporter", Image: exporter.ImageID, Config: exporter.Config, HostConfig: exporter.HostConfig, Mounts: exporter.Mounts}
	d.containers[exporter.ID] = running
	hash := sha256.Sum256([]byte(system))
	p := Payload{Version: 1, Module: "docker", Roots: []Root{{ID: hex.EncodeToString(hash[:16]), Path: system, Module: "docker", Directory: true, Bytes: 4, Entries: 2}}, Containers: []Container{exporter}, Networks: map[string]json.RawMessage{}, Volumes: map[string]Volume{}}
	_ = os.WriteFile(e.host(system+"/version"), []byte("pack"), 0600)
	directory := t.TempDir()
	if err := otherHost(t, e).writePayload(context.Background(), filepath.Join(directory, "docker.payload"), p); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(e.host(system+"/version"), []byte("host"), 0600)

	read, facts, err := e.readPayload(context.Background(), filepath.Join(directory, "docker.payload"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	review := ImportReview{Modules: []ModuleReview{moduleReview(read, facts, []string{system})}}
	current, err := e.Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	e.annotateReview(&review, current)
	root := review.Modules[0].Roots[0]
	if root.Path != system || !root.SystemPath || !root.Existing || !slices.Equal(root.UsedBy, []RootUser{{Container: "node-exporter", ReadOnly: true}}) || !review.Modules[0].Containers[0].Replaces {
		t.Fatalf("read-only system bind replacement not shown: %+v %+v", root, review.Modules[0].Containers)
	}
	if err := e.Restore(context.Background(), backup.NewID(), directory, []string{"docker"}, false); !errors.Is(err, ErrForeignUnconfirmed) {
		t.Fatal("unconfirmed replacement of a system bind was not refused:", err)
	}
	if data, _ := os.ReadFile(e.host(system + "/version")); string(data) != "host" {
		t.Fatal("system directory changed without confirmation")
	}
}

func TestForeignReviewCountsSetIDFiles(t *testing.T) {
	e, _, p := hostFixture(t)
	root := p.Roots[0]
	info, err := os.Stat(e.host(root.Path))
	if err != nil {
		t.Fatal(err)
	}
	owner, err := tar.FileInfoHeader(info, "")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	manifest, _ := json.Marshal(p)
	_ = tw.WriteHeader(&tar.Header{Name: "manifest.json", Typeflag: tar.TypeReg, Mode: 0600, Size: int64(len(manifest))})
	_, _ = tw.Write(manifest)
	_ = tw.WriteHeader(&tar.Header{Name: "data/" + root.ID, Typeflag: tar.TypeDir, Mode: 0755, Uid: owner.Uid, Gid: owner.Gid})
	_ = tw.WriteHeader(&tar.Header{Name: "data/" + root.ID + "/tool", Typeflag: tar.TypeReg, Mode: 04755, Size: 2, Uid: owner.Uid, Gid: owner.Gid})
	_, _ = tw.Write([]byte("#!"))
	_ = tw.Close()
	_ = gz.Close()
	path := filepath.Join(t.TempDir(), "apps.payload")
	if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	read, facts, err := e.readPayload(context.Background(), path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if r := moduleReview(read, facts, nil); r.Origin != OriginForeign || r.SetIDFiles != 1 {
		t.Fatalf("setuid entry not counted: %+v", r)
	}
}

func serve(t *testing.T, s *Service, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "http://agent"+path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	s.ServeHTTP(recorder, request)
	return recorder
}

func waitJob(t *testing.T, s *Service, id string, statuses ...string) backup.Record {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		record, err := s.Jobs.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(statuses, record.Status) {
			return record
		}
		if record.Status == "failed" {
			t.Fatalf("job %s failed: %s", id, record.ErrorCode)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job did not finish:", id)
	return backup.Record{}
}

// The Agent protocol shared by Panel and CLI refuses a foreign restore until
// the request carries the confirmation, and reports the review it needs.
func TestServiceForeignImportConfirmation(t *testing.T) {
	for _, foreign := range []bool{true, false} {
		t.Run(map[bool]string{true: "foreign", false: "local"}[foreign], func(t *testing.T) {
			e, _, p := hostFixture(t)
			s, err := NewService(e)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			writer := e
			if foreign {
				p = hostReachingPayload(p)
				writer = otherHost(t, e)
			}
			payload, err := os.ReadFile(writeFixturePayload(t, writer, p))
			if err != nil {
				t.Fatal(err)
			}
			response := serve(t, s, "POST", "/v1/backups", []byte(`{"action":"import","modules":["apps"]}`))
			var job backup.Record
			if response.Code != 202 || json.Unmarshal(response.Body.Bytes(), &job) != nil {
				t.Fatal(response.Code, response.Body.String())
			}
			if response := serve(t, s, "PUT", "/v1/backups/"+job.ID+"/apps", payload); response.Code != 204 {
				t.Fatal(response.Code, response.Body.String())
			}
			if response := serve(t, s, "POST", "/v1/backups/"+job.ID+"/inspect", nil); response.Code != 202 {
				t.Fatal(response.Code, response.Body.String())
			}
			waitJob(t, s, job.ID, "ready")
			response = serve(t, s, "GET", "/v1/backups/"+job.ID+"/review", nil)
			var review ImportReview
			if response.Code != 200 || backup.Decode(response.Body.Bytes(), &review) != nil {
				t.Fatal(response.Code, response.Body.String())
			}
			if review.Foreign([]string{"apps"}) != foreign {
				t.Fatalf("review origin = %+v", review.Modules)
			}
			if foreign && (len(review.Modules[0].Containers) != 1 || !review.Modules[0].Containers[0].Privileged) {
				t.Fatalf("review lacks host configuration: %+v", review.Modules[0])
			}
			response = serve(t, s, "POST", "/v1/backups/"+job.ID+"/preview", nil)
			var previewed backup.Record
			if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &previewed) != nil {
				t.Fatal(response.Code, response.Body.String())
			}
			restore := func(confirm bool) *httptest.ResponseRecorder {
				body, _ := json.Marshal(Request{Action: "restore", SourceID: job.ID, Modules: []string{"apps"}, Revision: previewed.TargetRevision, ConfirmForeign: confirm})
				return serve(t, s, "POST", "/v1/backups", body)
			}
			// A legacy import must be inspected again even when a caller
			// supplies confirmation without configuration details.
			reviewPath := filepath.Join(s.Jobs.Root, job.ID, reviewFile)
			savedReview, err := os.ReadFile(reviewPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(reviewPath); err != nil {
				t.Fatal(err)
			}
			response = restore(true)
			if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "backup_review_unavailable") {
				t.Fatal("legacy import accepted without review:", response.Code, response.Body.String())
			}
			if err := os.WriteFile(reviewPath, savedReview, 0600); err != nil {
				t.Fatal(err)
			}
			response = restore(false)
			if foreign {
				if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "foreign_confirmation_required") {
					t.Fatal("unconfirmed foreign restore accepted:", response.Code, response.Body.String())
				}
				response = restore(true)
			}
			var restored backup.Record
			if response.Code != 202 || json.Unmarshal(response.Body.Bytes(), &restored) != nil {
				t.Fatal(response.Code, response.Body.String())
			}
			waitJob(t, s, restored.ID, "completed")
		})
	}
}
