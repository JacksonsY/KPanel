// Package sitetraffic reads existing Nginx logs without changing host configuration.
package sitetraffic

import "time"

const MaxRecords = 2048
const MaxSources = 16
const MaxBatchBytes int64 = 1 << 20

type Cursor struct {
	ID      string `json:"id"`
	Offset  int64  `json:"offset"`
	Discard bool   `json:"discard,omitempty"`
}
type CollectInput struct {
	Generation int64    `json:"generation"`
	Salt       string   `json:"salt"`
	Cursors    []Cursor `json:"cursors"`
}
type Record struct {
	Kind     string   `json:"kind"`
	EventKey string   `json:"eventKey,omitempty"`
	Action   string   `json:"action,omitempty"`
	Rules    []string `json:"rules,omitempty"`
	Severity int      `json:"severity,omitempty"`
	At       int64    `json:"at"`
	Host     string   `json:"host"`
	Path     string   `json:"path"`
	Referrer string   `json:"referrer"`
	Country  string   `json:"country"`
	IPHash   string   `json:"ipHash"`
	Status   int      `json:"status"`
	Bytes    int64    `json:"bytes"`
}
type Batch struct {
	AccessSource bool     `json:"accessSource"`
	Generation   int64    `json:"generation"`
	WAFSource    bool     `json:"wafSource"`
	Records      []Record `json:"records"`
	Cursors      []Cursor `json:"cursors"`
	GeoStatus    string   `json:"geoStatus"`
	ScannedBytes int64    `json:"scannedBytes"`
	Skipped      int      `json:"skipped"`
	HasMore      bool     `json:"hasMore"`
	Warnings     []string `json:"warnings"`
}
type Metric struct {
	Requests  int64 `json:"requests"`
	Bytes     int64 `json:"bytes"`
	Errors    int64 `json:"errors"`
	UniqueIPs int64 `json:"uniqueIPs"`
}
type Point struct {
	At time.Time `json:"at"`
	Metric
}
type Ranking struct {
	Name string `json:"name"`
	Metric
}
type Country struct {
	Code string `json:"code"`
	Metric
}
type SecurityEvent struct {
	ID       string    `json:"id"`
	At       time.Time `json:"at"`
	Host     string    `json:"host"`
	Path     string    `json:"path"`
	Country  string    `json:"country"`
	Action   string    `json:"action"`
	Rules    []string  `json:"rules"`
	Severity int       `json:"severity"`
}
type Protection struct {
	Events          int64           `json:"events"`
	Blocked         int64           `json:"blocked"`
	Limited         int64           `json:"limited"`
	Matched         int64           `json:"matched"`
	Rules           []Ranking       `json:"rules"`
	Recent          []SecurityEvent `json:"recent"`
	Trend           []Point         `json:"trend"`
	Countries       []Country       `json:"countries"`
	SourceAvailable bool            `json:"sourceAvailable"`
}
type Snapshot struct {
	AccessSource bool       `json:"accessSource"`
	Enabled      bool       `json:"enabled"`
	Protection   Protection `json:"protection"`
	Period       string     `json:"period"`
	From         time.Time  `json:"from"`
	To           time.Time  `json:"to"`
	GeneratedAt  time.Time  `json:"generatedAt"`
	State        string     `json:"state"`
	Metric
	Trend           []Point    `json:"trend"`
	Countries       []Country  `json:"countries"`
	Paths           []Ranking  `json:"paths"`
	Referrers       []Ranking  `json:"referrers"`
	Hosts           []string   `json:"hosts"`
	LocatedRequests int64      `json:"locatedRequests"`
	GeoStatus       string     `json:"geoStatus"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	CollectedAt     *time.Time `json:"collectedAt,omitempty"`
	CatchingUp      bool       `json:"catchingUp"`
	SiteAttribution bool       `json:"siteAttribution"`
	Warnings        []string   `json:"warnings"`
}
