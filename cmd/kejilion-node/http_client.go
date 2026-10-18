package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kejilion/kejilion-panel/internal/tlsfallback"
)

const nodeTLSHandshakeTimeout = 5 * time.Second

var nodePinnedHTTPClients sync.Map

func newHTTPClient() *http.Client {
	return newHTTPClientWithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12})
}

func newHTTPClientWithTLSConfig(config *tls.Config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = config.Clone()
	transport.TLSHandshakeTimeout = nodeTLSHandshakeTimeout
	transport.MaxIdleConns = 4
	transport.MaxIdleConnsPerHost = 2
	return &http.Client{
		Transport: tlsfallback.New(transport, nil), Timeout: 40 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func decodeTLSPublicKeyPin(value string) ([]byte, error) {
	pin, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(pin) != sha256.Size || base64.RawURLEncoding.EncodeToString(pin) != value {
		return nil, errors.New("TLS public key pin is invalid")
	}
	return pin, nil
}

func validLightNodeTLSPublicKeyPin(value string) bool {
	_, err := decodeTLSPublicKeyPin(value)
	return err == nil
}

func newPinnedHTTPClient(encodedPin, serverName string) (*http.Client, error) {
	pin, err := decodeTLSPublicKeyPin(encodedPin)
	if err != nil || strings.TrimSpace(serverName) == "" {
		return nil, errors.New("pinned HTTPS target is invalid")
	}
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: serverName,
		// Verification is replaced only for an enrollment that explicitly
		// carries this pin. VerifyConnection below still checks the pinned key,
		// SAN, validity period, and server-auth purpose.
		InsecureSkipVerify: true, //nolint:gosec // The complete pinned verifier is installed below.
		VerifyConnection: func(state tls.ConnectionState) error {
			return verifyPinnedServerCertificate(state, pin, serverName, time.Now())
		},
	}
	return newHTTPClientWithTLSConfig(config), nil
}

func nodeHTTPClientForOrigin(origin, encodedPin string) (*http.Client, error) {
	if encodedPin == "" {
		return nodeHTTPClient, nil
	}
	validatedOrigin, err := validateHTTPSOrigin(origin)
	if err != nil || validatedOrigin != origin {
		return nil, errors.New("pinned HTTPS origin is invalid")
	}
	if _, err := decodeTLSPublicKeyPin(encodedPin); err != nil {
		return nil, err
	}
	key := origin + "\n" + encodedPin
	if client, ok := nodePinnedHTTPClients.Load(key); ok {
		return client.(*http.Client), nil
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return nil, errors.New("pinned HTTPS origin is invalid")
	}
	client, err := newPinnedHTTPClient(encodedPin, parsed.Hostname())
	if err != nil {
		return nil, err
	}
	actual, _ := nodePinnedHTTPClients.LoadOrStore(key, client)
	return actual.(*http.Client), nil
}

func verifyPinnedServerCertificate(state tls.ConnectionState, expectedPin []byte, serverName string, now time.Time) error {
	if len(expectedPin) != sha256.Size || len(state.PeerCertificates) == 0 {
		return errors.New("pinned HTTPS certificate is unavailable")
	}
	leaf := state.PeerCertificates[0]
	if leaf == nil || leaf.NotBefore.After(now) || now.After(leaf.NotAfter) {
		return errors.New("pinned HTTPS certificate is invalid or expired")
	}
	if err := leaf.VerifyHostname(serverName); err != nil {
		return errors.New("pinned HTTPS certificate hostname does not match")
	}
	if leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return errors.New("pinned HTTPS certificate is not valid for server authentication")
	}
	serverAuth := false
	for _, usage := range leaf.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			serverAuth = true
			break
		}
	}
	if !serverAuth {
		return errors.New("pinned HTTPS certificate is not valid for server authentication")
	}
	digest := sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
	if subtle.ConstantTimeCompare(digest[:], expectedPin) != 1 {
		return errors.New("pinned HTTPS public key does not match")
	}
	return nil
}
