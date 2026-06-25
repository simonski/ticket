package store

import (
	"context"
	"strings"
	"testing"
)

func TestValidAttrsKey(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"severity", "wip_limit", "Repro2"} {
		if !ValidAttrsKey(k) {
			t.Errorf("ValidAttrsKey(%q) = false, want true", k)
		}
	}
	for _, k := range []string{"", "a.b", "a'b", "a b", "$.x", "a);DROP"} {
		if ValidAttrsKey(k) {
			t.Errorf("ValidAttrsKey(%q) = true, want false", k)
		}
	}
}

func TestEnsureAttrIndexRejectsBadInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, _ := attrsTestDB(t)
	if err := EnsureAttrIndex(ctx, db, "not_a_table", "x"); err == nil {
		t.Fatal("EnsureAttrIndex with bad table = nil error, want error")
	}
	if err := EnsureAttrIndex(ctx, db, "tickets", "bad key"); err == nil {
		t.Fatal("EnsureAttrIndex with bad key = nil error, want error")
	}
}

// TestListTicketsByAttrUsesIndex demonstrates the queryable bag end-to-end: filter
// and sort tickets on a bag field, and confirm the expression index is used.
func TestListTicketsByAttrUsesIndex(t *testing.T) {
	// Not parallel: creates an index on the shared schema of its own DB.
	ctx := context.Background()
	db, _ := attrsTestDB(t)

	mk := func(title, sev string) {
		if _, err := CreateTicket(ctx, db, TicketCreateParams{
			ProjectID: 1, Type: "bug", Title: title, Attrs: Attrs{"severity": sev},
		}); err != nil {
			t.Fatalf("CreateTicket(%s) error = %v", title, err)
		}
	}
	mk("a", "high")
	mk("b", "low")
	mk("c", "high")

	// Promote the field with an expression index (idempotent; call twice).
	if err := EnsureAttrIndex(ctx, db, "tickets", "severity"); err != nil {
		t.Fatalf("EnsureAttrIndex() error = %v", err)
	}
	if err := EnsureAttrIndex(ctx, db, "tickets", "severity"); err != nil {
		t.Fatalf("EnsureAttrIndex() second call error = %v", err)
	}

	got, err := ListTicketsByAttr(ctx, db, 1, "severity", "high")
	if err != nil {
		t.Fatalf("ListTicketsByAttr() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListTicketsByAttr() returned %d tickets, want 2", len(got))
	}
	for _, tk := range got {
		if tk.Attrs.GetString("severity") != "high" {
			t.Fatalf("unexpected ticket in result: %s severity=%s", tk.Title, tk.Attrs.GetString("severity"))
		}
	}

	// Confirm the planner uses the expression index for the filter.
	var planRows strings.Builder
	rows, err := db.QueryContext(ctx, `EXPLAIN QUERY PLAN
		SELECT ticket_id FROM tickets
		WHERE project_id = ? AND deleted = 0 AND json_extract(attrs, '$.severity') = ?`, 1, "high")
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN error = %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if scanErr := rows.Scan(&id, &parent, &notused, &detail); scanErr != nil {
			t.Fatalf("scan plan error = %v", scanErr)
		}
		planRows.WriteString(detail)
		planRows.WriteString("\n")
	}
	plan := planRows.String()
	if !strings.Contains(plan, "idx_tickets_attrs_severity") {
		t.Fatalf("expression index not used by planner; plan:\n%s", plan)
	}
}

func TestEnsureGeneratedColumnRejectsBadInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, _ := attrsTestDB(t)

	if err := EnsureGeneratedColumn(ctx, db, "not_a_table", "health_score", "INTEGER"); err == nil {
		t.Fatal("expected error for unknown table")
	}
	if err := EnsureGeneratedColumn(ctx, db, "tickets", "bad key", "INTEGER"); err == nil {
		t.Fatal("expected error for invalid attrs key")
	}
	if err := EnsureGeneratedColumn(ctx, db, "tickets", "health_score", "DROP TABLE tickets"); err == nil {
		t.Fatal("expected error for unsupported column type")
	}
}

// TestEnsureGeneratedColumnTypedIndexedQuery promotes the health_score attrs key
// to a typed VIRTUAL generated column, proves creation is idempotent, that the
// column reflects the bag value with INTEGER affinity (typed comparison), and
// that the planner uses the generated-column index for a range query.
func TestEnsureGeneratedColumnTypedIndexedQuery(t *testing.T) {
	// Not parallel: alters the schema of its own DB.
	ctx := context.Background()
	db, _ := attrsTestDB(t)

	mk := func(title string, score int) {
		tk, err := CreateTicket(ctx, db, TicketCreateParams{ProjectID: 1, Type: "task", Title: title})
		if err != nil {
			t.Fatalf("CreateTicket(%s) error = %v", title, err)
		}
		if _, err := db.ExecContext(ctx,
			`UPDATE tickets SET attrs = json_set(attrs, '$.health_score', ?) WHERE ticket_id = ?`, score, tk.ID); err != nil {
			t.Fatalf("seed health_score(%s) error = %v", title, err)
		}
	}
	mk("low", 3)
	mk("mid", 6)
	mk("high", 9)

	// Idempotent: declaring the generated column twice must not error.
	if err := EnsureGeneratedColumn(ctx, db, "tickets", "health_score", "INTEGER"); err != nil {
		t.Fatalf("EnsureGeneratedColumn() error = %v", err)
	}
	if err := EnsureGeneratedColumn(ctx, db, "tickets", "health_score", "INTEGER"); err != nil {
		t.Fatalf("EnsureGeneratedColumn() second call error = %v", err)
	}

	col := GeneratedColumnName("health_score") // gen_health_score
	if exists, err := generatedColumnExists(ctx, db, "tickets", col); err != nil {
		t.Fatalf("generatedColumnExists error = %v", err)
	} else if !exists {
		t.Fatalf("generated column %q was not created", col)
	}

	// Typed (INTEGER) comparison via the generated column: a numeric range query
	// returns the rows with score >= 6, ordered, proving the column carries the
	// bag value with integer affinity (string affinity would mis-sort).
	// #nosec G202 -- col is the allowlist-derived generated column name, no user input.
	rows, err := db.QueryContext(ctx,
		`SELECT title FROM tickets WHERE project_id = 1 AND `+col+` >= 6 ORDER BY `+col)
	if err != nil {
		t.Fatalf("range query error = %v", err)
	}
	defer rows.Close()
	var titles []string
	for rows.Next() {
		var title string
		if scanErr := rows.Scan(&title); scanErr != nil {
			t.Fatalf("scan error = %v", scanErr)
		}
		titles = append(titles, title)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error = %v", err)
	}
	if len(titles) != 2 || titles[0] != "mid" || titles[1] != "high" {
		t.Fatalf("typed range query returned %v, want [mid high]", titles)
	}

	// The planner uses the generated-column index for the range filter.
	var plan strings.Builder
	// #nosec G202 -- col is allowlist-derived.
	planRows, err := db.QueryContext(ctx, `EXPLAIN QUERY PLAN SELECT title FROM tickets WHERE `+col+` >= 6`)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN error = %v", err)
	}
	defer planRows.Close()
	for planRows.Next() {
		var id, parent, notused int
		var detail string
		if scanErr := planRows.Scan(&id, &parent, &notused, &detail); scanErr != nil {
			t.Fatalf("scan plan error = %v", scanErr)
		}
		plan.WriteString(detail)
		plan.WriteString("\n")
	}
	if !strings.Contains(plan.String(), "idx_tickets_gencol_health_score") {
		t.Fatalf("generated-column index not used by planner; plan:\n%s", plan.String())
	}
}
