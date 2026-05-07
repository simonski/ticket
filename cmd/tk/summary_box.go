package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func projectGitRepoValue(project store.Project) string {
	if strings.TrimSpace(project.GitRepository) == "" {
		return "(none)"
	}
	return strings.TrimSpace(project.GitRepository)
}

func buildProjectSummaryCoreLines(svc libticket.Service, project store.Project, statusUnicode, includeOpenTickets bool) []statusLine {
	all, _ := svc.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, false)
	var allTickets []store.Ticket
	var activeTickets []store.Ticket
	for _, t := range all {
		if ticketIsOpenForList(t) {
			allTickets = append(allTickets, t)
			if t.State == store.StateActive {
				activeTickets = append(activeTickets, t)
			}
		}
	}

	typeCounts := map[string]int{}
	for _, t := range allTickets {
		typeCounts[t.Type]++
	}

	recent := make([]store.Ticket, len(allTickets))
	copy(recent, allTickets)
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].UpdatedAt > recent[j].UpdatedAt
	})
	if len(recent) > 5 {
		recent = recent[:5]
	}

	var lines []statusLine
	lines = append(lines, statusLine{key: "project", value: project.Prefix + " — " + project.Title})
	if strings.TrimSpace(project.Description) != "" {
		lines = append(lines, statusLine{key: "description", value: strings.TrimSpace(project.Description)})
	}
	lines = append(lines, statusLine{key: "git", value: projectGitRepoValue(project)})
	workflowName := "(none)"
	if project.WorkflowID != nil {
		if wf, err := svc.GetWorkflow(context.Background(), *project.WorkflowID); err == nil && strings.TrimSpace(wf.Name) != "" {
			workflowName = strings.TrimSpace(wf.Name)
		}
	}
	lines = append(lines, statusLine{key: "workflow", value: workflowName})
	lines = append(lines, statusLine{key: "draft", value: fmt.Sprintf("%t", project.DefaultDraft)})

	if includeOpenTickets {
		lines = append(lines, statusLine{})
		total := len(allTickets)
		typeOrder := []string{"task", "epic", "bug", "story", "requirement", "decision", "question", "note"}
		var typeBreakdown []string
		for _, t := range typeOrder {
			if n := typeCounts[t]; n > 0 {
				label := t + "s"
				if t == "story" {
					label = "stories"
				}
				typeBreakdown = append(typeBreakdown, fmt.Sprintf("%d %s", n, label))
			}
		}
		ticketVal := fmt.Sprintf("%d open", total)
		if len(typeBreakdown) > 0 {
			ticketVal += "  (" + strings.Join(typeBreakdown, ", ") + ")"
		}
		lines = append(lines, statusLine{key: "open tickets", value: ticketVal})
	}

	if len(activeTickets) > 0 {
		lines = append(lines, statusLine{})
		lines = append(lines, statusLine{key: "active", value: fmt.Sprintf("%d in progress", len(activeTickets))})
		for _, t := range activeTickets {
			assignee := t.Assignee
			if assignee == "" {
				assignee = "unassigned"
			}
			val := fmt.Sprintf("%-*s  %s  %s", 30, t.Title, t.Stage, assignee)
			lines = append(lines, statusLine{key: "  " + t.ID, value: val, color: "\x1b[32m"})
		}
	}

	if len(recent) > 0 {
		lines = append(lines, statusLine{})
		lines = append(lines, statusLine{key: "recently active", value: ""})
		now := time.Now().UTC()
		for _, t := range recent {
			sym := formatTicketStatusSymbol(t.Status, statusUnicode)
			ago := timeAgo(t.UpdatedAt, now)
			val := fmt.Sprintf("%s  %-*s  %s  %s", sym, 30, t.Title, t.Status, ago)
			lines = append(lines, statusLine{key: "  " + t.ID, value: val})
		}
	}

	lines = append(lines, statusLine{})
	projects, _ := svc.ListProjects(context.Background())
	users, _ := svc.ListUsers(context.Background())
	agents, _ := svc.ListAgents(context.Background())
	lines = append(lines, statusLine{key: "projects", value: fmt.Sprintf("%d", len(projects))})
	lines = append(lines, statusLine{key: "users", value: fmt.Sprintf("%d", len(users))})
	lines = append(lines, statusLine{key: "agents", value: fmt.Sprintf("%d", len(agents))})

	return lines
}

func buildProjectSummaryLines(svc libticket.Service, project store.Project, statusUnicode bool) []statusLine {
	ticketHome, _ := config.Home()
	resolved, _ := config.ResolveURL()
	cfgPath, _ := config.Path()
	envHome := envValue("TICKET_HOME")

	lines := buildProjectSummaryCoreLines(svc, project, statusUnicode, true)

	lines = append(lines, statusLine{})
	lines = append(lines, statusLine{key: "database", value: resolved.DBPath})
	lines = append(lines, statusLine{key: "config", value: cfgPath})
	if envHome != "" {
		lines = append(lines, statusLine{key: "TICKET_HOME", value: envHome})
	} else {
		lines = append(lines, statusLine{key: "TICKET_HOME", value: ticketHome + "  (auto-discovered)"})
	}

	return lines
}

func printProjectSummaryBox(svc libticket.Service, project store.Project, statusUnicode bool) {
	printStatusBox(buildProjectSummaryLines(svc, project, statusUnicode))
}

func currentProjectSummaryCoreLines(cfg config.Config, svc libticket.Service, statusUnicode bool) []statusLine {
	if svc == nil || strings.TrimSpace(cfg.ProjectID) == "" {
		return nil
	}
	project, err := svc.GetProject(context.Background(), cfg.ProjectID)
	if err != nil {
		return nil
	}
	return buildProjectSummaryCoreLines(svc, project, statusUnicode, false)
}
