package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/client"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/libticket"
)

func runWorkItem(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("usage: tk work-item <list|queue|start|create|reassign|cancel|retry|feedback|state-get|state-set> [flags]")
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	api := client.New(cfg)
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ctx := context.Background()

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("work-item list", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "ticket id")
		status := fs.String("status", "", "work item status")
		assigneeType := fs.String("assignee_type", "", "assignee type (human|agent)")
		limit := fs.Int("limit", 20, "max items")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		ticketID, rest, err := resolveIDFlag(*id, fs.Args())
		if err != nil || len(rest) != 0 || ticketID == "" {
			return errors.New("usage: tk work-item list [-id] <ticket-id> [-status <active|success|fail|stopped>] [-assignee_type <human|agent>] [-limit <n>]")
		}
		items, err := api.ListWorkItems(ctx, ticketID, *status, *assigneeType, *limit)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(items)
		}
		for _, item := range items {
			fmt.Printf("%s\t%s\t%s\t%s\n", item.ID, item.Status, item.AssigneeType, item.AssigneeID)
		}
		return nil
	case "reassign", "cancel", "retry", "feedback":
		fs := flag.NewFlagSet("work-item "+args[0], flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "ticket id")
		workItemID := fs.String("work-item", "", "work item id")
		assignee := fs.String("assignee", "", "assignee username")
		message := fs.String("m", "", "message")
		commitRef := fs.String("commit_ref", "", "commit ref")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		ticketID, rest, err := resolveIDFlag(*id, fs.Args())
		if err != nil || len(rest) != 0 || strings.TrimSpace(ticketID) == "" || strings.TrimSpace(*workItemID) == "" {
			return fmt.Errorf("usage: tk work-item %s [-id] <ticket-id> -work-item <work-item-id> [-assignee <username>] [-m message] [-commit_ref sha]", args[0])
		}
		item, err := api.ActWorkItem(ctx, ticketID, strings.TrimSpace(*workItemID), args[0], client.WorkItemActionRequest{
			Assignee:  strings.TrimSpace(*assignee),
			Message:   strings.TrimSpace(*message),
			CommitRef: strings.TrimSpace(*commitRef),
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(item)
		}
		fmt.Printf("work-item %s: %s (%s)\n", args[0], item.ID, item.Status)
		return nil
	case "state-get":
		fs := flag.NewFlagSet("work-item state-get", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "ticket id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		ticketID, rest, err := resolveIDFlag(*id, fs.Args())
		if err != nil || len(rest) != 0 || strings.TrimSpace(ticketID) == "" {
			return errors.New("usage: tk work-item state-get [-id] <ticket-id>")
		}
		state, err := api.GetInterventionState(ctx, ticketID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(state)
		}
		fmt.Printf("ticket=%s state=%s owner=%s\n", state.TicketID, state.State, state.OwnerName)
		return nil
	case "state-set":
		fs := flag.NewFlagSet("work-item state-set", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "ticket id")
		stateValue := fs.String("state", "", "state (open|triaged|in_progress|resolved|wont_fix)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		ticketID, rest, err := resolveIDFlag(*id, fs.Args())
		if err != nil || len(rest) != 0 || strings.TrimSpace(ticketID) == "" || strings.TrimSpace(*stateValue) == "" {
			return errors.New("usage: tk work-item state-set [-id] <ticket-id> -state <open|triaged|in_progress|resolved|wont_fix>")
		}
		state, err := api.SetInterventionState(ctx, ticketID, strings.TrimSpace(*stateValue))
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(state)
		}
		fmt.Printf("ticket=%s state=%s owner=%s\n", state.TicketID, state.State, state.OwnerName)
		return nil
	case "queue":
		fs := flag.NewFlagSet("work-item queue", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectID := fs.String("project_id", "", "project id, title, prefix, or alias (default current)")
		id := fs.String("id", "", "specific ticket id/ref")
		dryRun := fs.Bool("dry-run", false, "preview without assignment")
		explain := fs.Bool("explain", false, "print explanation for returned status")
		strategy := fs.String("strategy", "priority", "queue strategy (priority|order|aging)")
		preview := fs.Bool("preview", false, "list queue candidates without claiming")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if len(fs.Args()) != 0 {
			return errors.New("usage: tk work-item queue [-project_id <id>] [-id <ticket-id>] [-dry-run] [-explain] [-strategy <priority|order|aging>] [-preview]")
		}
		resolvedProjectID, err := resolveWorkItemProjectID(ctx, cfg, svc, *projectID)
		if err != nil {
			return err
		}
		if *preview {
			candidates, queueErr := api.ListProjectWorkItemQueue(ctx, resolvedProjectID, strings.TrimSpace(*strategy), 20)
			if queueErr != nil {
				return queueErr
			}
			if outputJSON {
				return printJSON(candidates)
			}
			for _, candidate := range candidates {
				blocked := ""
				if candidate.Blocked {
					blocked = " blocked_by=" + candidate.BlockedBy
				}
				fmt.Printf("%s\tp%d\torder=%d\t%s/%s%s\t%s\n", candidate.TicketID, candidate.Priority, candidate.Order, candidate.Stage, candidate.State, blocked, candidate.Title)
			}
			return nil
		}
		response, err := api.RequestTicket(ctx, client.TicketRequest{
			ProjectID: resolvedProjectID,
			TicketRef: strings.TrimSpace(*id),
			DryRun:    *dryRun,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(response)
		}
		fmt.Printf("queue status: %s\n", response.Status)
		if response.Ticket != nil {
			fmt.Printf("ticket: %s\t%s\t%s/%s\n", response.Ticket.ID, response.Ticket.Title, response.Ticket.Stage, response.Ticket.State)
		}
		if *explain {
			fmt.Println(requestStatusExplanation(response.Status, strings.TrimSpace(*id)))
		}
		return nil
	case "start":
		fs := flag.NewFlagSet("work-item start", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "ticket id")
		message := fs.String("m", "", "optional comment")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		ticketID, rest, err := resolveIDFlag(*id, fs.Args())
		if err != nil || len(rest) != 0 || strings.TrimSpace(ticketID) == "" {
			return errors.New("usage: tk work-item start [-id] <ticket-id> [-m message]")
		}
		if _, readyErr := api.ReadyTicket(ctx, ticketID, strings.TrimSpace(*message)); readyErr != nil {
			return readyErr
		}
		ticket, err := api.GetTicket(ctx, ticketID)
		if err != nil {
			return err
		}
		response, err := api.RequestTicket(ctx, client.TicketRequest{
			ProjectID: ticket.ProjectID,
			TicketRef: ticketID,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(response)
		}
		fmt.Printf("start status: %s\n", response.Status)
		if response.Ticket != nil {
			fmt.Printf("ticket: %s\t%s\t%s/%s\n", response.Ticket.ID, response.Ticket.Title, response.Ticket.Stage, response.Ticket.State)
		}
		return nil
	case "create":
		fs := flag.NewFlagSet("work-item create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectID := fs.String("project_id", "", "project id, title, prefix, or alias (default current)")
		title := fs.String("title", "", "title")
		typ := fs.String("type", "task", "ticket type")
		description := fs.String("description", "", "description")
		startNow := fs.Bool("start", false, "immediately queue after creating")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if len(fs.Args()) != 0 || strings.TrimSpace(*title) == "" {
			return errors.New("usage: tk work-item create [-project_id <id>] -title <title> [-type <task|bug|story|chore>] [-description <text>] [-start]")
		}
		resolvedProjectID, err := resolveWorkItemProjectID(ctx, cfg, svc, *projectID)
		if err != nil {
			return err
		}
		created, err := api.CreateTicket(ctx, client.TicketCreateRequest{
			ProjectID:   resolvedProjectID,
			Type:        strings.TrimSpace(*typ),
			Title:       strings.TrimSpace(*title),
			Description: strings.TrimSpace(*description),
		})
		if err != nil {
			return err
		}
		readyTicket, err := api.ReadyTicket(ctx, created.ID, "")
		if err != nil {
			return err
		}
		if *startNow {
			_, err := api.RequestTicket(ctx, client.TicketRequest{ProjectID: readyTicket.ProjectID, TicketRef: readyTicket.ID})
			if err != nil {
				return err
			}
		}
		if outputJSON {
			return printJSON(readyTicket)
		}
		fmt.Printf("created work item ticket: %s\t%s\t%s/%s\n", readyTicket.ID, readyTicket.Title, readyTicket.Stage, readyTicket.State)
		return nil
	default:
		return fmt.Errorf("unknown work-item command %q", args[0])
	}
}

func resolveWorkItemProjectID(ctx context.Context, cfg config.Config, svc libticket.Service, provided string) (int64, error) {
	project, err := resolveProjectFromFlagOrConfig(ctx, cfg, svc, provided)
	if err != nil {
		ref := firstNonEmpty(strings.TrimSpace(provided), strings.TrimSpace(cfg.ProjectID))
		switch strings.ToLower(ref) {
		case "public", "private":
			if projects, listErr := svc.ListProjects(ctx); listErr == nil {
				for _, candidate := range projects {
					if strings.EqualFold(ref, candidate.Visibility) {
						return candidate.ID, nil
					}
				}
			}
		}
		return 0, errors.New("project_id is required (set an active project or pass -project_id)")
	}
	return project.ID, nil
}
