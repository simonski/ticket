package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"
	ansiGreen = "\033[32m"
	ansiRed   = "\033[31m"
	ansiGray  = "\033[90m"
	ansiWhite = "\033[97m"
)

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd())) // #nosec G115 -- uintptr→int is safe for terminal file descriptors on all supported platforms
}

// rowColor returns an ANSI color code based on the ticket state embedded in the status string.
// active → green, fail → red, idle → white (normal), success → gray (dimmed).
func rowColor(status string) string {
	_, state, err := store.ParseLifecycleStatus(status)
	if err != nil {
		return ""
	}
	switch state {
	case store.StateActive:
		return ansiGreen
	case store.StateFail:
		return ansiRed
	case store.StateIdle:
		return ansiWhite
	case store.StateSuccess:
		return ansiGray
	}
	return ""
}

func printProject(project store.Project) {
	if outputJSON {
		if err := printJSON(project); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not print project JSON: %v\n", err)
		}
		return
	}
	fmt.Printf("project: %s\n", project.Title)
	fmt.Printf("project_id: %d\n", project.ID)
	fmt.Printf("prefix: %s\n", project.Prefix)
	fmt.Printf("status: %s\n", project.Status)
	if project.Description != "" {
		fmt.Printf("wow: %s\n", project.Description)
		fmt.Printf("description: %s\n", project.Description)
	}
	if project.AcceptanceCriteria != "" {
		fmt.Printf("ac: %s\n", project.AcceptanceCriteria)
		fmt.Printf("acceptance_criteria: %s\n", project.AcceptanceCriteria)
	}
	printGuidanceMap("dor_map", project.DORMap)
	printGuidanceMap("dod_map", project.DODMap)
	printGuidanceMap("ac_map", project.ACMap)
	if project.Notes != "" {
		fmt.Printf("dod: %s\n", project.Notes)
		fmt.Printf("notes: %s\n", project.Notes)
	}
	if project.GitRepository != "" {
		fmt.Printf("git_repository: %s\n", project.GitRepository)
	}
	if project.WorkflowID != nil {
		fmt.Printf("workflow_id: %d\n", *project.WorkflowID)
	}
}

func printProjectTable(projects []store.Project, currentProjectID string, workflowNames map[int64]string) {
	if len(projects) == 0 {
		printNoEntitiesAvailable("projects")
		return
	}
	currentID := strings.TrimSpace(currentProjectID)
	rows := make([]string, 0, len(projects))
	for _, project := range projects {
		marker := " "
		if strconv.FormatInt(project.ID, 10) == currentID || strings.EqualFold(project.Prefix, currentID) {
			marker = "*"
		}
		desc := project.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		workflow := ""
		if project.WorkflowID != nil {
			if name, ok := workflowNames[*project.WorkflowID]; ok {
				workflow = name
			}
		}
		rows = append(rows, fmt.Sprintf("%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s", marker, project.ID, project.Prefix, project.Title, project.Status, workflow, project.GitRepository, desc))
	}
	printBoxTable(" \tID\tPREFIX\tTITLE\tSTATUS\tWorkflow\tGIT\tDESCRIPTION", rows)
}

func printProjectAccessRequestTable(requests []store.ProjectAccessRequest) {
	if len(requests) == 0 {
		printNoEntitiesAvailable("project access requests")
		return
	}
	rows := make([]string, 0, len(requests))
	for _, request := range requests {
		message := request.Message
		if strings.TrimSpace(request.DecisionMessage) != "" {
			if strings.TrimSpace(message) != "" {
				message += " | decision: " + request.DecisionMessage
			} else {
				message = "decision: " + request.DecisionMessage
			}
		}
		if len(message) > 60 {
			message = message[:57] + "..."
		}
		project := strings.TrimSpace(request.ProjectPrefix)
		if project == "" {
			project = strconv.FormatInt(request.ProjectID, 10)
		}
		if title := strings.TrimSpace(request.ProjectTitle); title != "" {
			project += " (" + title + ")"
		}
		rows = append(rows, fmt.Sprintf("%d\t%s\t%s\t%s\t%s\t%s\t%s", request.ID, project, request.Username, request.UserID, request.Status, request.CreatedAt, message))
	}
	printBoxTable("REQUEST_ID\tPROJECT\tUSERNAME\tUSER_ID\tSTATUS\tCREATED\tMESSAGE", rows)
}

func printUserNotificationTable(notifications []store.UserNotification) {
	if len(notifications) == 0 {
		printNoEntitiesAvailable("notifications")
		return
	}
	rows := make([]string, 0, len(notifications))
	for _, notification := range notifications {
		message := notification.Message
		if len(message) > 72 {
			message = message[:69] + "..."
		}
		rows = append(rows, fmt.Sprintf("%d\t%s\t%s\t%s\t%s\t%s", notification.ID, notification.Status, notification.Kind, notification.Title, notification.CreatedAt, message))
	}
	printBoxTable("NOTIFICATION_ID\tSTATUS\tKIND\tTITLE\tCREATED\tMESSAGE", rows)
}

func ticketLabel(ticket store.Ticket) string {
	return ticket.ID
}

func printTicket(ticket store.Ticket) {
	if outputJSON {
		if err := printJSON(ticket); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not print ticket JSON: %v\n", err)
		}
		return
	}
	fmt.Printf("ticket: %s\n", ticket.Title)
	fmt.Printf("key: %s\n", ticket.ID)
	fmt.Printf("type: %s\n", ticket.Type)
	fmt.Printf("status: %s\n", ticket.Status)
	fmt.Printf("draft: %t\n", ticket.Draft)
	fmt.Printf("complete: %s\n", ticketCompleteLabel(ticket))
	fmt.Printf("archived: %t\n", ticket.Archived)
	fmt.Printf("deleted: %t\n", ticket.Deleted)
	if ticket.Author != "" {
		fmt.Printf("author: %s\n", ticket.Author)
	}
	if ticket.Description != "" {
		fmt.Printf("description: %s\n", ticket.Description)
	}
	if ticket.AcceptanceCriteria != "" {
		fmt.Printf("acceptance_criteria: %s\n", ticket.AcceptanceCriteria)
	}
	printGuidanceMap("dor_map", ticket.DORMap)
	printGuidanceMap("dod_map", ticket.DODMap)
	printGuidanceMap("ac_map", ticket.ACMap)
	if ticket.GitRepository != "" {
		fmt.Printf("git_repository: %s\n", ticket.GitRepository)
	}
	if ticket.GitBranch != "" {
		fmt.Printf("git_branch: %s\n", ticket.GitBranch)
	}
	if ticket.Author != "" {
		fmt.Printf("author: %s\n", ticket.Author)
	}
	if ticket.EstimateEffort != 0 {
		fmt.Printf("estimate_effort: %d\n", ticket.EstimateEffort)
	}
	if ticket.EstimateComplete != "" {
		fmt.Printf("estimate_complete: %s\n", ticket.EstimateComplete)
	}
}

func printGuidanceMap(label string, m store.GuidanceMap) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("%s[%s]: %s\n", label, key, m[key])
	}
}

func guidanceDetailLabels(prefix string, m store.GuidanceMap) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	labels := make([]string, 0, len(keys))
	for _, key := range keys {
		labels = append(labels, fmt.Sprintf("%s[%s]", prefix, key))
	}
	return labels
}

func printAlignedGuidanceMap(width int, prefix string, m store.GuidanceMap) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("%-*s : %s\n", width, fmt.Sprintf("%s[%s]", prefix, key), m[key])
	}
}

func printRequestContext(resp libticket.TicketRequestResponse) {
	if resp.Ticket != nil {
		printTicket(*resp.Ticket)
	}
	if resp.Project != nil {
		fmt.Println()
		fmt.Printf("project: %s — %s\n", resp.Project.Prefix, resp.Project.Title)
		if resp.Project.GitRepository != "" {
			fmt.Printf("  git: %s\n", resp.Project.GitRepository)
		}
	}
	if len(resp.Parents) > 0 {
		fmt.Println()
		fmt.Println("parents:")
		for _, p := range resp.Parents {
			fmt.Printf("  %s [%s] %s\n", p.ID, p.Type, p.Title)
		}
	}
	if resp.Workflow != nil {
		fmt.Println()
		fmt.Printf("workflow: %s\n", resp.Workflow.Name)
		for _, stage := range resp.Workflow.Stages {
			marker := "  "
			if resp.Ticket != nil && resp.Ticket.WorkflowStageID != nil && stage.ID == *resp.Ticket.WorkflowStageID {
				marker = "> "
			}
			role := ""
			if len(stage.Roles) > 0 {
				var names []string
				for _, r := range stage.Roles {
					names = append(names, r.Title)
				}
				role = fmt.Sprintf(" (roles: %s)", strings.Join(names, ", "))
			}
			fmt.Printf("  %s%s%s\n", marker, stage.StageName, role)
		}
	}
	if resp.Role != nil {
		fmt.Println()
		fmt.Printf("current role: %s\n", resp.Role.Title)
		if resp.Role.Description != "" {
			fmt.Printf("  description: %s\n", resp.Role.Description)
		}
		if resp.Role.AcceptanceCriteria != "" {
			fmt.Printf("  acceptance criteria: %s\n", resp.Role.AcceptanceCriteria)
		}
	}
}

func printTicketDetails(ticket store.Ticket, dependencies []store.Dependency, history []store.HistoryEvent, workflowStages []store.WorkflowStage, labels []store.Label, totalMinutes int, parentKey, cloneKey string, childTotal, childOpen, childClosed int) {
	dependsOn := formatDependsOn(dependencies)

	type ticketField struct {
		label string
		value string
	}
	fields := []ticketField{
		{label: "Key", value: ticket.ID},
		{label: "Type", value: ticket.Type},
		{label: "Description", value: ticket.Description},
	}
	if parentKey != "" {
		fields = append(fields, ticketField{label: "Parent", value: parentKey})
	}
	if cloneKey != "" {
		fields = append(fields, ticketField{label: "CloneOf", value: cloneKey})
	}
	fields = append(fields,
		ticketField{label: "Title", value: ticket.Title},
		ticketField{label: "Author", value: ticket.Author},
		ticketField{label: "Assignee", value: ticket.Assignee},
		ticketField{label: "Order", value: fmt.Sprintf("%d", ticket.Order)},
		ticketField{label: "EstimateEffort", value: fmt.Sprintf("%d", ticket.EstimateEffort)},
		ticketField{label: "EstimateComplete", value: ticket.EstimateComplete},
		ticketField{label: "DependsOn", value: dependsOn},
		ticketField{label: "Status", value: ticket.Status},
		ticketField{label: "Stage", value: ticket.Stage},
		ticketField{label: "State", value: ticket.State},
	)
	if len(workflowStages) > 0 {
		fields = append(fields, ticketField{label: "Workflow", value: renderWorkflowProgress(ticket.Stage, workflowStages)})
	}
	fields = append(fields,
		ticketField{label: "Draft", value: fmt.Sprintf("%t", ticket.Draft)},
		ticketField{label: "Complete", value: ticketCompleteLabel(ticket)},
		ticketField{label: "Archived", value: fmt.Sprintf("%t", ticket.Archived)},
		ticketField{label: "Deleted", value: fmt.Sprintf("%t", ticket.Deleted)},
		ticketField{label: "Priority", value: fmt.Sprintf("%d", ticket.Priority)},
		ticketField{label: "Created", value: ticket.CreatedAt},
		ticketField{label: "LastModified", value: ticket.UpdatedAt},
		ticketField{label: "Acceptance Criteria", value: ticket.AcceptanceCriteria},
	)
	maxLabelWidth := 0
	for _, field := range fields {
		if len(field.label) > maxLabelWidth {
			maxLabelWidth = len(field.label)
		}
	}
	for _, label := range guidanceDetailLabels("dor_map", ticket.DORMap) {
		if len(label) > maxLabelWidth {
			maxLabelWidth = len(label)
		}
	}
	for _, label := range guidanceDetailLabels("dod_map", ticket.DODMap) {
		if len(label) > maxLabelWidth {
			maxLabelWidth = len(label)
		}
	}
	for _, label := range guidanceDetailLabels("ac_map", ticket.ACMap) {
		if len(label) > maxLabelWidth {
			maxLabelWidth = len(label)
		}
	}
	for _, field := range fields {
		fmt.Printf("%-*s : %s\n", maxLabelWidth, field.label, field.value)
	}
	printAlignedGuidanceMap(maxLabelWidth, "dor_map", ticket.DORMap)
	printAlignedGuidanceMap(maxLabelWidth, "dod_map", ticket.DODMap)
	printAlignedGuidanceMap(maxLabelWidth, "ac_map", ticket.ACMap)
	if len(ticket.Comments) > 0 {
		fmt.Printf("%-*s :\n", maxLabelWidth, "Comments")
		for _, comment := range ticket.Comments {
			fmt.Printf("  - [%s] %s: %s\n", comment.CreatedAt, comment.Author, comment.Text)
		}
	}
	if len(labels) > 0 {
		var labelNames []string
		for _, l := range labels {
			labelNames = append(labelNames, l.Name)
		}
		fmt.Printf("%-*s : %s\n", maxLabelWidth, "Labels", strings.Join(labelNames, ", "))
	}
	if totalMinutes > 0 {
		hours := totalMinutes / 60
		mins := totalMinutes % 60
		if hours > 0 {
			fmt.Printf("%-*s : %dh %dm\n", maxLabelWidth, "TimeLogged", hours, mins)
		} else {
			fmt.Printf("%-*s : %dm\n", maxLabelWidth, "TimeLogged", mins)
		}
	}
	if len(history) > 0 {
		fmt.Printf("%-*s :\n", maxLabelWidth, "History")
		for _, event := range history {
			fmt.Printf("  - [%s] %s\n", event.CreatedAt, formatHistoryEvent(event))
		}
	}
	fmt.Printf("%-*s : total=%d open=%d closed=%d\n", maxLabelWidth, "ChildCounts", childTotal, childOpen, childClosed)
}

func printTicketSummary(ticket store.Ticket) {
	fields := []struct {
		label string
		value string
	}{
		{label: "id/type", value: fmt.Sprintf("%s/%s", ticket.ID, ticket.Type)},
		{label: "title", value: ticket.Title},
		{label: "description", value: ticket.Description},
		{label: "a/c", value: ticket.AcceptanceCriteria},
	}
	maxLabelWidth := 0
	for _, field := range fields {
		if len(field.label) > maxLabelWidth {
			maxLabelWidth = len(field.label)
		}
	}
	for _, field := range fields {
		printSummaryField(maxLabelWidth, field.label, field.value)
	}
	fmt.Println()
	fmt.Println("(use `tk get XXX -v` for more information)")
}

func printSummaryField(width int, label, rawValue string) {
	value := strings.ReplaceAll(strings.ReplaceAll(rawValue, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(value, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	fmt.Printf("%-*s : %s\n", width, label, lines[0])
	if len(lines) == 1 {
		return
	}
	indent := strings.Repeat(" ", width+3)
	for _, line := range lines[1:] {
		fmt.Printf("%s%s\n", indent, line)
	}
}

func compactTicketTitle(rawTitle string) string {
	title := strings.ReplaceAll(strings.ReplaceAll(rawTitle, "\r\n", "\n"), "\r", "\n")
	first, rest, found := strings.Cut(title, "\n")
	if !found {
		return title
	}
	first = strings.TrimSpace(first)
	if first == "" {
		if strings.TrimSpace(rest) == "" {
			return ""
		}
		return "..."
	}
	return first + "..."
}

func printTicketChildren(children []store.Ticket) {
	fmt.Println("Children     :")
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	rowColors := make([]string, 0, len(children))
	for _, c := range children {
		symbol := formatTicketStatusSymbol(c.Status, true)
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", symbol, c.ID, c.Type, c.Status, compactTicketTitle(c.Title))
		rowColors = append(rowColors, childTicketColor(c))
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not flush child ticket table: %v\n", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	useColor := isTerminal() && !noColorOutput
	for i, line := range lines {
		if useColor && i < len(rowColors) && rowColors[i] != "" {
			fmt.Printf("%s%s%s\n", rowColors[i], line, ansiReset)
			continue
		}
		fmt.Println(line)
	}
}

func childTicketColor(ticket store.Ticket) string {
	if !ticketIsOpenForList(ticket) {
		return ansiDim + ansiGray
	}

	state := strings.TrimSpace(strings.ToLower(ticket.State))
	if state == "" {
		_, parsedState, err := store.ParseLifecycleStatus(ticket.Status)
		if err == nil {
			state = parsedState
		}
	}

	switch state {
	case store.StateActive:
		return ansiWhite
	case store.StateIdle:
		return ansiGray
	case store.StateFail:
		return ansiRed
	default:
		return ansiGray
	}
}

func renderWorkflowProgress(currentStage string, stages []store.WorkflowStage) string {
	var parts []string
	for _, s := range stages {
		if s.StageName == currentStage {
			if noColorOutput {
				parts = append(parts, "["+s.StageName+"]")
			} else {
				parts = append(parts, "\x1b[1;32m"+s.StageName+"\x1b[0m")
			}
		} else {
			parts = append(parts, s.StageName)
		}
	}
	return strings.Join(parts, " → ")
}

func formatDependsOn(dependencies []store.Dependency) string {
	var ids []string
	for _, dependency := range dependencies {
		ids = append(ids, dependency.DependsOn)
	}
	if len(ids) == 0 {
		return "[]"
	}
	return "[" + strings.Join(ids, ",") + "]"
}

func printCountSummary(summary store.CountSummary, scopedToProject bool) {
	var lines []statusLine
	lines = append(lines, statusLine{key: "users", value: fmt.Sprintf("%d", summary.Users)})
	if !scopedToProject {
		lines = append(lines, statusLine{key: "projects", value: fmt.Sprintf("%d", summary.Projects)})
	}
	if len(summary.Types) > 0 {
		lines = append(lines, statusLine{}) // blank separator
	}
	for _, item := range summary.Types {
		val := fmt.Sprintf("%d", item.Total)
		if suffix := formatStatusCounts(item.Statuses); suffix != "" {
			val += "  (" + suffix + ")"
		}
		lines = append(lines, statusLine{key: item.Type + "s", value: val})
	}
	printStatusBox(lines)
}

// buildTreeDisplay reorders tickets for tree display so that parent tickets appear
// before their children, and returns a tree-connector prefix string for each ticket ID.
// Tickets whose parent is not in the list are treated as roots (empty prefix).
func buildTreeDisplay(tickets []store.Ticket) (ordered []store.Ticket, treePrefix map[string]string) {
	inList := make(map[string]bool, len(tickets))
	for _, t := range tickets {
		inList[t.ID] = true
	}

	children := make(map[string][]store.Ticket, len(tickets))
	var roots []store.Ticket
	for _, t := range tickets {
		if t.ParentID != nil && inList[*t.ParentID] {
			children[*t.ParentID] = append(children[*t.ParentID], t)
		} else {
			roots = append(roots, t)
		}
	}

	ordered = make([]store.Ticket, 0, len(tickets))
	treePrefix = make(map[string]string, len(tickets))

	// visit processes t and its subtree.
	// ancestorBars is the accumulated bar/space prefix from ancestor nodes.
	// isRoot indicates no connector is rendered for this node.
	// isLast indicates this is the last child among its siblings.
	var visit func(t store.Ticket, ancestorBars string, isRoot, isLast bool)
	visit = func(t store.Ticket, ancestorBars string, isRoot, isLast bool) {
		switch {
		case isRoot:
			treePrefix[t.ID] = ""
		case isLast:
			treePrefix[t.ID] = ancestorBars + "└─ "
		default:
			treePrefix[t.ID] = ancestorBars + "├─ "
		}
		ordered = append(ordered, t)

		// Compute the ancestor bar prefix that children of t will inherit.
		// Root nodes contribute no bar; non-last nodes contribute "│  "; last nodes contribute "   ".
		var childAncestorBars string
		switch {
		case isRoot:
			childAncestorBars = ancestorBars
		case isLast:
			childAncestorBars = ancestorBars + "   "
		default:
			childAncestorBars = ancestorBars + "│  "
		}
		kids := children[t.ID]
		for i, kid := range kids {
			visit(kid, childAncestorBars, false, i == len(kids)-1)
		}
	}

	for _, root := range roots {
		visit(root, "", true, false)
	}
	return ordered, treePrefix
}

func printTicketTable(tickets []store.Ticket, parentKeys map[string]string, agentUsernames map[string]bool, statusUnicode, includeArchived bool) {
	if len(tickets) == 0 {
		fmt.Println("no tickets")
		return
	}

	// Reorder tickets into tree (parent-before-children) order and get per-ticket prefixes.
	tickets, treePfx := buildTreeDisplay(tickets)

	useColor := isTerminal()

	// Get terminal width; default to 120 for non-terminal output.
	termW := 120
	if useColor {
		if tw, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && tw > 0 { // #nosec G115
			termW = tw
		}
	}

	showOpen := includeArchived // only show OPEN column when mixed open/closed

	makeHeader := func() string {
		if showOpen {
			return "ID\tTYPE\tTITLE\tSTAGE\tSTATE\tDRAFT\tCOMPLETE\tASSIGNEE\tPRIORITY"
		}
		return "ID\tTYPE\tTITLE\tSTAGE\tSTATE\tDRAFT\tASSIGNEE\tPRIORITY"
	}

	// Maximum display width for the assignee column.
	const maxAssigneeW = 16

	// truncateRunes truncates s to at most n runes, appending "…" if truncated.
	truncateRunes := func(s string, n int) string {
		r := []rune(s)
		if len(r) <= n {
			return s
		}
		if n <= 1 {
			return "…"
		}
		return string(r[:n-1]) + "…"
	}

	makeDataRow := func(t store.Ticket, title string) string {
		symbol := formatTicketStatusSymbol(t.Status, statusUnicode)
		assignee := strings.TrimSpace(t.Assignee)
		if assignee == "" {
			assignee = "-"
		} else if agentUsernames[assignee] {
			assignee = "agent-" + assignee
		}
		assignee = truncateRunes(assignee, maxAssigneeW)
		key := treePfx[t.ID] + symbol + " " + t.ID
		draft := "no"
		if t.Draft {
			draft = "yes"
		}
		if showOpen {
			return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d",
				key, t.Type, title, t.Stage, t.State, draft, ticketCompleteLabel(t), assignee, t.Priority)
		}
		return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d",
			key, t.Type, title, t.Stage, t.State, draft, assignee, t.Priority)
	}

	// Pass 1: render with a sentinel title to locate where the title column
	// starts. We measure from the HEADER row (all ASCII) rather than a data
	// row, because data rows contain multi-byte Unicode symbols (○/◑/◉ = 3
	// bytes, 1 visual column) that cause tabwriter's byte-based alignment to
	// diverge from the actual visual width — leading to overdraw.
	const titleSentinel = "\x01"
	const titleHeaderLen = 5 // len("TITLE")
	var mBuf bytes.Buffer
	mw := tabwriter.NewWriter(&mBuf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(mw, makeHeader())
	displayTitles := make([]string, len(tickets))
	for i, t := range tickets {
		displayTitles[i] = compactTicketTitle(t.Title)
		fmt.Fprintln(mw, makeDataRow(t, titleSentinel))
	}
	if err := mw.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not flush ticket width probe: %v\n", err)
	}

	mLines := strings.Split(strings.TrimRight(mBuf.String(), "\n"), "\n")

	titleW := 40 // fallback
	preWidth := 0
	if len(mLines) >= 1 {
		// The header is pure ASCII: byte index == visual column index.
		headerLine := mLines[0]
		if idx := strings.Index(headerLine, "TITLE"); idx >= 0 {
			preWidth = idx // ASCII header: bytes == visual columns
			// Title column in pass 1 = titleHeaderLen + 2 padding = 7 chars.
			postWidth := len(headerLine) - idx - (titleHeaderLen + 2)
			if postWidth < 0 {
				postWidth = 0
			}
			// Inside the box: "│ " (2) + preWidth + titleW + 2(pad) + postWidth + " │" (2) = termW
			if computed := termW - 4 - preWidth - 2 - postWidth; computed >= 10 {
				titleW = computed
			}
		}
	}

	runeCount := utf8.RuneCountInString

	// padToWidth pads or truncates s to exactly n runes.
	padToWidth := func(s string, n int) string {
		r := []rune(s)
		if len(r) >= n {
			return string(r[:n])
		}
		return s + strings.Repeat(" ", n-len(r))
	}

	// wrapRunes splits s into lines of at most w runes, breaking at word
	// boundaries so words are never cut in half.
	wrapRunes := func(s string, w int) []string {
		words := strings.Fields(s)
		if len(words) == 0 {
			return []string{""}
		}
		var out []string
		var line []rune
		for _, word := range words {
			wr := []rune(word)
			if len(line) > 0 && len(line)+1+len(wr) > w {
				out = append(out, string(line))
				line = nil
			}
			if len(line) > 0 {
				line = append(line, ' ')
			}
			line = append(line, wr...)
		}
		if len(line) > 0 {
			out = append(out, string(line))
		}
		return out
	}

	// Pass 2: render with titles padded to exactly titleW runes so the tab
	// writer fixes the TITLE column at titleW (plus its 2-space padding).
	var buf bytes.Buffer
	bw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(bw, makeHeader())
	for i, t := range tickets {
		fmt.Fprintln(bw, makeDataRow(t, padToWidth(displayTitles[i], titleW)))
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not flush ticket table: %v\n", err)
	}

	rawLines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	// Build the final list of display lines, inserting title continuation lines
	// immediately after each ticket row that has a title longer than titleW,
	// and blank separator lines before each new root-level group.
	type displayLine struct {
		text    string
		status  string // non-empty on ticket rows, enables column coloring
		draft   bool   // ticket draft flag, for coloring the DRAFT column
		isBlank bool   // blank separator row between root groups
	}

	display := make([]displayLine, 0, len(rawLines))
	display = append(display, displayLine{text: rawLines[0]}) // header

	// treeContPrefix converts a connector prefix to its bar-continuation form,
	// so that multi-line title rows keep the vertical bar aligned with the connector.
	//   "├─ "      → "│  "    (bar continues down)
	//   "└─ "      → "   "    (last child, no bar)
	//   "│  ├─ "   → "│  │  "
	//   "│  └─ "   → "│     "
	treeContPrefix := func(pfx string) string {
		r := []rune(pfx)
		if len(r) < 3 {
			return strings.Repeat(" ", len(r))
		}
		last3 := string(r[len(r)-3:])
		var repl string
		switch last3 {
		case "├─ ":
			repl = "│  "
		case "└─ ":
			repl = "   "
		default:
			repl = strings.Repeat(" ", 3)
		}
		return string(r[:len(r)-3]) + repl
	}

	for i, t := range tickets {
		if i+1 >= len(rawLines) {
			break
		}
		display = append(display, displayLine{text: rawLines[i+1], status: t.Status, draft: t.Draft})
		chunks := wrapRunes(displayTitles[i], titleW)
		for _, chunk := range chunks[1:] {
			// Build continuation indent: tree bar continuation + spaces to title column.
			contPfx := treeContPrefix(treePfx[t.ID])
			contPfxW := len([]rune(contPfx))
			indent := contPfx + strings.Repeat(" ", preWidth-contPfxW)
			cont := indent + chunk
			display = append(display, displayLine{text: cont, status: t.Status, draft: t.Draft})
		}
	}

	// Compute max visible width across all display lines, clamped to terminal.
	maxW := 0
	for _, l := range display {
		if n := runeCount(l.text); n > maxW {
			maxW = n
		}
	}
	// Clamp to terminal width minus box borders ("│ " + " │" = 4 chars).
	if maxW > termW-4 {
		maxW = termW - 4
	}
	if maxW < 10 {
		maxW = 10
	}

	if !useColor {
		for _, l := range display {
			if l.isBlank {
				fmt.Println()
				continue
			}
			r := []rune(l.text)
			if len(r) > maxW {
				fmt.Println(string(r[:maxW]))
			} else {
				fmt.Println(l.text)
			}
		}
		return
	}

	// Locate STAGE, STATE, DRAFT column positions from the header line.
	header := display[0].text
	type colSpan struct{ start, end int }
	findCol := func(name string) colSpan {
		idx := strings.Index(header, name)
		if idx < 0 {
			return colSpan{-1, -1}
		}
		// Column extends from idx to the start of next column (or end of line).
		// Find next column by looking for the next non-space after trailing spaces.
		end := idx + len(name)
		for end < len(header) && header[end] == ' ' {
			end++
		}
		return colSpan{idx, end}
	}
	stageCol := findCol("STAGE")
	stateCol := findCol("STATE")
	draftCol := findCol("DRAFT")

	// colorizeColumns applies ANSI color to specific column ranges in a line,
	// leaving the rest of the text uncolored (white).
	colorizeColumns := func(line string, status string, draft bool) string {
		runes := []rune(line)
		lineLen := len(runes)
		stateColor := rowColor(status)
		draftColor := ansiGray
		if !draft {
			draftColor = ansiGreen
		}

		type span struct {
			start, end int
			color      string
		}
		spans := []span{}
		if stageCol.start >= 0 && stateColor != "" {
			spans = append(spans, span{stageCol.start, stageCol.end, stateColor})
		}
		if stateCol.start >= 0 && stateColor != "" {
			spans = append(spans, span{stateCol.start, stateCol.end, stateColor})
		}
		if draftCol.start >= 0 {
			spans = append(spans, span{draftCol.start, draftCol.end, draftColor})
		}
		if len(spans) == 0 {
			return line
		}

		var b strings.Builder
		pos := 0
		for _, s := range spans {
			start := s.start
			end := s.end
			if start >= lineLen {
				continue
			}
			if end > lineLen {
				end = lineLen
			}
			if pos < start {
				b.WriteString(string(runes[pos:start]))
			}
			b.WriteString(s.color)
			b.WriteString(string(runes[start:end]))
			b.WriteString(ansiReset)
			pos = end
		}
		if pos < lineLen {
			b.WriteString(string(runes[pos:]))
		}
		return b.String()
	}

	// Render inside a rounded Unicode box with per-column coloring.
	border := strings.Repeat("─", maxW+2)
	fmt.Println("╭" + border + "╮")
	for _, l := range display {
		if l.isBlank {
			fmt.Printf("│ %s │\n", strings.Repeat(" ", maxW))
			continue
		}
		lineText := l.text
		lineRunes := []rune(lineText)
		// Truncate lines that exceed maxW to prevent overdraw.
		if len(lineRunes) > maxW {
			lineText = string(lineRunes[:maxW])
		}
		pad := strings.Repeat(" ", maxW-runeCount(lineText))
		text := lineText
		if l.status != "" {
			text = colorizeColumns(lineText, l.status, l.draft)
		}
		fmt.Printf("│ %s%s │\n", text, pad)
	}
	fmt.Println("╰" + border + "╯")
}

// printBoxTable renders tabwriter-formatted lines inside a rounded Unicode box.
// If the terminal is not a TTY, plain text is printed instead.
func printBoxTable(header string, rows []string) {
	lines := make([]string, 0, 1+len(rows))
	lines = append(lines, header)
	lines = append(lines, rows...)

	// Render via tabwriter to align columns.
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not flush table: %v\n", err)
	}
	formatted := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	if !isTerminal() {
		for _, l := range formatted {
			fmt.Println(l)
		}
		return
	}

	maxW := 0
	for _, l := range formatted {
		if n := utf8.RuneCountInString(l); n > maxW {
			maxW = n
		}
	}
	border := strings.Repeat("─", maxW+2)
	fmt.Println("╭" + border + "╮")
	for i, l := range formatted {
		pad := strings.Repeat(" ", maxW-utf8.RuneCountInString(l))
		text := l
		if i == 0 && isTerminal() {
			text = ansiBold + l + ansiReset
		}
		fmt.Printf("│ %s%s │\n", text, pad)
	}
	fmt.Println("╰" + border + "╯")
}

func ticketCompleteLabel(ticket store.Ticket) string {
	if ticket.Complete {
		return "closed"
	}
	return "open"
}

func formatTicketStatusSymbol(status string, useUnicode bool) string {
	if !useUnicode {
		return ""
	}
	stage, state, err := store.ParseLifecycleStatus(status)
	if err != nil {
		return ""
	}
	switch {
	case stage == store.StageDesign && state == store.StateIdle:
		return "○"
	case stage == store.StageDevelop && state == store.StateIdle:
		return "○"
	case state == store.StateActive:
		return "◑"
	case state == store.StateSuccess:
		return "◉"
	default:
		return ""
	}
}

func formatStatusCounts(statuses map[string]int) string {
	order := []string{
		"design/idle", "design/active", "design/success", "design/fail",
		"develop/idle", "develop/active", "develop/success", "develop/fail",
		"test/idle", "test/active", "test/success", "test/fail",
		"done/success", "done/fail",
	}
	labels := map[string]string{
		"design/idle":     "design/idle",
		"design/active":   "design/active",
		"design/success":  "design/success",
		"design/fail":     "design/fail",
		"develop/idle":    "develop/idle",
		"develop/active":  "develop/active",
		"develop/success": "develop/success",
		"develop/fail":    "develop/fail",
		"test/idle":       "test/idle",
		"test/active":     "test/active",
		"test/success":    "test/success",
		"test/fail":       "test/fail",
		"done/success":    "done/success",
		"done/fail":       "done/fail",
	}
	var parts []string
	for _, status := range order {
		if count := statuses[status]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, labels[status]))
		}
	}
	return strings.Join(parts, ", ")
}

func printRoleTable(roles []store.Role) {
	if len(roles) == 0 {
		fmt.Println("no roles")
		return
	}

	termW := 120
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 { // #nosec G115
		termW = w
	}

	// Fixed columns: ID (6) + gaps (6 for 3 x 2-char gaps) = 12.
	// Remaining space split: title 25%, description 37.5%, ac 37.5%.
	const idW = 6
	const gaps = 6
	remaining := termW - idW - gaps
	if remaining < 30 {
		remaining = 30
	}
	titleW := remaining / 4
	motW := (remaining - titleW) / 2
	goalW := remaining - titleW - motW

	truncRune := func(s string, n int) string {
		r := []rune(strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", ""))
		if len(r) <= n {
			return string(r)
		}
		if n <= 3 {
			return string(r[:n])
		}
		return string(r[:n-3]) + "..."
	}

	rows := make([]string, 0, len(roles))
	for _, role := range roles {
		rows = append(rows, fmt.Sprintf("%d\t%s\t%s\t%s",
			role.ID,
			truncRune(role.Title, titleW),
			truncRune(role.Description, motW),
			truncRune(role.AcceptanceCriteria, goalW),
		))
	}
	printBoxTable("ID\tTITLE\tDESCRIPTION\tAC", rows)
}

func formatHistoryEvent(event store.HistoryEvent) string {
	payload := strings.TrimSpace(event.Payload)
	if payload == "" || payload == "{}" {
		return event.EventType
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return event.EventType + ": " + payload
	}

	switch event.EventType {
	case "ticket_created":
		title, _ := data["title"].(string)
		typ, _ := data["type"].(string)
		status, _ := data["status"].(string)
		return fmt.Sprintf("created %s %q [%s]", typ, title, status)

	case "ticket_lifecycle_changed":
		fromStatus, _ := data["from_status"].(string)
		toStatus, _ := data["to_status"].(string)
		who, _ := data["who"].(string)
		if who != "" {
			return fmt.Sprintf("%s → %s (by %s)", fromStatus, toStatus, who)
		}
		return fmt.Sprintf("%s → %s", fromStatus, toStatus)

	case "ticket_updated":
		return formatTicketUpdatePayload(data)

	case "ticket_assigned":
		assignee, _ := data["assignee"].(string)
		return fmt.Sprintf("assigned to %s", assignee)

	case "ticket_unassigned":
		assignee, _ := data["assignee"].(string)
		return fmt.Sprintf("unassigned from %s", assignee)

	case "ticket_commented":
		text, _ := data["text"].(string)
		author, _ := data["author"].(string)
		if len(text) > 80 {
			text = text[:77] + "..."
		}
		if author != "" {
			return fmt.Sprintf("comment by %s: %s", author, text)
		}
		return fmt.Sprintf("comment: %s", text)

	case "ticket_closed":
		return "closed"

	case "ticket_opened":
		return "reopened"

	case "ticket_archived":
		return "archived"

	case "ticket_unarchived":
		return "unarchived"

	case "ticket_cloned":
		cloneOf, _ := data["clone_of"].(float64)
		if cloneOf > 0 {
			return fmt.Sprintf("cloned from #%d", int64(cloneOf))
		}
		return "cloned"

	case "ticket_parent_set":
		parentID, _ := data["parent_id"].(float64)
		return fmt.Sprintf("parent set to #%d", int64(parentID))

	case "ticket_parent_cleared":
		return "parent removed"

	case "project_access_request_created":
		username, _ := data["username"].(string)
		projectPrefix, _ := data["project_prefix"].(string)
		message, _ := data["message"].(string)
		summary := fmt.Sprintf("%s requested access to %s", username, projectPrefix)
		if strings.TrimSpace(message) != "" {
			summary += ": " + message
		}
		return summary

	case "project_access_request_approved", "project_access_request_rejected":
		username, _ := data["username"].(string)
		projectPrefix, _ := data["project_prefix"].(string)
		requestID, _ := data["request_id"].(float64)
		decisionMessage, _ := data["decision_message"].(string)
		verb := "approved"
		if event.EventType == "project_access_request_rejected" {
			verb = "rejected"
		}
		summary := ""
		switch {
		case requestID > 0:
			summary = fmt.Sprintf("%s access request #%d for %s on %s", verb, int64(requestID), username, projectPrefix)
		default:
			summary = fmt.Sprintf("%s access request for %s on %s", verb, username, projectPrefix)
		}
		if strings.TrimSpace(decisionMessage) != "" {
			summary += ": " + decisionMessage
		}
		return summary

	default:
		return event.EventType + ": " + formatPayloadKeyValues(data)
	}
}

func formatTicketUpdatePayload(data map[string]interface{}) string {
	var parts []string

	interesting := []struct {
		key   string
		label string
	}{
		{"title", "title"},
		{"status", "status"},
		{"assignee", "assignee"},
		{"priority", "priority"},
		{"parent_id", "parent"},
		{"description", "description"},
		{"acceptance_criteria", "acceptance criteria"},
	}

	for _, field := range interesting {
		val, ok := data[field.key]
		if !ok {
			continue
		}
		switch v := val.(type) {
		case string:
			if v == "" {
				continue
			}
			if field.key == "description" || field.key == "acceptance_criteria" {
				if len(v) > 60 {
					v = v[:57] + "..."
				}
			}
			parts = append(parts, fmt.Sprintf("%s: %s", field.label, v))
		case float64:
			parts = append(parts, fmt.Sprintf("%s: %v", field.label, v))
		}
	}

	if len(parts) == 0 {
		return "updated"
	}
	return "updated — " + strings.Join(parts, ", ")
}

func formatPayloadKeyValues(data map[string]interface{}) string {
	var parts []string
	for k, v := range data {
		switch val := v.(type) {
		case string:
			if val != "" {
				parts = append(parts, fmt.Sprintf("%s=%s", k, val))
			}
		default:
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	}
	return strings.Join(parts, ", ")
}
