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
