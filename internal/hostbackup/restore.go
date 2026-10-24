package hostbackup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

const recoveryTimeout = 4 * time.Minute

var containerName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func validContainerName(name string) bool { return containerName.MatchString(name) }

type restoreRoot struct {
	Target            string `json:"target"`
	Previous          string `json:"previous"`
	Next              string `json:"next"`
	HadPrevious       bool   `json:"hadPrevious"`
	Applied           bool   `json:"applied"`
	ParentIdentity    string `json:"parentIdentity,omitempty"`
	Workspace         string `json:"workspace,omitempty"`
	WorkspaceIdentity string `json:"workspaceIdentity,omitempty"`
	TargetIdentity    string `json:"targetIdentity,omitempty"`
	NextIdentity      string `json:"nextIdentity,omitempty"`
	parent            *os.Root
	workspace         *os.Root
	workspaceParent   *os.Root
}
type restoreContainer struct {
	Name       string `json:"name"`
	Previous   string `json:"previous"`
	OldID      string `json:"oldId"`
	OldRunning bool   `json:"oldRunning"`
	NewID      string `json:"newId"`
}
type restoreVolume struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

type restoreJournal struct {
	FilesystemVersion int                `json:"filesystemVersion,omitempty"`
	ID                string             `json:"id"`
	Phase             string             `json:"phase"`
	Roots             []restoreRoot      `json:"roots"`
	Containers        []restoreContainer `json:"containers"`
	Networks          []string           `json:"networks"`
	Volumes           []restoreVolume    `json:"volumes,omitempty"`
}

func (e *Engine) Restore(ctx context.Context, id, directory string, modules []string, foreignConfirmed bool) (err error) {
	if !backup.ValidID(id) {
		return backup.ErrInvalid
	}
	journalPath := filepath.Join(directory, "restore-journal.json")
	if _, err := os.Lstat(journalPath); err == nil {
		return errors.New("restore already attempted; inspect retained recovery data")
	}
	all := Payload{Version: 1, Networks: map[string]json.RawMessage{}, Volumes: map[string]Volume{}}
	paths := map[string]bool{}
	names := map[string]bool{}
	for _, module := range modules {
		if module == "panel" {
			continue
		}
		p, facts, err := e.readPayload(ctx, filepath.Join(directory, module+".payload"), filepath.Join(directory, "unpack-"+module))
		if err != nil {
			return err
		}
		if p.Module != module {
			return backup.ErrInvalid
		}
		// Re-derived from the payload itself, before any image, volume or
		// host change, so a stale or missing review cannot skip it.
		if !facts.Local && !foreignConfirmed {
			var cleanupErr error
			for _, unpacked := range modules {
				cleanupErr = errors.Join(cleanupErr, os.RemoveAll(filepath.Join(directory, "unpack-"+unpacked)))
			}
			return &backup.Failure{Code: "foreign_confirmation_required", Err: errors.Join(ErrForeignUnconfirmed, cleanupErr)}
		}
		for _, r := range p.Roots {
			for path := range paths {
				if within(path, r.Path) || within(r.Path, path) {
					return errors.New("selected backups contain overlapping resource roots")
				}
			}
			paths[r.Path] = true
			all.Roots = append(all.Roots, r)
		}
		for _, c := range p.Containers {
			if names[c.Name] {
				return backup.ErrInvalid
			}
			names[c.Name] = true
			all.Containers = append(all.Containers, c)
		}
		for name, value := range p.Networks {
			if old, ok := all.Networks[name]; ok && string(networkConfiguration(old)) != string(networkConfiguration(value)) {
				return backup.ErrInvalid
			}
			all.Networks[name] = value
		}
		for name, value := range p.Volumes {
			if old, ok := all.Volumes[name]; ok {
				a, _ := json.Marshal(volumeCreateBody(old))
				b, _ := json.Marshal(volumeCreateBody(value))
				if string(a) != string(b) || old.Mountpoint != value.Mountpoint {
					return backup.ErrInvalid
				}
			}
			all.Volumes[name] = value
		}
	}
	if len(all.Roots) > 4096 || len(all.Containers) > 512 || len(all.Networks) > 512 || len(all.Volumes) > 4096 {
		return backup.ErrInvalid
	}
	// Every data mount must be covered by the selected modules. An imported
	// manifest cannot silently expand a selection to another application's data.
	for _, c := range all.Containers {
		for _, mount := range c.Mounts {
			if mount.Type == "tmpfs" || e.excluded(mount.Source) {
				continue
			}
			covered := false
			for path := range paths {
				if within(path, mount.Source) {
					covered = true
					break
				}
			}
			if !covered {
				return fmt.Errorf("container %s requires an unselected data module", c.Name)
			}
		}
	}
	// A shared volume may be referenced by several module payloads, but its
	// data root belongs to exactly one selected module. Resolve this once on
	// the combined plan before any Docker preparation can create resources.
	for _, volume := range all.Volumes {
		covered := false
		for _, root := range all.Roots {
			if root.Path == volume.Mountpoint && root.Directory {
				covered = true
			}
		}
		if !covered {
			return errors.New("volume requires an unselected data module")
		}
	}
	if err := orderContainers(&all); err != nil {
		return err
	}
	if err := e.checkRestoreDockerBinds(all, directory); err != nil {
		return err
	}
	j := restoreJournal{FilesystemVersion: 2, ID: id, Phase: "preparing", Roots: []restoreRoot{}, Containers: []restoreContainer{}, Networks: []string{}}
	defer j.closeParents()
	if err := backup.WriteJSON(journalPath, j); err != nil {
		return err
	}
	committed := false
	defer func() {
		if err == nil {
			return
		}
		if committed {
			j.Phase = "cleanup_pending"
			err = &backup.Failure{Code: "cleanup_pending", Err: errors.Join(err, backup.WriteJSON(journalPath, j))}
			return
		}
		recovery, cancel := context.WithTimeout(context.Background(), recoveryTimeout)
		defer cancel()
		rollbackErr := e.rollback(recovery, &j)
		j.Phase = "rolled_back"
		if rollbackErr != nil {
			j.Phase = "needs_attention"
		}
		saveErr := backup.WriteJSON(journalPath, j)
		if rollbackErr == nil && saveErr == nil {
			j.releaseWorkspaces()
		}
		code := "rolled_back"
		if rollbackErr != nil || saveErr != nil {
			code = "recovery_required"
		}
		err = &backup.Failure{Code: code, Err: errors.Join(err, rollbackErr, saveErr)}
	}()
	// Pulled images are shared content-addressed cache. Rollback retains them;
	// volumes and runtime resources are transaction-owned and recovered below.
	if err := e.prepareImages(ctx, all.Containers); err != nil {
		return err
	}
	if err := e.prepareVolumes(ctx, &all, &j, journalPath); err != nil {
		return err
	}
	// Check destination writers too: restoring a directory cannot overwrite a
	// running, unselected service that happens to mount the same data.
	current, err := e.Inventory(ctx)
	if err != nil {
		return err
	}
	for _, root := range all.Roots {
		if protectionRootOverlap(current.ProtectedRoots, root.Path) {
			return errors.New("restore data overlaps protected Panel data")
		}
	}
	// Re-verify payload roots against the destination inventory: an imported
	// manifest may only replace data the destination actually owns. Local
	// volume roots are resolved by prepareVolumes against this Docker daemon,
	// including newly created volumes without a destination container yet.
	for _, root := range all.Roots {
		covered := declaredLocalVolumeRoot(all, root)
		for _, existing := range current.Roots {
			if existing.Path == root.Path {
				covered = true
				break
			}
		}
		if !covered {
			return fmt.Errorf("restore root %s is not part of the destination data", root.Path)
		}
	}
	for _, c := range current.ProtectedContainers {
		if names[c.Name] {
			return errors.New("restore name conflicts with protected Panel runtime")
		}
	}
	for _, m := range current.Modules {
		if m.Issue == "user_namespace_requires_adapter" {
			return errors.New("destination Docker user namespace mapping is unsupported")
		}
	}
	for _, existing := range current.Containers {
		if names[existing.Name] {
			if err := e.checkDockerBindSources([]Container{existing}); err != nil {
				return err
			}
			var autoRemove bool
			_ = json.Unmarshal(existing.HostConfig["AutoRemove"], &autoRemove)
			if autoRemove && existing.Running {
				return errors.New("destination auto-remove container must be stopped before restore")
			}
			continue
		}
		for _, mount := range existing.Mounts {
			for _, root := range all.Roots {
				if within(root.Path, mount.Source) || within(mount.Source, root.Path) {
					return errors.New("another destination container uses selected data; include its module")
				}
			}
		}
	}
	var total int64
	for n, r := range all.Roots {
		var size int64
		err := filepath.WalkDir(filepath.Join(directory, "unpack-"+r.Module, "data", r.ID), func(_ string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				size += info.Size()
			}
			return nil
		})
		if err != nil {
			return err
		}
		all.Roots[n].Bytes = size
		total += size
	}
	if total > backup.MaxBytes {
		return backup.ErrInvalid
	}
	for _, r := range all.Roots {
		parent := filepath.Dir(e.host(r.Path))
		for {
			if _, err := os.Stat(parent); err == nil {
				break
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			next := filepath.Dir(parent)
			if next == parent {
				return backup.ErrInvalid
			}
			parent = next
		}
		if err := backup.RequireSpace(parent, total); err != nil {
			return err
		}
	}
	plannedRoots := restoreJournal{}
	defer plannedRoots.closeParents()
	for _, r := range all.Roots {
		target := e.host(r.Path)
		if err := noNestedMounts(target); err != nil {
			return err
		}
		if err := noLinkParents(target); err != nil {
			return err
		}
		entry := restoreRoot{Target: target, Previous: target + ".kpanel-restore-" + id, Next: target + ".kpanel-next-" + id}
		if err := entry.bindParent(false); err != nil {
			return err
		}
		plannedRoots.Roots = append(plannedRoots.Roots, entry)
		parent := entry.parent
		previous := target + ".kpanel-restore-" + id
		next := target + ".kpanel-next-" + id
		if _, err := parent.Lstat(filepath.Base(previous)); !errors.Is(err, os.ErrNotExist) {
			return errors.New("recovery path already exists")
		}
		if _, err := parent.Lstat(filepath.Base(next)); !errors.Is(err, os.ErrNotExist) {
			return errors.New("staging path already exists")
		}
		_, statErr := parent.Lstat(filepath.Base(target))
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		plannedRoots.Roots[len(plannedRoots.Roots)-1].HadPrevious = statErr == nil
		if statErr == nil {
			identity, err := restoreObjectIdentity(parent, filepath.Base(target))
			if err != nil {
				return err
			}
			plannedRoots.Roots[len(plannedRoots.Roots)-1].TargetIdentity = identity
		}
	}
	for _, c := range all.Containers {
		var listed []struct {
			ID    string `json:"Id"`
			Names []string
			State string
		}
		if err := e.docker(ctx, "GET", "/containers/json?all=1", nil, &listed); err != nil {
			return err
		}
		r := restoreContainer{Name: c.Name, Previous: "kpanel-previous-" + id + "-" + c.Name}
		for _, existing := range listed {
			for _, name := range existing.Names {
				if name == "/"+c.Name {
					var actual Container
					if err := e.docker(ctx, "GET", "/containers/"+url.PathEscape(existing.ID)+"/json", nil, &actual); err != nil {
						return err
					}
					actual.Name = strings.TrimPrefix(actual.Name, "/")
					if protected, _ := runtimeProtection(actual); protected {
						return errors.New("destination container is protected Panel runtime")
					}
					r.OldID = existing.ID
					r.OldRunning = existing.State == "running"
				}
			}
		}
		plannedRoots.Containers = append(plannedRoots.Containers, r)
	}
	j.Roots = plannedRoots.Roots
	j.Containers = plannedRoots.Containers
	plannedRoots.Roots = nil
	j.Phase = "applying"
	if err := backup.WriteJSON(journalPath, j); err != nil {
		return err
	}
	// Establish every protected same-filesystem workspace before stopping the
	// destination's writers. Unsupported filesystems fail during preparation.
	for n := range j.Roots {
		if err := j.Roots[n].createWorkspace(); err != nil {
			return err
		}
		if err := backup.WriteJSON(journalPath, j); err != nil {
			return err
		}
	}
	for index := len(j.Containers) - 1; index >= 0; index-- {
		r := &j.Containers[index]
		if r.OldID == "" {
			continue
		}
		if r.OldRunning {
			if err := e.docker(ctx, "POST", "/containers/"+r.OldID+"/stop", nil, nil); err != nil {
				return err
			}
		}
		if err := e.docker(ctx, "POST", "/containers/"+r.OldID+"/rename?name="+url.QueryEscape(r.Previous), nil, nil); err != nil {
			return err
		}
	}
	for index, r := range all.Roots {
		entry := &j.Roots[index]
		parent, target := entry.parent, filepath.Base(entry.Target)
		staged, err := copyTreeIn(ctx, filepath.Join(directory, "unpack-"+r.Module, "data", r.ID), entry.workspace, "next", func(_ os.FileInfo) error {
			identity, err := restoreObjectIdentity(entry.workspace, "next")
			if err != nil {
				return err
			}
			entry.NextIdentity = identity
			return backup.WriteJSON(journalPath, j)
		})
		if err != nil {
			return err
		}
		if err := j.checkParentPaths(); err != nil {
			return err
		}
		if entry.HadPrevious {
			if err := entry.capture(target, "previous", entry.TargetIdentity); err != nil {
				return err
			}
		}
		entry.Applied = true
		if err := backup.WriteJSON(journalPath, j); err != nil {
			return err
		}
		if err := stagedAtIn(entry.workspace, "next", staged); err != nil {
			return err
		}
		if err := renameRestoreObject(entry.workspace, "next", parent, target); err != nil {
			return err
		}
		if err := stagedAtIn(parent, target, staged); err != nil {
			return err
		}
		entry.Applied = true
		if err := syncRestoreParent(parent); err != nil {
			return err
		}
		if err := backup.WriteJSON(journalPath, j); err != nil {
			return err
		}
	}
	// Existing networks are reused. Missing networks are recreated from native
	// configuration only; runtime IDs/containers do not enter create requests.
	for name, raw := range all.Networks {
		if name == "host" || name == "none" || name == "bridge" {
			continue
		}
		var networks []struct{ Name string }
		if err := e.docker(ctx, "GET", "/networks", nil, &networks); err != nil {
			return err
		}
		exists := false
		for _, n := range networks {
			if n.Name == name {
				exists = true
			}
		}
		if exists {
			var current json.RawMessage
			if err := e.docker(ctx, "GET", "/networks/"+url.PathEscape(name), nil, &current); err != nil {
				return err
			}
			if string(networkConfiguration(current)) != string(networkConfiguration(raw)) {
				return errors.New("destination network configuration differs from backup")
			}
			continue
		}
		input, err := networkCreateBody(name, raw)
		if err != nil {
			return err
		}
		var labels map[string]string
		_ = json.Unmarshal(input["Labels"], &labels)
		if labels == nil {
			labels = map[string]string{}
		}
		labels["io.kpanel.restore.job"] = id
		input["Labels"], _ = json.Marshal(labels)
		j.Networks = append(j.Networks, name)
		if err := backup.WriteJSON(journalPath, j); err != nil {
			return err
		}
		if err := e.docker(ctx, "POST", "/networks/create", input, nil); err != nil {
			return err
		}
		if err := backup.WriteJSON(journalPath, j); err != nil {
			return err
		}
	}
	for index, c := range all.Containers {
		if err := j.checkParentPaths(); err != nil {
			return err
		}
		if err := e.checkDockerBindSources([]Container{c}); err != nil {
			return err
		}
		input := map[string]json.RawMessage{}
		for key, value := range c.Config {
			input[key] = value
		}
		hostConfig, err := nativeMountConfig(c)
		if err != nil {
			return err
		}
		var networkMode string
		_ = json.Unmarshal(hostConfig["NetworkMode"], &networkMode)
		if strings.HasPrefix(networkMode, "container:") {
			delete(input, "Hostname")
			delete(input, "Domainname")
			delete(input, "ExposedPorts")
		}
		input["HostConfig"], _ = json.Marshal(hostConfig)
		input["Image"], _ = json.Marshal(c.ImageID)
		var labels map[string]string
		_ = json.Unmarshal(c.Config["Labels"], &labels)
		if labels == nil {
			labels = map[string]string{}
		}
		labels["io.kpanel.restore.job"] = id
		input["Labels"], _ = json.Marshal(labels)
		endpoints := map[string]any{}
		for name, raw := range c.Networks {
			var old map[string]json.RawMessage
			if json.Unmarshal(raw, &old) != nil {
				return backup.ErrInvalid
			}
			endpoint := map[string]json.RawMessage{}
			for _, key := range []string{"Aliases", "Links", "IPAMConfig", "DriverOpts", "GwPriority"} {
				if value, ok := old[key]; ok {
					endpoint[key] = value
				}
			}
			endpoints[name] = endpoint
		}
		input["NetworkingConfig"], _ = json.Marshal(map[string]any{"EndpointsConfig": endpoints})
		var created struct {
			ID string `json:"Id"`
		}
		if err := e.docker(ctx, "POST", "/containers/create?name="+url.QueryEscape(c.Name), input, &created); err != nil {
			return fmt.Errorf("cannot recreate %s; ensure the backup image is available: %w", c.Name, err)
		}
		if created.ID == "" {
			return backup.ErrInvalid
		}
		j.Containers[index].NewID = created.ID
		if err := backup.WriteJSON(journalPath, j); err != nil {
			return err
		}
	}
	for n, c := range all.Containers {
		if c.Running {
			if err := j.checkParentPaths(); err != nil {
				return err
			}
			if err := e.startContainer(ctx, j.Containers[n].NewID); err != nil {
				return err
			}
		}
	}
	for n, c := range all.Containers {
		if c.Running {
			if err := e.healthy(ctx, j.Containers[n].NewID, c); err != nil {
				return err
			}
		}
	}
	j.Phase = "cleanup_pending"
	if err := backup.WriteJSON(journalPath, j); err != nil {
		return err
	}
	committed = true
	if err := e.cleanupRecovery(ctx, &j); err != nil {
		return err
	}
	j.Phase = "completed"
	if err := backup.WriteJSON(journalPath, j); err != nil {
		return err
	}
	j.releaseWorkspaces()
	return nil
}

// networkCreateBody is the native configuration Restore sends to create a
// missing network; the import review describes the same body.
func networkCreateBody(name string, raw json.RawMessage) (map[string]json.RawMessage, error) {
	var old map[string]json.RawMessage
	if json.Unmarshal(raw, &old) != nil {
		return nil, backup.ErrInvalid
	}
	input := map[string]json.RawMessage{}
	input["Name"], _ = json.Marshal(name)
	for _, key := range []string{"Driver", "Internal", "Attachable", "Ingress", "EnableIPv4", "EnableIPv6", "IPAM", "Options", "Labels"} {
		if value, ok := old[key]; ok {
			input[key] = value
		}
	}
	return input, nil
}

func (e *Engine) healthy(ctx context.Context, id string, c Container) error {
	var health struct {
		StartPeriod, Interval, Timeout time.Duration
		Retries                        int
	}
	_ = json.Unmarshal(c.Config["Healthcheck"], &health)
	timeout := 60 * time.Second
	if health.Retries > 0 && health.Retries < 1000 && health.Interval > 0 && health.Interval < time.Hour {
		calculated := health.StartPeriod + time.Duration(health.Retries)*(health.Interval+health.Timeout) + 30*time.Second
		if calculated > timeout {
			timeout = calculated
		}
	}
	if timeout > 10*time.Minute {
		timeout = 10 * time.Minute
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	stable := 0
	for {
		var state struct {
			State struct {
				Running    bool
				Restarting bool
				Health     *struct{ Status string }
			}
		}
		if err := e.docker(ctx, "GET", "/containers/"+id+"/json", nil, &state); err != nil {
			return err
		}
		if !state.State.Running || state.State.Restarting {
			return errors.New("restored service did not stay running")
		}
		stable++
		if state.State.Health == nil && stable >= 3 || state.State.Health != nil && state.State.Health.Status == "healthy" {
			return nil
		}
		if state.State.Health != nil && state.State.Health.Status == "unhealthy" {
			return errors.New("restored service is unhealthy")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("restored service health check timed out")
		case <-tick.C:
		}
	}
}

func (e *Engine) rollback(ctx context.Context, j *restoreJournal) error {
	var result error
	// Resolve a create that completed before its returned ID reached the journal.
	for n := range j.Containers {
		r := &j.Containers[n]
		if r.NewID != "" {
			continue
		}
		var current struct {
			ID     string `json:"Id"`
			Config struct{ Labels map[string]string }
		}
		if e.docker(ctx, "GET", "/containers/"+url.PathEscape(r.Name)+"/json", nil, &current) == nil && current.Config.Labels["io.kpanel.restore.job"] == j.ID {
			r.NewID = current.ID
		}
	}
	for n := len(j.Containers) - 1; n >= 0; n-- {
		r := j.Containers[n]
		if r.NewID != "" {
			result = errors.Join(result, e.docker(ctx, "DELETE", "/containers/"+r.NewID+"?force=1", nil, nil))
		}
	}
	if result != nil {
		return result
	} // Never touch data while a new writer survives.
	for n := len(j.Roots) - 1; n >= 0; n-- {
		result = errors.Join(result, j.Roots[n].rollbackData())
	}
	if result != nil {
		return result
	} // Do not start old writers until every data root is restored.
	for _, r := range j.Containers {
		if r.OldID != "" {
			var state struct{ Name string }
			if err := e.docker(ctx, "GET", "/containers/"+r.OldID+"/json", nil, &state); err != nil {
				result = errors.Join(result, err)
				continue
			}
			if state.Name == "/"+r.Previous {
				if err := e.docker(ctx, "POST", "/containers/"+r.OldID+"/rename?name="+url.QueryEscape(r.Name), nil, nil); err != nil {
					result = errors.Join(result, err)
					continue
				}
			} else if state.Name != "/"+r.Name {
				result = errors.Join(result, backup.ErrInvalid)
				continue
			}
			if r.OldRunning {
				if err := j.checkParentPaths(); err != nil {
					return errors.Join(result, err)
				}
				result = errors.Join(result, e.startContainer(ctx, r.OldID))
			}
		}
	}
	for _, name := range j.Networks {
		var network struct{ Labels map[string]string }
		if err := e.docker(ctx, "GET", "/networks/"+url.PathEscape(name), nil, &network); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				result = errors.Join(result, err)
			}
			continue
		}
		if network.Labels["io.kpanel.restore.job"] == j.ID {
			result = errors.Join(result, e.docker(ctx, "DELETE", "/networks/"+url.PathEscape(name), nil, nil))
		}
	}
	for _, volume := range j.Volumes {
		var actual Volume
		if err := e.docker(ctx, "GET", "/volumes/"+url.PathEscape(volume.Name), nil, &actual); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				result = errors.Join(result, err)
			}
			continue
		}
		if actual.Name != volume.Name || actual.Labels["io.kpanel.restore.job"] != j.ID || actual.Labels["io.kpanel.restore.volume"] != volume.Token {
			// Unrelated/replaced volumes are never deleted. Retain the journal to
			// report the unresolved ownership instead of claiming full recovery.
			result = errors.Join(result, errors.New("restore volume ownership changed"))
			continue
		}
		result = errors.Join(result, e.docker(ctx, "DELETE", "/volumes/"+url.PathEscape(volume.Name), nil, nil))
	}
	return result
}

func removeDataTree(path string) error {
	if err := backup.NoLinkParents(path); err != nil {
		return err
	}
	if err := noNestedMounts(path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func noLinkParents(path string) error {
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return backup.ErrInvalid
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if current == filepath.Dir(current) {
			break
		}
	}
	return nil
}

// copyTreeIn stages source at target and returns the staged object's identity.
// Directory stages are created only in the private transaction workspace.
// Content, ownership and modes go through an os.Root or an open descriptor;
// the creation callback durably records identity before any content is copied.
func copyTreeIn(ctx context.Context, source string, parent *os.Root, target string, created ...func(os.FileInfo) error) (os.FileInfo, error) {
	info, err := os.Lstat(source)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return nil, backup.ErrInvalid
		}
		// O_EXCL with O_CREATE never follows a symbolic link at target.
		out, err := parent.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		if len(created) != 0 {
			info, statErr := out.Stat()
			if statErr == nil {
				statErr = created[0](info)
			}
			if statErr != nil {
				out.Close()
				return nil, statErr
			}
		}
		staged, err := writeStagedFile(ctx, source, out, info)
		if err = errors.Join(err, out.Close()); err != nil {
			return nil, err
		}
		return staged, nil
	}
	// mkdir does not return a descriptor. Only bootstrap below a pinned,
	// non-writable transaction directory, so a sibling owned by root cannot
	// be substituted between mkdir and OpenRoot.
	if err := requireRestorePrivateParent(parent); err != nil {
		return nil, err
	}
	if err := parent.Mkdir(target, 0700); err != nil {
		return nil, err
	}
	stage, err := parent.OpenRoot(target)
	if err != nil {
		return nil, err
	}
	defer stage.Close()
	staged, err := stage.Stat(".")
	if err != nil {
		return nil, err
	}
	if stagedAtIn(parent, target, staged) != nil || !ownedByProcess(staged) {
		return nil, backup.ErrInvalid
	}
	if len(created) != 0 {
		if err := created[0](staged); err != nil {
			return nil, err
		}
	}
	directories := map[string]os.FileInfo{}
	err = filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			directories[relative] = info
			if relative == "." {
				return nil
			}
			return stage.Mkdir(relative, 0700)
		}
		if err := stagedParentsAreDirectories(stage, relative); err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil || filepath.IsAbs(linkTarget) || strings.Contains(linkTarget, "\\") {
				return backup.ErrInvalid
			}
			if validateArchiveSymlinkTarget("/backup-root", filepath.ToSlash(relative), filepath.ToSlash(linkTarget)) != nil || validateStagedSymlink(source, path) != nil {
				return backup.ErrInvalid
			}
			if err := stage.Symlink(filepath.FromSlash(linkTarget), relative); err != nil {
				return err
			}
			return copySymlinkOwnershipIn(stage, relative, info)
		}
		if !info.Mode().IsRegular() {
			return backup.ErrInvalid
		}
		out, err := stage.OpenFile(relative, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = writeStagedFile(ctx, path, out, info)
		return errors.Join(err, out.Close())
	})
	if err != nil {
		return nil, err
	}
	names := []string{}
	for name := range directories {
		if name != "." {
			names = append(names, name)
		}
	}
	// Children before parents; the staged root itself goes last.
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, name := range append(names, ".") {
		info := directories[name]
		if err := copyOwnershipIn(stage, name, info); err != nil {
			return nil, err
		}
		if err := stage.Chmod(name, info.Mode()); err != nil {
			return nil, err
		}
	}
	return staged, nil
}

// writeStagedFile copies source into out and sets ownership before the mode,
// since chown clears set-id bits. Both act on the descriptor, not a path.
func writeStagedFile(ctx context.Context, source string, out *os.File, info os.FileInfo) (os.FileInfo, error) {
	in, err := backup.OpenRegular(source, 10<<30)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	if _, err := io.Copy(out, contextReader{ctx, in}); err != nil {
		return nil, err
	}
	if err := out.Sync(); err != nil {
		return nil, err
	}
	if err := copyFileOwnership(out, info); err != nil {
		return nil, err
	}
	if err := out.Chmod(info.Mode()); err != nil {
		return nil, err
	}
	return out.Stat()
}

func stagedParentsAreDirectories(stage *os.Root, name string) error {
	parent := filepath.Dir(name)
	if parent == "." {
		return nil
	}
	current := ""
	for _, part := range strings.Split(parent, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := stage.Lstat(current)
		if err != nil || !info.IsDir() {
			return backup.ErrInvalid
		}
	}
	return nil
}

func stagedAtIn(parent *os.Root, name string, staged os.FileInfo) error {
	current, err := parent.Lstat(name)
	if err != nil {
		return err
	}
	if current.Mode()&os.ModeSymlink != 0 || !os.SameFile(current, staged) {
		return backup.ErrInvalid
	}
	return nil
}

// publishStagedIn checks the staged identity before and after a rename in the
// pinned parent. None of the three operations re-resolves an ancestor pathname.
func publishStagedIn(parent *os.Root, next, target string, staged os.FileInfo) error {
	if err := stagedAtIn(parent, next, staged); err != nil {
		return err
	}
	if err := renameRestoreObject(parent, next, parent, target); err != nil {
		return err
	}
	return stagedAtIn(parent, target, staged)
}

func containerDependencies(c Container) []string {
	refs := []string{}
	for _, key := range []string{"NetworkMode", "PidMode", "IpcMode"} {
		var value string
		_ = json.Unmarshal(c.HostConfig[key], &value)
		if ref, ok := strings.CutPrefix(value, "container:"); ok {
			refs = append(refs, ref)
		}
	}
	var links []string
	_ = json.Unmarshal(c.HostConfig["Links"], &links)
	for _, link := range links {
		ref, _, _ := strings.Cut(link, ":")
		refs = append(refs, strings.TrimPrefix(ref, "/"))
	}
	return refs
}
func orderContainers(p *Payload) error {
	pending := append([]Container(nil), p.Containers...)
	ordered := []Container{}
	done := map[string]bool{}
	known := map[string]bool{}
	for _, c := range pending {
		known[c.Name] = true
	}
	for len(pending) > 0 {
		progress := false
		for n := 0; n < len(pending); {
			c := pending[n]
			ready := true
			for _, ref := range containerDependencies(c) {
				if !known[ref] {
					return errors.New("container dependency must be included in the backup")
				}
				if !done[ref] {
					ready = false
				}
			}
			if !ready {
				n++
				continue
			}
			ordered = append(ordered, c)
			done[c.Name] = true
			pending = append(pending[:n], pending[n+1:]...)
			progress = true
		}
		if !progress {
			return errors.New("container namespace dependency cycle")
		}
	}
	p.Containers = ordered
	return nil
}

func (e *Engine) cleanupRecovery(ctx context.Context, j *restoreJournal) error {
	for _, r := range j.Containers {
		if r.OldID != "" {
			if err := e.docker(ctx, "DELETE", "/containers/"+r.OldID, nil, nil); err != nil {
				return err
			}
		}
	}
	for _, r := range j.Roots {
		if err := r.removeOwned("previous", r.TargetIdentity); err != nil {
			return err
		}
		if err := r.removeOwned("next", r.NextIdentity); err != nil {
			return err
		}
	}
	return nil
}
