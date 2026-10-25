package history

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kejilion/kejilion-panel/internal/sitetraffic"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func enabledStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err = s.Action(context.Background(), "enable"); err != nil {
		t.Fatal(err)
	}
	return s
}
func put(t *testing.T, s *Store, records ...sitetraffic.Record) {
	t.Helper()
	input, _, err := s.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = s.Apply(context.Background(), sitetraffic.Batch{GeoStatus: "ready", Generation: input.Generation, Records: records, Cursors: []sitetraffic.Cursor{{ID: "fixture", Offset: 123}}, WAFSource: true, Warnings: []string{}})
	if err != nil {
		t.Fatal(err)
	}
}
func request(path string, status int) sitetraffic.Record {
	return sitetraffic.Record{Kind: "access", At: time.Now().Unix(), Host: "site.example", Path: path, Referrer: "search.example", Country: "GB", IPHash: strings.Repeat("a", 32), Status: status, Bytes: 200}
}
func TestHistoryExactlyOncePrivateAndSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Action(ctx, "enable"); err != nil {
		t.Fatal(err)
	}
	input, _, _ := s.State(ctx)
	batch := sitetraffic.Batch{GeoStatus: "ready", Generation: input.Generation, Records: []sitetraffic.Record{request("/a", 200), request("/b", 403)}, Cursors: []sitetraffic.Cursor{{ID: "log", Offset: 250}}, Warnings: []string{}}
	if err = s.Apply(ctx, batch); err != nil {
		t.Fatal(err)
	}
	if err = s.Apply(ctx, batch); !errors.Is(err, ErrStale) {
		t.Fatal("duplicate batch accepted", err)
	}
	before, _ := s.Query(ctx, "24h", "")
	if before.Requests != 2 || before.Errors != 1 || before.Bytes != 400 || before.UniqueIPs != 1 {
		t.Fatalf("totals %+v", before)
	}
	raw, _ := json.Marshal(before)
	if strings.Contains(string(raw), input.Salt) || strings.Contains(string(raw), strings.Repeat("a", 32)) {
		t.Fatal("private identity exposed")
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	after, err := s.Query(ctx, "24h", "")
	if err != nil || after.Requests != 2 {
		t.Fatalf("restart %+v %v", after, err)
	}
	cursor, enabled, err := s.State(ctx)
	if err != nil || !enabled || cursor.Cursors[0].Offset != 250 || cursor.Salt != input.Salt {
		t.Fatal("checkpoint not durable")
	}
}
func TestHistoryWAFMergeAndCorrectCounts(t *testing.T) {
	s := enabledStore(t)
	audit := request("/api", 0)
	audit.Kind = "waf"
	audit.EventKey = strings.Repeat("b", 32)
	audit.Action = "matched"
	audit.Rules = []string{"942100", "949110"}
	audit.Severity = 2
	put(t, s, audit)
	blocked := audit
	blocked.Action = "blocked"
	put(t, s, blocked)
	put(t, s, audit)
	snap, err := s.Query(context.Background(), "24h", "")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Requests != 0 || snap.Protection.Events != 1 || snap.Protection.Blocked != 1 || snap.Protection.Matched != 0 || len(snap.Protection.Recent) != 1 || len(snap.Protection.Rules) != 2 {
		t.Fatalf("double counted %+v", snap)
	}
}
func TestPauseClearAndInvalidBatchAreAtomic(t *testing.T) {
	ctx := context.Background()
	s := enabledStore(t)
	input, _, _ := s.State(ctx)
	put(t, s, request("/ok", 200))
	if err := s.Action(ctx, "pause"); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx, sitetraffic.Batch{GeoStatus: "ready", Generation: input.Generation, Records: []sitetraffic.Record{request("/late", 200)}}); !errors.Is(err, ErrStale) {
		t.Fatalf("late commit %v", err)
	}
	if err := s.Action(ctx, "clear"); err != nil {
		t.Fatal(err)
	}
	snap, _ := s.Query(ctx, "24h", "")
	if snap.Requests != 0 || snap.Enabled {
		t.Fatal("clear changed enablement or retained counts")
	}
	s.Action(ctx, "enable")
	input, _, _ = s.State(ctx)
	invalid := request("/bad", 200)
	invalid.Bytes = -1
	if err := s.Apply(ctx, sitetraffic.Batch{GeoStatus: "ready", Generation: input.Generation, Records: []sitetraffic.Record{request("/rollback", 200), invalid}}); err == nil {
		t.Fatal("invalid batch committed")
	}
	snap, _ = s.Query(ctx, "24h", "")
	if snap.Requests != 0 {
		t.Fatal("partial transaction")
	}
}
func TestHistoryRetentionAndHostQuery(t *testing.T) {
	s := enabledStore(t)
	old := request("/expired", 200)
	old.At = time.Now().Add(-31 * 24 * time.Hour).Unix()
	put(t, s, request("/now", 200), old)
	snap, err := s.Query(context.Background(), "30d", "' OR 1=1--")
	if err != nil || snap.Requests != 0 {
		t.Fatalf("host injection %+v %v", snap, err)
	}
	snap, _ = s.Query(context.Background(), "30d", "site.example")
	if snap.Requests != 1 || len(snap.Hosts) != 1 {
		t.Fatal("wrong range or source")
	}
	if _, err = s.Query(context.Background(), "forever", ""); err == nil {
		t.Fatal("unbounded period")
	}
}
func TestHistoryCorruptionIsPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "analytics.sqlite")
	os.WriteFile(path, []byte("not a database"), 0600)
	if s, err := Open(dir); err == nil {
		s.Close()
		t.Fatal("corrupt database accepted")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "not a database" {
		t.Fatal("corrupt original replaced")
	}
}

func TestOfflineBackupRestore(t *testing.T) {
	ctx := context.Background()
	source := t.TempDir()
	s, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	s.Action(ctx, "enable")
	put(t, s, request("/backup", 200))
	before, _, _ := s.State(ctx)
	s.Close()
	backup := t.TempDir()
	entries, _ := os.ReadDir(source)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(backup, entry.Name()), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := Open(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	snap, err := restored.Query(ctx, "24h", "")
	after, enabled, _ := restored.State(ctx)
	if err != nil || snap.Requests != 1 || !enabled || after.Salt != before.Salt || after.Cursors[0].Offset != before.Cursors[0].Offset {
		t.Fatalf("restore %+v %v", snap, err)
	}
}
