package history

import (
	"context"
	"fmt"
	"github.com/kejilion/kejilion-panel/internal/sitetraffic"
	"strings"
	"testing"
	"time"
)

func TestRegressionWAFLateMergeAfterRecentCap(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	s.Action(ctx, "enable")
	base := time.Now().Add(-2 * time.Hour).Unix()
	apply := func(records []sitetraffic.Record) {
		in, _, _ := s.State(ctx)
		if err := s.Apply(ctx, sitetraffic.Batch{Generation: in.Generation, GeoStatus: "missing", Records: records, WAFSource: true}); err != nil {
			t.Fatal(err)
		}
	}
	event := func(i int) sitetraffic.Record {
		return sitetraffic.Record{Kind: "waf", At: base + int64(i), Host: "site.example", Country: "ZZ", IPHash: strings.Repeat("a", 32), EventKey: fmt.Sprintf("%032x", i+1), Action: "matched", Rules: []string{"942100"}, Severity: 2}
	}
	for start := 0; start < 5001; start += 1000 {
		rs := []sitetraffic.Record{}
		for i := start; i < start+1000 && i < 5001; i++ {
			rs = append(rs, event(i))
		}
		apply(rs)
	}
	before, _ := s.Query(ctx, "24h", "")
	late := event(0)
	late.Action = "blocked"
	apply([]sitetraffic.Record{late})
	after, _ := s.Query(ctx, "24h", "")
	fmt.Printf("late_merge: beforeEvents=%d beforeMatched=%d afterEvents=%d afterMatched=%d afterBlocked=%d\n", before.Protection.Events, before.Protection.Matched, after.Protection.Events, after.Protection.Matched, after.Protection.Blocked)
	if after.Protection.Events != 5001 || after.Protection.Matched != 5000 || after.Protection.Blocked != 1 {
		t.Fatalf("same transaction counted twice after recent cap")
	}
}
