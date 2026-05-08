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
)

func runWorkItem(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("usage: tk work-item <list|reassign|cancel|retry|feedback|state-get|state-set> [flags]")
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	api := client.New(cfg)
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
	default:
		return fmt.Errorf("unknown work-item command %q", args[0])
	}
}
