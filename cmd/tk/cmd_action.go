package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/libticket"
)

func runAction(args []string) error {
	if len(args) == 0 {
		return runList([]string{"-t", "action"})
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "help", "-h", "--help":
		fmt.Print(renderBanner())
		fmt.Print(renderCommandHelp("action"))
		printTicketEnvironment()
		return nil
	case "new", "add", "create":
		return runTypedTicketCreate("action", args[1:])
	case "ls", "list":
		return runList([]string{"-t", "action"})
	case "get", "show":
		if len(args) > 2 {
			return errors.New("usage: tk act get [<id>]")
		}
		id := ""
		if len(args) == 2 {
			id = args[1]
		}
		return runTypedTicketGet("action", id)
	case "update":
		return runActionUpdate(args[1:])
	case "complete", "done":
		if len(args) != 2 {
			return errors.New("usage: tk act complete <id>")
		}
		ticketRef, err := resolveActionTicketRef(args[1])
		if err != nil {
			return err
		}
		return runTicketStateAlias([]string{"-id", ticketRef}, "success", "complete")
	case "reject", "cancel":
		if len(args) != 2 {
			return errors.New("usage: tk act reject <id>")
		}
		ticketRef, err := resolveActionTicketRef(args[1])
		if err != nil {
			return err
		}
		return runTicketStateAlias([]string{"-id", ticketRef}, "fail", "fail")
	case "comment":
		return runActionComment(args[1:])
	case "edit":
		if len(args) != 2 {
			return errors.New("usage: tk act edit <id>")
		}
		ticketRef, err := resolveActionTicketRef(args[1])
		if err != nil {
			return err
		}
		return runEdit([]string{ticketRef})
	case "assign":
		if len(args) != 3 {
			return errors.New("usage: tk act assign <id> <name>")
		}
		ticketRef, err := resolveActionTicketRef(args[1])
		if err != nil {
			return err
		}
		return assignTicket(ticketRef, strings.TrimSpace(args[2]), false)
	case "unassign":
		if len(args) != 2 {
			return errors.New("usage: tk act unassign <id>")
		}
		ticketRef, err := resolveActionTicketRef(args[1])
		if err != nil {
			return err
		}
		return unassignActionTicket(ticketRef)
	default:
		return runTypedTicketCreate("action", args)
	}
}

func runActionUpdate(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tk act update <id> [flags]")
	}
	ticketRef, err := resolveActionTicketRef(args[0])
	if err != nil {
		return err
	}
	rewritten := make([]string, 0, len(args))
	rewritten = append(rewritten, ticketRef)
	for i := 1; i < len(args); i++ {
		if args[i] == "-due" {
			if i+1 >= len(args) {
				return errors.New("usage: tk act update <id> -due <yyyy-mm-dd>")
			}
			estimateComplete, convErr := actionDueToEstimateComplete(args[i+1])
			if convErr != nil {
				return convErr
			}
			rewritten = append(rewritten, "-estimate_complete", estimateComplete)
			i++
			continue
		}
		rewritten = append(rewritten, args[i])
	}
	return runUpdate(rewritten)
}

func runActionComment(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: tk act comment <id> -m <comment>")
	}
	ticketRef, err := resolveActionTicketRef(args[0])
	if err != nil {
		return err
	}
	commentArgs := args[1:]
	if len(commentArgs) >= 2 && commentArgs[0] == "-m" {
		comment := strings.TrimSpace(strings.Join(commentArgs[1:], " "))
		if comment == "" {
			return errors.New("usage: tk act comment <id> -m <comment>")
		}
		return runComment([]string{"-id", ticketRef, comment})
	}
	comment := strings.TrimSpace(strings.Join(commentArgs, " "))
	if comment == "" {
		return errors.New("usage: tk act comment <id> -m <comment>")
	}
	return runComment([]string{"-id", ticketRef, comment})
}

func resolveActionTicketRef(raw string) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return "", err
	}
	return normalizeBareTicketRef(cfg, svc, raw), nil
}

func actionDueToEstimateComplete(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("due date cannot be empty")
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed.Format(time.RFC3339), nil
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return value, nil
	}
	return "", fmt.Errorf("invalid due date %q (expected yyyy-mm-dd)", raw)
}

func unassignActionTicket(ticketRef string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	current, err := svc.GetTicket(context.Background(), ticketRef)
	if err != nil {
		return err
	}
	nextState := current.State
	if nextState == "active" {
		nextState = "idle"
	}
	updated, err := svc.UpdateTicket(context.Background(), current.ID, libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           "",
		State:              nextState,
		Stage:              current.Stage,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	fmt.Printf("unassigned %s\n", ticketLabel(updated))
	return nil
}
