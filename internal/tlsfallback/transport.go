package tlsfallback

import (
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

const classicalHostCacheTTL = 5 * time.Minute

var classicalCurves = []tls.CurveID{
	tls.X25519,
	tls.CurveP256,
	tls.CurveP384,
	tls.CurveP521,
}

// HostCache temporarily remembers endpoints that need the classical TLS
// compatibility fallback. Its zero value is ready for use and may be shared.
type HostCache struct {
	mu        sync.Mutex
	classical map[string]time.Time
}

// Transport tries Go's default TLS curve preferences first and retries an
// exact TLS handshake timeout with classical curves when the request body can
// be replayed safely. A successful fallback is cached only temporarily.
type Transport struct {
	primary   http.RoundTripper
	classical http.RoundTripper
	cache     *HostCache
}

func New(primary *http.Transport, cache *HostCache) *Transport {
	if cache == nil {
		cache = &HostCache{}
	}
	classical := primary.Clone()
	if classical.TLSClientConfig == nil {
		classical.TLSClientConfig = &tls.Config{}
	} else {
		classical.TLSClientConfig = classical.TLSClientConfig.Clone()
	}
	classical.TLSClientConfig.CurvePreferences = append([]tls.CurveID(nil), classicalCurves...)
	return newTransport(primary, classical, cache)
}

func newTransport(primary, classical http.RoundTripper, cache *HostCache) *Transport {
	if cache == nil {
		cache = &HostCache{}
	}
	return &Transport{primary: primary, classical: classical, cache: cache}
}

func (transport *Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	if transport.cache.useClassical(request.URL.Host) {
		return transport.classical.RoundTrip(request)
	}

	response, err := transport.primary.RoundTrip(request)
	if err == nil || response != nil || !isTLSHandshakeTimeout(err) || !requestReplayable(request) {
		return response, err
	}

	retry := request.Clone(request.Context())
	if request.Body != nil && request.Body != http.NoBody {
		retryBody, bodyErr := request.GetBody()
		if bodyErr != nil {
			return response, err
		}
		retry.Body = retryBody
	}
	slog.Warn("TLS handshake timed out; retrying with classical key exchange", "host", request.URL.Host)
	response, retryErr := transport.classical.RoundTrip(retry)
	if retryErr == nil {
		transport.cache.rememberClassical(request.URL.Host)
	}
	return response, retryErr
}

func (cache *HostCache) useClassical(host string) bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	until, ok := cache.classical[host]
	if !ok {
		return false
	}
	if !time.Now().Before(until) {
		delete(cache.classical, host)
		return false
	}
	return true
}

func (cache *HostCache) rememberClassical(host string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	now := time.Now()
	if cache.classical == nil {
		cache.classical = make(map[string]time.Time)
	} else {
		for cachedHost, until := range cache.classical {
			if !now.Before(until) {
				delete(cache.classical, cachedHost)
			}
		}
	}
	cache.classical[host] = now.Add(classicalHostCacheTTL)
}

func (transport *Transport) CloseIdleConnections() {
	for _, roundTripper := range []http.RoundTripper{transport.primary, transport.classical} {
		if closer, ok := roundTripper.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
	}
}

func requestReplayable(request *http.Request) bool {
	return request.Body == nil || request.Body == http.NoBody || request.GetBody != nil
}

func isTLSHandshakeTimeout(err error) bool {
	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout() && strings.Contains(err.Error(), "TLS handshake timeout")
}
