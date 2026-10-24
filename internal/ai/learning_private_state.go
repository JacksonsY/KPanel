package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// Learning content is private even when it contains no recognized credential.
// Only this representation is persisted; projections must never become inputs
// to revisions, rollback, approval checks or tool execution.
type learningPrivateState struct {
	Title     string          `json:"title"`
	Content   string          `json:"content,omitempty"`
	Condition string          `json:"condition,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Steps     json.RawMessage `json:"steps,omitempty"`
}

func learningIdentity(table, id, userID string) string {
	return "ai-learning-private-v1\x00" + table + "\x00" + userID + "\x00" + id
}

func (s *Store) sealLearningState(table, id, userID string, value learningPrivateState) ([]byte, error) {
	raw, err := marshalProviderJSON(value, maxLearningStateBytes-40)
	if err != nil {
		if errors.Is(err, errProviderJSONBudget) {
			return nil, ErrLearningBudget
		}
		return nil, errors.New("invalid AI private learning state")
	}
	return s.secrets.Seal(learningIdentity(table, id, userID), string(raw))
}

func (s *Store) openLearningState(table, id, userID string, encrypted []byte) (learningPrivateState, error) {
	var value learningPrivateState
	if len(encrypted) == 0 || len(encrypted) > maxPrivateStateBytes+1024 {
		return value, errors.New("invalid AI private learning state")
	}
	raw, err := s.secrets.Open(learningIdentity(table, id, userID), encrypted)
	if err != nil || json.Unmarshal([]byte(raw), &value) != nil {
		return value, errors.New("AI private learning state cannot be decrypted")
	}
	return value, nil
}

var learningTables = []struct{ table, columns, clear, clean, identity string }{
	{"evolution_proposals", "title,content,'',payload,X''", "title='',content='',payload='" + privateStatePlaceholder + "'", "title='' AND content='' AND CAST(payload AS TEXT)='" + privateStatePlaceholder + "'", "id"},
	{"memories", "title,content,'',X'',X''", "title='',content=''", "title='' AND content=''", "id"},
	{"procedures", "title,'',condition_text,X'',steps", "title='',condition_text='',steps='" + privateStatePlaceholder + "'", "title='' AND condition_text='' AND CAST(steps AS TEXT)='" + privateStatePlaceholder + "'", "id"},
	{"procedure_versions", "title,'',condition_text,X'',steps", "title='',condition_text='',steps='" + privateStatePlaceholder + "'", "title='' AND condition_text='' AND CAST(steps AS TEXT)='" + privateStatePlaceholder + "'", "id||char(0)||procedure_id"},
}

// Validate before migrating ANY private state, including tools. A restored
// mismatched key must not produce a partially re-encrypted mixed-key database.
func (s *Store) validateLearningState(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := validateLearningStorage(ctx, tx); err != nil {
		return err
	}
	for _, table := range learningTables {
		rows, err := tx.QueryContext(ctx, `SELECT `+table.identity+`,user_id,CASE WHEN length(private_state)<=10486784 THEN private_state END,(`+table.clean+`) FROM `+table.table+` WHERE length(private_state)>0`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, userID string
			var encrypted []byte
			var clean bool
			if err = rows.Scan(&id, &userID, &encrypted, &clean); err == nil {
				_, err = s.openLearningState(table.table, id, userID, encrypted)
				if err == nil && !clean {
					err = errors.New("AI legacy learning fields conflict with private state")
				}
			}
			if err != nil {
				rows.Close()
				return err
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) protectLearningState(ctx context.Context) error {
	var scrubbed int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE version=12`).Scan(&scrubbed); err != nil {
		return err
	}
	tx, err := beginLearningWrite(ctx, s.db)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	budget, err := readLearningBudget(ctx, tx)
	if err != nil {
		return err
	}
	for _, table := range learningTables {
		last := ""
		for {
			var id, userID, identity string
			var encrypted []byte
			var previousBytes int
			var value learningPrivateState
			// Legacy input is bounded before it enters Go, including arbitrary
			// records left by an older binary after a downgrade.
			columns := strings.Split(table.columns, ",")
			for i, column := range columns {
				columns[i] = "CASE WHEN length(" + column + ")<=10485760 THEN " + column + " END"
			}
			err := tx.QueryRowContext(ctx, `SELECT id,user_id,`+table.identity+`,`+strings.Join(columns, ",")+`,`+learningRowBytesSQL(table.table)+` FROM `+table.table+` WHERE id>? AND length(private_state)=0 ORDER BY id LIMIT 1`, last).Scan(&id, &userID, &identity, &value.Title, &value.Content, &value.Condition, &value.Payload, &value.Steps, &previousBytes)
			if errors.Is(err, sql.ErrNoRows) {
				break
			}
			if err != nil {
				return err
			}
			last = id
			encrypted, err = s.sealLearningState(table.table, identity, userID, value)
			if err != nil {
				return err
			}
			plainBytes := len(value.Title) + len(value.Content) + len(value.Condition) + len(value.Payload) + len(value.Steps)
			placeholderBytes := 0
			if table.table != "memories" {
				placeholderBytes = len(privateStatePlaceholder)
			}
			nextBytes := previousBytes - plainBytes + placeholderBytes + len(encrypted)
			if nextBytes > maxLearningRowBytes {
				return ErrLearningBudget
			}
			// Reserve each conversion's growth before updating it; a bounded
			// plaintext database cannot expand into a huge doomed transaction.
			if err = budget.reserve(table.table, userID, previousBytes, nextBytes, 0); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version=12`); err != nil {
				return err
			}
			scrubbed = 0
			if _, err = tx.ExecContext(ctx, `UPDATE `+table.table+` SET `+table.clear+`,private_state=? WHERE id=?`, encrypted, id); err != nil {
				return err
			}
		}
	}
	if err := validateLearningStorage(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if scrubbed == 0 {
		return s.scrubPrivateState(ctx, 12)
	}
	return nil
}

func safeLearningSteps(raw json.RawMessage) json.RawMessage {
	var steps []ProcedureStep
	if json.Unmarshal(raw, &steps) != nil {
		return json.RawMessage(`[]`)
	}
	for i := range steps {
		steps[i].Tool = redactAndLimit(steps[i].Tool, 256)
		steps[i].Arguments = json.RawMessage(redactToolArguments(steps[i].Arguments))
		if !json.Valid(steps[i].Arguments) {
			steps[i].Arguments = json.RawMessage(`"[REDACTED]"`)
		}
	}
	encoded, _ := json.Marshal(steps)
	return encoded
}

func (p EvolutionProposal) MarshalJSON() ([]byte, error) {
	type publicProposal EvolutionProposal
	p.Title, p.Content = redactAndLimit(p.Title, 120), redactAndLimit(p.Content, 500)
	if p.Type == EvolutionProcedure {
		var body struct {
			Condition string          `json:"condition"`
			Steps     json.RawMessage `json:"steps"`
		}
		if json.Unmarshal(p.Payload, &body) == nil {
			body.Condition, body.Steps = redactAndLimit(body.Condition, 500), safeLearningSteps(body.Steps)
			p.Payload, _ = json.Marshal(body)
		} else {
			p.Payload = nil
		}
	} else {
		p.Payload = nil
	}
	return json.Marshal(publicProposal(p))
}

func (p Procedure) MarshalJSON() ([]byte, error) {
	type publicProcedure Procedure
	p.Title, p.Condition, p.Steps = redactAndLimit(p.Title, 120), redactAndLimit(p.Condition, 500), safeLearningSteps(p.Steps)
	return json.Marshal(publicProcedure(p))
}

func (m Memory) MarshalJSON() ([]byte, error) {
	type publicMemory Memory
	m.Title, m.Content = redactAndLimit(m.Title, 120), redactAndLimit(m.Content, 500)
	return json.Marshal(publicMemory(m))
}
