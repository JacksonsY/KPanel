package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kejilion/kejilion-panel/internal/redact"
)

const maxPrivateStateBytes = 10 << 20

// A legacy reader must reject the private-state format instead of treating
// missing execution arguments as an empty, potentially executable request.
const privateStatePlaceholder = `{"_kpanelPrivateState":true}`

type toolPrivateState struct {
	Arguments    json.RawMessage `json:"arguments"`
	ProviderData json.RawMessage `json:"providerData,omitempty"`
}

func toolPrivateIdentity(call ToolCall) string {
	return "ai-tool-private-v1\x00" + call.SessionID + "\x00" + call.RunID + "\x00" + call.ID + "\x00" + call.Name
}

func (s *Store) sealToolState(call ToolCall) ([]byte, error) {
	raw, err := marshalToolState(call)
	if err != nil {
		return nil, err
	}
	return s.secrets.Seal(toolPrivateIdentity(call), string(raw))
}

// New writes must fit the bounded reader after JSON escaping and encryption.
// The legacy migration keeps its separate original-format ceiling so an old
// oversized record is preserved rather than silently rewritten or truncated.
func (s *Store) sealCurrentToolState(call ToolCall) ([]byte, error) {
	args, size, err := currentToolStateParts(call)
	if err != nil {
		return nil, err
	}
	// These are individually validated JSON values, not strings embedded in
	// HTML. Preserve their original representation: re-escaping '<' in a valid
	// 64 KiB argument could otherwise make the next bounded read reject it.
	raw := make([]byte, 0, size)
	raw = append(raw, `{"arguments":`...)
	raw = append(raw, args...)
	if len(call.ProviderData) > 0 {
		raw = append(raw, `,"providerData":`...)
		raw = append(raw, call.ProviderData...)
	}
	raw = append(raw, '}')
	return s.secrets.Seal(toolPrivateIdentity(call), string(raw))
}

// Reserve the same encoded envelope and XChaCha20-Poly1305 nonce/tag as the
// store before a new batch can execute or enter pending approval.
func toolStateStorageSize(call ToolCall) (int, error) {
	_, size, err := currentToolStateParts(call)
	return size + 40, err
}

func currentToolStateParts(call ToolCall) (json.RawMessage, int, error) {
	if len(call.Arguments) > MaxToolResultBytes || len(call.ProviderData) > maxProviderNativeBytes {
		return nil, 0, ErrContextPrivateLimit
	}
	if err := checkProviderJSONBudget(call.ProviderData, maxProviderNativeBytes); err != nil {
		return nil, 0, err
	}
	args := call.Arguments
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if !json.Valid(args) || (len(call.ProviderData) > 0 && !json.Valid(call.ProviderData)) {
		return nil, 0, errors.New("invalid AI private tool state")
	}
	size := len(`{"arguments":`) + len(args) + 1
	if len(call.ProviderData) > 0 {
		size += len(`,"providerData":`) + len(call.ProviderData)
	}
	if size > maxToolReadStateBytes-64 {
		return nil, 0, ErrContextPrivateLimit
	}
	return args, size, nil
}

func marshalToolState(call ToolCall) ([]byte, error) {
	if len(call.Arguments)+len(call.ProviderData) > maxPrivateStateBytes {
		return nil, errors.New("AI private tool state exceeds the storage budget")
	}
	args := call.Arguments
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	raw, err := json.Marshal(toolPrivateState{Arguments: args, ProviderData: call.ProviderData})
	if err != nil {
		return nil, errors.New("invalid AI private tool state")
	}
	return raw, nil
}

func (s *Store) openToolState(call *ToolCall, encrypted []byte) error {
	if len(encrypted) == 0 || len(encrypted) > maxPrivateStateBytes+1024 {
		return errors.New("invalid AI private tool state")
	}
	plain, err := s.secrets.Open(toolPrivateIdentity(*call), encrypted)
	if err != nil {
		return errors.New("AI private tool state cannot be decrypted")
	}
	var value toolPrivateState
	if json.Unmarshal([]byte(plain), &value) != nil {
		return errors.New("invalid AI private tool state")
	}
	call.Arguments, call.ProviderData = value.Arguments, value.ProviderData
	return nil
}

// Startup owns the database exclusively. Legacy rows are migrated without
// changing status, identity, approval or timestamps. A durable marker is only
// recorded after SQLite has removed obsolete plaintext pages and WAL frames.
// An interrupted migration repeats the scrub before exposing the service.
func (s *Store) protectToolState(ctx context.Context, path string) error {
	count, err := s.EncryptedSecretCount(ctx)
	if err != nil {
		return err
	}
	s.secrets, err = OpenSecretBox(filepath.Join(filepath.Dir(path), "ai-secrets.key"), count > 0)
	if err != nil {
		return err
	}
	if err := s.validateLearningState(ctx); err != nil {
		return err
	}
	// Validate the existing provider key material before encrypting any legacy
	// tools, so a mismatched restored key cannot create a mixed-key database.
	rows, err := s.db.QueryContext(ctx, `SELECT id,CASE WHEN length(encrypted_key)<=5120 THEN encrypted_key END FROM providers WHERE length(encrypted_key)>0`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		var encrypted []byte
		if err := rows.Scan(&id, &encrypted); err != nil {
			rows.Close()
			return err
		}
		if _, err := s.secrets.Open(id, encrypted); err != nil {
			rows.Close()
			return errors.New("AI provider key cannot be decrypted")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var scrubbed int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE version=11`).Scan(&scrubbed); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	markDirty := func() error {
		if scrubbed != 0 {
			if _, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version=11`); err != nil {
				return err
			}
		}
		scrubbed = 0
		return nil
	}
	// Old binaries persisted short API keys verbatim as hints. All ciphertext
	// has already been validated above, so repair copies only after a paired
	// key restore is known to be valid. Repeat after downgrade writes too.
	lastProvider := ""
	for {
		var id string
		var encrypted []byte
		err := tx.QueryRowContext(ctx, `SELECT id,CASE WHEN length(encrypted_key)<=5120 THEN encrypted_key END
		 FROM providers WHERE id>? ORDER BY id LIMIT 1`, lastProvider).Scan(&id, &encrypted)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return err
		}
		lastProvider = id
		key := ""
		if len(encrypted) > 0 {
			key, err = s.secrets.Open(id, encrypted)
			if err != nil {
				return errors.New("AI provider key cannot be decrypted")
			}
		}
		result, err := tx.ExecContext(ctx, `UPDATE providers SET api_key_hint=? WHERE id=? AND api_key_hint<>?`, keyHint(key), id, keyHint(key))
		if err != nil {
			return err
		}
		if changed, err := result.RowsAffected(); err != nil {
			return err
		} else if changed > 0 {
			if err := markDirty(); err != nil {
				return err
			}
		}
		if key != "" {
			// Only copies attributed to this provider are repaired. User chat,
			// attachments and arbitrary generated passwords remain verbatim.
			if err := protectProviderPublicCopies(ctx, tx, id, key, markDirty); err != nil {
				return err
			}
		}
	}
	last := ""
	for {
		var call ToolCall
		var arguments, providerData, encrypted []byte
		var storedBytes int
		err := tx.QueryRowContext(ctx, `SELECT id,run_id,session_id,name,
		 CASE WHEN length(arguments)+length(provider_data)<=10485760 THEN arguments END,
		 CASE WHEN length(arguments)+length(provider_data)<=10485760 THEN provider_data END,
		 CASE WHEN length(private_state)<=10486784 THEN private_state END,
		 length(arguments)+length(provider_data)+length(private_state),
		 CASE WHEN length(result_preview)<=1048576 THEN result_preview END
		 FROM tool_calls WHERE id>? ORDER BY id LIMIT 1`, last).Scan(&call.ID, &call.RunID, &call.SessionID, &call.Name, &arguments, &providerData, &encrypted, &storedBytes, &call.ResultPreview)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return err
		}
		if storedBytes > maxPrivateStateBytes+2048 || (len(encrypted) == 0 && storedBytes > maxPrivateStateBytes) {
			return errors.New("AI private tool state exceeds the storage budget")
		}
		last = call.ID
		safeResult := redactAndLimit(call.ResultPreview, MaxToolResultBytes)
		if safeResult != call.ResultPreview {
			if err := markDirty(); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE tool_calls SET result_preview=? WHERE id=?`, safeResult, call.ID); err != nil {
				return err
			}
		}
		if len(encrypted) > 0 {
			if err := s.openToolState(&call, encrypted); err != nil {
				return err
			}
		} else {
			call.Arguments, call.ProviderData = arguments, providerData
		}
		// A legacy pending call can retain its exact encrypted arguments, but
		// neither preview is allowed to reveal its provider's known credential.
		var providerID string
		var providerKey []byte
		credentialBlocked := false
		err = tx.QueryRowContext(ctx, `SELECT p.id,p.encrypted_key FROM runs r JOIN providers p ON p.id=r.provider_id WHERE r.id=?`, call.RunID).Scan(&providerID, &providerKey)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if len(providerKey) > 0 {
			key, err := s.secrets.Open(providerID, providerKey)
			if err != nil {
				return errors.New("AI provider key cannot be decrypted")
			}
			identityBudget := credentialProjectionBudget{nodes: 262144, limit: 2 << 20}
			for _, value := range []string{call.ID, call.Name} {
				_, found, err := knownCredentialTextBudget(value, key, &identityBudget)
				if err != nil {
					return err
				}
				if found {
					return errors.New("AI legacy tool identity contains provider credential; restore a paired database and key backup before starting AI")
				}
			}
			var oldPreview sql.NullString
			var previewBytes int
			if err := tx.QueryRowContext(ctx, `SELECT CASE WHEN length(CAST(arguments_preview AS BLOB))<=1048576 THEN arguments_preview END,length(CAST(arguments_preview AS BLOB)) FROM tool_calls WHERE id=?`, call.ID).Scan(&oldPreview, &previewBytes); err != nil {
				return err
			}
			if !oldPreview.Valid || previewBytes > MaxAssistantBytes {
				return errors.New("AI tool preview exceeds the migration read budget")
			}
			safePreview, changed, err := knownCredentialTextBudget(oldPreview.String, key, &identityBudget)
			if err != nil {
				return err
			}
			if changed {
				if err := markDirty(); err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx, `UPDATE tool_calls SET arguments_preview=? WHERE id=?`, safePreview, call.ID); err != nil {
					return err
				}
			}
			projectedResult, _, err := knownCredentialTextBudget(call.ResultPreview, key, &identityBudget)
			if err != nil {
				return err
			}
			safeResult := redactAndLimit(publicProviderText(projectedResult, key), MaxToolResultBytes)
			if safeResult != call.ResultPreview {
				if err := markDirty(); err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx, `UPDATE tool_calls SET result_preview=? WHERE id=?`, safeResult, call.ID); err != nil {
					return err
				}
			}
			if containsCredentialJSON(call.Arguments, key) {
				credentialBlocked = true
				var preview string
				if err := tx.QueryRowContext(ctx, `SELECT arguments_preview FROM tool_calls WHERE id=?`, call.ID).Scan(&preview); err != nil {
					return err
				}
				if preview != "[REDACTED]" {
					if err := markDirty(); err != nil {
						return err
					}
					if _, err := tx.ExecContext(ctx, `UPDATE tool_calls SET arguments_preview='[REDACTED]' WHERE id=?`, call.ID); err != nil {
						return err
					}
				}
			}
		}
		if len(encrypted) > 0 {
			continue
		}
		encrypted, err = s.sealToolState(call)
		if err != nil {
			return err
		}
		if err := markDirty(); err != nil {
			return err
		}
		preview := safeToolArgumentsPreview(call.Arguments)
		if credentialBlocked {
			preview = "[REDACTED]"
		}
		if _, err := tx.ExecContext(ctx, `UPDATE tool_calls SET arguments=?,provider_data=X'',private_state=?,arguments_preview=? WHERE id=?`, []byte(privateStatePlaceholder), encrypted, preview, call.ID); err != nil {
			return err
		}
	}
	// Tool results are safe history, unlike the user's original messages and
	// attachments. Re-project historical tool messages with the current decoded
	// JSON rules before an old result can be sent to a provider again.
	last = ""
	for {
		var id, content string
		err := tx.QueryRowContext(ctx, `SELECT id,CASE WHEN length(content)<=1048576 THEN content END FROM messages WHERE role='tool' AND id>? ORDER BY id LIMIT 1`, last).Scan(&id, &content)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return err
		}
		last = id
		safe := redactAndLimit(content, MaxToolResultBytes+1024)
		if safe == content {
			continue
		}
		if err := markDirty(); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE messages SET content=? WHERE id=?`, safe, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if scrubbed == 0 {
		return s.scrubPrivateState(ctx, 11)
	}
	return nil
}

func protectProviderPublicCopies(ctx context.Context, tx *sql.Tx, providerID, key string, markDirty func() error) error {
	for _, target := range []struct{ table, column, ownership string }{
		{"messages", "content", `role IN ('assistant','tool') AND (provider_id=? OR (provider_id='' AND run_id IN (SELECT id FROM runs WHERE provider_id=?)))`},
		{"sessions", "summary", `(provider_id=? OR id IN (SELECT session_id FROM runs WHERE provider_id=?))`},
	} {
		last := ""
		for {
			var id string
			var value sql.NullString
			// Check bytes inside SQLite before a legacy body enters the driver.
			// A JSON-encoded occurrence needs a backslash unless the literal key
			// already appears. Unrelated prose stays in SQLite rather than being
			// copied and scanned again on every startup.
			err := tx.QueryRowContext(ctx, `SELECT id,CASE WHEN length(CAST(`+target.column+` AS BLOB))<=2097152 THEN `+target.column+` END FROM `+target.table+` WHERE id>? AND `+target.ownership+` AND (instr(`+target.column+`,?)>0 OR instr(`+target.column+`,char(92))>0) ORDER BY id LIMIT 1`, last, providerID, providerID, key).Scan(&id, &value)
			if errors.Is(err, sql.ErrNoRows) {
				break
			}
			if err != nil {
				return err
			}
			if !value.Valid {
				return errors.New("AI historical credential projection exceeds the 2 MiB row budget")
			}
			last = id
			safe, changed, err := knownCredentialText(value.String, key)
			if err != nil {
				return err
			}
			if !changed {
				continue
			}
			if err := markDirty(); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE `+target.table+` SET `+target.column+`=? WHERE id=?`, safe, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) scrubPrivateState(ctx context.Context, version int) error {
	var busy, frames, checkpointed int
	if err := s.db.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &frames, &checkpointed); err != nil || busy != 0 {
		return errors.New("AI private state migration could not clear the WAL")
	}
	if _, err := s.db.ExecContext(ctx, `VACUUM`); err != nil {
		return fmt.Errorf("scrub AI legacy state: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &frames, &checkpointed); err != nil || busy != 0 {
		return errors.New("AI private state migration could not clear the WAL")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version,applied_at) VALUES(?,?)`, version, millis(s.now())); err != nil {
		return err
	}
	return nil
}

func safeToolArgumentsPreview(raw json.RawMessage) string {
	return redactAndLimit(redactToolArguments(raw), 4096)
}

func redactToolArguments(raw json.RawMessage) string {
	return projectToolArguments(raw, true)
}

func projectToolArguments(raw json.RawMessage, hideBodies bool) string {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return "[REDACTED]"
	}
	var hide func(any)
	hide = func(v any) {
		switch n := v.(type) {
		case map[string]any:
			for k, item := range n {
				key := strings.ToLower(k)
				switch {
				case hideBodies && (key == "data" || key == "content" || key == "compose" || key == "command"):
					n[k] = "[REDACTED]"
				case key == "command":
					if parts, ok := item.([]any); ok {
						var command []string
						for _, part := range parts {
							if text, ok := part.(string); ok {
								command = append(command, text)
							}
						}
						// Inspect flags across adjacent argv elements before showing
						// the command. This string is never used for execution.
						n[k] = redact.Text(strings.Join(command, " "))
					}
				default:
					hide(item)
				}
			}
		case []any:
			for _, item := range n {
				hide(item)
			}
		}
	}
	hide(value)
	encoded, _ := json.Marshal(value)
	safe, err := redact.JSON(encoded)
	if err != nil {
		return "[REDACTED]"
	}
	return string(safe)
}

// Approval details exist only in the authenticated pending-call representation;
// they are never a second persisted copy of execution arguments. Native provider
// context is excluded even here. The administrator must be able to inspect the
// concrete command/write, with supported credential patterns redacted.
func (call ToolCall) MarshalJSON() ([]byte, error) {
	type publicToolCall ToolCall
	preview := ""
	if call.Status == ToolPendingApproval {
		preview = redactAndLimit(projectToolArguments(call.Arguments, false), MaxToolResultBytes)
		if call.ArgumentsPreview == "[REDACTED]" {
			preview = "[REDACTED]"
		}
	}
	return json.Marshal(struct {
		publicToolCall
		ApprovalPreview string `json:"approvalPreview,omitempty"`
	}{publicToolCall(call), preview})
}
