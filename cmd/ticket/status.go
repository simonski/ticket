package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

// statusEnvVars returns the relevant environment variable names and their
// current values (empty string when unset).
func statusEnvVars() map[string]string {
	vars := []string{"TICKET_HOME", "TICKET_URL", "TICKET_USERNAME"}
	out := make(map[string]string, len(vars))
	for _, k := range vars {
		out[k] = os.Getenv(k)
	}
	return out
}

// resolveCurrentProject returns the active project key and where it came from.
func resolveCurrentProject(cfg config.Config) (project, source string) {
	if cfg.CurrentProject != "" {
		cfgPath, _ := config.Path()
		return cfg.CurrentProject, cfgPath
	}
	return "", ""
}

// statusLine is a key/value row for the status box.
type statusLine struct {
	key   string
	value string
	color string // ANSI color code prefix, e.g. "\x1b[32m"; empty = default
}

func envStatusLine(name, value string) statusLine {
	if value == "" {
		return statusLine{key: name, value: "(not set)"}
	}
	return statusLine{key: name, value: value}
}

func connectionStatusLine(ok bool) statusLine {
	if ok {
		return statusLine{key: "connection", value: "success", color: "\x1b[32m"}
	}
	return statusLine{key: "connection", value: "failure", color: "\x1b[31m"}
}

func projectStatusLine(project, source string) statusLine {
	if project == "" {
		return statusLine{key: "current_project", value: "(none)"}
	}
	return statusLine{key: "current_project", value: project + "  [" + source + "]"}
}

// printStatusBox renders lines inside a rounded Unicode box.
//
// Each line is rendered in two passes: first as a plain string to measure
// visual width, then as a styled string (with any ANSI codes) for printing.
// This keeps the right-hand padding consistent regardless of ANSI content.
func printStatusBox(lines []statusLine) {
	const keyWidth = 17
	const padding = 2 // minimum spaces on each side of content

	type row struct {
		plain  string // visible text, for width measurement
		styled string // text to print (may contain ANSI codes)
	}

	rows := make([]row, len(lines))
	maxWidth := 0
	for i, l := range lines {
		if l.key == "" {
			continue // blank separator — width handled separately
		}
		plain := fmt.Sprintf("%-*s: %s", keyWidth, l.key, l.value)
		styled := plain
		if !noColorOutput && l.color != "" {
			styled = fmt.Sprintf("%-*s: %s%s\x1b[0m", keyWidth, l.key, l.color, l.value)
		}
		rows[i] = row{plain, styled}
		if w := utf8.RuneCountInString(plain); w > maxWidth {
			maxWidth = w
		}
	}

	// inner = visible chars between │ and │ on every row
	inner := maxWidth + padding*2

	fmt.Println("╭" + strings.Repeat("─", inner) + "╮")
	for i, l := range lines {
		if l.key == "" {
			fmt.Println("│" + strings.Repeat(" ", inner) + "│")
			continue
		}
		r := rows[i]
		rightPad := inner - padding - utf8.RuneCountInString(r.plain)
		fmt.Printf("│%s%s%s│\n",
			strings.Repeat(" ", padding),
			r.styled,
			strings.Repeat(" ", rightPad))
	}
	fmt.Println("╰" + strings.Repeat("─", inner) + "╯")
}

func runRemoteStatus(cfg config.Config) error {
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	serverURL := strings.TrimSpace(resolved.ServerURL)
	if serverURL == "" {
		return errors.New("TICKET_URL is required for remote mode")
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	status, err := svc.Status()
	authenticated := err == nil && status.Authenticated
	username := strings.TrimSpace(cfg.Username)
	if status.User != nil {
		username = status.User.Username
	}
	cfgPath, _ := config.Path()
	envVars := statusEnvVars()
	project, projectSource := resolveCurrentProject(cfg)
	if outputJSON {
		return printJSON(map[string]any{
			"TICKET_URL":      serverURL,
			"TICKET_HOME":     envVars["TICKET_HOME"],
			"TICKET_USERNAME": envVars["TICKET_USERNAME"],
			"config_file":     cfgPath,
			"current_project": project,
			"project_source":  projectSource,
			"username":        username,
			"authenticated":   authenticated,
			"connection":      map[bool]string{true: "success", false: "failure"}[err == nil],
		})
	}
	lines := []statusLine{
		envStatusLine("TICKET_HOME", envVars["TICKET_HOME"]),
		envStatusLine("TICKET_URL", serverURL),
		envStatusLine("TICKET_USERNAME", envVars["TICKET_USERNAME"]),
		{},
		{key: "config_file", value: cfgPath},
		projectStatusLine(project, projectSource),
		{key: "username", value: username},
		{key: "authenticated", value: fmt.Sprintf("%t", authenticated)},
		connectionStatusLine(err == nil),
	}
	printStatusBox(lines)
	return err
}

func runLocalStatus() error {
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	dbPath := resolved.DBPath
	_, statErr := os.Stat(dbPath)
	dbExists := statErr == nil
	cfgPath, _ := config.Path()
	envVars := statusEnvVars()
	cfg, _ := config.Load()
	project, projectSource := resolveCurrentProject(cfg)
	connErr := localStatusCheck(dbPath)
	if outputJSON {
		return printJSON(map[string]any{
			"db_path":         dbPath,
			"TICKET_HOME":     envVars["TICKET_HOME"],
			"TICKET_USERNAME": envVars["TICKET_USERNAME"],
			"config_file":     cfgPath,
			"current_project": project,
			"project_source":  projectSource,
			"db_exists":       dbExists,
			"connection":      map[bool]string{true: "success", false: "failure"}[connErr == nil],
		})
	}
	lines := []statusLine{
		envStatusLine("TICKET_HOME", envVars["TICKET_HOME"]),
		envStatusLine("TICKET_URL", "(not set — local mode)"),
		envStatusLine("TICKET_USERNAME", envVars["TICKET_USERNAME"]),
		{},
		{key: "db_path", value: dbPath},
		{key: "config_file", value: cfgPath},
		projectStatusLine(project, projectSource),
		{key: "db_exists", value: fmt.Sprintf("%t", dbExists)},
		connectionStatusLine(connErr == nil),
	}
	printStatusBox(lines)
	if !dbExists {
		fmt.Println("hint: run tk setup")
	}
	return connErr
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
