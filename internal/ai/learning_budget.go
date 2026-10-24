package ai

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

const (
	maxLearningRecords     = 2048
	maxLearningBytes       = 32 << 20
	maxUserLearningRecords = 1024
	maxUserLearningBytes   = 16 << 20
	maxLearningListRecords = 512
	maxLearningListBytes   = 8 << 20
	maxLearningStateBytes  = 4 << 20
	maxLearningRowBytes    = maxLearningStateBytes + 4096
)

var ErrLearningBudget = errors.New("AI learning capacity exceeded; existing records are retained, stop adding learning data and restore a bounded database backup or review the data offline")

type learningUsage struct{ records, bytes int }
type learningBudget struct {
	total learningUsage
	users map[string]learningUsage
	lists map[string]learningUsage
}

// Count stored bytes, including legacy plaintext, before copying any private
// body. The allowance covers bounded scalar fields/row overhead; it is not a
// promise about SQLite page size or physical media reclamation.
func learningRowBytesSQL(table string) string {
	columns := []string{"id", "user_id", "title", "private_state"}
	switch table {
	case "evolution_proposals":
		columns = append(columns, "session_id", "run_id", "type", "content", "payload")
	case "memories":
		columns = append(columns, "content")
	case "procedures":
		columns = append(columns, "condition_text", "steps")
	case "procedure_versions":
		columns = append(columns, "procedure_id", "condition_text", "steps")
	default:
		panic("unknown learning budget table")
	}
	parts := []string{"128"}
	for _, column := range columns {
		parts = append(parts, "length(CAST("+column+" AS BLOB))")
	}
	return strings.Join(parts, "+")
}

func learningListKey(table, user string) string { return table + "\x00" + user }

func learningInvalidMetadataSQL(table string) string {
	invalid := "length(CAST(id AS BLOB))>256 OR length(CAST(user_id AS BLOB))>256"
	if table == "evolution_proposals" {
		// Status changes use the fixed allowance, so rejecting a pending
		// proposal never consumes another data byte.
		invalid += " OR length(CAST(session_id AS BLOB))>256 OR length(CAST(run_id AS BLOB))>256 OR length(CAST(type AS BLOB))>32 OR length(CAST(status AS BLOB))>32"
	} else if table == "procedure_versions" {
		invalid += " OR length(CAST(procedure_id AS BLOB))>256"
	}
	return invalid
}

func (b *learningBudget) reserve(table, user string, oldBytes, newBytes, added int) error {
	if newBytes < 0 {
		return ErrLearningBudget
	}
	delta := newBytes - oldBytes
	total := learningUsage{b.total.records + added, b.total.bytes + delta}
	owned := b.users[user]
	owned = learningUsage{owned.records + added, owned.bytes + delta}
	key := learningListKey(table, user)
	list := b.lists[key]
	list = learningUsage{list.records + added, list.bytes + delta}
	if total.records > maxLearningRecords || total.bytes > maxLearningBytes ||
		owned.records > maxUserLearningRecords || owned.bytes > maxUserLearningBytes ||
		(table != "procedure_versions" && (list.records > maxLearningListRecords || list.bytes > maxLearningListBytes)) {
		return ErrLearningBudget
	}
	b.total, b.users[user], b.lists[key] = total, owned, list
	return nil
}

func readLearningBudget(ctx context.Context, tx *sql.Tx) (*learningBudget, error) {
	b := &learningBudget{users: map[string]learningUsage{}, lists: map[string]learningUsage{}}
	// Bound row work before aggregate scans or grouping on any legacy value.
	for _, table := range learningTables {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM (SELECT 1 FROM `+table.table+` LIMIT ?)`, maxLearningRecords+1).Scan(&count); err != nil {
			return nil, err
		}
		b.total.records += count
		if b.total.records > maxLearningRecords {
			return nil, ErrLearningBudget
		}
	}
	b.total.records = 0
	for _, table := range learningTables {
		var invalid bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM `+table.table+` WHERE `+learningInvalidMetadataSQL(table.table)+` OR length(private_state)>? OR (`+learningRowBytesSQL(table.table)+`)>?)`, maxLearningStateBytes, maxLearningRowBytes).Scan(&invalid); err != nil {
			return nil, err
		}
		if invalid {
			return nil, ErrLearningBudget
		}
		// Return only length metadata. Grouping raw learning rows could put
		// ciphertext columns into SQLite's temporary aggregate/sort records.
		rows, err := tx.QueryContext(ctx, `SELECT user_id,`+learningRowBytesSQL(table.table)+` FROM `+table.table)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var user string
			var bytes int
			if err = rows.Scan(&user, &bytes); err == nil {
				err = b.reserve(table.table, user, 0, bytes, 1)
			}
			if err != nil {
				rows.Close()
				return nil, err
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

func validateLearningStorage(ctx context.Context, tx *sql.Tx) error {
	_, err := readLearningBudget(ctx, tx)
	return err
}

func beginLearningWrite(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	// Acquire SQLite's writer reservation before reading a shared quota. Schema
	// version 1 exists even while a later private-state scrub is still pending.
	if _, err = tx.ExecContext(ctx, `UPDATE schema_migrations SET applied_at=applied_at WHERE version=1`); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err = validateLearningStorage(ctx, tx); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func beginLearningRead(ctx context.Context, db *sql.DB, table, user string) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	var records, bytes, largest, ciphertext int
	var invalid bool
	err = tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(sum(`+learningRowBytesSQL(table)+`),0),COALESCE(max(`+learningRowBytesSQL(table)+`),0),COALESCE(max(length(private_state)),0),COALESCE(max(CASE WHEN `+learningInvalidMetadataSQL(table)+` THEN 1 ELSE 0 END),0) FROM `+table+` WHERE user_id=?`, user).Scan(&records, &bytes, &largest, &ciphertext, &invalid)
	if err == nil && (invalid || records > maxLearningListRecords || bytes > maxLearningListBytes || largest > maxLearningRowBytes || ciphertext > maxLearningStateBytes) {
		err = ErrLearningBudget
	}
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}
