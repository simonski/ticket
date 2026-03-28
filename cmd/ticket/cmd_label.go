package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/libticket"
)

func runLabel(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(labelUsage)
		return nil
	}
	_, svc, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	switch args[0] {
	case "list", "ls":
		labels, err := svc.ListLabels(project.ID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(labels)
		}
		if len(labels) == 0 {
			fmt.Println("no labels")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tCOLOR")
		for _, l := range labels {
			fmt.Fprintf(w, "%d\t%s\t%s\n", l.ID, l.Name, l.Color)
		}
		return w.Flush()
	case "create":
		fs := flag.NewFlagSet("label create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "label name")
		color := fs.String("color", "", "label color (e.g. #ff0000)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" && fs.NArg() > 0 {
			*name = fs.Arg(0)
		}
		if *name == "" {
			return errors.New("usage: ticket label create <name> [-color <color>]")
		}
		label, err := svc.CreateLabel(project.ID, libticket.LabelRequest{Name: *name, Color: *color})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(label)
		}
		fmt.Printf("label created: %d %s\n", label.ID, label.Name)
		return nil
	case "delete":
		fs := flag.NewFlagSet("label delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.String("id", "", "label ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		idStr := *idFlag
		if idStr == "" && fs.NArg() > 0 {
			idStr = fs.Arg(0)
		}
		if idStr == "" {
			return errors.New("usage: ticket label delete -id <label-id>")
		}
		var id int64
		if _, err := fmt.Sscan(idStr, &id); err != nil {
			return errors.New("label id must be numeric")
		}
		return svc.DeleteLabel(id)
	case "add":
		fs := flag.NewFlagSet("label add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.String("id", "", "ticket ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		var ticketID string
		var labelID int64
		if *idFlag != "" && fs.NArg() > 0 {
			ticketID = *idFlag
			if _, err := fmt.Sscan(fs.Arg(0), &labelID); err != nil {
				return errors.New("label id must be numeric")
			}
		} else if fs.NArg() >= 2 {
			// positional fallback
			ticketID = fs.Arg(0)
			if _, err := fmt.Sscan(fs.Arg(1), &labelID); err != nil {
				return errors.New("label id must be numeric")
			}
		} else {
			return errors.New("usage: ticket label add -id <ticket-id> <label-id>")
		}
		return svc.AddTicketLabel(ticketID, labelID)
	case "remove":
		fs := flag.NewFlagSet("label remove", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.String("id", "", "ticket ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		var ticketID string
		var labelID int64
		if *idFlag != "" && fs.NArg() > 0 {
			ticketID = *idFlag
			if _, err := fmt.Sscan(fs.Arg(0), &labelID); err != nil {
				return errors.New("label id must be numeric")
			}
		} else if fs.NArg() >= 2 {
			ticketID = fs.Arg(0)
			if _, err := fmt.Sscan(fs.Arg(1), &labelID); err != nil {
				return errors.New("label id must be numeric")
			}
		} else {
			return errors.New("usage: ticket label remove -id <ticket-id> <label-id>")
		}
		return svc.RemoveTicketLabel(ticketID, labelID)
	case "show":
		fs := flag.NewFlagSet("label show", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.String("id", "", "ticket ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		idStr := *idFlag
		if idStr == "" && fs.NArg() > 0 {
			idStr = fs.Arg(0)
		}
		if idStr == "" {
			return errors.New("usage: ticket label show -id <ticket-id>")
		}
		ticketID := idStr
		labels, err := svc.ListTicketLabels(ticketID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(labels)
		}
		if len(labels) == 0 {
			fmt.Println("no labels")
			return nil
		}
		for _, l := range labels {
			fmt.Printf("%d\t%s\n", l.ID, l.Name)
		}
		return nil
	default:
		return fmt.Errorf("unknown label command %q; see: ticket label help", args[0])
	}
}

func runTime(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(timeUsage)
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	switch args[0] {
	case "log", "add":
		fs := flag.NewFlagSet("time log", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		ticketID := fs.String("id", "", "ticket id")
		minutes := fs.Int("m", 0, "minutes spent")
		note := fs.String("note", "", "optional note")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *ticketID == "" || *minutes <= 0 {
			return errors.New("usage: ticket time log -id <ticket-id> -m <minutes> [-note <text>]")
		}
		entry, err := svc.LogTime(*ticketID, libticket.TimeEntryRequest{Minutes: *minutes, Note: *note})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(entry)
		}
		fmt.Printf("logged %d min on ticket %s\n", entry.Minutes, entry.TicketID)
		return nil
	case "list", "ls":
		fs := flag.NewFlagSet("time list", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.String("id", "", "ticket ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		ticketID := *idFlag
		if ticketID == "" && fs.NArg() > 0 {
			ticketID = fs.Arg(0)
		}
		if ticketID == "" {
			return errors.New("usage: ticket time list -id <ticket-id>")
		}
		entries, err := svc.ListTimeEntries(ticketID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(entries)
		}
		if len(entries) == 0 {
			fmt.Println("no time entries")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tMINUTES\tUSER\tNOTE\tDATE")
		for _, e := range entries {
			fmt.Fprintf(w, "%d\t%d\t%s\t%s\t%s\n", e.ID, e.Minutes, e.UserID, e.Note, e.CreatedAt)
		}
		return w.Flush()
	case "total":
		fs := flag.NewFlagSet("time total", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.String("id", "", "ticket ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		ticketID := *idFlag
		if ticketID == "" && fs.NArg() > 0 {
			ticketID = fs.Arg(0)
		}
		if ticketID == "" {
			return errors.New("usage: ticket time total -id <ticket-id>")
		}
		total, err := svc.TotalTimeForTicket(ticketID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]int{"total": total})
		}
		hours := total / 60
		mins := total % 60
		if hours > 0 {
			fmt.Printf("%dh %dm (%d min total)\n", hours, mins, total)
		} else {
			fmt.Printf("%d min\n", total)
		}
		return nil
	case "delete":
		fs := flag.NewFlagSet("time delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.Int64("id", 0, "time entry ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		id := *idFlag
		if id == 0 && fs.NArg() > 0 {
			if _, err := fmt.Sscan(fs.Arg(0), &id); err != nil {
				return errors.New("time entry id must be numeric")
			}
		}
		if id == 0 {
			return errors.New("usage: ticket time delete -id <entry-id>")
		}
		return svc.DeleteTimeEntry(id)
	default:
		return fmt.Errorf("unknown time command %q; see: ticket time help", args[0])
	}
}
