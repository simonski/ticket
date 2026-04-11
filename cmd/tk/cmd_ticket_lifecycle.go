package main

import (
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal := strings.TrimSpace(*id)
	var stateArg string
	switch {
	case idVal != "" && fs.NArg() == 1:
		stateArg = fs.Args()[0]
	case idVal == "" && fs.NArg() == 2:
		idVal = fs.Args()[0]
		stateArg = fs.Args()[1]
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
	current, err := svc.GetTicket(idArg)
	if err != nil {
		return err
	}
	var msg string
	if len(message) > 0 {
		msg = message[0]
	}
	updated, err := svc.UpdateTicket(current.ID, libticket.TicketUpdateRequest{
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
	current, err := svc.GetTicket(idArg)
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
		status, err := svc.Status()
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
	updated, err := svc.UpdateTicket(id, libticket.TicketUpdateRequest{
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
	ticket, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	var updated store.Ticket
	if closed {
		updated, err = svc.CloseTicket(ticket.ID, *message)
	} else {
		updated, err = svc.OpenTicket(ticket.ID, *message)
	}
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	return nil
}

func runSetTicketArchived(args []string, archived bool) error {
	command := "unarchive"
	if archived {
		command = "archive"
	}
	usage := fmt.Sprintf("ticket %s [-id] <id> [-m comment]", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
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
	ticket, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	var updated store.Ticket
	if archived {
		updated, err = svc.ArchiveTicket(ticket.ID, *message)
	} else {
		updated, err = svc.UnarchiveTicket(ticket.ID, *message)
	}
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	return nil
}

func runSetTicketDraft(args []string, ready bool) error {
	command := "notready"
	if ready {
		command = "ready"
	}
	usage := fmt.Sprintf("ticket %s [-id] <id> [-m comment]", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
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
	ticket, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	var updated store.Ticket
	if ready {
		updated, err = svc.ReadyTicket(ticket.ID, *message)
	} else {
		updated, err = svc.NotReadyTicket(ticket.ID, *message)
	}
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk complete [-id] <id> [-m comment]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	updated, err := svc.CompleteTicket(idVal, *message)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	fmt.Printf("%s completed (stage: done, complete: true)\n", updated.ID)
	return nil
}

func runReopen(args []string) error {
	fs := flag.NewFlagSet("reopen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
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
	updated, err := svc.ReopenTicket(idVal, *message)
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
	before, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	updated, err := svc.NextTicket(before.ID)
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
	before, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	updated, err := svc.PreviousTicket(before.ID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	fmt.Printf("%s regressed: %s -> %s (idle)\n", updated.ID, before.Status, updated.Status)
	return nil
}
