package agent

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/filemanager"
)

func TestScreenPreviewDimensionsLimitsAndAnimation(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 2400, 1600))); err != nil {
		t.Fatal(err)
	}
	preview, mime, err := makeFilePreview(bytes.NewReader(encoded.Bytes()), int64(encoded.Len()))
	if err != nil || mime != "image/png" {
		t.Fatalf("preview: %s %v", mime, err)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(preview))
	if err != nil || config.Width != 1440 || config.Height != 960 {
		t.Fatalf("preview dimensions: %+v %v", config, err)
	}
	thumb, _, err := makeFileThumbnail(bytes.NewReader(encoded.Bytes()), int64(encoded.Len()))
	small, _, _ := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil || small.Width != 315 || small.Height != 210 {
		t.Fatalf("thumbnail changed: %+v %v", small, err)
	}
	if _, _, err := makeFilePreview(bytes.NewReader(encoded.Bytes()), fileThumbnailMaxBytes+1); err != filemanager.ErrTooLarge {
		t.Fatalf("source bound: %v", err)
	}
	// A valid oversized header is rejected before allocating its pixel buffer.
	var huge bytes.Buffer
	if err := png.Encode(&huge, image.NewNRGBA(image.Rect(0, 0, 6000, 4100))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := makeFilePreview(bytes.NewReader(huge.Bytes()), int64(huge.Len())); err != filemanager.ErrTooLarge {
		t.Fatalf("pixel bound: %v", err)
	}
	var animation bytes.Buffer
	palette := image.NewPaletted(image.Rect(0, 0, 2, 2), []color.Color{color.Black, color.White})
	if err := gif.Encode(&animation, palette, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := makeFilePreview(bytes.NewReader(animation.Bytes()), int64(animation.Len())); err != errThumbnailUnavailable {
		t.Fatalf("animation must use original: %v", err)
	}
}

func TestScreenPreviewPreservesAllJPEGOrientationsWithoutOtherEXIF(t *testing.T) {
	var source bytes.Buffer
	if err := jpeg.Encode(&source, image.NewNRGBA(image.Rect(0, 0, 80, 40)), nil); err != nil {
		t.Fatal(err)
	}
	for orientation := 1; orientation <= 8; orientation++ {
		segment := []byte{0xff, 0xe1, 0, 34, 'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, byte(orientation), 0, 0, 0, 0, 0, 0, 0}
		data := append(append(append([]byte{}, source.Bytes()[:2]...), segment...), source.Bytes()[2:]...)
		// A legal JPEG fill byte must not erase EXIF orientation.
		data = append(append(append([]byte{}, data[:2]...), 0xff), data[2:]...)
		preview, _, err := makeFilePreview(bytes.NewReader(data), int64(len(data)))
		if err != nil || fileJPEGOrientation(bytes.NewReader(preview)) != orientation {
			t.Fatalf("orientation %d: %v", orientation, err)
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(preview))
		if err != nil || config.Width != 80 || config.Height != 40 {
			t.Fatalf("small image upscaled: %+v %v", config, err)
		}
	}
	for _, data := range [][]byte{nil, {0xff, 0xd8, 0xff, 0xe1, 0, 1}, {0xff, 0xd8, 0xff, 0xe1, 0xff, 0xff}} {
		if got := fileJPEGOrientation(bytes.NewReader(data)); got != 1 {
			t.Fatalf("malformed EXIF: %d", got)
		}
	}
}

func TestScreenPreviewVersionCacheAndCancellation(t *testing.T) {
	server := testServer(t)
	server.files = writeTestPhoto(t, t.TempDir())
	path := strings.Replace(thumbnailPath(t, server.files), "mode=thumbnail", "mode=preview", 1)
	first := fileRequest(server, http.MethodGet, path, "")
	if first.Code != 200 || !strings.HasPrefix(first.Header().Get("ETag"), `"preview-`) {
		t.Fatalf("preview: %d %s", first.Code, first.Body.String())
	}
	server.previewGate <- struct{}{}
	for range cap(server.thumbnailGate) {
		server.thumbnailGate <- struct{}{}
	}
	defer func() {
		<-server.previewGate
		for range cap(server.thumbnailGate) {
			<-server.thumbnailGate
		}
	}()
	cached := fileRequest(server, http.MethodGet, path, "")
	if cached.Code != 200 || !bytes.Equal(cached.Body.Bytes(), first.Body.Bytes()) {
		t.Fatalf("cache: %d", cached.Code)
	}
	revalidated := fileRequestWithHeaders(server, http.MethodGet, path, "", map[string]string{"If-None-Match": first.Header().Get("ETag")})
	if revalidated.Code != 304 {
		t.Fatalf("revalidation: %d", revalidated.Code)
	}
	conflict := fileRequestWithHeaders(server, http.MethodGet, path+"wrong", "", map[string]string{"If-None-Match": first.Header().Get("ETag")})
	if conflict.Code != 409 {
		t.Fatalf("stale version: %d", conflict.Code)
	}
	server.previews = newPreviewCache()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	response := httptest.NewRecorder()
	// Call the same file handler with the occupied gates; no authorization bypass is added.
	server.fileRead(response, request, "preview-test", false)
	if response.Code != 408 {
		t.Fatalf("cancelled gate: %d %s", response.Code, response.Body.String())
	}
	handler := NewFileHandler(server.files)
	standalone := httptest.NewRecorder()
	handler.ServeHTTP(standalone, httptest.NewRequest(http.MethodGet, path, nil))
	if standalone.Code != 200 {
		t.Fatalf("node broker: %d", standalone.Code)
	}
}

func TestScreenPreviewDoesNotFlattenAPNG(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	chunk := make([]byte, 20)
	binary.BigEndian.PutUint32(chunk[:4], 8)
	copy(chunk[4:8], "acTL")
	binary.BigEndian.PutUint32(chunk[8:12], 2)
	binary.BigEndian.PutUint32(chunk[16:20], crc32.ChecksumIEEE(chunk[4:16]))
	data := append(append(append([]byte{}, encoded.Bytes()[:33]...), chunk...), encoded.Bytes()[33:]...)
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("valid PNG: %v", err)
	}
	if _, _, err := makeFilePreview(bytes.NewReader(data), int64(len(data))); err != errThumbnailUnavailable {
		t.Fatalf("animation flattened: %v", err)
	}
	if filePNGStatic(bytes.NewReader(data)) || filePNGStatic(bytes.NewReader(nil)) {
		t.Fatal("animation/invalid header accepted")
	}
}
