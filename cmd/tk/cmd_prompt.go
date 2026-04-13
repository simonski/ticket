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
	if err := fs.Parse(args); err != nil {
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

	epic, hasEpic := findAncestorByType(ancestors, "epic")
	story, hasStory := findAncestorByType(ancestors, "story")
	if strings.EqualFold(ticket.Type, "story") {
		story = ticket
		hasStory = true
	}

	roleTitle, roleAC := "N/A", "N/A"
	if ticket.RoleID != nil {
		roles, roleErr := svc.ListRoles(ctx)
		if roleErr == nil {
			for _, role := range roles {
				if role.ID == *ticket.RoleID {
					roleTitle = promptValue(role.Title)
					roleAC = promptValue(role.AcceptanceCriteria)
					break
				}
			}
		}
	}

	stageName := promptValue(ticket.Stage)
	stageAC := "N/A"
	if stage, stageErr := resolveStageForPrompt(ctx, svc, ticket, project); stageErr == nil && stage != nil {
		stageName = promptValue(stage.StageName)
		stageAC = promptValue(stage.AcceptanceCriteria)
		if strings.TrimSpace(stage.DefinitionOfReady) != "" {
			stageAC = promptValue(stage.DefinitionOfReady)
		}
	}

	var b strings.Builder
	b.WriteString("AGENT EXECUTION PROMPT\n\n")
	b.WriteString("PROJECT\n")
	b.WriteString("Title: " + promptValue(project.Title) + "\n")
	b.WriteString("Description: " + promptValue(project.Description) + "\n")
	b.WriteString("Definition of Ready: " + promptValue(project.AcceptanceCriteria) + "\n")
	b.WriteString("Definition of Done: " + promptValue(project.Notes) + "\n\n")

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
	b.WriteString("Acceptance Criteria: " + promptValue(ticket.AcceptanceCriteria) + "\n\n")

	b.WriteString("ROLE\n")
	b.WriteString("Title: " + roleTitle + "\n")
	b.WriteString("Acceptance Criteria: " + roleAC + "\n\n")

	b.WriteString("STAGE\n")
	b.WriteString("Name: " + stageName + "\n")
	b.WriteString("Acceptance Criteria: " + stageAC + "\n")

	return b.String(), nil
}

type promptService interface {
	GetTicketByID(ctx context.Context, id string) (store.Ticket, error)
	GetTicket(ctx context.Context, ref string) (store.Ticket, error)
	GetProject(ctx context.Context, id string) (store.Project, error)
	ListRoles(ctx context.Context) ([]store.Role, error)
	GetSdlcStage(ctx context.Context, stageID int64) (store.SdlcStage, error)
	ListSdlcStages(ctx context.Context, sdlcID int64) ([]store.SdlcStage, error)
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

func resolveStageForPrompt(ctx context.Context, svc promptService, ticket store.Ticket, project store.Project) (*store.SdlcStage, error) {
	if ticket.SdlcStageID != nil {
		stage, err := svc.GetSdlcStage(ctx, *ticket.SdlcStageID)
		if err == nil {
			return &stage, nil
		}
	}

	sdlcID := ticket.SdlcID
	if sdlcID == nil {
		sdlcID = project.SdlcID
	}
	if sdlcID == nil {
		return nil, nil
	}

	stages, err := svc.ListSdlcStages(ctx, *sdlcID)
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
