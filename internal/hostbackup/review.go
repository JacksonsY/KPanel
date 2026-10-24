package hostbackup

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/kejilion/kejilion-panel/internal/backup"
	"github.com/kejilion/kejilion-panel/internal/redact"
)

// ErrForeignUnconfirmed rejects restoring a package this Agent did not export
// until the administrator has seen its host configuration and confirmed once.
var ErrForeignUnconfirmed = errors.New("restoring a foreign backup needs confirmation of its host configuration")

// ErrReviewUnavailable means the current Agent/import cannot enumerate the
// configuration to be restored. Confirmation cannot supply missing details.
var ErrReviewUnavailable = errors.New("update the Agent and reimport the backup to review its complete configuration")

const (
	OriginLocal   = "local"
	OriginForeign = "foreign"

	reviewFile     = "import-review.json"
	maxReviewBytes = 6 << 20
	maxReviewList  = 64
)

// ImportReview is kept beside an import job, not in its record, so older
// Panels and Agents still read the record. Details are only listed for
// foreign modules; local modules keep the previous restore experience.
type ImportReview struct {
	Version  int            `json:"version"`
	Revision string         `json:"revision"`
	Modules  []ModuleReview `json:"modules"`
}
type ModuleReview struct {
	Module string `json:"module"`
	Origin string `json:"origin"`
	// Unreviewed marks an import checked before reviews existed.
	Unreviewed bool              `json:"unreviewed,omitempty"`
	SetIDFiles int               `json:"setIdFiles,omitempty"`
	Roots      []RootReview      `json:"roots,omitempty"`
	Containers []ContainerReview `json:"containers,omitempty"`
	Networks   []NetworkReview   `json:"networks,omitempty"`
	Volumes    []VolumeReview    `json:"volumes,omitempty"`
}

// RootReview is a destination directory the restore replaces. SystemPath
// marks one outside /home that is not a Docker volume; UsedBy lists the
// destination containers that currently mount it.
type RootReview struct {
	Path       string     `json:"path"`
	SystemPath bool       `json:"systemPath,omitempty"`
	Existing   bool       `json:"existing,omitempty"`
	UsedBy     []RootUser `json:"usedBy,omitempty"`
}
type RootUser struct {
	Container string `json:"container"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}
type ContainerReview struct {
	Name           string          `json:"name"`
	Image          string          `json:"image"`
	ImageID        string          `json:"imageId"`
	ImageRef       string          `json:"imageRef,omitempty"`
	HostConfig     json.RawMessage `json:"hostConfig"`
	Replaces       bool            `json:"replaces,omitempty"`
	Privileged     bool            `json:"privileged,omitempty"`
	CapAdd         []string        `json:"capAdd,omitempty"`
	Devices        []string        `json:"devices,omitempty"`
	HostNamespaces []string        `json:"hostNamespaces,omitempty"`
	SecurityOpt    []string        `json:"securityOpt,omitempty"`
	Binds          []BindReview    `json:"binds,omitempty"`
	MoreItems      int             `json:"moreItems,omitempty"`
}

// BindReview is a host directory given to a restored container. Live means
// the package does not restore it: the container gets the host's current
// directory, which includes system paths such as / or /etc.
type BindReview struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ReadOnly    bool   `json:"readOnly,omitempty"`
	Live        bool   `json:"live,omitempty"`
}
type VolumeReview struct {
	Name   string          `json:"name"`
	Driver string          `json:"driver"`
	Config json.RawMessage `json:"config"`
}

type NetworkReview struct {
	Name     string          `json:"name"`
	Driver   string          `json:"driver"`
	Internal bool            `json:"internal,omitempty"`
	Subnets  []string        `json:"subnets,omitempty"`
	Options  []string        `json:"options,omitempty"`
	Config   json.RawMessage `json:"config"`
}

func (r ImportReview) Complete(modules []string) bool {
	for _, module := range modules {
		found := false
		for _, m := range r.Modules {
			if m.Module == module && !m.Unreviewed && (m.Origin == OriginLocal || m.Origin == OriginForeign) {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (r ImportReview) Foreign(modules []string) bool {
	for _, module := range modules {
		local := false
		for _, m := range r.Modules {
			if m.Module == module && m.Origin == OriginLocal && !m.Unreviewed {
				local = true
			}
		}
		if !local {
			return true
		}
	}
	return false
}

func moduleReview(p Payload, facts payloadFacts, restored []string) ModuleReview {
	m := ModuleReview{Module: p.Module, Origin: OriginLocal}
	if facts.Local {
		return m
	}
	m.Origin = OriginForeign
	m.SetIDFiles = facts.SetIDFiles
	for _, r := range p.Roots {
		m.Roots = append(m.Roots, RootReview{Path: r.Path, SystemPath: !within("/home", r.Path) && !declaredLocalVolumeRoot(p, r)})
	}
	for _, c := range p.Containers {
		m.Containers = append(m.Containers, containerReview(c, restored))
	}
	names := make([]string, 0, len(p.Networks))
	for name := range p.Networks {
		if name != "host" && name != "none" && name != "bridge" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		m.Networks = append(m.Networks, networkReview(name, p.Networks[name]))
	}
	names = names[:0]
	for name := range p.Volumes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		v := p.Volumes[name]
		config, _ := json.Marshal(volumeCreateBody(v))
		m.Volumes = append(m.Volumes, VolumeReview{Name: name, Driver: v.Driver, Config: reviewJSON(config)})
	}
	sanitizeModuleReview(&m)
	return m
}

func containerReview(c Container, restored []string) ContainerReview {
	out := ContainerReview{Name: c.Name, ImageID: c.ImageID, ImageRef: c.ImageRef}
	_ = json.Unmarshal(c.Config["Image"], &out.Image)
	// Decode the exact HostConfig body Restore sends the way the Docker Engine
	// does (encoding/json), so the review cannot differ from what runs.
	var host struct {
		Privileged                                    bool
		CapAdd, SecurityOpt, DeviceCgroupRules, Binds []string
		Devices                                       []struct{ PathOnHost, PathInContainer, CgroupPermissions string }
		Mounts                                        []struct {
			Type, Source, Target string
			ReadOnly             bool
		}
		NetworkMode, PidMode, IpcMode, UTSMode, UsernsMode, CgroupnsMode string
	}
	if hostConfig, err := nativeMountConfig(c); err == nil {
		raw, _ := json.Marshal(hostConfig)
		out.HostConfig = reviewJSON(raw)
		_ = json.Unmarshal(raw, &host)
	}
	out.Privileged = host.Privileged
	out.CapAdd = boundedList(host.CapAdd, &out.MoreItems)
	out.SecurityOpt = boundedList(host.SecurityOpt, &out.MoreItems)
	devices := []string{}
	for _, d := range host.Devices {
		devices = append(devices, d.PathOnHost+":"+d.PathInContainer+":"+d.CgroupPermissions)
	}
	for _, rule := range host.DeviceCgroupRules {
		devices = append(devices, "cgroup "+rule)
	}
	out.Devices = boundedList(devices, &out.MoreItems)
	for _, mode := range []struct{ Value, Name string }{{host.NetworkMode, "network"}, {host.PidMode, "pid"}, {host.IpcMode, "ipc"}, {host.UTSMode, "uts"}, {host.UsernsMode, "userns"}, {host.CgroupnsMode, "cgroup"}} {
		if mode.Value == "host" {
			out.HostNamespaces = append(out.HostNamespaces, mode.Name)
		}
	}
	bind := func(source, target string, readOnly bool) {
		covered := false
		for _, root := range restored {
			if within(root, source) {
				covered = true
			}
		}
		out.Binds = append(out.Binds, BindReview{Source: source, Destination: target, ReadOnly: readOnly, Live: !covered})
	}
	for _, b := range host.Binds {
		parts := strings.Split(b, ":")
		if len(parts) < 2 || !path.IsAbs(parts[0]) {
			continue // named volume
		}
		readOnly := len(parts) == 3 && slices.Contains(strings.Split(parts[2], ","), "ro")
		bind(parts[0], parts[1], readOnly)
	}
	for _, m := range host.Mounts {
		if m.Type == "bind" {
			bind(m.Source, m.Target, m.ReadOnly)
		}
	}
	return out
}

func networkReview(name string, raw json.RawMessage) NetworkReview {
	out := NetworkReview{Name: name}
	var config struct {
		Driver   string
		Internal bool
		IPAM     struct {
			Config []struct{ Subnet, IPRange, Gateway string }
		}
		Options map[string]string
	}
	input, err := networkCreateBody(name, raw)
	if err != nil {
		return out
	}
	body, _ := json.Marshal(input)
	out.Config = reviewJSON(body)
	_ = json.Unmarshal(body, &config)
	out.Driver, out.Internal = config.Driver, config.Internal
	more := 0
	subnets := []string{}
	for _, c := range config.IPAM.Config {
		described := c.Subnet
		if c.Gateway != "" {
			described += " gw " + c.Gateway
		}
		if c.IPRange != "" {
			described += " range " + c.IPRange
		}
		subnets = append(subnets, described)
	}
	out.Subnets = boundedList(subnets, &more)
	options := []string{}
	for key, value := range config.Options {
		options = append(options, key+"="+value)
	}
	sort.Strings(options)
	out.Options = boundedList(options, &more)
	out.Options = redact.Records(out.Options)
	return out
}

func boundedList(values []string, more *int) []string {
	// The complete review is bounded by maxReviewBytes. Never silently omit a
	// risky entry from the configuration the administrator is confirming.
	return values
}

// annotateReview records what the restore would replace on this destination
// now. It is recomputed whenever the restore revision is refreshed.
func (e *Engine) annotateReview(r *ImportReview, current Inventory) {
	r.Revision = current.Revision
	names := map[string]bool{}
	for _, c := range current.Containers {
		names[c.Name] = true
	}
	for _, c := range current.ProtectedContainers {
		names[c.Name] = true
	}
	for n := range r.Modules {
		m := &r.Modules[n]
		for k := range m.Containers {
			m.Containers[k].Replaces = names[m.Containers[k].Name]
		}
		for k := range m.Roots {
			root := &m.Roots[k]
			_, err := os.Lstat(e.host(root.Path))
			root.Existing = err == nil
			root.UsedBy = nil
			for _, c := range current.Containers {
				for _, mount := range c.Mounts {
					if (mount.Type == "bind" || mount.Type == "volume") && (within(root.Path, mount.Source) || within(mount.Source, root.Path)) {
						root.UsedBy = append(root.UsedBy, RootUser{Container: c.Name, ReadOnly: !mount.RW})
					}
				}
			}
		}
	}
}

func saveReview(directory string, r ImportReview) error {
	r.Version = 2
	for i := range r.Modules {
		sanitizeModuleReview(&r.Modules[i])
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if len(data) > maxReviewBytes {
		return errors.New("backup host configuration is too large to review")
	}
	return backup.AtomicFile(filepath.Join(directory, reviewFile), data)
}

// Reviews are display projections only. Restore always reads the original
// payload; a projected HostConfig must never become an execution input.
func reviewJSON(raw json.RawMessage) json.RawMessage {
	safe, err := redact.JSON(raw)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return safe
}

// loadReview treats a missing review as an import checked by an older
// Agent: every module is foreign without details until it is imported again.
func loadReview(directory string, modules []string) (ImportReview, error) {
	data, err := backup.ReadFile(filepath.Join(directory, reviewFile), maxReviewBytes)
	if errors.Is(err, os.ErrNotExist) {
		r := ImportReview{Version: 1}
		for _, module := range modules {
			r.Modules = append(r.Modules, ModuleReview{Module: module, Origin: OriginForeign, Unreviewed: true})
		}
		return r, nil
	}
	if err != nil {
		return ImportReview{}, err
	}
	var r ImportReview
	if backup.Decode(data, &r) != nil || r.Version != 1 && r.Version != 2 {
		return ImportReview{}, backup.ErrInvalid
	}
	if r.Version == 1 {
		for n := range r.Modules {
			if r.Modules[n].Origin == OriginForeign {
				r.Modules[n].Unreviewed = true
			}
		}
	}
	for _, m := range r.Modules {
		if !slices.Contains(modules, m.Module) || m.Origin != OriginLocal && m.Origin != OriginForeign {
			return ImportReview{}, backup.ErrInvalid
		}
	}
	for i := range r.Modules {
		sanitizeModuleReview(&r.Modules[i])
	}
	safe, err := json.Marshal(r)
	if err != nil {
		return ImportReview{}, err
	}
	// Old Agent versions persisted raw host configuration. Scrub that copy
	// before returning it, and fail closed if the private file cannot be updated.
	if !bytes.Equal(data, safe) {
		if err := backup.AtomicFile(filepath.Join(directory, reviewFile), safe); err != nil {
			return ImportReview{}, err
		}
	}
	return r, nil
}
