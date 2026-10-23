//go:build linux

package filemanager

import (
	"errors"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// os.Root contains symlinks within its root, but the Agent's root is usually
// /. The file manager additionally forbids following links into protected
// subtrees. Keep that policy at the filesystem boundary for every operation.
type fileRoot struct{ root *os.Root }

func rawFileRoot(root *fileRoot) *os.Root  { return root.root }
func wrapFileRoot(root *os.Root) *fileRoot { return &fileRoot{root: root} }

func openFileRoot(name string) (*fileRoot, error) {
	root, err := os.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	return &fileRoot{root: root}, nil
}

func (r *fileRoot) Name() string { return r.root.Name() }
func (r *fileRoot) Close() error { return r.root.Close() }

// directory pins the checked identity of every component. A substituted
// directory entry can neither redirect this cursor nor cross a later operation.
func (r *fileRoot) directory(name string, create bool, mode os.FileMode) (*os.Root, error) {
	if !path.IsAbs(name) && (name == "." || path.Clean(name) == name && !strings.HasPrefix(name, "../") && name != "..") {
		pinned, _, err := openCheckedDirectory(r.root, name, create, mode)
		return pinned, err
	}
	return nil, ErrInvalidPath
}

func (r *fileRoot) parent(name string) (*os.Root, string, error) {
	if name == "" || path.IsAbs(name) || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") {
		return nil, "", ErrInvalidPath
	}
	parent, err := r.directory(path.Dir(name), false, 0)
	return parent, path.Base(name), err
}

func (r *fileRoot) OpenRoot(name string) (*fileRoot, error) {
	root, err := r.directory(name, false, 0)
	if err != nil {
		return nil, err
	}
	return &fileRoot{root: root}, nil
}

func (r *fileRoot) Open(name string) (*os.File, error) {
	return r.OpenFile(name, os.O_RDONLY|unix.O_NONBLOCK, 0)
}

func (r *fileRoot) OpenFile(name string, flag int, mode os.FileMode) (*os.File, error) {
	parent, base, err := r.parent(name)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	directory, err := parent.Open(".")
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	permissions := uint32(mode.Perm())
	if mode&os.ModeSetuid != 0 {
		permissions |= unix.S_ISUID
	}
	if mode&os.ModeSetgid != 0 {
		permissions |= unix.S_ISGID
	}
	if mode&os.ModeSticky != 0 {
		permissions |= unix.S_ISVTX
	}
	// os.Root resolves a leaf link itself even when O_NOFOLLOW was supplied.
	// The native operation must consume that flag on the held parent instead.
	fd, err := unix.Openat(int(directory.Fd()), base, flag|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, permissions)
	if errors.Is(err, unix.ELOOP) {
		return nil, ErrSymlink
	}
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path.Join(parent.Name(), base))
	if flag&unix.O_PATH != 0 {
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			_ = file.Close()
			return nil, ErrSymlink
		}
	}
	return file, nil
}

func (r *fileRoot) Lstat(name string) (os.FileInfo, error) {
	parent, base, err := r.parent(name)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return parent.Lstat(base)
}

func (r *fileRoot) Stat(name string) (os.FileInfo, error) {
	info, err := r.Lstat(name)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrSymlink
	}
	return info, err
}

func (r *fileRoot) ReadFile(name string) ([]byte, error) {
	file, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func (r *fileRoot) Mkdir(name string, mode os.FileMode) error {
	parent, base, err := r.parent(name)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Mkdir(base, mode)
}

func (r *fileRoot) MkdirAll(name string, mode os.FileMode) error {
	directory, err := r.directory(name, true, mode)
	if err != nil {
		return err
	}
	return directory.Close()
}

func (r *fileRoot) Remove(name string) error {
	parent, base, err := r.parent(name)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Remove(base)
}

func (r *fileRoot) RemoveAll(name string) error {
	parent, base, err := r.parent(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.RemoveAll(base)
}

func (r *fileRoot) Rename(oldName, newName string) error {
	from, source, err := r.parent(oldName)
	if err != nil {
		return err
	}
	defer from.Close()
	to, target, err := r.parent(newName)
	if err != nil {
		return err
	}
	defer to.Close()
	return publishReceiveObject(&fileRoot{root: from}, source, &fileRoot{root: to}, target, true)
}

func (r *fileRoot) Chmod(name string, mode os.FileMode) error {
	file, err := r.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Chmod(mode)
}

func (r *fileRoot) Chtimes(name string, atime, mtime time.Time) error {
	file, err := r.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	stamps := []unix.Timespec{{Sec: atime.Unix(), Nsec: int64(atime.Nanosecond())}, {Sec: mtime.Unix(), Nsec: int64(mtime.Nanosecond())}}
	err = unix.UtimesNanoAt(int(file.Fd()), "", stamps, unix.AT_EMPTY_PATH)
	if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.ENOENT) {
		// The proc descriptor is the already-held inode, not a mutable pathname.
		return unix.UtimesNanoAt(unix.AT_FDCWD, "/proc/self/fd/"+strconv.Itoa(int(file.Fd())), stamps, 0)
	}
	return err
}
