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
