package agent

import (
	"bytes"
	"context"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kejilion/kejilion-panel/internal/filemanager"
)

type thumbnailBuffer struct{ bytes.Buffer }

func (b *thumbnailBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > thumbnailCacheMaxEntry {
		return 0, filemanager.ErrTooLarge
	}
	return b.Buffer.Write(p)
}

type thumbnailReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r thumbnailReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// HEIF/AVIF need codecs outside the portable Go image decoders. Use an existing
// FFmpeg only, with a forced container, no external references, one decoder thread,
// bounded source/output/pixels and the request deadline. Never run a shell.
func makeExternalThumbnail(ctx context.Context, file io.ReadSeeker, size int64) ([]byte, string, error) {
	if size <= 0 || size > fileGridSourceMaxBytes {
		return nil, "", filemanager.ErrTooLarge
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, "", err
	}
	var header [64]byte
	n, _ := io.ReadFull(file, header[:])
	if n < 16 || string(header[4:8]) != "ftyp" {
		return nil, "", errThumbnailUnavailable
	}
	allowed := false
	for offset := 8; offset+4 <= n; offset += 4 {
		if offset == 12 {
			continue
		} // Minor version is not a brand.
		switch string(header[offset : offset+4]) {
		case "avif", "avis", "heic", "heix", "hevc", "hevx", "mif1", "msf1":
			allowed = true
		}
	}
	if !allowed {
		return nil, "", errThumbnailUnavailable
	}
	tool, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, "", errThumbnailUnavailable
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return nil, "", err
	}
	// A seekable private copy is necessary for ISO-BMFF metadata stored after pixels.
	directory, err := os.MkdirTemp("", "kpanel-thumbnail-")
	if err != nil {
		return nil, "", err
	}
	defer os.Remove(directory)
	name := filepath.Join(directory, "source")
	temp, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, "", err
	}
	defer os.Remove(name)
	written, copyErr := io.Copy(temp, thumbnailReader{ctx, io.LimitReader(file, fileGridSourceMaxBytes+1)})
	closeErr := temp.Close()
	if copyErr != nil {
		return nil, "", copyErr
	}
	if closeErr != nil {
		return nil, "", closeErr
	}
	if written != size || written > fileGridSourceMaxBytes {
		return nil, "", filemanager.ErrConflict
	}
	cmd := exec.CommandContext(ctx, tool, "-nostdin", "-hide_banner", "-loglevel", "error", "-max_alloc", "67108864", "-cpucount", "1",
		"-threads", "1", "-filter_threads", "1", "-max_pixels", strconv.Itoa(fileThumbnailMaxPixels), "-protocol_whitelist", "file,pipe",
		"-f", "mov", "-enable_drefs", "0", "-use_absolute_path", "0", "-i", name,
		"-map", "0:v:0", "-frames:v", "1", "-an", "-sn", "-dn", "-vf", "scale=w='min(320,iw)':h='min(210,ih)':force_original_aspect_ratio=decrease",
		"-threads", "1", "-c:v", "mjpeg", "-q:v", "3", "-f", "image2pipe", "pipe:1")
	// Do not inherit FFREPORT or other per-user FFmpeg configuration/logging knobs.
	for _, key := range []string{"PATH", "SystemRoot", "WINDIR"} {
		if value := os.Getenv(key); value != "" {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	cmd.Env = append(cmd.Env, "TMPDIR="+directory, "TEMP="+directory, "TMP="+directory)
	var output thumbnailBuffer
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	if err = cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return nil, "", errThumbnailUnavailable
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(output.Bytes()))
	if err != nil || format != "jpeg" || config.Width <= 0 || config.Height <= 0 || config.Width > fileThumbnailMaxWidth || config.Height > fileThumbnailMaxHeight {
		return nil, "", errThumbnailUnavailable
	}
	return bytes.Clone(output.Bytes()), "image/" + strings.ToLower(format), nil
}
