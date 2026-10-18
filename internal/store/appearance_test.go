package store

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAppearancePersistsAndReturnsIndependentSnapshots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	storage, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_, initialVersion := storage.Appearance()
	value := Appearance{Theme: "dark", Wallpaper: "prism", ClassicLevel: "ambient", Colors: &AppearanceColors{
		Brand: "#7856b6", Neutral: "#54475f", Signature: "#c76586",
	}, Branding: &SiteBranding{Name: "我的面板", Icon: testBrandIcon(t, 128)}}
	if err := storage.ReplaceAppearance(initialVersion, value); err != nil {
		t.Fatal(err)
	}
	if err := storage.ReplaceAppearance(initialVersion, value); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale update = %v", err)
	}
	value.Colors.Brand = "#000000"
	value.Branding.Name = "changed"
	saved, version := storage.Appearance()
	if saved.Colors.Brand != "#7856b6" || saved.Branding.Name != "我的面板" || version == initialVersion {
		t.Fatalf("saved = %#v version=%s", saved, version)
	}
	saved.Colors.Brand = "#ffffff"
	saved.Branding.Name = "mutated snapshot"
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restored, restoredVersion := reopened.Appearance()
	if restored.Colors.Brand != "#7856b6" || restored.Branding.Name != "我的面板" || restoredVersion != version {
		t.Fatalf("restored = %#v version=%s", restored, restoredVersion)
	}
}

func TestAppearanceIncludedInPanelBackup(t *testing.T) {
	source, err := Open(filepath.Join(t.TempDir(), "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	now := time.Now().UTC()
	if err := source.CreateInitialAdmin(User{ID: "admin", Username: "admin", PasswordHash: strings.Repeat("h", 32), Role: "admin", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	_, version := source.Appearance()
	choice := Appearance{Theme: "light", Wallpaper: "horizon", ClassicLevel: "clear", Branding: &SiteBranding{Name: "Backup", Icon: testBrandIcon(t, 64)}}
	if err := source.ReplaceAppearance(version, choice); err != nil {
		t.Fatal(err)
	}
	data, err := source.ExportIdentity()
	if err != nil || ValidateIdentityBackup(data) != nil {
		t.Fatalf("exported appearance backup: %v", err)
	}
	destination, err := Open(filepath.Join(t.TempDir(), "destination.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()
	if err := destination.RestoreIdentity(data); err != nil {
		t.Fatal(err)
	}
	restored, _ := destination.Appearance()
	if restored == nil || !reflect.DeepEqual(*restored, choice) {
		t.Fatalf("restored appearance = %#v", restored)
	}
}

func testBrandIcon(t *testing.T, size int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, size, size))); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestSiteBrandingValidation(t *testing.T) {
	valid := Appearance{Theme: "system", Wallpaper: "classic", ClassicLevel: "off"}
	for _, branding := range []SiteBranding{{}, {Name: strings.Repeat("站", 64)}, {Icon: testBrandIcon(t, 128)}} {
		valid.Branding = &branding
		if err := ValidateAppearance(valid); err != nil {
			t.Fatalf("valid branding rejected: %v", err)
		}
	}
	for _, branding := range []SiteBranding{
		{Name: strings.Repeat("站", 65)}, {Name: " title "}, {Name: "bad\nname"},
		{Icon: "https://example.com/icon.png"}, {Icon: "data:image/svg+xml;base64,PHN2Zy8+"},
		{Icon: "data:image/png;base64,ZmFrZQ=="}, {Icon: testBrandIcon(t, 129)},
		{Icon: "data:image/png;base64," + strings.Repeat("A", 90000)},
	} {
		valid.Branding = &branding
		if err := ValidateAppearance(valid); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("invalid branding accepted: name=%q iconLength=%d", branding.Name, len(branding.Icon))
		}
	}
}

func TestAppearanceAcceptsWindowsXPWallpaper(t *testing.T) {
	if err := ValidateAppearance(Appearance{Theme: "system", Wallpaper: "xp", ClassicLevel: "off"}); err != nil {
		t.Fatalf("Windows XP wallpaper rejected: %v", err)
	}
}
