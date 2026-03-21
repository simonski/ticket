package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/simonski/ticket/internal/store"
)

func printProject(project store.Project) {
	if outputJSON {
		_ = printJSON(project)
		return
	}
	fmt.Printf("project: %s\n", project.Title)
	fmt.Printf("project_id: %d\n", project.ID)
	fmt.Printf("prefix: %s\n", project.Prefix)
	fmt.Printf("status: %s\n", project.Status)
	if project.Description != "" {
		fmt.Printf("description: %s\n", project.Description)
	}
	if project.AcceptanceCriteria != "" {
		fmt.Printf("acceptance_criteria: %s\n", project.AcceptanceCriteria)
	}
	if project.GitRepository != "" {
		fmt.Printf("git_repository: %s\n", project.GitRepository)
	}
	if project.GitBranch != "" {
		fmt.Printf("git_branch: %s\n", project.GitBranch)
	}
	if project.WorkflowID != nil {
		fmt.Printf("workflow_id: %d\n", *project.WorkflowID)
	}
}

func printProjectTable(projects []store.Project, currentProjectID string) {
	if len(projects) == 0 {
		fmt.Println("no projects")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, " \tID\tPREFIX\tTITLE\tSTATUS\tDESCRIPTION")
	currentID := strings.TrimSpace(currentProjectID)
	for _, project := range projects {
		marker := " "
		if strconv.FormatInt(project.ID, 10) == currentID || strings.EqualFold(project.Prefix, currentID) {
			marker = "*"
		}
		desc := project.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\n", marker, project.ID, project.Prefix, project.Title, project.Status, desc)
	}
	_ = w.Flush()
}

func ticketLabel(ticket store.Ticket) string {
	if strings.TrimSpace(ticket.Key) != "" {
		return ticket.Key
	}
	return strconv.FormatInt(ticket.ID, 10)
}

func printTicket(ticket store.Ticket) {
	if outputJSON {
		_ = printJSON(ticket)
		return
	}
	fmt.Printf("ticket: %s\n", ticket.Title)
	fmt.Printf("id: %d\n", ticket.ID)
	fmt.Printf("key: %s\n", ticket.Key)
	fmt.Printf("type: %s\n", ticket.Type)
	fmt.Printf("status: %s\n", ticket.Status)
	fmt.Printf("open: %s\n", ticketOpenLabel(ticket))
	fmt.Printf("archived: %t\n", ticket.Archived)
	fmt.Printf("project_id: %d\n", ticket.ProjectID)
	if ticket.ParentID != nil {
		fmt.Printf("parent_id: %d\n", *ticket.ParentID)
	}
	if ticket.CloneOf != nil {
		fmt.Printf("clone_of: %d\n", *ticket.CloneOf)
	}
	if ticket.Description != "" {
		fmt.Printf("description: %s\n", ticket.Description)
	}
	if ticket.AcceptanceCriteria != "" {
		fmt.Printf("acceptance_criteria: %s\n", ticket.AcceptanceCriteria)
	}
	if ticket.GitRepository != "" {
		fmt.Printf("git_repository: %s\n", ticket.GitRepository)
	}
	if ticket.GitBranch != "" {
		fmt.Printf("git_branch: %s\n", ticket.GitBranch)
	}
	if ticket.EstimateEffort != 0 {
		fmt.Printf("estimate_effort: %d\n", ticket.EstimateEffort)
	}
	if ticket.EstimateComplete != "" {
		fmt.Printf("estimate_complete: %s\n", ticket.EstimateComplete)
	}
}

func printTicketDetails(ticket store.Ticket, dependencies []store.Dependency, history []store.HistoryEvent, workflowStages []store.WorkflowStage, labels []store.Label, totalMinutes int) {
	parentID := ""
	if ticket.ParentID != nil {
		parentID = fmt.Sprintf("%d", *ticket.ParentID)
	}
	dependsOn := formatDependsOn(dependencies)
	fmt.Printf("ID           : %d\n", ticket.ID)
	fmt.Printf("Key          : %s\n", ticket.Key)
	fmt.Printf("Type         : %s\n", ticket.Type)
	fmt.Printf("Description  : %s\n", ticket.Description)
	fmt.Printf("ParentID     : %s\n", parentID)
	if ticket.CloneOf != nil {
		fmt.Printf("CloneOf      : %d\n", *ticket.CloneOf)
	}
	fmt.Printf("ProjectID    : %d\n", ticket.ProjectID)
	fmt.Printf("Title        : %s\n", ticket.Title)
	fmt.Printf("Assignee     : %s\n", ticket.Assignee)
	fmt.Printf("Order        : %d\n", ticket.Order)
	fmt.Printf("EstimateEffort   : %d\n", ticket.EstimateEffort)
	fmt.Printf("EstimateComplete : %s\n", ticket.EstimateComplete)
	fmt.Printf("DependsOn    : %s\n", dependsOn)
	fmt.Printf("Status       : %s\n", ticket.Status)
	fmt.Printf("Stage        : %s\n", ticket.Stage)
	fmt.Printf("State        : %s\n", ticket.State)
	if len(workflowStages) > 0 {
		fmt.Printf("Workflow     : %s\n", renderWorkflowProgress(ticket.Stage, workflowStages))
	}
	fmt.Printf("Open         : %s\n", ticketOpenLabel(ticket))
	fmt.Printf("Archived     : %t\n", ticket.Archived)
	fmt.Printf("Priority     : %d\n", ticket.Priority)
	fmt.Printf("Created      : %s\n", ticket.CreatedAt)
	fmt.Printf("LastModified : %s\n", ticket.UpdatedAt)
	fmt.Printf("Acceptance Criteria : %s\n", ticket.AcceptanceCriteria)
	if len(ticket.Comments) > 0 {
		fmt.Println("Comments     :")
		for _, comment := range ticket.Comments {
			fmt.Printf("  - [%s] %s: %s\n", comment.CreatedAt, comment.Author, comment.Text)
		}
	}
	if len(labels) > 0 {
		var labelNames []string
		for _, l := range labels {
			labelNames = append(labelNames, l.Name)
		}
		fmt.Printf("Labels       : %s\n", strings.Join(labelNames, ", "))
	}
	if totalMinutes > 0 {
		hours := totalMinutes / 60
		mins := totalMinutes % 60
		if hours > 0 {
			fmt.Printf("TimeLogged   : %dh %dm\n", hours, mins)
		} else {
			fmt.Printf("TimeLogged   : %dm\n", mins)
		}
	}
	if len(history) > 0 {
		fmt.Println("History      :")
		for _, event := range history {
			fmt.Printf("  - [%s] %s\n", event.CreatedAt, formatHistoryEvent(event))
		}
	}
}

func printTicketChildren(children []store.Ticket) {
	fmt.Println("Children     :")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, c := range children {
		symbol := formatTicketStatusSymbol(c.Status, true)
		key := c.Key
		if key == "" {
			key = strconv.FormatInt(c.ID, 10)
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", symbol, key, c.Type, c.Status, c.Title)
	}
	_ = w.Flush()
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
		ids = append(ids, strconv.FormatInt(dependency.DependsOn, 10))
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

func printTicketTable(tickets []store.Ticket, dependencies map[int64]string, statusUnicode bool, includeArchived bool, workflowStages []store.WorkflowStage) {
	if len(tickets) == 0 {
		fmt.Println("no tickets")
		return
	}
	// Build stage index for progress display
	stageIndex := make(map[string]int, len(workflowStages))
	for i, s := range workflowStages {
		stageIndex[s.StageName] = i + 1
	}
	totalStages := len(workflowStages)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if includeArchived {
		fmt.Fprintln(w, "MOON\tKEY\tTYPE\tSTATUS\tPROGRESS\tOPEN\tARCHIVED\tPARENT_ID\tASSIGNEE\tPRIORITY\tDEPENDSON\tHEALTH\tTITLE")
	} else {
		fmt.Fprintln(w, "MOON\tKEY\tTYPE\tSTATUS\tPROGRESS\tOPEN\tPARENT_ID\tASSIGNEE\tPRIORITY\tDEPENDSON\tHEALTH\tTITLE")
	}
	for _, ticket := range tickets {
		symbol := formatTicketStatusSymbol(ticket.Status, statusUnicode)
		assignee := ticket.Assignee
		if strings.TrimSpace(assignee) == "" {
			assignee = "-"
		}
		dependsOn := dependencies[ticket.ID]
		if dependsOn == "[]" {
			dependsOn = ""
		}
		parentID := ""
		if ticket.ParentID != nil {
			parentID = strconv.FormatInt(*ticket.ParentID, 10)
		}
		key := ticket.Key
		if strings.TrimSpace(key) == "" {
			key = strconv.FormatInt(ticket.ID, 10)
		}
		progress := ""
		if totalStages > 0 {
			if idx, ok := stageIndex[ticket.Stage]; ok {
				progress = fmt.Sprintf("[%d/%d]", idx, totalStages)
			}
		}
		if includeArchived {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%t\t%s\t%s\t%d\t%s\t%.2f\t%s\n", symbol, key, ticket.Type, ticket.Status, progress, ticketOpenLabel(ticket), ticket.Archived, parentID, assignee, ticket.Priority, dependsOn, float64(ticket.HealthScore)/4.0, ticket.Title)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%.2f\t%s\n", symbol, key, ticket.Type, ticket.Status, progress, ticketOpenLabel(ticket), parentID, assignee, ticket.Priority, dependsOn, float64(ticket.HealthScore)/4.0, ticket.Title)
		}
	}
	_ = w.Flush()
}

func ticketOpenLabel(ticket store.Ticket) string {
	if !ticket.Open {
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tMOTIVATION\tGOALS")
	for _, role := range roles {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", role.ID, role.Title, role.Motivation, role.Goals)
	}
	_ = w.Flush()
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
		return fmt.Sprintf("created %s \"%s\" [%s]", typ, title, status)

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
