package store

import (
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func sampleDesktop() *DesktopPreferences {
	return &DesktopPreferences{Mode: "desktop", SideSplitRatio: .6, FocusedID: 3,
		Windows:     []DesktopWindow{{ID: 3, Path: "/files?path=%2Fhome", Geometry: DesktopGeometry{Left: 10, Top: 20, Width: 900, Height: 600}, Z: 101}},
		WindowSizes: []DesktopWindowSize{{Path: "/files", Width: 900, Height: 600}},
	}
}

func TestPreferencesPersistenceConflictRollbackAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_, initial := s.Preferences()
	value := Preferences{Locale: "zh-TW", Desktop: sampleDesktop()}
	if err := s.PatchPreferences(initial, value); err != nil {
		t.Fatal(err)
	}
	if err := s.PatchPreferences(initial, Preferences{Locale: "en-US"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale write: %v", err)
	}
	value.Desktop.Windows[0].Path = "/settings"
	got, version := s.Preferences()
	if got.Desktop.Windows[0].Path != "/files?path=%2Fhome" {
		t.Fatal("input aliases store")
	}
	if err := s.PatchPreferences(version, Preferences{Locale: "en-US"}); err != nil {
		t.Fatal(err)
	}
	got, version = s.Preferences()
	if got.Locale != "en-US" || got.Desktop.FocusedID != 3 {
		t.Fatal("locale patch erased desktop")
	}
	got.Desktop.Windows[0].Path = "/settings"
	got, _ = s.Preferences()
	if got.Desktop.Windows[0].Path == "/settings" {
		t.Fatal("snapshot aliases store")
	}
	// Force atomic replacement to fail and verify the in-memory snapshot rolls back.
	originalPath := s.path
	s.path = filepath.Join(t.TempDir(), "missing", "panel.json")
	if err := s.PatchPreferences(version, Preferences{Locale: "zh-CN"}); err == nil {
		t.Fatal("expected persistence failure")
	}
	s.path = originalPath
	after, afterVersion := s.Preferences()
	if !reflect.DeepEqual(after, got) || afterVersion != version {
		t.Fatal("failed save changed state")
	}
	if err := s.CreateInitialAdmin(User{ID: "admin", Username: "admin", PasswordHash: strings.Repeat("h", 32), Role: "admin", CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	backup, err := s.ExportIdentity()
	if err != nil || ValidateIdentityBackup(backup) != nil {
		t.Fatalf("backup: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	restored, restoredVersion := s.Preferences()
	if !reflect.DeepEqual(restored, got) || restoredVersion != version {
		t.Fatal("restart lost preferences")
	}
	destination, err := Open(filepath.Join(t.TempDir(), "restore.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()
	if err := destination.RestoreIdentity(backup); err != nil {
		t.Fatal(err)
	}
	restored, _ = destination.Preferences()
	if !reflect.DeepEqual(restored, got) {
		t.Fatal("backup lost preferences")
	}
}

func TestPreferencesRejectInvalidAndUnboundedState(t *testing.T) {
	cases := map[string]func(*Preferences){
		"locale":     func(p *Preferences) { p.Locale = "xx" },
		"windows":    func(p *Preferences) { p.Desktop.Windows = make([]DesktopWindow, 9) },
		"sizes":      func(p *Preferences) { p.Desktop.WindowSizes = make([]DesktopWindowSize, 33) },
		"external":   func(p *Preferences) { p.Desktop.Windows[0].Path = "https://example.com" },
		"auth route": func(p *Preferences) { p.Desktop.Windows[0].Path = "/login?password=abc" },
		"duplicate":  func(p *Preferences) { p.Desktop.Windows = append(p.Desktop.Windows, p.Desktop.Windows[0]) },
		"focus":      func(p *Preferences) { p.Desktop.FocusedID = 999 },
		"geometry":   func(p *Preferences) { p.Desktop.Windows[0].Geometry.Width = math.Inf(1) },
		"ratio":      func(p *Preferences) { p.Desktop.SideSplitRatio = math.NaN() },
		"size query": func(p *Preferences) { p.Desktop.WindowSizes[0].Path = "/files?path=/" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := Preferences{Locale: "en-US", Desktop: sampleDesktop()}
			mutate(&p)
			if ValidatePreferences(p) == nil {
				t.Fatal("accepted invalid preferences")
			}
		})
	}
}
