package notification

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/containrrr/shoutrrr/pkg/services/bark"
	"github.com/containrrr/shoutrrr/pkg/services/discord"
	"github.com/containrrr/shoutrrr/pkg/services/gotify"
	"github.com/containrrr/shoutrrr/pkg/services/ntfy"
	"github.com/containrrr/shoutrrr/pkg/services/slack"
	"github.com/containrrr/shoutrrr/pkg/services/smtp"
	"github.com/containrrr/shoutrrr/pkg/types"
)

const ShoutrrrWorkerArgument = "notification-send-worker"

type ShoutrrrAPI interface {
	SendMessage(context.Context, Provider, string, string) error
}

type ShoutrrrClient struct{}

func newShoutrrrService(provider Provider) types.Service {
	switch provider {
	case ProviderBark:
		return &bark.Service{}
	case ProviderNtfy:
		return &ntfy.Service{}
	case ProviderGotify:
		return &gotify.Service{}
	case ProviderDiscord:
		return &discord.Service{}
	case ProviderSlack:
		return &slack.Service{}
	case ProviderSMTP:
		return &smtp.Service{}
	}
	return nil
}

// Only the six explicit adapters are reachable. Do not accept arbitrary router
// schemes, headers, template parameters or alternative endpoint overrides.
func validShoutrrrURL(u *url.URL) bool {
	provider := Provider(u.Scheme)
	if newShoutrrrService(provider) == nil || u.Hostname() == "" || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" {
		return false
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return false
	}
	allowed := map[string]bool{}
	switch provider {
	case ProviderSMTP:
		allowed = map[string]bool{"from": true, "to": true, "encryption": true}
		if u.User == nil || u.Path != "/" || q.Get("encryption") != "ImplicitTLS" {
			return false
		}
		if _, err := mail.ParseAddress(q.Get("from")); err != nil {
			return false
		}
		to := strings.Split(q.Get("to"), ",")
		if len(to) > 10 {
			return false
		}
		for _, address := range to {
			if _, err := mail.ParseAddress(address); err != nil {
				return false
			}
		}
	case ProviderBark:
		if u.User == nil || u.Path != "" {
			return false
		}
		key, _ := u.User.Password()
		if key == "" {
			return false
		}
	case ProviderNtfy:
		if u.Path == "" || u.Path == "/" || strings.Contains(strings.TrimPrefix(u.Path, "/"), "/") {
			return false
		}
	case ProviderGotify:
		if u.User != nil || u.Path == "" || u.Path == "/" {
			return false
		}
		token := u.Path[strings.LastIndex(u.Path, "/")+1:]
		if len(token) != 15 || token[0] != 'A' || strings.ContainsAny(token, "/\r\n\x00") {
			return false
		}
	case ProviderDiscord:
		if u.User == nil || u.User.Username() == "" || u.Port() != "" || u.Path != "" {
			return false
		}
		if _, err := strconv.ParseUint(u.Hostname(), 10, 64); err != nil {
			return false
		}
	case ProviderSlack:
		// Webhook form only, converted from the official hooks.slack.com URL.
		if u.User == nil || u.Hostname() == "" || u.Port() != "" || len(strings.Split(strings.TrimPrefix(u.Path, "/"), "/")) != 2 {
			return false
		}
	}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 || strings.ContainsAny(values[0], "\r\n\x00") {
			return false
		}
	}
	service := newShoutrrrService(provider)
	return service.Initialize(u, nil) == nil
}

type shoutrrrRequest struct {
	Provider   Provider `json:"provider"`
	Credential string   `json:"credential"`
	Message    string   `json:"message"`
}

func (ShoutrrrClient) SendMessage(ctx context.Context, provider Provider, credential, message string) error {
	if !ValidChannelCredential(provider, credential) || newShoutrrrService(provider) == nil {
		return &Error{Code: "invalid_credential", Cause: ErrInvalidCredential}
	}
	if strings.TrimSpace(message) == "" || len(message) > 4096 || !utf8.ValidString(message) {
		return &Error{Code: "invalid_message", Cause: ErrNotReady}
	}
	ctx, cancel := context.WithTimeout(ctx, channelSendTimeout)
	defer cancel()
	executable, err := os.Executable()
	if err != nil {
		return &Error{Code: "unavailable", Cause: ErrChannelUnavailable}
	}
	input, _ := json.Marshal(shoutrrrRequest{provider, credential, message})
	command := exec.CommandContext(ctx, executable, ShoutrrrWorkerArgument)
	command.Stdin = bytes.NewReader(input)
	// Credentials travel only over stdin. Discard upstream output, which may
	// contain URLs, passwords or remote response bodies. Wait reaps the worker.
	command.Env = []string{}
	if err := command.Run(); err != nil {
		return &Error{Code: "unavailable", Retryable: true, Cause: ErrChannelUnavailable}
	}
	return nil
}

type boundedNotificationTransport struct {
	base http.RoundTripper
	host string
}

func (t boundedNotificationTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" || (t.host != "" && !strings.EqualFold(r.URL.Host, t.host)) {
		return nil, ErrInvalidCredential
	}
	res, err := t.base.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	if res.ContentLength > robotResponseBytes {
		res.Body.Close()
		return nil, ErrChannelUnavailable
	}
	res.Body = &boundedNotificationBody{ReadCloser: res.Body, remaining: robotResponseBytes}
	return res, nil
}

type boundedNotificationBody struct {
	io.ReadCloser
	remaining int64
}

func (b *boundedNotificationBody) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		return 0, ErrChannelUnavailable
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.ReadCloser.Read(p)
	b.remaining -= int64(n)
	return n, err
}

// RunShoutrrrWorker must run only in the short-lived helper process, before
// loading Panel configuration. It cannot access the notification store.
func RunShoutrrrWorker(input io.Reader) error {
	return runShoutrrrWorker(input, nil)
}

func runShoutrrrWorker(input io.Reader, base http.RoundTripper) error {
	var request shoutrrrRequest
	decoder := json.NewDecoder(io.LimitReader(input, MaxChannelCredentialBytes+8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil {
		return ErrInvalidCredential
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || !ValidChannelCredential(request.Provider, request.Credential) ||
		strings.TrimSpace(request.Message) == "" || len(request.Message) > 4096 || !utf8.ValidString(request.Message) {
		return ErrInvalidCredential
	}
	service := newShoutrrrService(request.Provider)
	if service == nil {
		return ErrInvalidCredential
	}
	u, _ := url.Parse(request.Credential)
	if service.Initialize(u, nil) != nil {
		return ErrInvalidCredential
	}
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, DisableCompression: true}
	defer transport.CloseIdleConnections()
	if base == nil {
		base = transport
	}
	host := u.Host
	if request.Provider == ProviderSlack {
		host = "hooks.slack.com"
	}
	if request.Provider == ProviderDiscord {
		host = "discord.com"
	}
	// Upstream adapters use http.DefaultClient/DefaultTransport. Changes are
	// confined to this process, never shared with the Panel or other channels.
	http.DefaultTransport = boundedNotificationTransport{base: base, host: host}
	http.DefaultClient = &http.Client{Timeout: channelSendTimeout, Transport: http.DefaultTransport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if service, ok := service.(*gotify.Service); ok {
		*service.GetHTTPClient() = *http.DefaultClient
	}
	params := types.Params{"title": "KPanel"}
	if err := service.Send(request.Message, &params); err != nil {
		return errors.New("notification delivery failed")
	}
	return nil
}
