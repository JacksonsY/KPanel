package contract

import (
	"regexp"
	"strings"
	"time"
)

const (
	SystemServiceMaxEntries     = 2000
	SystemServiceLogLines       = 100
	SystemServiceOutputLimit    = 1 << 20
	SystemServiceLogOutputLimit = 256 << 10
)

var systemServiceNamePattern = regexp.MustCompile(`^(?:[A-Za-z0-9_.:@-]|\\x[0-9a-fA-F]{2})+\.service$`)
var systemServiceVersionPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type SystemService struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	LoadState     string `json:"loadState"`
	ActiveState   string `json:"activeState"`
	SubState      string `json:"subState"`
	UnitFileState string `json:"unitFileState"`
}

type SystemServicesSnapshot struct {
	Supported  bool            `json:"supported"`
	Reason     string          `json:"reason,omitempty"`
	Services   []SystemService `json:"services"`
	ObservedAt time.Time       `json:"observedAt"`
	Truncated  bool            `json:"truncated"`
}

type SystemServiceDetail struct {
	SystemService
	ResourceVersion string `json:"resourceVersion"`
	FragmentPath    string `json:"fragmentPath"`
	MainPID         int    `json:"mainPID"`
	CanReload       bool   `json:"canReload"`
	CanStart        bool   `json:"canStart"`
	CanStop         bool   `json:"canStop"`
}

type SystemServiceLogs struct {
	Name       string    `json:"name"`
	Content    string    `json:"content"`
	Truncated  bool      `json:"truncated"`
	ObservedAt time.Time `json:"observedAt"`
}

type SystemServiceActionRequest struct {
	Name                    string `json:"name"`
	Action                  string `json:"action"`
	ExpectedResourceVersion string `json:"expectedResourceVersion"`
}

type SystemServiceActionResult struct {
	Name     string               `json:"name"`
	Action   string               `json:"action"`
	Accepted bool                 `json:"accepted"`
	Message  string               `json:"message"`
	Service  *SystemServiceDetail `json:"service,omitempty"`
}

func ValidSystemServiceName(name string) bool {
	return len(name) > len(".service") && len(name) <= 255 &&
		!strings.HasPrefix(name, "-") && systemServiceNamePattern.MatchString(name)
}

// ParseSystemServicePath accepts only one decoded unit-name segment and the
// fixed optional logs suffix. A unit name is data, never a path or glob.
func ParseSystemServicePath(path, prefix string) (name string, logs bool, ok bool) {
	if !strings.HasPrefix(path, prefix+"/") {
		return "", false, false
	}
	name = strings.TrimPrefix(path, prefix+"/")
	if strings.HasSuffix(name, "/logs") {
		name = strings.TrimSuffix(name, "/logs")
		logs = true
	}
	return name, logs, ValidSystemServiceName(name)
}

func ValidateSystemServiceAction(input *SystemServiceActionRequest) (string, string) {
	if input == nil {
		return "request", "request is required"
	}
	if !ValidSystemServiceName(input.Name) {
		return "name", "name must be a single systemd .service unit"
	}
	switch input.Action {
	case "start", "stop", "restart", "reload", "enable", "disable":
	default:
		return "action", "action must be start, stop, restart, reload, enable or disable"
	}
	if !systemServiceVersionPattern.MatchString(input.ExpectedResourceVersion) {
		return "expectedResourceVersion", "expectedResourceVersion must be 64 lowercase hexadecimal characters"
	}
	return "", ""
}
