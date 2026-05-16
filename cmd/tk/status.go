package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func statusEnvValue(name string, secret bool) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "UNSET"
	}
	if secret {
		return "********"
	}
	return value
}

func statusHomeLines(home string) []statusLine {
	home = strings.TrimSpace(home)
	if home == "" {
		home = "UNSET"
	}
	return []statusLine{{key: "TICKET_HOME", value: home}}
}

func mergeStatusHeaderLines(summary []statusLine, configFile string, details []statusLine) []statusLine {
	if len(summary) == 0 {
		if strings.TrimSpace(configFile) != "" {
			details = append(details, statusLine{key: "config_file", value: configFile})
		}
		return details
	}
	if strings.TrimSpace(configFile) != "" {
		summary = injectConfigFileIntoSummary(summary, configFile)
	}
	lines := make([]statusLine, 0, len(details)+1+len(summary))
	lines = append(lines, details...)
	lines = append(lines, statusLine{})
	lines = append(lines, summary...)
	return lines
}

func injectConfigFileIntoSummary(summary []statusLine, configFile string) []statusLine {
	lines := make([]statusLine, 0, len(summary)+1)
	inserted := false
	for _, line := range summary {
		lines = append(lines, line)
		if !inserted && line.key == "project" {
			lines = append(lines, statusLine{key: "config_file", value: configFile})
			inserted = true
		}
	}
	if !inserted {
		lines = append([]statusLine{{key: "config_file", value: configFile}}, lines...)
	}
	return lines
}

// resolveCurrentProject returns the active project key and where it came from.
func resolveCurrentProject(cfg config.Config) (projectID, source string) {
	if projectRef := resolveConfiguredProjectReference(cfg); projectRef != "" {
		return projectRef, effectiveConfigPath()
	}
	return "", ""
}

type currentProjectContext struct {
	project      store.Project
	projectID    string
	source       string
	workflowName string
	defaultDraft *bool
	ok           bool
}

func effectiveConfigPath() string {
	if projectPath, ok, _ := config.ProjectPath(); ok {
		return projectPath
	}
	cfgPath, _ := config.Path()
	return cfgPath
}

func resolveCurrentProjectContext(cfg config.Config, svc libticket.Service) currentProjectContext {
	projectID, source := resolveCurrentProject(cfg)
	if svc == nil {
		return currentProjectContext{projectID: projectID, source: source}
	}
	currentProject, resolvedRef, err := resolveProjectContext(context.Background(), cfg, svc, statusProjectReference(cfg))
	if err != nil {
		return currentProjectContext{projectID: projectID, source: source}
	}
	if strings.TrimSpace(projectID) == "" {
		projectID = resolvedRef
	}
	workflowName := ""
	if currentProject.WorkflowID != nil {
		if wf, err := svc.GetWorkflow(context.Background(), *currentProject.WorkflowID); err == nil {
			workflowName = wf.Name
		}
	}
	return currentProjectContext{
		project:      currentProject,
		projectID:    projectID,
		source:       source,
		workflowName: workflowName,
		defaultDraft: &currentProject.DefaultDraft,
		ok:           true,
	}
}

func statusProjectReference(cfg config.Config) string {
	if ref := resolveConfiguredProjectReference(cfg); ref != "" {
		return ref
	}
	if nearestGitRemoteFromCLI() != "" {
		return ""
	}
	return cfg.ProjectID
}

// statusLine is a key/value row for the status box.
type statusLine struct {
	key   string
	value string
	color string // ANSI color code prefix, e.g. "\x1b[32m"; empty = default
}

// printStatusBox renders lines inside a rounded Unicode box.
//
// Each line is rendered in two passes: first as a plain string to measure
// visual width, then as a styled string (with any ANSI codes) for printing.
// This keeps the right-hand padding consistent regardless of ANSI content.
func printStatusBox(lines []statusLine) {
	printStatusBoxWidth(lines, 0)
}

func printStatusBoxWidth(lines []statusLine, fixedWidth int) {
	const keyWidth = 17
	const padding = 2 // minimum spaces on each side of content

	// Determine terminal width for capping box width.
	// Non-terminal (piped/tests): no cap. Terminal: use detected width.
	maxContent := 0 // 0 = unlimited
	if isTerminal() {
		termW := 120
		if tw, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && tw > 0 { // #nosec G115
			termW = tw
		}
		maxContent = termW - 2 - padding*2
		if maxContent < 40 {
			maxContent = 40
		}
	}

	type row struct {
		plain  string // visible text, for width measurement
		styled string // text to print (may contain ANSI codes)
	}

	rows := make([]row, len(lines))
	maxWidth := 0
	for i, l := range lines {
		if l.key == "" {
			continue
		}
		plainVal := l.value
		keyPart := fmt.Sprintf("%-*s: ", keyWidth, l.key)
		// Truncate value if the full line would exceed terminal width
		if maxContent > 0 {
			maxVal := maxContent - utf8.RuneCountInString(keyPart)
			if maxVal > 0 && utf8.RuneCountInString(plainVal) > maxVal {
				plainVal = string([]rune(plainVal)[:maxVal-1]) + "…"
			}
		}
		plain := keyPart + plainVal
		styled := plain
		if !noColorOutput && l.color != "" {
			styled = fmt.Sprintf("%-*s: %s%s\x1b[0m", keyWidth, l.key, l.color, plainVal)
		}
		rows[i] = row{plain, styled}
		if w := utf8.RuneCountInString(plain); w > maxWidth {
			maxWidth = w
		}
	}

	// Expand to fill the terminal width (or fixedWidth if provided).
	targetWidth := 0
	if fixedWidth > 0 {
		targetWidth = fixedWidth
	} else if isTerminal() {
		if tw, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && tw > 0 { // #nosec G115
			targetWidth = tw
		}
	}
	if targetWidth > 0 {
		contentW := targetWidth - 2 - padding*2 // subtract borders and padding
		if contentW > maxWidth {
			maxWidth = contentW
		}
	}
	inner := maxWidth + padding*2

	fmt.Println("╭" + strings.Repeat("─", inner) + "╮")
	for i, l := range lines {
		if l.key == "" {
			fmt.Println("│" + strings.Repeat(" ", inner) + "│")
			continue
		}
		r := rows[i]
		rightPad := inner - padding - utf8.RuneCountInString(r.plain)
		if rightPad < 0 {
			rightPad = 0
		}
		fmt.Printf("│%s%s%s│\n",
			strings.Repeat(" ", padding),
			r.styled,
			strings.Repeat(" ", rightPad))
	}
	fmt.Println("╰" + strings.Repeat("─", inner) + "╯")
}

func runRemoteStatusWithSummaryStyle(cfg config.Config, statusUnicode bool) error {
	var err error
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	status, err := svc.Status(context.Background())
	authenticated := err == nil && status.Authenticated
	username := strings.TrimSpace(cfg.Username)
	if status.User != nil {
		username = status.User.Username
	}
	cfgPath := effectiveConfigPath()
	ticketHome, _ := config.Home()
	projectSvc := svc
	if err != nil || !authenticated {
		projectSvc = nil
	}
	projectContext := resolveCurrentProjectContext(cfg, projectSvc)
	var summary []statusLine
	if projectContext.ok {
		summary = buildProjectSummaryCoreLines(projectSvc, projectContext.project, statusUnicode, false)
	}
	if outputJSON {
		payload := map[string]any{
			"location":       cfg.Location,
			"TICKET_HOME":    statusEnvValue("TICKET_HOME", false),
			"AGENT_ID":       statusEnvValue("AGENT_ID", false),
			"AGENT_PASSWORD": statusEnvValue("AGENT_PASSWORD", true),
			"config_file":    cfgPath,
			"project_id":     projectContext.projectID,
			"project_source": projectContext.source,
			"username":       username,
			"authenticated":  authenticated,
			"connection":     map[bool]string{true: "success", false: "failure"}[err == nil],
		}
		if serverVersion := strings.TrimSpace(status.ServerVersion); serverVersion != "" {
			payload["server_version"] = serverVersion
		}
		if projectContext.workflowName != "" {
			payload["project_workflow"] = projectContext.workflowName
		}
		if projectContext.defaultDraft != nil {
			payload["project_default_draft"] = *projectContext.defaultDraft
		}
		return printJSON(payload)
	}
	lines := append(statusHomeLines(ticketHome), []statusLine{
		{key: "server_version", value: valueOrDefault(strings.TrimSpace(status.ServerVersion), "(unknown)")},
		{key: "username", value: username},
		{key: "authenticated", value: fmt.Sprintf("%t", authenticated)},
	}...)
	printStatusBox(mergeStatusHeaderLines(summary, cfgPath, lines))
	return err
}

//nolint:unused // retained temporarily during server-only migration cleanup
func runLocalStatusWithSummaryStyle(statusUnicode bool) error {
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	dbPath := resolved.DBPath
	_, statErr := os.Stat(dbPath)
	dbExists := statErr == nil
	cfgPath := effectiveConfigPath()
	cfg, _ := config.Load()
	svc, svcErr := resolveService(cfg)
	if svcErr != nil {
		svc = nil
	}
	projectContext := resolveCurrentProjectContext(cfg, svc)
	var summary []statusLine
	if projectContext.ok {
		summary = buildProjectSummaryCoreLines(svc, projectContext.project, statusUnicode, false)
	}
	connErr := localStatusCheck(dbPath)
	if outputJSON {
		payload := map[string]any{
			"db_path":         dbPath,
			"TICKET_HOME":     statusEnvValue("TICKET_HOME", false),
			"AGENT_ID":        statusEnvValue("AGENT_ID", false),
			"AGENT_PASSWORD":  statusEnvValue("AGENT_PASSWORD", true),
			"config_file":     cfgPath,
			"current_project": projectContext.projectID,
			"project_source":  projectContext.source,
			"db_exists":       dbExists,
			"connection":      map[bool]string{true: "success", false: "failure"}[connErr == nil],
		}
		if projectContext.workflowName != "" {
			payload["project_workflow"] = projectContext.workflowName
		}
		if projectContext.defaultDraft != nil {
			payload["project_default_draft"] = *projectContext.defaultDraft
		}
		return printJSON(payload)
	}
	lines := append(statusHomeLines(filepath.Dir(dbPath)), []statusLine{
		{key: "db_exists", value: fmt.Sprintf("%t", dbExists)},
	}...)
	printStatusBox(mergeStatusHeaderLines(summary, cfgPath, lines))
	if !dbExists {
		fmt.Println("hint: run tk initdb")
	}
	return connErr
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

//nolint:unused // retained temporarily during server-only migration cleanup
func localStatusCheck(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil {
		return err
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&count); err != nil {
		return err
	}
	return nil
}
