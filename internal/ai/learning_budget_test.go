package ai

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func quotaStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ai.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func seedQuotaRecords(t *testing.T, s *Store, table, user, parent string, count, contentBytes int, retired bool) {
	t.Helper()
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := range count {
		id := fmt.Sprintf("seed-%s-%s-%d", table, user, i)
		identity := id
		value := learningPrivateState{Title: "ordinary", Content: "preference"}
		if table != "memories" {
			value.Content, value.Condition, value.Steps = "", strings.Repeat("x", contentBytes), json.RawMessage(`[{"tool":"host_read","arguments":{}}]`)
		}
		if table == "procedure_versions" {
			identity += "\x00" + parent
		}
		encrypted, err := s.sealLearningState(table, identity, user, value)
		if err != nil {
			t.Fatal(err)
		}
		switch table {
		case "memories":
			_, err = tx.Exec(`INSERT INTO memories(id,user_id,title,content,enabled,retired,version,created_at,updated_at,private_state) VALUES(?,?,'','',1,?,1,1,1,?)`, id, user, retired, encrypted)
		case "procedures":
			_, err = tx.Exec(`INSERT INTO procedures(id,user_id,title,condition_text,steps,enabled,retired,version,created_at,updated_at,private_state) VALUES(?,?,'','',?,1,?,1,1,1,?)`, id, user, []byte(privateStatePlaceholder), retired, encrypted)
		case "procedure_versions":
			_, err = tx.Exec(`INSERT INTO procedure_versions(id,procedure_id,user_id,title,condition_text,steps,version,created_at,private_state) VALUES(?,?,?,'','',?,?,1,?)`, id, parent, user, []byte(privateStatePlaceholder), i+1, encrypted)
		default:
			t.Fatal("invalid fixture table")
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func quotaMemory(t *testing.T, s *Store, user string) EvolutionProposal {
	t.Helper()
	p, err := s.SaveProposal(context.Background(), EvolutionProposal{UserID: user, Type: EvolutionMemory, Title: "preference", Content: "ordinary preference"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLearningBudgetSerializesLastApprovalSlotAndRetainsPending(t *testing.T) {
	s, _ := quotaStore(t)
	seedQuotaRecords(t, s, "memories", "admin", "", maxLearningListRecords-1, 0, false)
	a, b := quotaMemory(t, s, "admin"), quotaMemory(t, s, "admin")
	start, results := make(chan struct{}), make(chan error, 2)
	for _, p := range []EvolutionProposal{a, b} {
		go func() { <-start; results <- s.DecideProposal(context.Background(), "admin", p.ID, true, nil) }()
	}
	close(start)
	success, rejected := 0, 0
	for range 2 {
		switch err := <-results; {
		case err == nil:
			success++
		case errors.Is(err, ErrLearningBudget):
			rejected++
		default:
			t.Fatal(err)
		}
	}
	items, err := s.Memories(context.Background(), "admin")
	pending, pendingErr := s.Proposals(context.Background(), "admin", EvolutionPending)
	if err != nil || pendingErr != nil || len(items) != maxLearningListRecords || len(pending) != 1 || success != 1 || rejected != 1 {
		t.Fatalf("partial approval or quota race: memories=%d pending=%d success=%d rejected=%d err=%v/%v", len(items), len(pending), success, rejected, err, pendingErr)
	}
	if _, err := s.UpdateMemory(context.Background(), "admin", items[0].ID, nil); err != nil {
		t.Fatalf("neutral update at capacity rejected: %v", err)
	}
	if err := s.DeleteMemory(context.Background(), "admin", items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DecideProposal(context.Background(), "admin", pending[0].ID, false, nil); err != nil {
		t.Fatalf("rejection at capacity rejected: %v", err)
	}
	items, err = s.Memories(context.Background(), "admin")
	retired := 0
	for _, item := range items {
		if item.Retired {
			retired++
		}
	}
	if err != nil || len(items) != maxLearningListRecords || retired != 1 {
		t.Fatal("retirement changed into deletion or disappeared from the public list")
	}
}

func TestLearningBudgetCountsAllTablesAndUsers(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(fmt.Sprint(global), func(t *testing.T) {
			s, _ := quotaStore(t)
			users, versions := []string{"admin"}, []int{maxUserLearningRecords - 2}
			if global {
				users, versions = []string{"a", "b", "c"}, []int{682, 681, 681}
			}
			for i, user := range users {
				seedQuotaRecords(t, s, "procedures", user, "", 1, 0, false)
				seedQuotaRecords(t, s, "procedure_versions", user, "seed-procedures-"+user+"-0", versions[i], 0, false)
			}
			start, results := make(chan struct{}), make(chan error, 2)
			for range 2 {
				go func() {
					<-start
					_, err := s.SaveProposal(context.Background(), EvolutionProposal{UserID: "admin", Type: EvolutionMemory, Title: "new", Content: "preference"})
					results <- err
				}()
			}
			close(start)
			success, rejected := 0, 0
			for range 2 {
				if err := <-results; err == nil {
					success++
				} else if errors.Is(err, ErrLearningBudget) {
					rejected++
				} else {
					t.Fatal(err)
				}
			}
			if success != 1 || rejected != 1 {
				t.Fatalf("cross-table/global quota race: successes=%d rejections=%d", success, rejected)
			}
		})
	}
}

func TestLearningBudgetBytesRejectApprovalAndVersionAtomically(t *testing.T) {
	s, _ := quotaStore(t)
	seedQuotaRecords(t, s, "procedures", "admin", "", 1, 0, false)
	parent := "seed-procedures-admin-0"
	seedQuotaRecords(t, s, "procedure_versions", "admin", parent, 15, (1<<20)-256, false)
	if _, err := s.db.Exec(`UPDATE procedures SET version=15 WHERE id=?`, parent); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	large := strings.Repeat("c", 650<<10)
	payload, _ := json.Marshal(map[string]any{"condition": large, "steps": []ProcedureStep{{Tool: "host_read", Arguments: json.RawMessage(`{}`)}}})
	p, err := s.SaveProposal(ctx, EvolutionProposal{UserID: "admin", Type: EvolutionProcedure, Title: "new procedure", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DecideProposal(ctx, "admin", p.ID, true, map[string]bool{"host_read": true}); !errors.Is(err, ErrLearningBudget) {
		t.Fatalf("approval passed the cumulative byte budget: %v", err)
	}
	var procedures, versions int
	if err := s.db.QueryRow(`SELECT count(*) FROM procedures`).Scan(&procedures); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM procedure_versions`).Scan(&versions); err != nil || procedures != 1 || versions != 15 {
		t.Fatalf("partial artifact/version remained: %d/%d %v", procedures, versions, err)
	}
	if _, err := s.ReviseProcedure(ctx, "admin", parent, "new revision", large, nil, 0); !errors.Is(err, ErrLearningBudget) {
		t.Fatalf("revision passed the byte budget: %v", err)
	}
	// A rollback creates a new retained revision too; it cannot discard history
	// to make room, and the current version must remain unchanged on failure.
	if _, err := s.ReviseProcedure(ctx, "admin", parent, "", "", nil, 1); !errors.Is(err, ErrLearningBudget) {
		t.Fatalf("rollback passed the byte budget: %v", err)
	}
	items, err := s.Procedures(ctx, "admin")
	pending, pendingErr := s.Proposals(ctx, "admin", EvolutionPending)
	if err != nil || pendingErr != nil || items[0].Version != 15 || items[0].Title != "ordinary" || len(pending) != 1 {
		t.Fatal("quota failure changed approval or current procedure")
	}
}

func TestLearningBudgetRejectsLegacyReadsBeforeDecrypting(t *testing.T) {
	s, _ := quotaStore(t)
	seedQuotaRecords(t, s, "procedures", "admin", "", 16, 600<<10, true)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	items, err := s.Procedures(context.Background(), "admin")
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("legacy 9.4 MiB private list rejected with totalAlloc=%d", allocated)
	if !errors.Is(err, ErrLearningBudget) || items != nil || allocated > 512<<10 {
		t.Fatalf("bodies materialized before list rejection: items=%d bytes=%d err=%v", len(items), allocated, err)
	}
}

func TestLearningBudgetNewEncodingFailsBeforeAllocation(t *testing.T) {
	s, _ := quotaStore(t)
	value := learningPrivateState{Title: "ordinary", Condition: strings.Repeat("<", 800<<10)}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := s.sealLearningState("procedures", "new", "admin", value)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, ErrLearningBudget) || after.TotalAlloc-before.TotalAlloc > 512<<10 {
		t.Fatalf("expanded private JSON allocated before rejection: bytes=%d err=%v", after.TotalAlloc-before.TotalAlloc, err)
	}
	if _, err := s.sealLearningState("evolution_proposals", "invalid", "admin", learningPrivateState{Title: "ordinary", Payload: json.RawMessage(`{`)}); err == nil || errors.Is(err, ErrLearningBudget) {
		t.Fatalf("invalid content was reported as a capacity failure: %v", err)
	}
}

func TestLearningBudgetKeepsTenMaximumHTMLSteps(t *testing.T) {
	s, _ := quotaStore(t)
	ctx := context.Background()
	const prefix, suffix = `{"path":"/home/web/example.html","content":"`, `"}`
	content := strings.Repeat("<", MaxToolResultBytes-len(prefix)-len(suffix))
	arguments := json.RawMessage(prefix + content + suffix)
	if len(arguments) != MaxToolResultBytes {
		t.Fatal("fixture does not cover the registered per-call argument maximum")
	}
	// Preserve the legal incoming JSON representation. Its HTML escaping is
	// performed by the existing private-state encoder, not by this fixture.
	var rawSteps bytes.Buffer
	rawSteps.WriteByte('[')
	for i := 0; i < 10; i++ {
		if i > 0 {
			rawSteps.WriteByte(',')
		}
		rawSteps.WriteString(`{"tool":"host_file_write","arguments":`)
		rawSteps.Write(arguments)
		rawSteps.WriteByte('}')
	}
	rawSteps.WriteByte(']')
	payload := json.RawMessage(`{"condition":"restore the approved page","steps":` + rawSteps.String() + `}`)
	if len(payload) > MaxAssistantBytes {
		t.Fatal("fixture exceeds the existing learning response limit")
	}
	proposal, err := s.SaveProposal(ctx, EvolutionProposal{UserID: "admin", Type: EvolutionProcedure, Title: "page recovery", Payload: payload})
	if err != nil {
		t.Fatalf("legal ten-step proposal rejected: %v", err)
	}
	if err := s.DecideProposal(ctx, "admin", proposal.ID, true, map[string]bool{"host_file_write": true}); err != nil {
		t.Fatalf("legal proposal cannot be approved with its initial version: %v", err)
	}
	items, err := s.Procedures(ctx, "admin")
	if err != nil || len(items) != 1 {
		t.Fatalf("legal procedure cannot be read: %d %v", len(items), err)
	}
	rolled, err := s.ReviseProcedure(ctx, "admin", items[0].ID, "", "", nil, 1)
	if err != nil || rolled.Version != 2 {
		t.Fatalf("legal procedure cannot retain its rollback version: %v", err)
	}
	var steps []ProcedureStep
	if err := json.Unmarshal(rolled.Steps, &steps); err != nil || len(steps) != 10 {
		t.Fatalf("steps changed after storage/approval/rollback: %d %v", len(steps), err)
	}
	for _, step := range steps {
		var argument struct{ Path, Content string }
		if err := json.Unmarshal(step.Arguments, &argument); err != nil || argument.Content != content || argument.Path != "/home/web/example.html" {
			t.Fatal("private encoding changed legitimate execution parameters")
		}
	}
}

func TestLearningBudgetRejectsLegacyPlaintextBeforeMigration(t *testing.T) {
	for _, growth := range []bool{false, true} {
		t.Run(fmt.Sprint(growth), func(t *testing.T) {
			s, path := quotaStore(t)
			count, condition := 16, strings.Repeat("x", 600<<10)
			if growth {
				count, condition = 5, strings.Repeat("<", 300<<10)
			}
			for i := range count {
				id := fmt.Sprintf("legacy-%d", i)
				if _, err := s.db.Exec(`INSERT INTO procedures(id,user_id,title,condition_text,steps,enabled,version,created_at,updated_at) VALUES(?,'admin','legacy',?,X'5B5D',1,1,1,1)`, id, condition); err != nil {
					t.Fatal(err)
				}
			}
			if growth {
				if _, err := s.db.Exec(`CREATE TRIGGER no_fifth_conversion BEFORE UPDATE OF private_state ON procedures WHEN OLD.id='legacy-4' BEGIN SELECT RAISE(ABORT,'conversion proceeded past reserved capacity'); END`); err != nil {
					t.Fatal(err)
				}
			}
			s.Close()
			if opened, err := OpenStore(path); !errors.Is(err, ErrLearningBudget) {
				if opened != nil {
					opened.Close()
				}
				t.Fatalf("legacy capacity not rejected before migration: %v", err)
			}
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var retained int
			if err := db.QueryRow(`SELECT count(*) FROM procedures WHERE length(private_state)=0 AND title='legacy' AND condition_text=?`, condition).Scan(&retained); err != nil || retained != count {
				t.Fatalf("rejected migration changed records: %d %v", retained, err)
			}
		})
	}
}
