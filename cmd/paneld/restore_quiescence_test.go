package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/panel"
)

func TestFailedInstanceNeverRecoversInSameProcess(t *testing.T) {
	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, "backups"), 0700); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(directory, "backups", "panel-restore.json")
	// Recovery must not even open a pending journal while a failed HTTP drain
	// can leave old handlers alive. A fresh process owns recovery instead.
	marker := []byte(`{"id":"preserve-for-fresh-process","phase":"pending"}`)
	if err := os.WriteFile(journal, marker, 0600); err != nil {
		t.Fatal(err)
	}
	want := errors.New("HTTP shutdown did not drain active handlers")
	calls := 0
	err := runPanelInstances(panel.Config{DataDir: directory}, func(panel.Config) error { calls++; return want })
	if err != want || calls != 1 {
		t.Fatalf("failed instance attempted same-process recovery/restart: err=%v calls=%d", err, calls)
	}
	actual, err := os.ReadFile(journal)
	if err != nil || string(actual) != string(marker) {
		t.Fatalf("pending recovery intent changed: %s error=%v", actual, err)
	}
}
