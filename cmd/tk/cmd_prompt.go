package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func runPrompt(args []string) error {
	cfg, svc, _, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	_ = cfg

	fs := flag.NewFlagSet("prompt", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	err = fs.Parse(args)
	if err != nil {
		return err
	}

	ticketID := strings.TrimSpace(*id)
	if ticketID == "" && fs.NArg() > 0 {
		ticketID = strings.TrimSpace(fs.Arg(0))
	}
	if ticketID == "" {
		return errors.New("usage: tk prompt <ticket-id>")
	}

	prompt, err := buildPromptForTicket(context.Background(), svc, ticketID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]string{"ticket_id": ticketID, "prompt": prompt})
	}
	fmt.Println(prompt)
	return nil
}

func buildPromptForTicket(ctx context.Context, svc promptService, ticketRef string) (string, error) {
	ticket, err := svc.GetTicketByID(ctx, ticketRef)
	if err != nil {
		ticket, err = svc.GetTicket(ctx, ticketRef)
		if err != nil {
			return "", err
		}
	}

	project, err := svc.GetProject(ctx, strconv.FormatInt(ticket.ProjectID, 10))
	if err != nil {
		return "", err
	}

	ancestors, err := listTicketAncestors(ctx, svc, ticket)
	if err != nil {
		return "", err
	}

	projectGuidance := project.ResolveGuidance(ticket.Stage)
	projectDOD := projectGuidance.DOD
	if strings.TrimSpace(projectDOD) == "" {
		projectDOD = project.Notes
	}

	epic, hasEpic := findAncestorByType(ancestors, "epic")
	story, hasStory := findAncestorByType(ancestors, "story")
	if strings.EqualFold(ticket.Type, "story") {
		story = ticket
		hasStory = true
	}

	roleTitle, roleDOR, roleDOD, roleAC := "N/A", "N/A", "N/A", "N/A"
	if ticket.RoleID != nil {
		roles, roleErr := svc.ListRoles(ctx)
		if roleErr == nil {
			for _, role := range roles {
				if role.ID != *ticket.RoleID {
					continue
				}
				roleTitle = promptValue(role.Title)
				roleGuidance := role.ResolveGuidance(ticket.Stage)
				roleDOR = promptValue(roleGuidance.DOR)
				roleDOD = promptValue(roleGuidance.DOD)
				roleAC = promptValue(roleGuidance.AC)
				break
			}
		}
	}

	stageName := promptValue(ticket.Stage)
	stageDOR, stageDOD, stageAC := "N/A", "N/A", "N/A"
	if stage, stageErr := resolveStageForPrompt(ctx, svc, ticket, project); stageErr == nil && stage != nil {
		stageName = promptValue(stage.StageName)
		stageAC = promptValue(stage.AcceptanceCriteria)
		stageDOR = promptValue(stage.DefinitionOfReady)
		stageDOD = promptValue(stage.DefinitionOfDone)
	}
	ticketGuidance := ticket.ResolveGuidance(ticket.Stage)

	var b strings.Builder
	b.WriteString("AGENT EXECUTION PROMPT\n\n")
	b.WriteString("PROJECT\n")
	b.WriteString("Title: " + promptValue(project.Title) + "\n")
	b.WriteString("Description: " + promptValue(project.Description) + "\n")
	b.WriteString("Definition of Ready: " + promptValue(projectGuidance.DOR) + "\n")
	b.WriteString("Definition of Done: " + promptValue(projectDOD) + "\n")
	b.WriteString("Acceptance Criteria: " + promptValue(projectGuidance.AC) + "\n")
	b.WriteString("\n")

	b.WriteString("EPIC\n")
	if hasEpic {
		b.WriteString("Key: " + epic.ID + "\n")
		b.WriteString("Title: " + promptValue(epic.Title) + "\n")
		b.WriteString("Description: " + promptValue(epic.Description) + "\n")
	} else {
		b.WriteString("Key: N/A\nTitle: N/A\nDescription: N/A\n")
	}
	b.WriteString("\n")

	b.WriteString("STORY\n")
	if hasStory {
		b.WriteString("Key: " + story.ID + "\n")
		b.WriteString("Title: " + promptValue(story.Title) + "\n")
		b.WriteString("Description: " + promptValue(story.Description) + "\n")
	} else {
		b.WriteString("Key: N/A\nTitle: N/A\nDescription: N/A\n")
	}
	b.WriteString("\n")

	b.WriteString("TICKET\n")
	b.WriteString("Key: " + promptValue(ticket.ID) + "\n")
	b.WriteString("Type: " + promptValue(ticket.Type) + "\n")
	b.WriteString("Title: " + promptValue(ticket.Title) + "\n")
	b.WriteString("Description: " + promptValue(ticket.Description) + "\n")
	b.WriteString("Definition of Ready: " + promptValue(ticketGuidance.DOR) + "\n")
	b.WriteString("Definition of Done: " + promptValue(ticketGuidance.DOD) + "\n")
	b.WriteString("Acceptance Criteria: " + promptValue(ticketGuidance.AC) + "\n\n")

	b.WriteString("ROLE\n")
	b.WriteString("Title: " + roleTitle + "\n")
	b.WriteString("Definition of Ready: " + roleDOR + "\n")
	b.WriteString("Definition of Done: " + roleDOD + "\n")
	b.WriteString("Acceptance Criteria: " + roleAC + "\n\n")

	b.WriteString("STAGE\n")
	b.WriteString("Name: " + stageName + "\n")
	b.WriteString("Definition of Ready: " + stageDOR + "\n")
	b.WriteString("Definition of Done: " + stageDOD + "\n")
	b.WriteString("Acceptance Criteria: " + stageAC + "\n\n")

	repo, baseBranch, workBranch := resolveTicketVCS(ticket, project.GitRepository)
	b.WriteString("VCS\n")
	b.WriteString("Ticket: " + promptValue(ticket.ID) + "\n")
	b.WriteString("Repository: " + repo + "\n")
	b.WriteString("Branch to take from (base): " + baseBranch + "\n")
	b.WriteString("Branch to commit to (work): " + workBranch + "\n\n")

	b.WriteString("EXECUTION STEPS\n")
	b.WriteString("1. Set up the agent: load its skills, sub-agents, and this codebase.\n")
	b.WriteString("2. Build the project first to confirm a green baseline before changing anything.\n")
	b.WriteString("3. Set up the environment (dependencies, config, env vars).\n")
	b.WriteString("4. Clone the repository " + repo + " (skip if already present).\n")
	b.WriteString("5. Create and check out the work branch " + workBranch + " from " + baseBranch + ".\n")
	b.WriteString("6. Do the work for " + promptValue(ticket.ID) + " to satisfy the acceptance criteria above.\n")
	b.WriteString("7. Run the tests (and linters); fix until green.\n")
	b.WriteString("8. Commit and push " + workBranch + ", then stop for review.\n")

	return b.String(), nil
}

// resolveTicketVCS derives the repository and the base/work branches for a
// ticket's work. The repository falls back from the ticket to the project; the
// work branch defaults to feature/<ticket-id> when the ticket has no branch set;
// the base branch defaults to main.
func resolveTicketVCS(ticket store.Ticket, projectRepo string) (repo, baseBranch, workBranch string) {
	repo = strings.TrimSpace(ticket.GitRepository)
	if repo == "" {
		repo = strings.TrimSpace(projectRepo)
	}
	if repo == "" {
		repo = "N/A"
	}
	baseBranch = "main"
	workBranch = strings.TrimSpace(ticket.GitBranch)
	if workBranch == "" {
		workBranch = "feature/" + ticket.ID
	}
	return repo, baseBranch, workBranch
}

type promptService interface {
	GetTicketByID(ctx context.Context, id string) (store.Ticket, error)
	GetTicket(ctx context.Context, ref string) (store.Ticket, error)
	GetProject(ctx context.Context, id string) (store.Project, error)
	ListRoles(ctx context.Context) ([]store.Role, error)
	GetWorkflowStage(ctx context.Context, stageID int64) (store.WorkflowStage, error)
	ListWorkflowStages(ctx context.Context, workflowID int64) ([]store.WorkflowStage, error)
}

func listTicketAncestors(ctx context.Context, svc promptService, ticket store.Ticket) ([]store.Ticket, error) {
	visited := map[string]bool{}
	ancestors := make([]store.Ticket, 0)
	current := ticket.ParentID
	for current != nil && strings.TrimSpace(*current) != "" {
		parentID := strings.TrimSpace(*current)
		if visited[parentID] {
			break
		}
		visited[parentID] = true
		parent, err := svc.GetTicketByID(ctx, parentID)
		if err != nil {
			return nil, err
		}
		ancestors = append(ancestors, parent)
		current = parent.ParentID
	}
	return ancestors, nil
}

func findAncestorByType(ancestors []store.Ticket, ticketType string) (store.Ticket, bool) {
	for _, ancestor := range ancestors {
		if strings.EqualFold(ancestor.Type, ticketType) {
			return ancestor, true
		}
	}
	return store.Ticket{}, false
}

func resolveStageForPrompt(ctx context.Context, svc promptService, ticket store.Ticket, project store.Project) (*store.WorkflowStage, error) {
	if ticket.WorkflowStageID != nil {
		stage, err := svc.GetWorkflowStage(ctx, *ticket.WorkflowStageID)
		if err == nil {
			return &stage, nil
		}
	}

	workflowID := ticket.WorkflowID
	if workflowID == nil {
		workflowID = project.WorkflowID
	}
	if workflowID == nil {
		return nil, nil
	}

	stages, err := svc.ListWorkflowStages(ctx, *workflowID)
	if err != nil {
		return nil, err
	}
	for _, stage := range stages {
		if strings.EqualFold(stage.StageName, ticket.Stage) {
			return &stage, nil
		}
	}
	return nil, nil
}

func promptValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "N/A"
	}
	return v
}
