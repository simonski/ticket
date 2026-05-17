package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/simonski/ticket/libticket"
)

func runStory(args []string) error {
	if len(args) == 0 {
		fmt.Println(storyUsage)
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
		fs := flag.NewFlagSet("story create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "force story id")
		printID := fs.Bool("printid", false, "print only the created story id")
		title := fs.String("title", "", "story title")
		description := fs.String("d", "", "story description")
		// Pull positional title before flags so flag parser sees flags only.
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
			return errors.New("usage: tk story create -title <title> [-id <id>] [-d description]")
		}
		story, err := svc.CreateStoryWithRequest(context.Background(), libticket.StoryCreateRequest{
			ID:          optionalInt64Flag(*id),
			ProjectID:   project.ID,
			Title:       *title,
			Description: *description,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(story)
		}
		if printCreatedID(story.ID, *printID) {
			return nil
		}
		fmt.Printf("story %d: %s\n", story.ID, story.Title)
		return nil
	case "list", "ls":
		stories, err := svc.ListStories(context.Background(), project.ID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(stories)
		}
		if len(stories) == 0 {
			printNoEntitiesAvailable("stories")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tTITLE")
		for _, s := range stories {
			fmt.Fprintf(w, "%d\t%s\t%s\n", s.ID, s.Status, s.Title)
		}
		return w.Flush()
	case "get", "show":
		if len(args) > 2 {
			return errors.New("usage: tk story get <id>")
		}
		var rawID string
		if len(args) == 2 {
			rawID = args[1]
		}
		id, err := resolveStoryID(svc, project.ID, rawID)
		if err != nil {
			return err
		}
		story, err := svc.GetStory(context.Background(), id)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(story)
		}
		fmt.Printf("ID          : %d\n", story.ID)
		fmt.Printf("ProjectID   : %d\n", story.ProjectID)
		fmt.Printf("Title       : %s\n", story.Title)
		fmt.Printf("Description : %s\n", story.Description)
		fmt.Printf("Status      : %s\n", story.Status)
		fmt.Printf("Created     : %s\n", story.CreatedAt)
		fmt.Printf("Updated     : %s\n", story.UpdatedAt)
		return nil
	case "update":
		if len(args) < 2 {
			return errors.New("usage: tk story update <id> -title <title> [-d description]")
		}
		var id int64
		if _, err := fmt.Sscan(args[1], &id); err != nil {
			return fmt.Errorf("invalid story id %q", args[1])
		}
		fs := flag.NewFlagSet("story update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		title := fs.String("title", "", "story title")
		description := fs.String("d", "", "story description")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		// Fetch current to use as defaults
		current, err := svc.GetStory(context.Background(), id)
		if err != nil {
			return err
		}
		if strings.TrimSpace(*title) == "" {
			*title = current.Title
		}
		if strings.TrimSpace(*description) == "" {
			*description = current.Description
		}
		story, err := svc.UpdateStory(context.Background(), id, *title, *description)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(story)
		}
		fmt.Printf("story %d updated: %s\n", story.ID, story.Title)
		return nil
	case "delete":
		if len(args) != 2 {
			return errors.New("usage: tk story delete <id>")
		}
		var id int64
		if _, err := fmt.Sscan(args[1], &id); err != nil {
			return fmt.Errorf("invalid story id %q", args[1])
		}
		if err := svc.DeleteStory(context.Background(), id); err != nil {
			return err
		}
		fmt.Printf("deleted story %d\n", id)
		return nil
	default:
		return runStory(append([]string{"create"}, args...))
	}
}

func runEpic(args []string) error {
	// Subcommands: get/use <id>, clear, list/ls — otherwise fall through to create
	if len(args) > 0 {
		switch args[0] {
		case "get":
			if len(args) > 2 {
				return errors.New("usage: tk epic get <id>")
			}
			id := ""
			if len(args) == 2 {
				id = args[1]
			}
			if strings.TrimSpace(id) == "" {
				return errors.New("usage: tk epic get <id>")
			}
			return runTypedTicketGet("epic", id)
		case "use", "clear":
			return errors.New("tk epic use/clear has been removed; pass -parent explicitly when creating child tickets")
		case "list", "ls":
			cfg, _, project, err := resolveCurrentProjectClient()
			if err != nil {
				return err
			}
			svc, err := resolveService(cfg)
			if err != nil {
				return err
			}
			epics, err := svc.ListTicketsFiltered(context.Background(), project.ID, "epic", "", "", "", "", "", 0, false)
			if err != nil {
				return err
			}
			if outputJSON {
				return printJSON(epics)
			}
			if len(epics) == 0 {
				printNoEntitiesAvailable("epics")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "KEY\tSTATUS\tTITLE")
			for _, t := range epics {
				fmt.Fprintf(w, "%s\t%s\t%s\n", t.ID, t.Status, t.Title)
			}
			return w.Flush()
		}
	}
	return runTypedTicketCreate("epic", args)
}
