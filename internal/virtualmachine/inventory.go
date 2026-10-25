package virtualmachine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

type Config struct {
	StateDir, Executable string
	Enabled              bool
	Runner               Runner
}
type Service struct {
	dir, executable string
	enabled         bool
	runner          Runner
	mu              sync.Mutex
	reads           chan struct{}
}

func New(c Config) *Service {
	if c.Runner == nil {
		c.Runner = CommandRunner{}
	}
	if c.StateDir == "" {
		c.StateDir = "/var/lib/kejilion-panel/virtual-machines"
	}
	if c.Executable == "" {
		c.Executable, _ = os.Executable()
	}
	return &Service{dir: c.StateDir, executable: c.Executable, enabled: c.Enabled, runner: c.Runner, reads: make(chan struct{}, 2)}
}

func (s *Service) Environment(ctx context.Context) contract.VMEnvironment {
	e := contract.VMEnvironment{Supported: runtime.GOOS == "linux" && (runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64"), Writable: s.enabled, Architecture: runtime.GOARCH, CPUs: runtime.NumCPU(), Missing: []string{}}
	if !e.Supported {
		e.Reason = "platform_unsupported"
		return e
	}
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		e.MemoryMiB = propertiesMemory(b)
	}
	for _, tool := range []string{"virsh", "virt-install", "qemu-img", "cloud-localds"} {
		if _, err := s.runner.LookPath(tool); err != nil {
			e.Missing = append(e.Missing, tool)
		}
	}
	_, aptErr := s.runner.LookPath("apt-get")
	_, initErr := s.runner.LookPath("systemctl")
	e.Installable = aptErr == nil && initErr == nil && s.enabled
	if len(e.Missing) > 0 {
		e.Reason = "dependencies_missing"
		return e
	}
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	capabilities, err := s.virsh(probe, "domcapabilities", "--virttype", "kvm")
	if err != nil {
		e.Reason = "kvm_unavailable"
		return e
	}
	if !strings.Contains(string(capabilities), "<domain>kvm</domain>") {
		e.Reason = "kvm_unavailable"
		return e
	}
	e.Ready = true
	return e
}

func propertiesMemory(b []byte) int64 {
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			p := strings.Fields(line)
			if len(p) > 1 {
				n, _ := strconv.ParseInt(p[1], 10, 64)
				return n / 1024
			}
		}
	}
	return 0
}

func (s *Service) Snapshot(ctx context.Context) (contract.VMSnapshot, error) {
	select {
	case s.reads <- struct{}{}:
		defer func() { <-s.reads }()
	default:
		return contract.VMSnapshot{}, ErrBusy
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	r := contract.VMSnapshot{Environment: s.Environment(ctx), Machines: []contract.VirtualMachine{}, Networks: []contract.VMResource{}, Pools: []contract.VMResource{}, Images: []contract.VMResource{}, Jobs: []contract.VMJob{}, ObservedAt: time.Now().UTC()}
	jobs, err := s.Jobs(ctx)
	if err != nil {
		return r, err
	}
	r.Jobs = jobs
	if !r.Environment.Ready {
		return r, nil
	}
	ids, err := s.virsh(ctx, "list", "--all", "--uuid")
	if err != nil {
		return r, err
	}
	for _, id := range strings.Fields(string(ids)) {
		if !contract.ValidVMID(id) {
			return r, errors.New("invalid libvirt domain identity")
		}
		if len(r.Machines) >= contract.VMMaxEntries {
			r.Truncated = true
			break
		}
		m, err := s.Machine(ctx, id)
		if err != nil {
			return r, err
		}
		r.Machines = append(r.Machines, m)
	}
	sort.Slice(r.Machines, func(i, j int) bool { return r.Machines[i].Name < r.Machines[j].Name })
	for _, item := range []struct {
		command string
		target  *[]contract.VMResource
	}{{"net-list", &r.Networks}, {"pool-list", &r.Pools}} {
		b, err := s.virsh(ctx, item.command, "--name")
		if err != nil {
			return r, err
		}
		for _, name := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if name == "" {
				continue
			}
			if len(*item.target) >= 64 {
				r.Truncated = true
				break
			}
			*item.target = append(*item.target, contract.VMResource{Name: name})
		}
	}
	for _, pool := range r.Pools {
		b, err := s.virsh(ctx, "vol-list", "--pool", pool.Name, "--name")
		if err != nil {
			return r, err
		}
		for _, name := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if !strings.HasSuffix(strings.ToLower(name), ".iso") {
				continue
			}
			if len(r.Images) >= 128 {
				r.Truncated = true
				break
			}
			r.Images = append(r.Images, contract.VMResource{Name: name, Path: pool.Name, Kind: "iso"})
		}
	}
	return r, nil
}

type domainXML struct {
	XMLName xml.Name `xml:"domain"`
	UUID    string   `xml:"uuid"`
	Name    string   `xml:"name"`
	Devices struct {
		Disks []struct {
			Device string `xml:"device,attr"`
			Target struct {
				Dev string `xml:"dev,attr"`
			} `xml:"target"`
			Source struct {
				File string `xml:"file,attr"`
			} `xml:"source"`
		} `xml:"disk"`
		Graphics []struct {
			Type   string `xml:"type,attr"`
			Port   int    `xml:"port,attr"`
			Listen string `xml:"listen,attr"`
			Socket string `xml:"socket,attr"`
		} `xml:"graphics"`
	} `xml:"devices"`
}

func (s *Service) inspect(ctx context.Context, id string) (contract.VirtualMachine, domainXML, error) {
	if !contract.ValidVMID(id) {
		return contract.VirtualMachine{}, domainXML{}, ErrInvalid
	}
	b, err := s.virsh(ctx, "dumpxml", id)
	if err != nil {
		return contract.VirtualMachine{}, domainXML{}, err
	}
	var d domainXML
	if err = xml.Unmarshal(b, &d); err != nil || d.UUID != id {
		return contract.VirtualMachine{}, d, errors.New("libvirt domain identity changed")
	}
	info, err := s.virsh(ctx, "dominfo", id)
	if err != nil {
		return contract.VirtualMachine{}, d, err
	}
	p := map[string]string{}
	for _, line := range strings.Split(string(info), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if ok {
			p[k] = strings.TrimSpace(v)
		}
	}
	m := contract.VirtualMachine{ID: id, Name: d.Name, State: p["State"], Autostart: p["Autostart"] == "enable", Addresses: []string{}, Disks: []contract.VMDisk{}}
	m.CPUs, _ = strconv.Atoi(p["CPU(s)"])
	fields := strings.Fields(p["Max memory"])
	if len(fields) > 0 {
		m.MemoryMiB, _ = strconv.ParseInt(fields[0], 10, 64)
		m.MemoryMiB /= 1024
	}
	// Compare inactive configuration rather than volatile VNC ports, guest
	// memory statistics or host-assigned devices when guarding a write.
	config, err := s.virsh(ctx, "dumpxml", id, "--inactive")
	if err != nil {
		config = b
	}
	digest := sha256.Sum256(append(config, []byte("\x00"+m.State+"\x00"+p["Autostart"])...))
	m.ResourceVersion = hex.EncodeToString(digest[:])
	for _, disk := range d.Devices.Disks {
		m.Disks = append(m.Disks, contract.VMDisk{Target: disk.Target.Dev, Device: disk.Device, Path: disk.Source.File})
	}
	for _, g := range d.Devices.Graphics {
		if g.Type == "vnc" {
			m.Console = true
		}
	}
	return m, d, nil
}

func (s *Service) Machine(ctx context.Context, id string) (contract.VirtualMachine, error) {
	m, _, err := s.inspect(ctx, id)
	if err == nil && m.State == "running" {
		lookup, cancel := context.WithTimeout(ctx, 2*time.Second)
		output, lookupErr := s.virsh(lookup, "domifaddr", id, "--source", "lease")
		cancel()
		if lookupErr == nil {
			for _, line := range strings.Split(string(output), "\n") {
				p := strings.Fields(line)
				if len(p) == 4 {
					ip, _, e := net.ParseCIDR(p[3])
					if e == nil && len(m.Addresses) < 8 {
						m.Addresses = append(m.Addresses, ip.String())
					}
				}
			}
		}
	}
	return m, err
}

func (s *Service) Console(ctx context.Context, id string) (net.Conn, error) {
	m, d, err := s.inspect(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.State != "running" {
		return nil, ErrConflict
	}
	for _, g := range d.Devices.Graphics {
		if g.Type != "vnc" {
			continue
		}
		dial := net.Dialer{Timeout: 3 * time.Second}
		if g.Socket != "" {
			return nil, fmt.Errorf("%w: Unix VNC sockets are not yet supported", ErrUnsupported)
		}
		ip := net.ParseIP(g.Listen)
		if ip == nil || !ip.IsLoopback() || g.Port < 5900 || g.Port > 65535 {
			return nil, fmt.Errorf("%w: VNC must listen on loopback", ErrUnsupported)
		}
		return dial.DialContext(ctx, "tcp", net.JoinHostPort(g.Listen, strconv.Itoa(g.Port)))
	}
	return nil, ErrUnsupported
}

func (s *Service) poolPath(ctx context.Context, name string) (string, error) {
	b, err := s.virsh(ctx, "pool-dumpxml", name)
	if err != nil {
		return "", err
	}
	var p struct {
		Type   string `xml:"type,attr"`
		Target struct {
			Path string `xml:"path"`
		} `xml:"target"`
	}
	if err = xml.Unmarshal(b, &p); err != nil {
		return "", err
	}
	if p.Type != "dir" || !filepath.IsAbs(p.Target.Path) || filepath.Clean(p.Target.Path) == "/" {
		return "", fmt.Errorf("%w: select a directory storage pool", ErrUnsupported)
	}
	if err = trustedPoolDirectory(p.Target.Path); err != nil {
		return "", err
	}
	return p.Target.Path, nil
}

// A 0755 directory owned by a local user is still writable by that user.
// Check the whole path so the privileged worker cannot be redirected while
// preparing an image. Root-owned sticky ancestors (e.g. /tmp) protect their
// root-owned children from replacement and are permitted.
func trustedPoolDirectory(path string) error {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !rootOwned(info) {
			return errors.New("storage pool path must use root-owned real directories")
		}
		if info.Mode().Perm()&0022 != 0 && (current == filepath.Clean(path) || info.Mode()&os.ModeSticky == 0) {
			return errors.New("storage pool path is writable outside the root trust boundary")
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}
