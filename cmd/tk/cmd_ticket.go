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
	"github.com/simonski/ticket/internal/ticketmarkdown"
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

// ticketIDSeq returns the numeric sequence suffix from a ticket ID like "PRJ-42".
func ticketIDSeq(id string) int {
	if i := strings.LastIndex(id, "-"); i >= 0 {
		n, _ := strconv.Atoi(id[i+1:])
		return n
	}
	return 0
}

func effectiveTicketUpdatedAt(tickets []store.Ticket) map[string]string {
	byID := make(map[string]store.Ticket, len(tickets))
	children := make(map[string][]string, len(tickets))
	for _, ticket := range tickets {
		byID[ticket.ID] = ticket
		if ticket.ParentID != nil {
			children[*ticket.ParentID] = append(children[*ticket.ParentID], ticket.ID)
		}
	}

	effective := make(map[string]string, len(tickets))
	var visit func(id string) string
	visit = func(id string) string {
		if cached, ok := effective[id]; ok {
			return cached
		}
		best := strings.TrimSpace(byID[id].UpdatedAt)
		for _, childID := range children[id] {
			if childUpdated := visit(childID); childUpdated > best {
				best = childUpdated
			}
		}
		effective[id] = best
		return best
	}

	for _, ticket := range tickets {
		visit(ticket.ID)
	}
	return effective
}

func ticketListMoreRecent(left, right store.Ticket, effectiveUpdatedAt map[string]string) bool {
	leftEffective := strings.TrimSpace(effectiveUpdatedAt[left.ID])
	rightEffective := strings.TrimSpace(effectiveUpdatedAt[right.ID])
	if leftEffective != rightEffective {
		return leftEffective > rightEffective
	}

	leftUpdated := strings.TrimSpace(left.UpdatedAt)
	rightUpdated := strings.TrimSpace(right.UpdatedAt)
	if leftUpdated != rightUpdated {
		return leftUpdated > rightUpdated
	}

	leftSort := ticketSortKey(left)
	rightSort := ticketSortKey(right)
	if leftSort != rightSort {
		return leftSort < rightSort
	}

	return left.ID < right.ID
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
	case "intervene":
		return runIntervene(args[1:])

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
	case "merge":
		return runMerge(args[1:])
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
  get     -id <id> [-v] [-json]               View ticket detail
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
  intervene -id <id> -outcome <decision>      Apply intervention decision

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
  merge    <target-id> <source-id>...        Merge draft tickets into the first ticket
  delete   -id <id>                           Soft-delete

  gen      -f <files> -o <output>             Generate tickets via agent`

func runTicketExport(args []string) error {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append([]string{"-id", args[0]}, args[1:]...)
	}
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	outputPath := fs.String("o", "", "write markdown to file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk export [-id] <id> [-o <file>]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticketRef := normalizeBareTicketRef(cfg, svc, idVal)
	ticket, err := svc.GetTicket(context.Background(), ticketRef)
	if err != nil {
		return err
	}
	content := ticketmarkdown.Render(ticket)
	if outputJSON {
		return printJSON(map[string]string{
			"ticket_id": ticket.ID,
			"content":   content,
		})
	}
	if strings.TrimSpace(*outputPath) == "" {
		fmt.Print(content)
		return nil
	}
	if err := os.WriteFile(strings.TrimSpace(*outputPath), []byte(content), 0o600); err != nil {
		return fmt.Errorf("cannot write %s: %w", strings.TrimSpace(*outputPath), err)
	}
	fmt.Printf("%s exported to %s\n", ticket.ID, strings.TrimSpace(*outputPath))
	return nil
}

func runTicketImport(args []string) error {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append([]string{"-f", args[0]}, args[1:]...)
	}
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	filePath := fs.String("f", "", "import markdown from file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*filePath) == "" || fs.NArg() != 0 {
		return errors.New("usage: tk import [-f] <file>")
	}
	data, err := os.ReadFile(strings.TrimSpace(*filePath)) // #nosec G304 -- user-specified import path
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", strings.TrimSpace(*filePath), err)
	}
	doc, err := ticketmarkdown.Parse(string(data))
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	current, err := svc.GetTicket(context.Background(), normalizeBareTicketRef(cfg, svc, doc.ID))
	if err != nil {
		return err
	}
	updated, err := svc.ImportTicketMarkdown(context.Background(), libticket.TicketMarkdownImportRequest{Content: string(data)})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printUpdateSummary(updated, current, svc)
	return nil
}

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
// parseFlagsWithPositionals parses fs while allowing flags to appear before or
// after positional arguments. Go's flag package stops parsing at the first
// non-flag token, so without this a flag that trails a positional id (for
// example "tk active 42 -m note") is silently left unparsed and the command
// fails. Positional arguments are returned in their original order.
func parseFlagsWithPositionals(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		positionals = append(positionals, rest[0])
		args = rest[1:]
	}
	return positionals, nil
}

func resolveIDFlag(flagVal string, positional []string) (id string, remaining []string, err error) {
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

func resolveLifecycleInput(status, stage, state string) (resolvedStage, resolvedState string, err error) {
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
		return errors.New("usage: tk list|ls [<type>]\n" +
			"  [-type <type>] [-t <type>] [-stage <stage>] [-state <state>] [-status <stage/state>]\n" +
			"  [-u <user>] [-n <limit>] [-a] [-d] [-label <name>]\n" +
			"  [-count] [-expect_equals <n>] [-expect_notequals <n>]")
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
		printListProjectHeader(project)
		if strings.TrimSpace(*taskType) == "" {
			fmt.Println(noTicketsAvailableForProject(project.Title))
			return nil
		}
		printNoEntitiesAvailable(entityPlural(*taskType))
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
	effectiveUpdatedAt := effectiveTicketUpdatedAt(tickets)
	// Active tickets float to the top (TK-73); within each group keep the
	// most-recent-first ordering. printTicketTable draws a separator between the
	// leading active block and the rest.
	sort.SliceStable(tickets, func(i, j int) bool {
		ai, aj := ticketIsActive(tickets[i]), ticketIsActive(tickets[j])
		if ai != aj {
			return ai
		}
		return ticketListMoreRecent(tickets[i], tickets[j], effectiveUpdatedAt)
	})
	// Build agent username set so we can prefix agent assignees.
	agentUsernames := make(map[string]bool)
	if agents, err := api.ListAgents(context.Background()); err == nil {
		for _, a := range agents {
			agentUsernames[a.Username] = true
		}
	}
	// Build a per-ticket PR summary so the table can show a PR column (TK-83).
	// Shows the latest PR (highest id) as "#id status", with "+N" when a ticket
	// has more than one. The column only renders when some ticket has a PR.
	prByTicket := make(map[string]string)
	if prs, prErr := api.ListPullRequestsByProject(context.Background(), fmt.Sprintf("%d", project.ID)); prErr == nil {
		type prAgg struct {
			latest store.PullRequest
			count  int
		}
		agg := make(map[string]*prAgg)
		for _, pr := range prs {
			a := agg[pr.TicketID]
			if a == nil {
				a = &prAgg{}
				agg[pr.TicketID] = a
			}
			a.count++
			if pr.ID >= a.latest.ID {
				a.latest = pr
			}
		}
		for ticketID, a := range agg {
			// Compact, fixed-width-friendly: latest PR id, "+N" for extras.
			summary := fmt.Sprintf("#%d", a.latest.ID)
			if a.count > 1 {
				summary += fmt.Sprintf("+%d", a.count-1)
			}
			prByTicket[ticketID] = summary
		}
	}
	printTicketTable(tickets, parentKeys, agentUsernames, statusUnicode, *includeAll, projectHeaderLabel(project), prByTicket)
	return nil
}

// ticketIsActive reports whether a ticket is in the active state.
func ticketIsActive(t store.Ticket) bool {
	return strings.EqualFold(strings.TrimSpace(t.State), "active")
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
	usage := "tk get [-id] <id> [-v]"
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	verbose := fs.Bool("v", false, "show full ticket detail")
	rest, err := parseFlagsWithPositionals(fs, args)
	if err != nil {
		return err
	}
	// Allow positional: tk get FOO is the same as tk get -id FOO.
	if strings.TrimSpace(*id) == "" && len(rest) >= 1 {
		v := rest[0]
		id = &v
		rest = rest[1:]
	}
	if len(rest) != 0 {
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
		// No ID: fall back to the most recently updated ticket in the project.
		_, projectSvc, resolvedProject, resolveErr := resolveCurrentProjectClient()
		if resolveErr != nil {
			return resolveErr
		}
		tickets, listErr := projectSvc.ListTicketsFiltered(context.Background(), resolvedProject.ID, "", "", "", "", "", "", 0, false)
		if listErr != nil {
			return listErr
		}
		if len(tickets) == 0 {
			return errors.New("no tickets in project")
		}
		latest := tickets[0]
		for _, t := range tickets[1:] {
			if t.UpdatedAt > latest.UpdatedAt ||
				(t.UpdatedAt == latest.UpdatedAt && ticketIDSeq(t.ID) > ticketIDSeq(latest.ID)) {
				latest = t
			}
		}
		ticketRef = latest.ID
	}
	ticketRef = normalizeBareTicketRef(cfg, svc, ticketRef)
	ticket, err := svc.GetTicket(context.Background(), ticketRef)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(ticket)
	}
	pullRequests, _ := svc.ListPullRequestsByTicket(context.Background(), ticket.ID)
	if !*verbose {
		printTicketSummary(ticket)
		printTicketPullRequests(pullRequests)
		return nil
	}
	dependencies, _ := svc.ListDependencies(context.Background(), ticket.ID)
	history, _ := svc.ListHistory(context.Background(), ticket.ID)
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
	printTicketPullRequests(pullRequests)
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
		return errors.New("usage: tk search <free form query>\n" +
			"  [-status <status>] [-title <text>] [-description <text>]\n" +
			"  [-priority <n>] [-owner <user>]")
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
	usage := "tk update [-f <filename>] [-commit] [-id <id>|<id>]\n  [-title <title>]\n  [-desc <description> | -description <description>]\n  [-dor <text>] [-dod <text>] [-ac <text>]\n  [-dor-map <stage=value,...>] [-dod-map <stage=value,...>] [-ac-map <stage=value,...>]\n  [-git-repository <repo>]\n  [-git-branch <branch>]\n  [-priority <n>]\n  [-order <n>]\n  [-stage <stage>]\n  [-state <state>]\n  [-status <stage/state>]\n  [-parent_id <id>]\n  [-estimate_effort <n>]\n  [-estimate_complete <rfc3339>]\n  [-t <type> | -type <type>]"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append([]string{"-id", args[0]}, args[1:]...)
	}
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	filePath := fs.String("f", "", "update tickets from a file")
	commit := fs.Bool("commit", false, "apply updates to storage")
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
	ticketType := fs.String("type", "", "ticket type (task, bug, epic, spike, chore, story, note, question, requirement, decision, action)")
	fs.StringVar(ticketType, "t", "", "ticket type (shorthand)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	inputFile := strings.TrimSpace(*filePath)
	if inputFile != "" {
		if strings.TrimSpace(*id) != "" || fs.NArg() != 0 {
			return errors.New("usage: tk update -f <filename> [-commit]")
		}
		return runUpdateFromFile(inputFile, *commit, strings.TrimSpace(*message))
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
		nextMap, mergeErr := mergeGuidanceMap(current.DORMap, *dor, *dorMapRaw, hasDOR, hasDORMap)
		if mergeErr != nil {
			return mergeErr
		}
		next.DORMap = nextMap
	}
	if hasDOD || hasDODMap {
		nextMap, mergeErr := mergeGuidanceMap(current.DODMap, *dod, *dodMapRaw, hasDOD, hasDODMap)
		if mergeErr != nil {
			return mergeErr
		}
		next.DODMap = nextMap
	}
	if hasAC || hasACMap {
		nextMap, mergeErr := mergeGuidanceMap(current.ACMap, *acceptanceCriteria, *acMapRaw, hasAC, hasACMap)
		if mergeErr != nil {
			return mergeErr
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
		resolvedStage, resolvedState, lifecycleErr := resolveLifecycleInput(*status, "", "")
		if lifecycleErr != nil {
			return lifecycleErr
		}
		next.Stage = resolvedStage
		next.State = resolvedState
	}
	if hasStage {
		resolvedStage, stageErr := validateTicketStageInput(svc, current, *stage)
		if stageErr != nil {
			return stageErr
		}
		next.Stage = resolvedStage
	}
	if hasState {
		next.State = *state
	}
	if strings.TrimSpace(next.State) == store.StateActive && strings.TrimSpace(next.Assignee) == "" {
		statusInfo, statusErr := svc.Status(context.Background())
		if statusErr == nil && statusInfo.User != nil && strings.TrimSpace(statusInfo.User.Username) != "" {
			next.Assignee = statusInfo.User.Username
		} else {
			next.Assignee = fallbackCommandUsername()
		}
	}
	if hasParentID {
		parentRef := normalizeBareTicketRef(cfg, svc, strings.TrimSpace(*parentIDRaw))
		parent, parentErr := svc.GetTicket(context.Background(), parentRef)
		if parentErr != nil {
			return parentErr
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

func runUpdateFromFile(filePath string, commit bool, message string) error {
	data, readErr := os.ReadFile(filePath) // #nosec G304 -- user-specified file path for ticket update
	if readErr != nil {
		return fmt.Errorf("cannot read %s: %w", filePath, readErr)
	}
	if ticketmarkdown.LooksLikeDocument(string(data)) {
		return fmt.Errorf("cannot parse %s: this file is a single-ticket markdown export; use `tk import %s`", filePath, filePath)
	}
	entries, parseErr := parseTicketBatchFile(string(data), "")
	if parseErr != nil {
		return fmt.Errorf("cannot parse %s: %w", filePath, parseErr)
	}
	for i, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" {
			return fmt.Errorf("cannot parse %s: ticket %d (%s) is missing id", filePath, i+1, entry.Title)
		}
	}
	if !commit {
		printTicketBatchIntent(entries, "update")
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
	for i := range entries {
		var structuredParentID *string
		if entries[i].ParentIndex >= 0 {
			if entries[i].ParentIndex >= len(entries) {
				return fmt.Errorf("cannot parse %s: invalid parent reference for %q", filePath, entries[i].Title)
			}
			parentID := strings.TrimSpace(entries[entries[i].ParentIndex].ID)
			if parentID == "" {
				return fmt.Errorf("cannot parse %s: parent id is missing for %q", filePath, entries[i].Title)
			}
			structuredParentID = &parentID
		}
		updated, updateErr := updateTicketFromBatchEntry(context.Background(), cfg, svc, entries[i], message, structuredParentID)
		if updateErr != nil {
			return updateErr
		}
		entries[i].ID = updated.ID
		if outputJSON {
			if err := printJSON(updated); err != nil {
				return err
			}
			continue
		}
		fmt.Printf("%s\t%s\t%s\t%s\n", ticketLabel(updated), updated.Type, updated.Status, updated.Title)
	}
	if writeErr := os.WriteFile(filePath, []byte(serializeTicketBatchFile(entries)), 0o600); writeErr != nil {
		return fmt.Errorf("cannot write %s: %w", filePath, writeErr)
	}
	return nil
}

func updateTicketFromBatchEntry(ctx context.Context, cfg config.Config, svc libticket.Service, entry ticketBatchEntry, message string, parentID *string) (store.Ticket, error) {
	ticketRef := normalizeBareTicketRef(cfg, svc, strings.TrimSpace(entry.ID))
	current, err := svc.GetTicket(ctx, ticketRef)
	if err != nil {
		return store.Ticket{}, err
	}
	next := libticket.TicketUpdateRequest{
		Title:              entry.Title,
		Description:        entry.Description,
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
		Type:               current.Type,
		Message:            message,
	}
	if parentID != nil {
		next.ParentID = parentID
	}
	if entry.TypeExplicit && strings.TrimSpace(entry.TicketType) != "" {
		next.Type = strings.TrimSpace(entry.TicketType)
	}
	updated, err := svc.UpdateTicket(ctx, current.ID, next)
	if err != nil {
		return store.Ticket{}, err
	}
	if labelErr := syncTicketLabels(ctx, svc, current.ProjectID, updated.ID, entry.Labels); labelErr != nil {
		return store.Ticket{}, labelErr
	}
	refreshed, err := svc.GetTicket(ctx, updated.ID)
	if err != nil {
		return store.Ticket{}, err
	}
	return refreshed, nil
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
	positional, err := parseFlagsWithPositionals(fs, args)
	if err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, positional)
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk unclaim [-id] <id> [-m comment]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	username := strings.TrimSpace(cfg.Username)
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
		users, usersErr := svc.ListUsers(context.Background())
		if usersErr != nil {
			return usersErr
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
			if key == "" && event.TicketID == "" {
				key = "project"
			} else if key == "" {
				key = "#" + event.TicketID
			}
			fmt.Printf("[%s] %-10s %s\n", event.CreatedAt, key, formatHistoryEvent(event))
		}
		return nil
	}

	if len(remaining) != 1 {
		return errors.New("usage: tk history [<id>]\n" +
			"  [-n <limit>] [-offset <offset>]\n" +
			"  [-user_id ID] [-agent_id ID] [-team_id ID]")
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

func runIntervene(args []string) error {
	fs := flag.NewFlagSet("intervene", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	outcome := fs.String("outcome", "", "decision outcome: retry-role|retry-stage|split-work|cancel")
	message := fs.String("m", "", "decision message/comment")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ticketID, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: tk intervene [-id] <id>\n" +
			"  -outcome <retry-role|retry-stage|split-work|cancel>\n" +
			"  [-m comment]")
	}
	if strings.TrimSpace(*outcome) == "" {
		return errors.New("usage: tk intervene [-id] <id>\n" +
			"  -outcome <retry-role|retry-stage|split-work|cancel>\n" +
			"  [-m comment]")
	}

	_, svc, _, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	response, err := svc.InterveneTicket(context.Background(), ticketID, libticket.InterventionRequest{
		Outcome: *outcome,
		Message: *message,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(response)
	}
	fmt.Printf("intervention applied: %s on %s\n", response.Decision, response.Ticket.ID)
	if response.FollowUp != nil {
		fmt.Printf("follow-up created: %s\n", response.FollowUp.ID)
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

func runMerge(args []string) error {
	const usage = "tk merge <target-id> <source-id> [<source-id> ...] [-m comment]"

	fs := flag.NewFlagSet("merge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	message := fs.String("m", "", "comment to attach")
	if err := fs.Parse(args); err != nil {
		return err
	}

	refs := make([]string, 0, len(fs.Args()))
	for _, arg := range fs.Args() {
		if trimmed := strings.TrimSpace(arg); trimmed != "" {
			refs = append(refs, trimmed)
		}
	}
	if len(refs) < 2 {
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

	tickets := make([]store.Ticket, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		resolved := normalizeBareTicketRef(cfg, svc, ref)
		ticket, getErr := svc.GetTicket(context.Background(), resolved)
		if getErr != nil {
			return getErr
		}
		if _, exists := seen[ticket.ID]; exists {
			return fmt.Errorf("duplicate ticket %s", ticketLabel(ticket))
		}
		seen[ticket.ID] = struct{}{}
		tickets = append(tickets, ticket)
	}

	target := tickets[0]
	for _, ticket := range tickets {
		if ticket.ProjectID != target.ProjectID {
			return errors.New("all tickets must belong to the same project")
		}
		if !ticket.Draft {
			return fmt.Errorf("%s is not draft; only draft tickets can be merged", ticketLabel(ticket))
		}
		if ticket.Archived {
			return fmt.Errorf("%s is archived; only draft tickets can be merged", ticketLabel(ticket))
		}
		if ticket.Deleted {
			return fmt.Errorf("%s is deleted; only draft tickets can be merged", ticketLabel(ticket))
		}
	}

	updated, err := svc.UpdateTicket(context.Background(), target.ID, libticket.TicketUpdateRequest{
		Title:              target.Title,
		Description:        mergeTicketText(tickets, func(ticket store.Ticket) string { return ticket.Description }),
		AcceptanceCriteria: mergeTicketText(tickets, func(ticket store.Ticket) string { return ticket.AcceptanceCriteria }),
		DORMap:             target.DORMap,
		DODMap:             target.DODMap,
		ACMap:              target.ACMap,
		GitRepository:      target.GitRepository,
		GitBranch:          target.GitBranch,
		ParentID:           target.ParentID,
		Assignee:           target.Assignee,
		Priority:           target.Priority,
		Order:              target.Order,
		EstimateEffort:     target.EstimateEffort,
		EstimateComplete:   target.EstimateComplete,
		Message:            strings.TrimSpace(*message),
	})
	if err != nil {
		return err
	}

	archiveMessage := strings.TrimSpace(*message)
	if archiveMessage == "" {
		archiveMessage = fmt.Sprintf("merged into %s", ticketLabel(updated))
	}
	archived := make([]store.Ticket, 0, len(tickets)-1)
	for _, ticket := range tickets[1:] {
		archivedTicket, archiveErr := svc.ArchiveTicket(context.Background(), ticket.ID, archiveMessage)
		if archiveErr != nil {
			return archiveErr
		}
		archived = append(archived, archivedTicket)
	}

	if outputJSON {
		return printJSON(map[string]any{
			"merged_into": updated,
			"archived":    archived,
		})
	}

	printTicket(updated)
	archivedLabels := make([]string, 0, len(archived))
	for _, ticket := range archived {
		archivedLabels = append(archivedLabels, ticketLabel(ticket))
	}
	fmt.Printf("merged: %s\n", strings.Join(archivedLabels, ", "))
	return nil
}

func mergeTicketText(tickets []store.Ticket, selector func(store.Ticket) string) string {
	parts := make([]string, 0, len(tickets))
	for _, ticket := range tickets {
		if value := strings.TrimSpace(selector(ticket)); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "\n----\n")
}

func runDeleteTicket(args []string) error {
	usage := "tk rm|delete [-id <id[,id...]>|<id[,id...]>] [--confirm <token>]"
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	confirm := fs.String("confirm", "", "repeat the ticket id list shown by the first run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	refs, err := resolveDeleteTicketRefs(*id, fs.Args())
	if err != nil {
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
	tickets := make([]store.Ticket, 0, len(refs))
	for _, ref := range refs {
		resolved := normalizeBareTicketRef(cfg, svc, ref)
		ticket, getErr := svc.GetTicket(context.Background(), resolved)
		if getErr != nil {
			return getErr
		}
		tickets = append(tickets, ticket)
	}
	confirmTarget := joinDeleteConfirmTicketIDs(tickets)
	if strings.TrimSpace(*confirm) == "" {
		for _, ticket := range tickets {
			fmt.Printf("ticket   : %s — %s\n", ticket.ID, ticket.Title)
			fmt.Printf("type     : %s\n", ticket.Type)
		}
		fmt.Printf("\nThis will permanently delete the ticket and all associated data.\n")
		fmt.Printf("To confirm, run:\n\n")
		fmt.Printf("  tk rm -id %s --confirm %s\n\n", confirmTarget, confirmTarget)
		return nil
	}
	if strings.TrimSpace(*confirm) != confirmTarget {
		return fmt.Errorf("invalid confirmation value: expected %s", confirmTarget)
	}
	for _, ticket := range tickets {
		if err := svc.DeleteTicket(context.Background(), ticket.ID); err != nil {
			return err
		}
	}
	if outputJSON {
		ticketIDs := make([]string, 0, len(tickets))
		for _, ticket := range tickets {
			ticketIDs = append(ticketIDs, ticket.ID)
		}
		return printJSON(map[string]any{"status": "deleted", "ticket_ids": ticketIDs})
	}
	for _, ticket := range tickets {
		fmt.Printf("deleted ticket %s\n", ticketLabel(ticket))
	}
	return nil
}

func resolveDeleteTicketRefs(idFlag string, positional []string) ([]string, error) {
	var rawParts []string
	if strings.TrimSpace(idFlag) != "" {
		rawParts = append(rawParts, strings.Split(idFlag, ",")...)
	}
	for _, arg := range positional {
		rawParts = append(rawParts, strings.Split(arg, ",")...)
	}
	refs := make([]string, 0, len(rawParts))
	seen := map[string]bool{}
	for _, part := range rawParts {
		ref := strings.TrimSpace(part)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return nil, errors.New("missing ticket id")
	}
	return refs, nil
}

func joinDeleteConfirmTicketIDs(tickets []store.Ticket) string {
	ids := make([]string, 0, len(tickets))
	for _, ticket := range tickets {
		ids = append(ids, ticket.ID)
	}
	return strings.Join(ids, ",")
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
	Labels             []string
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
	filePath := fs.String("f", "", "create tickets from a file")
	commit := fs.Bool("commit", false, "apply file entries to storage")
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
	if parseErr := fs.Parse(normalizedArgs); parseErr != nil {
		return parseErr
	}
	inputFile := strings.TrimSpace(*filePath)
	title := strings.TrimSpace(*titleFlag)
	if title == "" {
		title = strings.Join(fs.Args(), " ")
	}
	if inputFile != "" {
		if title != "" {
			return errors.New("usage: tk add|create|new -f <filename> (title words are not allowed when -f is used)")
		}
		data, readErr := os.ReadFile(inputFile) // #nosec G304 -- user-specified file path for ticket creation
		if readErr != nil {
			return fmt.Errorf("cannot read %s: %w", inputFile, readErr)
		}
		entries, parseErr := parseTicketBatchFile(string(data), *taskType)
		if parseErr != nil {
			return fmt.Errorf("cannot parse %s: %w", inputFile, parseErr)
		}
		if !*commit {
			printTicketBatchIntent(entries, "new")
			return nil
		}
		cfg, svc, projectCtx, resolveErr := resolveCurrentProjectClient()
		if resolveErr != nil {
			return resolveErr
		}
		dorMap, mapErr := mergeGuidanceMap(nil, *dor, *dorMapRaw, containsFlag(normalizedArgs, "-dor"), containsFlag(normalizedArgs, "-dor-map"))
		if mapErr != nil {
			return mapErr
		}
		dodMap, mapErr := mergeGuidanceMap(nil, *dod, *dodMapRaw, containsFlag(normalizedArgs, "-dod"), containsFlag(normalizedArgs, "-dod-map"))
		if mapErr != nil {
			return mapErr
		}
		acMap, mapErr := mergeGuidanceMap(nil, *acceptanceCriteria, *acMapRaw, containsFlag(normalizedArgs, "-ac"), containsFlag(normalizedArgs, "-ac-map"))
		if mapErr != nil {
			return mapErr
		}
		for i, entry := range entries {
			var structuredParentID *string
			if entry.ParentIndex >= 0 {
				if entry.ParentIndex >= len(entries) {
					return fmt.Errorf("cannot parse %s: invalid parent reference for %q", inputFile, entry.Title)
				}
				parentID := strings.TrimSpace(entries[entry.ParentIndex].ID)
				if parentID == "" {
					return fmt.Errorf("cannot parse %s: parent id is missing for %q", inputFile, entry.Title)
				}
				structuredParentID = &parentID
			}
			if strings.TrimSpace(entry.ID) != "" {
				updated, updateErr := updateTicketFromBatchEntry(context.Background(), cfg, svc, entry, strings.TrimSpace(*message), structuredParentID)
				if updateErr != nil {
					return updateErr
				}
				entries[i].ID = updated.ID
				if outputJSON {
					if printErr := printJSON(updated); printErr != nil {
						return printErr
					}
					continue
				}
				fmt.Printf("%s\t%s\t%s\t%s\n", ticketLabel(updated), updated.Type, updated.Status, updated.Title)
				continue
			}
			opts := ticketCreateOptions{
				TicketType:         entry.TicketType,
				Title:              entry.Title,
				Description:        entry.Description,
				Labels:             entry.Labels,
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
			if structuredParentID != nil {
				opts.ParentID = structuredParentID
			} else if *parent != "" {
				opts.ParentID = parent
			}
			created, createErr := createTicketEntity(context.Background(), cfg, svc, projectCtx, opts)
			if createErr != nil {
				return createErr
			}
			entries[i].ID = created.ID
			if outputJSON {
				if printErr := printJSON(created); printErr != nil {
					return printErr
				}
				continue
			}
			if printCreatedID(created.ID, opts.PrintID) {
				continue
			}
			fmt.Println(ticketLabel(created))
		}
		if writeErr := os.WriteFile(inputFile, []byte(serializeTicketBatchFile(entries)), 0o600); writeErr != nil {
			return fmt.Errorf("cannot write %s: %w", inputFile, writeErr)
		}
		return nil
	}

	// Support @filename: read markdown file, parse key-value headers and body as description.
	if strings.HasPrefix(title, "@") {
		filePath := title[1:]
		data, readErr := os.ReadFile(filePath) // #nosec G304 -- user-specified file path for ticket creation
		if readErr != nil {
			return fmt.Errorf("cannot read %s: %w", filePath, readErr)
		}
		fileTitle, fileFields, fileBody := parseTicketFile(string(data))
		if fileTitle != "" && *titleFlag == "" {
			title = fileTitle
		}
		if v, ok := fileFields["type"]; ok && *taskType == "task" {
			*taskType = v
		}
		if v, ok := fileFields["priority"]; ok {
			if n, parseErr := strconv.Atoi(v); parseErr == nil {
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
		if v, ok := fileFields["project_id"]; ok && *project == "" {
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
		return errors.New("usage: tk add|create|new [title words | @filename]\n" +
			"  [-f filename] [-commit]\n" +
			"  [-title title] [-t type] [-p priority] [-a assignee]\n" +
			"  [-d description] [-dor text] [-dod text] [-ac text]\n" +
			"  [-dor-map stage=value,...] [-dod-map stage=value,...] [-ac-map stage=value,...]\n" +
			"  [-parent id] [-project project] [-project_id project]\n" +
			"  [-estimate_effort n] [-estimate_complete rfc3339]")
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
		"-f":                 true,
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
		"-project_id":        true,
		"-project":           true,
		"-m":                 true,
	}
	knownBoolFlags := map[string]bool{
		"-commit":  true,
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
	ticket, err := createTicketEntity(context.Background(), cfg, api, project, opts)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(ticket)
	}
	if printCreatedID(ticket.ID, opts.PrintID) {
		return nil
	}
	fmt.Println(ticketLabel(ticket))
	return nil
}

func createTicketEntity(ctx context.Context, cfg config.Config, api libticket.Service, project store.Project, opts ticketCreateOptions) (store.Ticket, error) {
	if strings.TrimSpace(opts.Project) != "" {
		resolvedProject, err := api.GetProject(ctx, opts.Project)
		if err != nil {
			return store.Ticket{}, err
		}
		project = resolvedProject
	}
	parentID := opts.ParentID
	ticket, err := api.CreateTicket(ctx, libticket.TicketCreateRequest{
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
		return store.Ticket{}, err
	}
	if err := applyTicketLabels(ctx, api, project.ID, ticket.ID, opts.Labels); err != nil {
		return store.Ticket{}, err
	}
	return ticket, nil
}

func applyTicketLabels(ctx context.Context, svc libticket.Service, projectID int64, ticketID string, labels []string) error {
	labelNames := normalizeLabelNames(labels)
	if len(labelNames) == 0 {
		return nil
	}
	existing, err := svc.ListLabels(ctx, projectID)
	if err != nil {
		return err
	}
	labelsByName := make(map[string]store.Label, len(existing))
	for _, label := range existing {
		key := strings.ToLower(strings.TrimSpace(label.Name))
		if key != "" {
			labelsByName[key] = label
		}
	}
	for _, name := range labelNames {
		key := strings.ToLower(name)
		label, ok := labelsByName[key]
		if !ok {
			created, createErr := svc.CreateLabel(ctx, projectID, libticket.LabelRequest{Name: name})
			if createErr != nil {
				return createErr
			}
			label = created
			labelsByName[key] = created
		}
		if err := svc.AddTicketLabel(ctx, ticketID, label.ID); err != nil {
			return err
		}
	}
	return nil
}

func syncTicketLabels(ctx context.Context, svc libticket.Service, projectID int64, ticketID string, labels []string) error {
	desiredNames := normalizeLabelNames(labels)
	desired := make(map[string]string, len(desiredNames))
	for _, name := range desiredNames {
		desired[strings.ToLower(name)] = name
	}
	projectLabels, err := svc.ListLabels(ctx, projectID)
	if err != nil {
		return err
	}
	labelByName := make(map[string]store.Label, len(projectLabels))
	for _, label := range projectLabels {
		key := strings.ToLower(strings.TrimSpace(label.Name))
		if key != "" {
			labelByName[key] = label
		}
	}
	for key, canonical := range desired {
		if _, ok := labelByName[key]; ok {
			continue
		}
		created, createErr := svc.CreateLabel(ctx, projectID, libticket.LabelRequest{Name: canonical})
		if createErr != nil {
			return createErr
		}
		labelByName[key] = created
	}
	currentLabels, err := svc.ListTicketLabels(ctx, ticketID)
	if err != nil {
		return err
	}
	current := make(map[string]store.Label, len(currentLabels))
	for _, label := range currentLabels {
		key := strings.ToLower(strings.TrimSpace(label.Name))
		if key != "" {
			current[key] = label
		}
	}
	for key, label := range current {
		if _, ok := desired[key]; ok {
			continue
		}
		if err := svc.RemoveTicketLabel(ctx, ticketID, label.ID); err != nil {
			return err
		}
	}
	for key := range desired {
		if _, ok := current[key]; ok {
			continue
		}
		if err := svc.AddTicketLabel(ctx, ticketID, labelByName[key].ID); err != nil {
			return err
		}
	}
	return nil
}

func normalizeLabelNames(labels []string) []string {
	if len(labels) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(labels))
	out := make([]string, 0, len(labels))
	for _, raw := range labels {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	return out
}

type ticketBatchEntry struct {
	ID           string
	Title        string
	Description  string
	TicketType   string
	TypeExplicit bool
	Labels       []string
	Level        int
	ParentIndex  int
}

func printTicketBatchIntent(entries []ticketBatchEntry, mode string) {
	for _, entry := range entries {
		idPrefix := "(new)"
		if strings.TrimSpace(entry.ID) != "" {
			idPrefix = strings.TrimSpace(entry.ID)
		}
		typeVal := strings.TrimSpace(entry.TicketType)
		if typeVal == "" {
			typeVal = "-"
		}
		statusVal := "design/idle"
		if mode == "update" || strings.TrimSpace(entry.ID) != "" {
			statusVal = "update/preview"
		}
		fmt.Printf("%s\t%s\t%s\t%s\n", idPrefix, typeVal, statusVal, entry.Title)
		descLines := strings.Split(strings.TrimSpace(entry.Description), "\n")
		preview := 2
		if len(descLines) < preview {
			preview = len(descLines)
		}
		for i := 0; i < preview; i++ {
			fmt.Printf("  %s\n", strings.TrimSpace(descLines[i]))
		}
		if len(entry.Labels) > 0 {
			fmt.Printf("  labels: %s\n", strings.Join(normalizeLabelNames(entry.Labels), ", "))
		}
	}
	fmt.Println("Tip: `use -commit` to write back to tk")
}

func serializeTicketBatchFile(entries []ticketBatchEntry) string {
	var b strings.Builder
	for i, entry := range entries {
		if i > 0 {
			b.WriteString("\n\n")
		}
		level := entry.Level
		if level <= 0 {
			level = 1
		}
		b.WriteString(strings.Repeat("#", level))
		b.WriteString(" ")
		b.WriteString(strings.TrimSpace(entry.Title))
		b.WriteString("\n")
		if strings.TrimSpace(entry.ID) != "" {
			b.WriteString("id: ")
			b.WriteString(strings.TrimSpace(entry.ID))
			b.WriteString("\n")
		}
		if entry.TypeExplicit || strings.TrimSpace(entry.ID) == "" {
			if strings.TrimSpace(entry.TicketType) != "" {
				b.WriteString("type: ")
				b.WriteString(strings.TrimSpace(entry.TicketType))
				b.WriteString("\n")
			}
		}
		labelNames := normalizeLabelNames(entry.Labels)
		if len(labelNames) > 0 {
			b.WriteString("labels: ")
			b.WriteString(strings.Join(labelNames, ", "))
			b.WriteString("\n")
		}
		if strings.TrimSpace(entry.Description) != "" {
			b.WriteString("\n")
			b.WriteString(strings.TrimSpace(entry.Description))
		}
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	return b.String()
}

func parseTicketBatchFile(content, defaultType string) ([]ticketBatchEntry, error) {
	lines := strings.Split(content, "\n")
	type workingEntry struct {
		id           string
		title        string
		ticketType   string
		typeExplicit bool
		labels       []string
		description  []string
		level        int
		parentIndex  int
	}
	var (
		entries []ticketBatchEntry
		current *workingEntry
		stack   = map[int]int{}
		inFence bool
		fenceCh byte
		fenceN  int
	)
	flushCurrent := func() {
		if current == nil {
			return
		}
		entries = append(entries, ticketBatchEntry{
			ID:           strings.TrimSpace(current.id),
			Title:        current.title,
			Description:  strings.TrimSpace(strings.Join(current.description, "\n")),
			TicketType:   current.ticketType,
			TypeExplicit: current.typeExplicit,
			Labels:       normalizeLabelNames(current.labels),
			Level:        current.level,
			ParentIndex:  current.parentIndex,
		})
		idx := len(entries) - 1
		stack[current.level] = idx
		for level := range stack {
			if level > current.level {
				delete(stack, level)
			}
		}
	}
	for i, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")
		trimmed := strings.TrimSpace(line)
		if current != nil {
			if inFence {
				if ch, n, rest, ok := parseFenceMarker(trimmed); ok && ch == fenceCh && n >= fenceN && rest == "" {
					inFence = false
					fenceCh = 0
					fenceN = 0
				}
				current.description = append(current.description, line)
				continue
			}
			if ch, n, _, ok := parseFenceMarker(trimmed); ok {
				inFence = true
				fenceCh = ch
				fenceN = n
				current.description = append(current.description, line)
				continue
			}
		}
		if strings.HasPrefix(trimmed, "#") && !inFence {
			headingLevel := 0
			for headingLevel < len(trimmed) && trimmed[headingLevel] == '#' {
				headingLevel++
			}
			if headingLevel == 0 {
				return nil, fmt.Errorf("line %d: invalid heading", i+1)
			}
			title := strings.TrimSpace(trimmed[headingLevel:])
			if title == "" {
				return nil, fmt.Errorf("line %d: ticket heading is missing a title", i+1)
			}
			flushCurrent()
			parentIndex := -1
			if headingLevel > 1 {
				parent, ok := stack[headingLevel-1]
				if !ok {
					return nil, fmt.Errorf("line %d: heading level %d has no parent level %d", i+1, headingLevel, headingLevel-1)
				}
				parentIndex = parent
			}
			current = &workingEntry{
				title:       title,
				ticketType:  strings.TrimSpace(defaultType),
				level:       headingLevel,
				parentIndex: parentIndex,
			}
			continue
		}
		if current == nil {
			if trimmed == "" {
				continue
			}
			return nil, fmt.Errorf("line %d: expected a ticket heading starting with '#'", i+1)
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "label:") || strings.HasPrefix(lower, "labels:"):
			idx := strings.Index(trimmed, ":")
			labelNames := splitCSV(trimmed[idx+1:])
			if len(labelNames) == 0 {
				return nil, fmt.Errorf("line %d: label directive requires at least one label", i+1)
			}
			current.labels = append(current.labels, labelNames...)
		case strings.HasPrefix(lower, "type:"):
			idx := strings.Index(trimmed, ":")
			ticketType := strings.TrimSpace(trimmed[idx+1:])
			if ticketType == "" {
				return nil, fmt.Errorf("line %d: type directive requires a value", i+1)
			}
			current.ticketType = ticketType
			current.typeExplicit = true
		case strings.HasPrefix(lower, "id:"):
			idx := strings.Index(trimmed, ":")
			ticketID := strings.TrimSpace(trimmed[idx+1:])
			if ticketID == "" {
				return nil, fmt.Errorf("line %d: id directive requires a value", i+1)
			}
			current.id = ticketID
		default:
			current.description = append(current.description, line)
		}
	}
	flushCurrent()
	if len(entries) == 0 {
		return nil, errors.New("no tickets found: add headings like '# Title'")
	}
	return entries, nil
}

func parseFenceMarker(trimmed string) (ch byte, n int, rest string, ok bool) {
	if trimmed == "" {
		return 0, 0, "", false
	}
	ch = trimmed[0]
	if ch != '`' && ch != '~' {
		return 0, 0, "", false
	}
	n = 0
	for n < len(trimmed) && trimmed[n] == ch {
		n++
	}
	if n < 3 {
		return 0, 0, "", false
	}
	return ch, n, strings.TrimSpace(trimmed[n:]), true
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
