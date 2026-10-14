package scenepacks

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

// ZIP overhead is separate from the existing 5 MiB uncompressed theme quota.
const MaxThemeUploadBytes int64 = 6 << 20

// UploadTheme installs a new local theme without fetching a catalog or selecting
// it. Existing IDs cannot be overwritten; deletion uses the normal version guard.
func (s *Store) UploadTheme(ctx context.Context, data []byte, expected string) (View, error) {
	if !s.profile.themes || int64(len(data)) > MaxThemeUploadBytes {
		return View{}, ErrInvalid
	}
	if !s.opMu.TryLock() {
		return View{}, ErrBusy
	}
	defer s.opMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable {
		return View{}, ErrUnavailable
	}
	if expected == "" || expected != s.stateVersion() {
		return View{}, ErrConflict
	}
	pack, bodies, err := decodeThemeZIP(data)
	if err != nil {
		return View{}, err
	}
	if _, exists := s.state.Installed[pack.ID]; exists {
		return View{}, ErrConflict
	}
	for _, remote := range s.catalog.Packs {
		if remote.ID == pack.ID {
			return View{}, ErrConflict
		}
	}
	var total int64
	for _, item := range s.state.Installed {
		total += item.Pack.SizeBytes
	}
	if len(s.state.Installed) >= MaxInstalled || total+pack.SizeBytes > MaxTotalBytes {
		return View{}, ErrQuota
	}
	if s.pruneObjects() != nil {
		return View{}, ErrUnavailable
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return View{}, ErrUnavailable
	}
	token := hex.EncodeToString(random[:])
	target := filepath.Join(s.root, "objects", token)
	if backup.PrivateDir(target) != nil {
		return View{}, ErrUnavailable
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(target)
		}
	}()
	for _, file := range pack.Files {
		if ctx.Err() != nil || s.ctx.Err() != nil {
			return View{}, ErrUnavailable
		}
		name := filepath.Join(target, filepath.FromSlash(file.Path))
		if backup.PrivateDir(filepath.Dir(name)) != nil {
			return View{}, ErrUnavailable
		}
		if _, err := backup.CopyFile(name, bytes.NewReader(bodies[file.Path]), file.Size); err != nil {
			return View{}, ErrUnavailable
		}
	}
	next := s.nextState()
	next.Installed[pack.ID] = installedPack{Pack: pack, Token: token, Uploaded: true}
	if err := s.save(next); err != nil {
		return View{}, err
	}
	committed = true
	return s.view(pack), nil
}

func decodeThemeZIP(data []byte) (Pack, map[string][]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(archive.File) < 2 || len(archive.File) > 80 {
		return Pack{}, nil, ErrInvalid
	}
	bodies := map[string][]byte{}
	files := []File{}
	var total int64
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			if !strings.HasSuffix(file.Name, "/") || !ValidPath(file.Name+"file.txt") || file.UncompressedSize64 != 0 {
				return Pack{}, nil, ErrInvalid
			}
			continue
		}
		// Never extract ZIP paths. Only validated regular files are copied into
		// a newly generated capability directory after all content is verified.
		if !file.Mode().IsRegular() || !ValidPath(file.Name) || bodies[file.Name] != nil {
			return Pack{}, nil, ErrInvalid
		}
		switch path.Ext(file.Name) {
		case ".html", ".css", ".js", ".mjs", ".json", ".webp", ".png", ".jpg", ".avif", ".woff2", ".txt", ".md":
		default:
			return Pack{}, nil, ErrInvalid
		}
		limit := int64(5 << 20)
		if file.Name == "manifest.json" || file.Name == "index.html" {
			limit = 64 << 10
		}
		if file.UncompressedSize64 == 0 || file.UncompressedSize64 > uint64(limit) || file.UncompressedSize64 > uint64((5<<20)-total) {
			return Pack{}, nil, ErrInvalid
		}
		reader, err := file.Open()
		if err != nil {
			return Pack{}, nil, ErrInvalid
		}
		body, readErr := io.ReadAll(io.LimitReader(reader, limit+1))
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil || uint64(len(body)) != file.UncompressedSize64 || int64(len(body)) > limit {
			return Pack{}, nil, ErrInvalid
		}
		total += int64(len(body))
		bodies[file.Name] = body
		files = append(files, File{Path: file.Name, Size: int64(len(body)), SHA256: contentDigest(body)})
	}
	var pack Pack
	if json.Unmarshal(bodies["manifest.json"], &pack) != nil {
		return Pack{}, nil, ErrInvalid
	}
	// File hashes and storage paths are derived locally, never trusted from ZIP metadata.
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	pack.Path, pack.Files, pack.SizeBytes = pack.ID+"/dist", files, total
	if validatePackProfile(pack, themes) != nil {
		return Pack{}, nil, ErrInvalid
	}
	return pack, bodies, nil
}
