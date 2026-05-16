package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/client"
	"github.com/simonski/ticket/internal/config"
)

func formatRuntimeError(err error) error {
	if err == nil || outputJSON {
		return err
	}
	if concise := conciseRuntimeError(err); concise != nil {
		return concise
	}
	if !shouldExplainSetup(err) {
		return err
	}
	details, detailsErr := currentSetupDetails(err)
	if detailsErr != nil || strings.TrimSpace(details) == "" {
		return err
	}
	return errors.New(err.Error() + "\n\n" + details)
}

func conciseRuntimeError(err error) error {
	serverURL, _, resolveErr := currentConfiguredRemoteServer()
	if resolveErr != nil || strings.TrimSpace(serverURL) == "" {
		return nil
	}
	var statusErr *client.HTTPStatusError
	if errors.As(err, &statusErr) {
		subject := remoteConfigSubject()
		if msg, ok := remoteHTTPStatusMessage(subject, serverURL, statusErr); ok {
			return errors.New(msg)
		}
		return nil
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(msg, "cannot connect to ") {
		return fmt.Errorf("Unable to access %s.", serverURL)
	}
	return nil
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
	globalPath, _ := config.Path()
	projectPath, hasProject, _ := config.ProjectPath()
	if serverURL, source, resolveErr := currentConfiguredRemoteServer(); resolveErr == nil && strings.TrimSpace(serverURL) != "" {
		lines := []string{
			"setup:",
			"  mode             : server",
			fmt.Sprintf("  configured via   : %s", remoteConfiguredVia(serverURL, source)),
			fmt.Sprintf("  explanation      : %s", remoteIssueExplanation(err)),
		}
		return strings.Join(lines, "\n"), nil
	}
	resolved, resolveErr := currentRemoteResolution()
	if resolveErr != nil {
		return "", resolveErr
	}

	lines := []string{
		"setup:",
		fmt.Sprintf("  mode             : %s", valueOrDefault(strings.TrimSpace(resolved.Mode), "local")),
		fmt.Sprintf("  configured via   : %s", locationSource(globalPath, projectPath, hasProject)),
	}
	if strings.TrimSpace(resolved.DBPath) != "" {
		lines = append(lines, fmt.Sprintf("  database         : %s", resolved.DBPath))
	}
	lines = append(lines, fmt.Sprintf("  explanation      : %s", localIssueExplanation(err)))
	return strings.Join(lines, "\n"), nil
}

func locationSource(globalPath, projectPath string, hasProject bool) string {
	if config.HasLocationOverride() {
		return "-f command-line override"
	}
	if strings.TrimSpace(os.Getenv("TICKET_URL")) != "" {
		return "TICKET_URL"
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

func currentRemoteResolution() (config.Resolved, error) {
	if config.HasLocationOverride() {
		return config.ResolveURL()
	}
	if envURL := strings.TrimSpace(os.Getenv("TICKET_URL")); envURL != "" {
		return config.ResolveLocation(envURL)
	}
	cfg, err := config.Load()
	if err != nil {
		return config.Resolved{}, err
	}
	location := configuredServiceLocation(cfg)
	if location == "" {
		location = strings.TrimSpace(cfg.Location)
	}
	return config.ResolveLocation(location)
}

func currentConfiguredRemoteServer() (serverURL, source string, err error) {
	globalPath, _ := config.Path()
	projectPath, hasProject, _ := config.ProjectPath()
	source = locationSource(globalPath, projectPath, hasProject)
	if config.HasLocationOverride() {
		var resolved config.Resolved
		resolved, err = config.ResolveURL()
		if err != nil {
			return "", source, err
		}
		if resolved.Mode == config.ModeRemote {
			return strings.TrimSpace(resolved.ServerURL), source, nil
		}
		return "", source, nil
	}
	if envURL := strings.TrimSpace(os.Getenv("TICKET_URL")); envURL != "" {
		resolved, resolveErr := config.ResolveLocation(envURL)
		if resolveErr != nil {
			return "", source, resolveErr
		}
		if resolved.Mode == config.ModeRemote {
			return strings.TrimSpace(resolved.ServerURL), source, nil
		}
		return "", source, nil
	}
	var cfg config.Config
	cfg, err = config.Load()
	if err != nil {
		return "", source, err
	}
	location := configuredServiceLocation(cfg)
	if location == "" {
		location = strings.TrimSpace(cfg.Location)
	}
	if strings.TrimSpace(location) == "" {
		return "", source, nil
	}
	var resolved config.Resolved
	resolved, err = config.ResolveLocation(location)
	if err != nil {
		return "", source, err
	}
	if resolved.Mode == config.ModeRemote {
		return strings.TrimSpace(resolved.ServerURL), source, nil
	}
	return "", source, nil
}

func loadSetupConfig(path string) (config.Config, bool) {
	if strings.TrimSpace(path) == "" {
		return config.Config{}, false
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is from trusted CLI/config locations
	if err != nil {
		return config.Config{}, false
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return config.Config{}, false
	}
	return cfg, true
}

func remoteConfiguredVia(serverURL, source string) string {
	if config.HasLocationOverride() {
		return serverURL
	}
	return source
}

func remoteIssueExplanation(err error) string {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "503") || strings.Contains(msg, "service unavailable") {
		if config.HasLocationOverride() {
			return "this command is using an explicit server override; a 503 means that server, or something in front of it, is currently unavailable"
		}
		return "this directory is configured for a remote server; a 503 means that remote server, or something in front of it, is currently unavailable"
	}
	if config.HasLocationOverride() {
		return "this command is using an explicit server override; check that host, port, credentials, and any proxy or tunnel are the ones you expect"
	}
	return "this directory is configured for a remote server, so check the repo or global config that selected that remote"
}

func localIssueExplanation(err error) string {
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "no such file or directory"),
		strings.Contains(msg, "unable to open database file"):
		return "this command is currently using local mode, but the local ticket database is missing or unreadable"
	case strings.Contains(msg, "permission denied"):
		return "this command is currently using local mode, but the local ticket database is not readable with the current permissions"
	default:
		return "this command is currently using local mode, so check the local ticket database path and permissions"
	}
}

func remoteConfigSubject() string {
	if config.HasLocationOverride() {
		return "this command"
	}
	if projectPath, ok, _ := config.ProjectPath(); ok && strings.TrimSpace(projectPath) != "" {
		return "this repository"
	}
	return "your ticket CLI"
}

func remoteHTTPStatusMessage(subject, serverURL string, err *client.HTTPStatusError) (string, bool) {
	if err == nil {
		return "", false
	}
	apiMessage := strings.TrimSpace(err.APIError)
	switch err.StatusCode {
	case 401:
		if apiMessage != "" && apiMessage != "unauthorized" {
			return "", false
		}
		return fmt.Sprintf("%s is configured for %s, but the server rejected the saved credentials (%s).\nRun `tk login` for that server, or check whether this remote is the right one.", subject, serverURL, err.Status), true
	case 403:
		if apiMessage != "" && apiMessage != "forbidden" {
			return "", false
		}
		return fmt.Sprintf("%s is configured for %s, but that server refused this request (%s).\nYour account is authenticated but does not have permission for this operation.", subject, serverURL, err.Status), true
	case 404:
		if apiMessage != "" {
			return "", false
		}
		return fmt.Sprintf("%s is configured for %s, but that server does not expose the expected Ticket API (%s).\nCheck that the remote URL points to the Ticket server, not a different site or path.", subject, serverURL, err.Status), true
	case 429:
		return fmt.Sprintf("%s is configured for %s, but that server is rate limiting requests (%s).\nWait a moment and try again, or check whether another process is hammering the API.", subject, serverURL, err.Status), true
	case 500:
		return fmt.Sprintf("%s is configured for %s, but that server hit an internal error (%s).\nCheck the server logs, or try again once the server-side fault is fixed.", subject, serverURL, err.Status), true
	case 502:
		return fmt.Sprintf("%s is configured for %s, but that server is unavailable behind a proxy or gateway (%s).\nCheck the upstream Ticket service and any reverse proxy in front of it.", subject, serverURL, err.Status), true
	case 503:
		return fmt.Sprintf("%s is configured for %s, but that server is currently unavailable (%s).\nCheck whether the server, proxy, or tunnel is up.", subject, serverURL, err.Status), true
	case 504:
		return fmt.Sprintf("%s is configured for %s, but that server timed out behind a proxy or gateway (%s).\nCheck the upstream Ticket service and any reverse proxy in front of it.", subject, serverURL, err.Status), true
	default:
		if err.StatusCode >= 500 {
			return fmt.Sprintf("%s is configured for %s, but that server returned %s.\nCheck the server logs or try again once the remote fault is fixed.", subject, serverURL, err.Status), true
		}
		if err.StatusCode >= 400 && strings.TrimSpace(err.APIError) == "" {
			return fmt.Sprintf("%s is configured for %s, but that server rejected the request (%s).\nCheck that the remote URL and server are the ones you expect.", subject, serverURL, err.Status), true
		}
		return "", false
	}
}
