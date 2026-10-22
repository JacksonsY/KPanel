package filemanager

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"strings"
	"time"
)

// ExportArchiveMembers streams validated members without extracting to disk.
// A raw download requires exactly one regular file; other selections become ZIP.
func (m *Manager) ExportArchiveMembers(ctx context.Context, source, version string, members []string, raw bool, output io.Writer) error {
	if len(members) == 0 || version == "" || (raw && len(members) != 1) {
		return ErrInvalidArchive
	}
	if err := validateArchiveSelection(members); err != nil {
		return err
	}
	if err := acquireNow(ctx, m.downloadGate); err != nil {
		return err
	}
	defer release(m.downloadGate)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	_, normalized, err := m.resolveExisting(source)
	if err != nil {
		return err
	}
	info, err := m.rootFS.Lstat(rootName(normalized))
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return ErrNotRegular
	}
	if resourceVersion(normalized, info) != version {
		return ErrConflict
	}
	file, err := m.rootFS.Open(rootName(normalized))
	if err != nil {
		return err
	}
	defer file.Close()
	verify := func() error {
		opened, err := file.Stat()
		if err != nil || !os.SameFile(info, opened) || resourceVersion(normalized, opened) != version {
			return ErrConflict
		}
		return m.checkExpectedVersion(normalized, version)
	}
	if err := verify(); err != nil {
		return err
	}
	found := make(map[string]bool, len(members))
	preflight := &archiveOperation{inspect: func(name string, entry os.FileInfo, _ int64) error {
		for _, member := range members {
			if raw {
				if strings.HasPrefix(name, member+"/") {
					return ErrInvalidArchive
				}
				if name == member && !entry.IsDir() {
					found[member] = true
				}
			} else if name == member || strings.HasPrefix(name, member+"/") {
				found[member] = true
			}
		}
		return nil
	}}
	read := func(operation *archiveOperation) error {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		times := []archiveDirectoryTime{}
		return m.readArchive(context.WithValue(ctx, archiveContextKey{}, operation), file, info.Size(), archiveFormatForName(normalized), "", archiveBudget(ctx, m), &times)
	}
	if err := read(preflight); err != nil {
		return err
	}
	for _, member := range members {
		if !found[member] {
			return ErrConflict
		}
	}
	if err := verify(); err != nil {
		return err
	}
	var writer *zip.Writer
	if !raw {
		writer = zip.NewWriter(output)
	}
	operation := &archiveOperation{selection: members, found: make(map[string]bool)}
	operation.export = func(name string, entry os.FileInfo, size int64, input io.Reader) error {
		target := output
		if writer != nil {
			header, err := zip.FileInfoHeader(entry)
			if err != nil {
				return err
			}
			header.Name = name
			if entry.IsDir() {
				header.Name += "/"
			} else {
				header.Method = zip.Deflate
			}
			target, err = writer.CreateHeader(header)
			if err != nil {
				return err
			}
		}
		if entry.IsDir() {
			if raw {
				return ErrInvalidArchive
			}
			return nil
		}
		written, err := io.Copy(target, io.LimitReader(&contextReader{ctx: ctx, reader: input}, size+1))
		if err != nil {
			return err
		}
		if written != size {
			return ErrInvalidArchive
		}
		return nil
	}
	if err := read(operation); err != nil {
		return err
	}
	if err := verify(); err != nil {
		return err
	}
	if writer != nil {
		return writer.Close()
	}
	return nil
}
