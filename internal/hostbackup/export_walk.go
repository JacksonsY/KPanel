package hostbackup

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

// Each descent pins a directory and verifies its lstat identity. All reads
// after that point are relative to that handle, including regular-file roots.
type exportWalker struct {
	ctx    context.Context
	writer *tar.Writer
	count  int
	bytes  int64
	hook   func(stage, relative string)
}

func (w *exportWalker) checkpoint(stage, relative string) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	if w.hook != nil {
		w.hook(stage, relative)
	}
	return w.ctx.Err()
}

func (w *exportWalker) writeRoot(source string, root Root) error {
	parent, err := openRestoreParent(filepath.Dir(source))
	if err != nil {
		return err
	}
	defer parent.Close()
	info, err := parent.Lstat(filepath.Base(source))
	if err != nil {
		return err
	}
	if info.IsDir() != root.Directory || !info.IsDir() && !info.Mode().IsRegular() {
		return backup.ErrInvalid
	}
	return w.entry(parent, filepath.Base(source), ".", source, root, nil, info)
}

func sameExportFile(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func (w *exportWalker) entry(parent *os.Root, name, relative, source string, root Root, boundary *os.Root, info os.FileInfo) error {
	if err := w.checkpoint("before-open", relative); err != nil {
		return err
	}
	w.count++
	if w.count > 100000 {
		return backup.ErrInvalid
	}
	var directory *os.Root
	var file *os.File
	var link string
	var err error
	switch {
	case info.IsDir():
		directory, err = parent.OpenRoot(name)
		if err != nil {
			return err
		}
		defer directory.Close()
		after, err := directory.Stat(".")
		if err != nil || !os.SameFile(info, after) {
			return errors.Join(backup.ErrInvalid, err)
		}
		if boundary == nil {
			boundary = directory
		}
		if err := w.checkpoint("directory-opened", relative); err != nil {
			return err
		}
	case info.Mode().IsRegular():
		w.bytes += info.Size()
		if info.Size() < 0 || info.Size() > 10<<30 || w.bytes > backup.MaxBytes {
			return backup.ErrInvalid
		}
		if w.writer == nil {
			// Inventory only accounts metadata; it never reads file contents.
			break
		}
		// O_NONBLOCK prevents a regular file swapped for a FIFO from hanging
		// before fstat can reject it. Root keeps symlink resolution contained.
		file, err = parent.OpenFile(name, os.O_RDONLY|exportNonblock, 0)
		if err != nil {
			return err
		}
		defer file.Close()
		before, err := file.Stat()
		if err != nil || !before.Mode().IsRegular() || !sameExportFile(info, before) {
			return errors.Join(backup.ErrInvalid, err)
		}
		if err := w.checkpoint("file-opened", relative); err != nil {
			return err
		}
	case info.Mode()&os.ModeSymlink != 0:
		if boundary == nil || relative == "." {
			return backup.ErrInvalid
		}
		link, err = parent.Readlink(name)
		if err != nil {
			return err
		}
		after, err := parent.Lstat(name)
		if err != nil || !sameExportFile(info, after) {
			return errors.Join(backup.ErrInvalid, err)
		}
		link, err = exportSymlinkTarget(boundary, source, root.Path, relative, link)
		if err != nil {
			return err
		}
	default:
		return backup.ErrInvalid
	}
	if w.writer != nil {
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		header.Name = "data/" + root.ID
		if relative != "." {
			header.Name += "/" + filepath.ToSlash(relative)
		}
		if err := w.writer.WriteHeader(header); err != nil {
			return err
		}
	}
	if file != nil {
		if _, err := io.CopyN(w.writer, contextReader{w.ctx, file}, info.Size()); err != nil {
			return err
		}
		after, err := file.Stat()
		if err != nil || !sameExportFile(info, after) {
			return errors.Join(errors.New("source changed during backup"), err)
		}
	}
	if directory == nil {
		return nil
	}
	entries, err := directory.Open(".")
	if err != nil {
		return err
	}
	defer entries.Close()
	return readDirectoryBatches(w.ctx, entries, func(batch []os.DirEntry) error {
		if err := w.checkpoint("directory-batch", relative); err != nil {
			return err
		}
		for _, item := range batch {
			// Do not trust DirEntry.Info's cached/path-based metadata. Lstat
			// and open the child through the same pinned directory handle.
			info, err := directory.Lstat(item.Name())
			if err != nil {
				return err
			}
			if err := w.entry(directory, item.Name(), filepath.Join(relative, item.Name()), source, root, boundary, info); err != nil {
				return err
			}
		}
		return nil
	})
}

// Keep directory allocation bounded before consumers apply their entry budget.
// Both discovery and pinned traversal use this same batch reader.
func readDirectoryBatches(ctx context.Context, entries *os.File, visit func([]os.DirEntry) error) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch, readErr := entries.ReadDir(128)
		if len(batch) != 0 {
			if err := visit(batch); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// Resolve link text through the pinned export root, never an absolute reopen.
// Absolute links inside the original root are rewritten to portable relative
// targets; intermediate links must remain inside that root as well.
func exportSymlinkTarget(root *os.Root, source, logicalRoot, relative, target string) (string, error) {
	normalize := func(base, value string) (string, error) {
		if strings.ContainsRune(value, '\x00') || (filepath.Separator != '\\' && strings.Contains(value, "\\")) {
			return "", backup.ErrInvalid
		}
		if filepath.IsAbs(value) {
			var err error
			value, err = filepath.Rel(source, value)
			if err != nil {
				return "", backup.ErrInvalid
			}
		} else {
			value = filepath.Join(base, value)
		}
		value = filepath.Clean(value)
		if filepath.IsAbs(value) || value == ".." || strings.HasPrefix(value, ".."+string(filepath.Separator)) {
			return "", backup.ErrInvalid
		}
		return value, nil
	}
	resolved, err := normalize(filepath.Dir(relative), target)
	if err != nil {
		return "", err
	}
	for links := 0; ; links++ {
		if links > 40 {
			return "", backup.ErrInvalid
		}
		parts := strings.Split(resolved, string(filepath.Separator))
		prefix := "."
		followed := false
		for i, part := range parts {
			prefix = filepath.Join(prefix, part)
			info, err := root.Lstat(prefix)
			if err != nil {
				return "", err
			}
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			text, err := root.Readlink(prefix)
			if err != nil {
				return "", err
			}
			next, err := normalize(filepath.Dir(prefix), text)
			if err != nil {
				return "", err
			}
			resolved = filepath.Join(append([]string{next}, parts[i+1:]...)...)
			followed = true
			break
		}
		if !followed {
			break
		}
	}
	if filepath.IsAbs(target) {
		target, err = filepath.Rel(filepath.Dir(relative), resolved)
		if err != nil {
			return "", err
		}
	}
	if err := validateArchiveSymlinkTarget(logicalRoot, filepath.ToSlash(relative), filepath.ToSlash(target)); err != nil {
		return "", err
	}
	return filepath.ToSlash(target), nil
}
