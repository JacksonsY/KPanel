package desktopworkspace

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func TestSiteIconsSurviveWorkspaceChangesAndRestart(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	id := "0123456789abcdef0123456789abcdef"
	key := "site-" + id
	data := encodedPNG(t, 16, 16)
	icon, err := store.PutIcon(key, "image/png", data)
	if err != nil {
		t.Fatal(err)
	}
	// Same raw ID cannot expose a site icon through the shortcut namespace.
	if _, err := store.Icon(id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("shortcut namespace: %v", err)
	}
	input := validReplaceInput(store.Workspace().ResourceVersion)
	input.Labels = map[string]string{"site:" + id: "我的站点"}
	if _, err := store.Replace(input); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Workspace().SiteIcons[id] != icon.Version {
		t.Fatal("site icon version lost on restart")
	}
	got, err := reopened.Icon(key)
	if err != nil || !bytes.Equal(got.Data, data) {
		t.Fatalf("site icon lost: %v", err)
	}
	// Failed validation or persistence must preserve the previous confirmed icon.
	if _, err := reopened.PutIcon(key, "image/png", []byte("invalid")); !errors.Is(err, ErrIconInvalid) {
		t.Fatalf("invalid image: %v", err)
	}
	reopened.writeAtomic = func(string, string, []byte) error { return errors.New("disk full") }
	if _, err := reopened.PutIcon(key, "image/png", encodedPNG(t, 12, 12)); err == nil {
		t.Fatal("expected write failure")
	}
	got, err = reopened.Icon(key)
	if err != nil || got.Version != icon.Version {
		t.Fatalf("failed update replaced icon: %v", err)
	}
	if err := reopened.DeleteIcon(key); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Icon(key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted icon: %v", err)
	}
	if len(reopened.Workspace().SiteIcons) != 0 {
		t.Fatal("deleted icon metadata retained")
	}
}

func TestSiteIconLimitsAndInvalidIDs(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data := encodedPNG(t, 8, 8)
	for _, id := range []string{"site-../escape", "site-" + "A0123456789abcdef0123456789abcdef", "site-short", "site-/0123456789abcdef0123456789abcdef"} {
		if _, err := store.PutIcon(id, "image/png", data); !errors.Is(err, ErrNotFound) {
			t.Fatalf("invalid ID %q: %v", id, err)
		}
	}
	for index := range MaxSiteIcons {
		store.iconVersions[fmt.Sprintf("site-%032x", index)] = "existing"
	}
	if _, err := store.PutIcon(fmt.Sprintf("site-%032x", MaxSiteIcons), "image/png", data); !errors.Is(err, ErrIconQuota) {
		t.Fatalf("site count limit: %v", err)
	}
	// Replacing an existing item still works when the count limit is reached.
	if _, err := store.PutIcon(fmt.Sprintf("site-%032x", 0), "image/png", data); err != nil {
		t.Fatal(err)
	}
}
