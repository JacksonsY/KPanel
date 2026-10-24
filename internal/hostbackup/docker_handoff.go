package hostbackup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Every lifecycle start, including export cleanup and interrupted-job recovery,
// must check the daemon's current mounts before Docker resolves their paths.
func (e *Engine) startContainer(ctx context.Context, id string) error {
	var actual struct{ Mounts []Mount }
	if err := e.docker(ctx, "GET", "/containers/"+id+"/json", nil, &actual); err != nil {
		return err
	}
	if err := e.checkDockerBindSources([]Container{{Mounts: actual.Mounts}}); err != nil {
		return err
	}
	return e.docker(ctx, "POST", "/containers/"+id+"/start", nil, nil)
}

// Docker opens bind sources by pathname, outside our pinned restore handles.
// Every directory controlling that pathname must therefore remain trusted for
// the entire handoff. Rechecking inode identity alone cannot close this race.
// The final data object may belong to the application; its trusted parent
// prevents that application from replacing it with a link to another tree.
func stableDockerBindPath(anchor, source string, missingLeaf bool) error {
	if !canonicalMountPath(source) {
		return errors.New("Docker bind source is not canonical")
	}
	pending := strings.Split(strings.TrimPrefix(source, "/"), "/")
	links := 0
	for {
		root, err := os.OpenRoot(anchor)
		if err != nil {
			return err
		}
		resolved := "/"
		restart := false
		for index, component := range pending {
			if component == "" || component == "." { // The anchor itself is a final data object.
				continue
			}
			if err = requireRestorePrivateParent(root); err != nil {
				break
			}
			if component == ".." {
				pending = append(strings.Split(strings.TrimPrefix(path.Dir(resolved), "/"), "/"), pending[index+1:]...)
				restart = true
				break
			}
			var info os.FileInfo
			info, err = root.Lstat(component)
			if errors.Is(err, os.ErrNotExist) && missingLeaf && index == len(pending)-1 {
				err = nil
				break
			}
			if err != nil {
				break
			}
			if info.Mode()&os.ModeSymlink != 0 {
				// The link's parent was verified before reading it, so a lower
				// privilege writer cannot replace even a common /var/run alias.
				var target string
				target, err = root.Readlink(component)
				links++
				if err != nil || links > 40 || strings.ContainsAny(target, "\\\x00") {
					err = errors.Join(errors.New("Docker bind source has an unsupported link"), err)
					break
				}
				components := strings.Split(target, "/")
				if !path.IsAbs(target) {
					components = append(strings.Split(strings.TrimPrefix(resolved, "/"), "/"), components...)
				}
				// Keep dot-dot components until their preceding directories and
				// links have actually been resolved, just as the kernel does.
				pending = append(components, pending[index+1:]...)
				restart = true
				break
			}
			if index == len(pending)-1 {
				break
			}
			if !info.IsDir() {
				err = errors.New("Docker bind source parent is not a directory")
				break
			}
			var next *os.Root
			next, err = root.OpenRoot(component)
			if err != nil {
				break
			}
			var opened os.FileInfo
			opened, err = next.Stat(".")
			root.Close()
			root = next
			if err != nil || !os.SameFile(info, opened) {
				err = errors.Join(errors.New("Docker bind source parent changed"), err)
				break
			}
			resolved = path.Join(resolved, component)
		}
		root.Close()
		if err != nil {
			return fmt.Errorf("Docker bind source %s requires trusted, non-writable parent directories: %w", source, err)
		}
		if !restart {
			return nil
		}
	}
}

func (e *Engine) checkDockerBindSources(containers []Container) error {
	for _, c := range containers {
		for _, mount := range c.Mounts {
			if mount.Type == "bind" {
				if err := stableDockerBindPath(e.Root, mount.Source, true); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Check both the host ancestors and the permissions that will be published from
// the archive before stopping any destination writer. A nested bind's parents
// inside a restored root must be trusted too; a writable final leaf is allowed.
func (e *Engine) checkRestoreDockerBinds(all Payload, directory string) error {
	for _, c := range all.Containers {
		for _, mount := range c.Mounts {
			if mount.Type != "bind" {
				continue
			}
			covered := false
			for _, root := range all.Roots {
				if !within(root.Path, mount.Source) {
					continue
				}
				covered = true
				if err := stableDockerBindPath(e.Root, root.Path, true); err != nil {
					return err
				}
				staged := filepath.Join(directory, "unpack-"+root.Module, "data", root.ID)
				relative := strings.TrimPrefix(mount.Source, root.Path)
				if relative != "" {
					if err := stableDockerBindPath(staged, relative, false); err != nil {
						return err
					}
				}
				break
			}
			if !covered {
				if err := stableDockerBindPath(e.Root, mount.Source, true); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
