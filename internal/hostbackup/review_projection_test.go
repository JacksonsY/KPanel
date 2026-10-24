package hostbackup

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestReviewSecurityFactsDoNotComeFromSecretProjection(t *testing.T) {
	c := Container{Name: "demo", ImageID: "sha256:test", Config: map[string]json.RawMessage{"Image": rawJSON("-----BEGIN PRIVATE KEY-----")},
		HostConfig: map[string]json.RawMessage{
			"AaNote":      rawJSON("-----BEGIN PRIVATE KEY-----"),
			"Privileged":  rawJSON(true),
			"NetworkMode": rawJSON("host"),
			"ZzNote":      rawJSON("-----END PRIVATE KEY-----"),
		}, Mounts: []Mount{{Type: "bind", Source: "/home/data", Destination: "/data", RW: true}}}
	before := rawJSON(c)
	review := moduleReview(Payload{Module: "apps", Containers: []Container{c}}, payloadFacts{}, nil)
	if len(review.Containers) != 1 {
		t.Fatal("missing container review")
	}
	got := review.Containers[0]
	if !got.Privileged || !slices.Contains(got.HostNamespaces, "network") || len(got.Binds) != 1 || !got.Binds[0].Live {
		t.Fatalf("security facts changed by text projection: %+v", got)
	}
	if !bytes.Equal(rawJSON(c), before) {
		t.Fatal("review changed the execution configuration")
	}
	dir := t.TempDir()
	if err := saveReview(dir, ImportReview{Revision: "revision-kept", Modules: []ModuleReview{review}}); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadReview(dir, []string{"apps"})
	if err != nil || !loaded.Complete([]string{"apps"}) || loaded.Revision != "revision-kept" || !loaded.Modules[0].Containers[0].Privileged || !loaded.Modules[0].Containers[0].Binds[0].Live {
		t.Fatalf("persisted projection changed security facts: %+v %v", loaded, err)
	}
}

func TestLegacyReviewProjectsTextWithoutErasingTypedFacts(t *testing.T) {
	const secret = "synthetic-review-private-marker"
	r := ImportReview{Version: 2, Revision: "revision-kept", Modules: []ModuleReview{{Module: "apps", Origin: OriginForeign,
		Containers: []ContainerReview{{Name: "app", Image: "-----BEGIN PRIVATE KEY-----", Privileged: true, Replaces: true, HostConfig: rawJSON(map[string]any{"token": secret}), Binds: []BindReview{{Source: "/home/data", Destination: "/data", Live: true}}}},
		Networks:   []NetworkReview{{Name: "lan", Internal: true, Config: rawJSON(map[string]string{"token": secret})}},
		Volumes:    []VolumeReview{{Name: "data", Config: rawJSON(map[string]string{"password": secret})}},
	}}}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, reviewFile), rawJSON(r), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := loadReview(dir, []string{"apps"})
	if err != nil || !got.Complete([]string{"apps"}) || !got.Modules[0].Containers[0].Privileged || !got.Modules[0].Networks[0].Internal {
		t.Fatalf("legacy projection erased security facts: %+v %v", got, err)
	}
	for _, data := range [][]byte{rawJSON(got), mustReviewBytes(t, filepath.Join(dir, reviewFile))} {
		if bytes.Contains(data, []byte(secret)) || bytes.Contains(data, []byte("-----BEGIN PRIVATE KEY-----")) {
			t.Fatal("legacy review retained unprojected text")
		}
	}
}

func mustReviewBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
