package hostbackup

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

func (e *Engine) Create(ctx context.Context, directory string, modules []string, expected string) (err error) {
	i, err := e.Inventory(ctx)
	if err != nil {
		return err
	}
	if i.Revision != expected {
		return errors.New("host resources changed; scan again")
	}
	if err := validateSelection(i, modules); err != nil {
		return err
	}
	if err := backup.PrivateDir(directory); err != nil {
		return err
	}
	var expectedBytes int64
	for _, m := range i.Modules {
		if slices.Contains(modules, m.ID) {
			expectedBytes += m.Bytes
		}
	}
	if expectedBytes > backup.MaxBytes {
		return backup.ErrInvalid
	}
	if err := backup.RequireSpace(directory, expectedBytes+(128<<20)); err != nil {
		return err
	}
	selected := Payload{}
	for _, c := range i.Containers {
		if slices.Contains(modules, c.Module) {
			selected.Containers = append(selected.Containers, c)
		}
	}
	if err := orderContainers(&selected); err != nil {
		return err
	}
	// Reject an unsafe source before stopping any service. Recovery must never
	// trade host isolation for restoring availability.
	if err := e.checkDockerBindSources(selected.Containers); err != nil {
		return err
	}
	running := []Container{}
	for _, c := range selected.Containers {
		if c.Running {
			running = append(running, Container{ID: c.ID, Name: c.Name})
		}
	}
	journalPath := filepath.Join(directory, "export-journal.json")
	if err := backup.WriteJSON(journalPath, running); err != nil {
		return err
	}
	// Register recovery before stopping the first writer. Cleanup gets a fresh
	// bounded context so cancellation cannot leave source services stopped.
	defer func() {
		recovery, cancel := context.WithTimeout(context.Background(), recoveryTimeout)
		defer cancel()
		var restartErr error
		for _, c := range running {
			if startErr := e.startContainer(recovery, c.ID); startErr != nil {
				restartErr = errors.Join(restartErr, fmt.Errorf("source service %s could not restart: %w", c.Name, startErr))
			}
		}
		if restartErr == nil {
			restartErr = os.Remove(journalPath)
		}
		if restartErr != nil {
			err = &backup.Failure{Code: "recovery_required", Err: errors.Join(err, restartErr)}
		}
	}()
	for n := len(selected.Containers) - 1; n >= 0; n-- {
		c := selected.Containers[n]
		if !c.Running {
			continue
		}
		if err := e.docker(ctx, "POST", "/containers/"+c.ID+"/stop", nil, nil); err != nil {
			return err
		}
	}
	for n := range i.Roots {
		if slices.Contains(modules, i.Roots[n].Module) {
			i.Roots[n].Bytes = 0
			i.Roots[n].Entries = 0
			if err := e.measure(ctx, &i.Roots[n]); err != nil {
				return err
			}
		}
	}
	// All module payloads belong to one export and share its actual limits.
	walker := exportWalker{ctx: ctx, hook: e.exportHook}
	for _, module := range modules {
		if module == "panel" {
			continue
		}
		payload := Payload{Version: 1, Module: module, Roots: []Root{}, Containers: []Container{}, Networks: map[string]json.RawMessage{}, Volumes: map[string]Volume{}}
		for _, root := range i.Roots {
			if root.Module == module {
				payload.Roots = append(payload.Roots, root)
			}
		}
		for _, c := range i.Containers {
			if c.Module == module {
				payload.Containers = append(payload.Containers, c)
				for name := range c.Networks {
					payload.Networks[name] = i.Networks[name]
				}
				for _, m := range c.Mounts {
					if m.Type == "volume" {
						payload.Volumes[m.Name] = i.Volumes[m.Name]
					}
				}
			}
		}
		if err := e.validatePayload(payload); err != nil {
			return err
		}
		if err := e.writePayloadWithWalker(filepath.Join(directory, module+".payload"), payload, &walker); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) writePayload(ctx context.Context, path string, p Payload) (err error) {
	walker := exportWalker{ctx: ctx, hook: e.exportHook}
	return e.writePayloadWithWalker(path, p, &walker)
}

func (e *Engine) writePayloadWithWalker(path string, p Payload, walker *exportWalker) (err error) {
	key, err := e.originKey(true)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	gz.Extra = originPlaceholder()
	stream := sha256.New()
	tw := tar.NewWriter(io.MultiWriter(gz, stream))
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if len(body) > 4<<20 {
		return backup.ErrInvalid
	}
	if err := tw.WriteHeader(&tar.Header{Name: "manifest.json", Size: int64(len(body)), Mode: 0600, Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	if _, err := tw.Write(body); err != nil {
		return err
	}
	walker.writer = tw
	defer func() { walker.writer = nil }()
	for _, root := range p.Roots {
		if err := walker.writeRoot(e.host(root.Path), root); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if err := writeOriginTag(f, key, stream.Sum(nil)); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return f.Close()
}

// requireSpace is replaced in tests that cannot exhaust a real filesystem.
var requireSpace = backup.RequireSpace

// payloadFacts are derived while unpacking and never read from the archive's
// own declarations.
type payloadFacts struct {
	Local      bool
	SetIDFiles int
}

func (e *Engine) ReadPayload(ctx context.Context, path, directory string) (Payload, error) {
	p, _, err := e.readPayload(ctx, path, directory)
	return p, err
}

func (e *Engine) readPayload(ctx context.Context, path, directory string) (Payload, payloadFacts, error) {
	var facts payloadFacts
	p, err := e.unpackPayload(ctx, path, directory, &facts)
	return p, facts, err
}

func (e *Engine) unpackPayload(ctx context.Context, path, directory string, facts *payloadFacts) (Payload, error) {
	var p Payload
	key, err := e.originKey(false)
	if err != nil {
		return p, err
	}
	input, err := backup.OpenRegular(path, backup.MaxBytes)
	if err != nil {
		return p, err
	}
	defer input.Close()
	compressed := bufio.NewReader(input)
	gz, err := gzip.NewReader(compressed)
	if err != nil {
		return p, err
	}
	defer gz.Close()
	gz.Multistream(false)
	tag := originTag(gz.Header.Extra)
	stream := sha256.New()
	bounded := &io.LimitedReader{R: contextReader{ctx, io.TeeReader(gz, stream)}, N: backup.MaxBytes + (128 << 20)}
	tr := tar.NewReader(bounded)
	h, err := tr.Next()
	if err != nil || h.Name != "manifest.json" || h.Typeflag != tar.TypeReg || h.Size > 4<<20 {
		return p, backup.ErrInvalid
	}
	body, err := io.ReadAll(tr)
	if err != nil || backup.Decode(body, &p) != nil {
		return p, backup.ErrInvalid
	}
	if err := e.validatePayload(p); err != nil {
		return p, err
	}
	if err := backup.PrivateDir(directory); err != nil {
		return p, err
	}
	var estimated int64
	for _, r := range p.Roots {
		estimated += r.Bytes
	}
	if estimated > backup.MaxBytes {
		return p, backup.ErrInvalid
	}
	if err := requireSpace(directory, estimated); err != nil {
		return p, err
	}
	roots := map[string]Root{}
	for _, r := range p.Roots {
		roots[r.ID] = r
	}
	seen := map[string]bool{}
	var total int64
	directories := map[string]*tar.Header{}
	symlinks := map[string]*tar.Header{}
	for {
		if ctx.Err() != nil {
			return p, ctx.Err()
		}
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return p, err
		}
		parts := strings.Split(h.Name, "/")
		if len(parts) < 2 || parts[0] != "data" || roots[parts[1]].ID == "" || h.Name != filepath.ToSlash(filepath.Clean(h.Name)) || strings.Contains(h.Name, "\\") || seen[h.Name] {
			return p, backup.ErrInvalid
		}
		for _, part := range parts {
			if part == "" || part == "." || part == ".." {
				return p, backup.ErrInvalid
			}
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir && h.Typeflag != tar.TypeSymlink || backup.SparseEntry(h) || h.Size < 0 || h.Size > 10<<30 || h.Mode < 0 || h.Mode > 07777 || h.Uid < 0 || h.Gid < 0 || h.Uid > 1<<31-1 || h.Gid > 1<<31-1 {
			return p, backup.ErrInvalid
		}
		if h.Typeflag == tar.TypeSymlink {
			if h.Size != 0 || len(parts) < 3 || len(h.Linkname) == 0 || len(h.Linkname) > 4096 || strings.ContainsRune(h.Linkname, '\x00') || strings.Contains(h.Linkname, "\\") {
				return p, backup.ErrInvalid
			}
			root := roots[parts[1]]
			relative := strings.Join(parts[2:], "/")
			if err := validateArchiveSymlinkTarget(root.Path, relative, h.Linkname); err != nil {
				return p, backup.ErrInvalid
			}
			copy := *h
			symlinks[h.Name] = &copy
		}
		seen[h.Name] = true
		total += h.Size
		if len(seen) > 100000 || total > backup.MaxBytes {
			return p, backup.ErrInvalid
		}
		if h.Typeflag == tar.TypeSymlink {
			continue
		}
		target := filepath.Join(directory, filepath.FromSlash(h.Name))
		if h.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0700); err != nil {
				return p, err
			}
			copy := *h
			directories[target] = &copy
		} else {
			// Root sizes are declared by the archive itself. Once entries
			// outgrow them, each further file needs real free space.
			if total > estimated {
				if err := requireSpace(directory, h.Size); err != nil {
					return p, err
				}
			}
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return p, err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return p, err
			}
			_, err = io.Copy(f, tr)
			if err == nil {
				err = f.Sync()
			}
			closeErr := f.Close()
			if err != nil {
				return p, err
			}
			if closeErr != nil {
				return p, closeErr
			}
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		if h.Mode&06000 != 0 {
			facts.SetIDFiles++
		}
		if err := restoreOwnership(target, h.Uid, h.Gid); err != nil {
			return p, err
		}
		if err := os.Chmod(target, archiveMode(h.Mode)); err != nil {
			return p, err
		}
	}
	// Drain the gzip footer: a valid tar prefix is insufficient.
	tail := make([]byte, 4096)
	tailBytes := 0
	for {
		n, err := bounded.Read(tail)
		tailBytes += n
		if tailBytes > 1<<20 {
			return p, backup.ErrInvalid
		}
		for _, b := range tail[:n] {
			if b != 0 {
				return p, backup.ErrInvalid
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return p, err
		}
	}
	if bounded.N <= 0 {
		return p, backup.ErrInvalid
	}
	if _, err := compressed.Peek(1); err != io.EOF {
		return p, backup.ErrInvalid
	}
	// The whole decompressed stream has now passed through the digest.
	facts.Local = originLocal(key, tag, stream.Sum(nil))
	symlinkNames := make([]string, 0, len(symlinks))
	for name := range symlinks {
		symlinkNames = append(symlinkNames, name)
	}
	sort.Strings(symlinkNames)
	for _, name := range symlinkNames {
		h := symlinks[name]
		parts := strings.Split(name, "/")
		rootDir := filepath.Join(directory, "data", parts[1])
		target := filepath.Join(directory, filepath.FromSlash(name))
		if err := ensureArchiveParents(rootDir, target); err != nil {
			return p, backup.ErrInvalid
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return p, err
		}
		if err := os.Symlink(filepath.FromSlash(h.Linkname), target); err != nil {
			return p, err
		}
	}
	for _, name := range symlinkNames {
		h := symlinks[name]
		parts := strings.Split(name, "/")
		rootDir := filepath.Join(directory, "data", parts[1])
		target := filepath.Join(directory, filepath.FromSlash(name))
		if err := validateStagedSymlink(rootDir, target); err != nil {
			return p, backup.ErrInvalid
		}
		if err := restoreSymlinkOwnership(target, h.Uid, h.Gid); err != nil {
			return p, err
		}
	}
	for path, h := range directories {
		if err := restoreOwnership(path, h.Uid, h.Gid); err != nil {
			return p, err
		}
		if err := os.Chmod(path, archiveMode(h.Mode)); err != nil {
			return p, err
		}
	}
	for _, r := range p.Roots {
		info, err := os.Lstat(filepath.Join(directory, "data", r.ID))
		if err != nil || info.IsDir() != r.Directory {
			return p, backup.ErrInvalid
		}
	}
	return p, nil
}

func validateArchiveSymlinkTarget(rootPath, relativePath, target string) error {
	rootPath = path.Clean(rootPath)
	relativePath = path.Clean(filepath.ToSlash(relativePath))
	if !path.IsAbs(rootPath) || rootPath == "/" || relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, "../") || target == "" || strings.ContainsRune(target, '\x00') || strings.Contains(target, "\\") || path.IsAbs(target) {
		return backup.ErrInvalid
	}
	linkPath := path.Join(rootPath, relativePath)
	targetPath := path.Clean(path.Join(path.Dir(linkPath), target))
	if !within(rootPath, targetPath) {
		return backup.ErrInvalid
	}
	return nil
}

func validateStagedSymlink(root, linkPath string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	resolvedTarget, err := filepath.EvalSymlinks(linkPath)
	if err != nil || !within(filepath.ToSlash(resolvedRoot), filepath.ToSlash(resolvedTarget)) {
		return backup.ErrInvalid
	}
	return nil
}

func ensureArchiveParents(root, target string) error {
	root = filepath.Clean(root)
	parent := filepath.Dir(target)
	relative, err := filepath.Rel(root, parent)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return backup.ErrInvalid
	}
	current := root
	if relative == "." {
		return nil
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return backup.ErrInvalid
		}
	}
	return nil
}

func (e *Engine) validatePayload(p Payload) error {
	if p.Version != 1 || !slices.Contains([]string{"apps", "web", "docker"}, p.Module) || len(p.Roots) > 4096 || len(p.Containers) > 512 || len(p.Networks) > 512 || len(p.Volumes) > 4096 {
		return backup.ErrInvalid
	}
	ids := map[string]bool{}
	paths := []string{}
	for _, r := range p.Roots {
		if !backup.ValidID(r.ID) || ids[r.ID] || r.Module != p.Module || !canonicalMountPath(r.Path) || e.excluded(r.Path) {
			return backup.ErrInvalid
		}
		if !rootMatchesModule(p, r) {
			return backup.ErrInvalid
		}
		if r.Bytes < 0 || r.Bytes > backup.MaxBytes || r.Entries < 0 || r.Entries > 100000 {
			return backup.ErrInvalid
		}
		for _, old := range paths {
			if within(old, r.Path) || within(r.Path, old) {
				return backup.ErrInvalid
			}
		}
		ids[r.ID] = true
		paths = append(paths, r.Path)
	}
	containerNames := map[string]bool{}
	volumeRefs, networkRefs := map[string]bool{}, map[string]bool{}
	edges := 0
	for _, c := range p.Containers {
		if containerNames[c.Name] {
			return backup.ErrInvalid
		}
		containerNames[c.Name] = true
		edges += len(c.Mounts) + len(c.Networks)
		if edges > 8192 {
			return backup.ErrInvalid
		}
		for name := range c.Networks {
			if _, ok := p.Networks[name]; !ok {
				return backup.ErrInvalid
			}
			networkRefs[name] = true
		}
		if !imageIDPattern.MatchString(c.ImageID) {
			return backup.ErrInvalid
		}
		// Restore sends Config at the top of the create body and adds its own
		// HostConfig, image and labels; none may be shadowed by the archive.
		if !canonicalKeys(c.Config, "Image", "Labels", "Env", "Entrypoint", "Cmd", "Hostname", "Domainname", "ExposedPorts", "Healthcheck", "Volumes", "HostConfig", "NetworkingConfig") || c.Config["HostConfig"] != nil || c.Config["NetworkingConfig"] != nil {
			return backup.ErrInvalid
		}
		if !canonicalKeys(c.HostConfig, "Binds", "Mounts", "VolumesFrom", "NetworkMode", "PidMode", "IpcMode", "Links", "AutoRemove") {
			return backup.ErrInvalid
		}
		// Image-declared/configured volumes must resolve to inspected mounts;
		// otherwise Docker could create an anonymous resource outside this plan.
		var configuredVolumes map[string]json.RawMessage
		if raw, ok := c.Config["Volumes"]; ok && json.Unmarshal(raw, &configuredVolumes) != nil {
			return backup.ErrInvalid
		}
		for target := range configuredVolumes {
			covered := false
			for _, mount := range c.Mounts {
				if mount.Destination == target {
					covered = true
				}
			}
			if !covered {
				return backup.ErrInvalid
			}
		}
		var networkMode string
		if raw, ok := c.HostConfig["NetworkMode"]; ok && json.Unmarshal(raw, &networkMode) != nil {
			return backup.ErrInvalid
		}
		if networkMode != "" && networkMode != "default" && networkMode != "host" && networkMode != "none" && networkMode != "bridge" && !strings.HasPrefix(networkMode, "container:") {
			if !networkRefs[networkMode] {
				return backup.ErrInvalid
			}
			if _, ok := c.Networks[networkMode]; !ok {
				return backup.ErrInvalid
			}
		}
		if _, err := nativeMountConfig(c); err != nil {
			return err
		}
		if protected, _ := runtimeProtection(c); protected {
			return errors.New("backup cannot replace Panel or Agent runtime")
		}
		if c.Module != p.Module || !validContainerName(c.Name) || len(c.Mounts) > 512 || len(c.Networks) > 64 {
			return backup.ErrInvalid
		}
		var image string
		if json.Unmarshal(c.Config["Image"], &image) != nil || image == "" || len(image) > 512 {
			return backup.ErrInvalid
		}
		for _, mount := range c.Mounts {
			if !canonicalMountPath(mount.Destination) {
				return backup.ErrInvalid
			}
			if mount.Type == "tmpfs" {
				continue
			}
			if mount.Type != "bind" && mount.Type != "volume" || !canonicalMountPath(mount.Source) {
				return backup.ErrInvalid
			}
			if mount.Type == "volume" {
				volumeRefs[mount.Name] = true
				v, ok := p.Volumes[mount.Name]
				if !ok || v.Name != mount.Name || v.Mountpoint != mount.Source {
					return backup.ErrInvalid
				}
			}
		}
	}
	for name, v := range p.Volumes {
		if !volumeRefs[name] || !validContainerName(name) || v.Name != name || v.Driver != "local" || len(v.Options) != 0 || !canonicalMountPath(v.Mountpoint) || e.excluded(v.Mountpoint) {
			return backup.ErrInvalid
		}
	}
	for name, raw := range p.Networks {
		if !networkRefs[name] || !validContainerName(name) {
			return backup.ErrInvalid
		}
		var config map[string]json.RawMessage
		if json.Unmarshal(raw, &config) != nil || !canonicalKeys(config, "Name", "Driver", "Labels", "Options", "IPAM") {
			return backup.ErrInvalid
		}
		var declared string
		if value, ok := config["Name"]; ok && (json.Unmarshal(value, &declared) != nil || declared != name) {
			return backup.ErrInvalid
		}
	}
	return nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
func archiveMode(mode int64) os.FileMode {
	m := os.FileMode(mode & 0777)
	if mode&04000 != 0 {
		m |= os.ModeSetuid
	}
	if mode&02000 != 0 {
		m |= os.ModeSetgid
	}
	if mode&01000 != 0 {
		m |= os.ModeSticky
	}
	return m
}
