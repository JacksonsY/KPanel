package panel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

const (
	panelAccessPath               = "/api/v1/settings/panel-access"
	panelAccessSelfSignedTLSPath  = panelAccessPath + "/self-signed-tls"
	panelAccessCertificatePath    = panelAccessSelfSignedTLSPath + "/certificate"
	panelAccessDomainCheckPath    = panelAccessPath + "/domain-check"
	panelAccessDomainCheckTimeout = 10 * time.Second
	// The probe is public but only answers for a nonce this process is
	// waiting on, so a match proves the domain reaches this very panel
	// through normal Host validation, not merely some KPanel.
	panelAccessProbePrefix = "/api/v1/panel-access/probe/"
	panelAccessProbeTTL    = 30 * time.Second
	panelAccessProbeLimit  = 16
	panelAccessProofHeader = "X-KPanel-Domain-Proof"
)

type panelDomainProbe struct {
	expires time.Time
	proof   string
}

type panelAccessResponse struct {
	AllowIPHosts      bool              `json:"allowIpHosts"`
	PublicURL         string            `json:"publicUrl,omitempty"`
	RequestTLS        bool              `json:"requestTls"`
	RequestProxyHTTPS bool              `json:"requestProxyHttps"`
	RequestIPHost     bool              `json:"requestIpHost"`
	SelfSignedTLS     selfSignedTLSInfo `json:"selfSignedTls"`
	SelfSignedError   string            `json:"selfSignedError,omitempty"`
}

type panelDomainCheckResponse struct {
	Domain    string `json:"domain"`
	Reachable bool   `json:"reachable"`
	Matched   bool   `json:"matched"`
	Problem   string `json:"problem,omitempty"`
}

func (s *Server) handlePanelAccess(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == panelAccessPath && r.Method == http.MethodGet:
		if _, _, ok := s.requireSession(w, r); !ok {
			return
		}
		s.writeJSON(w, http.StatusOK, s.panelAccessStatus(r))
	case r.URL.Path == panelAccessSelfSignedTLSPath && r.Method == http.MethodPut:
		s.handleSelfSignedTLSUpdate(w, r)
	case r.URL.Path == panelAccessCertificatePath && r.Method == http.MethodGet:
		s.handleSelfSignedTLSCertificate(w, r)
	case r.URL.Path == panelAccessDomainCheckPath && r.Method == http.MethodPost:
		s.handlePanelDomainCheck(w, r)
	default:
		s.writeProblem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

func (s *Server) panelAccessStatus(r *http.Request) panelAccessResponse {
	_, proxyHTTPS := s.trustedProxyHTTPSOrigin(r)
	_, ipHost := directIPOrigin(r)
	response := panelAccessResponse{
		AllowIPHosts:      s.config.AllowIPHosts,
		PublicURL:         s.config.PublicURL,
		RequestTLS:        r.TLS != nil,
		RequestProxyHTTPS: proxyHTTPS,
		RequestIPHost:     ipHost,
		SelfSignedTLS:     s.selfTLS.Info(),
	}
	response.SelfSignedError = s.selfTLS.LoadError()
	return response
}

func (s *Server) handleSelfSignedTLSUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	_, session, ok := s.requireSession(w, r)
	if !ok || !s.checkCSRF(w, r, session) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var input struct {
		Enabled    bool `json:"enabled"`
		Regenerate bool `json:"regenerate,omitempty"`
	}
	if s.decodeJSON(w, r, &input) != nil {
		return
	}
	if s.selfTLS == nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "self_signed_tls_unavailable", "Self-signed HTTPS is unavailable", "")
		return
	}
	if input.Enabled && !s.config.AllowIPHosts {
		s.writeProblem(w, r, http.StatusConflict, "ip_hosts_disabled", "Self-signed HTTPS requires direct IP access", "")
		return
	}
	hosts := []string{}
	if origin, ok := directIPOrigin(r); ok {
		if parsed, err := url.Parse(origin); err == nil && parsed.Hostname() != "" {
			hosts = append(hosts, parsed.Hostname())
		}
	}
	change := map[string]any{"enabled": input.Enabled, "regenerate": input.Regenerate, "hosts": hosts}
	if err := s.audit(r, session.User.ID, "settings.self_signed_tls.update", "panel", "self-signed-tls", "intent", change); err != nil {
		s.writeProblem(w, r, http.StatusServiceUnavailable, "audit_unavailable", "Audit storage unavailable", "")
		return
	}
	var err error
	if input.Enabled {
		err = s.auth.ChangeTransportBoundary(func() error {
			if err := s.selfTLS.markSessionsSecured(); err != nil {
				return err
			}
			_, enableErr := s.selfTLS.Enable(hosts, input.Regenerate)
			return enableErr
		})
	} else {
		err = s.auth.ChangeTransportBoundary(func() error {
			_, disableErr := s.selfTLS.Disable()
			return disableErr
		})
	}
	if err != nil {
		_ = s.audit(r, session.User.ID, "settings.self_signed_tls.update", "panel", "self-signed-tls", "failure", change)
		slog.Error("self-signed TLS update failed", "error", err)
		s.writeProblem(w, r, http.StatusInternalServerError, "self_signed_tls_update_failed", "Self-signed HTTPS update failed", "")
		return
	}
	change["fingerprintSha256"] = s.selfTLS.Info().Fingerprint
	s.clearAuthCookies(w, r)
	_ = s.audit(r, session.User.ID, "settings.self_signed_tls.update", "panel", "self-signed-tls", "success", change)
	s.writeJSON(w, http.StatusOK, s.panelAccessStatus(r))
}

func (s *Server) handleSelfSignedTLSCertificate(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireSession(w, r); !ok {
		return
	}
	if s.selfTLS == nil {
		http.NotFound(w, r)
		return
	}
	certificate, err := s.selfTLS.CertificatePEM()
	if err != nil {
		s.writeProblem(w, r, http.StatusNotFound, "self_signed_tls_missing", "No self-signed certificate has been issued", "")
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", `attachment; filename="kpanel-self-signed.crt"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(certificate)
}

// handlePanelDomainCheck verifies from the server side that a domain already
// serves this panel over publicly trusted HTTPS. The browser cannot do this
// itself because the panel CSP limits connect-src to 'self'. The request is
// fixed to a pending nonce URL, follows no redirects, refuses loopback/link-local
// targets and requires an independent response proof held by this process.
func (s *Server) handlePanelDomainCheck(w http.ResponseWriter, r *http.Request) {
	if !s.checkOrigin(w, r) {
		return
	}
	_, session, ok := s.requireSession(w, r)
	if !ok || !s.checkCSRF(w, r, session) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var input struct {
		Domain string `json:"domain"`
	}
	if s.decodeJSON(w, r, &input) != nil {
		return
	}
	domain, valid := normalizePanelSiteDomain(strings.TrimSpace(input.Domain))
	if !valid {
		s.writeProblem(w, r, http.StatusBadRequest, "invalid_domain", "Invalid domain", "")
		return
	}
	nonce, proof, err := s.addDomainProbe()
	if err != nil {
		s.writeProblem(w, r, http.StatusTooManyRequests, "domain_check_busy", "Too many domain checks in progress", "")
		return
	}
	defer s.removeDomainProbe(nonce)
	ctx, cancel := context.WithTimeout(r.Context(), panelAccessDomainCheckTimeout)
	defer cancel()
	s.writeJSON(w, http.StatusOK, checkPanelDomain(ctx, domain, nonce, proof, s.panelDomainCheckClient()))
}

func (s *Server) addDomainProbe() (string, string, error) {
	buffer := make([]byte, 64)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", err
	}
	nonce := hex.EncodeToString(buffer[:32])
	proof := hex.EncodeToString(buffer[32:])
	now := time.Now()
	s.domainProbeMu.Lock()
	defer s.domainProbeMu.Unlock()
	if s.domainProbes == nil {
		s.domainProbes = map[string]panelDomainProbe{}
	}
	for pending, probe := range s.domainProbes {
		if now.After(probe.expires) {
			delete(s.domainProbes, pending)
		}
	}
	if len(s.domainProbes) >= panelAccessProbeLimit {
		return "", "", errors.New("too many pending domain probes")
	}
	s.domainProbes[nonce] = panelDomainProbe{expires: now.Add(panelAccessProbeTTL), proof: proof}
	return nonce, proof, nil
}

func (s *Server) removeDomainProbe(nonce string) {
	s.domainProbeMu.Lock()
	defer s.domainProbeMu.Unlock()
	delete(s.domainProbes, nonce)
}

// handlePanelDomainProbe answers 204 only for a nonce this process issued.
// It is deliberately not exempt from Host validation, so a domain the panel
// would reject at login also fails the switch check.
func (s *Server) handlePanelDomainProbe(w http.ResponseWriter, r *http.Request) {
	nonce := strings.TrimPrefix(r.URL.Path, panelAccessProbePrefix)
	if r.Method != http.MethodGet || len(nonce) != 64 || strings.Trim(nonce, "0123456789abcdef") != "" {
		http.NotFound(w, r)
		return
	}
	s.domainProbeMu.Lock()
	probe, ok := s.domainProbes[nonce]
	s.domainProbeMu.Unlock()
	if !ok || time.Now().After(probe.expires) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set(panelAccessProofHeader, probe.proof)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) panelDomainCheckClient() *http.Client {
	if s.domainCheckClient != nil {
		return s.domainCheckClient
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: refuseInternalDial}
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            dialer.DialContext,
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  5 * time.Second,
		DisableKeepAlives:      true,
		MaxResponseHeaderBytes: 16 << 10,
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func refuseInternalDial(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return errors.New("refusing internal domain check target")
	}
	return nil
}

func checkPanelDomain(ctx context.Context, domain, nonce, proof string, client *http.Client) panelDomainCheckResponse {
	result := panelDomainCheckResponse{Domain: domain}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+domain+panelAccessProbePrefix+nonce, nil)
	if err != nil {
		result.Problem = "request_invalid"
		return result
	}
	response, err := client.Do(request)
	if err != nil {
		result.Problem = classifyDomainCheckError(err)
		return result
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 16<<10))
	response.Body.Close()
	result.Reachable = true
	switch response.StatusCode {
	case http.StatusNoContent:
		if proof != "" && response.Header.Get(panelAccessProofHeader) == proof {
			result.Matched = true
		} else {
			result.Problem = "not_this_panel"
		}
	case http.StatusMisdirectedRequest:
		result.Problem = "host_rejected"
	case http.StatusNotFound:
		result.Problem = "not_this_panel"
	default:
		result.Problem = "unexpected_status"
	}
	return result
}

func classifyDomainCheckError(err error) string {
	var dnsError *net.DNSError
	message := err.Error()
	switch {
	case errors.As(err, &dnsError):
		return "dns_unresolved"
	case strings.Contains(message, "refusing internal"):
		return "internal_target"
	case strings.Contains(message, "x509:") || strings.Contains(message, "tls:"):
		return "certificate_invalid"
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(message, "timeout"):
		return "timeout"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection_refused"
	default:
		return "unreachable"
	}
}
