package sitetraffic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRegressionNativeRateEmptyStatus(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "log/nginx"), 0700)
	line := time.Now().Format(time.RFC3339) + ` key="site.example|192.0.2.1" event=rate request_limit="REJECTED" connection_limit=""` + "\n"
	os.WriteFile(filepath.Join(root, "log/nginx/waf-rate.log"), []byte(line), 0600)
	b, err := NewReader(root).Collect(context.Background(), CollectInput{Salt: strings.Repeat("ab", 32)})
	fmt.Printf("native_rate_empty_status: records=%d skipped=%d wafSource=%v error=%v line=%s", len(b.Records), b.Skipped, b.WAFSource, err, line)
	if err != nil || len(b.Records) != 1 || b.Records[0].Action != "limited" {
		t.Fatalf("native rejected event was lost: records=%d skipped=%d error=%v", len(b.Records), b.Skipped, err)
	}
}
