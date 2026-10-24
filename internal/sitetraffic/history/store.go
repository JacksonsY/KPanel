// Package history holds derived site telemetry in Panel-owned bounded SQLite storage.
package history

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/kejilion/kejilion-panel/internal/sitetraffic"
	_ "modernc.org/sqlite"
)

var ErrStale = errors.New("traffic batch is stale")
var ErrUnavailable = errors.New("traffic history unavailable")

var countryPattern = regexp.MustCompile(`^[A-Z]{2}$`)
var hashPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

const applicationID = 0x4b505457
const MaxDBBytes int64 = 128 << 20

type Store struct {
	mu     sync.Mutex
	db     *sql.DB
	path   string
	opened os.FileInfo
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUnavailable
	}
	if err = os.Chmod(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "analytics.sqlite")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		info, e := os.Lstat(path + suffix)
		if e == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > MaxDBBytes+16<<20) {
			return nil, ErrUnavailable
		}
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return nil, e
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	info, err = f.Stat()
	f.Close()
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db, path: path, opened: info}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var app, version int
	if err = db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&app); err != nil {
		db.Close()
		return nil, err
	}
	if err = db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if info.Size() != 0 && (app != applicationID || version != 1) {
		db.Close()
		return nil, ErrUnavailable
	}
	statements := []string{
		"PRAGMA foreign_keys=ON", "PRAGMA trusted_schema=OFF", "PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA busy_timeout=1000", "PRAGMA mmap_size=0", "PRAGMA cache_size=-2048", "PRAGMA journal_size_limit=4194304", "PRAGMA wal_autocheckpoint=256", "PRAGMA max_page_count=32768",
		`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS traffic (hour INTEGER,host TEXT,country TEXT,requests INTEGER,bytes INTEGER,errors INTEGER,PRIMARY KEY(hour,host,country)) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS pages (hour INTEGER,host TEXT,name TEXT,requests INTEGER,bytes INTEGER,errors INTEGER,PRIMARY KEY(hour,host,name)) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS referrers (hour INTEGER,host TEXT,name TEXT,requests INTEGER,bytes INTEGER,errors INTEGER,PRIMARY KEY(hour,host,name)) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS ips (hour INTEGER,host TEXT,hash TEXT,country TEXT,PRIMARY KEY(hour,host,hash,country)) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS waf (hour INTEGER,host TEXT,country TEXT,action TEXT,requests INTEGER,PRIMARY KEY(hour,host,country,action)) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS rule_counts (hour INTEGER,host TEXT,name TEXT,requests INTEGER,PRIMARY KEY(hour,host,name)) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS events (id TEXT PRIMARY KEY,at INTEGER NOT NULL,host TEXT,path TEXT,country TEXT,action TEXT,severity INTEGER,rules TEXT) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS transactions (id TEXT PRIMARY KEY,at INTEGER NOT NULL,host TEXT,country TEXT,action TEXT) WITHOUT ROWID`,
		`CREATE INDEX IF NOT EXISTS transactions_time ON transactions(at)`,
		`CREATE INDEX IF NOT EXISTS events_time ON events(at DESC,id)`,
		`CREATE TABLE IF NOT EXISTS event_rules (event TEXT,rule TEXT,PRIMARY KEY(event,rule)) WITHOUT ROWID`,
		fmt.Sprintf("PRAGMA application_id=%d", applicationID), "PRAGMA user_version=1",
	}
	for _, statement := range statements {
		if _, err = db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	salt := make([]byte, 32)
	if _, err = rand.Read(salt); err != nil {
		db.Close()
		return nil, err
	}
	for key, value := range map[string]string{"salt": hex.EncodeToString(salt), "generation": "0", "enabled": "0", "cursors": "[]", "geo": "missing", "warnings": "[]", "waf": "0", "access": "0", "started": "", "collected": "", "more": "0"} {
		if _, err = db.ExecContext(ctx, "INSERT OR IGNORE INTO meta VALUES(?,?)", key, value); err != nil {
			db.Close()
			return nil, err
		}
	}
	return s, nil
}
func (s *Store) Close() error { s.mu.Lock(); defer s.mu.Unlock(); return s.db.Close() }
func (s *Store) check() error {
	info, err := os.Lstat(s.path)
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(info, s.opened) {
		return ErrUnavailable
	}
	return nil
}
func (s *Store) State(ctx context.Context) (sitetraffic.CollectInput, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out sitetraffic.CollectInput
	if err := s.check(); err != nil {
		return out, false, err
	}
	var salt, cursors string
	var enabled int
	for _, item := range []struct {
		key    string
		target any
	}{{"salt", &salt}, {"cursors", &cursors}, {"generation", &out.Generation}, {"enabled", &enabled}} {
		if err := s.db.QueryRowContext(ctx, "SELECT value FROM meta WHERE key=?", item.key).Scan(item.target); err != nil {
			return out, false, err
		}
	}
	if len(cursors) > 8192 || json.Unmarshal([]byte(cursors), &out.Cursors) != nil || len(out.Cursors) > sitetraffic.MaxSources {
		return out, false, ErrUnavailable
	}
	out.Salt = salt
	return out, enabled == 1, nil
}
func (s *Store) Action(ctx context.Context, action string) error {
	if action != "enable" && action != "pause" && action != "clear" {
		return errors.New("invalid traffic action")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if action == "clear" {
		if _, err = tx.ExecContext(ctx, "UPDATE meta SET value='[]' WHERE key='warnings'"); err != nil {
			return err
		}
		for _, table := range []string{"traffic", "pages", "referrers", "ips", "waf", "rule_counts", "events", "transactions", "event_rules"} {
			if _, err = tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, "UPDATE meta SET value=? WHERE key='started'", time.Now().UTC().Format(time.RFC3339)); err != nil {
			return err
		}
		salt := make([]byte, 32)
		if _, err = rand.Read(salt); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE meta SET value=? WHERE key='salt'", hex.EncodeToString(salt)); err != nil {
			return err
		}
	} else {
		value := "0"
		if action == "enable" {
			value = "1"
		}
		if _, err = tx.ExecContext(ctx, "UPDATE meta SET value=? WHERE key='enabled'", value); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE meta SET value=CAST(value AS INTEGER)+1 WHERE key='generation'"); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Apply(ctx context.Context, batch sitetraffic.Batch) error {
	if len(batch.Records) > sitetraffic.MaxRecords || len(batch.Cursors) > sitetraffic.MaxSources || len(batch.Warnings) > 16 {
		return ErrUnavailable
	}
	for _, cursor := range batch.Cursors {
		if cursor.ID == "" || len(cursor.ID) > 160 || cursor.Offset < 0 || cursor.Offset > 1<<50 {
			return ErrUnavailable
		}
	}
	if batch.GeoStatus != "ready" && batch.GeoStatus != "missing" && batch.GeoStatus != "invalid" {
		return ErrUnavailable
	}
	for _, warning := range batch.Warnings {
		if len(warning) > 64 {
			return ErrUnavailable
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var generation int64
	var enabled int
	if err = tx.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='generation'").Scan(&generation); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='enabled'").Scan(&enabled); err != nil {
		return err
	}
	if generation != batch.Generation || enabled != 1 {
		return ErrStale
	}
	now := time.Now().Unix()
	oldest := now - 30*86400
	for _, record := range batch.Records {
		if record.At < oldest || record.At > now+300 {
			continue
		}
		if len(record.Host) > 253 || len(record.Path) > 200 || len(record.Referrer) > 120 || !hashPattern.MatchString(record.IPHash) || !countryPattern.MatchString(record.Country) || !utf8.ValidString(record.Path) || strings.ContainsAny(record.Host+record.Path+record.Referrer, "\x00\r\n") || record.Bytes < 0 || record.Bytes > 1<<50 {
			return ErrUnavailable
		}
		hour := record.At / 3600 * 3600
		if record.Kind == "access" {
			if record.Status < 100 || record.Status > 599 {
				return ErrUnavailable
			}
			errorsCount := 0
			if record.Status >= 400 {
				errorsCount = 1
			}
			for _, row := range []struct{ table, name string }{{"traffic", record.Country}, {"pages", record.Path}, {"referrers", record.Referrer}} {
				column := "name"
				if row.table == "traffic" {
					column = "country"
				}
				// Identifiers come exclusively from the fixed table list above.
				statement := "INSERT INTO " + row.table + "(hour,host," + column + ",requests,bytes,errors) VALUES(?,?,?,1,?,?) ON CONFLICT(hour,host," + column + ") DO UPDATE SET requests=requests+1,bytes=bytes+excluded.bytes,errors=errors+excluded.errors"
				if _, err = tx.ExecContext(ctx, statement, hour, record.Host, row.name, record.Bytes, errorsCount); err != nil {
					return err
				}
			}
			if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO ips VALUES(?,?,?,?)", hour, record.Host, record.IPHash, record.Country); err != nil {
				return err
			}
		} else if record.Kind == "waf" {
			if record.At < now-7*86400 {
				continue
			}
			if err = applyEvent(ctx, tx, record, hour); err != nil {
				return err
			}
		} else {
			return ErrUnavailable
		}
	}
	for _, table := range []string{"traffic", "pages", "referrers", "ips", "waf", "rule_counts"} {
		if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE hour<?", oldest); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM events WHERE at<? OR id IN (SELECT id FROM events ORDER BY at DESC,id LIMIT -1 OFFSET 5000)", now-7*86400); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM event_rules WHERE event NOT IN (SELECT id FROM transactions)"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM transactions WHERE at<?", now-7*86400); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM event_rules WHERE event NOT IN (SELECT id FROM transactions)"); err != nil {
		return err
	}
	var used, free int
	if err = tx.QueryRowContext(ctx, "PRAGMA page_count").Scan(&used); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&free); err != nil {
		return err
	}
	var rowCount int64
	if err = tx.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM traffic)+(SELECT COUNT(*) FROM pages)+(SELECT COUNT(*) FROM referrers)+(SELECT COUNT(*) FROM ips)+(SELECT COUNT(*) FROM rule_counts)+(SELECT COUNT(*) FROM transactions)+(SELECT COUNT(*) FROM event_rules)").Scan(&rowCount); err != nil {
		return err
	}
	if used-free > 24576 || rowCount > 400000 {
		var first int64
		if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MIN(hour),?) FROM traffic", now).Scan(&first); err != nil {
			return err
		}
		for _, table := range []string{"traffic", "pages", "referrers", "ips", "waf", "rule_counts"} {
			if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE hour<?", first+86400); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM transactions WHERE at<?", first+86400); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM event_rules WHERE event NOT IN (SELECT id FROM transactions)"); err != nil {
			return err
		}
		batch.Warnings = append(batch.Warnings, "history_capacity")
	}
	var previous string
	if err = tx.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='warnings'").Scan(&previous); err != nil {
		return err
	}
	var past []string
	_ = json.Unmarshal([]byte(previous), &past)
	for _, warning := range past {
		found := false
		for _, current := range batch.Warnings {
			if current == warning {
				found = true
			}
		}
		if !found && len(batch.Warnings) < 16 {
			batch.Warnings = append(batch.Warnings, warning)
		}
	}
	cursors, _ := json.Marshal(batch.Cursors)
	warnings, _ := json.Marshal(batch.Warnings)
	values := map[string]string{"cursors": string(cursors), "geo": batch.GeoStatus, "warnings": string(warnings), "collected": time.Now().UTC().Format(time.RFC3339), "more": "0", "waf": "0", "access": "0"}
	if batch.HasMore {
		values["more"] = "1"
	}
	if batch.AccessSource {
		values["access"] = "1"
	}
	if batch.WAFSource {
		values["waf"] = "1"
	}
	for key, value := range values {
		if _, err = tx.ExecContext(ctx, "UPDATE meta SET value=? WHERE key=?", value, key); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE meta SET value=? WHERE key='started' AND value=''", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE meta SET value=CAST(value AS INTEGER)+1 WHERE key='generation'"); err != nil {
		return err
	}
	return tx.Commit()
}
func applyEvent(ctx context.Context, tx *sql.Tx, record sitetraffic.Record, hour int64) error {
	if !hashPattern.MatchString(record.EventKey) || len(record.Rules) > 32 || record.Severity < 0 || record.Severity > 7 {
		return ErrUnavailable
	}
	if record.Action != "blocked" && record.Action != "limited" && record.Action != "matched" && record.Action != "detected" && record.Action != "delayed" {
		return ErrUnavailable
	}
	var action, host, country string
	var at int64
	err := tx.QueryRowContext(ctx, "SELECT action,at,host,country FROM transactions WHERE id=?", record.EventKey).Scan(&action, &at, &host, &country)
	fresh := errors.Is(err, sql.ErrNoRows)
	if err != nil && !fresh {
		return err
	}
	if fresh {
		if _, err = tx.ExecContext(ctx, "INSERT INTO transactions VALUES(?,?,?,?,?)", record.EventKey, record.At, record.Host, record.Country, record.Action); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO events VALUES(?,?,?,?,?,?,?,?)", record.EventKey, record.At, record.Host, record.Path, record.Country, record.Action, record.Severity, "[]"); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO waf VALUES(?,?,?,?,1) ON CONFLICT(hour,host,country,action) DO UPDATE SET requests=requests+1", hour, record.Host, record.Country, record.Action); err != nil {
			return err
		}
	} else {
		hour = at / 3600 * 3600
		record.Host = host
		record.Country = country
		if (record.Action == "blocked" || record.Action == "limited") && action != "blocked" && action != "limited" {
			if _, err = tx.ExecContext(ctx, "UPDATE waf SET requests=requests-1 WHERE hour=? AND host=? AND country=? AND action=?", hour, host, country, action); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO waf VALUES(?,?,?,?,1) ON CONFLICT(hour,host,country,action) DO UPDATE SET requests=requests+1", hour, host, country, record.Action); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, "UPDATE transactions SET action=? WHERE id=?", record.Action, record.EventKey); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, "UPDATE events SET action=? WHERE id=?", record.Action, record.EventKey); err != nil {
				return err
			}
		}
	}
	for _, rule := range record.Rules {
		if len(rule) == 0 || len(rule) > 32 || strings.ContainsAny(rule, "\x00\r\n") {
			return ErrUnavailable
		}
		result, e := tx.ExecContext(ctx, "INSERT OR IGNORE INTO event_rules VALUES(?,?)", record.EventKey, rule)
		if e != nil {
			return e
		}
		inserted, _ := result.RowsAffected()
		if inserted > 0 {
			if _, err = tx.ExecContext(ctx, "INSERT INTO rule_counts VALUES(?,?,?,1) ON CONFLICT(hour,host,name) DO UPDATE SET requests=requests+1", hour, record.Host, rule); err != nil {
				return err
			}
		}
	}
	rows, err := tx.QueryContext(ctx, "SELECT rule FROM event_rules WHERE event=? ORDER BY rule LIMIT 32", record.EventKey)
	if err != nil {
		return err
	}
	rules := []string{}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			rows.Close()
			return err
		}
		rules = append(rules, value)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	encoded, _ := json.Marshal(rules)
	_, err = tx.ExecContext(ctx, "UPDATE events SET rules=?,path=CASE WHEN path='' THEN ? ELSE path END,severity=MIN(severity,?) WHERE id=?", string(encoded), record.Path, record.Severity, record.EventKey)
	return err
}
func (s *Store) Query(ctx context.Context, period, host string) (sitetraffic.Snapshot, error) {
	hours, bucket := 24, int64(3600)
	switch period {
	case "24h":
	case "7d":
		hours = 168
		bucket = 6 * 3600
	case "30d":
		hours = 720
		bucket = 24 * 3600
	default:
		return sitetraffic.Snapshot{}, errors.New("invalid period")
	}
	if len(host) > 253 || strings.ContainsAny(host, "\x00\r\n") {
		return sitetraffic.Snapshot{}, errors.New("invalid host")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(); err != nil {
		return sitetraffic.Snapshot{}, err
	}
	now := time.Now().UTC()
	from := now.Truncate(time.Hour).Add(-time.Duration(hours-1) * time.Hour)
	since := from.Unix()
	out := sitetraffic.Snapshot{Period: period, From: from, To: now, GeneratedAt: now, State: "disabled", GeoStatus: "missing", Trend: []sitetraffic.Point{}, Countries: []sitetraffic.Country{}, Paths: []sitetraffic.Ranking{}, Referrers: []sitetraffic.Ranking{}, Hosts: []string{}, Warnings: []string{}, Protection: sitetraffic.Protection{Rules: []sitetraffic.Ranking{}, Recent: []sitetraffic.SecurityEvent{}, Trend: []sitetraffic.Point{}, Countries: []sitetraffic.Country{}}}
	meta := map[string]string{}
	rows, err := s.db.QueryContext(ctx, "SELECT key,value FROM meta WHERE key NOT IN ('salt','cursors','generation')")
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var key, value string
		if err = rows.Scan(&key, &value); err != nil {
			rows.Close()
			return out, err
		}
		meta[key] = value
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.AccessSource = meta["access"] == "1"
	out.Enabled = meta["enabled"] == "1"
	out.GeoStatus = meta["geo"]
	out.CatchingUp = meta["more"] == "1"
	out.Protection.SourceAvailable = meta["waf"] == "1"
	if out.Enabled {
		out.State = "collecting"
	}
	if t, e := time.Parse(time.RFC3339, meta["started"]); e == nil {
		out.StartedAt = &t
	}
	if t, e := time.Parse(time.RFC3339, meta["collected"]); e == nil {
		out.CollectedAt = &t
		if out.Enabled {
			out.State = "active"
			if !out.AccessSource && !out.Protection.SourceAvailable {
				out.State = "waiting"
			}
		}
	}
	_ = json.Unmarshal([]byte(meta["warnings"]), &out.Warnings)
	clause := "hour>=? AND (?='' OR host=?)"
	args := []any{since, host, host}
	err = s.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(requests),0),COALESCE(SUM(bytes),0),COALESCE(SUM(errors),0) FROM traffic WHERE "+clause, args...).Scan(&out.Requests, &out.Bytes, &out.Errors)
	if err != nil {
		return out, err
	}
	if err = s.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT hash) FROM ips WHERE "+clause, args...).Scan(&out.UniqueIPs); err != nil {
		return out, err
	}
	rows, err = s.db.QueryContext(ctx, "SELECT hour/?*?,SUM(requests),SUM(bytes),SUM(errors) FROM traffic WHERE "+clause+" GROUP BY 1 ORDER BY 1", append([]any{bucket, bucket}, args...)...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var p sitetraffic.Point
		var at int64
		if err = rows.Scan(&at, &p.Requests, &p.Bytes, &p.Errors); err != nil {
			rows.Close()
			return out, err
		}
		p.At = time.Unix(at, 0).UTC()
		out.Trend = append(out.Trend, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = s.db.QueryContext(ctx, "SELECT country,SUM(requests),SUM(bytes),SUM(errors) FROM traffic WHERE "+clause+" GROUP BY country ORDER BY SUM(requests) DESC LIMIT 251", args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var c sitetraffic.Country
		if err = rows.Scan(&c.Code, &c.Requests, &c.Bytes, &c.Errors); err != nil {
			rows.Close()
			return out, err
		}
		out.Countries = append(out.Countries, c)
		if c.Code != "ZZ" {
			out.LocatedRequests += c.Requests
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, item := range []struct {
		table  string
		target *[]sitetraffic.Ranking
	}{{"pages", &out.Paths}, {"referrers", &out.Referrers}} {
		rows, err = s.db.QueryContext(ctx, "SELECT name,SUM(requests),SUM(bytes),SUM(errors) FROM "+item.table+" WHERE "+clause+" GROUP BY name ORDER BY SUM(requests) DESC,name LIMIT 20", args...)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var row sitetraffic.Ranking
			if err = rows.Scan(&row.Name, &row.Requests, &row.Bytes, &row.Errors); err != nil {
				rows.Close()
				return out, err
			}
			*item.target = append(*item.target, row)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, err
		}
	}
	rows, err = s.db.QueryContext(ctx, "SELECT host FROM (SELECT host FROM traffic WHERE hour>=? UNION SELECT host FROM waf WHERE hour>=?) WHERE host<>'' ORDER BY host LIMIT 100", since, since)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			rows.Close()
			return out, err
		}
		out.Hosts = append(out.Hosts, value)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.SiteAttribution = len(out.Hosts) > 0
	var shared int64
	if err = s.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(requests),0) FROM traffic WHERE hour>=? AND host=''", since).Scan(&shared); err != nil {
		return out, err
	}
	if shared > 0 {
		out.Warnings = append(out.Warnings, "shared_log_unattributed")
	}
	if err = queryProtection(ctx, s.db, &out, clause, args); err != nil {
		return out, err
	}
	return out, nil
}
func queryProtection(ctx context.Context, db *sql.DB, out *sitetraffic.Snapshot, clause string, args []any) error {
	rows, err := db.QueryContext(ctx, "SELECT action,SUM(requests) FROM waf WHERE "+clause+" GROUP BY action", args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var action string
		var n int64
		if err = rows.Scan(&action, &n); err != nil {
			rows.Close()
			return err
		}
		out.Protection.Events += n
		switch action {
		case "blocked":
			out.Protection.Blocked += n
		case "limited":
			out.Protection.Limited += n
		default:
			out.Protection.Matched += n
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = db.QueryContext(ctx, "SELECT name,SUM(requests) FROM rule_counts WHERE "+clause+" GROUP BY name ORDER BY SUM(requests) DESC,name LIMIT 20", args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item sitetraffic.Ranking
		if err = rows.Scan(&item.Name, &item.Requests); err != nil {
			rows.Close()
			return err
		}
		out.Protection.Rules = append(out.Protection.Rules, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = db.QueryContext(ctx, "SELECT hour,SUM(requests) FROM waf WHERE "+clause+" GROUP BY hour ORDER BY hour", args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var point sitetraffic.Point
		var at int64
		if err = rows.Scan(&at, &point.Requests); err != nil {
			rows.Close()
			return err
		}
		point.At = time.Unix(at, 0).UTC()
		out.Protection.Trend = append(out.Protection.Trend, point)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = db.QueryContext(ctx, "SELECT country,SUM(requests) FROM waf WHERE "+clause+" GROUP BY country ORDER BY SUM(requests) DESC LIMIT 251", args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item sitetraffic.Country
		if err = rows.Scan(&item.Code, &item.Requests); err != nil {
			rows.Close()
			return err
		}
		out.Protection.Countries = append(out.Protection.Countries, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = db.QueryContext(ctx, "SELECT id,at,host,path,country,action,severity,rules FROM events WHERE at>=? AND (?='' OR host=?) ORDER BY at DESC,id LIMIT 50", args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var event sitetraffic.SecurityEvent
		var at int64
		var rules string
		if err = rows.Scan(&event.ID, &at, &event.Host, &event.Path, &event.Country, &event.Action, &event.Severity, &rules); err != nil {
			rows.Close()
			return err
		}
		event.At = time.Unix(at, 0).UTC()
		if json.Unmarshal([]byte(rules), &event.Rules) != nil {
			rows.Close()
			return ErrUnavailable
		}
		out.Protection.Recent = append(out.Protection.Recent, event)
	}
	err = rows.Err()
	rows.Close()
	return err
}
