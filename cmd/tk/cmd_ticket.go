package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/internal/tui"
	"github.com/simonski/ticket/libticket"
)

// ticketSortKey returns a numeric sort key so that complete/late-stage tickets
// sink to the bottom of the list while active work stays at the top (TK-189).
// Lower values appear first.
func ticketSortKey(t store.Ticket) int {
	stageOrd := map[string]int{
		store.StageDesign:  0,
		store.StageDevelop: 1,
		store.StageTest:    2,
		store.StageDone:    3,
	}
	stateOrd := map[string]int{
		store.StateActive:  0,
		store.StateIdle:    1,
		store.StateFail:    2,
		store.StateSuccess: 3,
	}
	s := stageOrd[t.Stage] * 10
	s += stateOrd[t.State]
	return s
}

func ticketIsOpenForList(t store.Ticket) bool {
	if t.Archived || t.Complete {
		return false
	}
	return strings.TrimSpace(strings.ToLower(t.Stage)) != store.StageDone
}

func childTicketCounts(children []store.Ticket) (total, open, closed int) {
	total = len(children)
	for _, child := range children {
		if ticketIsOpenForList(child) {
			open++
		}
	}
	closed = total - open
	return total, open, closed
}

func runTicketNS(args []string) error {
	if len(args) == 0 {
		return runList(nil)
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Println(ticketNSUsage)
		return nil

	// List & search
	case "list", "ls":
		return runList(args[1:])
	case "search":
		return runSearch(args[1:])
	case "board":
		return runBoard(args[1:])
	case "count":
		return runCount(args[1:])
	case "orphans":
		return runOrphans(args[1:])

	// Create
	case "add", "create", "new":
		return runTicketCreate(args[1:])

	// View
	case "get", "show":
		return runGet(args[1:])
	case "tree":
		return errors.New("tk ticket tree was a placeholder alias and has been removed; use `tk get` for details or the web hierarchy view for nested browsing")

	// Edit (TUI)
	case "edit":
		return runEdit(args[1:])

	// Update
	case "update":
		return runUpdate(args[1:])

	// State
	case "active":
		return runTicketStateAlias(args[1:], store.StateActive, "active")
	case "idle":
		return runTicketStateAlias(args[1:], store.StateIdle, "idle")
	case "complete":
		return runTicketStateAlias(args[1:], store.StateSuccess, "complete")
	case "fail":
		return runTicketStateAlias(args[1:], store.StateFail, "fail")
	case "state":
		return runTicketState(args[1:])

	// Ownership
	case "claim":
		return runClaim(args[1:])
	case "unclaim":
		return runUnclaim(args[1:])
	case "assign":
		return runAssign(args[1:])
	case "unassign":
		return runUnassign(args[1:])
	case "request":
		return runRequest(args[1:])

	// Hierarchy
	case "attach":
		return runSetParent(args[1:], "attach")
	case "detach":
		return runUnsetParent(args[1:], "detach")

	// Comments & history
	case "comment":
		return runComment(args[1:])
	case "history":
		return runHistory(args[1:])
	case "conversation":
		return runConversation(args[1:])

	// Lifecycle
	case "close":
		return runSetTicketClosed(args[1:], true)
	case "open":
		return runSetTicketClosed(args[1:], false)
	case "archive":
		return runSetTicketArchived(args[1:], true)
	case "unarchive":
		return runSetTicketArchived(args[1:], false)
	case "ready":
		return runSetTicketDraft(args[1:], true)
	case "notready":
		return runSetTicketDraft(args[1:], false)
	case "reject":
		return runRejectTicket(args[1:])
	case "clone", "cp":
		return runClone(args[1:])
	case "delete", "rm":
		return runDeleteTicket(args[1:])

	// Legacy: agent-based ticket generation
	case "gen":
		return runTicketGen(args[1:])

	default:
		return fmt.Errorf("unknown ticket command %q; see: ticket ticket help", args[0])
	}
}

const ticketNSUsage = `Usage: ticket ticket <command> [flags]

Commands:
  list    [--type T] [--status S] [-u user]   List tickets
  search  "query"                             Full-text search
  board                                       Kanban view
  count                                       Aggregate counts
  orphans                                     Tickets with no parent

  add     "title" [-type T] [-d desc] [-ac criteria]   Create a ticket
  get     -id <id> [-json]                    View ticket detail
  edit    [-id] <id>                          Open TUI editor for ticket
  update  -id <id> [field flags]              Update ticket fields

  active   -id <id>                           Start work
  idle     -id <id>                           Pause work
  complete -id <id>                           Finish stage, advance
  fail     -id <id>                           Mark failed

  claim    -id <id>                           Assign to self
  unclaim  -id <id>                           Unassign self
  assign   -id <id> <user>                    Assign to someone
  unassign -id <id> <user>                    Unassign someone
  request                                     Next available ticket

  attach   -id <id> <parent-id>               Set parent
  detach   -id <id>                           Remove parent

  comment  add -id <id> "text"                Add comment
  history  [<id>] [-user_id ID] [-agent_id ID] [-team_id ID]  Activity log
  conversation show <id>                      Full thread

  close    -id <id>                           Close ticket
  open     -id <id>                           Reopen ticket
  archive  -id <id>                           Archive
  unarchive -id <id>                          Unarchive
  ready    -id <id>                           Mark ready for work
  notready -id <id>                          Mark not ready
  reject   -id <id>                           Send ticket back to the first workflow stage as draft
  clone    -id <id>                           Duplicate
  delete   -id <id>                           Soft-delete

  gen      -f <files> -o <output>             Generate tickets via agent`

func runTicketGen(args []string) error {
	fs := flag.NewFlagSet("ticket", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	filesArg := fs.String("f", "", "comma-separated input files")
	outputFile := fs.String("o", "", "output file")
	agent := fs.String("agent", "codex", "agent command")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*filesArg) == "" || strings.TrimSpace(*outputFile) == "" {
		return errors.New("usage: tk ticket -f <file1,file2,...> -o <output-file> [-agent <agent>]")
	}

	files := splitCSV(*filesArg)
	if len(files) == 0 {
		return errors.New("at least one input file is required")
	}
	prompt, err := buildTicketPrompt(files, *outputFile)
	if err != nil {
		return err
	}
	response, err := runAgentCommand(strings.TrimSpace(*agent), prompt, false, "")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*outputFile, []byte(response), 0o644); err != nil { // #nosec G306 -- output file is user-specified, 0644 is intentional
		return err
	}
	fmt.Print(response)
	if response != "" && !strings.HasSuffix(response, "\n") {
		fmt.Println()
	}
	if outputJSON {
		return printJSON(map[string]string{
			"status": "ok",
			"agent":  strings.TrimSpace(*agent),
			"output": *outputFile,
		})
	}
	fmt.Printf("wrote %s using %s\n", *outputFile, strings.TrimSpace(*agent))
	return nil
}

func splitCSV(raw string) []string {
	var values []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func buildTicketPrompt(files []string, outputFile string) (string, error) {
	var b strings.Builder
	b.WriteString("Write an example breakdown of implementation requirements as ")
	b.WriteString(outputFile)
	b.WriteString(" in the format:\n\n")
	b.WriteString("EPIC: title\n")
	b.WriteString("ID: E1, E2, E3 etc\n")
	b.WriteString("DESCRIPTION: description\n")
	b.WriteString("AC: list of acceptance criteria\n")
	b.WriteString("PRIORITY: 1-N (1 highest, do this first)\n")
	b.WriteString("DEPENDS-ON: E2, E4\n\n")
	b.WriteString("<indent for stories \"in\" the epic (the story ID should increment and be EPIC-STORY)>\n")
	b.WriteString("    STORY: title\n")
	b.WriteString("    ID: E1-S1, E1-2, E1-S3 etc.\n")
	b.WriteString("    DESCRIPTION: description\n")
	b.WriteString("    AC: list of acceptance criteria\n")
	b.WriteString("    PRIORITY: 1-N (1 highest, do this first)\n")
	b.WriteString("    DEPENDS-ON: E1-S2\n\n")
	b.WriteString("Use the following input files as source material:\n\n")
	for _, file := range files {
		data, err := os.ReadFile(file) // #nosec G304 -- file is a CLI argument provided by the operator
		if err != nil {
			return "", err
		}
		b.WriteString("FILE: ")
		b.WriteString(file)
		b.WriteString("\n")
		b.WriteString("-----\n")
		b.Write(data)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			b.WriteString("\n")
		}
		b.WriteString("-----\n\n")
	}
	return b.String(), nil
}

// resolveIDFlag extracts a ticket ID from either an -id flag value or a
// positional argument. It returns the resolved ID and remaining positional
// args, or an error if neither form provides an ID.
func resolveIDFlag(flagVal string, positional []string) (string, []string, error) {
	idVal := strings.TrimSpace(flagVal)
	if idVal != "" {
		return idVal, positional, nil
	}
	if len(positional) > 0 {
		return positional[0], positional[1:], nil
	}
	return "", nil, errors.New("missing ticket id")
}

func normalizeBareTicketRef(cfg config.Config, svc libticket.Service, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.Contains(ref, "-") || !isBareTicketSequence(ref) {
		return ref
	}
	prefix := strings.TrimSpace(cfg.ProjectID)
	if prefix == "" {
		return ref
	}
	if project, err := svc.GetProject(context.Background(), prefix); err == nil && strings.TrimSpace(project.Prefix) != "" {
		prefix = strings.TrimSpace(project.Prefix)
	}
	if prefix == "" {
		return ref
	}
	return prefix + "-" + ref
}

func isBareTicketSequence(ref string) bool {
	if strings.TrimSpace(ref) == "" {
		return false
	}
	for _, r := range ref {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func resolveLifecycleInput(status, stage, state string) (string, string, error) {
	if strings.TrimSpace(stage) != "" || strings.TrimSpace(state) != "" {
		return stage, state, nil
	}
	if strings.TrimSpace(status) == "" {
		return "", "", nil
	}
	return store.ParseLifecycleStatus(status)
}

func ticketWorkflowStageNames(svc libticket.Service, ticket store.Ticket) ([]string, error) {
	if ticket.WorkflowStageID != nil {
		stage, err := svc.GetWorkflowStage(context.Background(), *ticket.WorkflowStageID)
		if err == nil {
			stages, err := svc.ListWorkflowStages(context.Background(), stage.WorkflowID)
			if err != nil {
				return nil, err
			}
			if names := normalizeWorkflowStageNames(stages); len(names) > 0 {
				return names, nil
			}
		}
	}
	if ticket.WorkflowID != nil {
		stages, err := svc.ListWorkflowStages(context.Background(), *ticket.WorkflowID)
		if err != nil {
			return nil, err
		}
		if names := normalizeWorkflowStageNames(stages); len(names) > 0 {
			return names, nil
		}
	}
	if ticket.ParentID != nil {
		parent, err := svc.GetTicket(context.Background(), *ticket.ParentID)
		if err != nil {
			return nil, err
		}
		return ticketWorkflowStageNames(svc, parent)
	}
	project, err := svc.GetProject(context.Background(), strconv.FormatInt(ticket.ProjectID, 10))
	if err != nil {
		return nil, err
	}
	if project.WorkflowID != nil {
		stages, err := svc.ListWorkflowStages(context.Background(), *project.WorkflowID)
		if err != nil {
			return nil, err
		}
		if names := normalizeWorkflowStageNames(stages); len(names) > 0 {
			return names, nil
		}
	}
	return []string{store.StageDesign, store.StageDevelop, store.StageTest, store.StageDone}, nil
}

func normalizeWorkflowStageNames(stages []store.WorkflowStage) []string {
	names := make([]string, 0, len(stages))
	seen := make(map[string]bool, len(stages))
	for _, stage := range stages {
		name := strings.ToLower(strings.TrimSpace(stage.StageName))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func validateTicketStageInput(svc libticket.Service, ticket store.Ticket, stage string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(stage))
	if normalized == "" {
		return "", nil
	}
	validStages, err := ticketWorkflowStageNames(svc, ticket)
	if err != nil {
		return "", err
	}
	for _, validStage := range validStages {
		if normalized == validStage {
			return normalized, nil
		}
	}
	return "", fmt.Errorf("invalid stage %q; valid stages: %s", stage, strings.Join(validStages, ", "))
}

// expandListShortFlags expands combined POSIX-style boolean flags for the
// list command. For example: -ad → -a -d, -la → -l -a.
// Only boolean short flags (a, d, l) are expanded. Flags that take values
// (t, u, n) are left as-is to avoid consuming the next argument.
func expandListShortFlags(args []string) []string {
	boolFlags := map[byte]bool{
		'a': true, // include all (closed)
		'd': true, // include archived
	}
	var expanded []string
	for _, arg := range args {
		if len(arg) > 2 && arg[0] == '-' && arg[1] != '-' {
			allBool := true
			for i := 1; i < len(arg); i++ {
				if !boolFlags[arg[i]] {
					allBool = false
					break
				}
			}
			if allBool {
				for i := 1; i < len(arg); i++ {
					expanded = append(expanded, "-"+string(arg[i]))
				}
				continue
			}
		}
		expanded = append(expanded, arg)
	}
	return expanded
}

func runList(args []string) error {
	// Expand combined boolean short flags: -ad → -a -d, -la → -l -a, etc.
	args = expandListShortFlags(args)

	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	taskType := fs.String("type", "", "filter by ticket type")
	fs.StringVar(taskType, "t", "", "filter by ticket type (shorthand)")
	stage := fs.String("stage", "", "filter by ticket stage")
	state := fs.String("state", "", "filter by ticket state")
	status := fs.String("status", "", "filter by rendered ticket status")
	assignee := fs.String("user", "", "filter by assignee")
	fs.StringVar(assignee, "u", "", "filter by assignee")
	limit := fs.Int("n", 0, "maximum number of tickets to return; 0 means all")
	useUnicode := fs.Bool("unicode", true, "render status symbols as unicode")
	plain := fs.Bool("plain", false, "render status as plain text")
	includeAll := fs.Bool("a", false, "include all tickets (closed and archived)")
	includeDeleted := fs.Bool("d", false, "include archived tickets")
	labelFilter := fs.String("label", "", "filter by label name")
	countOnly := fs.Bool("count", false, "print only the number of matching tickets")
	expectEquals := fs.String("expect_equals", "", "expect the resulting count to equal this number")
	expectNotEquals := fs.String("expect_notequals", "", "expect the resulting count to not equal this number")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Allow positional type: tk ls epic  ==  tk ls -type epic
	if *taskType == "" && fs.NArg() == 1 {
		v := fs.Arg(0)
		taskType = &v
	}
	if *limit < 0 {
		return errors.New("usage: tk list|ls [<type>] [-type <type>] [-t <type>] [-stage <stage>] [-state <state>] [-status <stage/state>] [-u <user>] [-n <limit>] [-a] [-d] [-label <name>] [-count] [-expect_equals <n>] [-expect_notequals <n>]")
	}
	hasExpectEquals := strings.TrimSpace(*expectEquals) != ""
	hasExpectNotEquals := strings.TrimSpace(*expectNotEquals) != ""
	if hasExpectEquals && hasExpectNotEquals {
		return errors.New("list expects only one of -expect_equals or -expect_notequals")
	}
	// -d implies -a (archived tickets are a superset of closed)
	if *includeDeleted {
		*includeAll = true
	}
	statusUnicode := *useUnicode && !*plain
	resolvedStage, resolvedState, err := resolveLifecycleInput(*status, *stage, *state)
	if err != nil {
		return err
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	tickets, err := api.ListTicketsFiltered(context.Background(), project.ID, *taskType, resolvedStage, resolvedState, "", "", *assignee, *limit, *includeAll)
	if err != nil {
		return err
	}
	if !*includeAll {
		open := tickets[:0]
		for _, t := range tickets {
			if ticketIsOpenForList(t) {
				open = append(open, t)
			}
		}
		tickets = open
	} else if !*includeDeleted {
		// -a without -d: show closed but hide archived
		nonArchived := tickets[:0]
		for _, t := range tickets {
			if !t.Archived {
				nonArchived = append(nonArchived, t)
			}
		}
		tickets = nonArchived
	}
	if *labelFilter != "" {
		filtered := tickets[:0]
		for _, ticket := range tickets {
			labels, err := api.ListTicketLabels(context.Background(), ticket.ID)
			if err != nil {
				return err
			}
			for _, l := range labels {
				if strings.EqualFold(l.Name, *labelFilter) {
					filtered = append(filtered, ticket)
					break
				}
			}
		}
		tickets = filtered
	}
	if *countOnly || hasExpectEquals || hasExpectNotEquals {
		count := len(tickets)
		if hasExpectEquals {
			expected, err := parseExpectedCount("expect_equals", *expectEquals)
			if err != nil {
				return err
			}
			if count != expected {
				return fmt.Errorf("expected count to equal %d, got %d", expected, count)
			}
		}
		if hasExpectNotEquals {
			expected, err := parseExpectedCount("expect_notequals", *expectNotEquals)
			if err != nil {
				return err
			}
			if count == expected {
				return fmt.Errorf("expected count to not equal %d, got %d", expected, count)
			}
		}
		if outputJSON {
			return printJSON(map[string]any{"count": count})
		}
		fmt.Println(count)
		return nil
	}
	if len(tickets) == 0 {
		if outputJSON {
			return printJSON(tickets)
		}
		name := "tickets"
		if strings.TrimSpace(*taskType) != "" {
			name = entityPlural(*taskType)
		}
		printNoEntitiesAvailable(name)
		return nil
	}
	// Build parent key map: ticket ID → parent's key string.
	// Cache lookups so shared parents are only fetched once.
	parentKeys := make(map[string]string, len(tickets))
	parentCache := make(map[string]string)
	for _, ticket := range tickets {
		if ticket.ParentID == nil {
			continue
		}
		pid := *ticket.ParentID
		if key, ok := parentCache[pid]; ok {
			parentKeys[ticket.ID] = key
		} else if p, err := api.GetTicket(context.Background(), pid); err == nil {
			key := ticketLabel(p)
			parentCache[pid] = key
			parentKeys[ticket.ID] = key
		}
	}
	if outputJSON {
		return printJSON(tickets)
	}
	// Sort: tickets in later stages or with success/fail state sink to the bottom
	// so in-progress work is visible first (TK-189).
	sort.SliceStable(tickets, func(i, j int) bool {
		return ticketSortKey(tickets[i]) < ticketSortKey(tickets[j])
	})
	// Build agent username set so we can prefix agent assignees.
	agentUsernames := make(map[string]bool)
	if agents, err := api.ListAgents(context.Background()); err == nil {
		for _, a := range agents {
			agentUsernames[a.Username] = true
		}
	}
	printTicketTable(tickets, parentKeys, agentUsernames, statusUnicode, *includeAll)
	return nil
}

func runBoard(args []string) error {
	fs := flag.NewFlagSet("board", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	includeArchived := fs.Bool("a", false, "include archived tickets")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	tickets, err := api.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, *includeArchived)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(tickets)
	}
	var workflowStages []store.WorkflowStage
	if project.WorkflowID != nil {
		if wf, err := api.GetWorkflow(context.Background(), *project.WorkflowID); err == nil {
			workflowStages = wf.Stages
		}
	}
	if len(workflowStages) == 0 {
		fmt.Println("no workflow stages defined for this project")
		return nil
	}

	// Group tickets by stage
	byStage := make(map[string][]store.Ticket)
	for _, t := range tickets {
		byStage[t.Stage] = append(byStage[t.Stage], t)
	}

	stateIcon := func(state string) string {
		switch state {
		case "idle":
			return "○"
		case "active":
			return "◑"
		case "success":
			return "◉"
		case "fail":
			return "✗"
		}
		return " "
	}

	// Print each stage as a lane
	for _, ws := range workflowStages {
		stageTickets := byStage[ws.StageName]
		fmt.Printf("── %s (%d) ──\n", strings.ToUpper(ws.StageName), len(stageTickets))
		if len(stageTickets) == 0 {
			fmt.Println("  (empty)")
		} else {
			// Sort within the lane: active work first, complete/late-stage last.
			sort.SliceStable(stageTickets, func(i, j int) bool {
				return ticketSortKey(stageTickets[i]) < ticketSortKey(stageTickets[j])
			})
			// Build tree display (epic → stories → tasks indented).
			ordered, treePfx := buildTreeDisplay(stageTickets)
			for _, t := range ordered {
				assignee := strings.TrimSpace(t.Assignee)
				if assignee == "" {
					assignee = "-"
				}
				fmt.Printf("  %s%s %s  %s  [%s]  @%s\n",
					treePfx[t.ID], stateIcon(t.State), t.ID, t.Title, t.Type, assignee)
			}
		}
		fmt.Println()
	}
	return nil
}

func runOrphans(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: tk orphans")
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	tickets, err := api.ListTickets(context.Background(), project.ID)
	if err != nil {
		return err
	}
	var orphans []store.Ticket
	for _, ticket := range tickets {
		if ticket.ParentID == nil && strings.TrimSpace(ticket.Type) != "epic" {
			orphans = append(orphans, ticket)
		}
	}
	if outputJSON {
		return printJSON(orphans)
	}
	for _, ticket := range orphans {
		fmt.Printf("%s\t%s\t%s\t%s\n", ticketLabel(ticket), ticket.Type, ticket.Status, ticket.Title)
	}
	return nil
}

func runGet(args []string) error {
	usage := "tk get [-id] <id>"
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Allow positional: tk get FOO is the same as tk get -id FOO
	if strings.TrimSpace(*id) == "" && fs.NArg() == 1 {
		v := fs.Arg(0)
		id = &v
	} else if fs.NArg() != 0 {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticketRef := strings.TrimSpace(*id)
	if ticketRef == "" {
		project, err := requireCurrentProject(cfg, svc)
		if err != nil {
			return err
		}
		latest, err := mostRecentTicket(svc, project.ID, "")
		if err != nil {
			return err
		}
		ticketRef = latest.ID
	} else {
		ticketRef = normalizeBareTicketRef(cfg, svc, ticketRef)
	}
	ticket, err := svc.GetTicket(context.Background(), ticketRef)
	if err != nil {
		return err
	}
	dependencies, _ := svc.ListDependencies(context.Background(), ticket.ID)
	history, _ := svc.ListHistory(context.Background(), ticket.ID)
	if outputJSON {
		return printJSON(ticket)
	}
	// Look up workflow stages for progress display
	var workflowStages []store.WorkflowStage
	project, projectErr := svc.GetProject(context.Background(), fmt.Sprintf("%d", ticket.ProjectID))
	if projectErr == nil && project.WorkflowID != nil {
		if wf, err := svc.GetWorkflow(context.Background(), *project.WorkflowID); err == nil {
			workflowStages = wf.Stages
		}
	}
	ticketLabels, _ := svc.ListTicketLabels(context.Background(), ticket.ID)
	totalTime, _ := svc.TotalTimeForTicket(context.Background(), ticket.ID)
	parentKey := ""
	if ticket.ParentID != nil {
		if p, err := svc.GetTicket(context.Background(), *ticket.ParentID); err == nil {
			parentKey = ticketLabel(p)
		}
	}
	cloneKey := ""
	if ticket.CloneOf != nil {
		if c, err := svc.GetTicket(context.Background(), *ticket.CloneOf); err == nil {
			cloneKey = ticketLabel(c)
		}
	}
	var children []store.Ticket
	childTotal, childOpen, childClosed := 0, 0, 0
	if projectErr == nil {
		all, _ := svc.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, true)
		for _, t := range all {
			if t.ParentID != nil && *t.ParentID == ticket.ID {
				children = append(children, t)
			}
		}
		childTotal, childOpen, childClosed = childTicketCounts(children)
	}
	printTicketDetails(ticket, dependencies, history, workflowStages, ticketLabels, totalTime, parentKey, cloneKey, childTotal, childOpen, childClosed)
	if len(children) > 0 {
		printTicketChildren(children)
	}
	return nil
}

func runEdit(args []string) error {
	usage := "tk edit [-id] <id>"
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id or key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" && fs.NArg() == 1 {
		v := fs.Arg(0)
		id = &v
	} else if strings.TrimSpace(*id) == "" && fs.NArg() == 0 {
		// No ID: open the most recently modified ticket in the current project.
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		var project store.Project
		if cfg.ProjectID != "" {
			project, err = svc.GetProject(context.Background(), cfg.ProjectID)
			if err != nil {
				return err
			}
		} else {
			return errors.New(usage)
		}
		tickets, err := svc.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, false)
		if err != nil {
			return err
		}
		if len(tickets) == 0 {
			return errors.New("no tickets in project")
		}
		// Find most recently updated ticket.
		latest := tickets[0]
		for _, t := range tickets[1:] {
			if t.UpdatedAt > latest.UpdatedAt {
				latest = t
			}
		}
		return tui.RunEdit(svc, cfg, project, latest)
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New(usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(context.Background(), strings.TrimSpace(*id))
	if err != nil {
		return err
	}
	var project store.Project
	if cfg.ProjectID != "" {
		project, err = svc.GetProject(context.Background(), cfg.ProjectID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load project %s: %v\n", cfg.ProjectID, err)
		}
	}
	return tui.RunEdit(svc, cfg, project, ticket)
}

func runSearch(args []string) error {
	query, filters, err := parseSearchArgs(args)
	if err != nil {
		return err
	}
	if query == "" {
		return errors.New("usage: tk search <free form query> [-status <status>] [-title <text>] [-description <text>] [-priority <n>] [-owner <user>]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	var projects []store.Project
	if filters.allProjects {
		projects, err = svc.ListProjects(context.Background())
		if err != nil {
			return err
		}
	} else {
		_, _, project, err := resolveCurrentProjectClient()
		if err != nil {
			return err
		}
		projects = []store.Project{project}
	}
	var tickets []store.Ticket
	for _, project := range projects {
		projectTasks, err := svc.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, false)
		if err != nil {
			return err
		}
		for _, ticket := range projectTasks {
			if !ticketMatchesSearch(ticket, query, filters.stage, filters.state, filters.status, filters.title, filters.description, filters.priority, filters.owner) {
				continue
			}
			tickets = append(tickets, ticket)
		}
	}
	if outputJSON {
		return printJSON(tickets)
	}
	for _, ticket := range tickets {
		fmt.Printf("%s\t%s\t%s\t%s\n", ticketLabel(ticket), ticket.Type, ticket.Status, ticket.Title)
	}
	return nil
}

type searchFilters struct {
	stage       string
	state       string
	status      string
	title       string
	description string
	priority    int
	owner       string
	allProjects bool
}

func parseSearchArgs(args []string) (string, searchFilters, error) {
	var filters searchFilters
	var terms []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-stage":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -stage requires a value")
			}
			filters.stage = args[i+1]
			i++
		case "-state":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -state requires a value")
			}
			filters.state = args[i+1]
			i++
		case "-status":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -status requires a value")
			}
			filters.status = args[i+1]
			i++
		case "-title":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -title requires a value")
			}
			filters.title = args[i+1]
			i++
		case "-description":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -description requires a value")
			}
			filters.description = args[i+1]
			i++
		case "-priority":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -priority requires a value")
			}
			priority, err := strconv.Atoi(args[i+1])
			if err != nil {
				return "", filters, errors.New("search priority must be numeric")
			}
			filters.priority = priority
			i++
		case "-owner":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -owner requires a value")
			}
			filters.owner = args[i+1]
			i++
		case "-allprojects":
			filters.allProjects = true
		default:
			terms = append(terms, args[i])
		}
	}
	return strings.TrimSpace(strings.Join(terms, " ")), filters, nil
}

func ticketMatchesSearch(ticket store.Ticket, query, stage, state, status, title, description string, priority int, owner string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query != "" {
		haystack := strings.ToLower(strings.Join([]string{
			ticket.Title,
			ticket.Description,
			ticket.AcceptanceCriteria,
			ticket.Assignee,
			ticket.Status,
			strconv.Itoa(ticket.Priority),
		}, "\n"))
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		stageFilter, stateFilter, err := resolveLifecycleInput(trimmed, "", "")
		if err == nil {
			if ticket.Stage != strings.TrimSpace(stageFilter) || ticket.State != strings.TrimSpace(stateFilter) {
				return false
			}
		} else if ticket.Status != trimmed {
			return false
		}
	}
	if trimmed := strings.TrimSpace(stage); trimmed != "" && ticket.Stage != trimmed {
		return false
	}
	if trimmed := strings.TrimSpace(state); trimmed != "" && ticket.State != trimmed {
		return false
	}
	if trimmed := strings.ToLower(strings.TrimSpace(title)); trimmed != "" && !strings.Contains(strings.ToLower(ticket.Title), trimmed) {
		return false
	}
	if trimmed := strings.ToLower(strings.TrimSpace(description)); trimmed != "" {
		descriptionFields := strings.ToLower(ticket.Description + "\n" + ticket.AcceptanceCriteria)
		if !strings.Contains(descriptionFields, trimmed) {
			return false
		}
	}
	if priority != 0 && ticket.Priority != priority {
		return false
	}
	if trimmed := strings.TrimSpace(owner); trimmed != "" && ticket.Assignee != trimmed {
		return false
	}
	return true
}

func runSetParent(args []string, command string) error {
	usage := fmt.Sprintf("ticket %s [-id] <id> <parent-id> [-m comment]", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 1 {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	idVal = normalizeBareTicketRef(cfg, svc, idVal)
	parentRef := normalizeBareTicketRef(cfg, svc, rest[0])
	child, err := svc.GetTicket(context.Background(), idVal)
	if err != nil {
		return err
	}
	parent, err := svc.GetTicket(context.Background(), parentRef)
	if err != nil {
		return err
	}
	updated, err := svc.SetTicketParent(context.Background(), child.ID, parent.ID, *message)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	return nil
}

func runUnsetParent(args []string, command string) error {
	usage := fmt.Sprintf("ticket %s [-id] <id> [-m comment]", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	idVal = normalizeBareTicketRef(cfg, svc, idVal)
	ticket, err := svc.GetTicket(context.Background(), idVal)
	if err != nil {
		return err
	}
	updated, err := svc.UnsetTicketParent(context.Background(), ticket.ID, *message)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	return nil
}

func runUpdate(args []string) error {
	usage := "tk update [-id <id>|<id>]\n  [-title <title>]\n  [-desc <description> | -description <description>]\n  [-dor <text>] [-dod <text>] [-ac <text>]\n  [-dor-map <stage=value,...>] [-dod-map <stage=value,...>] [-ac-map <stage=value,...>]\n  [-git-repository <repo>]\n  [-git-branch <branch>]\n  [-priority <n>]\n  [-order <n>]\n  [-stage <stage>]\n  [-state <state>]\n  [-status <stage/state>]\n  [-parent_id <id>]\n  [-estimate_effort <n>]\n  [-estimate_complete <rfc3339>]\n  [-t <type> | -type <type>]"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append([]string{"-id", args[0]}, args[1:]...)
	}
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	title := fs.String("title", "", "ticket title")
	description := fs.String("description", "", "ticket description")
	desc := fs.String("desc", "", "ticket description")
	dor := fs.String("dor", "", "ticket default definition of ready")
	dod := fs.String("dod", "", "ticket default definition of done")
	acceptanceCriteria := fs.String("ac", "", "ticket acceptance criteria")
	dorMapRaw := fs.String("dor-map", "", "stage-specific DoR entries (stage=value,...)")
	dodMapRaw := fs.String("dod-map", "", "stage-specific DoD entries (stage=value,...)")
	acMapRaw := fs.String("ac-map", "", "stage-specific acceptance criteria entries (stage=value,...)")
	gitRepository := fs.String("git-repository", "", "ticket git repository")
	gitBranch := fs.String("git-branch", "", "ticket git branch")
	priority := fs.Int("priority", 0, "ticket priority")
	order := fs.Int("order", 0, "ticket order")
	estimateEffort := fs.Int("estimate_effort", 0, "estimated effort")
	estimateComplete := fs.String("estimate_complete", "", "estimated completion time (RFC3339)")
	status := fs.String("status", "", "rendered ticket status (<stage>/<state>)")
	stage := fs.String("stage", "", "ticket stage")
	state := fs.String("state", "", "ticket state")
	parentIDRaw := fs.String("parent_id", "", "ticket parent id")
	message := fs.String("m", "", "comment to attach")
	ticketType := fs.String("type", "", "ticket type (task, bug, epic, spike, chore, story, note, question, requirement, decision)")
	fs.StringVar(ticketType, "t", "", "ticket type (shorthand)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("usage: " + usage)
	}
	if fs.NArg() != 0 {
		return errors.New("usage: " + usage)
	}
	hasTitle := containsFlag(args, "-title")
	hasDescription := containsFlag(args, "-description")
	hasDesc := containsFlag(args, "-desc")
	hasDOR := containsFlag(args, "-dor")
	hasDOD := containsFlag(args, "-dod")
	hasAC := containsFlag(args, "-ac")
	hasDORMap := containsFlag(args, "-dor-map")
	hasDODMap := containsFlag(args, "-dod-map")
	hasACMap := containsFlag(args, "-ac-map")
	hasPriority := containsFlag(args, "-priority")
	hasGitRepository := containsFlag(args, "-git-repository")
	hasGitBranch := containsFlag(args, "-git-branch")
	hasOrder := containsFlag(args, "-order")
	hasEstimateEffort := containsFlag(args, "-estimate_effort")
	hasEstimateComplete := containsFlag(args, "-estimate_complete")
	hasStatus := containsFlag(args, "-status")
	hasStage := containsFlag(args, "-stage")
	hasState := containsFlag(args, "-state")
	hasParentID := containsFlag(args, "-parent_id")
	hasType := containsFlag(args, "-type") || containsFlag(args, "-t")
	if !hasTitle && !hasDescription && !hasDesc && !hasDOR && !hasDOD && !hasAC && !hasDORMap && !hasDODMap && !hasACMap && !hasGitRepository && !hasGitBranch && !hasPriority && !hasOrder && !hasEstimateEffort && !hasEstimateComplete && !hasStatus && !hasStage && !hasState && !hasParentID && !hasType {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticketRef := normalizeBareTicketRef(cfg, svc, strings.TrimSpace(*id))
	current, err := svc.GetTicket(context.Background(), ticketRef)
	if err != nil {
		return err
	}
	next := libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		DORMap:             current.DORMap,
		DODMap:             current.DODMap,
		ACMap:              current.ACMap,
		GitRepository:      current.GitRepository,
		GitBranch:          current.GitBranch,
		ParentID:           current.ParentID,
		Assignee:           current.Assignee,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
	}
	if hasTitle {
		next.Title = *title
	}
	if hasDescription {
		next.Description = *description
	}
	if hasDesc {
		next.Description = *desc
	}
	if hasAC {
		next.AcceptanceCriteria = *acceptanceCriteria
	}
	if hasDOR || hasDORMap {
		nextMap, err := mergeGuidanceMap(current.DORMap, *dor, *dorMapRaw, hasDOR, hasDORMap)
		if err != nil {
			return err
		}
		next.DORMap = nextMap
	}
	if hasDOD || hasDODMap {
		nextMap, err := mergeGuidanceMap(current.DODMap, *dod, *dodMapRaw, hasDOD, hasDODMap)
		if err != nil {
			return err
		}
		next.DODMap = nextMap
	}
	if hasAC || hasACMap {
		nextMap, err := mergeGuidanceMap(current.ACMap, *acceptanceCriteria, *acMapRaw, hasAC, hasACMap)
		if err != nil {
			return err
		}
		next.ACMap = nextMap
	}
	if hasGitRepository {
		next.GitRepository = strings.TrimSpace(*gitRepository)
	}
	if hasGitBranch {
		next.GitBranch = strings.TrimSpace(*gitBranch)
	}
	if hasPriority {
		next.Priority = *priority
	}
	if hasOrder {
		next.Order = *order
	}
	if hasEstimateEffort {
		next.EstimateEffort = *estimateEffort
	}
	if hasEstimateComplete {
		next.EstimateComplete = *estimateComplete
	}
	if hasStatus {
		resolvedStage, resolvedState, err := resolveLifecycleInput(*status, "", "")
		if err != nil {
			return err
		}
		next.Stage = resolvedStage
		next.State = resolvedState
	}
	if hasStage {
		resolvedStage, err := validateTicketStageInput(svc, current, *stage)
		if err != nil {
			return err
		}
		next.Stage = resolvedStage
	}
	if hasState {
		next.State = *state
	}
	if strings.TrimSpace(next.State) == store.StateActive && strings.TrimSpace(next.Assignee) == "" {
		status, err := svc.Status(context.Background())
		if err == nil && status.User != nil && strings.TrimSpace(status.User.Username) != "" {
			next.Assignee = status.User.Username
		} else {
			next.Assignee = fallbackCommandUsername()
		}
	}
	if hasParentID {
		parentRef := normalizeBareTicketRef(cfg, svc, strings.TrimSpace(*parentIDRaw))
		parent, err := svc.GetTicket(context.Background(), parentRef)
		if err != nil {
			return err
		}
		next.ParentID = &parent.ID
	}
	if hasType {
		next.Type = strings.TrimSpace(*ticketType)
	}
	next.Message = strings.TrimSpace(*message)
	updated, err := svc.UpdateTicket(context.Background(), current.ID, next)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printUpdateSummary(updated, current, svc)
	return nil
}

func printUpdateSummary(updated, previous store.Ticket, svc libticket.Service) {
	var changes []string
	if updated.Title != previous.Title {
		changes = append(changes, fmt.Sprintf("title is now %q", updated.Title))
	}
	if updated.Description != previous.Description {
		changes = append(changes, "description updated")
	}
	if updated.AcceptanceCriteria != previous.AcceptanceCriteria {
		changes = append(changes, "acceptance criteria updated")
	}
	if updated.Type != previous.Type {
		changes = append(changes, fmt.Sprintf("type is now %s", updated.Type))
	}
	if updated.Status != previous.Status {
		changes = append(changes, fmt.Sprintf("status is now %s", updated.Status))
	} else if updated.State != previous.State {
		changes = append(changes, fmt.Sprintf("state is now %s", updated.State))
	}
	if updated.Priority != previous.Priority {
		changes = append(changes, fmt.Sprintf("priority is now %d", updated.Priority))
	}
	if updated.Order != previous.Order {
		changes = append(changes, fmt.Sprintf("order is now %d", updated.Order))
	}
	if updated.Assignee != previous.Assignee {
		if updated.Assignee == "" {
			changes = append(changes, "assignee cleared")
		} else {
			changes = append(changes, fmt.Sprintf("assignee is now %s", updated.Assignee))
		}
	}
	if updated.EstimateEffort != previous.EstimateEffort {
		changes = append(changes, fmt.Sprintf("estimate_effort is now %d", updated.EstimateEffort))
	}
	if updated.EstimateComplete != previous.EstimateComplete {
		changes = append(changes, fmt.Sprintf("estimate_complete is now %s", updated.EstimateComplete))
	}
	if updated.GitRepository != previous.GitRepository {
		changes = append(changes, fmt.Sprintf("git-repository is now %s", updated.GitRepository))
	}
	if updated.GitBranch != previous.GitBranch {
		changes = append(changes, fmt.Sprintf("git-branch is now %s", updated.GitBranch))
	}
	prevParent := ""
	if previous.ParentID != nil {
		prevParent = *previous.ParentID
	}
	newParent := ""
	if updated.ParentID != nil {
		newParent = *updated.ParentID
	}
	if newParent != prevParent {
		if newParent == "" {
			changes = append(changes, "parent removed")
		} else {
			parentLabel := newParent
			if p, err := svc.GetTicket(context.Background(), newParent); err == nil {
				parentLabel = ticketLabel(p)
			}
			changes = append(changes, fmt.Sprintf("parent is now %s", parentLabel))
		}
	}
	summary := strings.Join(changes, ", ")
	if summary == "" {
		summary = "no changes"
	}
	fmt.Printf("%s updated (%s)\n", updated.ID, summary)
}

func runAssign(args []string) error {
	fs := flag.NewFlagSet("assign", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 1 {
		return errors.New("usage: tk assign [-id] <id> <name> [-m comment]")
	}
	return assignTicket(idVal, rest[0], true, *message)
}

func runUnassign(args []string) error {
	fs := flag.NewFlagSet("unassign", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 1 {
		return errors.New("usage: tk unassign [-id] <id> <name> [-m comment]")
	}
	return unassignTicket(idVal, rest[0], true, *message)
}

func runClaim(args []string) error {
	rewritten := make([]string, 0, len(args)+2)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-id":
			if i+1 >= len(args) {
				return errors.New("usage: tk claim [-id <id>] [-dry-run]")
			}
			rewritten = append(rewritten, args[i+1])
			i++
		case "-dry-run":
			rewritten = append(rewritten, "--dryrun")
		default:
			rewritten = append(rewritten, args[i])
		}
	}
	return runRequest(rewritten)
}

func runUnclaim(args []string) error {
	fs := flag.NewFlagSet("unclaim", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk unclaim [-id] <id> [-m comment]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	username := strings.TrimSpace(cfg.Username)
	if resolved, rErr := config.ResolveURL(); rErr == nil && resolved.Mode == config.ModeLocal {
		username = localModeUsername()
	}
	if strings.TrimSpace(username) == "" {
		return errors.New("no current username; log in first")
	}
	return unassignTicket(idVal, username, false, *message)
}

func assignTicket(idArg, assignee string, requireAdmin bool, message ...string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		return err
	}
	if requireAdmin && (status.User == nil || status.User.Role != "admin") {
		return errors.New("user is not an admin")
	}
	current, err := svc.GetTicket(context.Background(), idArg)
	if err != nil {
		return err
	}
	var msg string
	if len(message) > 0 {
		msg = message[0]
	}
	updated, err := svc.UpdateTicket(context.Background(), current.ID, libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           assignee,
		State:              current.State,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
		Message:            msg,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	if strings.TrimSpace(updated.Assignee) == "" {
		fmt.Printf("unassigned %s\n", ticketLabel(updated))
		return nil
	}
	fmt.Printf("assigned %s to %s\n", ticketLabel(updated), updated.Assignee)
	return nil
}

func unassignTicket(idArg, expectedAssignee string, requireAdmin bool, message ...string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		return err
	}
	if requireAdmin && (status.User == nil || status.User.Role != "admin") {
		return errors.New("user is not an admin")
	}
	if requireAdmin {
		users, err := svc.ListUsers(context.Background())
		if err != nil {
			return err
		}
		var found bool
		for _, user := range users {
			if user.Username == expectedAssignee {
				found = true
				if !user.Enabled {
					return errors.New("user is disabled")
				}
				break
			}
		}
		if !found {
			return errors.New("user not found")
		}
	}
	current, err := svc.GetTicket(context.Background(), idArg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(current.Assignee) != strings.TrimSpace(expectedAssignee) {
		return fmt.Errorf("ticket is not assigned to %s", expectedAssignee)
	}
	var msg string
	if len(message) > 0 {
		msg = message[0]
	}
	updated, err := svc.UpdateTicket(context.Background(), current.ID, libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           "",
		Stage:              current.Stage,
		State:              map[bool]string{true: store.StateIdle, false: current.State}[current.State == store.StateActive],
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
		Message:            msg,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	fmt.Printf("unassigned %s from %s\n", ticketLabel(updated), expectedAssignee)
	return nil
}

func runHistory(args []string) error {
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	limit := fs.Int("n", 10, "maximum number of events to show; 0 uses default limit")
	offset := fs.Int("offset", 0, "number of ticket history events to skip")
	userID := fs.String("user_id", "", "filter by user id")
	agentID := fs.String("agent_id", "", "filter by agent id")
	teamID := fs.Int64("team_id", 0, "filter by team id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	remaining := fs.Args()
	// Merge -id flag into remaining for uniform handling below.
	if strings.TrimSpace(*id) != "" {
		remaining = append([]string{strings.TrimSpace(*id)}, remaining...)
	}

	filter := store.HistoryFilter{
		UserID:  *userID,
		AgentID: *agentID,
		TeamID:  *teamID,
	}

	// No positional args: show recent events for the active project.
	if len(remaining) == 0 {
		if *offset != 0 {
			return errors.New("offset is only supported when querying a specific ticket")
		}
		_, svc, project, err := resolveCurrentProjectClient()
		if err != nil {
			return err
		}
		events, err := svc.ListProjectHistoryFiltered(context.Background(), project.ID, *limit, filter)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(events)
		}
		if len(events) == 0 {
			fmt.Println("no history")
			return nil
		}
		for _, event := range events {
			key := event.TicketKey
			if key == "" {
				key = "#" + event.TicketID
			}
			fmt.Printf("[%s] %-10s %s\n", event.CreatedAt, key, formatHistoryEvent(event))
		}
		return nil
	}

	if len(remaining) != 1 {
		return errors.New("usage: tk history [-n <limit>] [-offset <offset>] [-user_id ID] [-agent_id ID] [-team_id ID] [<id>]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(context.Background(), remaining[0])
	if err != nil {
		return err
	}
	events, err := svc.ListHistoryPaged(context.Background(), ticket.ID, *limit, *offset)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(events)
	}
	if len(events) == 0 {
		fmt.Println("no history")
		return nil
	}
	for _, event := range events {
		fmt.Printf("[%s] %s\n", event.CreatedAt, formatHistoryEvent(event))
	}
	return nil
}

func runDependencyCommand(args []string, add bool) error {
	command := "add-dependency"
	if !add {
		command = "remove-dependency"
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: tk %s <id> <dependency-id[,dependency-id...]>", command)
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	ticket, err := api.GetTicket(context.Background(), args[0])
	if err != nil {
		return err
	}
	dependencyRefs := strings.Split(args[1], ",")
	if len(dependencyRefs) == 0 {
		return errors.New("at least one dependency id is required")
	}
	for _, depRef := range dependencyRefs {
		dependencyTicket, err := api.GetTicket(context.Background(), strings.TrimSpace(depRef))
		if err != nil {
			return err
		}
		dependencyRequest := libticket.DependencyRequest{
			ProjectID: project.ID,
			TicketID:  ticket.ID,
			DependsOn: dependencyTicket.ID,
		}
		if add {
			if _, err := api.AddDependency(context.Background(), dependencyRequest); err != nil {
				return err
			}
			continue
		}
		if err := api.RemoveDependency(context.Background(), dependencyRequest); err != nil {
			return err
		}
	}
	if outputJSON {
		return printJSON(map[string]any{
			"task_id":      ticket.ID,
			"dependencies": args[1],
			"action":       map[bool]string{true: "added", false: "removed"}[add],
		})
	}
	action := "added"
	if !add {
		action = "removed"
	}
	fmt.Printf("%s dependencies for %s: %s\n", action, ticketLabel(ticket), args[1])
	return nil
}

func runDependency(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(depUsage)
		return nil
	}
	depUsageErr := "usage: tk dep <add|remove> -id <id> <dependency-id>"
	action := args[0]
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("dependency add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "ticket id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*id) == "" || fs.NArg() != 1 {
			return errors.New(depUsageErr)
		}
		return runDependencyCommand([]string{strings.TrimSpace(*id), strings.TrimSpace(fs.Args()[0])}, true)
	case "remove":
		fs := flag.NewFlagSet("dependency remove", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "ticket id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*id) == "" || fs.NArg() != 1 {
			return errors.New(depUsageErr)
		}
		return runDependencyCommand([]string{strings.TrimSpace(*id), strings.TrimSpace(fs.Args()[0])}, false)
	default:
		if action == "" {
			return errors.New(depUsageErr)
		}
		return fmt.Errorf("unknown dep command %q; see: ticket dep help", action)
	}
}

func runRequest(args []string) error {
	dryRun := false
	explain := false
	var requestedRef string
	for _, arg := range args {
		switch arg {
		case "--dryrun", "-dryrun":
			dryRun = true
		case "--explain", "-explain":
			explain = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("usage: tk request [--dryrun] [--explain] [<id>]")
			}
			if requestedRef != "" {
				return fmt.Errorf("usage: tk request [--dryrun] [--explain] [<id>]")
			}
			requestedRef = arg
		}
	}
	if dryRun {
		if requestedRef == "" {
			return runRequestDryRun(nil)
		}
		return runRequestDryRun([]string{requestedRef})
	}

	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	taskRequest := libticket.TicketRequest{ProjectID: project.ID}
	if requestedRef != "" {
		taskRequest.TicketRef = requestedRef
	}
	response, err := api.RequestTicket(context.Background(), taskRequest)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(response)
	}
	if response.Ticket != nil {
		printRequestContext(response)
		return nil
	}
	fmt.Println(response.Status)
	if explain {
		fmt.Println(requestStatusExplanation(response.Status, requestedRef))
	}
	return nil
}

func requestStatusExplanation(status, requestedRef string) string {
	switch strings.TrimSpace(status) {
	case "NO-WORK":
		return "explain: no eligible ticket was found for assignment. Check project status, ticket stage/state, draft/archive/complete flags, and dependencies."
	case "REJECTED":
		if strings.TrimSpace(requestedRef) != "" {
			return "explain: the requested ticket is not claimable for your current context (already assigned, wrong project, blocked, or not in a claimable stage/state)."
		}
		return "explain: request was rejected because the selected ticket is not claimable in your current context."
	case "ASSIGNED":
		return "explain: work was assigned to you."
	case "AVAILABLE":
		return "explain: dry-run found assignable work."
	default:
		return "explain: no additional detail available for this status."
	}
}

func runRequestDryRun(args []string) error {
	if len(args) > 1 {
		return errors.New("usage: tk request-dryrun [<id>]")
	}

	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}

	var requestedRef string
	if len(args) == 1 {
		requestedRef = args[0]
	}

	response, err := api.RequestTicket(context.Background(), libticket.TicketRequest{
		ProjectID: project.ID,
		TicketRef: requestedRef,
		DryRun:    true,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(response)
	}
	fmt.Printf("dry run: %s\n", response.Status)
	if response.Ticket == nil {
		return nil
	}
	fmt.Printf("would assign ticket: %s\n", ticketLabel(*response.Ticket))
	printRequestContext(response)
	return nil
}

func parseIDList(raw string) ([]int64, error) {
	parts := strings.Split(raw, ",")
	if len(parts) == 0 {
		return nil, errors.New("at least one dependency id is required")
	}
	var ids []int64
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("dependency ids must be numeric")
		}
		var id int64
		if _, err := fmt.Sscan(part, &id); err != nil {
			return nil, errors.New("dependency ids must be numeric")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func runComment(args []string) error {
	usage := "tk comment <id> \"comment\""
	if len(args) == 0 {
		return errors.New("usage: " + usage)
	}
	// Support "add" subcommand for backwards compatibility
	addArgs := args
	if args[0] == "add" {
		addArgs = args[1:]
	} else if args[0] == "help" || args[0] == "-help" || args[0] == "--help" {
		fmt.Println("usage: " + usage)
		return nil
	}
	fs := flag.NewFlagSet("ticket comment", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(addArgs); err != nil {
		return err
	}
	idVal := strings.TrimSpace(*id)
	var commentText string
	switch {
	case idVal != "" && len(fs.Args()) == 1:
		commentText = fs.Args()[0]
	case idVal == "" && len(fs.Args()) == 2:
		idVal = fs.Args()[0]
		commentText = fs.Args()[1]
	default:
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(context.Background(), idVal)
	if err != nil {
		return err
	}
	comment, err := svc.AddComment(context.Background(), ticket.ID, commentText)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(comment)
	}
	fmt.Printf("commented on %s: %s\n", ticketLabel(ticket), comment.Comment)
	return nil
}

func runClone(args []string) error {
	fs := flag.NewFlagSet("clone", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	message := fs.String("m", "", "comment to attach")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk clone|cp [-id] <id> [-m comment]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	taskRef, err := svc.GetTicket(context.Background(), idVal)
	if err != nil {
		return err
	}
	ticket, err := svc.CloneTicket(context.Background(), taskRef.ID, *message)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(ticket)
	}
	printTicket(ticket)
	fmt.Printf("clone_of: %s\n", ticketLabel(taskRef))
	return nil
}

func runDeleteTicket(args []string) error {
	usage := "tk rm|delete [-id] <id> [--confirm <token>]"
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	confirm := fs.String("confirm", "", "confirmation token from first run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(context.Background(), idVal)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*confirm) == "" {
		// Phase 1: generate confirmation token
		token, err := generateConfirmToken()
		if err != nil {
			return err
		}
		fmt.Printf("ticket   : %s — %s\n", ticket.ID, ticket.Title)
		fmt.Printf("type     : %s\n", ticket.Type)
		fmt.Printf("\nThis will permanently delete the ticket and all associated data.\n")
		fmt.Printf("To confirm, run:\n\n")
		fmt.Printf("  tk rm -id %s --confirm %s\n\n", ticket.ID, token)
		cfg.DeleteConfirmToken = token
		cfg.DeleteConfirmTicket = ticket.ID
		return config.Save(cfg)
	}
	// Phase 2: verify token and delete
	if *confirm != cfg.DeleteConfirmToken || ticket.ID != cfg.DeleteConfirmTicket {
		return errors.New("invalid confirmation token")
	}
	if err := svc.DeleteTicket(context.Background(), ticket.ID); err != nil {
		return err
	}
	cfg.DeleteConfirmToken = ""
	cfg.DeleteConfirmTicket = ""
	if err := config.Save(cfg); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]any{"status": "deleted", "ticket_id": ticket.ID})
	}
	fmt.Printf("deleted ticket %s\n", ticketLabel(ticket))
	return nil
}

func runTypedTicketCreate(ticketType string, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "list", "ls":
			return runTypedTicketList(ticketType)
		case "get", "show":
			if len(args) > 2 {
				return fmt.Errorf("usage: tk %s get <id>", ticketType)
			}
			id := ""
			if len(args) == 2 {
				id = args[1]
			}
			return runTypedTicketGet(ticketType, id)
		case "new", "add", "create":
			return runTicketCreate(append([]string{"-type", ticketType}, args[1:]...))
		}
	}
	return runTicketCreate(append([]string{"-type", ticketType}, args...))
}

func runTypedTicketList(ticketType string) error {
	_, svc, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	tickets, err := svc.ListTicketsFiltered(context.Background(), project.ID, ticketType, "", "", "", "", "", 0, false)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(tickets)
	}
	if len(tickets) == 0 {
		printNoEntitiesAvailable(entityPlural(ticketType))
		return nil
	}
	for _, ticket := range tickets {
		fmt.Printf("%s\t%s\t%s\n", ticketLabel(ticket), ticket.Status, ticket.Title)
	}
	return nil
}

func runTypedTicketGet(ticketType, id string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	resolvedID, err := resolveTypedTicketRef(cfg, svc, ticketType, id)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(context.Background(), resolvedID)
	if err != nil {
		return err
	}
	if ticket.Type != ticketType {
		article := "a"
		if strings.HasPrefix(ticketType, "a") || strings.HasPrefix(ticketType, "e") || strings.HasPrefix(ticketType, "i") || strings.HasPrefix(ticketType, "o") || strings.HasPrefix(ticketType, "u") {
			article = "an"
		}
		return fmt.Errorf("ticket %s is not %s %s", resolvedID, article, ticketType)
	}
	return runGet([]string{"-id", ticket.ID})
}

type ticketCreateOptions struct {
	TicketType         string
	Title              string
	Description        string
	AcceptanceCriteria string
	DORMap             store.GuidanceMap
	DODMap             store.GuidanceMap
	ACMap              store.GuidanceMap
	GitRepository      string
	GitBranch          string
	Priority           int
	EstimateEffort     int
	EstimateComplete   string
	Assignee           string
	ParentID           *string
	Project            string
	Message            string
	PrintID            bool
}

func runTicketCreate(args []string) error {
	normalizedArgs, err := normalizeTicketCreateArgs(args)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	taskType := fs.String("type", "task", "ticket type")
	fs.StringVar(taskType, "t", "task", "ticket type")
	titleFlag := fs.String("title", "", "ticket title")
	priority := fs.Int("priority", 1, "ticket priority")
	fs.IntVar(priority, "p", 1, "ticket priority")
	estimateEffort := fs.Int("estimate_effort", 0, "estimated effort")
	assignee := fs.String("assignee", "", "ticket assignee")
	fs.StringVar(assignee, "a", "", "ticket assignee")
	description := fs.String("description", "", "ticket description")
	fs.StringVar(description, "d", "", "ticket description")
	dor := fs.String("dor", "", "default definition of ready")
	dod := fs.String("dod", "", "default definition of done")
	acceptanceCriteria := fs.String("ac", "", "acceptance criteria")
	dorMapRaw := fs.String("dor-map", "", "stage-specific DoR entries (stage=value,...)")
	dodMapRaw := fs.String("dod-map", "", "stage-specific DoD entries (stage=value,...)")
	acMapRaw := fs.String("ac-map", "", "stage-specific acceptance criteria entries (stage=value,...)")
	gitRepository := fs.String("git-repository", "", "ticket git repository")
	gitBranch := fs.String("git-branch", "", "ticket git branch")
	estimateComplete := fs.String("estimate_complete", "", "estimated completion time (RFC3339)")
	parent := fs.String("parent", "", "parent ticket id")
	project := fs.String("project", "", "project id")
	message := fs.String("m", "", "comment to attach")
	printID := fs.Bool("printid", false, "print only the created ticket id")
	if err := fs.Parse(normalizedArgs); err != nil {
		return err
	}
	title := strings.TrimSpace(*titleFlag)
	if title == "" {
		title = strings.Join(fs.Args(), " ")
	}

	// Support @filename: read markdown file, parse key-value headers and body as description.
	if strings.HasPrefix(title, "@") {
		filePath := title[1:]
		data, err := os.ReadFile(filePath) // #nosec G304 -- user-specified file path for ticket creation
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", filePath, err)
		}
		fileTitle, fileFields, fileBody := parseTicketFile(string(data))
		if fileTitle != "" && *titleFlag == "" {
			title = fileTitle
		}
		if v, ok := fileFields["type"]; ok && *taskType == "task" {
			*taskType = v
		}
		if v, ok := fileFields["priority"]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				*priority = n
			}
		}
		if v, ok := fileFields["assignee"]; ok && *assignee == "" {
			*assignee = v
		}
		if v, ok := fileFields["parent"]; ok && *parent == "" {
			*parent = v
		}
		if v, ok := fileFields["project"]; ok && *project == "" {
			*project = v
		}
		if v, ok := fileFields["dor"]; ok && *dor == "" {
			*dor = v
		}
		if v, ok := fileFields["dod"]; ok && *dod == "" {
			*dod = v
		}
		if v, ok := fileFields["ac"]; ok && *acceptanceCriteria == "" {
			*acceptanceCriteria = v
		}
		if fileBody != "" && *description == "" {
			*description = fileBody
		}
	}

	if title == "" {
		return errors.New("usage: tk add|create|new [-title title] [-t type] [-p priority] [-a assignee] [-d description] [-dor text] [-dod text] [-ac text] [-dor-map stage=value,...] [-dod-map stage=value,...] [-ac-map stage=value,...] [-parent id] [-project project] [-estimate_effort n] [-estimate_complete rfc3339] [title words | @filename]")
	}
	dorMap, err := mergeGuidanceMap(nil, *dor, *dorMapRaw, containsFlag(normalizedArgs, "-dor"), containsFlag(normalizedArgs, "-dor-map"))
	if err != nil {
		return err
	}
	dodMap, err := mergeGuidanceMap(nil, *dod, *dodMapRaw, containsFlag(normalizedArgs, "-dod"), containsFlag(normalizedArgs, "-dod-map"))
	if err != nil {
		return err
	}
	acMap, err := mergeGuidanceMap(nil, *acceptanceCriteria, *acMapRaw, containsFlag(normalizedArgs, "-ac"), containsFlag(normalizedArgs, "-ac-map"))
	if err != nil {
		return err
	}
	opts := ticketCreateOptions{
		TicketType:         *taskType,
		Title:              title,
		Description:        *description,
		AcceptanceCriteria: *acceptanceCriteria,
		DORMap:             dorMap,
		DODMap:             dodMap,
		ACMap:              acMap,
		GitRepository:      strings.TrimSpace(*gitRepository),
		GitBranch:          strings.TrimSpace(*gitBranch),
		Priority:           *priority,
		EstimateEffort:     *estimateEffort,
		EstimateComplete:   *estimateComplete,
		Assignee:           *assignee,
		Project:            *project,
		Message:            strings.TrimSpace(*message),
		PrintID:            *printID,
	}
	if *parent != "" {
		opts.ParentID = parent
	}
	return createTicket(opts)
}

func normalizeTicketCreateArgs(args []string) ([]string, error) {
	knownValueFlags := map[string]bool{
		"-type":              true,
		"-t":                 true,
		"-title":             true,
		"-priority":          true,
		"-p":                 true,
		"-estimate_effort":   true,
		"-assignee":          true,
		"-a":                 true,
		"-description":       true,
		"-d":                 true,
		"-dor":               true,
		"-dod":               true,
		"-ac":                true,
		"-dor-map":           true,
		"-dod-map":           true,
		"-ac-map":            true,
		"-git-repository":    true,
		"-git-branch":        true,
		"-estimate_complete": true,
		"-parent":            true,
		"-project":           true,
		"-m":                 true,
	}
	knownBoolFlags := map[string]bool{
		"-printid": true,
	}

	var flagArgs []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if knownValueFlags[arg] {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag needs an argument: %s", arg)
			}
			flagArgs = append(flagArgs, arg, args[i+1])
			i++
			continue
		}
		if knownBoolFlags[arg] {
			flagArgs = append(flagArgs, arg)
			continue
		}
		positional = append(positional, arg)
	}
	// Insert "--" before positional args so flag.Parse won't treat
	// words like "-id" in the title as unknown flags.
	if len(positional) > 0 {
		flagArgs = append(flagArgs, "--")
		return append(flagArgs, positional...), nil
	}
	return flagArgs, nil
}

func createTicket(opts ticketCreateOptions) error {
	cfg, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	if strings.TrimSpace(opts.Project) != "" {
		project, err = api.GetProject(context.Background(), opts.Project)
		if err != nil {
			return err
		}
	}
	parentID := opts.ParentID
	ticketType := strings.TrimSpace(strings.ToLower(opts.TicketType))
	if parentID == nil && cfg.CurrentEpicID != "" && (ticketType == "task" || ticketType == "bug" || ticketType == "chore") {
		epic, err := api.GetTicket(context.Background(), cfg.CurrentEpicID)
		if err != nil {
			return fmt.Errorf("current epic id %s is invalid: %w", cfg.CurrentEpicID, err)
		}
		if strings.TrimSpace(strings.ToLower(epic.Type)) != "epic" {
			return fmt.Errorf("current epic id %s is not an epic", cfg.CurrentEpicID)
		}
		if epic.ProjectID != project.ID {
			return fmt.Errorf("current epic id %s belongs to project %d, active project is %d", cfg.CurrentEpicID, epic.ProjectID, project.ID)
		}
		parentID = &epic.ID
	}
	ticket, err := api.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID:          project.ID,
		ParentID:           parentID,
		Type:               opts.TicketType,
		Title:              opts.Title,
		Description:        opts.Description,
		AcceptanceCriteria: opts.AcceptanceCriteria,
		DORMap:             opts.DORMap,
		DODMap:             opts.DODMap,
		ACMap:              opts.ACMap,
		GitRepository:      opts.GitRepository,
		GitBranch:          opts.GitBranch,
		Priority:           opts.Priority,
		EstimateEffort:     opts.EstimateEffort,
		EstimateComplete:   opts.EstimateComplete,
		Assignee:           opts.Assignee,
		Message:            opts.Message,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(ticket)
	}
	if printCreatedID(ticket.ID, opts.PrintID) {
		return nil
	}
	if ticket.Type == "epic" {
		cfg.CurrentEpicID = ticket.ID
		if err := config.Save(cfg); err != nil {
			return err
		}
	}
	fmt.Println(ticketLabel(ticket))
	return nil
}

// parseTicketFile parses a markdown file into ticket fields. Lines at the top
// of the file matching "key: value" are extracted as fields. The first
// "title: ..." line sets the title. Everything after the key-value header
// becomes the description body.
func parseTicketFile(content string) (title string, fields map[string]string, body string) {
	fields = make(map[string]string)
	lines := strings.Split(content, "\n")
	bodyStart := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			// Blank line ends the header section.
			bodyStart = i + 1
			break
		}
		idx := strings.Index(trimmed, ":")
		if idx < 1 {
			// Not a key-value line — treat everything from here as body.
			bodyStart = i
			break
		}
		key := strings.TrimSpace(strings.ToLower(trimmed[:idx]))
		value := strings.TrimSpace(trimmed[idx+1:])
		if key == "title" {
			title = value
		} else {
			fields[key] = value
		}
		bodyStart = i + 1
	}
	body = strings.TrimSpace(strings.Join(lines[bodyStart:], "\n"))
	return title, fields, body
}
