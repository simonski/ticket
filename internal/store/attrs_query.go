package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
)

// attrsKeyPattern restricts attrs JSON keys usable in SQL (json_extract paths and
// index names) to a safe identifier alphabet. Keys come from a validated allowlist,
// never raw user input concatenated into SQL — this keeps json_extract expressions
// injection-safe.
var attrsKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// attrsIndexableTables is the set of tables that carry an attrs bag and may have
// expression indexes created over it.
var attrsIndexableTables = map[string]bool{
	"tickets":         true,
	"projects":        true,
	"roles":           true,
	"workflow_stages": true,
}

// ValidAttrsKey reports whether key is a safe top-level attrs key for use in a
// json_extract path or an index name.
func ValidAttrsKey(key string) bool {
	return attrsKeyPattern.MatchString(key)
}

// attrsExtractExpr returns a json_extract SQL expression for a single top-level
// attrs key, optionally prefixed by a table alias. The key must be validated with
// ValidAttrsKey by the caller; this returns an error otherwise so a bad key can
// never reach SQL.
func attrsExtractExpr(alias, key string) (string, error) {
	if !ValidAttrsKey(key) {
		return "", fmt.Errorf("invalid attrs key %q", key)
	}
	column := "attrs"
	if alias != "" {
		column = alias + ".attrs"
	}
	return fmt.Sprintf("json_extract(%s, '$.%s')", column, key), nil
}

// EnsureAttrIndex creates, idempotently, an expression index over a single attrs
// JSON key on a table. This is the Tier-3 "promotion" mechanism from
// docs/design/extensible-schema.md: a bag field that needs to be filtered or sorted
// is promoted to query-grade with an index and NO table rewrite or schema-version
// bump. Safe to call repeatedly (CREATE INDEX IF NOT EXISTS).
func EnsureAttrIndex(ctx context.Context, db *sql.DB, table, key string) error {
	if !attrsIndexableTables[table] {
		return fmt.Errorf("unknown attrs table %q", table)
	}
	if !ValidAttrsKey(key) {
		return fmt.Errorf("invalid attrs key %q", key)
	}
	stmt := fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS idx_%s_attrs_%s ON %s (json_extract(attrs, '$.%s'))",
		table, key, table, key,
	)
	_, err := db.ExecContext(ctx, stmt)
	return err
}

// generatedColumnTypes is the allowlist of SQLite column affinities a generated
// column may declare. The type is interpolated into DDL, so it must never come
// from unvalidated input.
var generatedColumnTypes = map[string]bool{
	"TEXT":    true,
	"INTEGER": true,
	"REAL":    true,
}

// GeneratedColumnName returns the canonical column name for the generated column
// promoting attrs key on a table (e.g. "gen_health_score"). The "gen_" prefix
// keeps it from ever colliding with a hand-written Tier-1 column.
func GeneratedColumnName(key string) string {
	return "gen_" + key
}

// EnsureGeneratedColumn promotes a single attrs JSON key to a first-class,
// typed, indexed SQLite generated column — the Tier-3 mechanism from
// docs/design/extensible-schema.md. It idempotently adds a VIRTUAL generated
// column `gen_<key>` defined as json_extract(attrs,'$.<key>') with the given
// affinity, then an index over it. Unlike a Tier-2 expression index
// (EnsureAttrIndex), the value gains a real column surface — a typed handle
// usable anywhere a column is (e.g. typed comparisons, tools that introspect
// columns) — still with NO table rewrite, NO write-path change, and NO
// schema-version bump (the value continues to live in attrs; the column is
// computed). Safe to call repeatedly.
//
// VIRTUAL (not STORED) is used deliberately: SQLite forbids adding a STORED
// generated column via ALTER TABLE, and VIRTUAL adds zero on-disk cost while the
// index provides the lookup speed.
func EnsureGeneratedColumn(ctx context.Context, db *sql.DB, table, key, sqlType string) error {
	if !attrsIndexableTables[table] {
		return fmt.Errorf("unknown attrs table %q", table)
	}
	if !ValidAttrsKey(key) {
		return fmt.Errorf("invalid attrs key %q", key)
	}
	if !generatedColumnTypes[sqlType] {
		return fmt.Errorf("unsupported generated column type %q", sqlType)
	}
	col := GeneratedColumnName(key)
	exists, err := generatedColumnExists(ctx, db, table, col)
	if err != nil {
		return err
	}
	if !exists {
		// #nosec G201 -- table is allowlisted (attrsIndexableTables), key is allowlisted (ValidAttrsKey), sqlType is allowlisted (generatedColumnTypes); no value is interpolated.
		stmt := fmt.Sprintf(
			"ALTER TABLE %s ADD COLUMN %s %s GENERATED ALWAYS AS (json_extract(attrs, '$.%s')) VIRTUAL",
			table, col, sqlType, key,
		)
		if _, execErr := db.ExecContext(ctx, stmt); execErr != nil {
			return execErr
		}
	}
	// #nosec G201 -- identifiers are allowlist-validated as above.
	idx := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_gencol_%s ON %s (%s)", table, key, table, col)
	_, err = db.ExecContext(ctx, idx)
	return err
}

// generatedColumnExists reports whether a table has a column named col, using
// PRAGMA table_xinfo so it also sees VIRTUAL generated columns (which the plain
// PRAGMA table_info does not list).
func generatedColumnExists(ctx context.Context, db *sql.DB, table, col string) (bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_xinfo(?)`, table)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if scanErr := rows.Scan(&name); scanErr != nil {
			return false, scanErr
		}
		if name == col {
			return true, nil
		}
	}
	return false, rows.Err()
}

// ListTicketsByAttr returns the live tickets in a project whose attrs bag has the
// given top-level key equal to value, ordered by that value. It demonstrates the
// queryable bag end-to-end: filtering and sorting on a json_extract expression that
// an EnsureAttrIndex expression index can serve. The key is validated, so the query
// is injection-safe even though the path is interpolated.
func ListTicketsByAttr(ctx context.Context, db *sql.DB, projectID int64, key, value string) ([]Ticket, error) {
	if projectID == 0 {
		return nil, fmt.Errorf("project is required")
	}
	expr, err := attrsExtractExpr("", key)
	if err != nil {
		return nil, err
	}
	// #nosec G202 -- ticketSelectColumns is a fixed column list and expr is built from an allowlist-validated key (ValidAttrsKey); the compared value is a bound parameter.
	query := `SELECT ` + ticketSelectColumns("") + `
		FROM tickets
		WHERE project_id = ? AND deleted = 0 AND ` + expr + ` = ?
		ORDER BY ` + expr + `, ticket_id`
	rows, err := db.QueryContext(ctx, query, projectID, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tickets := make([]Ticket, 0)
	for rows.Next() {
		ticket, scanErr := scanTicket(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}
