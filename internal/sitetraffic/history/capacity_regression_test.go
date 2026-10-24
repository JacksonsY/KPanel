package history

import (
	"context"
	"fmt"
	"github.com/kejilion/kejilion-panel/internal/sitetraffic"
	"strings"
	"testing"
	"time"
)

func TestRegressionWAFCapacityDoesNotPoisonLateEvent(t *testing.T) {
	ctx := context.Background()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Action(ctx, "enable"); err != nil {
		t.Fatal(err)
	}
	// Bulk materialize the state of 13,000 accepted WAF transactions, each with
	// 32 rules, after presentation retention has kept the newest 5,000 events.
	base := time.Now().Add(-time.Hour).Unix()
	hour := base / 3600 * 3600
	_, err = s.db.Exec(`WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<13000) INSERT INTO transactions SELECT printf('%032x',x),?,'site.example','ZZ','matched' FROM n`, base)
	if err != nil {
		t.Fatal(err)
	}
	ruleList := []string{}
	for i := 1; i <= 32; i++ {
		ruleList = append(ruleList, fmt.Sprintf("\"%06d\"", 942000+i))
	}
	ruleJSON := "[" + strings.Join(ruleList, ",") + "]"
	_, err = s.db.Exec(`INSERT INTO events SELECT id,at,host,'/api',country,action,2,? FROM transactions ORDER BY id LIMIT 5000`, ruleJSON)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<32) INSERT INTO event_rules SELECT id,printf('%06d',942000+x) FROM transactions,n`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`INSERT INTO waf VALUES(?,'site.example','ZZ','matched',13000)`, hour)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`INSERT INTO rule_counts SELECT ?,'site.example',rule,13000 FROM event_rules GROUP BY rule`, hour)
	if err != nil {
		t.Fatal(err)
	}
	in, _, _ := s.State(ctx)
	if err = s.Apply(ctx, sitetraffic.Batch{Generation: in.Generation, GeoStatus: "missing", WAFSource: true, Cursors: []sitetraffic.Cursor{{ID: "fixture", Offset: 1}}}); err != nil {
		t.Fatal(err)
	}
	var ledger, recent int
	s.db.QueryRow(`SELECT count(*) FROM transactions`).Scan(&ledger)
	s.db.QueryRow(`SELECT count(*) FROM events`).Scan(&recent)
	in, _, _ = s.State(ctx)
	late := sitetraffic.Record{Kind: "waf", At: base, Host: "site.example", Country: "ZZ", IPHash: strings.Repeat("a", 32), EventKey: fmt.Sprintf("%032x", 1), Action: "blocked", Rules: []string{"942001"}, Severity: 2}
	err = s.Apply(ctx, sitetraffic.Batch{Generation: in.Generation, GeoStatus: "missing", WAFSource: true, Records: []sitetraffic.Record{late}, Cursors: []sitetraffic.Cursor{{ID: "fixture", Offset: 2}}})
	after, _, _ := s.State(ctx)
	fmt.Printf("pure_waf_capacity: ledger=%d recent=%d lateError=%v cursorAfter=%d\n", ledger, recent, err, after.Cursors[0].Offset)
	if err != nil || after.Cursors[0].Offset != 2 {
		t.Fatalf("capacity state permanently blocks late event and checkpoint: %v", err)
	}
}
