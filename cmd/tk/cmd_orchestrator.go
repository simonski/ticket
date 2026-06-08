package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/orchestrator"
	"github.com/simonski/ticket/internal/store"
)

// runOrchestrator implements `tk orchestrator` — an admin tool that reports (and,
// with -apply, performs) the deterministic actions the orchestrator would take.
//
//	tk orchestrator -id TDA-90      # one ticket
//	tk orchestrator TDA-90          # shorthand for -id TDA-90
//	tk orchestrator -project_id 3   # all tickets in a project
//	tk orchestrator                 # all tickets, all projects
//	tk orchestrator -apply ...      # actually apply, instead of dry-run
func runOrchestrator(args []string) error {
	fs := flag.NewFlagSet("orchestrator", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	idFlag := fs.String("id", "", "consider a single ticket by id (e.g. TDA-90)")
	projectID := fs.Int64("project_id", 0, "consider all tickets in a project")
	apply := fs.Bool("apply", false, "apply the actions (default is dry-run: report only)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Shorthand: `tk orchestrator N` == `-id N`.
	ticketID := strings.TrimSpace(*idFlag)
	if ticketID == "" {
		if rest := fs.Args(); len(rest) > 0 {
			ticketID = strings.TrimSpace(rest[0])
		}
	}

	// Resolve the database the same way the rest of the CLI does (respects the
	// global -f override and $TICKET_HOME). The orchestrator operates directly on a
	// local database.
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	if resolved.Mode != config.ModeLocal {
		return fmt.Errorf("tk orchestrator runs against a local database, but the active location is remote (%s); run it on the server host or pass -f <db>", resolved.ServerURL)
	}
	db, err := store.Open(resolved.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	decisions, err := orchestrator.Pass(context.Background(), db, orchestrator.Options{
		DryRun:    !*apply,
		ProjectID: *projectID,
		TicketID:  ticketID,
	})
	if err != nil {
		return err
	}

	mode := "dry-run"
	if *apply {
		mode = "apply"
	}
	fmt.Printf("Orchestrator %s — %d ticket(s) considered\n\n", mode, len(decisions))

	counts := map[orchestrator.ActionKind]int{}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "TICKET\tFROM\tACTION\tAGENT\tDETAIL")
	for _, d := range decisions {
		counts[d.Kind]++
		action := string(d.Kind)
		if *apply && d.Kind != orchestrator.ActionSkip {
			if d.Applied {
				action += " ✓"
			} else if d.Err != "" {
				action += " ✗"
			}
		}
		detail := d.Detail
		if d.Err != "" {
			detail = "error: " + d.Err
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", d.TicketID, d.From, action, d.Agent, detail)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	fmt.Printf("\nSummary: %d assign, %d advance, %d recover, %d abandon, %d skip\n",
		counts[orchestrator.ActionAssign], counts[orchestrator.ActionAdvance],
		counts[orchestrator.ActionRecover], counts[orchestrator.ActionAbandon],
		counts[orchestrator.ActionSkip])
	return nil
}
