package store

import (
	"context"
	"testing"
)

func TestRefinementDialogueTurn(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	adminID := testAdminID(t, db)

	proj, err := CreateProject(ctx, db, "Refine", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	idea, err := CreateTicket(ctx, db, TicketCreateParams{ProjectID: proj.ID, Type: "idea", Title: "An idea", CreatedBy: adminID})
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}

	// No comments yet → agent's turn.
	turn, err := RefinementDialogueTurn(ctx, db, idea.ID)
	if err != nil || turn != RefinementTurnAgent {
		t.Fatalf("turn(no comments) = %q, %v; want agent", turn, err)
	}

	// Human comment → agent's turn.
	if _, err := CreateUser(ctx, db, "human1", "password123", "user"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	hu, _ := GetUserByUsername(ctx, db, "human1")
	if _, err := AddComment(ctx, db, idea.ID, hu.ID, "please clarify X"); err != nil {
		t.Fatalf("AddComment(human): %v", err)
	}
	if turn, _ := RefinementDialogueTurn(ctx, db, idea.ID); turn != RefinementTurnAgent {
		t.Fatalf("turn(after human) = %q, want agent", turn)
	}

	// Agent comment → human's turn.
	ag, _, err := CreateAgent(ctx, db, "password")
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if _, err := AddComment(ctx, db, idea.ID, ag.ID, "here are my questions"); err != nil {
		t.Fatalf("AddComment(agent): %v", err)
	}
	if turn, _ := RefinementDialogueTurn(ctx, db, idea.ID); turn != RefinementTurnHuman {
		t.Fatalf("turn(after agent) = %q, want human", turn)
	}
}

func TestApproveRefinementSingleAndBreakdown(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	adminID := testAdminID(t, db)

	proj, err := CreateProject(ctx, db, "Approve", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Single story: approve → ready.
	single, err := CreateTicket(ctx, db, TicketCreateParams{ProjectID: proj.ID, Type: "idea", Title: "Single", CreatedBy: adminID})
	if err != nil {
		t.Fatalf("CreateTicket(single): %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET stage='refine', state='idle' WHERE ticket_id=?`, single.ID); err != nil {
		t.Fatalf("set refine: %v", err)
	}
	approved, err := ApproveRefinement(ctx, db, single.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("ApproveRefinement(single): %v", err)
	}
	if approved.Stage != StageReady || approved.Draft {
		t.Fatalf("single after approve: stage=%q draft=%v, want ready/false", approved.Stage, approved.Draft)
	}

	// Breakdown: idea with draft children → approve re-types to epic, children ready.
	epicIdea, err := CreateTicket(ctx, db, TicketCreateParams{ProjectID: proj.ID, Type: "idea", Title: "Big idea", CreatedBy: adminID})
	if err != nil {
		t.Fatalf("CreateTicket(epicIdea): %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET stage='refine', state='idle' WHERE ticket_id=?`, epicIdea.ID); err != nil {
		t.Fatalf("set refine: %v", err)
	}
	c1, err := AddRefinementProposalChild(ctx, db, epicIdea.ID, "Story A", "desc a", "", adminID)
	if err != nil {
		t.Fatalf("child A: %v", err)
	}
	if _, err := AddRefinementProposalChild(ctx, db, epicIdea.ID, "Story B", "desc b", "", adminID); err != nil {
		t.Fatalf("child B: %v", err)
	}
	epic, err := ApproveRefinement(ctx, db, epicIdea.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("ApproveRefinement(breakdown): %v", err)
	}
	if epic.Type != "epic" {
		t.Fatalf("parent type = %q, want epic", epic.Type)
	}
	gotChild, err := GetTicket(ctx, db, c1.ID)
	if err != nil {
		t.Fatalf("GetTicket(child): %v", err)
	}
	if gotChild.Stage != StageReady || gotChild.Draft {
		t.Fatalf("child after breakdown: stage=%q draft=%v, want ready/false", gotChild.Stage, gotChild.Draft)
	}
}

func TestParseRefinementProposal(t *testing.T) {
	t.Parallel()
	ready := ParseRefinementProposal("Looks clear.\nPROPOSE_READY\nDESCRIPTION: Build the export.\nACCEPTANCE_CRITERIA: CSV and JSON; under 30s")
	if ready.ProposalKind != "ready" || ready.Description == "" || ready.AcceptanceCriteria == "" {
		t.Fatalf("ready parse = %+v", ready)
	}
	bd := ParseRefinementProposal("This is large.\nPROPOSE_BREAKDOWN\nSTORY: Export CSV | the csv path\nSTORY: Export JSON | the json path")
	if bd.ProposalKind != "breakdown" || len(bd.Stories) != 2 || bd.Stories[0].Title != "Export CSV" {
		t.Fatalf("breakdown parse = %+v", bd)
	}
	q := ParseRefinementProposal("Which platforms do you need?")
	if q.ProposalKind != "question" {
		t.Fatalf("question parse = %+v", q)
	}
	// A breakdown marker with no parseable stories falls back to a question.
	empty := ParseRefinementProposal("PROPOSE_BREAKDOWN\n(no stories)")
	if empty.ProposalKind != "question" {
		t.Fatalf("empty breakdown = %+v", empty)
	}
}

func TestEnsureRefinerUser(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	// No refiner yet → creates one.
	id1, err := EnsureRefinerUser(ctx, db)
	if err != nil || id1 == "" {
		t.Fatalf("EnsureRefinerUser() = %q, %v", id1, err)
	}
	u, err := GetUserByID(ctx, db, id1)
	if err != nil || u.UserType != "agent" {
		t.Fatalf("refiner user = %+v, %v", u, err)
	}
	// Idempotent-ish: returns an existing refiner agent next time.
	id2, err := EnsureRefinerUser(ctx, db)
	if err != nil || id2 != id1 {
		t.Fatalf("EnsureRefinerUser() second = %q, %v (want %q)", id2, err, id1)
	}
}
