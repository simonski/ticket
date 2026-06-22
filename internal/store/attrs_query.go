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
