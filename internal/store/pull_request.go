package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrPullRequestNotFound is returned when a pull request cannot be located.
var ErrPullRequestNotFound = errors.New("pull request not found")

// Pull request lifecycle statuses.
const (
	PullRequestStatusDraft  = "draft"
	PullRequestStatusOpen   = "open"
	PullRequestStatusMerged = "merged"
	PullRequestStatusClosed = "closed"
)

// Pull request providers. "none" means the PR lives purely inside ticket;
// "github" means it is (or can be) mirrored to a real GitHub PR via gh.
const (
	PullRequestProviderNone   = "none"
	PullRequestProviderGitHub = "github"
)

// PullRequest is a VCS-host-agnostic merge request that lives inside ticket and
// is linked to a ticket and project. When the repository is a GitHub repo the
// provider is "github" and url may point at the real PR; otherwise it is native.
type PullRequest struct {
	ID           int64  `json:"id"`
	ProjectID    int64  `json:"project_id"`
	TicketID     string `json:"ticket_id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Repository   string `json:"repository"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	Status       string `json:"status"`
	Provider     string `json:"provider"`
	URL          string `json:"url"`
	CreatedBy    string `json:"created_by"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	MergedAt     string `json:"merged_at"`
}

// PullRequestParams are the inputs for creating a pull request.
type PullRequestParams struct {
	ProjectID    int64
	TicketID     string
	Title        string
	Description  string
	Repository   string
	SourceBranch string
	TargetBranch string
	Status       string
	Provider     string
	URL          string
	CreatedBy    string
}

const pullRequestColumns = `id, project_id, ticket_id, title, description, repository, source_branch, target_branch, status, provider, COALESCE(url, ''), COALESCE(created_by, ''), created_at, updated_at, COALESCE(merged_at, '')`

func scanPullRequest(row interface{ Scan(...any) error }) (PullRequest, error) {
	var pr PullRequest
	if err := row.Scan(&pr.ID, &pr.ProjectID, &pr.TicketID, &pr.Title, &pr.Description,
		&pr.Repository, &pr.SourceBranch, &pr.TargetBranch, &pr.Status, &pr.Provider,
		&pr.URL, &pr.CreatedBy, &pr.CreatedAt, &pr.UpdatedAt, &pr.MergedAt); err != nil {
		return PullRequest{}, err
	}
	return pr, nil
}

// NormalizePullRequestStatus lower-cases and validates a status, defaulting an
// empty value to "open". It returns "" for an invalid status.
func NormalizePullRequestStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "":
		return PullRequestStatusOpen
	case PullRequestStatusDraft:
		return PullRequestStatusDraft
	case PullRequestStatusOpen:
		return PullRequestStatusOpen
	case PullRequestStatusMerged:
		return PullRequestStatusMerged
	case PullRequestStatusClosed:
		return PullRequestStatusClosed
	default:
		return ""
	}
}

func normalizePullRequestProvider(provider string) string {
	if strings.EqualFold(strings.TrimSpace(provider), PullRequestProviderGitHub) {
		return PullRequestProviderGitHub
	}
	return PullRequestProviderNone
}

// CreatePullRequest inserts a pull request for an existing ticket. The ticket
// must exist; the project defaults to the ticket's project and the title
// defaults to the ticket's title when not supplied.
func CreatePullRequest(ctx context.Context, db *sql.DB, params PullRequestParams) (PullRequest, error) {
	ticketID := strings.TrimSpace(params.TicketID)
	if ticketID == "" {
		return PullRequest{}, errors.New("pull request requires a ticket id")
	}
	ticket, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return PullRequest{}, err
	}
	status := NormalizePullRequestStatus(params.Status)
	if status == "" {
		return PullRequest{}, errors.New("invalid pull request status")
	}
	projectID := params.ProjectID
	if projectID == 0 {
		projectID = ticket.ProjectID
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		title = ticket.Title
	}
	res, err := db.ExecContext(ctx, `
		INSERT INTO pull_requests
			(project_id, ticket_id, title, description, repository, source_branch, target_branch, status, provider, url, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, projectID, ticket.ID, title, strings.TrimSpace(params.Description),
		strings.TrimSpace(params.Repository), strings.TrimSpace(params.SourceBranch), strings.TrimSpace(params.TargetBranch),
		status, normalizePullRequestProvider(params.Provider), strings.TrimSpace(params.URL), nullableUserID(params.CreatedBy))
	if err != nil {
		return PullRequest{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return PullRequest{}, err
	}
	return GetPullRequest(ctx, db, id)
}

// GetPullRequest returns a single pull request by id.
func GetPullRequest(ctx context.Context, db *sql.DB, id int64) (PullRequest, error) {
	row := db.QueryRowContext(ctx, `SELECT `+pullRequestColumns+` FROM pull_requests WHERE id = ?`, id)
	pr, err := scanPullRequest(row)
	if errors.Is(err, sql.ErrNoRows) {
		return PullRequest{}, ErrPullRequestNotFound
	}
	if err != nil {
		return PullRequest{}, err
	}
	return pr, nil
}

const listPullRequestsByTicketQuery = `SELECT ` + pullRequestColumns + ` FROM pull_requests WHERE ticket_id = ? ORDER BY id DESC`

const listPullRequestsByProjectQuery = `SELECT ` + pullRequestColumns + ` FROM pull_requests WHERE project_id = ? ORDER BY id DESC`

func scanPullRequests(rows *sql.Rows) ([]PullRequest, error) {
	defer rows.Close()
	prs := make([]PullRequest, 0)
	for rows.Next() {
		pr, scanErr := scanPullRequest(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		prs = append(prs, pr)
	}
	return prs, rows.Err()
}

// ListPullRequestsByTicket returns all pull requests linked to a ticket.
func ListPullRequestsByTicket(ctx context.Context, db *sql.DB, ticketID string) ([]PullRequest, error) {
	rows, err := db.QueryContext(ctx, listPullRequestsByTicketQuery, strings.TrimSpace(ticketID))
	if err != nil {
		return nil, err
	}
	return scanPullRequests(rows)
}

// ListPullRequestsByProject returns all pull requests in a project.
func ListPullRequestsByProject(ctx context.Context, db *sql.DB, projectID int64) ([]PullRequest, error) {
	rows, err := db.QueryContext(ctx, listPullRequestsByProjectQuery, projectID)
	if err != nil {
		return nil, err
	}
	return scanPullRequests(rows)
}

// UpdatePullRequestStatus transitions a pull request to a new status. Moving to
// "merged" stamps merged_at; other transitions leave merged_at untouched.
func UpdatePullRequestStatus(ctx context.Context, db *sql.DB, id int64, status string) (PullRequest, error) {
	ns := NormalizePullRequestStatus(status)
	if ns == "" {
		return PullRequest{}, fmt.Errorf("invalid pull request status %q", status)
	}
	res, err := db.ExecContext(ctx, `
		UPDATE pull_requests
		SET status = ?,
		    merged_at = CASE WHEN ? = 'merged' THEN CURRENT_TIMESTAMP ELSE merged_at END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, ns, ns, id)
	if err != nil {
		return PullRequest{}, err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return PullRequest{}, ErrPullRequestNotFound
	}
	return GetPullRequest(ctx, db, id)
}
