package main

import (
	"context"
	"errors"
	"fmt"
	"os"
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

func statusEnvLines() []statusLine {
	return []statusLine{
		{key: "TICKET_URL", value: statusEnvValue("TICKET_URL", false)},
		{key: "TICKET_USERNAME", value: statusEnvValue("TICKET_USERNAME", false)},
		{key: "TICKET_PASSWORD", value: statusEnvValue("TICKET_PASSWORD", true)},
		{key: "AGENT_ID", value: statusEnvValue("AGENT_ID", false)},
		{key: "AGENT_PASSWORD", value: statusEnvValue("AGENT_PASSWORD", true)},
	}
}

func mergeStatusHeaderLines(cfg config.Config, svc libticket.Service, statusUnicode bool, details []statusLine) []statusLine {
	summary := currentProjectSummaryCoreLines(cfg, svc, statusUnicode)
	if len(summary) == 0 {
		return details
	}
	lines := make([]statusLine, 0, len(summary)+1+len(details))
	lines = append(lines, summary...)
	lines = append(lines, statusLine{})
	lines = append(lines, details...)
	return lines
}

// resolveCurrentProject returns the active project key and where it came from.
func resolveCurrentProject(cfg config.Config) (projectID, source string) {
	if cfg.ProjectID != "" {
		cfgPath, _ := config.Path()
		return cfg.ProjectID, cfgPath
	}
	return "", ""
}

func resolveCurrentProjectContext(cfg config.Config, svc libticket.Service) (projectID, projectTitle, source, sdlcName string, defaultDraft *bool) {
	projectID, source = resolveCurrentProject(cfg)
	if projectID == "" || svc == nil {
		return projectID, "", source, "", nil
	}
	currentProject, err := svc.GetProject(context.Background(), projectID)
	if err != nil {
		return projectID, "", source, "", nil
	}
	if currentProject.SdlcID != nil {
		if wf, err := svc.GetSdlc(context.Background(), *currentProject.SdlcID); err == nil {
			sdlcName = wf.Name
		}
	}
	defaultDraft = &currentProject.DefaultDraft
	return projectID, currentProject.Title, source, sdlcName, defaultDraft
}

// statusLine is a key/value row for the status box.
type statusLine struct {
	key   string
	value string
	color string // ANSI color code prefix, e.g. "\x1b[32m"; empty = default
}

func connectionStatusLine(ok bool) statusLine {
	if ok {
		return statusLine{key: "connection", value: "success", color: "\x1b[32m"}
	}
	return statusLine{key: "connection", value: "failure", color: "\x1b[31m"}
}

func projectStatusLine(projectID, projectTitle string) statusLine {
	if projectID == "" {
		return statusLine{key: "current project", value: "(none)"}
	}
	if strings.TrimSpace(projectTitle) == "" {
		return statusLine{key: "current project", value: projectID}
	}
	return statusLine{key: "current project", value: fmt.Sprintf("%s (%s)", projectTitle, projectID)}
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

func runRemoteStatus(cfg config.Config) error {
	return runRemoteStatusWithSummaryStyle(cfg, true)
}

func runRemoteStatusWithSummaryStyle(cfg config.Config, statusUnicode bool) error {
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	serverURL := strings.TrimSpace(resolved.ServerURL)
	if serverURL == "" {
		return errors.New("remote mode requires a location (run tk init to configure)")
	}
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
	cfgPath, _ := config.Path()
	projectID, projectSource := resolveCurrentProject(cfg)
	projectID, projectTitle, projectSource, sdlcName, defaultDraft := resolveCurrentProjectContext(cfg, svc)
	if outputJSON {
		payload := map[string]any{
			"location":        cfg.Location,
			"TICKET_URL":      statusEnvValue("TICKET_URL", false),
			"TICKET_USERNAME": statusEnvValue("TICKET_USERNAME", false),
			"TICKET_PASSWORD": statusEnvValue("TICKET_PASSWORD", true),
			"AGENT_ID":        statusEnvValue("AGENT_ID", false),
			"AGENT_PASSWORD":  statusEnvValue("AGENT_PASSWORD", true),
			"config_file":     cfgPath,
			"project_id":      projectID,
			"project_source":  projectSource,
			"username":        username,
			"authenticated":   authenticated,
			"connection":      map[bool]string{true: "success", false: "failure"}[err == nil],
		}
		if sdlcName != "" {
			payload["project_sdlc"] = sdlcName
		}
		if defaultDraft != nil {
			payload["project_default_draft"] = *defaultDraft
		}
		return printJSON(payload)
	}
	lines := append(statusEnvLines(), []statusLine{
		{key: "config_file", value: cfgPath},
		projectStatusLine(projectID, projectTitle),
		{key: "project_sdlc", value: valueOrDefault(sdlcName, "(none)")},
		{key: "project_default_draft", value: boolString(defaultDraft)},
		{key: "username", value: username},
		{key: "authenticated", value: fmt.Sprintf("%t", authenticated)},
		connectionStatusLine(err == nil),
	}...)
	printStatusBox(mergeStatusHeaderLines(cfg, svc, statusUnicode, lines))
	return err
}

func runLocalStatus() error {
	return runLocalStatusWithSummaryStyle(true)
}

func runLocalStatusWithSummaryStyle(statusUnicode bool) error {
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	dbPath := resolved.DBPath
	_, statErr := os.Stat(dbPath)
	dbExists := statErr == nil
	cfgPath, _ := config.Path()
	cfg, _ := config.Load()
	svc, svcErr := resolveService(cfg)
	if svcErr != nil {
		svc = nil
	}
	projectID, projectTitle, projectSource, sdlcName, defaultDraft := resolveCurrentProjectContext(cfg, svc)
	connErr := localStatusCheck(dbPath)
	if outputJSON {
		payload := map[string]any{
			"db_path":         dbPath,
			"TICKET_URL":      statusEnvValue("TICKET_URL", false),
			"TICKET_USERNAME": statusEnvValue("TICKET_USERNAME", false),
			"TICKET_PASSWORD": statusEnvValue("TICKET_PASSWORD", true),
			"AGENT_ID":        statusEnvValue("AGENT_ID", false),
			"AGENT_PASSWORD":  statusEnvValue("AGENT_PASSWORD", true),
			"config_file":     cfgPath,
			"current_project": projectID,
			"project_source":  projectSource,
			"db_exists":       dbExists,
			"connection":      map[bool]string{true: "success", false: "failure"}[connErr == nil],
		}
		if sdlcName != "" {
			payload["project_sdlc"] = sdlcName
		}
		if defaultDraft != nil {
			payload["project_default_draft"] = *defaultDraft
		}
		return printJSON(payload)
	}
	lines := append(statusEnvLines(), []statusLine{
		{key: "db_path", value: dbPath},
		{key: "config_file", value: cfgPath},
		projectStatusLine(projectID, projectTitle),
		{key: "project_sdlc", value: valueOrDefault(sdlcName, "(none)")},
		{key: "project_default_draft", value: boolString(defaultDraft)},
		{key: "db_exists", value: fmt.Sprintf("%t", dbExists)},
		connectionStatusLine(connErr == nil),
	}...)
	printStatusBox(mergeStatusHeaderLines(cfg, svc, statusUnicode, lines))
	if !dbExists {
		fmt.Println("hint: run tk init")
	}
	return connErr
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func boolString(value *bool) string {
	if value == nil {
		return "(unknown)"
	}
	return fmt.Sprintf("%t", *value)
}

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
