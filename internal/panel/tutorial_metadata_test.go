package panel

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type tutorialRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip tutorialRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestParseTutorialVideoURL(t *testing.T) {
	tests := []struct {
		name             string
		url              string
		wantPlatform     tutorialVideoPlatform
		wantID           string
		wantEnglishThumb bool
		wantError        bool
	}{
		{name: "Bilibili video", url: "https://www.bilibili.com/video/BV13xpb6tEq7/", wantPlatform: tutorialPlatformBilibili, wantID: "BV13xpb6tEq7"},
		{name: "YouTube localized video", url: "https://www.youtube.com/watch?v=3HwmSTCtozg&hl=en", wantPlatform: tutorialPlatformYouTube, wantID: "3HwmSTCtozg", wantEnglishThumb: true},
		{name: "YouTube short URL", url: "https://youtu.be/3HwmSTCtozg", wantPlatform: tutorialPlatformYouTube, wantID: "3HwmSTCtozg"},
		{name: "insecure link", url: "http://www.bilibili.com/video/BV13xpb6tEq7/", wantError: true},
		{name: "foreign host", url: "https://www.bilibili.com.attacker.example/video/BV13xpb6tEq7/", wantError: true},
		{name: "userinfo host confusion", url: "https://www.youtube.com@attacker.example/watch?v=3HwmSTCtozg", wantError: true},
		{name: "invalid video id", url: "https://www.youtube.com/watch?v=short", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseTutorialVideoURL(test.url)
			if (err != nil) != test.wantError {
				t.Fatalf("parseTutorialVideoURL() error = %v, wantError %v", err, test.wantError)
			}
			if test.wantError {
				return
			}
			if got.platform != test.wantPlatform || got.id != test.wantID || got.englishThumbnail != test.wantEnglishThumb {
				t.Fatalf("parseTutorialVideoURL() = %+v, want platform=%s id=%s englishThumbnail=%v", got, test.wantPlatform, test.wantID, test.wantEnglishThumb)
			}
		})
	}
}

func TestFetchTutorialVideoMetadataBilibili(t *testing.T) {
	client := &http.Client{Transport: tutorialRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "api.bilibili.com" || request.URL.Query().Get("bvid") != "BV13xpb6tEq7" {
			t.Fatalf("unexpected Bilibili metadata request: %s", request.URL)
		}
		return tutorialJSONResponse(`{"code":0,"data":{"title":"KPanel 使用教程 1.25","pic":"http://i1.hdslb.com/bfs/archive/example.jpg"}}`), nil
	})}
	link, err := parseTutorialVideoURL("https://www.bilibili.com/video/BV13xpb6tEq7/")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := fetchTutorialVideoMetadata(context.Background(), link, client)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Title != "KPanel 使用教程 1.25" || metadata.ThumbnailURL != "https://i1.hdslb.com/bfs/archive/example.jpg" {
		t.Fatalf("unexpected Bilibili metadata: %+v", metadata)
	}
}

func TestFetchTutorialVideoMetadataYouTubeUsesLocalizedCover(t *testing.T) {
	client := &http.Client{Transport: tutorialRoundTripper(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/oembed":
			return tutorialJSONResponse(`{"title":"KPanel overview","thumbnail_url":"https://i.ytimg.com/vi/3HwmSTCtozg/hqdefault.jpg"}`), nil
		case "/watch":
			if request.URL.Query().Get("hl") != "en" {
				t.Fatalf("localized thumbnail request did not set hl=en: %s", request.URL)
			}
			return tutorialTextResponse(`{"url":"https://i.ytimg.com/vi_lc/3HwmSTCtozg/maxresdefault_en-US.jpg?sqp=abc\u0026rs=signature"}`), nil
		default:
			t.Fatalf("unexpected YouTube request: %s", request.URL)
			return nil, nil
		}
	})}
	link, err := parseTutorialVideoURL("https://www.youtube.com/watch?v=3HwmSTCtozg&hl=en")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := fetchTutorialVideoMetadata(context.Background(), link, client)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Title != "KPanel overview" || metadata.ThumbnailURL != "https://i.ytimg.com/vi_lc/3HwmSTCtozg/maxresdefault_en-US.jpg?sqp=abc&rs=signature" {
		t.Fatalf("unexpected YouTube metadata: %+v", metadata)
	}
}

func TestTutorialMetadataRejectsUntrustedThumbnailHost(t *testing.T) {
	client := &http.Client{Transport: tutorialRoundTripper(func(*http.Request) (*http.Response, error) {
		return tutorialJSONResponse(`{"code":0,"data":{"title":"KPanel","pic":"http://127.0.0.1/private.jpg"}}`), nil
	})}
	link, err := parseTutorialVideoURL("https://www.bilibili.com/video/BV13xpb6tEq7/")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fetchTutorialVideoMetadata(context.Background(), link, client); err == nil {
		t.Fatal("fetchTutorialVideoMetadata() accepted an untrusted thumbnail host")
	}
}

func TestFetchTutorialThumbnailUsesValidatedPlatformHost(t *testing.T) {
	image := []byte{0xff, 0xd8, 0xff, 0x00, 0x01}
	client := &http.Client{Transport: tutorialRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Scheme != "https" || request.URL.Host != "i1.hdslb.com" || request.URL.Path != "/bfs/archive/example.jpg" {
			t.Fatalf("unexpected thumbnail request: %s", request.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
			Body:       io.NopCloser(bytes.NewReader(image)),
		}, nil
	})}
	body, contentType, err := fetchTutorialThumbnail(context.Background(), "http://i1.hdslb.com/bfs/archive/example.jpg", tutorialPlatformBilibili, client)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/jpeg" || !bytes.Equal(body, image) {
		t.Fatalf("unexpected proxied thumbnail: type=%q body=%v", contentType, body)
	}
}

func TestTutorialThumbnailURLStaysSameOrigin(t *testing.T) {
	videoURL := "https://www.youtube.com/watch?v=3HwmSTCtozg&hl=en"
	proxyURL, err := url.Parse(tutorialThumbnailProxyURL(videoURL))
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL.IsAbs() || proxyURL.Path != tutorialThumbnailPath || proxyURL.Query().Get("url") != videoURL {
		t.Fatalf("unexpected tutorial thumbnail proxy URL: %s", proxyURL)
	}
}

func TestTutorialMetadataRequiresSession(t *testing.T) {
	server, _ := newTestServer(t)
	path := tutorialMetadataPath + "?" + url.Values{"url": {"https://www.bilibili.com/video/BV13xpb6tEq7/"}}.Encode()
	for _, route := range []string{path, tutorialThumbnailPath + "?" + url.Values{"url": {"https://www.bilibili.com/video/BV13xpb6tEq7/"}}.Encode()} {
		response := performRequest(server, http.MethodGet, route, nil, nil)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated tutorial metadata request returned %d: %s", response.Code, response.Body.String())
		}
	}
}

func tutorialJSONResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func tutorialTextResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
