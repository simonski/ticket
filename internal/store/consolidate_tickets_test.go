package store

import (
	"context"
	"database/sql"
	"testing"
)

// TestConsolidateTicketsMigrationNoDataLoss simulates a pre-consolidation (v11)
// tickets table — the soft columns still present and populated — and verifies the
// upgrade moves every value into the attrs bag with no data loss, drops the
// columns, and that the values are still readable via the typed Ticket fields.
func TestConsolidateTicketsMigrationNoDataLoss(t *testing.T) {
	// Not parallel: manipulates schema_version and raw columns.
	ctx := context.Background()
	db, path := attrsTestDB(t)
	tk, err := CreateTicket(ctx, db, TicketCreateParams{ProjectID: 1, Type: "task", Title: "to-migrate"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	// Recreate the pre-consolidation shape.
	for _, stmt := range []string{
		`ALTER TABLE tickets ADD COLUMN git_repository TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE tickets ADD COLUMN git_branch TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE tickets ADD COLUMN estimate_complete TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE tickets ADD COLUMN health_score INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE tickets ADD COLUMN author TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE tickets ADD COLUMN pr_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE tickets ADD COLUMN open INTEGER NOT NULL DEFAULT 1`,
	} {
		if _, execErr := raw.Exec(stmt); execErr != nil {
			_ = raw.Close()
			t.Fatalf("setup %q error = %v", stmt, execErr)
		}
	}
	if _, execErr := raw.Exec(
		`UPDATE tickets SET git_repository=?, git_branch=?, estimate_complete=?, health_score=?, author=?, pr_url=?, attrs='{}' WHERE ticket_id=?`,
		"git@example.com:x.git", "feature/x", "2026-01-15", 42, "alice", "https://pr/1", tk.ID,
	); execErr != nil {
		_ = raw.Close()
		t.Fatalf("populate error = %v", execErr)
	}
	if _, execErr := raw.Exec(`UPDATE schema_meta SET value='11' WHERE key='schema_version'`); execErr != nil {
		_ = raw.Close()
		t.Fatalf("set version error = %v", execErr)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("raw.Close() error = %v", err)
	}

	if _, err := UpgradeInPlace(ctx, path); err != nil {
		t.Fatalf("UpgradeInPlace() error = %v", err)
	}

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Open() after upgrade error = %v", err)
	}
	defer db2.Close()

	got, err := GetTicket(ctx, db2, tk.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if got.GitRepository != "git@example.com:x.git" {
		t.Errorf("GitRepository = %q, want preserved", got.GitRepository)
	}
	if got.GitBranch != "feature/x" {
		t.Errorf("GitBranch = %q, want preserved", got.GitBranch)
	}
	if got.EstimateComplete != "2026-01-15" {
		t.Errorf("EstimateComplete = %q, want preserved", got.EstimateComplete)
	}
	if got.HealthScore != 42 {
		t.Errorf("HealthScore = %d, want 42", got.HealthScore)
	}
	if got.Author != "alice" {
		t.Errorf("Author = %q, want alice", got.Author)
	}
	if got.PrURL != "https://pr/1" {
		t.Errorf("PrURL = %q, want preserved", got.PrURL)
	}

	// Columns must be gone after consolidation.
	for _, col := range []string{"git_repository", "git_branch", "estimate_complete", "health_score", "author", "pr_url", "open"} {
		if columnExists(ctx, db2, "tickets", col) {
			t.Errorf("tickets.%s still exists after consolidation", col)
		}
	}
}

// TestConsolidatedTicketFieldsRoundTrip verifies the soft fields still work
// end-to-end through the typed Ticket fields after consolidation, and that the
// user-visible Attrs bag does not duplicate them.
func TestConsolidatedTicketFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, _ := attrsTestDB(t)

	tk, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: 1, Type: "bug", Title: "rt",
		GitRepository: "git@x", GitBranch: "main", Author: "bob",
		Attrs: Attrs{"severity": "high"},
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	got, err := GetTicket(ctx, db, tk.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if got.GitRepository != "git@x" || got.GitBranch != "main" || got.Author != "bob" {
		t.Fatalf("typed soft fields not round-tripped: %+v", got)
	}
	// The extra bag keeps the genuinely-extra key but not the typed/back-compat ones.
	if got.Attrs.GetString("severity") != "high" {
		t.Fatalf("extra attr lost: %#v", got.Attrs)
	}
	if _, dup := got.Attrs["git_repository"]; dup {
		t.Fatalf("attrs duplicates a typed field: %#v", got.Attrs)
	}

	// Dedicated setters write through to the bag.
	if _, err := SetTicketPrURL(ctx, db, tk.ID, "https://pr/9"); err != nil {
		t.Fatalf("SetTicketPrURL() error = %v", err)
	}
	after, err := GetTicket(ctx, db, tk.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if after.PrURL != "https://pr/9" {
		t.Fatalf("PrURL via setter = %q, want https://pr/9", after.PrURL)
	}
}
