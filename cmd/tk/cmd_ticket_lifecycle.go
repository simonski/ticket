package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func runTicketStateAlias(args []string, state, command string) error {
	fs := flag.NewFlagSet("ticket "+command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	positional, err := parseFlagsWithPositionals(fs, args)
	if err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, positional)
	if err != nil || len(rest) != 0 {
		return fmt.Errorf("usage: tk %s [-id] <id> [-m comment]", command)
	}
	return updateTicketState(idVal, state, *message)
}

func runTicketState(args []string) error {
	usage := "tk state <id> <idle|active|success|fail|design|develop|test|done> [-m comment]"
	fs := flag.NewFlagSet("ticket state", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	positional, err := parseFlagsWithPositionals(fs, args)
	if err != nil {
		return err
	}
	idVal := strings.TrimSpace(*id)
	var stateArg string
	switch {
	case idVal != "" && len(positional) == 1:
		stateArg = positional[0]
	case idVal == "" && len(positional) == 2:
		idVal = positional[0]
		stateArg = positional[1]
	default:
		return errors.New("usage: " + usage)
	}
	normalized := strings.ToLower(strings.TrimSpace(stateArg))
	switch {
	case store.ValidState(normalized):
		return updateTicketState(idVal, normalized, *message)
	case store.ValidStage(normalized):
		return updateTicketStage(idVal, normalized, *message)
	default:
		return errors.New("usage: " + usage)
	}
}

func updateTicketStage(idArg, stage string, message ...string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticketRef := normalizeBareTicketRef(cfg, svc, idArg)
	current, err := svc.GetTicket(context.Background(), ticketRef)
	if err != nil {
		return err
	}
	var msg string
	if len(message) > 0 {
		msg = message[0]
	}
	updated, err := svc.UpdateTicket(context.Background(), current.ID, libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           current.Assignee,
		Stage:              stage,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
		Message:            msg,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	return nil
}

func updateTicketState(idArg, state string, message ...string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticketRef := normalizeBareTicketRef(cfg, svc, idArg)
	current, err := svc.GetTicket(context.Background(), ticketRef)
	if err != nil {
		return err
	}
	var msg string
	if len(message) > 0 {
		msg = message[0]
	}
	return updateTicketLifecycleRequest(svc, current.ID, current, state, msg)
}

func updateTicketLifecycleRequest(svc libticket.Service, id string, current store.Ticket, state string, message ...string) error {
	assignee := current.Assignee
	if state == store.StateActive && strings.TrimSpace(assignee) == "" {
		status, err := svc.Status(context.Background())
		if err == nil && status.User != nil && strings.TrimSpace(status.User.Username) != "" {
			assignee = status.User.Username
		} else {
			assignee = fallbackCommandUsername()
		}
	}
	var msg string
	if len(message) > 0 {
		msg = message[0]
	}
	updated, err := svc.UpdateTicket(context.Background(), id, libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           assignee,
		State:              state,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
		Message:            msg,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	// Show a clear visual transition summary for lifecycle changes (TK-186).
	if state == store.StateSuccess {
		if updated.Stage != current.Stage {
			fmt.Printf("\n◉ %s advanced: %s → %s/%s\n", updated.ID, current.Status, updated.Stage, updated.State)
		} else if updated.Complete && !current.Complete {
			fmt.Printf("\n✓ %s is complete.\n", updated.ID)
		}
	}
	return nil
}

func runSetTicketClosed(args []string, closed bool) error {
	command := "open"
	if closed {
		command = "close"
	}
	usage := fmt.Sprintf("ticket %s <id> [-m comment]", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal := strings.TrimSpace(*id)
	switch {
	case idVal != "" && fs.NArg() == 0:
		// -id flag form
	case idVal == "" && fs.NArg() == 1:
		idVal = fs.Args()[0]
	default:
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(context.Background(), idVal)
	if err != nil {
		return err
	}
	var updated store.Ticket
	if closed {
		updated, err = svc.CloseTicket(context.Background(), ticket.ID, *message)
	} else {
		updated, err = svc.OpenTicket(context.Background(), ticket.ID, *message)
	}
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	// Keep the confirmation terse (see TK-43): a bare "OK" is enough.
	fmt.Println("OK")
	return nil
}

func runSetTicketArchived(args []string, archived bool) error {
	command := "unarchive"
	if archived {
		command = "archive"
	}
	usage := fmt.Sprintf("ticket %s [-id] <id> [-m comment] [-v]", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	verbose := fs.Bool("v", false, "show full ticket detail")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(context.Background(), idVal)
	if err != nil {
		return err
	}
	var updated store.Ticket
	if archived {
		updated, err = svc.ArchiveTicket(context.Background(), ticket.ID, *message)
	} else {
		updated, err = svc.UnarchiveTicket(context.Background(), ticket.ID, *message)
	}
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	if *verbose {
		printTicket(updated)
		return nil
	}
	if archived {
		fmt.Printf("%s archived\n", updated.ID)
	} else {
		fmt.Printf("%s unarchived\n", updated.ID)
	}
	return nil
}

func runSetTicketDraft(args []string, undraft bool) error {
	command := "draft"
	if undraft {
		command = "undraft"
	}
	usage := fmt.Sprintf("tk %s [-id <id>] <id> [<id> ...] [-m comment]", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ids := make([]string, 0, 1+len(fs.Args()))
	if trimmed := strings.TrimSpace(*id); trimmed != "" {
		ids = append(ids, trimmed)
	}
	for _, arg := range fs.Args() {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			continue
		}
		ids = append(ids, trimmed)
	}
	if len(ids) == 0 {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	updatedTickets := make([]store.Ticket, 0, len(ids))
	for _, idVal := range ids {
		ticket, getErr := svc.GetTicket(context.Background(), idVal)
		if getErr != nil {
			return getErr
		}
		var updated store.Ticket
		if undraft {
			updated, err = svc.ReadyTicket(context.Background(), ticket.ID, *message)
		} else {
			updated, err = svc.NotReadyTicket(context.Background(), ticket.ID, *message)
		}
		if err != nil {
			return err
		}
		updatedTickets = append(updatedTickets, updated)
	}
	if outputJSON {
		if len(updatedTickets) == 1 {
			return printJSON(updatedTickets[0])
		}
		return printJSON(updatedTickets)
	}
	for _, updated := range updatedTickets {
		printTicket(updated)
	}
	return nil
}

func runRejectTicket(args []string) error {
	usage := "tk reject [-id] <id> [-m comment]"
	fs := flag.NewFlagSet("reject", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	current, err := svc.GetTicket(context.Background(), idVal)
	if err != nil {
		return err
	}
	validStages, err := ticketWorkflowStageNames(svc, current)
	if err != nil {
		return err
	}
	if len(validStages) == 0 {
		return errors.New("current workflow has no stages")
	}
	updated, err := svc.UpdateTicket(context.Background(), current.ID, libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		GitRepository:      current.GitRepository,
		GitBranch:          current.GitBranch,
		ParentID:           current.ParentID,
		Assignee:           current.Assignee,
		Stage:              validStages[0],
		State:              store.StateIdle,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
		Type:               current.Type,
	})
	if err != nil {
		return err
	}
	updated, err = svc.DraftTicket(context.Background(), updated.ID, *message)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	return nil
}

func runComplete(args []string) error {
	fs := flag.NewFlagSet("complete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	mergePR := fs.Bool("merge-pr", false, "merge the ticket's open linked PR(s) on completion")
	closePR := fs.Bool("close-pr", false, "close the ticket's open linked PR(s) on completion")
	positional, err := parseFlagsWithPositionals(fs, args)
	if err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, positional)
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk complete [-id] <id> [-m comment] [--merge-pr|--close-pr]")
	}
	if *mergePR && *closePR {
		return errors.New("choose only one of --merge-pr or --close-pr")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ctx := context.Background()
	updated, err := svc.CompleteTicket(ctx, idVal, *message)
	if err != nil {
		return err
	}
	if !outputJSON {
		fmt.Printf("%s completed (stage: done, complete: true)\n", updated.ID)
	}
	// Reconcile any open linked PR (TK-160): act on the flag, else prompt on an
	// interactive terminal, else skip with a printed warning. PR-side failures are
	// non-fatal — the ticket is already complete.
	prAction := ""
	if *mergePR {
		prAction = store.PullRequestStatusMerged
	} else if *closePR {
		prAction = store.PullRequestStatusClosed
	}
	msgOut := os.Stdout
	if outputJSON {
		// Keep machine-readable stdout clean; route reconciliation notes to stderr.
		msgOut = os.Stderr
	}
	reconcileTicketPullRequests(ctx, svc, updated.ID, prAction, !outputJSON && interactiveStdio(), msgOut)
	if outputJSON {
		return printJSON(updated)
	}
	return nil
}

func runReopen(args []string) error {
	fs := flag.NewFlagSet("reopen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	positional, err := parseFlagsWithPositionals(fs, args)
	if err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, positional)
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk reopen [-id] <id> [-m comment]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	updated, err := svc.ReopenTicket(context.Background(), idVal, *message)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	fmt.Printf("%s reopened (stage: %s, complete: false)\n", updated.ID, updated.Stage)
	return nil
}

func runNext(args []string) error {
	fs := flag.NewFlagSet("next", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk next [-id] <id>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	before, err := svc.GetTicket(context.Background(), idVal)
	if err != nil {
		return err
	}
	updated, err := svc.NextTicket(context.Background(), before.ID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	if updated.Complete {
		fmt.Printf("%s advanced: %s -> done (complete)\n", updated.ID, before.Status)
	} else {
		fmt.Printf("%s advanced: %s -> %s (idle)\n", updated.ID, before.Status, updated.Status)
	}
	return nil
}

func runPrevious(args []string) error {
	fs := flag.NewFlagSet("previous", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk previous [-id] <id>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	before, err := svc.GetTicket(context.Background(), idVal)
	if err != nil {
		return err
	}
	updated, err := svc.PreviousTicket(context.Background(), before.ID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	fmt.Printf("%s regressed: %s -> %s (idle)\n", updated.ID, before.Status, updated.Status)
	return nil
}
