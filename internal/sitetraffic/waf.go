package sitetraffic

import (
	"encoding/json"
	"io"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ruleID = regexp.MustCompile(`^[0-9]{3,10}$`)
var rateLine = regexp.MustCompile(`^(\S+) key="([^|" ]+)\|([^" ]+)" event=rate request_limit="([A-Z_]+)" connection_limit="([A-Z_]+)"`)
var deniedLine = regexp.MustCompile(`^([0-9]{4}/[0-9]{2}/[0-9]{2} [0-9:]{8}) \[\w+\] [0-9]+#[0-9]+: \*[0-9]+ ModSecurity: Access denied with code ([0-9]{3}) `)
var fieldLine = regexp.MustCompile(`\[([a-z_]+) "((?:\\.|[^"\\])*)"\]`)

func (r *Reader) loadZone(root *os.Root) {
	r.zone = time.UTC
	conf, err := root.OpenRoot("config/waf")
	if err != nil {
		return
	}
	defer conf.Close()
	file, err := openRegular(conf, "policy.json")
	if err != nil {
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 64<<10))
	if err != nil {
		return
	}
	var policy struct {
		Zone string `json:"logTimezone"`
	}
	if json.Unmarshal(data, &policy) != nil {
		return
	}
	zone := policy.Zone
	if len(zone) != 8 || !strings.HasPrefix(zone, "UTC") || (zone[3] != '+' && zone[3] != '-') {
		return
	}
	hours, e := strconv.Atoi(zone[4:6])
	minutes, f := strconv.Atoi(zone[6:])
	if e != nil || f != nil || hours > 14 || minutes > 59 {
		return
	}
	offset := hours*3600 + minutes*60
	if zone[3] == '-' {
		offset = -offset
	}
	r.zone = time.FixedZone(zone, offset)
}
func (r *Reader) parseSource(name string, line []byte) (Record, netip.Addr, bool) {
	switch {
	case strings.HasPrefix(name, "waf-audit.json"):
		return r.parseAudit(line)
	case strings.HasPrefix(name, "waf-rate.log"):
		return r.parseRate(line)
	case strings.HasPrefix(name, "error.log"):
		return r.parseDenied(line)
	default:
		return parseLine(line)
	}
}
func (r *Reader) parseTime(value string) (time.Time, error) {
	var last error
	for _, layout := range []string{time.RFC3339Nano, "Mon Jan 2 15:04:05 2006", "02/Jan/2006:15:04:05 -0700", "2006/01/02 15:04:05"} {
		at, err := time.ParseInLocation(layout, value, r.zone)
		if err == nil {
			return at, nil
		}
		last = err
	}
	return time.Time{}, last
}
func (r *Reader) parseAudit(line []byte) (Record, netip.Addr, bool) {
	record := Record{Kind: "waf", Action: "matched", Severity: 7, Rules: []string{}}
	var data struct {
		Transaction struct {
			ClientIP string `json:"client_ip"`
			Time     string `json:"time_stamp"`
			UniqueID string `json:"unique_id"`
			Request  struct {
				URI string `json:"uri"`
			} `json:"request"`
			Messages []struct {
				Details struct {
					ID       json.RawMessage `json:"ruleId"`
					Severity json.RawMessage `json:"severity"`
				} `json:"details"`
			} `json:"messages"`
		} `json:"transaction"`
	}
	if len(line) > 64<<10 || json.Unmarshal(line, &data) != nil || len(data.Transaction.Messages) > 256 {
		return record, netip.Addr{}, false
	}
	tx := data.Transaction
	ip, err := netip.ParseAddr(tx.ClientIP)
	if err != nil {
		return record, netip.Addr{}, false
	}
	at, err := r.parseTime(tx.Time)
	if err != nil {
		return record, netip.Addr{}, false
	}
	record.At = at.Unix()
	parts := strings.SplitN(tx.UniqueID, "|", 3)
	if len(parts) == 3 {
		record.Host = normalizeHost(parts[0])
		record.EventKey = "tx:" + tx.UniqueID
	}
	if uri, e := url.ParseRequestURI(tx.Request.URI); e == nil {
		record.Path = bounded(uri.EscapedPath(), 200)
	}
	for _, message := range tx.Messages {
		id := scalar(message.Details.ID)
		if !ruleID.MatchString(id) {
			continue
		}
		if len(record.Rules) < 32 {
			record.Rules = appendUnique(record.Rules, id)
		}
		severity, e := strconv.Atoi(scalar(message.Details.Severity))
		if e == nil && severity >= 0 && severity < record.Severity {
			record.Severity = severity
		}
	}
	// The audit JSON proves rule matches. HTTP 403 alone does not prove interception.
	// A matching native "Access denied" error event can later upgrade this same ID.
	return record, ip.Unmap(), len(record.Rules) > 0
}
func (r *Reader) parseRate(line []byte) (Record, netip.Addr, bool) {
	record := Record{Kind: "waf", Rules: []string{}, Severity: 4, Action: "detected"}
	m := rateLine.FindSubmatch(line)
	if len(m) != 6 {
		return record, netip.Addr{}, false
	}
	at, err := time.Parse(time.RFC3339Nano, string(m[1]))
	if err != nil {
		return record, netip.Addr{}, false
	}
	ip, err := netip.ParseAddr(string(m[3]))
	if err != nil {
		return record, netip.Addr{}, false
	}
	record.At = at.Unix()
	record.Host = normalizeHost(string(m[2]))
	for i, name := range []string{"nginx.limit_req", "nginx.limit_conn"} {
		status := string(m[i+4])
		if status == "REJECTED" {
			record.Action = "limited"
			record.Rules = append(record.Rules, name)
		} else if strings.Contains(status, "DRY_RUN") {
			record.Rules = append(record.Rules, name)
		} else if status == "DELAYED" {
			record.Rules = append(record.Rules, name)
			if record.Action != "limited" {
				record.Action = "delayed"
			}
		}
	}
	return record, ip.Unmap(), len(record.Rules) > 0
}
func (r *Reader) parseDenied(line []byte) (Record, netip.Addr, bool) {
	record := Record{Kind: "waf", Action: "blocked", Severity: 4, Rules: []string{}}
	m := deniedLine.FindSubmatch(line)
	if len(m) != 3 {
		return record, netip.Addr{}, false
	}
	fields := map[string]string{}
	for _, pair := range fieldLine.FindAllSubmatch(line, 64) {
		if _, exists := fields[string(pair[1])]; !exists {
			fields[string(pair[1])] = string(pair[2])
		}
	}
	if !ruleID.MatchString(fields["id"]) {
		return record, netip.Addr{}, false
	}
	parts := strings.SplitN(fields["unique_id"], "|", 3)
	if len(parts) != 3 {
		return record, netip.Addr{}, false
	}
	ip, err := netip.ParseAddr(parts[1])
	if err != nil {
		return record, netip.Addr{}, false
	}
	at, err := r.parseTime(string(m[1]))
	if err != nil {
		return record, netip.Addr{}, false
	}
	record.At = at.Unix()
	record.Host = normalizeHost(parts[0])
	record.EventKey = "tx:" + fields["unique_id"]
	record.Rules = []string{fields["id"]}
	if uri, e := url.ParseRequestURI(fields["uri"]); e == nil {
		record.Path = bounded(uri.EscapedPath(), 200)
	}
	return record, ip.Unmap(), true
}
func scalar(value json.RawMessage) string {
	var text string
	if json.Unmarshal(value, &text) == nil {
		return text
	}
	return string(value)
}
