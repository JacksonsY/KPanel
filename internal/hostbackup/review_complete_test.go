package hostbackup

import (
	"encoding/json"
	"fmt"
	"testing"
)

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
