package panel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type AgentClient struct {
	socketPath string
	tokenFile  string
	maxBytes   int64
	client     *http.Client
	reads      agentReadGroup
}

type AgentResponse struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

type agentReadCall struct {
	done     chan struct{}
	cancel   context.CancelFunc
	waiters  int
	response AgentResponse
	err      error
}

// agentReadGroup coalesces identical in-flight reads without caching their
// result. This avoids repeated Docker inspections during parallel UI refreshes
// while ensuring every new refresh still observes live state.
type agentReadGroup struct {
	mu    sync.Mutex
	calls map[string]*agentReadCall
}

func (g *agentReadGroup) do(
	ctx context.Context,
	key string,
	read func(context.Context) (AgentResponse, error),
) (AgentResponse, error, bool) {
	if err := ctx.Err(); err != nil {
		return AgentResponse{}, err, false
	}
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*agentReadCall)
	}
	call, shared := g.calls[key]
	if !shared {
		// A page's cancellation belongs to its reader, not to other pages
		// sharing this request. The HTTP client retains its operation timeout.
		readCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		call = &agentReadCall{done: make(chan struct{}), cancel: cancel}
		g.calls[key] = call
		go func() {
			defer cancel()
			call.response, call.err = read(readCtx)
			g.mu.Lock()
			if g.calls[key] == call {
				delete(g.calls, key)
			}
			close(call.done)
			g.mu.Unlock()
		}()
	}
	call.waiters++
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		call.waiters--
		if call.waiters == 0 {
			if g.calls[key] == call {
				delete(g.calls, key)
			}
			call.cancel()
		}
		g.mu.Unlock()
	}()
	select {
	case <-call.done:
		return call.response, call.err, shared
	case <-ctx.Done():
		return AgentResponse{}, ctx.Err(), shared
	}
}

func NewAgentClient(socketPath, tokenFile string, maxBytes int64) *AgentClient {
	transport := &http.Transport{
		DisableCompression: true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			dialer := net.Dialer{Timeout: 3 * time.Second}
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	return &AgentClient{
		socketPath: socketPath,
		tokenFile:  tokenFile,
		maxBytes:   maxBytes,
		client: &http.Client{
			Transport: transport,
			// WordPress and declarative application installers may need to pull
			// a pinned package or image. The inbound request context still
			// cancels ordinary calls immediately.
			Timeout: 12 * time.Minute,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *AgentClient) Get(ctx context.Context, path, rawQuery, requestID string) (AgentResponse, error) {
	key := path + "?" + rawQuery
	response, err, shared := c.reads.do(ctx, key, func(readCtx context.Context) (AgentResponse, error) {
		return c.Do(readCtx, http.MethodGet, path, rawQuery, requestID, nil)
	})
	// Agent problem documents carry the originating request ID. Retry a shared
	// failure so each caller receives an independently traceable error.
	if shared && ctx.Err() == nil && (err != nil || response.StatusCode >= http.StatusBadRequest) {
		return c.Do(ctx, http.MethodGet, path, rawQuery, requestID, nil)
	}
	return response, err
}

func (c *AgentClient) Open(ctx context.Context, method, path, rawQuery, requestID string) (*http.Response, error) {
	return c.OpenStream(ctx, method, path, rawQuery, requestID, http.NoBody, nil, 0)
}

func (c *AgentClient) OpenStream(
	ctx context.Context,
	method, path, rawQuery, requestID string,
	body io.Reader,
	headers http.Header,
	contentLength int64,
) (*http.Response, error) {
	token, err := readSmallSecret(c.tokenFile)
	if err != nil {
		return nil, fmt.Errorf("read agent credential: %w", err)
	}
	if body == nil {
		body = http.NoBody
	}
	request, err := http.NewRequestWithContext(ctx, method, "http://unix"+path, body)
	if err != nil {
		return nil, fmt.Errorf("create agent request: %w", err)
	}
	request.URL.RawQuery = rawQuery
	request.Header.Set("Authorization", "Bearer "+token)
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	if contentLength >= 0 {
		request.ContentLength = contentLength
	}
	if requestID != "" {
		request.Header.Set("X-Request-ID", requestID)
	}
	streamClient := &http.Client{
		Transport: c.client.Transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := streamClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call agent: %w", err)
	}
	return response, nil
}

func (c *AgentClient) Do(ctx context.Context, method, path, rawQuery, requestID string, body []byte) (AgentResponse, error) {
	token, err := readSmallSecret(c.tokenFile)
	if err != nil {
		return AgentResponse{}, fmt.Errorf("read agent credential: %w", err)
	}
	var requestBody io.Reader = http.NoBody
	if len(body) > 0 {
		requestBody = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, "http://unix"+path, requestBody)
	if err != nil {
		return AgentResponse{}, fmt.Errorf("create agent request: %w", err)
	}
	request.URL.RawQuery = rawQuery
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	if requestID != "" {
		request.Header.Set("X-Request-ID", requestID)
	}

	response, err := c.client.Do(request)
	if err != nil {
		return AgentResponse{}, fmt.Errorf("call agent: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, c.maxBytes+1))
	if err != nil {
		return AgentResponse{}, fmt.Errorf("read agent response: %w", err)
	}
	if int64(len(responseBody)) > c.maxBytes {
		return AgentResponse{}, errors.New("agent response exceeds configured limit")
	}
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json; charset=utf-8"
	}
	return AgentResponse{
		StatusCode:  response.StatusCode,
		ContentType: contentType,
		Body:        responseBody,
	}, nil
}

func readSmallSecret(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return "", err
	}
	if len(content) > 4096 {
		return "", errors.New("secret file exceeds 4 KiB")
	}
	value := strings.TrimSpace(string(content))
	if value == "" {
		return "", errors.New("secret file is empty")
	}
	return value, nil
}
