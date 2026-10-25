package sitetraffic

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuiltinCountryDatabaseIntegrity(t *testing.T) {
	if sum := fmt.Sprintf("%x", sha256.Sum256(builtinCountryDatabase)); sum != "dbd70ccfa2a13627eaf4913d19920a1a229f1b3c5439ac01c509995b6364202a" {
		t.Fatalf("unexpected bundled database checksum: %s", sum)
	}
}

func TestCollectDefaultCountryDatabase(t *testing.T) {
	for _, configExists := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh-install", true: "existing-config"}[configExists], func(t *testing.T) {
			root, reader := readerFixture(t)
			defer reader.dropGeo()
			if !configExists {
				if err := os.Remove(filepath.Join(root, "config")); err != nil {
					t.Fatal(err)
				}
			}
			log := access("8.8.8.8", "/ipv4", 200) + access("2001:4860:4860::8888", "/ipv6", 200)
			if err := os.WriteFile(filepath.Join(root, "log/nginx/access.log"), []byte(log), 0600); err != nil {
				t.Fatal(err)
			}
			batch, err := reader.Collect(context.Background(), CollectInput{Salt: strings.Repeat("ab", 32)})
			if err != nil {
				t.Fatal(err)
			}
			if batch.GeoStatus != "ready" || len(batch.Records) != 2 {
				t.Fatalf("batch=%+v", batch)
			}
			for i, record := range batch.Records {
				if record.Country != []string{"US", "CA"}[i] {
					t.Fatalf("record=%+v", record)
				}
			}
		})
	}
}

func TestDefaultCountryDatabaseOverrideAndRemoval(t *testing.T) {
	root, reader := readerFixture(t)
	defer reader.dropGeo()
	web, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer web.Close()
	reader.loadGeo(web)
	ip := netip.MustParseAddr("8.8.8.8")
	if reader.geoStatus != "ready" || reader.country(ip) != "US" {
		t.Fatalf("initial status=%s", reader.geoStatus)
	}
	fixture, err := os.ReadFile("testdata/GeoIP2-Country-Test.mmdb")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config/GeoLite2-Country.mmdb")
	if err := os.WriteFile(path, fixture, 0600); err != nil {
		t.Fatal(err)
	}
	reader.geoChecked = time.Time{}
	reader.loadGeo(web)
	if reader.geoStatus != "ready" || reader.dbInfo == nil {
		t.Fatal("custom database was not loaded")
	}
	if reader.country(ip) != "ZZ" {
		t.Fatal("built-in database overrode the custom database")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	reader.geoChecked = time.Time{}
	reader.loadGeo(web)
	if reader.geoStatus != "ready" || reader.dbInfo != nil || reader.country(ip) != "US" {
		t.Fatal("built-in database was not restored")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("default must not write a database to the host")
	}
}

func TestInvalidCustomCountryDatabaseRemainsVisible(t *testing.T) {
	root, reader := readerFixture(t)
	defer reader.dropGeo()
	path := filepath.Join(root, "config/GeoLite2-Country.mmdb")
	if err := os.WriteFile(path, []byte("invalid database"), 0600); err != nil {
		t.Fatal(err)
	}
	web, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer web.Close()
	reader.loadGeo(web)
	if reader.geoStatus != "invalid" || reader.db != nil {
		t.Fatal("invalid custom database was hidden")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	reader.geoChecked = time.Time{}
	reader.loadGeo(web)
	if reader.geoStatus != "ready" {
		t.Fatal("removing invalid override did not restore the default")
	}
}
