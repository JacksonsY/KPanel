package systeminfo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

const (
	ipInfoIPv4URL = "https://ipinfo.io/json"
	ipInfoIPv6URL = "https://v6.ipinfo.io/json"
)

var (
	ipInfoIPv4Client = newIPInfoClient("tcp4")
	ipInfoIPv6Client = newIPInfoClient("tcp6")
)

type ipInfoResponse struct {
	IP          string `json:"ip"`
	City        string `json:"city"`
	Region      string `json:"region"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Org         string `json:"org"`
	Timezone    string `json:"timezone"`
	ASN         struct {
		Name string `json:"name"`
	} `json:"asn"`
}

type ipInfoResult struct {
	family int
	info   ipInfoResponse
	err    error
}

func (c *Collector) readPublicNetwork(ctx context.Context) contract.PublicNetworkSummary {
	now := c.Now().UTC()
	c.publicNetworkMu.Lock()
	cached := c.publicNetworkCache
	if !c.publicNetworkExpires.IsZero() && now.Before(c.publicNetworkExpires) {
		c.publicNetworkMu.Unlock()
		return cached
	}
	if c.PublicNetworkLookup == nil || c.publicNetworkLoading || ctx.Err() != nil {
		c.publicNetworkMu.Unlock()
		return cached
	}
	c.publicNetworkLoading = true
	c.publicNetworkDone = make(chan struct{})
	c.publicNetworkMu.Unlock()

	// Public IP and carrier metadata are informational and may take seconds on
	// hosts with an unavailable address family. Never make the host summary
	// wait for that external service: return the last snapshot immediately and
	// refresh it once in the background.
	go c.refreshPublicNetwork()
	return cached
}

// PublicNetwork returns the cached public-network identity, waiting only for
// the dedicated background refresh when the snapshot is cold or expired. It
// is intentionally separate from Collect so a slow external lookup can never
// delay local host metrics.
func (c *Collector) PublicNetwork(ctx context.Context) contract.PublicNetworkSummary {
	cached := c.readPublicNetwork(ctx)
	c.publicNetworkMu.Lock()
	loading := c.publicNetworkLoading
	done := c.publicNetworkDone
	c.publicNetworkMu.Unlock()
	if !loading || done == nil {
		return cached
	}
	select {
	case <-ctx.Done():
		return cached
	case <-done:
		c.publicNetworkMu.Lock()
		refreshed := c.publicNetworkCache
		c.publicNetworkMu.Unlock()
		return refreshed
	}
}

// CachedPublicNetwork returns the last completed public-network snapshot
// without starting or waiting for an external lookup. Callers that only need
// an optional locality hint must use this path so normal operations never
// acquire a new network dependency.
func (c *Collector) CachedPublicNetwork() contract.PublicNetworkSummary {
	c.publicNetworkMu.Lock()
	defer c.publicNetworkMu.Unlock()
	return c.publicNetworkCache
}

func (c *Collector) refreshPublicNetwork() {
	defer func() {
		c.publicNetworkMu.Lock()
		c.publicNetworkLoading = false
		if c.publicNetworkDone != nil {
			close(c.publicNetworkDone)
			c.publicNetworkDone = nil
		}
		c.publicNetworkMu.Unlock()
	}()
	// The caller was checked before reserving the worker. Its completion must
	// not cancel this independently bounded, shared background refresh.
	lookupContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := c.PublicNetworkLookup(lookupContext)
	now := c.Now().UTC()
	if err != nil || (result.IPv4 == "" && result.IPv6 == "") {
		retryAfter := min(c.PublicNetworkCacheTTL, 5*time.Minute)
		c.publicNetworkMu.Lock()
		c.publicNetworkExpires = now.Add(retryAfter)
		c.publicNetworkMu.Unlock()
		return
	}
	if result.Source == "" {
		result.Source = "ipinfo.io"
	}
	result.UpdatedAt = &now

	c.publicNetworkMu.Lock()
	c.publicNetworkCache = result
	c.publicNetworkExpires = now.Add(c.PublicNetworkCacheTTL)
	c.publicNetworkMu.Unlock()
}

func lookupPublicNetwork(ctx context.Context) (contract.PublicNetworkSummary, error) {
	results := make(chan ipInfoResult, 2)
	go func() {
		info, err := queryIPInfo(ctx, ipInfoIPv4Client, ipInfoIPv4URL, 4)
		results <- ipInfoResult{family: 4, info: info, err: err}
	}()
	go func() {
		info, err := queryIPInfo(ctx, ipInfoIPv6Client, ipInfoIPv6URL, 6)
		results <- ipInfoResult{family: 6, info: info, err: err}
	}()

	var summary contract.PublicNetworkSummary
	var lookupErrors []error
	for range 2 {
		result := <-results
		if result.err != nil {
			lookupErrors = append(lookupErrors, result.err)
			continue
		}
		if result.family == 4 {
			summary.IPv4 = cleanPublicField(result.info.IP, 64)
		} else {
			summary.IPv6 = cleanPublicField(result.info.IP, 64)
		}
		mergeIPInfoMetadata(&summary, result.info)
	}
	if summary.IPv4 == "" && summary.IPv6 == "" {
		return contract.PublicNetworkSummary{}, errors.Join(lookupErrors...)
	}
	summary.Source = "ipinfo.io"
	return summary, nil
}

func queryIPInfo(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	family int,
) (ipInfoResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ipInfoResponse{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "KPanel host information")
	response, err := client.Do(request)
	if err != nil {
		return ipInfoResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return ipInfoResponse{}, fmt.Errorf("ipinfo returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	if err != nil {
		return ipInfoResponse{}, fmt.Errorf("read ipinfo response: %w", err)
	}
	if len(body) > 64<<10 {
		return ipInfoResponse{}, errors.New("ipinfo response exceeded 64 KiB")
	}
	var info ipInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return ipInfoResponse{}, fmt.Errorf("decode ipinfo response: %w", err)
	}
	ip := net.ParseIP(strings.TrimSpace(info.IP))
	if ip == nil || (family == 4 && ip.To4() == nil) || (family == 6 && (ip.To4() != nil || ip.To16() == nil)) {
		return ipInfoResponse{}, fmt.Errorf("ipinfo returned an invalid IPv%d address", family)
	}
	info.IP = ip.String()
	return info, nil
}

func mergeIPInfoMetadata(summary *contract.PublicNetworkSummary, info ipInfoResponse) {
	if summary.ISP == "" {
		summary.ISP = cleanPublicField(info.Org, 160)
		if summary.ISP == "" {
			summary.ISP = cleanPublicField(info.ASN.Name, 160)
		}
	}
	if summary.Country == "" {
		summary.Country = cleanPublicField(info.Country, 32)
		if summary.Country == "" {
			summary.Country = cleanPublicField(info.CountryCode, 32)
		}
	}
	if summary.CountryCode == "" {
		summary.CountryCode = strings.ToUpper(cleanPublicField(info.CountryCode, 2))
		if summary.CountryCode == "" && len(summary.Country) == 2 {
			summary.CountryCode = strings.ToUpper(summary.Country)
		}
	}
	if summary.Region == "" {
		summary.Region = cleanPublicField(info.Region, 96)
	}
	if summary.City == "" {
		summary.City = cleanPublicField(info.City, 96)
	}
	if summary.Timezone == "" {
		summary.Timezone = cleanPublicField(info.Timezone, 96)
	}
}

func newIPInfoClient(network string) *http.Client {
	dialer := &net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, _, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, address)
	}
	transport.ResponseHeaderTimeout = 2 * time.Second
	transport.TLSHandshakeTimeout = 2 * time.Second
	return &http.Client{
		Transport: transport,
		Timeout:   2500 * time.Millisecond,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 2 || !strings.EqualFold(request.URL.Hostname(), via[0].URL.Hostname()) {
				return errors.New("ipinfo redirect rejected")
			}
			return nil
		},
	}
}

func cleanPublicField(value string, limit int) string {
	value = strings.TrimSpace(strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return -1
		}
		return character
	}, value))
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}
