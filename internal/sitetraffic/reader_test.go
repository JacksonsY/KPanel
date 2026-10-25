package sitetraffic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readerFixture(t *testing.T) (string, *Reader) {
	t.Helper()
	root := t.TempDir()
	for _, path := range []string{"log/nginx", "config"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0700); err != nil {
			t.Fatal(err)
		}
	}
	return root, NewReader(root)
}
func access(ip, path string, status int) string {
	return fmt.Sprintf("%s - - [%s] \"GET %s HTTP/1.1\" %d 123 \"https://search.example/find?q=secret\" \"Browser\" \"198.51.100.1\"\n", ip, time.Now().Format("02/Jan/2006:15:04:05 -0700"), path, status)
}
func TestCollectIncrementalGeoAndPrivacy(t *testing.T) {
	root, reader := readerFixture(t)
	data, err := os.ReadFile("testdata/GeoIP2-Country-Test.mmdb")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "config/GeoLite2-Country.mmdb"), data, 0600)
	path := filepath.Join(root, "log/nginx/access.log")
	os.WriteFile(path, []byte(access("81.2.69.160", "/account?token=secret", 200)), 0600)
	input := CollectInput{Salt: strings.Repeat("ab", 32), Generation: 7}
	batch, err := reader.Collect(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 1 || batch.Records[0].Country != "GB" || batch.GeoStatus != "ready" || batch.Generation != 7 {
		t.Fatalf("batch=%+v", batch)
	}
	record := batch.Records[0]
	if record.Path != "/account" || record.Referrer != "search.example" || len(record.IPHash) != 32 {
		t.Fatalf("record=%+v", record)
	}
	encoded, _ := json.Marshal(batch)
	for _, secret := range []string{"81.2.69.160", "token=", "q=secret", "198.51.100.1"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("exposed %q", secret)
		}
	}
	input.Cursors = batch.Cursors
	next, err := reader.Collect(context.Background(), input)
	if err != nil || len(next.Records) != 0 {
		t.Fatalf("replayed %+v %v", next, err)
	}
	file, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	file.WriteString(access("81.2.69.160", "/again", 500))
	file.Close()
	next, err = reader.Collect(context.Background(), input)
	if err != nil || len(next.Records) != 1 || next.Records[0].Path != "/again" {
		t.Fatalf("append %+v %v", next, err)
	}
	if next.Records[0].IPHash != record.IPHash {
		t.Fatal("unstable anonymized identity")
	}
}
func TestCollectIncompleteRotationAndOversizedLines(t *testing.T) {
	root, reader := readerFixture(t)
	path := filepath.Join(root, "log/nginx/access.log")
	line := access("192.0.2.1", "/first", 200)
	os.WriteFile(path, []byte(strings.TrimSuffix(line, "\n")), 0600)
	input := CollectInput{Salt: strings.Repeat("ab", 32)}
	b, err := reader.Collect(context.Background(), input)
	if err != nil || len(b.Records) != 0 {
		t.Fatalf("unfinished %+v %v", b, err)
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	f.WriteString("\n")
	f.Close()
	input.Cursors = b.Cursors
	b, err = reader.Collect(context.Background(), input)
	if err != nil || len(b.Records) != 1 {
		t.Fatalf("finished %+v %v", b, err)
	}
	input.Cursors = b.Cursors
	os.Rename(path, path+".1")
	os.WriteFile(path, []byte(strings.Repeat("X", 80<<10)+"\n"+access("192.0.2.2", "/second", 200)), 0600)
	b, err = reader.Collect(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Records) != 1 || b.Records[0].Path != "/second" {
		t.Fatalf("rotation replay %+v", b)
	}
}
func TestCollectRejectsUnsafeInputsAndLinks(t *testing.T) {
	root, reader := readerFixture(t)
	for _, input := range []CollectInput{{Salt: "bad"}, {Salt: strings.Repeat("ab", 32), Cursors: []Cursor{{ID: "x", Offset: -1}}}} {
		if _, err := reader.Collect(context.Background(), input); err != ErrInput {
			t.Fatalf("accepted %+v", input)
		}
	}
	target := filepath.Join(t.TempDir(), "private.log")
	os.WriteFile(target, []byte(access("192.0.2.1", "/secret", 200)), 0600)
	if err := os.Symlink(target, filepath.Join(root, "log/nginx/access.log")); err != nil {
		t.Skip("symlink unavailable")
	}
	b, err := reader.Collect(context.Background(), CollectInput{Salt: strings.Repeat("ab", 32)})
	if err != nil || len(b.Records) != 0 {
		t.Fatalf("followed symlink %+v %v", b, err)
	}
}
func TestWAFNativeSourcesDoNotTreat403AsInterception(t *testing.T) {
	root, reader := readerFixture(t)
	dir := filepath.Join(root, "log/nginx")
	os.WriteFile(filepath.Join(dir, "access.log"), []byte(access("192.0.2.1", "/private", 403)), 0600)
	unique := "site.example|192.0.2.1|abc123"
	audit := map[string]any{"transaction": map[string]any{"client_ip": "192.0.2.1", "time_stamp": time.Now().UTC().Format(time.RFC3339), "unique_id": unique, "response": map[string]int{"http_code": 403}, "request": map[string]string{"uri": "/api?password=secret"}, "messages": []any{map[string]any{"details": map[string]string{"ruleId": "942100", "severity": "2", "data": "secret"}}}}}
	payload, _ := json.Marshal(audit)
	os.WriteFile(filepath.Join(dir, "waf-audit.json"), append(payload, '\n'), 0600)
	denied := fmt.Sprintf("%s [error] 1#1: *5 ModSecurity: Access denied with code 403 (phase 2). [id \"942100\"] [uri \"/api?password=secret\"] [unique_id \"%s\"]\n", time.Now().UTC().Format("2006/01/02 15:04:05"), unique)
	os.WriteFile(filepath.Join(dir, "error.log"), []byte(denied), 0600)
	rate := time.Now().UTC().Format(time.RFC3339) + " key=\"site.example|192.0.2.1\" event=rate request_limit=\"REJECTED\" connection_limit=\"PASSED\"\n"
	os.WriteFile(filepath.Join(dir, "waf-rate.log"), []byte(rate), 0600)
	b, err := reader.Collect(context.Background(), CollectInput{Salt: strings.Repeat("ab", 32)})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Records) != 4 || !b.WAFSource {
		t.Fatalf("events %+v", b)
	}
	kinds := map[string]Record{}
	for _, event := range b.Records {
		kinds[event.Kind+event.Action] = event
	}
	if kinds["access"].Status != 403 || kinds["wafmatched"].Action != "matched" || kinds["wafblocked"].EventKey != kinds["wafmatched"].EventKey || kinds["waflimited"].Action != "limited" {
		t.Fatalf("classification %+v", kinds)
	}
	raw, _ := json.Marshal(b)
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "192.0.2.1") {
		t.Fatal("raw audit detail leaked")
	}
}

func TestCollectFairSourcesAndBounds(t *testing.T) {
	root, reader := readerFixture(t)
	data := strings.Repeat(access("192.0.2.1", "/busy", 200), 4000)
	os.WriteFile(filepath.Join(root, "log/nginx/access.log"), []byte(data), 0600)
	rate := time.Now().Format(time.RFC3339) + ` key="site.example|192.0.2.1" event=rate request_limit="REJECTED" connection_limit="PASSED"` + "\n"
	os.WriteFile(filepath.Join(root, "log/nginx/waf-rate.log"), []byte(rate), 0600)
	batch, err := reader.Collect(context.Background(), CollectInput{Salt: strings.Repeat("ab", 32)})
	if err != nil || batch.ScannedBytes > MaxBatchBytes || len(batch.Records) > MaxRecords || !batch.HasMore {
		t.Fatalf("bounds %+v %v", batch, err)
	}
	found := false
	for _, r := range batch.Records {
		if r.Action == "limited" {
			found = true
		}
	}
	if !found {
		t.Fatal("busy access source starved security source")
	}
}
func BenchmarkCollectBounded(b *testing.B) {
	root := b.TempDir()
	os.MkdirAll(filepath.Join(root, "log/nginx"), 0700)
	os.WriteFile(filepath.Join(root, "log/nginx/access.log"), []byte(strings.Repeat(access("192.0.2.1", "/sample", 200), 10000)), 0600)
	reader := NewReader(root)
	input := CollectInput{Salt: strings.Repeat("ab", 32)}
	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		if _, err := reader.Collect(context.Background(), input); err != nil {
			b.Fatal(err)
		}
	}
}

func TestCollectCopytruncateTransfersCheckpoint(t *testing.T) {
	root, reader := readerFixture(t)
	path := filepath.Join(root, "log/nginx/access.log")
	first := access("192.0.2.1", "/counted", 200)
	late := access("192.0.2.1", "/unread-before-copy", 200)
	fresh := access("192.0.2.2", "/new-current", 200)
	os.WriteFile(path, []byte(first), 0600)
	input := CollectInput{Salt: strings.Repeat("ab", 32)}
	batch, err := reader.Collect(context.Background(), input)
	if err != nil || len(batch.Records) != 1 {
		t.Fatal(err, batch)
	}
	input.Cursors = batch.Cursors
	os.WriteFile(path+".1", []byte(first+late), 0600)
	os.WriteFile(path, []byte(fresh), 0600)
	batch, err = reader.Collect(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, record := range batch.Records {
		seen[record.Path]++
	}
	if len(batch.Records) != 2 || seen["/counted"] != 0 || seen["/unread-before-copy"] != 1 || seen["/new-current"] != 1 {
		t.Fatalf("copytruncate replay or loss: %+v", batch)
	}
	input.Cursors = batch.Cursors
	batch, err = reader.Collect(context.Background(), input)
	if err != nil || len(batch.Records) != 0 {
		t.Fatal("second replay", batch, err)
	}
}
func TestCollectOutageKeepsCheckpoints(t *testing.T) {
	root, reader := readerFixture(t)
	path := filepath.Join(root, "log/nginx/access.log")
	os.WriteFile(path, []byte(access("192.0.2.1", "/once", 200)), 0600)
	input := CollectInput{Salt: strings.Repeat("ab", 32)}
	batch, err := reader.Collect(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	input.Cursors = batch.Cursors
	logs := filepath.Join(root, "log/nginx")
	away := filepath.Join(root, "log/nginx-away")
	if err = os.Rename(logs, away); err != nil {
		t.Fatal(err)
	}
	outage, err := reader.Collect(context.Background(), input)
	if err != nil || len(outage.Cursors) != len(input.Cursors) || outage.Cursors[0].Offset != input.Cursors[0].Offset {
		t.Fatal("checkpoint lost", outage, err)
	}
	if err = os.Rename(away, logs); err != nil {
		t.Fatal(err)
	}
	input.Cursors = outage.Cursors
	batch, err = reader.Collect(context.Background(), input)
	if err != nil || len(batch.Records) != 0 {
		t.Fatal("outage recovery replay", batch, err)
	}
}

func TestCollectCopytruncateDoesNotBorrowAnotherSiteCheckpoint(t *testing.T) {
	root, reader := readerFixture(t)
	logs := filepath.Join(root, "log/nginx")
	a := filepath.Join(logs, "a.example.access.log")
	b := filepath.Join(logs, "b.example.access.log")
	same := access("192.0.2.1", "/same", 200)
	os.WriteFile(a, []byte(same+same+same), 0600)
	os.WriteFile(b, []byte(same), 0600)
	input := CollectInput{Salt: strings.Repeat("ab", 32)}
	batch, err := reader.Collect(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	input.Cursors = batch.Cursors
	os.WriteFile(b+".1", []byte(same+access("192.0.2.1", "/late-b", 200)), 0600)
	os.WriteFile(b, []byte(access("192.0.2.2", "/fresh-b", 200)), 0600)
	batch, err = reader.Collect(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, record := range batch.Records {
		seen[record.Path]++
		if record.Host != "b.example" {
			t.Fatal("cross-site attribution", record)
		}
	}
	if len(batch.Records) != 2 || seen["/late-b"] != 1 || seen["/fresh-b"] != 1 {
		t.Fatalf("cross-site checkpoint borrowed: %+v", batch)
	}
}
