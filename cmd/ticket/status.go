package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

// statusEnvVars returns the relevant environment variable names and their
// current values (empty string when unset).
func statusEnvVars() map[string]string {
	vars := []string{"TICKET_URL", "TICKET_HOME"}
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
			"config_file":     cfgPath,
			"current_project": project,
			"project_source":  projectSource,
			"username":        username,
			"authenticated":   authenticated,
			"connection":      map[bool]string{true: "success", false: "failure"}[err == nil],
		})
	}
	fmt.Printf("TICKET_URL       : %s\n", serverURL)
	printEnvLine("TICKET_HOME", envVars["TICKET_HOME"])
	fmt.Printf("config_file      : %s\n", cfgPath)
	printProjectLine(project, projectSource)
	fmt.Printf("username         : %s\n", username)
	fmt.Printf("authenticated    : %t\n", authenticated)
	printConnectionLine(err == nil)
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
	if outputJSON {
		return printJSON(map[string]any{
			"db_path":         dbPath,
			"TICKET_HOME":     envVars["TICKET_HOME"],
			"config_file":     cfgPath,
			"current_project": project,
			"project_source":  projectSource,
			"db_exists":       dbExists,
			"connection":      map[bool]string{true: "success", false: "failure"}[localStatusCheck(dbPath) == nil],
		})
	}
	fmt.Printf("db_path          : %s\n", dbPath)
	printEnvLine("TICKET_HOME", envVars["TICKET_HOME"])
	fmt.Printf("config_file      : %s\n", cfgPath)
	printProjectLine(project, projectSource)
	fmt.Printf("db_exists        : %t\n", dbExists)
	err = localStatusCheck(dbPath)
	printConnectionLine(err == nil)
	if !dbExists {
		fmt.Println("hint: run ticket init")
	}
	return err
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

func printEnvLine(name, value string) {
	if value == "" {
		fmt.Printf("%-17s: (not set)\n", name)
	} else {
		fmt.Printf("%-17s: %s\n", name, value)
	}
}

func printProjectLine(project, source string) {
	if project == "" {
		fmt.Printf("current_project  : (none)\n")
		return
	}
	fmt.Printf("current_project  : %s  [%s]\n", project, source)
}

func printConnectionLine(ok bool) {
	status := "failure"
	color := "\x1b[31m"
	if ok {
		status = "success"
		color = "\x1b[32m"
	}
	if noColorOutput {
		fmt.Printf("connection       : %s\n", status)
		return
	}
	fmt.Printf("connection       : %s%s\x1b[0m\n", color, status)
}
