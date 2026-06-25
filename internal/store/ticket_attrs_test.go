package store

import (
	"context"
	"testing"
)

// TestTicketAttrScalarRegistryRoundTrip verifies the declare-once registry
// (TK-173) round-trips every scalar field: typed Ticket -> attrs JSON (write)
// -> typed Ticket (hydrate) returns the original values, and that empty/zero
// values stay sparse (absent from the bag).
func TestTicketAttrScalarRegistryRoundTrip(t *testing.T) {
	src := &Ticket{
		GitRepository:    "github.com/acme/repo.git",
		GitBranch:        "feature/x",
		EstimateComplete: "2026-07-01",
		Author:           "simon",
		PrURL:            "https://example/pr/1",
		HealthScore:      7,
	}
	jsonText, err := ticketAttrsForWrite(nil, src)
	if err != nil {
		t.Fatalf("ticketAttrsForWrite: %v", err)
	}
	parsed, err := parseAttrs(jsonText)
	if err != nil {
		t.Fatalf("parseAttrs: %v", err)
	}
	got := &Ticket{Attrs: parsed}
	hydrateTicketAttrs(got)

	if got.GitRepository != src.GitRepository ||
		got.GitBranch != src.GitBranch ||
		got.EstimateComplete != src.EstimateComplete ||
		got.Author != src.Author ||
		got.PrURL != src.PrURL ||
		got.HealthScore != src.HealthScore {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, src)
	}
	// Bag-backed keys are consumed into typed fields, so the user-visible bag is
	// empty (nil) after hydration.
	if got.Attrs != nil {
		t.Fatalf("expected empty Attrs after hydration, got %v", got.Attrs)
	}
}

// TestTicketAttrScalarSparse confirms empty/zero scalars are not persisted, so
// the stored object stays minimal.
func TestTicketAttrScalarSparse(t *testing.T) {
	jsonText, err := ticketAttrsForWrite(nil, &Ticket{})
	if err != nil {
		t.Fatalf("ticketAttrsForWrite: %v", err)
	}
	if jsonText != "{}" {
		t.Fatalf("expected empty bag {} for zero-value ticket, got %q", jsonText)
	}
	parsed, err := parseAttrs(jsonText)
	if err != nil {
		t.Fatalf("parseAttrs: %v", err)
	}
	for _, k := range ticketAttrScalarKeys() {
		if _, ok := parsed[k]; ok {
			t.Fatalf("zero-value field %q should be absent from bag, got present", k)
		}
	}
}

// TestTicketAttrScalarBasePreserved verifies non-declared base bag entries are
// preserved through a write (the registry only manages declared keys).
func TestTicketAttrScalarBasePreserved(t *testing.T) {
	base := Attrs{"custom_thing": "keepme"}
	jsonText, err := ticketAttrsForWrite(base, &Ticket{GitBranch: "b"})
	if err != nil {
		t.Fatalf("ticketAttrsForWrite: %v", err)
	}
	parsed, err := parseAttrs(jsonText)
	if err != nil {
		t.Fatalf("parseAttrs: %v", err)
	}
	if parsed.GetString("custom_thing") != "keepme" {
		t.Fatalf("base bag entry lost; got %v", parsed)
	}
	if parsed.GetString("git_branch") != "b" {
		t.Fatalf("declared field not written; got %v", parsed)
	}
}

// TestMigrateRecommendedReadyToAttrs proves the TK-174 migration moves a set
// recommended_ready column value into attrs losslessly and drops the column,
// while preserving row count (the reference-DB lossless guarantee, in miniature).
func TestMigrateRecommendedReadyToAttrs(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)

	// Recreate the pre-migration shape: a recommended_ready column with a set row.
	if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN recommended_ready INTEGER NOT NULL DEFAULT 0`); err != nil {
		t.Fatalf("re-add column: %v", err)
	}
	project, err := CreateProject(ctx, db, "Attrs Migration", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	pid := project.ID
	set, err := CreateTicket(ctx, db, TicketCreateParams{ProjectID: pid, Type: "task", Title: "set"})
	if err != nil {
		t.Fatalf("create set ticket: %v", err)
	}
	unset, err := CreateTicket(ctx, db, TicketCreateParams{ProjectID: pid, Type: "task", Title: "unset"})
	if err != nil {
		t.Fatalf("create unset ticket: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET recommended_ready = 1 WHERE ticket_id = ?`, set.ID); err != nil {
		t.Fatalf("seed recommended_ready: %v", err)
	}

	var before int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets`).Scan(&before); err != nil {
		t.Fatalf("count before: %v", err)
	}

	if err := migrateSchema(ctx, db); err != nil {
		t.Fatalf("migrateSchema: %v", err)
	}

	// Column is gone.
	if columnExists(ctx, db, "tickets", "recommended_ready") {
		t.Fatalf("recommended_ready column should have been dropped")
	}
	// Row count preserved (lossless).
	var after int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets`).Scan(&after); err != nil {
		t.Fatalf("count after: %v", err)
	}
	if after != before {
		t.Fatalf("row count changed by migration: before=%d after=%d", before, after)
	}
	// Set value carried into attrs and hydrates to true; unset stays sparse/false.
	gotSet, err := GetTicket(ctx, db, set.ID)
	if err != nil {
		t.Fatalf("get set: %v", err)
	}
	if !gotSet.RecommendedReady {
		t.Fatalf("migrated ticket should be RecommendedReady=true")
	}
	gotUnset, err := GetTicket(ctx, db, unset.ID)
	if err != nil {
		t.Fatalf("get unset: %v", err)
	}
	if gotUnset.RecommendedReady {
		t.Fatalf("unset ticket should be RecommendedReady=false")
	}
}

// TestRecommendedReadyRoundTripViaAttrs verifies the live write/read paths now
// route through attrs: SetRecommendedReady true then false, and that an
// unrelated UpdateTicket preserves a set flag (does not strip it).
func TestRecommendedReadyRoundTripViaAttrs(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	project, err := CreateProject(ctx, db, "Attrs Migration", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	pid := project.ID

	tk, err := CreateTicket(ctx, db, TicketCreateParams{ProjectID: pid, Type: "task", Title: "rr"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := SetRecommendedReady(ctx, db, tk.ID, true, "tester", testAdminID(t, db)); err != nil {
		t.Fatalf("set true: %v", err)
	}
	got, err := GetTicket(ctx, db, tk.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.RecommendedReady {
		t.Fatalf("expected RecommendedReady true after set")
	}
	// The key is stored in the row's attrs (sparse, value 1).
	var raw string
	if err := db.QueryRowContext(ctx, `SELECT json_extract(attrs, '$.recommended_ready') FROM tickets WHERE ticket_id = ?`, tk.ID).Scan(&raw); err != nil {
		t.Fatalf("json_extract: %v", err)
	}
	if raw != "1" {
		t.Fatalf("attrs.recommended_ready = %q, want \"1\"", raw)
	}

	// An unrelated update must preserve the flag.
	if _, err := UpdateTicket(ctx, db, tk.ID, TicketUpdateParams{Title: "rr-renamed", Description: got.Description, Priority: got.Priority}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err = GetTicket(ctx, db, tk.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if !got.RecommendedReady {
		t.Fatalf("UpdateTicket stripped recommended_ready; should be preserved")
	}

	// Clearing it removes the key (stays sparse).
	if _, err := SetRecommendedReady(ctx, db, tk.ID, false, "tester", testAdminID(t, db)); err != nil {
		t.Fatalf("set false: %v", err)
	}
	var present int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets WHERE ticket_id = ? AND json_extract(attrs, '$.recommended_ready') IS NOT NULL`, tk.ID).Scan(&present); err != nil {
		t.Fatalf("presence check: %v", err)
	}
	if present != 0 {
		t.Fatalf("recommended_ready key should be removed when false")
	}
}

// TestTicketAttrIndexedQuery proves a declared scalar key remains queryable via
// json_extract with EnsureAttrIndex, i.e. Tier-3a promotion still works after
// the framework refactor.
func TestTicketAttrIndexedQuery(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)

	if err := EnsureAttrIndex(ctx, db, "tickets", "health_score"); err != nil {
		t.Fatalf("EnsureAttrIndex: %v", err)
	}
	// Idempotent: calling again must not error.
	if err := EnsureAttrIndex(ctx, db, "tickets", "health_score"); err != nil {
		t.Fatalf("EnsureAttrIndex (repeat): %v", err)
	}

	row := db.QueryRowContext(ctx,
		`SELECT json_extract(?, '$.health_score')`,
		`{"health_score":9}`)
	var got int
	if err := row.Scan(&got); err != nil {
		t.Fatalf("json_extract scan: %v", err)
	}
	if got != 9 {
		t.Fatalf("json_extract returned %d, want 9", got)
	}
}
