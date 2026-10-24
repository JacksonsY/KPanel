package ai

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func learningFixture(t *testing.T) (*Store, string, EvolutionProposal, Procedure) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ai.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()
	proposal, err := s.SaveProposal(ctx, EvolutionProposal{UserID: "admin", Type: EvolutionProcedure, Title: "safe reusable procedure", Content: "token=metadata-private-marker", Payload: json.RawMessage(`{"condition":"ordinary condition","steps":[{"tool":"host_action","arguments":{"password":"credential-private-marker","to\u006ben":"decoded-private-marker","Content":"unmarked-body-private-marker","command":["run","--token","argv-private-marker"],"path":"/tmp/ordinary"}}]}`)})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DecideProposal(ctx, "admin", proposal.ID, true, map[string]bool{"host_action": true}); err != nil {
		t.Fatal(err)
	}
	items, err := s.Procedures(ctx, "admin")
	if err != nil || len(items) != 1 {
		t.Fatalf("procedures=%v err=%v", items, err)
	}
	return s, path, proposal, items[0]
}

func assertLearningFilesPrivate(t *testing.T, path string) {
	t.Helper()
	for _, suffix := range []string{"", "-wal", "-shm"} {
		raw, err := os.ReadFile(path + suffix)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		for _, marker := range []string{"credential-private-marker", "decoded-private-marker", "unmarked-body-private-marker", "argv-private-marker", "metadata-private-marker", "memory-private-marker"} {
			if bytes.Contains(raw, []byte(marker)) {
				t.Fatalf("plaintext %q remains in ai.db%s", marker, suffix)
			}
		}
	}
}

func TestLearningPrivateStatePreservesParametersAndProjectsPublicContext(t *testing.T) {
	s, path, proposal, procedure := learningFixture(t)
	ctx := context.Background()
	for _, item := range []any{proposal, procedure} {
		raw, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		for _, marker := range []string{"credential-private-marker", "decoded-private-marker", "unmarked-body-private-marker", "argv-private-marker", "metadata-private-marker"} {
			if bytes.Contains(raw, []byte(marker)) {
				t.Fatalf("public projection leaked %q: %s", marker, raw)
			}
		}
		if !bytes.Contains(raw, []byte("/tmp/ordinary")) {
			t.Fatalf("ordinary arguments lost: %s", raw)
		}
	}
	if !strings.Contains(string(procedure.Steps), "unmarked-body-private-marker") {
		t.Fatal("public projection mutated execution state")
	}
	runtime := &NativeRuntime{store: s}
	prompt := runtime.systemPrompt(ctx, "admin")
	if strings.Contains(prompt, "private-marker") || !strings.Contains(prompt, "/tmp/ordinary") {
		t.Fatalf("unsafe or unusable learned context: %s", prompt)
	}
	revised, err := s.ReviseProcedure(ctx, "admin", procedure.ID, "revised", "", nil, 0)
	if err != nil || !bytes.Equal(revised.Steps, procedure.Steps) {
		t.Fatalf("metadata revision rewrote parameters: %v", err)
	}
	rolled, err := s.ReviseProcedure(ctx, "admin", procedure.ID, "", "", nil, 1)
	if err != nil || !bytes.Equal(rolled.Steps, procedure.Steps) || rolled.Title != procedure.Title {
		t.Fatalf("rollback changed original parameters: %v", err)
	}
	if _, err := s.UpdateProcedure(ctx, "admin", procedure.ID, nil); err != nil {
		t.Fatal(err)
	}
	assertLearningFilesPrivate(t, path)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	items, err := reopened.Procedures(ctx, "admin")
	if err != nil || !bytes.Equal(items[0].Steps, procedure.Steps) {
		t.Fatalf("reopen changed original parameters: %v", err)
	}
}

func TestLearningLegacyMigrationScrubsAllTablesAndCanRepeat(t *testing.T) {
	s, path, proposal, procedure := learningFixture(t)
	ctx := context.Background()
	// Emulate an old binary writing a schema-11 database, including after a
	// previous successful schema-12 startup. No encrypted state is trusted as
	// approval, and the original lifecycle fields must remain unchanged.
	if _, err := s.db.ExecContext(ctx, `UPDATE evolution_proposals SET title=?,content=?,payload=?,private_state=X'' WHERE id=?`, proposal.Title, proposal.Content, []byte(proposal.Payload), proposal.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE procedures SET title=?,condition_text=?,steps=?,private_state=X'' WHERE id=?`, procedure.Title, procedure.Condition, []byte(procedure.Steps), procedure.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE procedure_versions SET title=?,condition_text=?,steps=?,private_state=X'' WHERE procedure_id=?`, procedure.Title, procedure.Condition, []byte(procedure.Steps), procedure.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO memories(id,user_id,title,content,enabled,version,created_at,updated_at) VALUES('legacy-memory','admin','preference','token=memory-private-marker',1,7,12,13)`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	items, err := s.Procedures(ctx, "admin")
	if err != nil || len(items) != 1 || !bytes.Equal(items[0].Steps, procedure.Steps) || !items[0].CreatedAt.Equal(procedure.CreatedAt) || items[0].Version != 1 {
		t.Fatalf("legacy procedure changed: %#v %v", items, err)
	}
	proposals, err := s.Proposals(ctx, "admin", EvolutionActive)
	if err != nil || len(proposals) != 1 || !bytes.Equal(proposals[0].Payload, proposal.Payload) {
		t.Fatalf("legacy proposal changed: %#v %v", proposals, err)
	}
	memories, err := s.Memories(ctx, "admin")
	if err != nil || len(memories) != 1 || memories[0].Content != "token=memory-private-marker" || memories[0].Version != 7 {
		t.Fatalf("legacy memory changed: %#v %v", memories, err)
	}
	public, _ := json.Marshal(memories)
	if bytes.Contains(public, []byte("memory-private-marker")) {
		t.Fatalf("memory projection leaked: %s", public)
	}
	if _, err := s.ReviseProcedure(ctx, "admin", procedure.ID, "", "", nil, 1); err != nil {
		t.Fatal(err)
	}
	assertLearningFilesPrivate(t, path)
	// Simulate a crash after the conversion commit but before the durable
	// scrub marker. A restart must repeat scrubbing rather than skip it.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version=12`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var marker int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE version=12`).Scan(&marker); err != nil || marker != 1 {
		t.Fatalf("scrub marker=%d err=%v", marker, err)
	}
	assertLearningFilesPrivate(t, path)
}

func TestLearningMigrationRollbackAndFailureClosed(t *testing.T) {
	s, path, proposal, _ := learningFixture(t)
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, `UPDATE evolution_proposals SET title=?,content=?,payload=?,private_state=X''`, proposal.Title, proposal.Content, []byte(proposal.Payload)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO memories(id,user_id,title,content,enabled,version,created_at,updated_at) VALUES('legacy','admin','preference','ordinary preference',1,1,1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER fail_learning BEFORE UPDATE OF private_state ON memories BEGIN SELECT RAISE(ABORT,'injected learning migration failure'); END`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if opened, err := OpenStore(path); err == nil {
		opened.Close()
		t.Fatal("partial migration exposed service")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := db.QueryRowContext(ctx, `SELECT payload FROM evolution_proposals WHERE id=? AND length(private_state)=0`, proposal.ID).Scan(&raw); err != nil || !bytes.Equal(raw, proposal.Payload) {
		t.Fatalf("failed migration did not roll back: %s %v", raw, err)
	}
	if _, err := db.ExecContext(ctx, `DROP TRIGGER fail_learning`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	assertLearningFilesPrivate(t, path)
}

func TestLearningOnlyKeyLossWrongKeyAndCiphertextSubstitutionFailClosed(t *testing.T) {
	for _, mode := range []string{"missing-key", "wrong-key", "wrong-user", "wrong-procedure", "corrupt", "legacy-shadow"} {
		t.Run(mode, func(t *testing.T) {
			s, path, _, procedure := learningFixture(t)
			ctx := context.Background()
			keyPath := filepath.Join(filepath.Dir(path), "ai-secrets.key")
			switch mode {
			case "wrong-user":
				_, _ = s.db.ExecContext(ctx, `UPDATE procedures SET user_id='other'`)
			case "wrong-procedure":
				_, _ = s.db.ExecContext(ctx, `INSERT INTO procedures(id,user_id,title,condition_text,steps,enabled,version,created_at,updated_at) VALUES('other','admin','','','[]',1,1,1,1)`)
				_, _ = s.db.ExecContext(ctx, `UPDATE procedure_versions SET procedure_id='other' WHERE procedure_id=?`, procedure.ID)
			case "corrupt":
				_, _ = s.db.ExecContext(ctx, `UPDATE procedures SET private_state=X'010203'`)
			case "legacy-shadow":
				_, _ = s.db.ExecContext(ctx, `UPDATE procedures SET steps='[{"tool":"other"}]'`)
			}
			s.Close()
			if mode == "missing-key" {
				if err := os.Remove(keyPath); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "wrong-key" {
				if err := os.WriteFile(keyPath, bytes.Repeat([]byte{3}, 32), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if opened, err := OpenStore(path); err == nil {
				opened.Close()
				t.Fatal("invalid learning state exposed service")
			}
			if mode == "missing-key" {
				if _, err := os.Stat(keyPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("missing key replaced")
				}
			}
		})
	}
}

func TestLearningConfigurationRestorePreservesTargetLearning(t *testing.T) {
	s, _, _, procedure := learningFixture(t)
	box := s.secrets
	providers, err := NewProviderService(s, box)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	backup, err := providers.ExportAccess(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(backup)
	if bytes.Contains(raw, []byte("private-marker")) || bytes.Contains(raw, []byte(procedure.ID)) {
		t.Fatal("learning entered configuration backup")
	}
	if err := providers.RestoreAccess(ctx, backup); err != nil {
		t.Fatal(err)
	}
	items, err := s.Procedures(ctx, "admin")
	if err != nil || len(items) != 1 || !bytes.Equal(items[0].Steps, procedure.Steps) {
		t.Fatalf("restore changed private learning: %v", err)
	}
}

func TestLearningDatabaseRestoreUsesCompanionKey(t *testing.T) {
	s, sourcePath, _, procedure := learningFixture(t)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	for _, name := range []string{"ai.db", "ai-secrets.key"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(sourcePath), name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destination, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := OpenStore(filepath.Join(destination, "ai.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	items, err := restored.Procedures(context.Background(), "admin")
	if err != nil || len(items) != 1 || !bytes.Equal(items[0].Steps, procedure.Steps) {
		t.Fatalf("paired restore lost learning: %v", err)
	}
}

type learningArgumentsTools struct {
	fakeTools
	checked json.RawMessage
}

func (tools *learningArgumentsTools) DryRun(_ string, args json.RawMessage) error {
	tools.checked = append(json.RawMessage(nil), args...)
	return nil
}

func TestGeneratedLearningChecksOriginalArgumentsAndCannotApproveItself(t *testing.T) {
	store, providers, provider, model := runtimeFixture(t)
	defer store.Close()
	ctx := context.Background()
	session, err := store.CreateSession(ctx, Session{UserID: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := store.CreateRun(ctx, Run{UserID: "admin", SessionID: session.ID})
	if err != nil {
		t.Fatal(err)
	}
	args := json.RawMessage(`{"path":"/tmp/ordinary","content":"unmarked-body-private-marker","password":"credential-private-marker"}`)
	if _, err := store.SaveToolCall(ctx, ToolCall{ID: "learning-call", RunID: run.ID, SessionID: session.ID, Name: "host_action", Arguments: args, Status: ToolCompleted}); err != nil {
		t.Fatal(err)
	}
	output := `{"decision":"procedure","confidence":1,"approved":true,"status":"active","title":"generated procedure","condition":"ordinary condition","steps":[{"tool":"host_action","approved":true,"arguments":` + string(args) + `}]}`
	tools := &learningArgumentsTools{}
	runtime, err := NewNativeRuntime(store, providers, &learningClient{output: output}, tools, NewEventHub())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if err := runtime.generateProposal(ctx, run, provider, "test", model, nil, true); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(tools.checked, args) {
		t.Fatalf("dry-run received projection: %s", tools.checked)
	}
	pending, err := store.Proposals(ctx, "admin", EvolutionPending)
	if err != nil || len(pending) != 1 {
		t.Fatalf("model changed pending status: %v", err)
	}
	procedures, err := store.Procedures(ctx, "admin")
	if err != nil || len(procedures) != 0 || tools.executed != 0 {
		t.Fatalf("model approved/executed learning: %v", err)
	}
}
