package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const SnapshotSchemaVersion = "ticket.schema.v1"

type SnapshotTable struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

type Snapshot struct {
	SchemaVersion string                   `json:"schema_version"`
	ExportedAt    string                   `json:"exported_at"`
	Tables        map[string]SnapshotTable `json:"tables"`
}

var snapshotTableOrder = []string{
	"users",
	"sessions",
	"agents",
	"roles",
	"projects",
	"tickets",
	"stories",
	"story_ticket_links",
	"history_events",
	"ticket_history",
	"comments",
	"dependencies",
	"app_settings",
	"project_members",
	"teams",
	"team_members",
	"team_agents",
	"project_teams",
}

func ExportSnapshot(db *sql.DB) (Snapshot, error) {
	if db == nil {
		return Snapshot{}, errors.New("database is required")
	}
	snapshot := Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		Tables:        make(map[string]SnapshotTable, len(snapshotTableOrder)),
	}
	for _, table := range snapshotTableOrder {
		columns, err := tableColumnNames(db, table)
		if err != nil {
			return Snapshot{}, err
		}
		selectCols := make([]string, 0, len(columns))
		for _, column := range columns {
			selectCols = append(selectCols, quoteIdentifier(column))
		}
		rows, err := db.Query(`SELECT ` + strings.Join(selectCols, ", ") + ` FROM ` + quoteIdentifier(table))
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
				_ = rows.Close()
				return Snapshot{}, err
			}
			row := make([]any, len(columns))
			for i, value := range scanned {
				row[i] = normalizeExportValue(value)
			}
			tableRows = append(tableRows, row)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return Snapshot{}, err
		}
		_ = rows.Close()
		snapshot.Tables[table] = SnapshotTable{
			Columns: append([]string{}, columns...),
			Rows:    tableRows,
		}
	}
	return snapshot, nil
}

func ImportSnapshot(db *sql.DB, snapshot Snapshot) error {
	if db == nil {
		return errors.New("database is required")
	}
	if strings.TrimSpace(snapshot.SchemaVersion) != SnapshotSchemaVersion {
		return fmt.Errorf("unsupported snapshot schema version %q", strings.TrimSpace(snapshot.SchemaVersion))
	}
	if snapshot.Tables == nil {
		return errors.New("snapshot tables are required")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	rollback := func(cause error) error {
		_ = tx.Rollback()
		return cause
	}
	if _, err := tx.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return rollback(err)
	}
	for i := len(snapshotTableOrder) - 1; i >= 0; i-- {
		table := snapshotTableOrder[i]
		if _, err := tx.Exec(`DELETE FROM ` + quoteIdentifier(table)); err != nil {
			return rollback(err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM sqlite_sequence`); err != nil {
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
		insertCols := make([]string, 0, len(tableSnapshot.Columns))
		placeholders := make([]string, 0, len(tableSnapshot.Columns))
		for _, column := range tableSnapshot.Columns {
			insertCols = append(insertCols, quoteIdentifier(column))
			placeholders = append(placeholders, "?")
		}
		query := `INSERT INTO ` + quoteIdentifier(table) + ` (` + strings.Join(insertCols, ", ") + `) VALUES (` + strings.Join(placeholders, ", ") + `)`
		for rowIdx, row := range tableSnapshot.Rows {
			if len(row) != len(tableSnapshot.Columns) {
				return rollback(fmt.Errorf("snapshot row mismatch in %s at index %d: got %d values, want %d", table, rowIdx, len(row), len(tableSnapshot.Columns)))
			}
			args := make([]any, len(row))
			for i, value := range row {
				args[i] = normalizeImportValue(value)
			}
			if _, err := tx.Exec(query, args...); err != nil {
				return rollback(err)
			}
		}
	}
	if _, err := tx.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return rollback(err)
	}
	fkRows, err := tx.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return rollback(err)
	}
	defer fkRows.Close()
	if fkRows.Next() {
		var table string
		var rowID any
		var parent string
		var fkID any
		if err := fkRows.Scan(&table, &rowID, &parent, &fkID); err != nil {
			return rollback(err)
		}
		return rollback(fmt.Errorf("foreign key violation after import in table %s", table))
	}
	if err := fkRows.Err(); err != nil {
		return rollback(err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func tableColumnNames(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query(`PRAGMA table_info(` + quoteIdentifier(table) + `)`)
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
