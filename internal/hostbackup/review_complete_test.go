package hostbackup

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestForeignReviewShowsEffectiveNetworkConfiguration(t *testing.T) {
	r := networkReview("external", json.RawMessage(`{"Driver":"bridge","IPAM":{"Driver":"custom-ipam","Options":{"custom":"value"}},"Attachable":true}`))
	if !strings.Contains(string(r.Config), "custom-ipam") || !strings.Contains(string(r.Config), "Attachable") {
		t.Fatalf("effective network config omitted: %s", r.Config)
	}
}

func TestForeignReviewShowsActualImageAndAllHostBinds(t *testing.T) {
	binds := []string{}
	mounts := []Mount{}
	for i := 0; i < 64; i++ {
		binds = append(binds, fmt.Sprintf("/home/data-%d:/data-%d", i, i))
		mounts = append(mounts, Mount{Type: "bind", Source: fmt.Sprintf("/home/data-%d", i), Destination: fmt.Sprintf("/data-%d", i), RW: true})
	}
	binds = append(binds, "/:/host:rw")
	mounts = append(mounts, Mount{Type: "bind", Source: "/", Destination: "/host", RW: true})
	body, _ := json.Marshal(binds)
	c := Container{Name: "test", ImageID: "sha256:actual", ImageRef: "registry.example/actual@sha256:pull", Config: map[string]json.RawMessage{"Image": json.RawMessage(`"display-only:tag"`)}, HostConfig: map[string]json.RawMessage{"Binds": body}}
	c.Mounts = mounts
	review := containerReview(c, nil)
	if review.ImageID != c.ImageID || review.ImageRef != c.ImageRef {
		t.Fatal("actual image not disclosed")
	}
	if len(review.Binds) != 65 || review.Binds[64].Source != "/" {
		t.Fatal("dangerous trailing bind hidden")
	}
	if len(review.HostConfig) == 0 {
		t.Fatal("effective host configuration absent")
	}
}
