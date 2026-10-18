package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/tlsfallback"
)

func TestNodeHTTPClientUsesTLSFallback(t *testing.T) {
	t.Parallel()

	client := newHTTPClient()
	if _, ok := client.Transport.(*tlsfallback.Transport); !ok {
		t.Fatal("HTTP client is not using TLS fallback transport")
	}
	if client.Timeout <= nodeTLSHandshakeTimeout {
		t.Fatal("HTTP client timeout cannot cover fallback handshake")
	}
	if client.CheckRedirect == nil {
		t.Fatal("redirect policy missing")
	}
	request, err := http.NewRequest(http.MethodGet, "https://panel.example", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(request, nil); err != http.ErrUseLastResponse {
		t.Fatalf("redirect error = %v, want %v", err, http.ErrUseLastResponse)
	}
}

func TestPinnedHTTPClientVerifiesSelfSignedKeyAndHostname(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	certificate := server.Certificate()
	if certificate == nil {
		t.Fatal("test server certificate is unavailable")
	}
	digest := sha256.Sum256(certificate.RawSubjectPublicKeyInfo)
	pin := base64.RawURLEncoding.EncodeToString(digest[:])

	client, err := nodeHTTPClientForOrigin(server.URL, pin)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("matching self-signed certificate was rejected: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("pinned response status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}

	wrongPin := base64.RawURLEncoding.EncodeToString(make([]byte, sha256.Size))
	if wrongPin == pin {
		wrongPin = base64.RawURLEncoding.EncodeToString([]byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"))
	}
	wrongClient, err := nodeHTTPClientForOrigin(server.URL, wrongPin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wrongClient.Get(server.URL); err == nil {
		t.Fatal("self-signed certificate with a different public key was accepted")
	}

	wrongHostClient, err := newPinnedHTTPClient(pin, "wrong.example")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wrongHostClient.Get(server.URL); err == nil {
		t.Fatal("certificate with a mismatched hostname was accepted")
	}

	state := tls.ConnectionState{PeerCertificates: []*x509.Certificate{certificate}}
	if err := verifyPinnedServerCertificate(state, digest[:], "127.0.0.1", certificate.NotAfter.Add(time.Second)); err == nil {
		t.Fatal("expired pinned certificate was accepted")
	}
}

func TestUnpinnedHTTPClientRetainsSystemCAValidation(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	client, err := nodeHTTPClientForOrigin(server.URL, "")
	if err != nil || client != nodeHTTPClient {
		t.Fatalf("unpinned origin client = %p, %p, %v; want the existing CA-verifying client", client, nodeHTTPClient, err)
	}
	if _, err := client.Get(server.URL); err == nil {
		t.Fatal("unpinned client accepted an unknown self-signed certificate")
	}
}

func TestLightReportReusesPersistedSelfSignedPin(t *testing.T) {
	received := make(chan struct{}, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != lightReportPath {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, fmt.Sprintf(`{"acceptedAt":%q,"nextReportSeconds":30}`, time.Now().UTC().Format(time.RFC3339Nano)))
		received <- struct{}{}
	}))
	defer server.Close()
	certificate := server.Certificate()
	digest := sha256.Sum256(certificate.RawSubjectPublicKeyInfo)
	config := nodeConfig{
		SchemaVersion: 1, Origin: server.URL, TLSPublicKeyPin: base64.RawURLEncoding.EncodeToString(digest[:]),
		NodeID: "0123456789abcdef0123456789abcdef", ReportingKey: base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
		ReportInterval: 30,
	}
	if _, _, err := sendLightReport(context.Background(), config, make([]byte, 32), reportRequest{}, nil); err != nil {
		t.Fatalf("sendLightReport() did not use the stored pin: %v", err)
	}
	select {
	case <-received:
	default:
		t.Fatal("pinned panel did not receive the report")
	}
}
