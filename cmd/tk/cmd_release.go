package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

func runRelease(args []string) error {
	if len(args) == 0 {
		fmt.Println(releaseUsage)
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
	case "list", "ls":
		releases, err := svc.ListReleases(context.Background(), project.ID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(releases)
		}
		if len(releases) == 0 {
			printNoEntitiesAvailable("releases")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tTITLE\tTARGET\tFEATURES\tSTORIES")
		for _, r := range releases {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%d\n", r.ID, r.Status, r.Title, r.TargetDate, r.FeatureCount, r.StoryCount)
		}
		return w.Flush()
	case "create", "add", "new":
		fs := flag.NewFlagSet("release create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		title := fs.String("title", "", "release title")
		purpose := fs.String("purpose", "", "release purpose")
		date := fs.String("date", "", "target date (YYYY-MM-DD)")
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
			return errors.New("usage: tk release create -title <title> [-purpose <p>] [-date <YYYY-MM-DD>]")
		}
		release, err := svc.CreateRelease(context.Background(), project.ID, *title, *purpose, *date)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(release)
		}
		fmt.Printf("release %d: %s\n", release.ID, release.Title)
		return nil
	case "status":
		fs := flag.NewFlagSet("release status", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "release id")
		status := fs.String("status", "", "release status (in_design|in_progress|complete)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 || strings.TrimSpace(*status) == "" {
			return errors.New("usage: tk release status -id <N> -status <in_design|in_progress|complete>")
		}
		release, err := svc.SetReleaseStatus(context.Background(), *id, *status)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(release)
		}
		fmt.Printf("release %d status: %s\n", release.ID, release.Status)
		return nil
	case "update":
		fs := flag.NewFlagSet("release update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "release id")
		title := fs.String("title", "", "release title")
		purpose := fs.String("purpose", "", "release purpose")
		date := fs.String("date", "", "target date (YYYY-MM-DD)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: tk release update -id <N> [-title <t>] [-purpose <p>] [-date <YYYY-MM-DD>]")
		}
		current, err := svc.GetRelease(context.Background(), *id)
		if err != nil {
			return err
		}
		if strings.TrimSpace(*title) == "" {
			*title = current.Title
		}
		if strings.TrimSpace(*purpose) == "" {
			*purpose = current.Purpose
		}
		if strings.TrimSpace(*date) == "" {
			*date = current.TargetDate
		}
		release, err := svc.UpdateRelease(context.Background(), *id, *title, *purpose, *date)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(release)
		}
		fmt.Printf("release %d updated: %s\n", release.ID, release.Title)
		return nil
	case "add-feature", "feature-add":
		fs := flag.NewFlagSet("release add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "release id")
		feature := fs.String("feature", "", "feature ticket key")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 || strings.TrimSpace(*feature) == "" {
			return errors.New("usage: tk release add -id <N> -feature <TICKET-KEY>")
		}
		if err := svc.AddFeatureToRelease(context.Background(), *feature, *id); err != nil {
			return err
		}
		fmt.Printf("added feature %s to release %d\n", *feature, *id)
		return nil
	case "remove", "remove-feature", "feature-remove":
		fs := flag.NewFlagSet("release remove", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		feature := fs.String("feature", "", "feature ticket key")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*feature) == "" {
			return errors.New("usage: tk release remove -feature <TICKET-KEY>")
		}
		if err := svc.RemoveFeatureFromRelease(context.Background(), *feature); err != nil {
			return err
		}
		fmt.Printf("removed feature %s from its release\n", *feature)
		return nil
	case "delete", "rm":
		fs := flag.NewFlagSet("release delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "release id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: tk release delete -id <N>")
		}
		if err := svc.DeleteRelease(context.Background(), *id); err != nil {
			return err
		}
		fmt.Printf("deleted release %d\n", *id)
		return nil
	default:
		fmt.Println(releaseUsage)
		return nil
	}
}

func runFeature(args []string) error {
	if len(args) == 0 {
		fmt.Println(featureUsage)
		return nil
	}

	cfg, _, _, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	switch args[0] {
	case "clone":
		fs := flag.NewFlagSet("feature clone", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "feature ticket key")
		rest := args[1:]
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			*id = rest[0]
			rest = rest[1:]
		}
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if strings.TrimSpace(*id) == "" {
			return errors.New("usage: tk feature clone -id <TICKET-KEY>")
		}
		ticket, err := svc.CloneFeature(context.Background(), *id)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(ticket)
		}
		fmt.Printf("cloned feature %s -> %s\n", *id, ticket.ID)
		return nil
	default:
		fmt.Println(featureUsage)
		return nil
	}
}
