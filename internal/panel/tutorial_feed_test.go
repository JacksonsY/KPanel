package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTutorialFeedBilibiliSortsAndValidates(t *testing.T) {
	client := &http.Client{Transport: tutorialRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.bilibili.com" || r.URL.Query().Get("keyword") != "kpanel" || r.URL.Query().Get("order") != "pubdate" || r.URL.Query().Get("page_size") != "9" || r.Header.Get("Referer") == "" {
			t.Fatalf("unexpected source: %s", r.URL)
		}
		return tutorialJSONResponse(`{"code":0,"data":{"result":[
		{"bvid":"BV13xpb6tEq7","title":"Older","pic":"//i0.hdslb.com/bfs/archive/old.jpg","pubdate":10},
		{"bvid":"BV1rvpe6dE9k","title":"<em class=keyword>KPanel</em> &amp; new","pic":"//i1.hdslb.com/bfs/archive/new.jpg","pubdate":20},
		{"bvid":"BV1rvpe6dE9k","title":"Duplicate","pic":"//i1.hdslb.com/bfs/archive/new.jpg","pubdate":20},
		{"bvid":"BV1J2HX6ME7p","title":"Unsafe image","pic":"http://127.0.0.1/internal","pubdate":30},
		{"bvid":"../../bad","title":"Invalid ID","pic":"//i0.hdslb.com/bfs/archive/new.jpg","pubdate":40}]}}`), nil
	})}
	videos, err := fetchTutorialFeed(context.Background(), tutorialPlatformBilibili, client)
	if err != nil || len(videos) != 2 || videos[0].Title != "KPanel & new" || videos[1].Title != "Older" {
		t.Fatalf("videos=%+v err=%v", videos, err)
	}
	if !strings.HasPrefix(videos[0].ThumbnailURL, tutorialThumbnailPath+"?") || videos[0].thumbnail != "https://i1.hdslb.com/bfs/archive/new.jpg" {
		t.Fatalf("unsafe cover: %+v", videos[0])
	}
}

func tutorialPlaylistFixture(t *testing.T, modern bool, ids ...string) string {
	t.Helper()
	items := make([]any, 0, len(ids))
	for _, id := range ids {
		endpoint := map[string]any{"videoId": id, "playlistId": tutorialPlaylistID}
		image := map[string]any{"url": "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg"}
		if modern {
			items = append(items, map[string]any{"lockupViewModel": map[string]any{
				"contentId": id, "contentType": "LOCKUP_CONTENT_TYPE_VIDEO",
				"metadata":        map[string]any{"lockupMetadataViewModel": map[string]any{"title": map[string]any{"content": "New " + id}}},
				"contentImage":    map[string]any{"thumbnailViewModel": map[string]any{"image": map[string]any{"sources": []any{image}}}},
				"rendererContext": map[string]any{"commandContext": map[string]any{"onTap": map[string]any{"innertubeCommand": map[string]any{"watchEndpoint": endpoint}}}},
			}})
		} else {
			items = append(items, map[string]any{"playlistVideoRenderer": map[string]any{
				"videoId": id, "isPlayable": true, "title": map[string]any{"runs": []any{map[string]any{"text": "Old " + id}}},
				"thumbnail": map[string]any{"thumbnails": []any{image}}, "navigationEndpoint": map[string]any{"watchEndpoint": endpoint},
			}})
		}
	}
	if !modern {
		items = []any{map[string]any{"playlistVideoListRenderer": map[string]any{"contents": items}}}
	}
	data := map[string]any{"contents": map[string]any{"twoColumnBrowseResultsRenderer": map[string]any{"tabs": []any{map[string]any{"tabRenderer": map[string]any{
		"selected": true, "content": map[string]any{"sectionListRenderer": map[string]any{"contents": []any{map[string]any{"itemSectionRenderer": map[string]any{"contents": items}}}}},
	}}}}}}
	body, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return "<script>var ytInitialData = " + string(body) + ";</script><script>unrelated();</script>"
}

func TestTutorialPlaylistLayoutsAndLimit(t *testing.T) {
	ids := []string{"3HwmSTCtozg", "6OFR8TxFguo", "kNBMTGF4G1k", "HLTtsALHlpw", "9z2webQ89pM", "qWs4xjed_yM", "Azlahc0Loo0", "a_yATf3ocmQ", "K46d0gR_Kkk", "N1jS2u2If60"}
	for _, modern := range []bool{false, true} {
		t.Run(fmt.Sprint(modern), func(t *testing.T) {
			body := tutorialPlaylistFixture(t, modern, append([]string{"bad-id", ids[0]}, ids...)...)
			videos, err := parseTutorialPlaylist([]byte(body))
			if err != nil || len(videos) != 9 {
				t.Fatalf("videos=%+v err=%v", videos, err)
			}
			for i, video := range videos {
				if video.URL != "https://www.youtube.com/watch?v="+ids[i]+"&hl=en" {
					t.Fatalf("wrong playlist order: %s", video.URL)
				}
			}
			wrongPlaylist := strings.ReplaceAll(body, tutorialPlaylistID, "another-list")
			videos, err = parseTutorialPlaylist([]byte(wrongPlaylist))
			if err != nil || len(videos) != 0 {
				t.Fatalf("accepted foreign playlist: %+v %v", videos, err)
			}
		})
	}
	for _, body := range []string{"<html>consent required</html>", "var ytInitialData = {bad", `var ytInitialData = {"contents":{}};`} {
		if _, err := parseTutorialPlaylist([]byte(body)); err == nil {
			t.Fatal("accepted missing/changed playlist data")
		}
	}
}

func TestTutorialFeedCacheRefreshBackoffAndExpiry(t *testing.T) {
	s := &Server{}
	var calls int
	fail := false
	client := &http.Client{Transport: tutorialRoundTripper(func(*http.Request) (*http.Response, error) {
		calls++
		if fail {
			return nil, errors.New("offline")
		}
		return tutorialJSONResponse(`{"code":0,"data":{"result":[{"bvid":"BV13xpb6tEq7","title":"KPanel","pic":"//i0.hdslb.com/bfs/archive/test.jpg","pubdate":10}]}}`), nil
	})}
	for i := 0; i < 2; i++ {
		result, err := s.tutorialFeed(context.Background(), tutorialPlatformBilibili, client)
		if err != nil || len(result.Videos) != 1 || result.Stale {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
	if calls != 1 {
		t.Fatalf("cache missed: %d", calls)
	}
	// Rendering a cover must not issue another metadata request for this video.
	metadata := s.tutorialMetadataCache["bilibili:BV13xpb6tEq7"]
	if metadata.metadata.ThumbnailURL != "https://i0.hdslb.com/bfs/archive/test.jpg" {
		t.Fatal("missing seeded thumbnail metadata")
	}
	s.tutorialFeeds[0].nextFetch = time.Time{}
	fail = true
	for i := 0; i < 2; i++ {
		result, err := s.tutorialFeed(context.Background(), tutorialPlatformBilibili, client)
		if err != nil || !result.Stale || len(result.Videos) != 1 {
			t.Fatalf("stale cache lost: %+v %v", result, err)
		}
	}
	if calls != 2 {
		t.Fatalf("failure backoff missed: %d", calls)
	}
	s.tutorialFeeds[0].fetchedAt = time.Now().Add(-tutorialFeedMaxStale - time.Hour)
	if _, err := s.tutorialFeed(context.Background(), tutorialPlatformBilibili, client); err == nil {
		t.Fatal("served expired stale list")
	}
	s.tutorialFeeds[0].nextFetch = time.Time{}
	fail = false
	result, err := s.tutorialFeed(context.Background(), tutorialPlatformBilibili, client)
	if err != nil || result.Stale {
		t.Fatalf("refresh did not recover: %+v %v", result, err)
	}
}

func TestTutorialFeedConcurrentFetchAndCanceledWaiter(t *testing.T) {
	s := &Server{}
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	client := &http.Client{Transport: tutorialRoundTripper(func(*http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return tutorialJSONResponse(`{"code":0,"data":{"result":[]}}`), nil
	})}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Go(func() {
			result, err := s.tutorialFeed(context.Background(), tutorialPlatformBilibili, client)
			if err != nil || result.Videos == nil || len(result.Videos) != 0 {
				t.Errorf("empty list/cache: %+v %v", result, err)
			}
		})
	}
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.tutorialFeed(ctx, tutorialPlatformBilibili, client); !errors.Is(err, context.Canceled) {
		t.Errorf("waiter ignored cancellation: %v", err)
	}
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("parallel upstream requests: %d", calls.Load())
	}
}

func TestTutorialFeedProviderFailures(t *testing.T) {
	for _, body := range []string{`{"code":-412}`, `{}`, `{"code":0,"data":{"result":[{"bvid":"bad"}]}}`, strings.Repeat("x", tutorialMetadataMaxBody+1)} {
		client := &http.Client{Transport: tutorialRoundTripper(func(*http.Request) (*http.Response, error) { return tutorialJSONResponse(body), nil })}
		if _, err := fetchTutorialFeed(context.Background(), tutorialPlatformBilibili, client); err == nil {
			t.Fatal("accepted invalid Bilibili response")
		}
	}
	for _, status := range []int{302, 412, 429, 500} {
		client := &http.Client{Transport: tutorialRoundTripper(func(*http.Request) (*http.Response, error) {
			r := tutorialTextResponse("unavailable")
			r.StatusCode = status
			return r, nil
		})}
		for _, platform := range []tutorialVideoPlatform{tutorialPlatformBilibili, tutorialPlatformYouTube} {
			if _, err := fetchTutorialFeed(context.Background(), platform, client); err == nil {
				t.Fatalf("accepted upstream %d", status)
			}
		}
	}
	client := &http.Client{Transport: tutorialRoundTripper(func(*http.Request) (*http.Response, error) {
		return tutorialTextResponse(strings.Repeat("x", tutorialYouTubePageMaxBody+1)), nil
	})}
	if _, err := fetchTutorialFeed(context.Background(), tutorialPlatformYouTube, client); err == nil {
		t.Fatal("accepted oversized YouTube page")
	}
}

func TestTutorialFeedRouteSessionAndValidation(t *testing.T) {
	s, token := newTestServer(t)
	if r := performRequest(s, http.MethodGet, tutorialFeedPath+"?platform=bilibili", nil, nil); r.Code != 401 {
		t.Fatalf("unauthenticated: %d", r.Code)
	}
	session, csrf := bootstrapCookies(t, s, token)
	for _, query := range []string{"", "platform=unknown", "platform=youtube&platform=bilibili", "platform=bilibili&url=http://127.0.0.1", "platform=%ZZ"} {
		r := authenticatedRequest(s, http.MethodGet, tutorialFeedPath+"?"+query, nil, session, csrf, nil)
		if r.Code != 400 {
			t.Fatalf("query=%s status=%d", query, r.Code)
		}
	}
	s.tutorialFeeds[0] = tutorialFeedCache{result: tutorialFeedResult{Videos: []tutorialFeedVideo{{URL: "https://www.bilibili.com/video/BV13xpb6tEq7/", Title: "Fresh"}}}, fetchedAt: time.Now(), nextFetch: time.Now().Add(time.Hour)}
	r := authenticatedRequest(s, http.MethodGet, tutorialFeedPath+"?platform=bilibili", nil, session, csrf, nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), "Fresh") || r.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("route contract: %d %s", r.Code, r.Body)
	}
}

// Opt-in live verification exercises the same parsers and limits used by Panel.
// Standard HTTPS_PROXY configuration can be supplied by the test environment.
func TestTutorialFeedLive(t *testing.T) {
	if os.Getenv("KPANEL_TEST_LIVE_TUTORIALS") != "1" {
		t.Skip("live provider verification is opt-in")
	}
	for _, platform := range []tutorialVideoPlatform{tutorialPlatformBilibili, tutorialPlatformYouTube} {
		t.Run(string(platform), func(t *testing.T) {
			videos, err := fetchTutorialFeed(context.Background(), platform, tutorialHTTPClient)
			if err != nil || len(videos) == 0 || len(videos) > 9 {
				t.Fatalf("live source: videos=%d err=%v", len(videos), err)
			}
			for _, video := range videos {
				t.Logf("%s %s", video.URL, video.Title)
			}
			body, contentType, err := fetchTutorialThumbnail(context.Background(), videos[0].thumbnail, platform, tutorialHTTPClient)
			if err != nil {
				t.Fatalf("live cover: %v", err)
			}
			t.Logf("cover: %s %d bytes", contentType, len(body))
		})
	}
}
