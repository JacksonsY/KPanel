package panel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	tutorialMetadataPath       = "/api/v1/tutorials/metadata"
	tutorialThumbnailPath      = "/api/v1/tutorials/thumbnail"
	tutorialMetadataMaxURL     = 2048
	tutorialMetadataMaxBody    = 1 << 20
	tutorialThumbnailMaxBody   = 2 << 20
	tutorialYouTubePageMaxBody = 4 << 20
	tutorialMetadataCacheTTL   = 6 * time.Hour
	tutorialMetadataCacheLimit = 128
)

type tutorialVideoPlatform string

const (
	tutorialPlatformBilibili tutorialVideoPlatform = "bilibili"
	tutorialPlatformYouTube  tutorialVideoPlatform = "youtube"
)

var (
	tutorialVideoIDPattern       = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	tutorialBilibiliIDPattern    = regexp.MustCompile(`^BV[A-Za-z0-9]{10}$`)
	tutorialBilibiliImagePattern = regexp.MustCompile(`^i[0-9]\.hdslb\.com$`)
	tutorialYouTubeImagePattern  = regexp.MustCompile(`^/vi(?:_lc|_webp)?/[A-Za-z0-9_-]{11}/[^/]+\.(?:jpg|webp)$`)
	tutorialHTTPClient           = &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

type tutorialVideoLink struct {
	platform         tutorialVideoPlatform
	id               string
	englishThumbnail bool
}

type tutorialVideoMetadata struct {
	Title        string `json:"title"`
	ThumbnailURL string `json:"thumbnailUrl"`
}

type tutorialMetadataCacheEntry struct {
	metadata tutorialVideoMetadata
	expires  time.Time
}

func (s *Server) handleTutorialMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeProblem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	if _, _, ok := s.requireSession(w, r); !ok {
		return
	}
	videoURL, videoLink, err := tutorialVideoRequestURL(r)
	if err != nil {
		s.writeProblem(w, r, http.StatusBadRequest, "invalid_video_url", err.Error(), "")
		return
	}

	metadata, err := s.tutorialVideoMetadata(r.Context(), videoLink)
	if err != nil {
		s.writeProblem(w, r, http.StatusBadGateway, "video_metadata_unavailable", "Video metadata is temporarily unavailable", "")
		return
	}
	metadata.ThumbnailURL = tutorialThumbnailProxyURL(videoURL)
	w.Header().Set("Cache-Control", "private, max-age=21600")
	s.writeJSON(w, http.StatusOK, metadata)
}

func (s *Server) handleTutorialThumbnail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeProblem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	if _, _, ok := s.requireSession(w, r); !ok {
		return
	}
	_, videoLink, err := tutorialVideoRequestURL(r)
	if err != nil {
		s.writeProblem(w, r, http.StatusBadRequest, "invalid_video_url", err.Error(), "")
		return
	}
	metadata, err := s.tutorialVideoMetadata(r.Context(), videoLink)
	if err != nil {
		s.writeProblem(w, r, http.StatusBadGateway, "video_metadata_unavailable", "Video thumbnail is temporarily unavailable", "")
		return
	}
	body, contentType, err := fetchTutorialThumbnail(r.Context(), metadata.ThumbnailURL, videoLink.platform, tutorialHTTPClient)
	if err != nil {
		s.writeProblem(w, r, http.StatusBadGateway, "video_thumbnail_unavailable", "Video thumbnail is temporarily unavailable", "")
		return
	}
	digest := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(digest[:]) + `"`
	w.Header().Set("Cache-Control", "private, max-age=21600, stale-if-error=86400")
	w.Header().Set("ETag", etag)
	if requestMatchesETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func tutorialVideoRequestURL(r *http.Request) (string, tutorialVideoLink, error) {
	if r.URL.RawPath != "" {
		return "", tutorialVideoLink{}, errors.New("Video URL is invalid")
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query) != 1 || len(query["url"]) != 1 {
		return "", tutorialVideoLink{}, errors.New("A single video URL is required")
	}
	videoURL := query.Get("url")
	videoLink, err := parseTutorialVideoURL(videoURL)
	if err != nil {
		return "", tutorialVideoLink{}, errors.New("Video URL is not a supported Bilibili or YouTube link")
	}
	return videoURL, videoLink, nil
}

func tutorialThumbnailProxyURL(videoURL string) string {
	return tutorialThumbnailPath + "?" + url.Values{"url": {videoURL}}.Encode()
}

func parseTutorialVideoURL(raw string) (tutorialVideoLink, error) {
	if len(raw) == 0 || len(raw) > tutorialMetadataMaxURL {
		return tutorialVideoLink{}, errors.New("video URL length is invalid")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Opaque != "" || parsed.User != nil || parsed.RawPath != "" {
		return tutorialVideoLink{}, errors.New("video URL must be an HTTPS URL")
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return tutorialVideoLink{}, errors.New("video URL has an unsupported port")
	}
	host := strings.ToLower(parsed.Hostname())
	query := parsed.Query()
	if host == "youtube.com" || host == "www.youtube.com" || host == "m.youtube.com" {
		if parsed.Path != "/watch" || len(query["v"]) != 1 || !tutorialVideoIDPattern.MatchString(query.Get("v")) {
			return tutorialVideoLink{}, errors.New("YouTube URL must identify one video")
		}
		return tutorialVideoLink{
			platform:         tutorialPlatformYouTube,
			id:               query.Get("v"),
			englishThumbnail: tutorialLinkRequestsEnglishThumbnail(query),
		}, nil
	}
	if host == "youtu.be" {
		id := strings.TrimPrefix(parsed.Path, "/")
		if strings.Contains(id, "/") || !tutorialVideoIDPattern.MatchString(id) {
			return tutorialVideoLink{}, errors.New("YouTube URL must identify one video")
		}
		return tutorialVideoLink{platform: tutorialPlatformYouTube, id: id, englishThumbnail: tutorialLinkRequestsEnglishThumbnail(query)}, nil
	}
	if host == "bilibili.com" || host == "www.bilibili.com" || host == "m.bilibili.com" {
		const prefix = "/video/"
		if !strings.HasPrefix(parsed.Path, prefix) {
			return tutorialVideoLink{}, errors.New("Bilibili URL must identify one video")
		}
		id := strings.Trim(strings.TrimPrefix(parsed.Path, prefix), "/")
		if strings.Contains(id, "/") || !tutorialBilibiliIDPattern.MatchString(id) {
			return tutorialVideoLink{}, errors.New("Bilibili URL must identify one BV video")
		}
		return tutorialVideoLink{platform: tutorialPlatformBilibili, id: id}, nil
	}
	return tutorialVideoLink{}, errors.New("video URL host is not supported")
}

func tutorialLinkRequestsEnglishThumbnail(query url.Values) bool {
	language := strings.ToLower(query.Get("hl"))
	return language == "en" || strings.HasPrefix(language, "en-")
}

func (s *Server) tutorialVideoMetadata(ctx context.Context, link tutorialVideoLink) (tutorialVideoMetadata, error) {
	cacheKey := string(link.platform) + ":" + link.id
	if link.englishThumbnail {
		cacheKey += ":en"
	}
	now := time.Now()
	s.tutorialMetadataMu.Lock()
	if cached, ok := s.tutorialMetadataCache[cacheKey]; ok {
		if now.Before(cached.expires) {
			s.tutorialMetadataMu.Unlock()
			return cached.metadata, nil
		}
		delete(s.tutorialMetadataCache, cacheKey)
	}
	s.tutorialMetadataMu.Unlock()

	metadata, err := fetchTutorialVideoMetadata(ctx, link, tutorialHTTPClient)
	if err != nil {
		return tutorialVideoMetadata{}, err
	}
	s.tutorialMetadataMu.Lock()
	if s.tutorialMetadataCache == nil {
		s.tutorialMetadataCache = make(map[string]tutorialMetadataCacheEntry)
	}
	if len(s.tutorialMetadataCache) >= tutorialMetadataCacheLimit {
		oldestKey := ""
		var oldestExpiry time.Time
		for key, entry := range s.tutorialMetadataCache {
			if oldestKey == "" || entry.expires.Before(oldestExpiry) {
				oldestKey = key
				oldestExpiry = entry.expires
			}
		}
		delete(s.tutorialMetadataCache, oldestKey)
	}
	s.tutorialMetadataCache[cacheKey] = tutorialMetadataCacheEntry{metadata: metadata, expires: now.Add(tutorialMetadataCacheTTL)}
	s.tutorialMetadataMu.Unlock()
	return metadata, nil
}

func fetchTutorialVideoMetadata(ctx context.Context, link tutorialVideoLink, client *http.Client) (tutorialVideoMetadata, error) {
	switch link.platform {
	case tutorialPlatformBilibili:
		endpoint := "https://api.bilibili.com/x/web-interface/view?bvid=" + url.QueryEscape(link.id)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return tutorialVideoMetadata{}, err
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Referer", "https://www.bilibili.com/")
		request.Header.Set("User-Agent", "Mozilla/5.0 (compatible; KPanelTutorialMetadata/1.0)")
		var result struct {
			Code int `json:"code"`
			Data struct {
				Title string `json:"title"`
				Pic   string `json:"pic"`
			} `json:"data"`
		}
		if err := tutorialJSONRequest(client, request, tutorialMetadataMaxBody, &result); err != nil {
			return tutorialVideoMetadata{}, err
		}
		if result.Code != 0 {
			return tutorialVideoMetadata{}, fmt.Errorf("Bilibili metadata API returned code %d", result.Code)
		}
		title := cleanTutorialVideoTitle(result.Data.Title)
		if title == "" {
			return tutorialVideoMetadata{}, errors.New("Bilibili returned an empty video title")
		}
		thumbnail, ok := normalizeTutorialThumbnailURL(result.Data.Pic, tutorialPlatformBilibili)
		if !ok {
			return tutorialVideoMetadata{}, errors.New("Bilibili returned an unsupported thumbnail URL")
		}
		return tutorialVideoMetadata{Title: title, ThumbnailURL: thumbnail}, nil
	case tutorialPlatformYouTube:
		videoURL := "https://www.youtube.com/watch?v=" + url.QueryEscape(link.id)
		query := url.Values{"url": {videoURL}, "format": {"json"}}
		endpoint := "https://www.youtube.com/oembed?" + query.Encode()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return tutorialVideoMetadata{}, err
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Accept-Language", "en-US,en;q=0.9")
		request.Header.Set("User-Agent", "KPanelTutorialMetadata/1.0")
		var result struct {
			Title        string `json:"title"`
			ThumbnailURL string `json:"thumbnail_url"`
		}
		if err := tutorialJSONRequest(client, request, tutorialMetadataMaxBody, &result); err != nil {
			return tutorialVideoMetadata{}, err
		}
		title := cleanTutorialVideoTitle(result.Title)
		if title == "" {
			return tutorialVideoMetadata{}, errors.New("YouTube returned an empty video title")
		}
		thumbnail, ok := normalizeTutorialThumbnailURL(result.ThumbnailURL, tutorialPlatformYouTube)
		if !ok {
			return tutorialVideoMetadata{}, errors.New("YouTube returned an unsupported thumbnail URL")
		}
		if link.englishThumbnail {
			if localized, err := fetchEnglishYouTubeThumbnail(ctx, client, link.id); err == nil {
				thumbnail = localized
			}
		}
		return tutorialVideoMetadata{Title: title, ThumbnailURL: thumbnail}, nil
	default:
		return tutorialVideoMetadata{}, errors.New("video platform is not supported")
	}
}

func tutorialJSONRequest(client *http.Client, request *http.Request, limit int64, target any) error {
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("metadata provider returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > limit {
		return errors.New("metadata provider response is too large")
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode metadata provider response: %w", err)
	}
	return nil
}

func fetchTutorialThumbnail(ctx context.Context, rawURL string, platform tutorialVideoPlatform, client *http.Client) ([]byte, string, error) {
	thumbnailURL, ok := normalizeTutorialThumbnailURL(rawURL, platform)
	if !ok {
		return nil, "", errors.New("thumbnail URL is not from a supported video image host")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, thumbnailURL, nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("Accept", "image/webp,image/jpeg,image/png;q=0.9,*/*;q=0.1")
	request.Header.Set("User-Agent", "KPanelTutorialMetadata/1.0")
	response, err := client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("thumbnail provider returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, tutorialThumbnailMaxBody+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) == 0 || len(body) > tutorialThumbnailMaxBody {
		return nil, "", errors.New("thumbnail provider response size is invalid")
	}
	contentType := strings.TrimSpace(strings.SplitN(response.Header.Get("Content-Type"), ";", 2)[0])
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return nil, "", errors.New("thumbnail provider returned an unsupported content type")
	}
	if !siteIconBodyMatchesContentType(body, contentType) {
		return nil, "", errors.New("thumbnail content does not match its content type")
	}
	return body, contentType, nil
}

func fetchEnglishYouTubeThumbnail(ctx context.Context, client *http.Client, videoID string) (string, error) {
	query := url.Values{"v": {videoID}, "hl": {"en"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.youtube.com/watch?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	request.Header.Set("User-Agent", "Mozilla/5.0 (compatible; KPanelTutorialMetadata/1.0)")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("YouTube returned HTTP %d for localized thumbnail metadata", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, tutorialYouTubePageMaxBody+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > tutorialYouTubePageMaxBody {
		return "", errors.New("YouTube page is too large")
	}
	page := string(body)
	for _, filename := range []string{"maxresdefault_en-US.jpg", "hq720_en-US.jpg", "hqdefault_en-US.jpg"} {
		pattern := regexp.MustCompile(`"url":"(https://i\.ytimg\.com/vi_lc/` + regexp.QuoteMeta(videoID) + `/` + regexp.QuoteMeta(filename) + `[^\"]*)"`)
		match := pattern.FindStringSubmatch(page)
		if len(match) != 2 {
			continue
		}
		localizedURL, err := strconv.Unquote(`"` + match[1] + `"`)
		if err != nil {
			localizedURL = strings.ReplaceAll(match[1], `\u0026`, "&")
		}
		if thumbnail, ok := normalizeTutorialThumbnailURL(localizedURL, tutorialPlatformYouTube); ok {
			return thumbnail, nil
		}
	}
	return "", errors.New("YouTube page has no English thumbnail")
}

func normalizeTutorialThumbnailURL(raw string, platform tutorialVideoPlatform) (string, bool) {
	thumbnail, err := url.Parse(raw)
	if err != nil || thumbnail.User != nil || thumbnail.Opaque != "" || thumbnail.Port() != "" {
		return "", false
	}
	if thumbnail.Scheme != "https" && thumbnail.Scheme != "http" {
		return "", false
	}
	host := strings.ToLower(thumbnail.Hostname())
	switch platform {
	case tutorialPlatformBilibili:
		if !tutorialBilibiliImagePattern.MatchString(host) || !strings.HasPrefix(thumbnail.Path, "/bfs/archive/") {
			return "", false
		}
	case tutorialPlatformYouTube:
		if host != "i.ytimg.com" || !tutorialYouTubeImagePattern.MatchString(thumbnail.Path) {
			return "", false
		}
	default:
		return "", false
	}
	thumbnail.Scheme = "https"
	thumbnail.Host = host
	thumbnail.Fragment = ""
	thumbnail.RawFragment = ""
	return thumbnail.String(), true
}

func cleanTutorialVideoTitle(title string) string {
	fields := strings.FieldsFunc(strings.TrimSpace(title), unicode.IsSpace)
	cleaned := strings.Join(fields, " ")
	runes := []rune(cleaned)
	if len(runes) > 300 {
		cleaned = string(runes[:300])
	}
	return cleaned
}
