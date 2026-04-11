package main

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"flag"
	"os"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func runCurate(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tk curate <id> [id...]")
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	var sourceIDs []string
	var titles []string
	for _, arg := range args {
		ticket, err := api.GetTicket(arg)
		if err != nil {
			return err
		}
		sourceIDs = append(sourceIDs, ticket.ID)
		titles = append(titles, ticket.Title)
	}
	title := "Curated requirement"
	if len(titles) > 0 {
		title = titles[0]
	}
	requirement, err := api.CreateTicket(libticket.TicketCreateRequest{
		ProjectID:   project.ID,
		Type:        "requirement",
		Title:       title,
		Description: "Curated from source items.",
	})
	if err != nil {
		return err
	}
	printTicket(requirement)
	return nil
}

// reviewStatusFilter holds stage/state filters for review vocabulary.
// "proposed" = design/idle, "accepted" = moved past design (develop stage), "rejected" = design/fail
type reviewFilter struct{ stage, state string }

var reviewStatusFilters = map[string]reviewFilter{
	"proposed": {store.StageDevelop, store.StateIdle},
	"accepted": {store.StageDone, ""},
	"rejected": {store.StageDevelop, store.StateFail},
}

func runReview(args []string) error {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	status := fs.String("status", "", "filter by review status: proposed, accepted, rejected")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	var stageFilter, stateFilter string
	if *status != "" {
		f, ok := reviewStatusFilters[strings.ToLower(strings.TrimSpace(*status))]
		if !ok {
			return fmt.Errorf("unknown review status %q; use proposed, accepted, or rejected", *status)
		}
		stageFilter = f.stage
		stateFilter = f.state
	}
	tickets, err := api.ListTicketsFiltered(project.ID, "requirement", stageFilter, stateFilter, "", "", "", 0, false)
	if err != nil {
		return err
	}
	for _, ticket := range tickets {
		fmt.Printf("%s\t%s\t%s\n", ticketLabel(ticket), ticket.Status, ticket.Title)
	}
	return nil
}

func runRequirementStatus(reviewStatus string, args []string) error {
	commandName := map[string]string{"accepted": "accept", "rejected": "reject"}[reviewStatus]
	if len(args) != 2 || args[0] != "requirement" {
		return fmt.Errorf("usage: tk %s requirement <id>", commandName)
	}
	stateToSet := map[string]string{"accepted": store.StateSuccess, "rejected": store.StateFail}[reviewStatus]
	return updateTicketState(args[1], stateToSet)
}

func runRevise(args []string) error {
	if len(args) != 2 || args[0] != "requirement" {
		return errors.New("usage: tk revise requirement <id>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	current, err := svc.GetTicket(args[1])
	if err != nil {
		return err
	}
	updated, err := svc.UpdateTicket(current.ID, libticket.TicketUpdateRequest{
		Title:              current.Title + " (revised)",
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           current.Assignee,
		State:              current.State,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
	})
	if err != nil {
		return err
	}
	printTicket(updated)
	return nil
}

// ---------------------------------------------------------------------------
// tk req — requirements namespace
// ---------------------------------------------------------------------------

func runReq(args []string) error {
	if len(args) == 0 {
		return runReqList(nil)
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Println(reqUsage)
		return nil
	case "add":
		return runReqAdd(args[1:])
	case "list", "ls":
		return runReqList(args[1:])
	case "get":
		return runReqGet(args[1:])
	case "shape":
		return runReqShape(args[1:])
	case "accept":
		return runReqAcceptReject("accept", args[1:])
	case "reject":
		return runReqAcceptReject("reject", args[1:])
	case "revise":
		return runReqRevise(args[1:])
	case "break":
		return runReqBreak(args[1:])
	case "pin":
		return runReqPin(args[1:])
	default:
		return fmt.Errorf("unknown req command %q; see: ticket req help", args[0])
	}
}

const reqUsage = `Usage: ticket req <command> [flags]

Commands:
  add    "title" [-d description] [-ac criteria]   Capture a new requirement
  list   [-status raw|shaping|accepted|rejected]    List requirements
  get    -id <id>                                   View requirement detail
  shape  -id <id> [-d text] [-ac text]              Refine a requirement
  break  -id <id> [--retry] [--reset]              Show/manage breakdown
  accept -id <id>                                   Approve a requirement
  reject -id <id>                                   Reject a requirement
  revise -id <id>                                   Send back for rethinking

Shortcuts:
  tk idea "title"    →  tk req add "title"
  tk ideas           →  tk req list`

func runIdea(args []string) error {
	if len(args) == 0 {
		fmt.Println(ideaUsage)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Println(ideaUsage)
		return nil
	case "ls", "list":
		return runReqList(args[1:])
	case "new", "add", "create":
		return runReqAdd(args[1:])
	case "get", "show":
		return runReqGet(args[1:])
	case "shape":
		return runReqShape(args[1:])
	case "accept":
		return runRequirementStatus("accepted", args[1:])
	case "reject":
		return runRequirementStatus("rejected", args[1:])
	default:
		// If the first arg doesn't look like a subcommand, treat as title for "new"
		return runReqAdd(args)
	}
}

func runReqAdd(args []string) error {
	return runTicketCreate(append([]string{"-type", "requirement"}, args...))
}

func runReqList(args []string) error {
	return runReview(args)
}

func runReqGet(args []string) error {
	return runGet(args)
}

func runReqShape(args []string) error {
	fs := flag.NewFlagSet("req shape", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "requirement ID")
	desc := fs.String("d", "", "description")
	ac := fs.String("ac", "", "acceptance criteria")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("usage: tk req shape -id <id> [-d description] [-ac criteria]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	current, err := svc.GetTicket(*id)
	if err != nil {
		return err
	}
	if current.Type != "requirement" {
		return fmt.Errorf("%s is a %s, not a requirement", current.ID, current.Type)
	}
	update := libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           current.Assignee,
		State:              current.State,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
	}
	if *desc != "" {
		update.Description = *desc
	}
	if *ac != "" {
		update.AcceptanceCriteria = *ac
	}
	updated, err := svc.UpdateTicket(current.ID, update)
	if err != nil {
		return err
	}
	printTicket(updated)
	return nil
}

func runReqAcceptReject(verb string, args []string) error {
	fs := flag.NewFlagSet("req "+verb, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "requirement ID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("usage: tk req %s -id <id>", verb)
	}
	stateToSet := map[string]string{"accept": store.StateSuccess, "reject": store.StateFail}[verb]
	return updateTicketState(*id, stateToSet)
}

func runReqRevise(args []string) error {
	fs := flag.NewFlagSet("req revise", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "requirement ID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("usage: tk req revise -id <id>")
	}
	return runRevise([]string{"requirement", *id})
}

func runReqBreak(args []string) error {
	fs := flag.NewFlagSet("req break", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "requirement ID")
	retry := fs.Bool("retry", false, "regenerate breakdown, keeping pinned items")
	reset := fs.Bool("reset", false, "discard all children and regenerate")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("usage: tk req break -id <id> [--retry] [--reset]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	req, err := svc.GetTicket(*id)
	if err != nil {
		return err
	}
	if req.Type != "requirement" {
		return fmt.Errorf("%s is a %s, not a requirement", req.ID, req.Type)
	}

	// List all tickets in the project, filter to children of this requirement.
	tickets, err := svc.ListTicketsFiltered(req.ProjectID, "", "", "", "", "", "", 0, false)
	if err != nil {
		return err
	}
	var children []store.Ticket
	for _, t := range tickets {
		if t.ParentID != nil && *t.ParentID == req.ID {
			children = append(children, t)
		}
	}

	if *reset {
		// Delete all unpinned children — for now, delete all (pin not yet tracked).
		for _, child := range children {
			if err := svc.DeleteTicket(child.ID); err != nil {
				return fmt.Errorf("failed to delete %s: %w", child.ID, err)
			}
			fmt.Printf("deleted %s: %s\n", child.ID, child.Title)
		}
		children = nil
	}

	_ = *retry // retry keeps pinned items; without pin tracking, behaves like showing current state

	if len(children) == 0 {
		fmt.Printf("no breakdown items for %s\n", req.ID)
		fmt.Println("hint: create child tickets with `tk add -parent <id> \"title\"` then re-run `tk req break -id <id>`")
		return nil
	}

	if outputJSON {
		return printJSON(children)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Breakdown of %s: %s\n\n", req.ID, req.Title)
	fmt.Fprintln(w, "KEY\tTYPE\tSTATUS\tTITLE")
	for _, child := range children {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", child.ID, child.Type, child.Status, child.Title)
	}
	return w.Flush()
}

func runReqPin(args []string) error {
	return errors.New("req pin is not yet implemented; planned for a future release")
}

func runDecision(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(decisionUsage)
		return nil
	}
	switch args[0] {
	case "add":
		if len(args) != 2 {
			return errors.New("usage: tk decision add \"text\"")
		}
		_, api, project, err := resolveCurrentProjectClient()
		if err != nil {
			return err
		}
		ticket, err := api.CreateTicket(libticket.TicketCreateRequest{
			ProjectID:   project.ID,
			Type:        "decision",
			Title:       args[1],
			Description: args[1],
		})
		if err != nil {
			return err
		}
		printTicket(ticket)
		return nil
	case "list":
		_, api, project, err := resolveCurrentProjectClient()
		if err != nil {
			return err
		}
		tickets, err := api.ListTicketsFiltered(project.ID, "decision", "", "", "", "", "", 0, false)
		if err != nil {
			return err
		}
		for _, ticket := range tickets {
			fmt.Printf("%s\t%s\t%s\n", ticketLabel(ticket), ticket.Status, ticket.Title)
		}
		return nil
	default:
		return fmt.Errorf("unknown decision command %q; see: ticket decision help", args[0])
	}
}

func runConversation(args []string) error {
	if len(args) < 2 || args[0] != "show" {
		return errors.New("usage: tk conversation show [-id] <id>")
	}
	return runHistory(args[1:])
}
