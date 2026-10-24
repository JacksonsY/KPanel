package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (s *Store) SaveProposal(ctx context.Context, proposal EvolutionProposal) (EvolutionProposal, error) {
	if proposal.Type != EvolutionMemory && proposal.Type != EvolutionProcedure {
		return EvolutionProposal{}, errors.New("invalid proposal type")
	}
	proposal.Title = strings.TrimSpace(proposal.Title)
	proposal.Content = strings.TrimSpace(proposal.Content)
	if proposal.Title == "" || len(proposal.Title) > 120 {
		return EvolutionProposal{}, errors.New("proposal title is invalid")
	}
	if proposal.Type == EvolutionMemory && (proposal.Content == "" || len([]rune(proposal.Content)) > 500) {
		return EvolutionProposal{}, errors.New("memory proposal must contain at most 500 characters")
	}
	if proposal.Type == EvolutionProcedure {
		var payload struct {
			Condition string          `json:"condition"`
			Steps     []ProcedureStep `json:"steps"`
		}
		if json.Unmarshal(proposal.Payload, &payload) != nil || strings.TrimSpace(payload.Condition) == "" || len(payload.Steps) == 0 || len(payload.Steps) > 10 {
			return EvolutionProposal{}, errors.New("procedure proposal is invalid")
		}
	}
	if len(proposal.Payload) == 0 {
		proposal.Payload = json.RawMessage(`{}`)
	}
	now := s.now().UTC()
	if proposal.ID == "" {
		proposal.ID = newID("evo")
	}
	proposal.Status = EvolutionPending
	proposal.Version = 1
	proposal.CreatedAt = now
	proposal.UpdatedAt = now
	encrypted, err := s.sealLearningState("evolution_proposals", proposal.ID, proposal.UserID, learningPrivateState{Title: proposal.Title, Content: proposal.Content, Payload: proposal.Payload})
	if err != nil {
		return EvolutionProposal{}, err
	}
	tx, err := beginLearningWrite(ctx, s.db)
	if err != nil {
		return EvolutionProposal{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO evolution_proposals(id,user_id,session_id,run_id,type,title,content,payload,status,version,created_at,updated_at,private_state) VALUES(?,?,?,?,?,'','',?,?,?,?,?,?)`, proposal.ID, proposal.UserID, proposal.SessionID, proposal.RunID, proposal.Type, []byte(privateStatePlaceholder), proposal.Status, proposal.Version, millis(now), millis(now), encrypted); err != nil {
		return EvolutionProposal{}, err
	}
	if err = validateLearningStorage(ctx, tx); err != nil {
		return EvolutionProposal{}, err
	}
	return proposal, tx.Commit()
}

type ProcedureStep struct {
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

func (s *Store) Proposals(ctx context.Context, userID string, status EvolutionStatus) ([]EvolutionProposal, error) {
	tx, err := beginLearningRead(ctx, s.db, "evolution_proposals", userID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	query := `SELECT id,user_id,session_id,run_id,type,CASE WHEN length(private_state)<=10486784 THEN private_state END,status,version,created_at,updated_at FROM evolution_proposals WHERE user_id=?`
	args := []any{userID}
	if status != "" {
		query += ` AND status=?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EvolutionProposal{}
	for rows.Next() {
		var item EvolutionProposal
		var payload []byte
		var created, updated int64
		if err := rows.Scan(&item.ID, &item.UserID, &item.SessionID, &item.RunID, &item.Type, &payload, &item.Status, &item.Version, &created, &updated); err != nil {
			return nil, err
		}
		value, err := s.openLearningState("evolution_proposals", item.ID, item.UserID, payload)
		if err != nil {
			return nil, err
		}
		item.Title, item.Content, item.Payload = value.Title, value.Content, value.Payload
		item.CreatedAt, item.UpdatedAt = fromMillis(created), fromMillis(updated)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) DecideProposal(ctx context.Context, userID, id string, approve bool, validTools map[string]bool) error {
	tx, err := beginLearningWrite(ctx, s.db)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var proposal EvolutionProposal
	var payload []byte
	err = tx.QueryRowContext(ctx, `SELECT id,user_id,type,CASE WHEN length(private_state)<=10486784 THEN private_state END,status,version FROM evolution_proposals WHERE id=? AND user_id=?`, id, userID).Scan(&proposal.ID, &proposal.UserID, &proposal.Type, &payload, &proposal.Status, &proposal.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if proposal.Status != EvolutionPending {
		return ErrConflict
	}
	value, err := s.openLearningState("evolution_proposals", proposal.ID, proposal.UserID, payload)
	if err != nil {
		return err
	}
	proposal.Title, proposal.Content, proposal.Payload = value.Title, value.Content, value.Payload
	payload = value.Payload
	now := s.now().UTC()
	status := EvolutionRejected
	if approve {
		status = EvolutionActive
		switch proposal.Type {
		case EvolutionMemory:
			memoryID := newID("mem")
			encrypted, sealErr := s.sealLearningState("memories", memoryID, userID, learningPrivateState{Title: proposal.Title, Content: proposal.Content})
			if sealErr != nil {
				return sealErr
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO memories(id,user_id,title,content,enabled,version,created_at,updated_at,private_state) VALUES(?,?,'','',1,1,?,?,?)`, memoryID, userID, millis(now), millis(now), encrypted)
		case EvolutionProcedure:
			var body struct {
				Condition string          `json:"condition"`
				Steps     []ProcedureStep `json:"steps"`
			}
			if json.Unmarshal(payload, &body) != nil || len(body.Steps) == 0 || len(body.Steps) > 10 {
				return errors.New("procedure payload is invalid")
			}
			for _, step := range body.Steps {
				if !validTools[step.Tool] {
					return fmt.Errorf("procedure references unknown tool %q", step.Tool)
				}
			}
			encoded, _ := json.Marshal(body.Steps)
			procedureID := newID("proc")
			versionID := newID("pver")
			value := learningPrivateState{Title: proposal.Title, Condition: body.Condition, Steps: encoded}
			encrypted, sealErr := s.sealLearningState("procedures", procedureID, userID, value)
			if sealErr != nil {
				return sealErr
			}
			versionEncrypted, sealErr := s.sealLearningState("procedure_versions", versionID+"\x00"+procedureID, userID, value)
			if sealErr != nil {
				return sealErr
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO procedures(id,user_id,title,condition_text,steps,enabled,version,created_at,updated_at,private_state) VALUES(?,?,'','',?,1,1,?,?,?)`, procedureID, userID, []byte(privateStatePlaceholder), millis(now), millis(now), encrypted)
			if err == nil {
				_, err = tx.ExecContext(ctx, `INSERT INTO procedure_versions(id,procedure_id,user_id,title,condition_text,steps,version,created_at,private_state) VALUES(?,?,?,'','',?,1,?,?)`, versionID, procedureID, userID, []byte(privateStatePlaceholder), millis(now), versionEncrypted)
			}
		default:
			return errors.New("invalid proposal type")
		}
		if err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE evolution_proposals SET status=?,updated_at=? WHERE id=? AND status='pending'`, status, millis(now), id)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return ErrConflict
	}
	if err := validateLearningStorage(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Memories(ctx context.Context, userID string) ([]Memory, error) {
	tx, err := beginLearningRead(ctx, s.db, "memories", userID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,user_id,CASE WHEN length(private_state)<=10486784 THEN private_state END,enabled,retired,version,created_at,updated_at FROM memories WHERE user_id=? ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Memory{}
	for rows.Next() {
		var item Memory
		var encrypted []byte
		var created, updated int64
		if err := rows.Scan(&item.ID, &item.UserID, &encrypted, &item.Enabled, &item.Retired, &item.Version, &created, &updated); err != nil {
			return nil, err
		}
		value, err := s.openLearningState("memories", item.ID, item.UserID, encrypted)
		if err != nil {
			return nil, err
		}
		item.Title, item.Content = value.Title, value.Content
		item.CreatedAt, item.UpdatedAt = fromMillis(created), fromMillis(updated)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateMemory(ctx context.Context, userID, id string, enabled *bool) (Memory, error) {
	items, err := s.Memories(ctx, userID)
	if err != nil {
		return Memory{}, err
	}
	var item *Memory
	for i := range items {
		if items[i].ID == id {
			item = &items[i]
			break
		}
	}
	if item == nil {
		return Memory{}, ErrNotFound
	}
	if item.Retired {
		return Memory{}, ErrConflict
	}
	if enabled != nil {
		item.Enabled = *enabled
	}
	item.Version++
	item.UpdatedAt = s.now().UTC()
	_, err = s.db.ExecContext(ctx, `UPDATE memories SET enabled=?,version=?,updated_at=? WHERE id=? AND user_id=?`, item.Enabled, item.Version, millis(item.UpdatedAt), id, userID)
	return *item, err
}
func (s *Store) DeleteMemory(ctx context.Context, userID, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE memories SET enabled=0,retired=1,version=version+1,updated_at=?
		WHERE id=? AND user_id=? AND retired=0`, millis(s.now()), id, userID)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Procedures(ctx context.Context, userID string) ([]Procedure, error) {
	tx, err := beginLearningRead(ctx, s.db, "procedures", userID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,user_id,CASE WHEN length(private_state)<=10486784 THEN private_state END,enabled,retired,version,created_at,updated_at FROM procedures WHERE user_id=? ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Procedure{}
	for rows.Next() {
		var item Procedure
		var steps []byte
		var created, updated int64
		if err := rows.Scan(&item.ID, &item.UserID, &steps, &item.Enabled, &item.Retired, &item.Version, &created, &updated); err != nil {
			return nil, err
		}
		value, err := s.openLearningState("procedures", item.ID, item.UserID, steps)
		if err != nil {
			return nil, err
		}
		item.Title, item.Condition, item.Steps = value.Title, value.Condition, value.Steps
		item.CreatedAt, item.UpdatedAt = fromMillis(created), fromMillis(updated)
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) UpdateProcedure(ctx context.Context, userID, id string, enabled *bool) (Procedure, error) {
	items, err := s.Procedures(ctx, userID)
	if err != nil {
		return Procedure{}, err
	}
	var item *Procedure
	for i := range items {
		if items[i].ID == id {
			item = &items[i]
			break
		}
	}
	if item == nil {
		return Procedure{}, ErrNotFound
	}
	if item.Retired {
		return Procedure{}, ErrConflict
	}
	if enabled != nil {
		item.Enabled = *enabled
	}
	item.Version++
	item.UpdatedAt = s.now().UTC()
	_, err = s.db.ExecContext(ctx, `UPDATE procedures SET enabled=?,version=?,updated_at=? WHERE id=? AND user_id=?`, item.Enabled, item.Version, millis(item.UpdatedAt), id, userID)
	return *item, err
}

func (s *Store) ReviseProcedure(ctx context.Context, userID, id, title, condition string, steps json.RawMessage, rollbackVersion int) (Procedure, error) {
	items, err := s.Procedures(ctx, userID)
	if err != nil {
		return Procedure{}, err
	}
	var current *Procedure
	for index := range items {
		if items[index].ID == id {
			current = &items[index]
			break
		}
	}
	if current == nil {
		return Procedure{}, ErrNotFound
	}
	if current.Retired {
		return Procedure{}, ErrConflict
	}
	tx, err := beginLearningWrite(ctx, s.db)
	if err != nil {
		return Procedure{}, err
	}
	defer tx.Rollback()
	now := s.now().UTC()
	if rollbackVersion > 0 {
		var version Procedure
		var versionID string
		var raw []byte
		err = tx.QueryRowContext(ctx, `SELECT id,CASE WHEN length(private_state)<=10486784 THEN private_state END,version FROM procedure_versions WHERE procedure_id=? AND user_id=? AND version=?`, id, userID, rollbackVersion).Scan(&versionID, &raw, &version.Version)
		if errors.Is(err, sql.ErrNoRows) {
			return Procedure{}, ErrNotFound
		}
		if err != nil {
			return Procedure{}, err
		}
		value, err := s.openLearningState("procedure_versions", versionID+"\x00"+id, userID, raw)
		if err != nil {
			return Procedure{}, err
		}
		title, condition, steps = value.Title, value.Condition, value.Steps
	}
	if strings.TrimSpace(title) == "" {
		title = current.Title
	}
	if strings.TrimSpace(condition) == "" {
		condition = current.Condition
	}
	if len(steps) == 0 {
		steps = current.Steps
	}
	var parsed []ProcedureStep
	if json.Unmarshal(steps, &parsed) != nil || len(parsed) == 0 || len(parsed) > 10 {
		return Procedure{}, errors.New("procedure steps must contain 1 to 10 registered tool calls")
	}
	newVersion := current.Version + 1
	versionID := newID("pver")
	value := learningPrivateState{Title: title, Condition: condition, Steps: steps}
	encrypted, err := s.sealLearningState("procedures", id, userID, value)
	if err != nil {
		return Procedure{}, err
	}
	versionEncrypted, err := s.sealLearningState("procedure_versions", versionID+"\x00"+id, userID, value)
	if err != nil {
		return Procedure{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE procedures SET private_state=?,version=?,updated_at=? WHERE id=? AND user_id=? AND version=?`, encrypted, newVersion, millis(now), id, userID, current.Version)
	if err != nil {
		return Procedure{}, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return Procedure{}, ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO procedure_versions(id,procedure_id,user_id,title,condition_text,steps,version,created_at,private_state) VALUES(?,?,?,'','',?,?,?,?)`, versionID, id, userID, []byte(privateStatePlaceholder), newVersion, millis(now), versionEncrypted); err != nil {
		return Procedure{}, err
	}
	if err = validateLearningStorage(ctx, tx); err != nil {
		return Procedure{}, err
	}
	if err = tx.Commit(); err != nil {
		return Procedure{}, err
	}
	current.Title, current.Condition, current.Steps, current.Version, current.UpdatedAt = title, condition, append(json.RawMessage(nil), steps...), newVersion, now
	return *current, nil
}
func (s *Store) DeleteProcedure(ctx context.Context, userID, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE procedures SET enabled=0,retired=1,version=version+1,updated_at=?
		WHERE id=? AND user_id=? AND retired=0`, millis(s.now()), id, userID)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return ErrNotFound
	}
	return nil
}
