package panel

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Self-signed HTTPS is a fallback for direct IP+port installations that have
// no domain. It encrypts the browser session and restores secure-context
// browser APIs, but it never establishes trust: the administrator verifies the
// fingerprint once. Plain HTTP stays available on the same port so healthchecks,
// trusted reverse proxies and Noise-encrypted cluster peers keep working.
const (
	selfSignedTLSDirName   = "self-signed-tls"
	selfSignedTLSCertFile  = "cert.pem"
	selfSignedTLSKeyFile   = "key.pem"
	selfSignedTLSEnabled   = "enabled"
	selfSignedTLSValidity  = 825 * 24 * time.Hour
	selfSignedTLSRenewal   = 30 * 24 * time.Hour
	selfSignedTLSMaxHosts  = 16
	selfSignedTLSSniffWait = 10 * time.Second
)

var errSelfSignedTLSInvalidHost = errors.New("self-signed TLS hosts must be IP literals or localhost")

type selfSignedTLSInfo struct {
	Enabled     bool       `json:"enabled"`
	Fingerprint string     `json:"fingerprintSha256,omitempty"`
	NotBefore   *time.Time `json:"notBefore,omitempty"`
	NotAfter    *time.Time `json:"notAfter,omitempty"`
	Hosts       []string   `json:"hosts,omitempty"`
}

type selfSignedTLSState struct {
	config    *tls.Config
	info      selfSignedTLSInfo
	loadError string
}

type selfSignedTLS struct {
	dir   string
	mu    sync.Mutex
	state atomic.Pointer[selfSignedTLSState]
}

func selfSignedTLSDir(dataDir string) string {
	return filepath.Join(dataDir, selfSignedTLSDirName)
}

// openSelfSignedTLS loads persisted state. A damaged certificate never blocks
// startup: the panel falls back to plain HTTP and reports the fallback.
func openSelfSignedTLS(dir string) (*selfSignedTLS, error) {
	manager := &selfSignedTLS{dir: dir}
	manager.state.Store(&selfSignedTLSState{})
	certificate, leaf, err := manager.readPair()
	if errors.Is(err, os.ErrNotExist) {
		return manager, nil
	}
	if err != nil {
		manager.state.Store(&selfSignedTLSState{loadError: "certificate_unreadable"})
		return manager, err
	}
	info := certificateInfo(leaf)
	state := &selfSignedTLSState{info: info}
	if manager.enabledMarker() {
		state.config = selfSignedTLSConfig(certificate)
		state.info.Enabled = true
	}
	manager.state.Store(state)
	return manager, nil
}

func (m *selfSignedTLS) Info() selfSignedTLSInfo {
	if m == nil {
		return selfSignedTLSInfo{}
	}
	info := m.state.Load().info
	info.Hosts = append([]string(nil), info.Hosts...)
	return info
}

// LoadError reports why a persisted certificate could not be served at startup.
func (m *selfSignedTLS) LoadError() string {
	if m == nil {
		return ""
	}
	return m.state.Load().loadError
}

func (m *selfSignedTLS) TLSConfig() *tls.Config {
	if m == nil {
		return nil
	}
	return m.state.Load().config
}

// Enable activates TLS on the shared listener. The existing key pair is kept
// so the fingerprint the administrator verified stays stable; a new pair is
// issued only on request, when missing, near expiry, or when the current IP
// host is not covered.
func (m *selfSignedTLS) Enable(hosts []string, regenerate bool) (selfSignedTLSInfo, error) {
	normalized, err := normalizeSelfSignedTLSHosts(hosts)
	if err != nil {
		return selfSignedTLSInfo{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	certificate, leaf, err := m.readPair()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		regenerate = true
	}
	if err != nil || regenerate || time.Until(leaf.NotAfter) < selfSignedTLSRenewal || !certificateCovers(leaf, normalized) {
		if leaf != nil && !regenerate {
			normalized = mergeSelfSignedTLSHosts(certificateHosts(leaf), normalized)
		}
		certificate, leaf, err = m.issue(normalized)
		if err != nil {
			return selfSignedTLSInfo{}, err
		}
	}
	if err := writeFileAtomic(filepath.Join(m.dir, selfSignedTLSEnabled), nil, 0o600); err != nil {
		return selfSignedTLSInfo{}, err
	}
	info := certificateInfo(leaf)
	info.Enabled = true
	m.state.Store(&selfSignedTLSState{config: selfSignedTLSConfig(certificate), info: info})
	return m.Info(), nil
}

// Disable stops new TLS handshakes. The key pair is retained so re-enabling
// keeps the verified fingerprint.
func (m *selfSignedTLS) Disable() (selfSignedTLSInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := disableSelfSignedTLSMarker(m.dir); err != nil {
		return selfSignedTLSInfo{}, err
	}
	info := m.state.Load().info
	info.Enabled = false
	m.state.Store(&selfSignedTLSState{info: info})
	return m.Info(), nil
}

func (m *selfSignedTLS) CertificatePEM() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, leaf, err := m.readPair()
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw}), nil
}

// DisableSelfSignedTLS is the offline recovery path used by
// `paneld self-signed-tls disable`; the panel must be restarted afterwards.
func DisableSelfSignedTLS(config Config) error {
	return disableSelfSignedTLSMarker(selfSignedTLSDir(config.DataDir))
}

func disableSelfSignedTLSMarker(dir string) error {
	err := os.Remove(filepath.Join(dir, selfSignedTLSEnabled))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (m *selfSignedTLS) enabledMarker() bool {
	info, err := os.Lstat(filepath.Join(m.dir, selfSignedTLSEnabled))
	return err == nil && info.Mode().IsRegular()
}

func (m *selfSignedTLS) readPair() (tls.Certificate, *x509.Certificate, error) {
	certPEM, err := readRegularFile(filepath.Join(m.dir, selfSignedTLSCertFile), 64<<10)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	keyPEM, err := readRegularFile(filepath.Join(m.dir, selfSignedTLSKeyFile), 64<<10)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("load self-signed TLS pair: %w", err)
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("parse self-signed TLS certificate: %w", err)
	}
	certificate.Leaf = leaf
	return certificate, leaf, nil
}

func (m *selfSignedTLS) issue(hosts []string) (tls.Certificate, *x509.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "KPanel self-signed", Organization: []string{"KPanel"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(selfSignedTLSValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return tls.Certificate{}, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := writeFileAtomic(filepath.Join(m.dir, selfSignedTLSKeyFile), keyPEM, 0o600); err != nil {
		return tls.Certificate{}, nil, err
	}
	if err := writeFileAtomic(filepath.Join(m.dir, selfSignedTLSCertFile), certPEM, 0o644); err != nil {
		return tls.Certificate{}, nil, err
	}
	return m.readPair()
}

func selfSignedTLSConfig(certificate tls.Certificate) *tls.Config {
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
		// Terminal and job streams hijack the connection, which HTTP/2 cannot.
		NextProtos: []string{"http/1.1"},
	}
}

func certificateInfo(leaf *x509.Certificate) selfSignedTLSInfo {
	if leaf == nil {
		return selfSignedTLSInfo{}
	}
	sum := sha256.Sum256(leaf.Raw)
	encoded := strings.ToUpper(hex.EncodeToString(sum[:]))
	parts := make([]string, 0, len(sum))
	for index := 0; index < len(encoded); index += 2 {
		parts = append(parts, encoded[index:index+2])
	}
	notBefore, notAfter := leaf.NotBefore.UTC(), leaf.NotAfter.UTC()
	return selfSignedTLSInfo{
		Fingerprint: strings.Join(parts, ":"),
		NotBefore:   &notBefore,
		NotAfter:    &notAfter,
		Hosts:       certificateHosts(leaf),
	}
}

func certificateHosts(leaf *x509.Certificate) []string {
	hosts := make([]string, 0, len(leaf.IPAddresses)+len(leaf.DNSNames))
	for _, ip := range leaf.IPAddresses {
		hosts = append(hosts, ip.String())
	}
	return append(hosts, leaf.DNSNames...)
}

func certificateCovers(leaf *x509.Certificate, hosts []string) bool {
	for _, host := range hosts {
		if leaf.VerifyHostname(host) != nil {
			return false
		}
	}
	return true
}

func normalizeSelfSignedTLSHosts(hosts []string) ([]string, error) {
	return mergeSelfSignedTLSHosts([]string{"127.0.0.1", "::1", "localhost"}, hosts), validateSelfSignedTLSHosts(hosts)
}

func validateSelfSignedTLSHosts(hosts []string) error {
	for _, host := range hosts {
		if host != "localhost" && net.ParseIP(host) == nil {
			return errSelfSignedTLSInvalidHost
		}
	}
	return nil
}

func mergeSelfSignedTLSHosts(base, extra []string) []string {
	seen := map[string]bool{}
	merged := make([]string, 0, len(base)+len(extra))
	for _, host := range append(append([]string(nil), base...), extra...) {
		if ip := net.ParseIP(host); ip != nil {
			host = ip.String()
		}
		if host == "" || seen[host] || len(merged) >= selfSignedTLSMaxHosts {
			continue
		}
		seen[host] = true
		merged = append(merged, host)
	}
	return merged
}

func readRegularFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("%s is not a regular file within %d bytes", filepath.Base(path), limit)
	}
	return os.ReadFile(path)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// selfSignedTLSListener serves TLS and plain HTTP on one port. Each accepted
// connection is classified in its own goroutine by the first byte (0x16 is a
// TLS handshake record), so a slow client cannot stall Accept.
type selfSignedTLSListener struct {
	net.Listener
	config    func() *tls.Config
	conns     chan net.Conn
	errs      chan error
	done      chan struct{}
	closeOnce sync.Once
}

func newSelfSignedTLSListener(inner net.Listener, config func() *tls.Config) *selfSignedTLSListener {
	listener := &selfSignedTLSListener{
		Listener: inner,
		config:   config,
		conns:    make(chan net.Conn),
		errs:     make(chan error),
		done:     make(chan struct{}),
	}
	go listener.acceptLoop()
	return listener
}

func (l *selfSignedTLSListener) acceptLoop() {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			select {
			case l.errs <- err:
			case <-l.done:
				return
			}
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		go l.classify(conn)
	}
}

func (l *selfSignedTLSListener) classify(conn net.Conn) {
	_ = conn.SetReadDeadline(time.Now().Add(selfSignedTLSSniffWait))
	reader := bufio.NewReaderSize(conn, 1024)
	first, err := reader.Peek(1)
	if err != nil {
		conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	var classified net.Conn = &peekedConn{Conn: conn, reader: reader}
	if first[0] == 0x16 {
		config := l.config()
		if config == nil {
			conn.Close()
			return
		}
		classified = tls.Server(classified, config)
	}
	select {
	case l.conns <- classified:
	case <-l.done:
		conn.Close()
	}
}

func (l *selfSignedTLSListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case err := <-l.errs:
		return nil, err
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *selfSignedTLSListener) Close() error {
	l.closeOnce.Do(func() { close(l.done) })
	return l.Listener.Close()
}

type peekedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *peekedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

// WrapListener lets paneld share its listener between plain HTTP and the
// optional self-signed TLS fallback.
func (s *Server) WrapListener(inner net.Listener) net.Listener {
	if s.selfTLS == nil {
		return inner
	}
	return newSelfSignedTLSListener(inner, s.selfTLS.TLSConfig)
}

// selfSignedTLSRedirect upgrades top-level browser navigations on a direct IP
// host once self-signed HTTPS is enabled. API calls, public share pages,
// trusted reverse proxies and loopback healthchecks keep plain HTTP.
func (s *Server) selfSignedTLSRedirect(w http.ResponseWriter, r *http.Request) bool {
	if r.TLS != nil || !s.config.AllowIPHosts || s.selfTLS.TLSConfig() == nil ||
		(r.Method != http.MethodGet && r.Method != http.MethodHead) || len(r.Header.Values("Upgrade")) != 0 {
		return false
	}
	if mode := r.Header.Get("Sec-Fetch-Mode"); mode != "" && mode != "navigate" {
		return false
	} else if mode == "" && !strings.Contains(r.Header.Get("Accept"), "text/html") {
		return false
	}
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/") || path == "/mcp" || path == "/f" ||
		strings.HasPrefix(path, fileShareRawPrefix) || strings.HasPrefix(path, clusterSharePagePrefix) {
		return false
	}
	if s.isTrustedProxyPeer(r) {
		return false
	}
	if _, ok := directIPOrigin(r); !ok {
		return false
	}
	target := "https://" + r.Host + r.URL.RequestURI()
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	return true
}
