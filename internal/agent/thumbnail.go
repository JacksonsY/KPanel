package agent

import (
	"context"
	"errors"
	"io"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/kejilion/kejilion-panel/internal/contract"
	"github.com/kejilion/kejilion-panel/internal/filemanager"
)

// The shared generation gate bounds decoding memory and coalesces identical misses:
// a waiting request rechecks both caches before doing any image work.
func (s *Server) loadThumbnail(ctx context.Context, file io.ReadSeekCloser, entry contract.FileEntry) ([]byte, string, error) {
	key := thumbnailCacheKey(entry.Path, entry.ResourceVersion)
	if body, mime, ok := s.thumbnailDisk.get(key); ok {
		s.thumbnails.put(key, body, mime)
		return body, mime, nil
	}
	select {
	case s.thumbnailGate <- struct{}{}:
		defer func() { <-s.thumbnailGate }()
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	if body, mime, ok := s.thumbnails.get(key); ok {
		return body, mime, nil
	}
	if body, mime, ok := s.thumbnailDisk.get(key); ok {
		s.thumbnails.put(key, body, mime)
		return body, mime, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	body, mime, err := makeFileThumbnail(file, entry.SizeBytes)
	if errors.Is(err, errThumbnailUnavailable) {
		body, mime, err = makeExternalThumbnail(ctx, file, entry.SizeBytes)
	}
	// Do not occupy a download slot while rechecking access and source identity.
	_ = file.Close()
	if err != nil {
		return nil, "", err
	}
	if err = ctx.Err(); err != nil {
		return nil, "", err
	}
	check, current, err := s.files.Open(ctx, entry.Path)
	if err != nil {
		return nil, "", err
	}
	_ = check.Close()
	if current.ResourceVersion != entry.ResourceVersion {
		return nil, "", filemanager.ErrConflict
	}
	s.thumbnails.put(key, body, mime)
	s.thumbnailDisk.put(key, body, mime)
	return body, mime, nil
}
