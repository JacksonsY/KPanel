// Package backupremote transports existing encrypted KPanel packages. It never
// interprets archive payloads or executes host operations.
package backupremote

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

const MaxStorages = 8
const MaxObjects = 1000

var ErrInvalid = errors.New("invalid remote backup configuration")
var ErrConflict = errors.New("backup configuration changed")
var ErrUnavailable = errors.New("remote backup storage unavailable")
var ErrLimit = errors.New("remote backup listing exceeds limit")

type Storage struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket,omitempty"`
	Region    string `json:"region,omitempty"`
	PathStyle bool   `json:"pathStyle,omitempty"`
	Prefix    string `json:"prefix"`
	Username  string `json:"username,omitempty"`
	AccessKey string `json:"accessKey,omitempty"`
	Secret    string `json:"secret,omitempty"`
	HasSecret bool   `json:"hasSecret,omitempty"`
	ClientID  string `json:"clientId,omitempty"`
	TenantID  string `json:"tenantId,omitempty"`
	DriveID   string `json:"driveId,omitempty"`
}

func (s Storage) Public() Storage { s.HasSecret = s.Secret != ""; s.Secret = ""; return s }

// Fingerprint binds receipts to a destination, not to rotating credentials.
func (s Storage) Fingerprint() string {
	b, _ := json.Marshal([]string{s.Kind, s.Endpoint, s.Bucket, s.Prefix})
	if s.Kind == "onedrive" {
		b, _ = json.Marshal([]string{s.Kind, s.Endpoint, s.ClientID, s.TenantID, s.DriveID, s.Prefix})
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (s *Storage) Validate() error {
	if !backup.ValidID(s.ID) || strings.TrimSpace(s.Name) == "" || len(s.Name) > 80 || controls(s.Name) || len(s.Secret) > 4096 || s.Secret == "" || controls(s.Secret) {
		return ErrInvalid
	}
	u, err := url.Parse(s.Endpoint)
	if err != nil || len(s.Endpoint) > 2048 || controls(s.Endpoint) || strings.Contains(s.Endpoint, "\\") || u.Hostname() == "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ErrInvalid
	}
	if strings.Contains(u.Hostname(), "%") || strings.HasSuffix(u.Host, ":") || !validPort(u) {
		return ErrInvalid
	}
	if len(s.Prefix) > 240 || !cleanRelative(s.Prefix, true) {
		return ErrInvalid
	}
	s.Prefix = strings.Trim(s.Prefix, "/")
	s.Endpoint = strings.TrimRight(s.Endpoint, "/")
	switch s.Kind {
	case "onedrive":
		if s.Endpoint != graphEndpoint || !validMicrosoftID(s.ClientID) || !validTenant(s.TenantID) || !validDriveID(s.DriveID) || len(s.Secret) > 4096 {
			return ErrInvalid
		}
		for _, part := range strings.Split(s.Prefix, "/") {
			if strings.ContainsAny(part, `:*<>|"`) || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || part == "" && s.Prefix != "" {
				return ErrInvalid
			}
		}
	case "s3":
		if u.Path != "" && u.Path != "/" || len(s.Bucket) < 3 || len(s.Bucket) > 63 || strings.ContainsAny(s.Bucket, "/\\:%?# ") || controls(s.Bucket) || len(s.AccessKey) > 256 || s.AccessKey == "" || controls(s.AccessKey) || len(s.Region) > 80 || controls(s.Region) {
			return ErrInvalid
		}
		if s.Region == "" {
			s.Region = "us-east-1"
		}
	case "webdav":
		if !cleanRelative(u.Path, true) || s.Username == "" || len(s.Username) > 256 || controls(s.Username) || strings.Contains(s.Username, ":") {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func controls(s string) bool { return strings.IndexFunc(s, unicode.IsControl) >= 0 }
func cleanRelative(s string, slash bool) bool {
	if controls(s) || strings.ContainsAny(s, "\\%?#") {
		return false
	}
	for _, part := range strings.Split(s, "/") {
		if part == "." || part == ".." {
			return false
		}
	}
	return slash || !strings.Contains(s, "/")
}

func ValidObject(name string) bool {
	return name != "" && len(name) <= 160 && cleanRelative(name, false) && path.Base(name) == name && strings.HasSuffix(name, ".kpb")
}

type Schedule struct {
	Enabled     bool      `json:"enabled"`
	Modules     []string  `json:"modules"`
	StorageID   string    `json:"storageId"`
	Frequency   string    `json:"frequency"`
	Hour        int       `json:"hour"`
	Minute      int       `json:"minute"`
	Weekday     int       `json:"weekday"`
	Day         int       `json:"day"`
	Timezone    string    `json:"timezone"`
	Keep        int       `json:"keep"`
	Password    string    `json:"password,omitempty"`
	HasPassword bool      `json:"hasPassword,omitempty"`
	NextRun     time.Time `json:"nextRun,omitempty"`
	LastRun     time.Time `json:"lastRun,omitempty"`
	LastError   string    `json:"lastError,omitempty"`
}

func (s Schedule) Public() Schedule { s.HasPassword = s.Password != ""; s.Password = ""; return s }
func (s Schedule) Validate() error {
	if _, err := backup.Selection(s.Modules); err != nil {
		return ErrInvalid
	}
	if s.StorageID != "" && !backup.ValidID(s.StorageID) {
		return ErrInvalid
	}
	if s.Hour < 0 || s.Hour > 23 || s.Minute < 0 || s.Minute > 59 || s.Weekday < 0 || s.Weekday > 6 || s.Day < 1 || s.Day > 31 || s.Keep < 1 || s.Keep > 20 {
		return ErrInvalid
	}
	if s.Frequency != "daily" && s.Frequency != "weekly" && s.Frequency != "monthly" {
		return ErrInvalid
	}
	if len(s.Timezone) > 80 || s.Timezone == "" || s.Timezone == "Local" {
		return ErrInvalid
	}
	if _, err := time.LoadLocation(s.Timezone); err != nil {
		return ErrInvalid
	}
	if (s.Enabled || s.Password != "") && backup.ValidatePassword(s.Password) != nil {
		return ErrInvalid
	}
	return nil
}

// Next skips nonexistent monthly dates and DST wall times instead of silently
// moving a maintenance window. A repeated DST minute is executed only once.
func (s Schedule) Next(after time.Time) time.Time {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil || !s.Enabled {
		return time.Time{}
	}
	local := after.In(loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, loc)
	for i := 0; i < 370; i++ {
		d := day.AddDate(0, 0, i)
		if s.Frequency == "weekly" && int(d.Weekday()) != s.Weekday || s.Frequency == "monthly" && d.Day() != s.Day {
			continue
		}
		next := time.Date(d.Year(), d.Month(), d.Day(), s.Hour, s.Minute, 0, 0, loc)
		if next.Day() == d.Day() && next.Hour() == s.Hour && next.Minute() == s.Minute && next.After(after) {
			return next.UTC()
		}
	}
	return time.Time{}
}
