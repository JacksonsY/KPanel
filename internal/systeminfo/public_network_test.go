package systeminfo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

func TestReadPublicNetworkCachesSuccessfulLookup(t *testing.T) {
	now := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	calls := 0
	lookedUp := make(chan struct{}, 2)
	collector := &Collector{
		Now:                   func() time.Time { return now },
		PublicNetworkCacheTTL: time.Hour,
		PublicNetworkLookup: func(context.Context) (contract.PublicNetworkSummary, error) {
			calls++
			lookedUp <- struct{}{}
			return contract.PublicNetworkSummary{
				IPv4: "203.0.113.10", ISP: "AS64500 Example Network", Country: "CN", City: "Shanghai",
			}, nil
		},
	}

	first := collector.readPublicNetwork(context.Background())
	<-lookedUp
	waitForPublicNetworkRefresh(t, collector)
	second := collector.readPublicNetwork(context.Background())
	if calls != 1 || first.IPv4 != "" || second.IPv4 != "203.0.113.10" {
		t.Fatalf("unexpected cached lookup: calls=%d first=%#v second=%#v", calls, first, second)
	}
	if second.Source != "ipinfo.io" || second.UpdatedAt == nil || !second.UpdatedAt.Equal(now) {
		t.Fatalf("lookup metadata was not normalized: %#v", second)
	}

	now = now.Add(2 * time.Hour)
	_ = collector.readPublicNetwork(context.Background())
	<-lookedUp
	waitForPublicNetworkRefresh(t, collector)
	if calls != 2 {
		t.Fatalf("expired cache was not refreshed: calls=%d", calls)
	}
}

func TestCachedPublicNetworkDoesNotStartLookup(t *testing.T) {
	lookupCalls := 0
	collector := NewCollector()
	collector.PublicNetworkLookup = func(context.Context) (contract.PublicNetworkSummary, error) {
		lookupCalls++
		return contract.PublicNetworkSummary{CountryCode: "CN"}, nil
	}
	collector.publicNetworkCache = contract.PublicNetworkSummary{CountryCode: "HK"}

	if got := collector.CachedPublicNetwork().CountryCode; got != "HK" {
		t.Fatalf("country = %q, want HK", got)
	}
	if lookupCalls != 0 {
		t.Fatalf("cached lookup started %d external requests", lookupCalls)
	}
}

func TestReadPublicNetworkCanceledCallerDoesNotStartLookup(t *testing.T) {
	c := NewCollector()
	c.PublicNetworkLookup = func(context.Context) (contract.PublicNetworkSummary, error) {
		t.Error("canceled caller started public-network lookup")
		return contract.PublicNetworkSummary{}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.readPublicNetwork(ctx)
	c.publicNetworkMu.Lock()
	defer c.publicNetworkMu.Unlock()
	if c.publicNetworkLoading {
		t.Fatal("canceled caller reserved a worker")
	}
}

func TestReadPublicNetworkCachesFailureBriefly(t *testing.T) {
	now := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	calls := 0
	lookedUp := make(chan struct{}, 2)
	collector := &Collector{
		Now:                   func() time.Time { return now },
		PublicNetworkCacheTTL: 30 * time.Minute,
		PublicNetworkLookup: func(context.Context) (contract.PublicNetworkSummary, error) {
			calls++
			lookedUp <- struct{}{}
			return contract.PublicNetworkSummary{}, errors.New("offline")
		},
	}

	if got := collector.readPublicNetwork(context.Background()); got.Source != "" {
		t.Fatalf("failed lookup returned data: %#v", got)
	}
	<-lookedUp
	waitForPublicNetworkRefresh(t, collector)
	_ = collector.readPublicNetwork(context.Background())
	if calls != 1 {
		t.Fatalf("failed lookup was repeated without backoff: calls=%d", calls)
	}

	now = now.Add(6 * time.Minute)
	_ = collector.readPublicNetwork(context.Background())
	<-lookedUp
	waitForPublicNetworkRefresh(t, collector)
	if calls != 2 {
		t.Fatalf("failed lookup was not retried after backoff: calls=%d", calls)
	}
}

func TestReadPublicNetworkDoesNotBlockSummary(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	collector := &Collector{
		Now:                   time.Now,
		PublicNetworkCacheTTL: time.Hour,
		PublicNetworkLookup: func(context.Context) (contract.PublicNetworkSummary, error) {
			close(started)
			<-release
			return contract.PublicNetworkSummary{IPv4: "203.0.113.20"}, nil
		},
	}

	begin := time.Now()
	got := collector.readPublicNetwork(context.Background())
	if elapsed := time.Since(begin); elapsed > 100*time.Millisecond {
		t.Fatalf("cached public network read blocked for %s", elapsed)
	}
	if got.IPv4 != "" {
		t.Fatalf("cold cache returned unexpected data: %#v", got)
	}
	<-started
	close(release)
	waitForPublicNetworkRefresh(t, collector)
}

func waitForPublicNetworkRefresh(t *testing.T, collector *Collector) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		collector.publicNetworkMu.Lock()
		loading := collector.publicNetworkLoading
		collector.publicNetworkMu.Unlock()
		if !loading {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("public network refresh did not finish")
}

func TestQueryIPInfoValidatesAddressFamilyAndBoundsMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
			"ip":"203.0.113.8",
			"city":"Shanghai",
			"region":"Shanghai",
			"country":"CN",
			"org":"AS64500 Example Network",
			"timezone":"Asia/Shanghai"
		}`))
	}))
	defer server.Close()

	info, err := queryIPInfo(context.Background(), server.Client(), server.URL, 4)
	if err != nil || info.IP != "203.0.113.8" || info.Org != "AS64500 Example Network" {
		t.Fatalf("queryIPInfo() = %#v, %v", info, err)
	}
	if _, err := queryIPInfo(context.Background(), server.Client(), server.URL, 6); err == nil {
		t.Fatal("IPv4 response was accepted as IPv6")
	}
	if got := cleanPublicField("  ISP\nName\x00  ", 16); got != "ISPName" {
		t.Fatalf("cleanPublicField() = %q", got)
	}
}

func TestQueryIPInfoRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write(make([]byte, (64<<10)+1))
	}))
	defer server.Close()

	if _, err := queryIPInfo(context.Background(), server.Client(), server.URL, 4); err == nil {
		t.Fatal("oversized response was accepted")
	}
}

func TestMergeIPInfoMetadataSupportsASNObject(t *testing.T) {
	var info ipInfoResponse
	info.IP = "2001:db8::1"
	info.CountryCode = "US"
	info.ASN.Name = "Example Transit"
	var summary contract.PublicNetworkSummary
	mergeIPInfoMetadata(&summary, info)
	if summary.ISP != "Example Transit" || summary.Country != "US" || summary.CountryCode != "US" {
		t.Fatalf("unexpected merged metadata: %#v", summary)
	}
}
