package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	tutorialFeedPath       = "/api/v1/tutorials/videos"
	tutorialPlaylistID     = "PLH8p8Llp_SZM"
	tutorialFeedLimit      = 9
	tutorialFeedTTL        = time.Hour
	tutorialFeedMaxStale   = 7 * 24 * time.Hour
	tutorialFeedRetryDelay = time.Minute
)

var tutorialTitleMarkup = regexp.MustCompile(`<[^>]*>`)
var tutorialInitialData = regexp.MustCompile(`(?:var\s+ytInitialData|window\["ytInitialData"\]|ytInitialData)\s*=\s*`)

type tutorialFeedVideo struct {
	URL          string `json:"url"`
	Title        string `json:"title"`
	ThumbnailURL string `json:"thumbnailUrl"`
	thumbnail    string
}

type tutorialFeedResult struct {
	Videos []tutorialFeedVideo `json:"videos"`
	Stale  bool                `json:"stale"`
}

// Only two fixed sources can occupy this cache. Concurrent callers share one fetch
// per source, and failures are backed off without replacing the last good list.
type tutorialFeedCache struct {
	mu        sync.Mutex
	result    tutorialFeedResult
	fetchedAt time.Time
	nextFetch time.Time
	inFlight  chan struct{}
}

func (s *Server) handleTutorialFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeProblem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	if _, _, ok := s.requireSession(w, r); !ok {
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	platform := tutorialVideoPlatform(query.Get("platform"))
	if err != nil || r.URL.RawPath != "" || len(query) != 1 || len(query["platform"]) != 1 || (platform != tutorialPlatformBilibili && platform != tutorialPlatformYouTube) {
		s.writeProblem(w, r, http.StatusBadRequest, "invalid_video_platform", "Choose Bilibili or YouTube", "")
		return
	}
	result, err := s.tutorialFeed(r.Context(), platform, tutorialHTTPClient)
	if err != nil {
		s.writeProblem(w, r, http.StatusBadGateway, "video_list_unavailable", "Video list is temporarily unavailable", "")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) tutorialFeed(ctx context.Context, platform tutorialVideoPlatform, client *http.Client) (tutorialFeedResult, error) {
	index := 0
	if platform == tutorialPlatformYouTube {
		index = 1
	}
	cache := &s.tutorialFeeds[index]
	for {
		cache.mu.Lock()
		now := time.Now()
		hasCache := !cache.fetchedAt.IsZero() && now.Sub(cache.fetchedAt) <= tutorialFeedMaxStale
		if now.Before(cache.nextFetch) {
			result := cache.result
			cache.mu.Unlock()
			if hasCache {
				return result, nil
			}
			return tutorialFeedResult{}, errors.New("tutorial source retry is delayed")
		}
		if cache.inFlight != nil {
			flight := cache.inFlight
			result := cache.result
			result.Stale = true
			cache.mu.Unlock()
			if hasCache {
				return result, nil
			}
			select {
			case <-ctx.Done():
				return tutorialFeedResult{}, ctx.Err()
			case <-flight:
				continue
			}
		}
		cache.inFlight = make(chan struct{})
		cache.mu.Unlock()

		// A disconnected first caller must not cancel the shared fetch for others.
		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
		videos, err := fetchTutorialFeed(fetchCtx, platform, client)
		cancel()
		if err == nil {
			s.cacheTutorialFeedMetadata(videos, platform)
		}
		cache.mu.Lock()
		now = time.Now()
		if err == nil {
			cache.result = tutorialFeedResult{Videos: videos}
			cache.fetchedAt = now
			cache.nextFetch = now.Add(tutorialFeedTTL)
		} else {
			cache.result.Stale = true
			cache.nextFetch = now.Add(tutorialFeedRetryDelay)
		}
		close(cache.inFlight)
		cache.inFlight = nil
		result := cache.result
		hasCache = !cache.fetchedAt.IsZero() && now.Sub(cache.fetchedAt) <= tutorialFeedMaxStale
		cache.mu.Unlock()
		if hasCache {
			return result, nil
		}
		return tutorialFeedResult{}, err
	}
}

func (s *Server) cacheTutorialFeedMetadata(videos []tutorialFeedVideo, platform tutorialVideoPlatform) {
	s.tutorialMetadataMu.Lock()
	defer s.tutorialMetadataMu.Unlock()
	if s.tutorialMetadataCache == nil {
		s.tutorialMetadataCache = make(map[string]tutorialMetadataCacheEntry)
	}
	for _, video := range videos {
		link, err := parseTutorialVideoURL(video.URL)
		if err != nil {
			continue
		}
		key := string(platform) + ":" + link.id
		if link.englishThumbnail {
			key += ":en"
		}
		if len(s.tutorialMetadataCache) >= tutorialMetadataCacheLimit {
			oldestKey := ""
			var oldest time.Time
			for candidate, entry := range s.tutorialMetadataCache {
				if oldestKey == "" || entry.expires.Before(oldest) {
					oldestKey, oldest = candidate, entry.expires
				}
			}
			delete(s.tutorialMetadataCache, oldestKey)
		}
		s.tutorialMetadataCache[key] = tutorialMetadataCacheEntry{
			metadata: tutorialVideoMetadata{Title: video.Title, ThumbnailURL: video.thumbnail},
			expires:  time.Now().Add(tutorialMetadataCacheTTL),
		}
	}
}

func fetchTutorialFeed(ctx context.Context, platform tutorialVideoPlatform, client *http.Client) ([]tutorialFeedVideo, error) {
	endpoint := "https://api.bilibili.com/x/web-interface/search/type?search_type=video&keyword=kpanel&order=pubdate&page=1&page_size=9"
	if platform == tutorialPlatformYouTube {
		endpoint = "https://www.youtube.com/playlist?list=" + tutorialPlaylistID + "&hl=en"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (compatible; KPanelTutorialMetadata/1.0)")
	request.Header.Set("Referer", "https://www.bilibili.com/")
	if platform == tutorialPlatformYouTube {
		request.Header.Set("Referer", "https://www.youtube.com/")
		request.Header.Set("Accept-Language", "en-US,en;q=0.9")
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("YouTube returned HTTP %d", response.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, tutorialYouTubePageMaxBody+1))
		if err != nil {
			return nil, err
		}
		if len(body) > tutorialYouTubePageMaxBody {
			return nil, errors.New("YouTube playlist is too large")
		}
		return parseTutorialPlaylist(body)
	}
	request.Header.Set("Accept", "application/json")
	var result struct {
		Code int
		Data struct {
			Result *[]struct {
				BVID, Title, Pic string
				Pubdate          int64
			}
		}
	}
	if err := tutorialJSONRequest(client, request, tutorialMetadataMaxBody, &result); err != nil {
		return nil, err
	}
	if result.Code != 0 || result.Data.Result == nil {
		return nil, errors.New("Bilibili search is unavailable")
	}
	entries := *result.Data.Result
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Pubdate > entries[j].Pubdate })
	videos := make([]tutorialFeedVideo, 0, tutorialFeedLimit)
	seen := make(map[string]bool)
	for _, entry := range entries {
		if !tutorialBilibiliIDPattern.MatchString(entry.BVID) || seen[entry.BVID] {
			continue
		}
		pic := entry.Pic
		if strings.HasPrefix(pic, "//") {
			pic = "https:" + pic
		}
		if video, ok := makeTutorialFeedVideo("https://www.bilibili.com/video/"+entry.BVID+"/", html.UnescapeString(tutorialTitleMarkup.ReplaceAllString(entry.Title, "")), pic, platform); ok {
			videos = append(videos, video)
			seen[entry.BVID] = true
		}
		if len(videos) == tutorialFeedLimit {
			break
		}
	}
	if len(entries) > 0 && len(videos) == 0 {
		return nil, errors.New("Bilibili search contains no usable videos")
	}
	return videos, nil
}

func makeTutorialFeedVideo(videoURL, title, thumbnail string, platform tutorialVideoPlatform) (tutorialFeedVideo, bool) {
	title = cleanTutorialVideoTitle(title)
	thumbnail, ok := normalizeTutorialThumbnailURL(thumbnail, platform)
	if title == "" || !ok {
		return tutorialFeedVideo{}, false
	}
	return tutorialFeedVideo{URL: videoURL, Title: title, ThumbnailURL: tutorialThumbnailProxyURL(videoURL), thumbnail: thumbnail}, true
}

type tutorialYouTubeText struct {
	SimpleText string
	Runs       []struct{ Text string }
}

type tutorialYouTubeEndpoint struct{ VideoID, PlaylistID string }

type tutorialYouTubeItem struct {
	PlaylistVideoListRenderer *struct{ Contents []tutorialYouTubeItem }
	PlaylistVideoRenderer     *struct {
		VideoID            string
		IsPlayable         bool
		Title              tutorialYouTubeText
		Thumbnail          struct{ Thumbnails []struct{ URL string } }
		NavigationEndpoint struct{ WatchEndpoint tutorialYouTubeEndpoint }
	}
	LockupViewModel *struct {
		ContentID, ContentType string
		Metadata               struct {
			LockupMetadataViewModel struct{ Title struct{ Content string } }
		}
		ContentImage struct {
			ThumbnailViewModel struct {
				Image struct{ Sources []struct{ URL string } }
			}
		}
		RendererContext struct {
			CommandContext struct {
				OnTap struct {
					InnertubeCommand struct{ WatchEndpoint tutorialYouTubeEndpoint }
				}
			}
		}
	}
}

func parseTutorialPlaylist(body []byte) ([]tutorialFeedVideo, error) {
	match := tutorialInitialData.FindIndex(body)
	if match == nil {
		return nil, errors.New("YouTube playlist data is missing")
	}
	var data struct {
		Contents struct {
			TwoColumnBrowseResultsRenderer struct {
				Tabs []struct {
					TabRenderer struct {
						Selected bool
						Content  struct {
							SectionListRenderer struct {
								Contents []struct {
									ItemSectionRenderer struct{ Contents []tutorialYouTubeItem }
								}
							}
						}
					}
				}
			}
		}
	}
	if err := json.NewDecoder(strings.NewReader(string(body[match[1]:]))).Decode(&data); err != nil {
		return nil, err
	}
	videos := make([]tutorialFeedVideo, 0, tutorialFeedLimit)
	seen := make(map[string]bool)
	recognized := false
	for _, tab := range data.Contents.TwoColumnBrowseResultsRenderer.Tabs {
		if !tab.TabRenderer.Selected {
			continue
		}
		for _, section := range tab.TabRenderer.Content.SectionListRenderer.Contents {
			items := section.ItemSectionRenderer.Contents
			for _, item := range items {
				if item.PlaylistVideoListRenderer != nil {
					recognized = true
					items = item.PlaylistVideoListRenderer.Contents
					break
				}
			}
			for _, item := range items {
				var id, title, thumbnail string
				var endpoint tutorialYouTubeEndpoint
				if renderer := item.PlaylistVideoRenderer; renderer != nil {
					recognized = true
					if !renderer.IsPlayable {
						continue
					}
					id, title, endpoint = renderer.VideoID, renderer.Title.SimpleText, renderer.NavigationEndpoint.WatchEndpoint
					for _, run := range renderer.Title.Runs {
						title += run.Text
					}
					for _, image := range renderer.Thumbnail.Thumbnails {
						if normalized, ok := normalizeTutorialThumbnailURL(image.URL, tutorialPlatformYouTube); ok {
							thumbnail = normalized
						}
					}
				} else if renderer := item.LockupViewModel; renderer != nil {
					recognized = true
					if renderer.ContentType != "LOCKUP_CONTENT_TYPE_VIDEO" {
						continue
					}
					id, title = renderer.ContentID, renderer.Metadata.LockupMetadataViewModel.Title.Content
					endpoint = renderer.RendererContext.CommandContext.OnTap.InnertubeCommand.WatchEndpoint
					for _, image := range renderer.ContentImage.ThumbnailViewModel.Image.Sources {
						if normalized, ok := normalizeTutorialThumbnailURL(image.URL, tutorialPlatformYouTube); ok {
							thumbnail = normalized
						}
					}
				}
				if !tutorialVideoIDPattern.MatchString(id) || endpoint.VideoID != id || endpoint.PlaylistID != tutorialPlaylistID || seen[id] {
					continue
				}
				if video, ok := makeTutorialFeedVideo("https://www.youtube.com/watch?v="+id+"&hl=en", title, thumbnail, tutorialPlatformYouTube); ok {
					videos = append(videos, video)
					seen[id] = true
				}
				if len(videos) == tutorialFeedLimit {
					return videos, nil
				}
			}
		}
	}
	if !recognized {
		return nil, errors.New("YouTube playlist layout is unsupported")
	}
	return videos, nil
}
