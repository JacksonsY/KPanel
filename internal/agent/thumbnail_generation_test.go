package agent

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/filemanager"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

type changingImageReader struct {
	*bytes.Reader
	replacement []byte
	seeks       int
}

func (r *changingImageReader) Seek(offset int64, whence int) (int64, error) {
	r.seeks++
	r.Reader = bytes.NewReader(r.replacement)
	return r.Reader.Seek(offset, whence)
}

func TestThumbnailUsesOneBoundedImageSnapshot(t *testing.T) {
	var small, larger bytes.Buffer
	if err := png.Encode(&small, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&larger, image.NewNRGBA(image.Rect(0, 0, 96, 96))); err != nil {
		t.Fatal(err)
	}
	input := &changingImageReader{Reader: bytes.NewReader(small.Bytes()), replacement: larger.Bytes()}
	body, _, err := makeFileImage(input, int64(small.Len()), 320, 210, 256, false)
	if err != nil {
		t.Fatal(err)
	}
	if input.seeks != 0 {
		t.Fatalf("decoder reread mutable input %d times after checking its header", input.seeks)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width != 8 || config.Height != 8 {
		t.Fatalf("snapshot dimensions=%+v error=%v", config, err)
	}
	// The observed byte count, not the stale size argument, bounds the snapshot.
	if _, _, err := makeFilePreview(io.NewSectionReader(bytes.NewReader(make([]byte, fileThumbnailMaxBytes+1)), 0, fileThumbnailMaxBytes+1), 1); err != filemanager.ErrTooLarge {
		t.Fatalf("growing source byte bound: %v", err)
	}
}

func TestThumbnailPortableFormatsAndLargerSources(t *testing.T) {
	for _, format := range []string{"jpeg", "png", "bmp", "tiff", "webp"} {
		t.Run(format, func(t *testing.T) {
			var data bytes.Buffer
			source := image.NewNRGBA(image.Rect(0, 0, 640, 480))
			var err error
			switch format {
			case "jpeg":
				err = jpeg.Encode(&data, source, nil)
			case "png":
				err = png.Encode(&data, source)
			case "bmp":
				err = bmp.Encode(&data, source)
			case "tiff":
				err = tiff.Encode(&data, source, nil)
			case "webp":
				var b []byte
				b, err = os.ReadFile("testdata/thumbnails/sample.webp")
				data.Write(b)
			}
			if err != nil {
				t.Fatal(err)
			}
			body, _, err := makeFileThumbnail(bytes.NewReader(data.Bytes()), int64(data.Len()))
			if err != nil {
				t.Fatal(err)
			}
			config, _, err := image.DecodeConfig(bytes.NewReader(body))
			if err != nil || config.Width > 320 || config.Height > 210 {
				t.Fatalf("dimensions %+v, %v", config, err)
			}
		})
	}
	// A normal 12MP phone image previously failed the 8MP grid limit.
	var phone bytes.Buffer
	if err := jpeg.Encode(&phone, image.NewYCbCr(image.Rect(0, 0, 4032, 3024), image.YCbCrSubsampleRatio420), nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := makeFileThumbnail(bytes.NewReader(phone.Bytes()), int64(phone.Len())); err != nil {
		t.Fatalf("phone: %v", err)
	}
	// Uncompressed scans exceed the old 12 MiB compressed-file limit.
	var scan bytes.Buffer
	if err := bmp.Encode(&scan, image.NewNRGBA(image.Rect(0, 0, 2400, 1800))); err != nil {
		t.Fatal(err)
	}
	if scan.Len() <= 12<<20 {
		t.Fatal("fixture must cross old source limit")
	}
	if _, _, err := makeFileThumbnail(bytes.NewReader(scan.Bytes()), int64(scan.Len())); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if _, _, err := makeFileThumbnail(bytes.NewReader(nil), fileGridSourceMaxBytes+1); err != filemanager.ErrTooLarge {
		t.Fatalf("source bound: %v", err)
	}
	// A crafted valid header must be rejected before allocating a pixel buffer.
	if _, _, err := makeFileImage(bytes.NewReader(phone.Bytes()), int64(phone.Len()), 320, 210, 8_000_000, false); err != filemanager.ErrTooLarge {
		t.Fatalf("pixel bound: %v", err)
	}
}

func TestThumbnailOptionalAVIFConverter(t *testing.T) {
	if os.Getenv("KPANEL_TEST_MEDIA_TOOLS") != "1" {
		t.Skip("optional host decoder integration")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Fatal("requested decoder test requires ffmpeg")
	}
	data, err := os.ReadFile("testdata/thumbnails/sample.avif")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body, mime, err := makeExternalThumbnail(ctx, bytes.NewReader(data), int64(len(data)))
	if err != nil || mime != "image/jpeg" {
		t.Fatalf("avif: %s %v", mime, err)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width != 32 || config.Height != 20 {
		t.Fatalf("avif dimensions %+v %v", config, err)
	}
	if _, _, err := makeExternalThumbnail(ctx, bytes.NewReader([]byte("not an image")), 12); err != errThumbnailUnavailable {
		t.Fatalf("unknown input: %v", err)
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	if _, _, err := makeExternalThumbnail(canceled, bytes.NewReader(data), int64(len(data))); err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestThumbnailOptionalHEICConverter(t *testing.T) {
	name := os.Getenv("KPANEL_TEST_HEIC_FILE")
	if name == "" {
		t.Skip("optional external HEIC fixture")
	}
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	body, mime, err := makeExternalThumbnail(ctx, bytes.NewReader(data), int64(len(data)))
	if err != nil || mime != "image/jpeg" {
		t.Fatalf("heic: %s %v", mime, err)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > 320 || config.Height > 210 {
		t.Fatalf("heic dimensions: %+v %v", config, err)
	}
}
