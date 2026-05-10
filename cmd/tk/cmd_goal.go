package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/simonski/ticket/libticket"
)

func runGoal(args []string) error {
	if len(args) == 0 {
		fmt.Println(goalUsage)
		return nil
	}

	cfg, _, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	switch args[0] {
	case "create", "add", "new":
		fs := flag.NewFlagSet("goal create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		title := fs.String("title", "", "goal title")
		description := fs.String("d", "", "goal description")
		notes := fs.String("notes", "", "goal notes")
		eta := fs.String("eta", "", "goal ETA")
		priority := fs.Int("priority", 1, "goal priority")
		rest := args[1:]
		var positional []string
		for len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			positional = append(positional, rest[0])
			rest = rest[1:]
		}
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if *title == "" && len(positional) > 0 {
			*title = strings.Join(positional, " ")
		}
		if strings.TrimSpace(*title) == "" {
			return errors.New("usage: tk goal create -title <title> [-d <description>] [-notes <notes>] [-eta <eta>] [-priority <n>]")
		}
		goal, err := svc.CreateGoal(context.Background(), project.ID, libticket.GoalRequest{
			Title:       *title,
			Description: *description,
			Notes:       *notes,
			ETA:         *eta,
			Priority:    *priority,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(goal)
		}
		fmt.Printf("goal %d: %s\n", goal.ID, goal.Title)
		return nil
	case "list", "ls":
		goals, err := svc.ListGoals(context.Background(), project.ID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(goals)
		}
		if len(goals) == 0 {
			printNoEntitiesAvailable("goals")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tPRIORITY\tTITLE")
		for _, goal := range goals {
			fmt.Fprintf(w, "%d\t%s\t%d\t%s\n", goal.ID, goal.Status, goal.Priority, goal.Title)
		}
		return w.Flush()
	case "get", "show":
		if len(args) != 2 {
			return errors.New("usage: tk goal get <id>")
		}
		id, err := parseGoalID(args[1])
		if err != nil {
			return err
		}
		goal, err := svc.GetGoal(context.Background(), id)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(goal)
		}
		fmt.Printf("ID          : %d\n", goal.ID)
		fmt.Printf("ProjectID   : %d\n", goal.ProjectID)
		fmt.Printf("Title       : %s\n", goal.Title)
		fmt.Printf("Description : %s\n", goal.Description)
		fmt.Printf("Notes       : %s\n", goal.Notes)
		fmt.Printf("ETA         : %s\n", goal.ETA)
		fmt.Printf("Priority    : %d\n", goal.Priority)
		fmt.Printf("Status      : %s\n", goal.Status)
		fmt.Printf("Created     : %s\n", goal.CreatedAt)
		fmt.Printf("Updated     : %s\n", goal.UpdatedAt)
		return nil
	case "update":
		if len(args) < 2 {
			return errors.New("usage: tk goal update <id> [-title <title>] [-d <description>] [-notes <notes>] [-eta <eta>] [-priority <n>]")
		}
		id, err := parseGoalID(args[1])
		if err != nil {
			return err
		}
		fs := flag.NewFlagSet("goal update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		title := fs.String("title", "", "goal title")
		description := fs.String("d", "", "goal description")
		notes := fs.String("notes", "", "goal notes")
		eta := fs.String("eta", "", "goal ETA")
		priority := fs.Int("priority", 0, "goal priority")
		if parseErr := fs.Parse(args[2:]); parseErr != nil {
			return parseErr
		}
		current, err := svc.GetGoal(context.Background(), id)
		if err != nil {
			return err
		}
		if strings.TrimSpace(*title) == "" {
			*title = current.Title
		}
		if strings.TrimSpace(*description) == "" {
			*description = current.Description
		}
		if strings.TrimSpace(*notes) == "" {
			*notes = current.Notes
		}
		if strings.TrimSpace(*eta) == "" {
			*eta = current.ETA
		}
		if *priority == 0 {
			*priority = current.Priority
		}
		goal, err := svc.UpdateGoal(context.Background(), id, libticket.GoalRequest{
			Title:       *title,
			Description: *description,
			Notes:       *notes,
			ETA:         *eta,
			Priority:    *priority,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(goal)
		}
		fmt.Printf("goal %d updated: %s\n", goal.ID, goal.Title)
		return nil
	case "delete", "rm":
		if len(args) != 2 {
			return errors.New("usage: tk goal delete <id>")
		}
		id, err := parseGoalID(args[1])
		if err != nil {
			return err
		}
		if err := svc.DeleteGoal(context.Background(), id); err != nil {
			return err
		}
		fmt.Printf("deleted goal %d\n", id)
		return nil
	default:
		return runGoal(append([]string{"create"}, args...))
	}
}

func parseGoalID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid goal id %q", raw)
	}
	return id, nil
}
