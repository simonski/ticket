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
	cwd, err := os.Getwd()
	if err == nil {
		if lc, ok := config.FindLocalConfig(cwd); ok {
			return lc.CurrentProject, "local: " + lc.Path
		}
	}
	if cfg.CurrentProject != "" {
		cfgPath, _ := config.Path()
		return cfg.CurrentProject, "global: " + cfgPath
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
func printStatusBox(lines []statusLine) {
	const keyWidth = 17
	const padding = 2 // spaces inside each border

	// Measure max content width
	maxContent := 0
	for _, l := range lines {
		if l.key == "" {
			continue
		}
		w := keyWidth + 2 + utf8.RuneCountInString(l.value) // "key : value"
		if w > maxContent {
			maxContent = w
		}
	}
	inner := maxContent + padding*2 // total inside box (between │ and │)

	top := "╭" + strings.Repeat("─", inner) + "╮"
	bot := "╰" + strings.Repeat("─", inner) + "╯"
	fmt.Println(top)
	for _, l := range lines {
		if l.key == "" {
			// blank separator row
			fmt.Println("│" + strings.Repeat(" ", inner) + "│")
			continue
		}
		valueStr := l.value
		if !noColorOutput && l.color != "" {
			valueStr = l.color + l.value + "\x1b[0m"
		}
		content := fmt.Sprintf("%-*s: %s", keyWidth, l.key, valueStr)
		// Pad to inner width (measure without ANSI codes)
		visibleLen := keyWidth + 2 + utf8.RuneCountInString(l.value)
		pad := inner - padding - visibleLen
		if pad < 0 {
			pad = 0
		}
		fmt.Printf("│%s%-*s%s│\n",
			strings.Repeat(" ", padding),
			0, content,
			strings.Repeat(" ", pad+padding))
	}
	fmt.Println(bot)
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
