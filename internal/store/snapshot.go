package store

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

type queryContexter interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

const SnapshotSchemaVersion = "ticket.schema.v1"

type SnapshotTable struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

type Snapshot struct {
	SchemaVersion string                   `json:"schema_version"`
	ExportedAt    string                   `json:"exported_at"`
	Signature     string                   `json:"signature,omitempty"`
	Tables        map[string]SnapshotTable `json:"tables"`
}

var snapshotTableOrder = []string{
	"users",
	"sessions",
	"roles",
	"workflows",
	"workflow_stages",
	"workflow_stage_roles",
	"projects",
	"goals",
	"tickets",
	"stories",
	"story_ticket_links",
	"history_events",
	"ticket_history",
	"comments",
	"labels",
	"ticket_labels",
	"time_entries",
	"dependencies",
	"app_settings",
	"project_members",
	"teams",
	"team_members",
	"team_agents",
	"project_teams",
	"messages",
	"agent_config",
}

func ExportSnapshot(ctx context.Context, db *sql.DB) (Snapshot, error) {
	if db == nil {
		return Snapshot{}, errors.New("database is required")
	}
	snapshot := Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		Tables:        make(map[string]SnapshotTable, len(snapshotTableOrder)),
	}
	for _, table := range snapshotTableOrder {
		columns, err := tableColumnNames(ctx, db, table)
		if err != nil {
			return Snapshot{}, err
		}
		selectCols := make([]string, 0, len(columns))
		for _, column := range columns {
			selectCols = append(selectCols, quoteIdentifier(column))
		}
		rows, err := db.QueryContext(ctx, `SELECT `+strings.Join(selectCols, ", ")+` FROM `+quoteIdentifier(table)) // #nosec G202 -- column and table names from live schema via quoteIdentifier
		if err != nil {
			return Snapshot{}, err
		}
		tableRows := make([][]any, 0)
		for rows.Next() {
			scanned := make([]any, len(columns))
			target := make([]any, len(columns))
			for i := range target {
				target[i] = &scanned[i]
			}
			if err := rows.Scan(target...); err != nil {
				if closeErr := rows.Close(); closeErr != nil {
					log.Printf("store: close snapshot rows after scan failure (table=%s): %v", table, closeErr)
				}
				return Snapshot{}, err
			}
			row := make([]any, len(columns))
			for i, value := range scanned {
				row[i] = normalizeExportValue(value)
			}
			tableRows = append(tableRows, row)
		}
		if err := rows.Err(); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				log.Printf("store: close snapshot rows after iteration failure (table=%s): %v", table, closeErr)
			}
			return Snapshot{}, err
		}
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("store: close snapshot rows (table=%s): %v", table, closeErr)
		}
		snapshot.Tables[table] = SnapshotTable{
			Columns: append([]string{}, columns...),
			Rows:    tableRows,
		}
	}
	if signature, err := signSnapshot(snapshot); err != nil {
		return Snapshot{}, err
	} else if signature != "" {
		snapshot.Signature = signature
	}
	return snapshot, nil
}

func ImportSnapshot(ctx context.Context, db *sql.DB, snapshot Snapshot) error {
	if db == nil {
		return errors.New("database is required")
	}
	if strings.TrimSpace(snapshot.SchemaVersion) != SnapshotSchemaVersion {
		return fmt.Errorf("unsupported snapshot schema version %q", strings.TrimSpace(snapshot.SchemaVersion))
	}
	if snapshot.Tables == nil {
		return errors.New("snapshot tables are required")
	}
	if err := verifySnapshotSignature(snapshot); err != nil {
		return err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	restoreForeignKeys := func() {
		_, _ = conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`)
	}
	defer restoreForeignKeys()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	rollback := func(cause error) error {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Printf("store: rollback import snapshot transaction: %v", rollbackErr)
		}
		return cause
	}
	for i := len(snapshotTableOrder) - 1; i >= 0; i-- {
		table := snapshotTableOrder[i]
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+quoteIdentifier(table)); err != nil { // #nosec G202 -- table name from hardcoded snapshotTableOrder via quoteIdentifier
			return rollback(err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sqlite_sequence`); err != nil {
		return rollback(err)
	}
	for _, table := range snapshotTableOrder {
		tableSnapshot, ok := snapshot.Tables[table]
		if !ok {
			continue
		}
		if len(tableSnapshot.Rows) == 0 {
			continue
		}
		if len(tableSnapshot.Columns) == 0 {
			return rollback(fmt.Errorf("snapshot table %s has rows but no columns", table))
		}
		targetColumns, err := tableColumnNamesQuery(ctx, tx, table)
		if err != nil {
			return rollback(err)
		}
		targetSet := make(map[string]struct{}, len(targetColumns))
		for _, column := range targetColumns {
			targetSet[column] = struct{}{}
		}
		insertCols := make([]string, 0, len(tableSnapshot.Columns))
		placeholders := make([]string, 0, len(tableSnapshot.Columns))
		keepIndexes := make([]int, 0, len(tableSnapshot.Columns))
		for i, column := range tableSnapshot.Columns {
			if _, ok := targetSet[column]; !ok {
				continue
			}
			insertCols = append(insertCols, quoteIdentifier(column))
			placeholders = append(placeholders, "?")
			keepIndexes = append(keepIndexes, i)
		}
		if len(insertCols) == 0 {
			continue
		}
		query := `INSERT INTO ` + quoteIdentifier(table) + ` (` + strings.Join(insertCols, ", ") + `) VALUES (` + strings.Join(placeholders, ", ") + `)` // #nosec G202 -- table/column names from snapshot schema via quoteIdentifier
		for rowIdx, row := range tableSnapshot.Rows {
			if len(row) != len(tableSnapshot.Columns) {
				return rollback(fmt.Errorf("snapshot row mismatch in %s at index %d: got %d values, want %d", table, rowIdx, len(row), len(tableSnapshot.Columns)))
			}
			args := make([]any, len(keepIndexes))
			for i, idx := range keepIndexes {
				args[i] = normalizeImportValue(row[idx])
			}
			if _, err := tx.ExecContext(ctx, query, args...); err != nil {
				return rollback(err)
			}
		}
	}
	if err := pruneForeignKeyViolations(ctx, tx); err != nil {
		return rollback(err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func tableColumnNames(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	return tableColumnNamesQuery(ctx, db, table)
}

func tableColumnNamesQuery(ctx context.Context, db queryContexter, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+quoteIdentifier(table)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make([]string, 0, 8)
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("table not found: %s", table)
	}
	return columns, nil
}

func pruneForeignKeyViolations(ctx context.Context, tx *sql.Tx) error {
	for attempts := 0; attempts < 1000; attempts++ {
		fkRows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
		if err != nil {
			return err
		}
		type violation struct {
			table string
			rowID int64
		}
		violations := make([]violation, 0)
		for fkRows.Next() {
			var table string
			var rowID any
			var parent string
			var fkID any
			if err := fkRows.Scan(&table, &rowID, &parent, &fkID); err != nil {
				_ = fkRows.Close()
				return err
			}
			parsedRowID, ok := normalizeSQLiteRowID(rowID)
			if !ok {
				_ = fkRows.Close()
				return fmt.Errorf("foreign key violation after import in table %s", table)
			}
			violations = append(violations, violation{table: table, rowID: parsedRowID})
		}
		if err := fkRows.Err(); err != nil {
			_ = fkRows.Close()
			return err
		}
		if err := fkRows.Close(); err != nil {
			return err
		}
		if len(violations) == 0 {
			return nil
		}
		for _, violation := range violations {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+quoteIdentifier(violation.table)+` WHERE rowid = ?`, violation.rowID); err != nil { // #nosec G202 -- table name comes from sqlite foreign_key_check output via quoteIdentifier
				return err
			}
		}
	}
	return errors.New("foreign key violations could not be resolved during import")
}

func normalizeSQLiteRowID(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case []byte:
		parsed, err := strconv.ParseInt(strings.TrimSpace(string(typed)), 10, 64)
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func signSnapshot(snapshot Snapshot) (string, error) {
	key, err := encryptionKey()
	if err != nil {
		return "", err
	}
	if len(key) == 0 {
		return "", nil
	}
	payload, err := snapshotSignaturePayload(snapshot)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write(payload); err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func verifySnapshotSignature(snapshot Snapshot) error {
	if strings.TrimSpace(snapshot.Signature) == "" {
		return nil
	}
	expected, err := signSnapshot(snapshot)
	if err != nil {
		return err
	}
	if expected == "" {
		return errors.New("TICKET_ENCRYPTION_KEY required to verify snapshot signature")
	}
	if !hmac.Equal([]byte(expected), []byte(strings.TrimSpace(snapshot.Signature))) {
		return errors.New("snapshot signature verification failed")
	}
	return nil
}

func snapshotSignaturePayload(snapshot Snapshot) ([]byte, error) {
	unsigned := Snapshot{
		SchemaVersion: snapshot.SchemaVersion,
		ExportedAt:    snapshot.ExportedAt,
		Tables:        snapshot.Tables,
	}
	return json.Marshal(unsigned)
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func normalizeExportValue(value any) any {
	switch typed := value.(type) {
	case []byte:
		return string(typed)
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano)
	default:
		return typed
	}
}

func normalizeImportValue(value any) any {
	switch typed := value.(type) {
	case json.Number:
		raw := typed.String()
		if strings.ContainsAny(raw, ".eE") {
			if parsed, err := typed.Float64(); err == nil {
				return parsed
			}
		}
		if parsed, err := typed.Int64(); err == nil {
			return parsed
		}
		return raw
	case float64:
		if math.Trunc(typed) == typed {
			return int64(typed)
		}
		return typed
	default:
		return typed
	}
}
