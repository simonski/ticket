package store

import (
	"context"
	"strings"
	"testing"
)

func TestPullRequestCRUD(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "PR Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Work item",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	pr, err := CreatePullRequest(ctx, db, PullRequestParams{
		TicketID:     ticket.ID,
		Repository:   "github.com/acme/widget",
		SourceBranch: "feature/" + ticket.ID,
		TargetBranch: "main",
		Provider:     PullRequestProviderGitHub,
		URL:          "https://github.com/acme/widget/pull/1",
	})
	if err != nil {
		t.Fatalf("CreatePullRequest() error = %v", err)
	}
	if pr.ID == 0 {
		t.Fatal("CreatePullRequest() returned zero id")
	}
	if pr.ProjectID != project.ID {
		t.Fatalf("pr.ProjectID = %d, want %d", pr.ProjectID, project.ID)
	}
	if pr.Title != ticket.Title {
		t.Fatalf("pr.Title = %q, want default to ticket title %q", pr.Title, ticket.Title)
	}
	if pr.Status != PullRequestStatusOpen {
		t.Fatalf("pr.Status = %q, want %q", pr.Status, PullRequestStatusOpen)
	}
	if pr.Provider != PullRequestProviderGitHub {
		t.Fatalf("pr.Provider = %q, want github", pr.Provider)
	}

	got, err := GetPullRequest(ctx, db, pr.ID)
	if err != nil {
		t.Fatalf("GetPullRequest() error = %v", err)
	}
	if got.SourceBranch != "feature/"+ticket.ID || got.TargetBranch != "main" {
		t.Fatalf("branches = %q -> %q", got.SourceBranch, got.TargetBranch)
	}

	byTicket, err := ListPullRequestsByTicket(ctx, db, ticket.ID)
	if err != nil {
		t.Fatalf("ListPullRequestsByTicket() error = %v", err)
	}
	if len(byTicket) != 1 || byTicket[0].ID != pr.ID {
		t.Fatalf("ListPullRequestsByTicket() = %+v", byTicket)
	}

	byProject, err := ListPullRequestsByProject(ctx, db, project.ID)
	if err != nil {
		t.Fatalf("ListPullRequestsByProject() error = %v", err)
	}
	if len(byProject) != 1 {
		t.Fatalf("ListPullRequestsByProject() len = %d, want 1", len(byProject))
	}

	// A non-GitHub PR defaults provider to none.
	native, err := CreatePullRequest(ctx, db, PullRequestParams{
		TicketID:     ticket.ID,
		Repository:   "git.example.com/acme/widget",
		SourceBranch: "wip",
	})
	if err != nil {
		t.Fatalf("CreatePullRequest(native) error = %v", err)
	}
	if native.Provider != PullRequestProviderNone {
		t.Fatalf("native.Provider = %q, want none", native.Provider)
	}

	if _, err := GetPullRequest(ctx, db, 99999); err != ErrPullRequestNotFound {
		t.Fatalf("GetPullRequest(missing) error = %v, want ErrPullRequestNotFound", err)
	}
}

func TestUpdatePullRequestStatus(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "PR Status", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{ProjectID: project.ID, Type: "task", Title: "Work"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	pr, err := CreatePullRequest(ctx, db, PullRequestParams{TicketID: ticket.ID, SourceBranch: "f"})
	if err != nil {
		t.Fatalf("CreatePullRequest() error = %v", err)
	}
	if pr.Status != PullRequestStatusOpen {
		t.Fatalf("initial status = %q, want open", pr.Status)
	}

	merged, err := UpdatePullRequestStatus(ctx, db, pr.ID, "merged")
	if err != nil {
		t.Fatalf("UpdatePullRequestStatus(merged) error = %v", err)
	}
	if merged.Status != PullRequestStatusMerged {
		t.Fatalf("status = %q, want merged", merged.Status)
	}
	if strings.TrimSpace(merged.MergedAt) == "" {
		t.Fatal("merged_at should be stamped on merge")
	}

	closed, err := UpdatePullRequestStatus(ctx, db, pr.ID, "closed")
	if err != nil {
		t.Fatalf("UpdatePullRequestStatus(closed) error = %v", err)
	}
	if closed.Status != PullRequestStatusClosed {
		t.Fatalf("status = %q, want closed", closed.Status)
	}

	if _, err := UpdatePullRequestStatus(ctx, db, pr.ID, "bogus"); err == nil {
		t.Fatal("UpdatePullRequestStatus(bogus) error = nil, want invalid status error")
	}
	if _, err := UpdatePullRequestStatus(ctx, db, 99999, "merged"); err != ErrPullRequestNotFound {
		t.Fatalf("UpdatePullRequestStatus(missing) error = %v, want ErrPullRequestNotFound", err)
	}
}
