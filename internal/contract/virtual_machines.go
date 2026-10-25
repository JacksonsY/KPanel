package contract

import (
	"regexp"
	"strings"
	"time"
)

const VMMaxEntries = 64

var vmUUID = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
var vmName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$`)
var vmVersion = regexp.MustCompile(`^[a-f0-9]{64}$`)
var vmUser = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,30}$`)

func ValidVMID(id string) bool { return vmUUID.MatchString(id) }

type VirtualMachine struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	State           string   `json:"state"`
	CPUs            int      `json:"cpus"`
	MemoryMiB       int64    `json:"memoryMiB"`
	Autostart       bool     `json:"autostart"`
	ResourceVersion string   `json:"resourceVersion"`
	Addresses       []string `json:"addresses"`
	Disks           []VMDisk `json:"disks"`
	Console         bool     `json:"console"`
}

type VMDisk struct {
	Target string `json:"target"`
	Path   string `json:"path"`
	Device string `json:"device"`
}

type VMEnvironment struct {
	Connected    bool     `json:"connected"`
	Supported    bool     `json:"supported"`
	Ready        bool     `json:"ready"`
	Writable     bool     `json:"writable"`
	Installable  bool     `json:"installable"`
	Reason       string   `json:"reason"`
	Architecture string   `json:"architecture"`
	Missing      []string `json:"missing"`
	CPUs         int      `json:"cpus"`
	MemoryMiB    int64    `json:"memoryMiB"`
}

type VMResource struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
	Kind string `json:"kind,omitempty"`
}

type VMSnapshot struct {
	Environment VMEnvironment    `json:"environment"`
	Machines    []VirtualMachine `json:"machines"`
	Networks    []VMResource     `json:"networks"`
	Pools       []VMResource     `json:"pools"`
	Images      []VMResource     `json:"images"`
	Jobs        []VMJob          `json:"jobs"`
	ObservedAt  time.Time        `json:"observedAt"`
	Truncated   bool             `json:"truncated"`
}

// One structured request is shared by the Panel, Agent and durable worker.
// Paths, XML, shell fragments, URLs and command-line options are never accepted.
type VMRequest struct {
	Action                  string `json:"action"`
	RequestID               string `json:"requestId"`
	ID                      string `json:"id,omitempty"`
	Name                    string `json:"name,omitempty"`
	Source                  string `json:"source,omitempty"`
	Image                   string `json:"image,omitempty"`
	Pool                    string `json:"pool,omitempty"`
	Network                 string `json:"network,omitempty"`
	CPUs                    int    `json:"cpus,omitempty"`
	MemoryMiB               int    `json:"memoryMiB,omitempty"`
	DiskGiB                 int    `json:"diskGiB,omitempty"`
	Username                string `json:"username,omitempty"`
	PublicKey               string `json:"publicKey,omitempty"`
	Password                string `json:"password,omitempty"`
	Autostart               bool   `json:"autostart,omitempty"`
	DeleteDisks             bool   `json:"deleteDisks,omitempty"`
	ExpectedResourceVersion string `json:"expectedResourceVersion,omitempty"`
}

type VMJob struct {
	Artifacts []string  `json:"artifacts,omitempty"`
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Name      string    `json:"name"`
	MachineID string    `json:"machineId,omitempty"`
	Status    string    `json:"status"`
	Stage     string    `json:"stage"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func ValidateVMRequest(r *VMRequest) (string, string) {
	if r == nil {
		return "request", "request is required"
	}
	if !ValidVMID(r.RequestID) {
		return "requestId", "a UUID is required"
	}
	if r.Action != "create" {
		allowed := VMRequest{Action: r.Action, RequestID: r.RequestID}
		if r.Action != "install" && r.Action != "prepare" {
			allowed.ID = r.ID
			allowed.ExpectedResourceVersion = r.ExpectedResourceVersion
		}
		if r.Action == "autostart" {
			allowed.Autostart = r.Autostart
		}
		if r.Action == "delete" {
			allowed.DeleteDisks = r.DeleteDisks
		}
		if *r != allowed {
			return "action", "unexpected operation parameters"
		}
	}
	if r.Action == "install" || r.Action == "prepare" {
		if r.ID != "" || r.Name != "" || r.DeleteDisks {
			return "action", "unexpected installation parameters"
		}
		return "", ""
	}
	if r.Action == "create" {
		if !vmName.MatchString(r.Name) {
			return "name", "use 1–63 letters, digits, dots, underscores or hyphens"
		}
		if r.Source != "cloud" && r.Source != "iso" {
			return "source", "select cloud or iso"
		}
		if !vmName.MatchString(r.Pool) || !vmName.MatchString(r.Network) {
			return "pool", "select an available storage pool and network"
		}
		if r.CPUs < 1 || r.CPUs > 256 || r.MemoryMiB < 256 || r.MemoryMiB > 1048576 || r.DiskGiB < 4 || r.DiskGiB > 65536 {
			return "resources", "CPU, memory or disk is outside the supported range"
		}
		if r.Source == "cloud" {
			if r.Image != "debian-12" && r.Image != "ubuntu-24.04" {
				return "image", "select a supported system image"
			}
			if !vmUser.MatchString(r.Username) || len(r.PublicKey) > 8192 || strings.ContainsAny(r.PublicKey, "\r\n\x00") {
				return "publicKey", "a user name and one SSH public key are required"
			}
			parts := strings.Fields(r.PublicKey)
			if r.PublicKey != "" && (len(parts) < 2 || (parts[0] != "ssh-ed25519" && parts[0] != "ssh-rsa" && parts[0] != "ecdsa-sha2-nistp256")) {
				return "publicKey", "provide an OpenSSH public key"
			}
			if r.Password != "" && (len(r.Password) < 12 || len(r.Password) > 72 || strings.ContainsAny(r.Password, "\r\n\x00")) {
				return "password", "use a 12–72 byte password"
			}
			if r.PublicKey == "" && r.Password == "" {
				return "password", "provide a password or SSH public key"
			}
		} else if r.Password != "" || r.PublicKey != "" || r.Username != "" {
			return "source", "ISO installation does not accept cloud credentials"
		} else if !vmName.MatchString(r.Image) || !strings.HasSuffix(strings.ToLower(r.Image), ".iso") {
			return "image", "select an ISO volume from the storage pool"
		}
		if r.ID != "" || r.DeleteDisks || r.ExpectedResourceVersion != "" {
			return "action", "unexpected creation parameters"
		}
		return "", ""
	}
	if !ValidVMID(r.ID) || !vmVersion.MatchString(r.ExpectedResourceVersion) {
		return "id", "select a current virtual machine and refresh its state"
	}
	switch r.Action {
	case "start", "shutdown", "reboot", "poweroff", "delete", "autostart", "eject":
	default:
		return "action", "unsupported virtual machine action"
	}
	if r.DeleteDisks && r.Action != "delete" {
		return "deleteDisks", "disk removal requires delete"
	}
	return "", ""
}
