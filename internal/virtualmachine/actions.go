package virtualmachine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

func (s *Service) install(ctx context.Context, stage func(string) error) error {
	e := s.Environment(ctx)
	if !e.Supported || !e.Installable {
		return ErrUnsupported
	}
	if err := stage("installing"); err != nil {
		return err
	}
	if _, err := s.runner.Run(ctx, nil, "apt-get", "update"); err != nil {
		return err
	}
	qemu := "qemu-system-x86"
	if runtime.GOARCH == "arm64" {
		qemu = "qemu-system-arm"
	}
	if _, err := s.runner.Run(ctx, nil, "apt-get", "install", "-y", "--no-install-recommends", qemu, "qemu-utils", "libvirt-daemon-system", "libvirt-clients", "virtinst", "cloud-image-utils", "dnsmasq-base", "ovmf"); err != nil {
		return err
	}
	if err := stage("configuring"); err != nil {
		return err
	}
	if _, err := s.runner.Run(ctx, nil, "systemctl", "enable", "--now", "libvirtd.service"); err != nil {
		return err
	}
	if err := s.prepare(ctx, stage); err != nil {
		return err
	}
	if !s.Environment(ctx).Ready {
		return errors.New("runtime installed; KVM is unavailable on this host")
	}
	return nil
}

func (s *Service) prepare(ctx context.Context, stage func(string) error) error {
	if err := stage("configuring"); err != nil {
		return err
	}
	if _, err := s.virsh(ctx, "pool-info", "default"); err != nil {
		if err := os.MkdirAll("/var/lib/libvirt/images", 0755); err != nil {
			return err
		}
		if _, err = s.virsh(ctx, "pool-define-as", "default", "dir", "--target", "/var/lib/libvirt/images"); err != nil {
			return err
		}
	}
	if _, err := s.virsh(ctx, "pool-start", "default"); err != nil {
		b, inspectErr := s.virsh(ctx, "pool-info", "default")
		if inspectErr != nil || !hasInfoValue(b, "State", "running") {
			return err
		}
	}
	if _, err := s.virsh(ctx, "pool-autostart", "default"); err != nil {
		return err
	}
	// Use the distribution's supplied libvirt network; do not rewrite host
	// interfaces, invent firewall templates or silently replace existing XML.
	if _, err := s.virsh(ctx, "net-start", "default"); err != nil {
		b, inspectErr := s.virsh(ctx, "net-info", "default")
		if inspectErr != nil || !hasInfoValue(b, "Active", "yes") {
			return fmt.Errorf("default network unavailable: %w", err)
		}
	}
	if _, err := s.virsh(ctx, "net-autostart", "default"); err != nil {
		return err
	}
	return nil
}

func hasInfoValue(info []byte, key, value string) bool {
	for _, line := range strings.Split(string(info), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v) == value
		}
	}
	return false
}

func (s *Service) create(ctx context.Context, j *jobRecord, stage func(string) error) error {
	r := j.Request
	if err := stage("checking"); err != nil {
		return err
	}
	pool, err := s.poolPath(ctx, r.Pool)
	if err != nil {
		return err
	}
	// virt-install has its own comma-separated suboption grammar. File paths
	// are validated independently of OS command argument separation.
	if strings.ContainsAny(pool, ",\r\n\x00") {
		return ErrUnsupported
	}
	if _, err = s.virsh(ctx, "net-info", r.Network); err != nil {
		return err
	}
	iso := ""
	if r.Source == "iso" {
		if _, err = s.virsh(ctx, "pool-refresh", r.Pool); err != nil {
			return err
		}
		isoBytes, err := s.virsh(ctx, "vol-path", "--pool", r.Pool, r.Image)
		if err != nil {
			return err
		}
		iso = strings.TrimSpace(string(isoBytes))
		if iso != filepath.Join(pool, r.Image) {
			return ErrConflict
		}
	}
	if names, err := s.virsh(ctx, "list", "--all", "--name"); err != nil {
		return err
	} else {
		for _, name := range strings.Split(strings.TrimSpace(string(names)), "\n") {
			if name == r.Name {
				return ErrConflict
			}
		}
	}
	volume := "kpanel-" + j.MachineID + ".qcow2"
	if err = stage("preparing_disk"); err != nil {
		return err
	}
	if _, err = s.virsh(ctx, "vol-create-as", r.Pool, volume, strconv.Itoa(r.DiskGiB)+"G", "--format", "qcow2"); err != nil {
		return err
	}
	pathBytes, err := s.virsh(ctx, "vol-path", "--pool", r.Pool, volume)
	if err != nil {
		return err
	}
	disk := strings.TrimSpace(string(pathBytes))
	if disk != filepath.Join(pool, volume) {
		return errors.New("created disk identity differs from selected pool")
	}
	j.Artifacts = append(j.Artifacts, disk)
	if err = s.save(j); err != nil {
		return err
	}
	// Every generated artifact has a unique task/domain identity. Preserve it
	// on failure and report needs_attention; never guess whether a disk is safe
	// to remove after a lost libvirt acknowledgement.
	args := []string{"--connect", "qemu:///system", "--name", r.Name, "--uuid", j.MachineID, "--memory", strconv.Itoa(r.MemoryMiB), "--vcpus", strconv.Itoa(r.CPUs), "--disk", "path=" + disk + ",format=qcow2,bus=virtio", "--network", "network=" + r.Network + ",model=virtio", "--graphics", "vnc,listen=127.0.0.1", "--osinfo", "detect=on,require=off", "--noautoconsole", "--wait", "0"}
	if r.Source == "cloud" {
		if err = stage("downloading"); err != nil {
			return err
		}
		image, err := s.downloadImage(ctx, r.Image)
		if err != nil {
			return err
		}
		defer os.Remove(image)
		if err = validateCloudDisk(image, int64(r.DiskGiB)<<30); err != nil {
			return err
		}
		if err = trustedPoolDirectory(pool); err != nil {
			return err
		}
		if err = stage("preparing_disk"); err != nil {
			return err
		}
		if _, err = s.runner.Run(ctx, nil, "qemu-img", "convert", "-n", "-f", "qcow2", "-O", "qcow2", image, disk); err != nil {
			return err
		}
		if _, err = s.runner.Run(ctx, nil, "qemu-img", "resize", "-f", "qcow2", disk, strconv.Itoa(r.DiskGiB)+"G"); err != nil {
			return err
		}
		seed := filepath.Join(pool, "kpanel-"+j.MachineID+"-seed.iso")
		seedFile, err := os.OpenFile(seed, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		if err = seedFile.Close(); err != nil {
			return err
		}
		j.Artifacts = append(j.Artifacts, seed)
		if err = s.save(j); err != nil {
			return err
		}
		user, _ := json.Marshal(r.Username)
		key, _ := json.Marshal(r.PublicKey)
		data := fmt.Sprintf("#cloud-config\nusers:\n  - name: %s\n    groups: [sudo]\n    shell: /bin/bash\n    sudo: [\"ALL=(ALL) NOPASSWD:ALL\"]\n", user)
		if j.PasswordHash != "" {
			hash, _ := json.Marshal(j.PasswordHash)
			data += fmt.Sprintf("    lock_passwd: false\n    hashed_passwd: %s\n", hash)
		} else {
			data += "    lock_passwd: true\n"
		}
		if r.PublicKey != "" {
			data += fmt.Sprintf("    ssh_authorized_keys: [%s]\n", key)
		}
		data += fmt.Sprintf("ssh_pwauth: %t\ndisable_root: %t\n", j.PasswordHash != "", r.Username != "root")
		temp, err := os.CreateTemp(s.dir, ".cloud-init-*")
		if err != nil {
			return err
		}
		defer os.Remove(temp.Name())
		if _, err = temp.WriteString(data); err != nil {
			temp.Close()
			return err
		}
		if err = temp.Close(); err != nil {
			return err
		}
		metadata, err := os.CreateTemp(s.dir, ".cloud-init-*")
		if err != nil {
			return err
		}
		defer os.Remove(metadata.Name())
		metadataJSON, _ := json.Marshal(map[string]string{"instance-id": j.MachineID, "local-hostname": strings.ReplaceAll(strings.ToLower(r.Name), "_", "-")})
		if _, err = metadata.Write(metadataJSON); err != nil {
			metadata.Close()
			return err
		}
		if err = metadata.Close(); err != nil {
			return err
		}
		if _, err = s.runner.Run(ctx, nil, "cloud-localds", seed, temp.Name(), metadata.Name()); err != nil {
			return err
		}
		poolRoot, err := os.OpenRoot(pool)
		if err != nil {
			return err
		}
		privateSeed, err := poolRoot.Open(filepath.Base(seed))
		poolRoot.Close()
		if err != nil {
			return err
		}
		if info, e := privateSeed.Stat(); e != nil || !info.Mode().IsRegular() || !rootOwned(info) {
			privateSeed.Close()
			return errors.New("cloud-init seed identity changed")
		}
		err = privateSeed.Chmod(0600)
		privateSeed.Close()
		if err != nil {
			return err
		}
		args = append(args, "--import", "--disk", "path="+seed+",device=cdrom")
	} else {
		args = append(args, "--cdrom", iso)
	}
	if err = stage("creating"); err != nil {
		return err
	}
	if _, err = s.runner.Run(ctx, nil, "virt-install", args...); err != nil {
		return err
	}
	if err = stage("verifying"); err != nil {
		return err
	}
	m, err := s.Machine(ctx, j.MachineID)
	if err != nil {
		return err
	}
	if m.Name != r.Name {
		return ErrConflict
	}
	if r.Source == "cloud" {
		if err = s.recordCloudOS(ctx, j.MachineID, r.Image, m.State); err != nil {
			return err
		}
	}
	if r.Autostart {
		if _, err = s.virsh(ctx, "autostart", j.MachineID); err != nil {
			return err
		}
	}
	return nil
}

// Store the known image identity in libvirt itself, so renaming the VM or
// pruning task history does not discard it. ISO installs retain osinfo detection.
func (s *Service) recordCloudOS(ctx context.Context, id, image, state string) error {
	osID := map[string]string{"debian-12": "http://debian.org/debian/12", "ubuntu-24.04": "http://ubuntu.com/ubuntu/24.04"}[image]
	if !contract.ValidVMID(id) || osID == "" {
		return ErrInvalid
	}
	// libvirt injects the --key namespace into every element. Predeclaring that
	// prefix makes virXMLInjectNamespace fail on supported libvirt versions.
	metadata := `<libosinfo><os id="` + osID + `"/></libosinfo>`
	args := []string{"metadata", id, "--uri", "http://libosinfo.org/xmlns/libvirt/domain/1.0", "--key", "libosinfo", "--set", metadata, "--config"}
	if state == "running" || state == "paused" {
		args = append(args, "--live")
	}
	_, err := s.virsh(ctx, args...)
	if err != nil {
		return err
	}
	m, _, err := s.inspect(ctx, id)
	if err != nil {
		return err
	}
	if m.OSID != osID {
		return ErrConflict
	}
	return nil
}

func (s *Service) action(ctx context.Context, j *jobRecord, stage func(string) error) error {
	r := j.Request
	m, _, err := s.inspect(ctx, r.ID)
	if err != nil {
		return err
	}
	if m.ResourceVersion != r.ExpectedResourceVersion {
		return ErrConflict
	}
	j.Name = m.Name
	if err = stage("applying"); err != nil {
		return err
	}
	var args []string
	switch r.Action {
	case "start":
		args = []string{"start", r.ID}
	case "shutdown":
		args = []string{"shutdown", r.ID}
	case "reboot":
		args = []string{"reboot", r.ID}
	case "poweroff":
		args = []string{"destroy", r.ID}
	case "autostart":
		args = []string{"autostart", r.ID}
		if !r.Autostart {
			args = append(args, "--disable")
		}
	case "eject":
		for _, d := range m.Disks {
			if d.Device == "cdrom" && d.Path != "" {
				if _, err = s.virsh(ctx, "change-media", r.ID, d.Target, "--eject", "--config"); err != nil {
					return err
				}
				if m.State == "running" {
					if _, err = s.virsh(ctx, "change-media", r.ID, d.Target, "--eject", "--live"); err != nil {
						return err
					}
				}
			}
		}
		return nil
	case "delete":
		if m.State != "shut off" {
			if _, err = s.virsh(ctx, "destroy", r.ID); err != nil {
				return err
			}
		}
		// libvirt owns volume association and unlink semantics. Do not translate
		// guest disk XML into arbitrary filesystem removals in the Agent.
		args = []string{"undefine", r.ID, "--nvram"}
		if r.DeleteDisks {
			args = append(args, "--remove-all-storage")
		}
	default:
		return ErrInvalid
	}
	if _, err = s.virsh(ctx, args...); err != nil {
		return err
	}
	if err = stage("verifying"); err != nil {
		return err
	}
	if r.Action == "delete" {
		ids, err := s.virsh(ctx, "list", "--all", "--uuid")
		if err != nil {
			return err
		}
		for _, id := range strings.Fields(string(ids)) {
			if id == r.ID {
				return errors.New("domain is still present; check its running state")
			}
		}
		return nil
	}
	_, err = s.Machine(ctx, r.ID)
	return err
}
