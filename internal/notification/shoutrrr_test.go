package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == ShoutrrrWorkerArgument {
		var request shoutrrrRequest
		if json.NewDecoder(os.Stdin).Decode(&request) != nil {
			os.Exit(1)
		}
		if request.Credential == "ntfy://:@timeout.test/sleep" {
			time.Sleep(10 * time.Second)
			os.Exit(0)
		}
		data, _ := json.Marshal(request)
		if RunShoutrrrWorker(bytes.NewReader(data)) != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestShoutrrrCredentialValidation(t *testing.T) {
	valid := []string{"ntfy://:@ntfy.example/ops", "bark://:devicekey@api.day.app", "gotify://gotify.example/push/A12345678901234", "discord://secret@12345678", "slack://@T12345678/B12345678/abcdefghijklmnopqrstuvwx", "smtp://user:pass@mail.example:465/?from=a%40example.com&to=b%40example.com&encryption=ImplicitTLS"}
	for _, value := range valid {
		if _, ok := DetectProvider(value); !ok {
			t.Errorf("valid credential rejected: %q", value)
		}
	}
	invalid := []string{"ntfy://:@ntfy.example/ops?scheme=http", "ntfy://:@ntfy.example/ops?host=other.example", "ntfy://:@ntfy.example/ops?scheme=https&scheme=http", "gotify://gotify.example/A12345678901234?disabletls=yes", "smtp://user:pass@mail.example:587/?from=a%40example.com&to=b%40example.com", "discord://secret@evil.example", "generic://evil.example", "bark://:key@api.day.app?scheme=http", "ntfy://:@ntfy.example/ops#secret"}
	for _, value := range invalid {
		if _, ok := DetectProvider(value); ok {
			t.Errorf("unsafe credential accepted: %q", value)
		}
	}
}

func TestShoutrrrRealAdapterPayloadAndBoundaries(t *testing.T) {
	oldClient, oldTransport := http.DefaultClient, http.DefaultTransport
	defer func() { http.DefaultClient = oldClient; http.DefaultTransport = oldTransport }()
	request := shoutrrrRequest{Provider: ProviderNtfy, Credential: "ntfy://:@ntfy.example/ops", Message: "KPanel alert"}
	data, _ := json.Marshal(request)
	var calls int
	transport := robotRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if r.URL.String() != "https://ntfy.example/ops" || string(body) != request.Message {
			t.Fatalf("unexpected payload to %s: %s", r.URL, body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":"test"}`)), Header: http.Header{}}, nil
	})
	if err := runShoutrrrWorker(bytes.NewReader(data), transport); err != nil || calls != 1 {
		t.Fatalf("adapter=%v calls=%d", err, calls)
	}
	t.Run("response limit", func(t *testing.T) {
		big := robotRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", int(robotResponseBytes)+1))), Header: http.Header{}}, nil
		})
		if err := runShoutrrrWorker(bytes.NewReader(data), big); err == nil {
			t.Fatal("unbounded response accepted")
		}
	})
	t.Run("destination binding", func(t *testing.T) {
		guard := boundedNotificationTransport{base: transport, host: "ntfy.example"}
		r, _ := http.NewRequest("GET", "https://other.example/", nil)
		if _, err := guard.RoundTrip(r); err == nil {
			t.Fatal("alternative host accepted")
		}
	})
}

func TestShoutrrrWorkerProcessCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := (ShoutrrrClient{}).SendMessage(ctx, ProviderNtfy, "ntfy://:@timeout.test/sleep", "notice")
	if err == nil || time.Since(start) > 2*time.Second || strings.Contains(err.Error(), "timeout.test") {
		t.Fatalf("cancellation=%v elapsed=%v", err, time.Since(start))
	}
}

func TestGotifyAdapterUsesBoundedClient(t *testing.T) {
	oldClient, oldTransport := http.DefaultClient, http.DefaultTransport
	defer func() { http.DefaultClient = oldClient; http.DefaultTransport = oldTransport }()
	data, _ := json.Marshal(shoutrrrRequest{Provider: ProviderGotify, Credential: "gotify://gotify.example/A12345678901234", Message: "alert"})
	for _, mode := range []string{"success", "redirect", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			transport := robotRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host != "gotify.example" {
					t.Fatal("notification redirected to another host")
				}
				res := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"id":1}`))}
				if mode == "redirect" {
					res.StatusCode = 307
					res.Header.Set("Location", "http://other.example/")
					res.Body = io.NopCloser(strings.NewReader("redirect"))
				}
				if mode == "oversize" {
					res.Body = io.NopCloser(strings.NewReader(strings.Repeat("x", int(robotResponseBytes)+1)))
				}
				return res, nil
			})
			err := runShoutrrrWorker(bytes.NewReader(data), transport)
			if calls != 1 || (mode == "success" && err != nil) || (mode != "success" && err == nil) {
				t.Fatalf("mode=%s calls=%d err=%v", mode, calls, err)
			}
		})
	}
}
