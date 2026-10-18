package panel

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"time"

	"github.com/kejilion/kejilion-panel/internal/store"
	"golang.org/x/image/webp"
)

const (
	avatarPath                  = "/api/v1/settings/avatar"
	maxAvatarRequestBytes       = store.MaxUserAvatarBytes + (32 << 10)
	maxAvatarDimension          = 2048
	maxAvatarPixels       int64 = 4 * 1024 * 1024
)

func (s *Server) handleAvatar(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawPath != "" || r.URL.RawQuery != "" {
		s.writeProblem(w, r, http.StatusBadRequest, "avatar_request_invalid", "Avatar request is invalid", "")
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", http.MethodPut)
		s.writeProblem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	if !s.checkOrigin(w, r) {
		return
	}
	_, session, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.checkCSRF(w, r, session) {
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarRequestBytes)
	if err := r.ParseMultipartForm(maxAvatarRequestBytes); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			s.writeProblem(w, r, http.StatusRequestEntityTooLarge, "avatar_file_too_large", "Avatar file is too large", "")
		} else {
			s.writeProblem(w, r, http.StatusBadRequest, "avatar_request_invalid", "Avatar request is invalid", "")
		}
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["avatar"]) != 1 || len(r.MultipartForm.Value) != 0 {
		s.writeProblem(w, r, http.StatusBadRequest, "avatar_request_invalid", "Avatar request is invalid", "")
		return
	}
	file, _, err := r.FormFile("avatar")
	if err != nil {
		s.writeProblem(w, r, http.StatusBadRequest, "avatar_request_invalid", "Avatar request is invalid", "")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, store.MaxUserAvatarBytes+1))
	if err != nil {
		s.writeProblem(w, r, http.StatusBadRequest, "avatar_request_invalid", "Avatar request is invalid", "")
		return
	}
	if len(data) == 0 {
		s.writeProblem(w, r, http.StatusUnprocessableEntity, "avatar_image_invalid", "Avatar image is invalid", "")
		return
	}
	if len(data) > store.MaxUserAvatarBytes {
		s.writeProblem(w, r, http.StatusRequestEntityTooLarge, "avatar_file_too_large", "Avatar file is too large", "")
		return
	}
	mediaType, err := validateAvatarImage(data)
	if err != nil {
		s.writeProblem(w, r, http.StatusUnprocessableEntity, "avatar_image_invalid", "Avatar image is invalid or its dimensions are too large", "")
		return
	}
	avatar := "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)
	change := map[string]any{"contentType": mediaType, "sizeBytes": len(data)}
	if err := s.audit(r, session.User.ID, "settings.avatar.update", "user", session.User.ID, "intent", change); err != nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "audit_unavailable", "Audit storage unavailable", "")
		return
	}
	if err := s.store.ReplaceUserAvatar(session.User.ID, avatar, time.Now().UTC()); err != nil {
		_ = s.audit(r, session.User.ID, "settings.avatar.update", "user", session.User.ID, "failure", change)
		s.writeProblem(w, r, http.StatusInternalServerError, "avatar_update_failed", "Avatar update failed", "")
		return
	}
	_ = s.audit(r, session.User.ID, "settings.avatar.update", "user", session.User.ID, "success", change)
	s.writeJSON(w, http.StatusOK, map[string]string{"avatar": avatar})
}

func validateAvatarImage(data []byte) (string, error) {
	var config image.Config
	var mediaType string
	var err error
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		config, err = png.DecodeConfig(bytes.NewReader(data))
		mediaType = "image/png"
	case len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8:
		config, err = jpeg.DecodeConfig(bytes.NewReader(data))
		mediaType = "image/jpeg"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		config, err = webp.DecodeConfig(bytes.NewReader(data))
		mediaType = "image/webp"
	default:
		return "", errors.New("unsupported avatar image format")
	}
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > maxAvatarDimension || config.Height > maxAvatarDimension || int64(config.Width)*int64(config.Height) > maxAvatarPixels {
		return "", errors.New("invalid avatar image dimensions")
	}
	switch mediaType {
	case "image/jpeg":
		_, err = jpeg.Decode(bytes.NewReader(data))
	case "image/png":
		_, err = png.Decode(bytes.NewReader(data))
	case "image/webp":
		_, err = webp.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return "", errors.New("invalid avatar image data")
	}
	return mediaType, nil
}
