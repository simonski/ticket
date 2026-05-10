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
	resolved, resolveErr := config.ResolveURL()
	if resolveErr != nil {
		return nil
	}
	subject := remoteConfigSubject()
	var statusErr *client.HTTPStatusError
	if errors.As(err, &statusErr) {
		if msg, ok := remoteHTTPStatusMessage(subject, resolved.ServerURL, statusErr); ok {
			return errors.New(msg)
		}
		return nil
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(msg, "cannot connect to ") {
		return fmt.Errorf("%s is configured for %s, but that server could not be reached.\nCheck that the server, port, and any proxy or tunnel are running.", subject, resolved.ServerURL)
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
	lines = append(lines,
		"  mode             : server",
		fmt.Sprintf("  configured via   : %s", remoteConfiguredVia(resolved.ServerURL, locationSource)),
		fmt.Sprintf("  explanation      : %s", remoteIssueExplanation(err)),
	)
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
	switch err.StatusCode {
	case 401:
		return fmt.Sprintf("%s is configured for %s, but the server rejected the saved credentials (%s).\nRun `tk login` for that server, or check whether this remote is the right one.", subject, serverURL, err.Status), true
	case 403:
		return fmt.Sprintf("%s is configured for %s, but that server refused this request (%s).\nYour account is authenticated but does not have permission for this operation.", subject, serverURL, err.Status), true
	case 404:
		if strings.TrimSpace(err.APIError) != "" {
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
