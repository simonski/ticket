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
  ls     [[-id] <ticket-id>] [-open|-closed|-all]
                                        List pull requests. With no ticket, lists the
                                        current project's PRs (open by default; -closed
                                        or -all to widen). With a ticket, lists its PRs.
  get    <pr-id>                       Show a pull request
  merge  <pr-id>                       Mark a pull request merged
  close  <pr-id>                       Mark a pull request closed
  reopen <pr-id>                       Reopen a closed pull request

Add -gh to create to open a real GitHub PR via gh (github repos only).
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
	case "merge":
		return runPullRequestStatus(args[1:], store.PullRequestStatusMerged)
	case "close":
		return runPullRequestStatus(args[1:], store.PullRequestStatusClosed)
	case "reopen":
		return runPullRequestStatus(args[1:], store.PullRequestStatusOpen)
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
	ghFlag := fs.Bool("gh", false, "open a real GitHub PR via gh and store its url (github repos only)")
	positional, err := parseFlagsWithPositionals(fs, args)
	if err != nil {
		return err
	}
	ticketArg, rest, err := resolveIDFlag(*idFlag, positional)
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk pr create [-id] <ticket-id> [-repo R] [-from B] [-to B] [-title T] [-url U] [-provider none|github] [-m desc] [-gh]")
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

	prURL := strings.TrimSpace(*urlFlag)
	if *ghFlag {
		if provider != store.PullRequestProviderGitHub {
			return fmt.Errorf("-gh requires a GitHub repository (provider is %q)", provider)
		}
		prTitle := strings.TrimSpace(*title)
		if prTitle == "" {
			prTitle = ticket.Title
		}
		openedURL, ghErr := ghCreatePullRequest(prTitle, strings.TrimSpace(*desc), source, target)
		if ghErr != nil {
			return ghErr
		}
		prURL = openedURL
	}

	pr, err := svc.CreatePullRequest(ctx, libticket.PullRequestRequest{
		TicketID:     ticket.ID,
		Title:        strings.TrimSpace(*title),
		Description:  strings.TrimSpace(*desc),
		Repository:   repository,
		SourceBranch: source,
		TargetBranch: target,
		Provider:     provider,
		URL:          prURL,
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
	idFlag := fs.String("id", "", "ticket id (list this ticket's PRs)")
	openFlag := fs.Bool("open", false, "only open/draft pull requests")
	closedFlag := fs.Bool("closed", false, "only merged/closed pull requests")
	allFlag := fs.Bool("all", false, "all pull requests regardless of status")
	positional, err := parseFlagsWithPositionals(fs, args)
	if err != nil {
		return err
	}
	ticketArg := strings.TrimSpace(*idFlag)
	if ticketArg == "" && len(positional) == 1 {
		ticketArg = strings.TrimSpace(positional[0])
	} else if (ticketArg != "" && len(positional) > 0) || len(positional) > 1 {
		return errors.New("usage: tk pr ls [[-id] <ticket-id>] [-open|-closed|-all]")
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

	var (
		prs   []store.PullRequest
		scope string
		// Project-wide listing defaults to open; a single ticket defaults to all.
		defaultClosed = false
		showAll       = *allFlag
	)
	if ticketArg != "" {
		ticket, ticketErr := svc.GetTicket(ctx, normalizeBareTicketRef(cfg, svc, ticketArg))
		if ticketErr != nil {
			return ticketErr
		}
		prs, err = svc.ListPullRequestsByTicket(ctx, ticket.ID)
		scope = ticket.ID
		if !*openFlag && !*closedFlag {
			showAll = true // a specific ticket shows all of its PRs by default
		}
	} else {
		_, projectSvc, project, projErr := resolveCurrentProjectClient()
		if projErr != nil {
			return projErr
		}
		prs, err = projectSvc.ListPullRequestsByProject(ctx, strconv.FormatInt(project.ID, 10))
		scope = project.Prefix
	}
	if err != nil {
		return err
	}

	if *closedFlag {
		defaultClosed = true
	}
	filtered := filterPullRequestsByStatus(prs, defaultClosed, showAll)

	if outputJSON {
		return printJSON(filtered)
	}
	if len(filtered) == 0 {
		fmt.Printf("no pull requests for %s\n", scope)
		return nil
	}
	for _, pr := range filtered {
		line := fmt.Sprintf("#%d  %s  %s  %s  %s → %s", pr.ID, pr.TicketID, pr.Status, pr.Provider, pr.SourceBranch, pr.TargetBranch)
		if strings.TrimSpace(pr.URL) != "" {
			line += "  " + pr.URL
		}
		fmt.Println(line)
	}
	return nil
}

// pullRequestIsClosed reports whether a PR status is in the closed set
// (merged or closed) versus the open set (draft or open).
func pullRequestIsClosed(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case store.PullRequestStatusMerged, store.PullRequestStatusClosed:
		return true
	default:
		return false
	}
}

// filterPullRequestsByStatus returns all PRs when all is true, otherwise the
// closed set when wantClosed is true, otherwise the open set.
func filterPullRequestsByStatus(prs []store.PullRequest, wantClosed, all bool) []store.PullRequest {
	if all {
		return prs
	}
	out := make([]store.PullRequest, 0, len(prs))
	for _, pr := range prs {
		if pullRequestIsClosed(pr.Status) == wantClosed {
			out = append(out, pr)
		}
	}
	return out
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

func runPullRequestStatus(args []string, status string) error {
	fs := flag.NewFlagSet("pr "+status, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	idFlag := fs.Int64("id", 0, "pull request id")
	positional, err := parseFlagsWithPositionals(fs, args)
	if err != nil {
		return err
	}
	prID := *idFlag
	if prID == 0 {
		if len(positional) != 1 {
			return fmt.Errorf("usage: tk pr %s <pr-id>", statusVerb(status))
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
	pr, err := svc.SetPullRequestStatus(context.Background(), prID, status)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(pr)
	}
	printPullRequest(pr)
	return nil
}

// statusVerb maps a target status to the CLI verb used to reach it.
func statusVerb(status string) string {
	switch status {
	case store.PullRequestStatusMerged:
		return "merge"
	case store.PullRequestStatusClosed:
		return "close"
	default:
		return "reopen"
	}
}

// ghCreatePullRequest opens a real GitHub PR via the gh CLI in the current
// directory and returns its URL. It requires gh to be installed.
func ghCreatePullRequest(title, body, head, base string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("gh CLI not found on PATH; install it or omit -gh")
	}
	out, err := exec.Command("gh", "pr", "create", "--head", head, "--base", base, "--title", title, "--body", body).CombinedOutput() // #nosec G204 -- args are ticket/branch metadata, not shell-interpreted
	if err != nil {
		return "", fmt.Errorf("gh pr create failed: %s", strings.TrimSpace(string(out)))
	}
	url := extractFirstURL(string(out))
	if url == "" {
		return "", fmt.Errorf("gh pr create succeeded but no PR url was found in its output")
	}
	return url, nil
}

// extractFirstURL returns the first https:// token found in s, or "".
func extractFirstURL(s string) string {
	for _, field := range strings.Fields(s) {
		if strings.HasPrefix(field, "https://") || strings.HasPrefix(field, "http://") {
			return strings.TrimRight(field, ".,)")
		}
	}
	return ""
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
