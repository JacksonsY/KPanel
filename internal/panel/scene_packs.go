package panel

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kejilion/kejilion-panel/internal/scenepacks"
)

const scenePacksPath = "/api/v1/desktop/scene-packs"

// A browser can request every file in one pack, plus every catalog image, in parallel.
// Keep that burst bounded while limiting buffered file bodies to four concurrent streams.
const maxScenePackStreamQueue = scenepacks.MaxFiles + 2*scenepacks.MaxPacks

var sceneHostPattern = regexp.MustCompile(`^[a-zA-Z0-9.:[\]-]+$`)

func scenePackFilePath(requestPath string) (id, token, name string, ok bool) {
	return libraryFilePath(requestPath, scenePacksPath)
}

func libraryFilePath(requestPath, prefix string) (id, token, name string, ok bool) {
	if !strings.HasPrefix(requestPath, prefix+"/") {
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(requestPath, prefix+"/"), "/", 4)
	if len(parts) != 4 || parts[1] != "files" || !scenepacks.ValidID(parts[0]) || !scenepacks.ValidToken(parts[2]) || !scenepacks.ValidPath(parts[3]) {
		return
	}
	return parts[0], parts[2], parts[3], true
}

const shareThemesPath = "/api/v1/cluster/share-themes"

func (s *Server) handleScenePacks(w http.ResponseWriter, r *http.Request) {
	s.handlePackLibrary(w, r, s.scenePacks, scenePacksPath, false)
}
func (s *Server) handleShareThemes(w http.ResponseWriter, r *http.Request) {
	s.handlePackLibrary(w, r, s.shareThemes, shareThemesPath, true)
}

func (s *Server) handlePackLibrary(w http.ResponseWriter, r *http.Request, library *scenepacks.Store, prefix string, isTheme bool) {
	if r.URL.RawPath != "" || len(r.URL.RawQuery) > 100 || (r.URL.RawQuery != "" && (!strings.HasSuffix(r.URL.Path, "/thumb") || len(r.URL.Query()) != 1 || len(r.URL.Query()["v"]) != 1)) {
		s.writeProblem(w, r, http.StatusBadRequest, "scene_pack_request_invalid", "Invalid scene pack request", "")
		return
	}
	if library == nil {
		s.scenePackError(w, r, scenepacks.ErrUnavailable)
		return
	}
	if id, token, name, ok := libraryFilePath(r.URL.Path, prefix); ok {
		if r.Method != http.MethodGet {
			s.scenePackMethodError(w, r)
			return
		}
		if !s.scenePackStream(w, r) {
			return
		}
		defer func() { <-s.scenePackStreams }()
		body, contentType, err := library.File(id, token, name)
		if err != nil {
			s.scenePackError(w, r, err)
			return
		}
		if !sceneHostPattern.MatchString(r.Host) {
			s.scenePackError(w, r, scenepacks.ErrInvalid)
			return
		}
		scheme := "http"
		if s.requestUsesHTTPS(r) {
			scheme = "https"
		}
		base := scheme + "://" + r.Host + prefix + "/" + id + "/files/" + token + "/"
		// A path-scoped source (not 'self') prevents the opaque frame from sending
		// requests to other Panel endpoints. CORS applies only to capability files.
		w.Header().Set("Content-Security-Policy", "sandbox allow-scripts; default-src 'none'; script-src "+base+" 'wasm-unsafe-eval'; style-src "+base+" 'unsafe-inline'; img-src "+base+" data: blob:; media-src "+base+" blob:; font-src "+base+" data:; connect-src "+base+"; worker-src "+base+" blob:; frame-src 'none'; frame-ancestors 'self'; form-action 'none'; base-uri 'none'; object-src 'none'")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=(), fullscreen=(), autoplay=()")
		s.serveScenePackBytes(w, r, body, contentType, name)
		return
	}
	if r.Method != http.MethodGet && !s.checkOrigin(w, r) {
		return
	}
	_, session, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet && !s.checkCSRF(w, r, session) {
		return
	}
	if r.URL.Path == prefix && r.Method == http.MethodGet {
		list, err := library.List(r.Context())
		if err != nil {
			s.scenePackError(w, r, err)
			return
		}
		s.writeJSON(w, http.StatusOK, list)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, prefix+"/"), "/")
	if isTheme && len(parts) == 1 && parts[0] == "upload" && r.Method == http.MethodPost {
		if !s.scenePackStream(w, r) {
			return
		}
		defer func() { <-s.scenePackStreams }()
		r.Body = http.MaxBytesReader(w, r.Body, scenepacks.MaxThemeUploadBytes+(64<<10))
		reader, err := r.MultipartReader()
		if err != nil {
			s.scenePackError(w, r, scenepacks.ErrInvalid)
			return
		}
		var data []byte
		var expected string
		seen := map[string]bool{}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				s.scenePackError(w, r, scenepacks.ErrInvalid)
				return
			}
			name := part.FormName()
			if seen[name] || (name != "file" && name != "expectedResourceVersion") {
				s.scenePackError(w, r, scenepacks.ErrInvalid)
				return
			}
			seen[name] = true
			limit := scenepacks.MaxThemeUploadBytes
			if name == "expectedResourceVersion" {
				limit = 100
			}
			body, err := io.ReadAll(io.LimitReader(part, limit+1))
			if err != nil || int64(len(body)) > limit {
				s.scenePackError(w, r, scenepacks.ErrInvalid)
				return
			}
			if name == "file" {
				data = body
			} else {
				expected = string(body)
			}
		}
		if data == nil || !resourceVersionPattern.MatchString(expected) {
			s.scenePackError(w, r, scenepacks.ErrInvalid)
			return
		}
		const action = "cluster.share-theme.upload"
		if s.audit(r, session.User.ID, action, "cluster-share-theme", "upload", "intent", nil) != nil {
			s.writeProblem(w, r, http.StatusServiceUnavailable, "audit_unavailable", "Audit storage unavailable", "")
			return
		}
		result, err := library.UploadTheme(r.Context(), data, expected)
		if err != nil {
			_ = s.audit(r, session.User.ID, action, "cluster-share-theme", "upload", "failure", nil)
			s.scenePackError(w, r, err)
			return
		}
		_ = s.audit(r, session.User.ID, action, "cluster-share-theme", result.ID, "success", nil)
		s.writeJSON(w, http.StatusOK, result)
		return
	}
	if !isTheme && len(parts) == 2 && scenepacks.ValidID(parts[0]) && (parts[1] == "thumb" || parts[1] == "poster") && r.Method == http.MethodGet {
		if !s.scenePackStream(w, r) {
			return
		}
		defer func() { <-s.scenePackStreams }()
		body, err := library.Image(r.Context(), parts[0], parts[1]+".webp")
		if err != nil {
			s.scenePackError(w, r, err)
			return
		}
		s.serveSceneBytes(w, body, "image/webp")
		return
	}
	var input struct {
		Source                  string `json:"source"`
		Selected                string `json:"selected"`
		ExpectedResourceVersion string `json:"expectedResourceVersion"`
	}
	sourceChange := len(parts) == 1 && parts[0] == "source" && r.Method == http.MethodPut
	install := len(parts) == 2 && scenepacks.ValidID(parts[0]) && parts[1] == "install" && r.Method == http.MethodPost
	remove := len(parts) == 1 && scenepacks.ValidID(parts[0]) && r.Method == http.MethodDelete
	selectTheme := isTheme && len(parts) == 1 && parts[0] == "selection" && r.Method == http.MethodPut
	if !sourceChange && !install && !remove && !selectTheme {
		s.scenePackMethodError(w, r)
		return
	}
	if err := decodeLimitedJSON(w, r, 1024, &input); err != nil {
		return
	}
	if sourceChange && !scenepacks.ValidSource(input.Source) || !sourceChange && !resourceVersionPattern.MatchString(input.ExpectedResourceVersion) {
		s.scenePackError(w, r, scenepacks.ErrInvalid)
		return
	}
	action := "desktop.scene-pack.install"
	if sourceChange {
		action = "desktop.scene-pack.source"
	} else if remove {
		action = "desktop.scene-pack.delete"
	}
	if selectTheme {
		action = "desktop.scene-pack.select"
	}
	if isTheme {
		action = strings.Replace(action, "desktop.scene-pack", "cluster.share-theme", 1)
	}
	if err := s.audit(r, session.User.ID, action, "desktop-scene-pack", parts[0], "intent", nil); err != nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "audit_unavailable", "Audit storage unavailable", "")
		return
	}
	var result any
	var err error
	if selectTheme {
		err = library.Select(input.Selected, input.ExpectedResourceVersion)
		result = map[string]string{"selected": input.Selected}
	} else if sourceChange {
		err = library.SetSource(input.Source)
		result = map[string]string{"source": input.Source}
	} else if install {
		// paneld defaults to a 60-second write deadline. Catalog refresh plus a
		// bounded 3-minute installation must still be able to return its result.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(4 * time.Minute))
		result, err = library.Install(r.Context(), parts[0], input.ExpectedResourceVersion)
	} else {
		err = library.Delete(parts[0], input.ExpectedResourceVersion)
	}
	if err != nil {
		_ = s.audit(r, session.User.ID, action, "desktop-scene-pack", parts[0], "failure", nil)
		s.scenePackError(w, r, err)
		return
	}
	_ = s.audit(r, session.User.ID, action, "desktop-scene-pack", parts[0], "success", nil)
	if remove {
		w.WriteHeader(http.StatusNoContent)
	} else {
		s.writeJSON(w, http.StatusOK, result)
	}
}

func (s *Server) scenePackStream(w http.ResponseWriter, r *http.Request) bool {
	if r.Context().Err() != nil {
		return false
	}
	select {
	case s.scenePackStreamQueue <- struct{}{}:
		defer func() { <-s.scenePackStreamQueue }()
	default:
		s.scenePackError(w, r, scenepacks.ErrBusy)
		return false
	}
	select {
	case s.scenePackStreams <- struct{}{}:
		if r.Context().Err() != nil {
			<-s.scenePackStreams
			return false
		}
		return true
	case <-r.Context().Done():
		return false
	}
}
func (s *Server) serveSceneBytes(w http.ResponseWriter, body []byte, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// Scene file URLs contain an unguessable, per-install capability. Allow the
// browser to retain bytes, but require revalidation before every reuse so an
// install replacement or uninstall immediately revokes the old capability.
func (s *Server) serveScenePackBytes(w http.ResponseWriter, r *http.Request, body []byte, contentType, name string) {
	digest := sha256.Sum256(body)
	etag := `W/"` + hex.EncodeToString(digest[:]) + `"`
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, no-cache, must-revalidate")
	w.Header().Set("ETag", etag)
	addVary(w.Header(), "Accept-Encoding")
	if matchesScenePackETag(r.Header.Values("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	payload := body
	compressed := len(body) >= 1024 && r.Header.Get("Range") == "" && acceptsGzip(r.Header.Get("Accept-Encoding")) && compressibleScenePackFile(name)
	if compressed {
		var buffer bytes.Buffer
		writer, err := gzip.NewWriterLevel(&buffer, gzip.BestSpeed)
		if err == nil {
			_, writeErr := writer.Write(body)
			closeErr := writer.Close()
			if writeErr == nil && closeErr == nil && float64(buffer.Len()) < float64(len(body))*0.9 {
				payload = buffer.Bytes()
				w.Header().Set("Content-Encoding", "gzip")
			}
		}
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func matchesScenePackETag(values []string, etag string) bool {
	current := strings.TrimPrefix(etag, "W/")
	for _, value := range values {
		for _, candidate := range strings.Split(value, ",") {
			candidate = strings.TrimSpace(candidate)
			if candidate == "*" || strings.TrimPrefix(candidate, "W/") == current {
				return true
			}
		}
	}
	return false
}

func compressibleScenePackFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".js", ".mjs", ".css", ".json", ".gltf", ".bin", ".glb", ".wasm", ".txt", ".md":
		return true
	default:
		return false
	}
}

func (s *Server) scenePackMethodError(w http.ResponseWriter, r *http.Request) {
	s.writeProblem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
}
func (s *Server) scenePackError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, title := http.StatusServiceUnavailable, "scene_pack_unavailable", "Scene pack storage unavailable"
	switch {
	case errors.Is(err, scenepacks.ErrNotFound):
		status, code, title = http.StatusNotFound, "scene_pack_not_found", "Scene pack not found"
	case errors.Is(err, scenepacks.ErrInvalid):
		status, code, title = http.StatusBadRequest, "scene_pack_invalid", "Invalid scene pack or request"
	case errors.Is(err, scenepacks.ErrConflict):
		status, code, title = http.StatusConflict, "scene_pack_changed", "Scene pack changed; refresh and retry"
	case errors.Is(err, scenepacks.ErrBusy):
		status, code, title = http.StatusConflict, "scene_pack_busy", "Scene pack operation in progress"
	case errors.Is(err, scenepacks.ErrQuota):
		status, code, title = http.StatusInsufficientStorage, "scene_pack_quota", "Scene pack storage limit reached"
	case errors.Is(err, scenepacks.ErrSource):
		status, code, title = http.StatusBadGateway, "scene_pack_source_unavailable", "Scene download or verification failed"
	}
	s.writeProblem(w, r, status, code, title, "")
}
