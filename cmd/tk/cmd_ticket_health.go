package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func runHealth(args []string) error {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, _, err := resolveIDFlag(*id, fs.Args())
	if err != nil {
		return errors.New("usage: tk health [-id] <id>|execute")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	if strings.EqualFold(idVal, "execute") {
		_, api, project, err := resolveCurrentProjectClient()
		if err != nil {
			return err
		}
		projectTickets, err := api.ListTickets(context.Background(), project.ID)
		if err != nil {
			return err
		}

		results := make([]map[string]any, 0, len(projectTickets))
		for _, ticket := range projectTickets {
			comments, err := svc.ListComments(context.Background(), ticket.ID)
			if err != nil {
				return err
			}
			checks, err := ticketHealthCheck(context.Background(), svc, ticket, comments)
			if err != nil {
				return err
			}
			updated, err := svc.SetTicketHealth(context.Background(), ticket.ID, checks.score)
			if err != nil {
				return err
			}
			result := map[string]any{
				"ticket_id":                  ticket.ID,
				"ticket_key":                 ticket.ID,
				"score":                      checks.score,
				"not_an_orphan":              checks.notOrphan,
				"has_acceptance_criteria":    checks.hasAC,
				"reviewed_by_reviewer_agent": checks.reviewedByReviewer,
				"definition_of_ready":        checks.ready,
				"project_acceptance_criteria": checks.projectAC,
				"project_definition_of_ready": checks.projectDoR,
				"project_definition_of_done":  checks.projectDoD,
				"sdlc_acceptance_criteria":    checks.sdlcAC,
				"stage_acceptance_criteria":   checks.stageAC,
				"ticket_acceptance_criteria":  checks.ticketAC,
				"persisted_score":            updated.HealthScore,
			}
			results = append(results, result)
		}

		if outputJSON {
			return printJSON(map[string]any{
				"ticket_health_execute": map[string]any{
					"tickets": len(results),
					"results": results,
				},
			})
		}

		fmt.Println("TICKET HEALTH EXECUTE")
		fmt.Printf("tickets: %d\n", len(results))
		for _, result := range results {
			label := fmt.Sprintf("%v", result["ticket_id"])
			if key, ok := result["ticket_key"].(string); ok && key != "" {
				label = key
			}
			if score, ok := result["score"].(int); ok {
				fmt.Printf("%s\t%.2f\n", label, float64(score)/float64(checksTotal()))
			} else {
				fmt.Printf("%s\t%s\n", label, fmt.Sprintf("%v", result["score"]))
			}
		}
		return nil
	}

	ticket, err := svc.GetTicket(context.Background(), idVal)
	if err != nil {
		return err
	}
	comments, err := svc.ListComments(context.Background(), ticket.ID)
	if err != nil {
		return err
	}

	checks, err := ticketHealthCheck(context.Background(), svc, ticket, comments)
	if err != nil {
		return err
	}
	updated, err := svc.SetTicketHealth(context.Background(), ticket.ID, checks.score)
	if err != nil {
		return err
	}
	section := map[string]any{
		"score":                      checks.score,
		"not_an_orphan":              checks.notOrphan,
		"has_acceptance_criteria":    checks.hasAC,
		"reviewed_by_reviewer_agent": checks.reviewedByReviewer,
		"definition_of_ready":        checks.ready,
		"project_acceptance_criteria": checks.projectAC,
		"project_definition_of_ready": checks.projectDoR,
		"project_definition_of_done":  checks.projectDoD,
		"sdlc_acceptance_criteria":    checks.sdlcAC,
		"stage_acceptance_criteria":   checks.stageAC,
		"ticket_acceptance_criteria":  checks.ticketAC,
	}
	if outputJSON {
		return printJSON(map[string]any{
			"ticket_health": section,
			"ticket": map[string]any{
				"ticket_id":    ticket.ID,
				"ticket_key":   ticketLabel(ticket),
				"health_score": updated.HealthScore,
			},
		})
	}
	fmt.Println("TICKET HEALTH")
	fmt.Printf("score: %.2f\n", float64(checks.score)/float64(checksTotal()))
	fmt.Printf("not_an_orphan: %t\n", checks.notOrphan)
	fmt.Printf("has_acceptance_criteria: %t\n", checks.hasAC)
	fmt.Printf("reviewed_by_reviewer_agent: %t\n", checks.reviewedByReviewer)
	fmt.Printf("definition_of_ready: %t\n", checks.ready)
	fmt.Printf("project_acceptance_criteria: %t\n", checks.projectAC)
	fmt.Printf("project_definition_of_ready: %t\n", checks.projectDoR)
	fmt.Printf("project_definition_of_done: %t\n", checks.projectDoD)
	fmt.Printf("sdlc_acceptance_criteria: %t\n", checks.sdlcAC)
	fmt.Printf("stage_acceptance_criteria: %t\n", checks.stageAC)
	fmt.Printf("ticket_acceptance_criteria: %t\n", checks.ticketAC)
	return nil
}

type ticketHealthResult struct {
	score              int
	notOrphan          bool
	hasAC              bool
	reviewedByReviewer bool
	ready              bool
	projectAC          bool
	projectDoR         bool
	projectDoD         bool
	sdlcAC             bool
	stageAC            bool
	ticketAC           bool
}

func ticketHealthCheck(ctx context.Context, svc libticket.Service, ticket store.Ticket, comments []store.Comment) (ticketHealthResult, error) {
	notOrphan := ticket.Type == "epic" || ticket.ParentID != nil
	ticketAC := strings.TrimSpace(ticket.AcceptanceCriteria) != ""
	hasAC := ticketAC
	reviewedByReviewer := hasReviewerAgentComment(comments)
	ready := ticket.Status == "develop/idle"
	if !ready {
		stage, state, err := store.ParseLifecycleStatus(ticket.Status)
		if err == nil {
			ready = stage == store.StageDevelop && state == store.StateIdle
		}
	}

	project, err := svc.GetProject(ctx, strconv.FormatInt(ticket.ProjectID, 10))
	if err != nil {
		return ticketHealthResult{}, err
	}
	projectAC := strings.TrimSpace(project.AcceptanceCriteria) != ""
	// Project DoR currently maps to the project's acceptance criteria field.
	projectDoR := projectAC
	projectDoD := strings.TrimSpace(project.Notes) != ""

	sdlcAC := false
	stageAC := false
	sdlcID := ticket.SdlcID
	if sdlcID == nil {
		sdlcID = project.SdlcID
	}
	if sdlcID != nil {
		wf, wfErr := svc.GetSdlc(ctx, *sdlcID)
		if wfErr != nil {
			return ticketHealthResult{}, wfErr
		}
		sdlcAC = strings.TrimSpace(wf.Description) != ""
		stage, stageErr := resolveStageForPrompt(ctx, svc, ticket, project)
		if stageErr != nil {
			return ticketHealthResult{}, stageErr
		}
		if stage != nil {
			stageAC = strings.TrimSpace(stage.DefinitionOfReady) != "" || strings.TrimSpace(stage.AcceptanceCriteria) != ""
		}
	}

	checks := []bool{
		notOrphan, hasAC, reviewedByReviewer, ready,
		projectAC, projectDoR, projectDoD,
		sdlcAC, stageAC, ticketAC,
	}
	score := 0
	for _, ok := range checks {
		if ok {
			score++
		}
	}
	return ticketHealthResult{
		score:              score,
		notOrphan:          notOrphan,
		hasAC:              hasAC,
		reviewedByReviewer: reviewedByReviewer,
		ready:              ready,
		projectAC:          projectAC,
		projectDoR:         projectDoR,
		projectDoD:         projectDoD,
		sdlcAC:             sdlcAC,
		stageAC:            stageAC,
		ticketAC:           ticketAC,
	}, nil
}

func checksTotal() int {
	return 10
}

func hasReviewerAgentComment(comments []store.Comment) bool {
	for _, comment := range comments {
		if isReviewerAuthor(comment.Author) || isReviewerCommentText(comment.Text) {
			return true
		}
	}
	return false
}

func isReviewerAuthor(author string) bool {
	a := strings.ToLower(strings.TrimSpace(author))
	return strings.Contains(a, "reviewer")
}

func isReviewerCommentText(commentText string) bool {
	text := strings.ToLower(strings.TrimSpace(commentText))
	for _, term := range []string{"reviewer", "reviewed", "approved", "approval"} {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func runDoctor(args []string) error {
	if len(args) == 0 {
		fmt.Println(`Usage: ticket doctor <target> [flags]

Targets:
  project [-id <id>]    Review project health
  ticket  [-id <id>]    Review ticket health`)
		return nil
	}

	_, svc, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	reader := bufio.NewReader(os.Stdin)

	switch args[0] {
	case "project":
		fs := flag.NewFlagSet("doctor project", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "project id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			*id = project.ID
		}
		proj, err := svc.GetProject(context.Background(), strconv.FormatInt(*id, 10))
		if err != nil {
			return fmt.Errorf("project %d not found: %w", *id, err)
		}

		fmt.Printf("=== Project Doctor: %s — %s ===\n\n", proj.Prefix, proj.Title)

		// Sdlc check
		if proj.SdlcID == nil {
			fmt.Println("[WARN] Project has no sdlc assigned")
		} else {
			wf, err := svc.GetSdlc(context.Background(), *proj.SdlcID)
			if err == nil {
				fmt.Printf("Sdlc: %s (%d stages)\n", wf.Name, len(wf.Stages))
				for _, s := range wf.Stages {
					var roleNames []string
					for _, r := range s.Roles {
						roleNames = append(roleNames, r.Title)
					}
					role := "-"
					if len(roleNames) > 0 {
						role = strings.Join(roleNames, ", ")
					}
					fmt.Printf("  %s (roles: %s)\n", s.StageName, role)
				}
			}
		}

		// Ticket stats
		tickets, err := svc.ListTickets(context.Background(), proj.ID)
		if err != nil {
			return err
		}
		fmt.Printf("\nTickets: %d total\n", len(tickets))

		var noDesc, noAC, noAssignee, notReady, stale int
		for _, t := range tickets {
			if t.Complete || t.Archived {
				continue
			}
			if strings.TrimSpace(t.Description) == "" {
				noDesc++
			}
			if strings.TrimSpace(t.AcceptanceCriteria) == "" {
				noAC++
			}
			if strings.TrimSpace(t.Assignee) == "" && t.State == store.StateActive {
				noAssignee++
			}
			if t.Draft && t.State == store.StateIdle {
				notReady++
			}
			if t.State == store.StateActive && strings.TrimSpace(t.Assignee) == "" {
				stale++
			}
		}

		fmt.Println("\nIssues found:")
		issues := 0
		if noDesc > 0 {
			fmt.Printf("  [WARN] %d ticket(s) have no description\n", noDesc)
			issues += noDesc
		}
		if noAC > 0 {
			fmt.Printf("  [WARN] %d ticket(s) have no acceptance criteria\n", noAC)
			issues += noAC
		}
		if noAssignee > 0 {
			fmt.Printf("  [WARN] %d active ticket(s) have no assignee\n", noAssignee)
			issues += noAssignee
		}
		if notReady > 0 {
			fmt.Printf("  [INFO] %d idle ticket(s) are not marked ready\n", notReady)
		}
		if issues == 0 {
			fmt.Println("  No issues found.")
		}

		// Interactive: offer to run health scores
		fmt.Println()
		if promptYN(reader, "Run health scoring on all open tickets?", false) {
			for _, t := range tickets {
				if t.Complete || t.Archived {
					continue
				}
				comments, _ := svc.ListComments(context.Background(), t.ID)
				checks, checkErr := ticketHealthCheck(context.Background(), svc, t, comments)
				if checkErr != nil {
					fmt.Printf("  [ERR] %s: %v\n", t.ID, checkErr)
					continue
				}
				if _, err := svc.SetTicketHealth(context.Background(), t.ID, checks.score); err != nil {
					fmt.Printf("  [ERR] %s: %v\n", t.ID, err)
				} else {
					fmt.Printf("  %s: score %d/%d\n", t.ID, checks.score, checksTotal())
				}
			}
		}
		return nil

	case "ticket":
		fs := flag.NewFlagSet("doctor ticket", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "ticket id or key")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		idVal, _, err := resolveIDFlag(*id, fs.Args())
		if err != nil || idVal == "" {
			return errors.New("usage: tk doctor ticket [-id] <id>")
		}
		ticket, err := svc.GetTicket(context.Background(), idVal)
		if err != nil {
			return err
		}

		fmt.Printf("=== Ticket Doctor: %s — %s ===\n\n", ticket.ID, ticket.Title)
		fmt.Printf("Type:     %s\n", ticket.Type)
		fmt.Printf("Status:   %s\n", ticket.Status)
		fmt.Printf("Assignee: %s\n", orDash(ticket.Assignee))
		fmt.Printf("Draft:    %t\n", ticket.Draft)
		fmt.Printf("Priority: %d\n", ticket.Priority)

		// Context — open DB directly for enrichment
		var ctx store.TicketContext
		if resolved, err := config.ResolveURL(); err == nil && resolved.DBPath != "" {
			if db, err := store.Open(resolved.DBPath); err == nil {
				ctx = store.EnrichTicketContext(context.Background(), db, ticket)
				if closeErr := db.Close(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "warning: could not close database: %v\n", closeErr)
				}
			}
		}

		if ctx.Project != nil {
			fmt.Printf("Project:  %s — %s\n", ctx.Project.Prefix, ctx.Project.Title)
		}
		if ctx.Sdlc != nil {
			fmt.Printf("Sdlc: %s\n", ctx.Sdlc.Name)
		}
		if ctx.Role != nil {
			fmt.Printf("Role:     %s\n", ctx.Role.Title)
		}
		if len(ctx.Parents) > 0 {
			fmt.Printf("Parents:  ")
			for i, p := range ctx.Parents {
				if i > 0 {
					fmt.Print(" → ")
				}
				fmt.Printf("%s", p.ID)
			}
			fmt.Println()
		}

		// Issues
		fmt.Println("\nIssues found:")
		issues := 0
		if strings.TrimSpace(ticket.Description) == "" {
			fmt.Println("  [WARN] No description")
			issues++
		}
		if strings.TrimSpace(ticket.AcceptanceCriteria) == "" {
			fmt.Println("  [WARN] No acceptance criteria")
			issues++
		}
		if ticket.Type != "epic" && ticket.ParentID == nil {
			fmt.Println("  [WARN] Orphan ticket (no parent)")
			issues++
		}
		if ticket.State == store.StateActive && strings.TrimSpace(ticket.Assignee) == "" {
			fmt.Println("  [WARN] Active but no assignee")
			issues++
		}
		if ticket.SdlcID == nil && ctx.Sdlc == nil {
			fmt.Println("  [WARN] No sdlc (inherited or explicit)")
			issues++
		}
		if issues == 0 {
			fmt.Println("  No issues found.")
		}

		// Interactive actions
		fmt.Println()
		if strings.TrimSpace(ticket.Description) == "" {
			if promptYN(reader, "Add a description?", false) {
				fmt.Print("Description: ")
				desc, _ := reader.ReadString('\n')
				desc = strings.TrimSpace(desc)
				if desc != "" {
					if _, err := svc.UpdateTicket(context.Background(), ticket.ID, libticket.TicketUpdateRequest{Description: desc}); err != nil {
						fmt.Printf("  [ERR] %v\n", err)
					} else {
						fmt.Println("  Updated.")
					}
				}
			}
		}
		if strings.TrimSpace(ticket.AcceptanceCriteria) == "" {
			if promptYN(reader, "Add acceptance criteria?", false) {
				fmt.Print("Acceptance Criteria: ")
				ac, _ := reader.ReadString('\n')
				ac = strings.TrimSpace(ac)
				if ac != "" {
					if _, err := svc.UpdateTicket(context.Background(), ticket.ID, libticket.TicketUpdateRequest{AcceptanceCriteria: ac}); err != nil {
						fmt.Printf("  [ERR] %v\n", err)
					} else {
						fmt.Println("  Updated.")
					}
				}
			}
		}
		if ticket.Draft && ticket.State == store.StateIdle {
			if promptYN(reader, "Mark ticket as ready?", false) {
				if _, err := svc.ReadyTicket(context.Background(), ticket.ID, ""); err != nil {
					fmt.Printf("  [ERR] %v\n", err)
				} else {
					fmt.Println("  Marked ready.")
				}
			}
		}

		// Health score
		comments, _ := svc.ListComments(context.Background(), ticket.ID)
		checks, checkErr := ticketHealthCheck(context.Background(), svc, ticket, comments)
		if checkErr != nil {
			return checkErr
		}
		if _, err := svc.SetTicketHealth(context.Background(), ticket.ID, checks.score); err == nil {
			fmt.Printf("\nHealth score: %d/%d\n", checks.score, checksTotal())
		}
		return nil

	default:
		return fmt.Errorf("unknown doctor target %q; use: project, ticket", args[0])
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
