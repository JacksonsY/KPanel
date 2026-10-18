package panel

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSelfSignedTLSKeepsFingerprintAcrossToggleAndRestart(t *testing.T) {
	dataDir := t.TempDir()
	dir := selfSignedTLSDir(dataDir)
	manager, err := openSelfSignedTLS(dir)
	if err != nil {
		t.Fatal(err)
	}
	if manager.TLSConfig() != nil || manager.Info().Enabled {
		t.Fatal("fresh manager must start disabled")
	}
	enabled, err := manager.Enable([]string{"203.0.113.5"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled.Enabled || enabled.Fingerprint == "" || enabled.TLSPublicKeyPin == "" || manager.TLSConfig() == nil {
		t.Fatalf("enable did not activate TLS: %+v", enabled)
	}
	if !containsString(enabled.Hosts, "203.0.113.5") || !containsString(enabled.Hosts, "127.0.0.1") {
		t.Fatalf("certificate hosts missing request IP or loopback: %v", enabled.Hosts)
	}
	if _, err := manager.Disable(); err != nil {
		t.Fatal(err)
	}
	if manager.TLSConfig() != nil || manager.Info().Enabled {
		t.Fatal("disable must stop TLS")
	}
	reopened, err := openSelfSignedTLS(dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Info().Enabled || reopened.Info().Fingerprint != enabled.Fingerprint {
		t.Fatalf("disabled state or fingerprint not persisted: %+v", reopened.Info())
	}
	again, err := reopened.Enable([]string{"203.0.113.5"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if again.Fingerprint != enabled.Fingerprint {
		t.Fatal("re-enabling must keep the verified fingerprint")
	}
	restarted, err := openSelfSignedTLS(dir)
	if err != nil || !restarted.Info().Enabled || restarted.TLSConfig() == nil {
		t.Fatalf("enabled state not restored after restart: %v %+v", err, restarted.Info())
	}
	widened, err := restarted.Enable([]string{"198.51.100.7"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if widened.Fingerprint == enabled.Fingerprint || widened.TLSPublicKeyPin != enabled.TLSPublicKeyPin ||
		!containsString(widened.Hosts, "203.0.113.5") || !containsString(widened.Hosts, "198.51.100.7") {
		t.Fatalf("new IP host must reissue while keeping existing hosts: %+v", widened)
	}
	regenerated, err := restarted.Enable([]string{"198.51.100.7"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if regenerated.Fingerprint == widened.Fingerprint || regenerated.TLSPublicKeyPin == widened.TLSPublicKeyPin ||
		containsString(regenerated.Hosts, "203.0.113.5") {
		t.Fatalf("regenerate must issue a fresh pair for the current host only: %+v", regenerated)
	}
	if _, err := restarted.Enable([]string{"panel.example.com"}, false); err == nil {
		t.Fatal("DNS names other than localhost must be rejected")
	}
	if err := DisableSelfSignedTLS(Config{DataDir: dataDir}); err != nil {
		t.Fatal(err)
	}
	if recovered, err := openSelfSignedTLS(dir); err != nil || recovered.TLSConfig() != nil {
		t.Fatalf("offline recovery must disable TLS on next start: %v", err)
	}
}

func TestLightEnrollmentPinOnlyAppliesToDirectBuiltinSelfSignedOrigin(t *testing.T) {
	server, _ := newTestServer(t)
	info, err := server.selfTLS.Enable([]string{"203.0.113.5"}, false)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "https://203.0.113.5:8865/api/v1/cluster/light-enrollments", nil)
	request.TLS = &tls.ConnectionState{Version: tls.VersionTLS13}
	if pin := server.lightEnrollmentTLSPublicKeyPin(request, "https://203.0.113.5:8865"); pin != info.TLSPublicKeyPin {
		t.Fatalf("direct self-signed pin = %q, want %q", pin, info.TLSPublicKeyPin)
	}
	request.Host = "LOCALHOST:8865"
	if pin := server.lightEnrollmentTLSPublicKeyPin(request, "https://LOCALHOST:8865"); pin != info.TLSPublicKeyPin {
		t.Fatalf("case-insensitive localhost certificate pin = %q, want %q", pin, info.TLSPublicKeyPin)
	}
	request.Host = "203.0.113.5:8865"
	request.TLS = nil
	if pin := server.lightEnrollmentTLSPublicKeyPin(request, "https://203.0.113.5:8865"); pin != "" {
		t.Fatalf("plain HTTP request received a pin: %q", pin)
	}
	request.TLS = &tls.ConnectionState{Version: tls.VersionTLS13}
	if pin := server.lightEnrollmentTLSPublicKeyPin(request, "https://198.51.100.7:8865"); pin != "" {
		t.Fatalf("origin outside the built-in certificate SAN received a pin: %q", pin)
	}
	request.Host = "203.0.113.5:8865"
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Forwarded-Proto", "http")
	if pin := server.lightEnrollmentTLSPublicKeyPin(request, "https://203.0.113.5:8865"); pin != "" {
		t.Fatalf("trusted proxy request received a pin: %q", pin)
	}
}

func TestSelfSignedTLSListenerServesHTTPAndTLSOnOnePort(t *testing.T) {
	manager, err := openSelfSignedTLS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := newSelfSignedTLSListener(inner, manager.TLSConfig)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "tls=%t", r.TLS != nil)
	}), ReadHeaderTimeout: time.Second}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	address := inner.Addr().String()

	plain := fetchBody(t, &http.Client{}, "http://"+address+"/")
	if plain != "tls=false" {
		t.Fatalf("plain HTTP not served: %q", plain)
	}
	insecure := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	if _, err := insecure.Get("https://" + address + "/"); err == nil {
		t.Fatal("TLS must be refused while disabled")
	}
	info, err := manager.Enable([]string{"127.0.0.1"}, false)
	if err != nil {
		t.Fatal(err)
	}
	encrypted := fetchBody(t, insecure, "https://"+address+"/")
	if encrypted != "tls=true" {
		t.Fatalf("TLS not served after enable: %q", encrypted)
	}
	connection, err := tls.Dial("tcp", address, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	served := certificateInfo(connection.ConnectionState().PeerCertificates[0])
	connection.Close()
	if served.Fingerprint != info.Fingerprint {
		t.Fatal("served certificate does not match reported fingerprint")
	}
	if fetchBody(t, &http.Client{}, "http://"+address+"/") != "tls=false" {
		t.Fatal("plain HTTP must remain available for healthchecks and cluster peers")
	}
	idle, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer idle.Close()
	if fetchBody(t, &http.Client{}, "http://"+address+"/") != "tls=false" {
		t.Fatal("a silent connection must not block other clients")
	}
}

func TestSelfSignedTLSRedirectsOnlyDirectIPNavigations(t *testing.T) {
	server, _ := newTestServer(t)
	server.config.AllowIPHosts = true
	if _, err := server.selfTLS.Enable([]string{"203.0.113.5"}, false); err != nil {
		t.Fatal(err)
	}
	navigate := map[string]string{"Host": "203.0.113.5:8080", "Sec-Fetch-Mode": "navigate", "Accept": "text/html"}
	response := performRequest(server, http.MethodGet, "/files?path=%2Fhome", nil, navigate)
	if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != "https://203.0.113.5:8080/files?path=%2Fhome" {
		t.Fatalf("direct IP navigation not upgraded: %d %q", response.Code, response.Header().Get("Location"))
	}
	for name, path := range map[string]string{"api": "/api/v1/health", "share": "/share/file/abc", "raw": "/f/abc"} {
		if response := performRequest(server, http.MethodGet, path, nil, navigate); response.Code == http.StatusTemporaryRedirect {
			t.Fatalf("%s path must not be redirected", name)
		}
	}
	fetch := map[string]string{"Host": "203.0.113.5:8080", "Sec-Fetch-Mode": "cors"}
	if response := performRequest(server, http.MethodGet, "/", nil, fetch); response.Code == http.StatusTemporaryRedirect {
		t.Fatal("non-navigation fetches must not be redirected")
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Host = "203.0.113.5:8080"
	request.RemoteAddr = "127.0.0.1:40000"
	request.Header.Set("Sec-Fetch-Mode", "navigate")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code == http.StatusTemporaryRedirect {
		t.Fatal("trusted proxy or loopback peers must keep plain HTTP")
	}
	domain := map[string]string{"Host": "panel.test", "Sec-Fetch-Mode": "navigate"}
	if response := performRequest(server, http.MethodGet, "/", nil, domain); response.Code == http.StatusTemporaryRedirect {
		t.Fatal("domain hosts must not be redirected to the self-signed listener")
	}
	if _, err := server.selfTLS.Disable(); err != nil {
		t.Fatal(err)
	}
	if response := performRequest(server, http.MethodGet, "/", nil, navigate); response.Code == http.StatusTemporaryRedirect {
		t.Fatal("redirect must stop once self-signed HTTPS is disabled")
	}
}

func TestPanelAccessSelfSignedTLSSettings(t *testing.T) {
	server, tokenPath := newTestServer(t)
	sessionCookie, csrfCookie := bootstrapCookies(t, server, tokenPath)
	headers := map[string]string{"Content-Type": "application/json", "Origin": "http://panel.test", "X-CSRF-Token": csrfCookie.Value}

	status := authenticatedRequest(server, http.MethodGet, panelAccessPath, nil, sessionCookie, csrfCookie, nil)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"enabled":false`) {
		t.Fatalf("status failed: %d %s", status.Code, status.Body.String())
	}
	body := []byte(`{"enabled":true}`)
	refused := authenticatedRequest(server, http.MethodPut, panelAccessSelfSignedTLSPath, body, sessionCookie, csrfCookie, headers)
	if refused.Code != http.StatusConflict || !strings.Contains(refused.Body.String(), "ip_hosts_disabled") {
		t.Fatalf("domain-only installs must refuse self-signed HTTPS: %d %s", refused.Code, refused.Body.String())
	}
	server.config.AllowIPHosts = true
	noCSRF := authenticatedRequest(server, http.MethodPut, panelAccessSelfSignedTLSPath, body, sessionCookie, csrfCookie,
		map[string]string{"Content-Type": "application/json", "Origin": "http://panel.test"})
	if noCSRF.Code == http.StatusOK {
		t.Fatal("update must require CSRF")
	}
	updated := authenticatedRequest(server, http.MethodPut, panelAccessSelfSignedTLSPath, body, sessionCookie, csrfCookie, headers)
	if updated.Code != http.StatusOK {
		t.Fatalf("enable failed: %d %s", updated.Code, updated.Body.String())
	}
	var response panelAccessResponse
	if err := json.Unmarshal(updated.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.SelfSignedTLS.Enabled || response.SelfSignedTLS.Fingerprint == "" {
		t.Fatalf("enable response incomplete: %+v", response)
	}
	certificate := authenticatedRequest(server, http.MethodGet, panelAccessCertificatePath, nil, sessionCookie, csrfCookie, nil)
	if certificate.Code != http.StatusOK || !bytes.HasPrefix(certificate.Body.Bytes(), []byte("-----BEGIN CERTIFICATE-----")) ||
		strings.Contains(certificate.Body.String(), "PRIVATE KEY") {
		t.Fatalf("certificate download wrong: %d", certificate.Code)
	}
	anonymous := performRequest(server, http.MethodGet, panelAccessCertificatePath, nil, nil)
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("certificate download must require a session: %d", anonymous.Code)
	}
	disabled := authenticatedRequest(server, http.MethodPut, panelAccessSelfSignedTLSPath, []byte(`{"enabled":false}`), sessionCookie, csrfCookie, headers)
	if disabled.Code != http.StatusOK || strings.Contains(disabled.Body.String(), `"enabled":true`) {
		t.Fatalf("disable failed: %d %s", disabled.Code, disabled.Body.String())
	}
	if files, err := readPanelBackupFiles(server.config.DataDir); err != nil {
		t.Fatal(err)
	} else {
		for name := range files {
			if strings.HasPrefix(name, selfSignedTLSDirName) {
				t.Fatalf("self-signed private key must not enter backups: %s", name)
			}
		}
	}
}

func TestPanelDomainCheckRequiresThisInstanceOverTrustedHTTPS(t *testing.T) {
	answer := http.StatusNoContent
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != panelAccessProbePrefix+"expected-nonce" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set(panelAccessProofHeader, "expected-proof")
		w.WriteHeader(answer)
	}))
	defer upstream.Close()
	client := upstream.Client()
	transport := client.Transport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, upstream.Listener.Addr().String())
	}
	client.Transport = transport

	if result := checkPanelDomain(context.Background(), "example.com", "expected-nonce", "expected-proof", client); !result.Matched || result.Problem != "" {
		t.Fatalf("matching panel not recognized: %+v", result)
	}
	for _, proof := range []string{"", "wrong-proof", "expected-nonce"} {
		if result := checkPanelDomain(context.Background(), "example.com", "expected-nonce", proof, client); result.Matched || result.Problem != "not_this_panel" {
			t.Fatalf("missing or wrong independent proof must fail: %+v", result)
		}
	}
	if result := checkPanelDomain(context.Background(), "example.com", "other-nonce", "expected-proof", client); result.Matched || result.Problem != "not_this_panel" {
		t.Fatalf("another panel must not match: %+v", result)
	}
	answer = http.StatusMisdirectedRequest
	if result := checkPanelDomain(context.Background(), "example.com", "expected-nonce", "expected-proof", client); result.Matched || result.Problem != "host_rejected" {
		t.Fatalf("host-rejected domain must not match: %+v", result)
	}
	untrusted := &http.Client{Transport: &http.Transport{DialContext: transport.DialContext}}
	if result := checkPanelDomain(context.Background(), "example.com", "expected-nonce", "expected-proof", untrusted); result.Matched || result.Problem != "certificate_invalid" {
		t.Fatalf("untrusted certificate must fail: %+v", result)
	}
	if err := refuseInternalDial("tcp", "127.0.0.1:443", nil); err == nil {
		t.Fatal("loopback targets must be refused")
	}
	if err := refuseInternalDial("tcp", "169.254.169.254:443", nil); err == nil {
		t.Fatal("metadata targets must be refused")
	}
	if err := refuseInternalDial("tcp", "203.0.113.9:443", nil); err != nil {
		t.Fatalf("public targets must be allowed: %v", err)
	}
}

func TestPanelDomainCheckRequiresValidDomainAndCSRF(t *testing.T) {
	server, tokenPath := newTestServer(t)
	sessionCookie, csrfCookie := bootstrapCookies(t, server, tokenPath)
	headers := map[string]string{"Content-Type": "application/json", "Origin": "http://panel.test", "X-CSRF-Token": csrfCookie.Value}
	invalid := authenticatedRequest(server, http.MethodPost, panelAccessDomainCheckPath, []byte(`{"domain":"203.0.113.5"}`), sessionCookie, csrfCookie, headers)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("IP literal must be rejected as a domain: %d", invalid.Code)
	}
	noCSRF := authenticatedRequest(server, http.MethodPost, panelAccessDomainCheckPath, []byte(`{"domain":"panel.example.com"}`), sessionCookie, csrfCookie,
		map[string]string{"Content-Type": "application/json", "Origin": "http://panel.test"})
	if noCSRF.Code == http.StatusOK {
		t.Fatal("domain check must require CSRF")
	}
}

func TestPanelDomainCheckRejectsUnrelatedNoContentServer(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	result := checkPanelDomain(context.Background(), strings.TrimPrefix(upstream.URL, "https://"), "nonce", "independent-proof", upstream.Client())
	if !result.Reachable || result.Matched || result.Problem != "not_this_panel" {
		t.Fatalf("generic 204 response must not prove panel identity: %+v", result)
	}
}

func TestPanelDomainProbeAnswersOnlyPendingNoncesThroughHostValidation(t *testing.T) {
	server, _ := newTestServer(t)
	nonce, proof, err := server.addDomainProbe()
	if err != nil {
		t.Fatal(err)
	}
	if response := performRequest(server, http.MethodGet, panelAccessProbePrefix+nonce, nil, nil); response.Code != http.StatusNoContent {
		t.Fatalf("pending nonce must answer 204: %d", response.Code)
	} else if len(proof) != 64 || proof == nonce || response.Header().Get(panelAccessProofHeader) != proof || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("probe must return an independent uncached proof")
	}
	if response := performRequest(server, http.MethodGet, panelAccessProbePrefix+nonce, nil, map[string]string{"Host": "other.example.com"}); response.Code != http.StatusMisdirectedRequest {
		t.Fatalf("probe must stay behind Host validation: %d", response.Code)
	}
	unknown := strings.Repeat("a", 64)
	if response := performRequest(server, http.MethodGet, panelAccessProbePrefix+unknown, nil, nil); response.Code != http.StatusNotFound {
		t.Fatalf("unknown nonce must be 404: %d", response.Code)
	}
	server.removeDomainProbe(nonce)
	if response := performRequest(server, http.MethodGet, panelAccessProbePrefix+nonce, nil, nil); response.Code != http.StatusNotFound {
		t.Fatalf("finished check must stop answering: %d", response.Code)
	}
	for index := 0; index < panelAccessProbeLimit; index++ {
		if _, _, err := server.addDomainProbe(); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := server.addDomainProbe(); err == nil {
		t.Fatal("pending probes must be bounded")
	}
}

func TestPanelDomainProbeWorksBehindSecurityEntrance(t *testing.T) {
	server, tokenPath := newTestServer(t)
	sessionCookie, csrfCookie := bootstrapCookies(t, server, tokenPath)
	_, resourceVersion := server.store.SecurityEntrance()
	body, err := json.Marshal(map[string]any{"enabled": true, "path": "panel-secure1", "expectedResourceVersion": resourceVersion})
	if err != nil {
		t.Fatal(err)
	}
	updated := authenticatedRequest(server, http.MethodPut, "/api/v1/settings/security-entry", body, sessionCookie, csrfCookie,
		map[string]string{"Content-Type": "application/json", "Origin": "http://panel.test", "X-CSRF-Token": csrfCookie.Value})
	if updated.Code != http.StatusOK {
		t.Fatalf("enable security entry failed: %d %s", updated.Code, updated.Body.String())
	}
	nonce, _, err := server.addDomainProbe()
	if err != nil {
		t.Fatal(err)
	}
	if response := performRequest(server, http.MethodGet, panelAccessProbePrefix+nonce, nil, nil); response.Code != http.StatusNoContent {
		t.Fatalf("hidden login must not hide the domain probe: %d", response.Code)
	}
	if response := performRequest(server, http.MethodGet, "/api/v1/settings/panel-access", nil, nil); response.Code == http.StatusOK {
		t.Fatal("settings must stay behind the security entrance")
	}
}

func fetchBody(t *testing.T, client *http.Client, target string) string {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
