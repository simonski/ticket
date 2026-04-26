package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/config"
)

func formatRuntimeError(err error) error {
	if err == nil || outputJSON || !shouldExplainSetup(err) {
		return err
	}
	details, detailsErr := currentSetupDetails(err)
	if detailsErr != nil || strings.TrimSpace(details) == "" {
		return err
	}
	return errors.New(err.Error() + "\n\n" + details)
}

func shouldExplainSetup(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "cannot connect to "):
		return true
	case strings.Contains(msg, "request failed with status 502"),
		strings.Contains(msg, "request failed with status 503"),
		strings.Contains(msg, "request failed with status 504"):
		return true
	case strings.Contains(msg, "service unavailable"),
		strings.Contains(msg, "timeout"),
		strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "broken pipe"):
		return true
	case strings.Contains(msg, "unable to open database file"),
		strings.Contains(msg, "no such file or directory"),
		strings.Contains(msg, "permission denied"),
		strings.Contains(msg, "database is locked"),
		strings.Contains(msg, "database is malformed"):
		return true
	default:
		return false
	}
}

func currentSetupDetails(err error) (string, error) {
	resolved, resolveErr := config.ResolveURL()
	if resolveErr != nil {
		return "", resolveErr
	}
	globalPath, _ := config.Path()
	projectPath, hasProject, _ := config.ProjectPath()
	locationSource := locationSource(globalPath, projectPath, hasProject)

	lines := []string{
		"setup:",
	}
	switch resolved.Mode {
	case config.ModeRemote:
		lines = append(lines,
			fmt.Sprintf("  mode             : %s", resolved.Mode),
			fmt.Sprintf("  configured via   : %s", remoteConfiguredVia(resolved.ServerURL, locationSource)),
		)
		lines = append(lines,
			fmt.Sprintf("  explanation      : %s", remoteIssueExplanation(err)),
		)
	case config.ModeLocal:
		lines = append(lines,
			fmt.Sprintf("  mode             : %s", resolved.Mode),
			fmt.Sprintf("  database         : %s", resolved.DBPath),
			fmt.Sprintf("  configured via   : %s", localConfiguredVia(locationSource)),
			fmt.Sprintf("  explanation      : %s", localIssueExplanation()),
		)
	default:
		lines = append(lines,
			fmt.Sprintf("  mode             : %s", resolved.Mode),
			fmt.Sprintf("  explanation      : %s", "the current setup could not be classified as local or remote"),
		)
	}
	return strings.Join(lines, "\n"), nil
}

func locationSource(globalPath, projectPath string, hasProject bool) string {
	if config.HasLocationOverride() {
		return "-f command-line override"
	}
	if hasProject {
		if projectCfg, ok := loadSetupConfig(projectPath); ok && (strings.TrimSpace(projectCfg.Remote) != "" || strings.TrimSpace(projectCfg.Location) != "") {
			return projectPath
		}
	}
	if globalCfg, ok := loadSetupConfig(globalPath); ok && (strings.TrimSpace(globalCfg.Location) != "" || strings.TrimSpace(globalCfg.DefaultRemote) != "" || len(globalCfg.Remotes) > 0) {
		return globalPath
	}
	return "default local database path"
}

func loadSetupConfig(path string) (config.Config, bool) {
	if strings.TrimSpace(path) == "" {
		return config.Config{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config.Config{}, false
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return config.Config{}, false
	}
	return cfg, true
}

func displayValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "UNSET"
	}
	return value
}

func displayPath(path string, ok bool) string {
	if !ok || strings.TrimSpace(path) == "" {
		return "(none)"
	}
	return path
}

func remoteConfiguredVia(serverURL, source string) string {
	if config.HasLocationOverride() {
		return serverURL
	}
	return source
}

func localConfiguredVia(source string) string {
	if config.HasLocationOverride() {
		return source
	}
	ticketHome := strings.TrimSpace(os.Getenv("TICKET_HOME"))
	if source == "default local database path" && ticketHome != "" {
		return source + " (TICKET_HOME=" + ticketHome + ")"
	}
	return source
}

func remoteIssueExplanation(err error) string {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "503") || strings.Contains(msg, "service unavailable") {
		if config.HasLocationOverride() {
			return "remote mode is active because this command is using an explicit override; a 503 means that remote server, or something in front of it, is currently unavailable"
		}
		return "this directory is configured for a remote server; a 503 means that remote server, or something in front of it, is currently unavailable"
	}
	if config.HasLocationOverride() {
		return "remote mode is active because this command is using an explicit override; check that host, port, credentials, and any proxy or tunnel are the ones you expect"
	}
	return "this directory is configured for a remote server, so check the repo or global config that selected that remote"
}

func localIssueExplanation() string {
	return "local mode is active; the CLI is trying to use the SQLite database shown above, so this looks like a local database path, file access, or initialisation problem"
}
