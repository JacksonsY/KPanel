package sitetraffic

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

var ErrBusy = errors.New("traffic collection already running")
var ErrInput = errors.New("invalid traffic collection input")
var combined = regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "((?:\\.|[^"\\])*)" ([0-9]{3}) ([0-9]+|-) "((?:\\.|[^"\\])*)"`)
var validHost = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]{0,251}[a-z0-9])?$`)
var validCountry = regexp.MustCompile(`^[A-Z]{2}$`)

type Reader struct {
	root       string
	mu         sync.Mutex
	db         *maxminddb.Reader
	dbInfo     os.FileInfo
	geoStatus  string
	geoChecked time.Time
	zone       *time.Location
}

func NewReader(webRoot string) *Reader {
	if webRoot == "" {
		webRoot = "/home/web"
	}
	return &Reader{root: webRoot}
}
func (r *Reader) Collect(ctx context.Context, input CollectInput) (Batch, error) {
	out := Batch{Generation: input.Generation, Records: []Record{}, Cursors: []Cursor{}, Warnings: []string{}, GeoStatus: "missing"}
	salt, err := hex.DecodeString(input.Salt)
	if err != nil || len(salt) != 32 || len(input.Cursors) > MaxSources {
		return out, ErrInput
	}
	cursors := map[string]Cursor{}
	for _, c := range input.Cursors {
		if len(c.ID) > 160 || c.ID == "" || c.Offset < 0 || c.Offset > 1<<50 || len(c.Name) > 255 || strings.ContainsAny(c.Name, "\x00\r\n") {
			return out, ErrInput
		}
		if _, exists := cursors[c.ID]; exists {
			return out, ErrInput
		}
		cursors[c.ID] = c
	}
	if !r.mu.TryLock() {
		return out, ErrBusy
	}
	defer r.mu.Unlock()
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	root, err := os.OpenRoot(r.root)
	if errors.Is(err, os.ErrNotExist) {
		return preserveCursors(out, input.Cursors), nil
	}
	if err != nil {
		return out, err
	}
	defer root.Close()
	// Both children are anchored beneath an already opened root. Reject directory
	// links rather than treating an unrelated tree as the Nginx log/database source.
	for _, name := range []string{"log", "log/nginx"} {
		info, e := root.Lstat(name)
		if errors.Is(e, os.ErrNotExist) {
			return preserveCursors(out, input.Cursors), nil
		}
		if e != nil {
			return out, e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return out, errors.New("unsafe nginx log directory")
		}
	}
	r.loadGeo(root)
	r.loadZone(root)
	out.GeoStatus = r.geoStatus
	logs, err := root.OpenRoot("log/nginx")
	if err != nil {
		return out, err
	}
	defer logs.Close()
	dir, err := logs.Open(".")
	if err != nil {
		return out, err
	}
	entries, err := dir.ReadDir(257)
	dir.Close()
	if err != nil && err != io.EOF {
		return out, err
	}
	if len(entries) > 256 {
		out.Warnings = append(out.Warnings, "log_directory_limit")
		entries = entries[:256]
	}
	type source struct {
		name string
		at   time.Time
	}
	sources := []source{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || e.Type()&os.ModeSymlink != 0 || (!strings.Contains(name, "access") && name != "error.log" && name != "error.log.1" && name != "waf-rate.log" && name != "waf-rate.log.1" && name != "waf-audit.json" && name != "waf-audit.json.1") || strings.HasSuffix(name, ".gz") {
			continue
		}
		if !strings.HasSuffix(name, ".log") && !strings.HasSuffix(name, ".log.1") && name != "waf-audit.json" && name != "waf-audit.json.1" {
			continue
		}
		if info, e := e.Info(); e == nil && info.Mode().IsRegular() {
			sources = append(sources, source{name, info.ModTime()})
		}
	}
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].at.Equal(sources[j].at) {
			return sources[i].name < sources[j].name
		}
		return sources[i].at.After(sources[j].at)
	})
	if len(sources) > MaxSources {
		sources = sources[:MaxSources]
		out.Warnings = append(out.Warnings, "source_limit")
	}
	seen := map[string]bool{}
	for index, source := range sources {
		if ctx.Err() != nil {
			return Batch{}, ctx.Err()
		}
		file, err := openRegular(logs, source.name)
		if err != nil {
			out.Warnings = appendUnique(out.Warnings, "unsafe_or_unreadable_log")
			continue
		}
		info, _ := file.Stat()
		id, err := fileID(file)
		if err != nil {
			file.Close()
			continue
		}
		// The first complete line also detects ordinary copytruncate replacement.
		head := make([]byte, 512)
		n, _ := file.ReadAt(head, 0)
		head = head[:n]
		if end := strings.IndexByte(string(head), '\n'); end >= 0 {
			head = head[:end+1]
		}
		sum := sha256.Sum256(head)
		id += "-" + hex.EncodeToString(sum[:8])
		if seen[id] {
			file.Close()
			continue
		}
		seen[id] = true
		// Scope copytruncate checkpoint transfer to this log name, not another site's log.
		groupSum := sha256.Sum256([]byte(strings.TrimSuffix(source.name, ".1")))
		group := hex.EncodeToString(groupSum[:8]) + "-"
		id = group + id
		cursor, known := cursors[id]
		if !known && strings.HasSuffix(source.name, ".1") {
			for _, previous := range input.Cursors {
				if strings.HasPrefix(previous.ID, group) && strings.HasSuffix(previous.ID, "-"+hex.EncodeToString(sum[:8])) && (!known || previous.Offset > cursor.Offset) {
					cursor = previous
					known = true
				}
			}
			if known && cursor.Offset > info.Size() {
				cursor.Offset = info.Size()
				cursor.Discard = false
			}
		}
		cursor.ID = id
		cursor.Name = source.name
		if strings.Contains(source.name, "access") {
			out.AccessSource = true
		}
		if strings.HasPrefix(source.name, "waf-") {
			out.WAFSource = true
		}
		if !known && info.Size() > MaxBatchBytes {
			cursor.Offset = info.Size() - MaxBatchBytes
			cursor.Discard = true
			out.Warnings = appendUnique(out.Warnings, "initial_tail_only")
		}
		if cursor.Offset > info.Size() {
			cursor.Offset = 0
			cursor.Discard = false
			out.Warnings = appendUnique(out.Warnings, "log_truncated")
		}
		if _, err = file.Seek(cursor.Offset, io.SeekStart); err != nil {
			file.Close()
			continue
		}
		budget := (MaxBatchBytes - out.ScannedBytes) / int64(len(sources)-index)
		recordLimit := len(out.Records) + (MaxRecords-len(out.Records))/(len(sources)-index)
		reader := bufio.NewReaderSize(io.LimitReader(file, budget), 64<<10)
		for len(out.Records) < recordLimit && out.ScannedBytes < MaxBatchBytes {
			if ctx.Err() != nil {
				file.Close()
				return Batch{}, ctx.Err()
			}
			line, readErr := reader.ReadSlice('\n')
			if len(line) == 0 {
				break
			}
			out.ScannedBytes += int64(len(line))
			complete := line[len(line)-1] == '\n'
			if cursor.Discard || readErr == bufio.ErrBufferFull {
				cursor.Offset += int64(len(line))
				cursor.Discard = !complete
				if complete {
					out.Skipped++
				}
			} else if !complete {
				// Keep an unfinished ordinary line for the next append; do not lose it.
				break
			} else {
				cursor.Offset += int64(len(line))
				record, ip, ok := r.parseSource(source.name, line)
				if !ok {
					if !strings.HasPrefix(source.name, "error.log") {
						out.Skipped++
					}
					continue
				}
				if record.Host == "" {
					record.Host = hostFromFile(source.name)
				}
				record.Country = r.country(ip)
				if record.Kind != "access" {
					out.WAFSource = true
					key := record.EventKey
					if key == "" {
						key = id + ":" + strconv.FormatInt(cursor.Offset, 10)
					}
					tag := hmac.New(sha256.New, salt)
					tag.Write([]byte(key))
					record.EventKey = hex.EncodeToString(tag.Sum(nil)[:16])
				}
				mac := hmac.New(sha256.New, salt)
				mac.Write(ip.AsSlice())
				record.IPHash = hex.EncodeToString(mac.Sum(nil)[:16])
				out.Records = append(out.Records, record)
			}
			if readErr != nil && readErr != bufio.ErrBufferFull {
				break
			}
		}
		if cursor.Offset < info.Size() {
			out.HasMore = true
		}
		out.Cursors = append(out.Cursors, cursor)
		file.Close()
	}
	if out.Skipped > 0 {
		out.Warnings = appendUnique(out.Warnings, "unparsed_lines")
	}
	return preserveCursors(out, input.Cursors), nil
}
func (r *Reader) loadGeo(root *os.Root) {
	if time.Since(r.geoChecked) < 30*time.Second {
		return
	}
	r.geoChecked = time.Now()
	info, err := root.Lstat("config")
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		r.geoStatus = "missing"
		r.dropGeo()
		return
	}
	conf, err := root.OpenRoot("config")
	if err != nil {
		r.geoStatus = "missing"
		return
	}
	defer conf.Close()
	file, err := openRegular(conf, "GeoLite2-Country.mmdb")
	if err != nil {
		r.geoStatus = "missing"
		r.dropGeo()
		return
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || info.Size() > 64<<20 {
		r.geoStatus = "invalid"
		r.dropGeo()
		return
	}
	if r.db != nil && r.dbInfo != nil && os.SameFile(info, r.dbInfo) && info.Size() == r.dbInfo.Size() && info.ModTime() == r.dbInfo.ModTime() {
		r.geoStatus = "ready"
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, (64<<20)+1))
	if err != nil || len(data) > 64<<20 {
		r.geoStatus = "invalid"
		r.dropGeo()
		return
	}
	db, err := maxminddb.OpenBytes(data)
	if err != nil {
		r.geoStatus = "invalid"
		r.dropGeo()
		return
	}
	if !strings.Contains(db.Metadata.DatabaseType, "Country") && !strings.Contains(db.Metadata.DatabaseType, "City") {
		db.Close()
		r.geoStatus = "invalid"
		r.dropGeo()
		return
	}
	if r.db != nil {
		r.db.Close()
	}
	r.db = db
	r.dbInfo = info
	r.geoStatus = "ready"
}
func (r *Reader) dropGeo() {
	if r.db != nil {
		r.db.Close()
	}
	r.db = nil
	r.dbInfo = nil
}
func (r *Reader) country(ip netip.Addr) string {
	if r.db == nil {
		return "ZZ"
	}
	var data struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}
	if err := r.db.Lookup(ip).Decode(&data); err != nil {
		return "ZZ"
	}
	if validCountry.MatchString(data.Country.ISOCode) {
		return data.Country.ISOCode
	}
	return "ZZ"
}
func parseLine(line []byte) (Record, netip.Addr, bool) {
	record := Record{Kind: "access"}
	var ipText, request, ref, stamp string
	if len(line) > 16<<10 {
		return record, netip.Addr{}, false
	}
	if len(line) > 0 && line[0] == '{' {
		var value struct {
			RemoteAddr string      `json:"remote_addr"`
			Time       string      `json:"time_local"`
			ISOTime    string      `json:"time_iso8601"`
			Request    string      `json:"request"`
			Status     json.Number `json:"status"`
			Bytes      json.Number `json:"body_bytes_sent"`
			Host       string      `json:"host"`
			Ref        string      `json:"http_referer"`
		}
		if json.Unmarshal(line, &value) != nil {
			return record, netip.Addr{}, false
		}
		ipText, request, ref, stamp = value.RemoteAddr, value.Request, value.Ref, value.Time
		if value.ISOTime != "" {
			stamp = value.ISOTime
		}
		record.Host = normalizeHost(value.Host)
		status, e := strconv.Atoi(string(value.Status))
		if e != nil {
			return record, netip.Addr{}, false
		}
		record.Status = status
		size, e := strconv.ParseInt(string(value.Bytes), 10, 64)
		if e != nil {
			return record, netip.Addr{}, false
		}
		record.Bytes = size
	} else {
		values := combined.FindSubmatch(line)
		if len(values) != 7 {
			return record, netip.Addr{}, false
		}
		ipText, stamp, request, ref = string(values[1]), string(values[2]), string(values[3]), string(values[6])
		record.Status, _ = strconv.Atoi(string(values[4]))
		if string(values[5]) != "-" {
			record.Bytes, _ = strconv.ParseInt(string(values[5]), 10, 64)
		}
	}
	ip, err := netip.ParseAddr(ipText)
	if err != nil || record.Status < 100 || record.Status > 599 || record.Bytes < 0 || record.Bytes > 1<<50 {
		return Record{}, netip.Addr{}, false
	}
	ip = ip.Unmap()
	at, err := time.Parse("02/Jan/2006:15:04:05 -0700", stamp)
	if err != nil {
		at, err = time.Parse(time.RFC3339, stamp)
	}
	if err != nil {
		return Record{}, netip.Addr{}, false
	}
	record.At = at.Unix()
	parts := strings.SplitN(request, " ", 3)
	if len(parts) < 2 {
		return Record{}, netip.Addr{}, false
	}
	target, err := url.ParseRequestURI(parts[1])
	if err != nil {
		return Record{}, netip.Addr{}, false
	}
	record.Path = target.EscapedPath()
	if record.Path == "" {
		record.Path = "/"
	}
	record.Path = bounded(record.Path, 200)
	if record.Host == "" && target.Host != "" {
		record.Host = normalizeHost(target.Host)
	}
	if origin, e := url.Parse(ref); e == nil && (origin.Scheme == "http" || origin.Scheme == "https") {
		record.Referrer = bounded(strings.ToLower(origin.Hostname()), 120)
	}
	return record, ip, true
}
func normalizeHost(host string) string {
	if value, _, err := net.SplitHostPort(host); err == nil {
		host = value
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if ip, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		return ip.Unmap().String()
	}
	if validHost.MatchString(host) {
		return host
	}
	return ""
}
func hostFromFile(name string) string {
	name = strings.TrimSuffix(name, ".1")
	for _, suffix := range []string{".access.log", "_access.log"} {
		if strings.HasSuffix(name, suffix) {
			return normalizeHost(strings.TrimSuffix(name, suffix))
		}
	}
	return ""
}
func bounded(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
func appendUnique(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}

// Keep checkpoints for sources that temporarily disappear or cannot be opened.
// A successfully read replacement of the same name retires the old identity.
func preserveCursors(out Batch, previous []Cursor) Batch {
	ids := map[string]bool{}
	names := map[string]bool{}
	for _, cursor := range out.Cursors {
		ids[cursor.ID] = true
		if cursor.Name != "" {
			names[cursor.Name] = true
		}
	}
	for _, cursor := range previous {
		if ids[cursor.ID] || (cursor.Name != "" && names[cursor.Name]) {
			continue
		}
		if len(out.Cursors) >= MaxSources {
			out.Warnings = appendUnique(out.Warnings, "source_limit")
			continue
		}
		out.Cursors = append(out.Cursors, cursor)
		ids[cursor.ID] = true
	}
	return out
}
