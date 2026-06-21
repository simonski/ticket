package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

const prUsage = `Usage: tk pr <command> [flags]

Commands:
  create [-id] <ticket-id> [-repo R] [-from B] [-to B] [-title T] [-url U] [-provider none|github] [-m desc]
                                        Open a pull request for a ticket. Repository,
                                        branches, and provider are inferred from the
                                        project repos and the current git branch.
  ls     [-id] <ticket-id>             List a ticket's pull requests
  get    <pr-id>                       Show a pull request

Shortcut: tk pr <ticket-id> lists that ticket's pull requests.`

func runPullRequest(args []string) error {
	if len(args) == 0 {
		return errors.New(prUsage)
	}
	switch args[0] {
	case "create", "new", "add", "open":
		return runPullRequestCreate(args[1:])
	case "ls", "list":
		return runPullRequestList(args[1:])
	case "get", "show":
		return runPullRequestGet(args[1:])
	case "help", "-h", "--help":
		fmt.Println(prUsage)
		return nil
	default:
		// Shorthand: `tk pr <ticket-id>` lists that ticket's pull requests.
		if !strings.HasPrefix(args[0], "-") {
			return runPullRequestList(args)
		}
		return errors.New(prUsage)
	}
}

// detectPRProvider returns the provider implied by a repository reference.
func detectPRProvider(repository string) string {
	if strings.Contains(strings.ToLower(repository), "github.com") {
		return store.PullRequestProviderGitHub
	}
	return store.PullRequestProviderNone
}

// currentGitBranch returns the checked-out branch in the current directory, or "".
func currentGitBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output() // #nosec G204 -- fixed command, no user input
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func runPullRequestCreate(args []string) error {
	fs := flag.NewFlagSet("pr create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	idFlag := fs.String("id", "", "ticket id")
	repo := fs.String("repo", "", "repository (default: project repo or cwd git origin)")
	from := fs.String("from", "", "source branch (default: current git branch)")
	to := fs.String("to", "", "target branch (default: main)")
	title := fs.String("title", "", "PR title (default: ticket title)")
	urlFlag := fs.String("url", "", "external PR url (e.g. GitHub)")
	providerFlag := fs.String("provider", "", "provider: none|github (default: inferred from repo)")
	desc := fs.String("m", "", "PR description")
	positional, err := parseFlagsWithPositionals(fs, args)
	if err != nil {
		return err
	}
	ticketArg, rest, err := resolveIDFlag(*idFlag, positional)
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk pr create [-id] <ticket-id> [-repo R] [-from B] [-to B] [-title T] [-url U] [-provider none|github] [-m desc]")
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
	ticket, err := svc.GetTicket(ctx, normalizeBareTicketRef(cfg, svc, ticketArg))
	if err != nil {
		return err
	}

	repository := strings.TrimSpace(*repo)
	if repository == "" {
		if repos, repoErr := svc.ListProjectGitRepositories(ctx, strconv.FormatInt(ticket.ProjectID, 10)); repoErr == nil && len(repos) > 0 {
			repository = repos[0]
		} else if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			repository = detectGitOriginAt(cwd)
		}
	}
	source := strings.TrimSpace(*from)
	if source == "" {
		source = currentGitBranch()
	}
	target := strings.TrimSpace(*to)
	if target == "" {
		target = "main"
	}
	provider := strings.TrimSpace(*providerFlag)
	if provider == "" {
		provider = detectPRProvider(repository)
	}

	pr, err := svc.CreatePullRequest(ctx, libticket.PullRequestRequest{
		TicketID:     ticket.ID,
		Title:        strings.TrimSpace(*title),
		Description:  strings.TrimSpace(*desc),
		Repository:   repository,
		SourceBranch: source,
		TargetBranch: target,
		Provider:     provider,
		URL:          strings.TrimSpace(*urlFlag),
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(pr)
	}
	printPullRequest(pr)
	return nil
}

func runPullRequestList(args []string) error {
	fs := flag.NewFlagSet("pr ls", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	idFlag := fs.String("id", "", "ticket id")
	positional, err := parseFlagsWithPositionals(fs, args)
	if err != nil {
		return err
	}
	ticketArg, rest, err := resolveIDFlag(*idFlag, positional)
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk pr ls [-id] <ticket-id>")
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
	ticket, err := svc.GetTicket(ctx, normalizeBareTicketRef(cfg, svc, ticketArg))
	if err != nil {
		return err
	}
	prs, err := svc.ListPullRequestsByTicket(ctx, ticket.ID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(prs)
	}
	if len(prs) == 0 {
		fmt.Printf("no pull requests for %s\n", ticket.ID)
		return nil
	}
	for _, pr := range prs {
		line := fmt.Sprintf("#%d  %s  %s  %s → %s", pr.ID, pr.Status, pr.Provider, pr.SourceBranch, pr.TargetBranch)
		if strings.TrimSpace(pr.URL) != "" {
			line += "  " + pr.URL
		}
		fmt.Println(line)
	}
	return nil
}

func runPullRequestGet(args []string) error {
	fs := flag.NewFlagSet("pr get", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	idFlag := fs.Int64("id", 0, "pull request id")
	positional, err := parseFlagsWithPositionals(fs, args)
	if err != nil {
		return err
	}
	prID := *idFlag
	if prID == 0 {
		if len(positional) != 1 {
			return errors.New("usage: tk pr get <pr-id>")
		}
		parsed, parseErr := strconv.ParseInt(strings.TrimSpace(positional[0]), 10, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid pull request id %q", positional[0])
		}
		prID = parsed
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	pr, err := svc.GetPullRequest(context.Background(), prID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(pr)
	}
	printPullRequest(pr)
	return nil
}

// printTicketPullRequests renders a compact PR section for tk get. It is a
// no-op when the ticket has no pull requests.
func printTicketPullRequests(prs []store.PullRequest) {
	if len(prs) == 0 {
		return
	}
	fmt.Println("pull requests :")
	for _, pr := range prs {
		line := fmt.Sprintf("  - #%d %s (%s) %s → %s", pr.ID, pr.Status, pr.Provider, pr.SourceBranch, pr.TargetBranch)
		if strings.TrimSpace(pr.URL) != "" {
			line += " " + pr.URL
		}
		fmt.Println(line)
	}
}

func printPullRequest(pr store.PullRequest) {
	fmt.Printf("pr         : #%d\n", pr.ID)
	fmt.Printf("ticket     : %s\n", pr.TicketID)
	fmt.Printf("title      : %s\n", pr.Title)
	fmt.Printf("status     : %s\n", pr.Status)
	fmt.Printf("provider   : %s\n", pr.Provider)
	if strings.TrimSpace(pr.Repository) != "" {
		fmt.Printf("repository : %s\n", pr.Repository)
	}
	fmt.Printf("branches   : %s → %s\n", pr.SourceBranch, pr.TargetBranch)
	if strings.TrimSpace(pr.URL) != "" {
		fmt.Printf("url        : %s\n", pr.URL)
	}
}
