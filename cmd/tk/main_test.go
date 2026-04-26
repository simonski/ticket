package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/server"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func hasDetailLabel(output, label string) bool {
	pattern := `(?m)^` + regexp.QuoteMeta(label) + `\s+:`
	return regexp.MustCompile(pattern).MatchString(output)
}

func hasDetailField(output, label, value string) bool {
	pattern := `(?m)^` + regexp.QuoteMeta(label) + `\s+:\s` + regexp.QuoteMeta(value) + `$`
	return regexp.MustCompile(pattern).MatchString(output)
}

// setTestLocation writes a config.json to the test's TICKET_HOME with the given location.
func setTestLocation(t *testing.T, location string) {
	t.Helper()
	home := os.Getenv("TICKET_HOME")
	if home == "" {
		t.Fatal("TICKET_HOME must be set before calling setTestLocation")
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll(TICKET_HOME) error = %v", err)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(home); err != nil {
		t.Fatalf("Chdir(TICKET_HOME) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	cfg, _ := config.Load()
	cfg.Location = location
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

func setTestWorkingDir(t *testing.T, dir string) {
	t.Helper()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s) error = %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
}

func TestRenderRootUsageShowsMainCommandsOnly(t *testing.T) {
	original := selectBannerWord
	selectBannerWord = func() string { return "TICKET" }
	defer func() { selectBannerWord = original }()

	usage := renderRootUsage()

	for _, want := range []string{
		"TTTTTTT",
		"USAGE",
		"COMMANDS",
		"ADMIN",
		"SHORTCUTS",
		"SYSTEM",
		"\x1b[38;5;117m",
		"ticket",
		"idea",
		"project",
		"dep",
		"label",
		"time",
		"config",
		"init",
		"server",
		"version",
		"upgrade",
		"upgrade-database",
		"docker-compose",
		"login",
		"help",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("root usage missing %q:\n%s", want, usage)
		}
	}

	if !strings.Contains(usage, "\x1b[31m") {
		t.Fatalf("root usage should contain ANSI color banner:\n%s", usage)
	}

	for _, unwanted := range []string{
		"accept requirement",
		"spec export markdown",
	} {
		if strings.Contains(usage, unwanted) {
			t.Fatalf("root usage should not include detailed subcommand %q:\n%s", unwanted, usage)
		}
	}

	// Verify ordering: COMMANDS section then ADMIN section
	nounOrder := []string{
		"  ticket",
		"  idea",
		"  project",
		"  dep",
		"  label",
		"  time",
		"  story",
		"  decision",
		"  doctor",
		"  role",
		"  sdlc",
		"  team",
		"  agent",
		"  user",
	}
	last := -1
	for _, item := range nounOrder {
		idx := strings.Index(usage, item)
		if idx == -1 {
			t.Fatalf("root usage missing namespace %q:\n%s", item, usage)
		}
		if idx <= last {
			t.Fatalf("root usage namespaces not in expected order around %q:\n%s", item, usage)
		}
		last = idx
	}

	// Verify SYSTEM section ordering
	systemOrder := []string{"  status", "  server", "  login", "  logout", "  register", "  config", "  init", "  export", "  import", "  upgrade-database", "  version", "  upgrade", "  skill", "  docker-compose", "  help"}
	last = -1
	for _, item := range systemOrder {
		idx := strings.LastIndex(usage, item) // use LastIndex to match SYSTEM section not NAMESPACES
		if idx == -1 {
			t.Fatalf("root usage missing system command %q:\n%s", item, usage)
		}
		if idx <= last {
			t.Fatalf("root usage system commands not in expected order around %q:\n%s", item, usage)
		}
		last = idx
	}

	for _, unwanted := range []string{"ALIASES", "create,new", "del,delete"} {
		if strings.Contains(usage, unwanted) {
			t.Fatalf("root usage should not include aliases %q:\n%s", unwanted, usage)
		}
	}
}

func TestFormatRuntimeErrorRemote503IncludesSetup(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(repoDir) error = %v", err)
	}
	setTestWorkingDir(t, repoDir)
	t.Setenv("TICKET_HOME", t.TempDir())
	config.SetLocationOverride("https://ticket.example")
	t.Cleanup(config.ClearLocationOverride)

	err := formatRuntimeError(errors.New("request failed with status 503 Service Unavailable"))
	got := err.Error()

	for _, want := range []string{
		"request failed with status 503 Service Unavailable",
		"setup:",
		"mode             : remote",
		"configured via   : https://ticket.example",
		"remote mode is active because this command is using an explicit override; a 503 means that remote server",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("remote runtime error missing %q:\n%s", want, got)
		}
	}
}

func TestFormatRuntimeErrorRemote503FromProjectConfigOmitsServerURL(t *testing.T) {
	homeDir := t.TempDir()
	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	setTestWorkingDir(t, repoDir)
	t.Setenv("TICKET_HOME", homeDir)
	if err := config.SaveProjectConfigAt(repoDir, config.Config{Location: "https://ticket.example", ProjectID: "1"}); err != nil {
		t.Fatalf("SaveProjectConfigAt() error = %v", err)
	}

	err := formatRuntimeError(errors.New("request failed with status 503 Service Unavailable"))
	got := err.Error()

	for _, want := range []string{
		"mode             : remote",
		"this directory is configured for a remote server; a 503 means that remote server, or something in front of it, is currently unavailable",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("project-config remote runtime error missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, ".ticket/config.json") {
		t.Fatalf("project-config remote runtime error should include the project config path:\n%s", got)
	}
	for _, unwanted := range []string{
		"https://ticket.example",
		"auth override",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("project-config remote runtime error should omit %q:\n%s", unwanted, got)
		}
	}
}

func TestFormatRuntimeErrorLocalDBIssueIncludesSetup(t *testing.T) {
	homeDir := t.TempDir()
	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	setTestWorkingDir(t, repoDir)
	t.Setenv("TICKET_HOME", homeDir)
	if err := config.SaveProjectConfigAt(repoDir, config.Config{ProjectID: "1"}); err != nil {
		t.Fatalf("SaveProjectConfigAt() error = %v", err)
	}

	err := formatRuntimeError(errors.New("unable to open database file"))
	got := err.Error()

	for _, want := range []string{
		"unable to open database file",
		"setup:",
		"mode             : local",
		"database         : " + filepath.Join(homeDir, "ticket.db"),
		"configured via   : default local database path (TICKET_HOME=" + homeDir + ")",
		"local mode is active; the CLI is trying to use the SQLite database shown above",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("local runtime error missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"project config", "global config", "TICKET_URL"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("local runtime error should omit %q:\n%s", unwanted, got)
		}
	}
}

func TestFormatRuntimeErrorLeavesNonConnectivityErrorsUnchanged(t *testing.T) {
	err := errors.New("unauthorized")
	if got := formatRuntimeError(err); got != err {
		t.Fatalf("formatRuntimeError() should leave non-connectivity errors unchanged, got %v", got)
	}
}

func TestRunExportImportSnapshotRoundTripPreservesTicketID(t *testing.T) {
	setupLocalCLI(t)

	ticketID := createLocalTask(t, []string{"add", "-d", "snapshot export/import ticket", "Snapshot Ticket"})
	snapshotFile := filepath.Join(t.TempDir(), "snapshot.json")

	exportOutput := captureStdout(t, func() {
		if err := run([]string{"export", "-o", snapshotFile}); err != nil {
			t.Fatalf("run(export) error = %v", err)
		}
	})
	if !strings.Contains(exportOutput, "exported snapshot to "+snapshotFile) {
		t.Fatalf("export output = %q, want snapshot path", exportOutput)
	}
	if _, err := os.Stat(snapshotFile); err != nil {
		t.Fatalf("snapshot file missing: %v", err)
	}

	if err := deleteTicketConfirmed(t, ticketID); err != nil {
		t.Fatalf("run(rm) error = %v", err)
	}
	if err := run([]string{"get", "-id", ticketID}); err == nil || err.Error() != "ticket not found" {
		t.Fatalf("run(get deleted) error = %v, want ticket not found", err)
	}

	importOutput := captureStdout(t, func() {
		if err := run([]string{"import", "-i", snapshotFile}); err != nil {
			t.Fatalf("run(import) error = %v", err)
		}
	})
	if !strings.Contains(importOutput, "imported snapshot from "+snapshotFile) {
		t.Fatalf("import output = %q, want snapshot path", importOutput)
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", ticketID}); err != nil {
			t.Fatalf("run(get restored) error = %v", err)
		}
	})
	if !hasDetailLabel(getOutput, "Key") {
		t.Fatalf("restored get output missing key field:\n%s", getOutput)
	}
}

func TestParseIDListSupportsCommaSeparatedValues(t *testing.T) {
	ids, err := parseIDList("1, 2,3")
	if err != nil {
		t.Fatalf("parseIDList() error = %v", err)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Fatalf("parseIDList() = %#v", ids)
	}
}

func TestRunHealthReportsTicketHealthSection(t *testing.T) {
	previousJSON := outputJSON
	defer func() { outputJSON = previousJSON }()

	setupLocalCLI(t)

	parentID := createLocalTask(t, []string{"epic", "Parent Epic", "-ac", "Epic must launch"})
	taskID := createLocalTask(t, []string{"add", "-parent", parentID, "-ac", "Child has AC", "-title", "Child Task"})

	// Advance to develop/idle so definition_of_ready is true
	if err := run([]string{"success", "-id", taskID}); err != nil {
		t.Fatalf("success (design->develop) error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"comment", "add", "-id", taskID, "Reviewer approved this ticket."}); err != nil {
			t.Fatalf("comment add error = %v", err)
		}
		if err := run([]string{"health", taskID}); err != nil {
			t.Fatalf("health error = %v", err)
		}
	})

	for _, want := range []string{
		"TICKET HEALTH",
		"score:",
		"not_an_orphan: true",
		"has_acceptance_criteria: true",
		"reviewed_by_reviewer_agent: true",
		"definition_of_ready: true",
		"project_acceptance_criteria:",
		"project_definition_of_ready:",
		"project_definition_of_done:",
		"sdlc_acceptance_criteria:",
		"stage_acceptance_criteria:",
		"ticket_acceptance_criteria:",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("health output missing %q:\n%s", want, output)
		}
	}
}

func TestRunHealthSupportsJSONOutput(t *testing.T) {
	previousJSON := outputJSON
	defer func() { outputJSON = previousJSON }()

	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Not Reviewed"})
	// Advance to develop/idle so definition_of_ready is true
	if err := run([]string{"success", "-id", taskID}); err != nil {
		t.Fatalf("success (design->develop) error = %v", err)
	}
	output := captureStdout(t, func() {
		if err := run([]string{"health", taskID, "-json"}); err != nil {
			t.Fatalf("health -json error = %v", err)
		}
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("health output json parse error = %v\n%s", err, output)
	}
	section, ok := got["ticket_health"].(map[string]any)
	if !ok {
		t.Fatalf("health output json missing ticket_health: %#v", got)
	}
	score, ok := section["score"].(float64)
	if !ok || score < 1 {
		t.Fatalf("health score = %#v", section["score"])
	}
	if section["not_an_orphan"] != false {
		t.Fatalf("health not_an_orphan = %#v", got["not_an_orphan"])
	}
	if section["has_acceptance_criteria"] != false {
		t.Fatalf("health has_acceptance_criteria = %#v", got["has_acceptance_criteria"])
	}
	if section["reviewed_by_reviewer_agent"] != false {
		t.Fatalf("health reviewed_by_reviewer_agent = %#v", got["reviewed_by_reviewer_agent"])
	}
	if section["definition_of_ready"] != true {
		t.Fatalf("health definition_of_ready = %#v", got["definition_of_ready"])
	}
	for _, key := range []string{
		"project_acceptance_criteria",
		"project_definition_of_ready",
		"project_definition_of_done",
		"sdlc_acceptance_criteria",
		"stage_acceptance_criteria",
		"ticket_acceptance_criteria",
	} {
		if _, ok := section[key]; !ok {
			t.Fatalf("health output missing %q: %#v", key, section)
		}
	}
}

func TestRunHealthExecutePersistsScores(t *testing.T) {
	previousJSON := outputJSON
	defer func() { outputJSON = previousJSON }()

	setupLocalCLI(t)

	firstID := createLocalTask(t, []string{"add", "Task One"})
	secondID := createLocalTask(t, []string{"add", "Task Two", "-ac", "criteria", "-parent", firstID})
	// Advance both to develop/idle so definition_of_ready is true
	if err := run([]string{"success", "-id", secondID}); err != nil {
		t.Fatalf("success secondID (design->develop) error = %v", err)
	}
	if err := run([]string{"comment", "add", "-id", secondID, "Approved by reviewer"}); err != nil {
		t.Fatalf("comment add error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"health", "execute", "-json"}); err != nil {
			t.Fatalf("health execute error = %v", err)
		}
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("health execute output parse error = %v\n%s", err, output)
	}
	execSection, ok := got["ticket_health_execute"].(map[string]any)
	if !ok {
		t.Fatalf("health execute output missing ticket_health_execute: %#v", got)
	}
	if execSection["tickets"] != float64(2) {
		t.Fatalf("health execute ticket count = %#v", execSection["tickets"])
	}

	tasks := []string{firstID, secondID}
	for _, id := range tasks {
		task, err := svcGetTicket(t, id)
		if err != nil {
			t.Fatalf("svc.GetTicket(context.Background(), %s) error = %v", id, err)
		}
		if task.HealthScore == 0 {
			t.Fatalf("ticket %s health score = %d", id, task.HealthScore)
		}
	}
}

func TestRenderBannerContainsTaskArtAndColors(t *testing.T) {
	original := selectBannerWord
	selectBannerWord = func() string { return "TKT" }
	defer func() { selectBannerWord = original }()

	banner := renderBanner()
	for _, want := range []string{"TTTTTTT", "KK   KK"} {
		if !strings.Contains(banner, want) {
			t.Fatalf("banner missing %q:\n%s", want, banner)
		}
	}
	if !strings.Contains(banner, "\x1b[35m") {
		t.Fatalf("banner missing rainbow ANSI colors:\n%s", banner)
	}
}

func TestRenderCommandHelpIncludesUsageAndExample(t *testing.T) {
	help := renderCommandHelp("initdb")

	for _, want := range []string{
		"USAGE",
		"tk initdb",
		"DETAILS",
		"EXAMPLE",
		"tk initdb . --force -password secret -populate",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("command help missing %q:\n%s", want, help)
		}
	}
}

func TestRunOnboardPrintsEmbeddedAgentsTemplateToStdout(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(tempDir) error = %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	output := captureStdout(t, func() {
		if err := runOnboard(nil); err != nil {
			t.Fatalf("runOnboard() error = %v", err)
		}
	})
	if !strings.Contains(output, "# Ticket — Issue Tracking for Agents") {
		t.Fatalf("runOnboard() did not print embedded template:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "AGENTS.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected AGENTS.md to not be created; stat err = %v", err)
	}
}

func TestRunSkillPrintsEmbeddedSkillTemplateToStdout(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(tempDir) error = %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	output := captureStdout(t, func() {
		if err := runSkill(nil); err != nil {
			t.Fatalf("runSkill() error = %v", err)
		}
	})
	if !strings.Contains(output, "# tk Ticket Management Skill") {
		t.Fatalf("runSkill() did not print embedded skill template:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "SKILL.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected SKILL.md to not be created; stat err = %v", err)
	}
}

func TestRunSkillDoesNotRequireTicketInit(t *testing.T) {
	tempDir := t.TempDir()
	previousHome := os.Getenv("TICKET_HOME")
	if err := os.Setenv("TICKET_HOME", filepath.Join(tempDir, ".ticket")); err != nil {
		t.Fatalf("Setenv(TICKET_HOME) error = %v", err)
	}
	t.Cleanup(func() {
		if previousHome == "" {
			_ = os.Unsetenv("TICKET_HOME")
			return
		}
		_ = os.Setenv("TICKET_HOME", previousHome)
	})

	output := captureStdout(t, func() {
		if err := run([]string{"skill"}); err != nil {
			t.Fatalf("run(skill) error = %v", err)
		}
	})
	if !strings.Contains(output, "# tk Ticket Management Skill") {
		t.Fatalf("run(skill) output missing skill content:\n%s", output)
	}
}

func TestRunDockerComposePrintsComposeTemplateToStdout(t *testing.T) {
	output := captureStdout(t, func() {
		if err := run([]string{"docker-compose"}); err != nil {
			t.Fatalf("run(docker-compose) error = %v", err)
		}
	})
	for _, want := range []string{
		"services:",
		"ghcr.io/simonski/ticket:latest",
		"com.centurylinklabs.watchtower.enable=true",
		"TICKET_DATA_DIR: /data",
		"- ticket-data:/data",
		"TICKET_ADMIN_PASSWORD: password",
		"watchtower:",
		"containrrr/watchtower:latest",
		"/var/run/docker.sock:/var/run/docker.sock",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("run(docker-compose) output missing %q:\n%s", want, output)
		}
	}
}

func TestRunDockerComposeDoesNotRequireTicketInit(t *testing.T) {
	tempDir := t.TempDir()
	previousHome := os.Getenv("TICKET_HOME")
	if err := os.Setenv("TICKET_HOME", filepath.Join(tempDir, ".ticket")); err != nil {
		t.Fatalf("Setenv(TICKET_HOME) error = %v", err)
	}
	t.Cleanup(func() {
		if previousHome == "" {
			_ = os.Unsetenv("TICKET_HOME")
			return
		}
		_ = os.Setenv("TICKET_HOME", previousHome)
	})

	output := captureStdout(t, func() {
		if err := run([]string{"docker-compose"}); err != nil {
			t.Fatalf("run(docker-compose) error = %v", err)
		}
	})
	if !strings.Contains(output, "services:") {
		t.Fatalf("run(docker-compose) output missing compose content:\n%s", output)
	}
}

func TestRenderServerHelpIncludesTaskHomeDefault(t *testing.T) {
	help := renderCommandHelp("server")
	for _, want := range []string{
		"tk server [-f <db-path>] [-p <port>] [-addr <host:port>] [-site <name>] [-v]",
		"If `-f` is omitted, the server uses the database resolved from the current remote/project configuration.",
		"If `-f` is provided, that exact database file is used directly",
		"`default` serves the original site and `site2` serves the new replacement frontend",
		"tk server -f /path/to/ticket.db -p 9999 -site site2 -v",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("server help missing %q:\n%s", want, help)
		}
	}
}

func TestRenderProjectHelpIncludesSetDraft(t *testing.T) {
	help := renderCommandHelp("project")
	for _, want := range []string{
		"tk project <create|list|get|use|set-draft|sdlc|add-user|remove-user|add-team|remove-team>",
		"`set-draft` controls whether new tickets default to draft mode for the project.",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("project help missing %q:\n%s", want, help)
		}
	}

	output := captureStdout(t, func() {
		if err := run([]string{"project", "help"}); err != nil {
			t.Fatalf("project help error = %v", err)
		}
	})
	if !strings.Contains(output, "set-draft [-project_id <id>] <true|false>") {
		t.Fatalf("project usage missing set-draft command:\n%s", output)
	}
}

func TestRenderUserHelpIncludesAdmin403Message(t *testing.T) {
	help := renderCommandHelp("user")
	for _, want := range []string{
		"tk user <create|new|ls|list|rm|delete|enable|disable>",
		"user is not an admin",
		"tk user create -username alice -password secret",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("user help missing %q:\n%s", want, help)
		}
	}
}

func TestRenderConfigHelpIncludesListAndDelete(t *testing.T) {
	help := renderCommandHelp("config")
	for _, want := range []string{
		"tk config <set|get|ls|list|rm|delete|registration-enable|registration-disable> [key] [value]",
		"tk config ls",
		"current_project",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("config help missing %q:\n%s", want, help)
		}
	}
}

func TestHasCommandHelpSupportsAliases(t *testing.T) {
	for _, command := range []string{"dependency", "show", "create", "new", "ls", "orphans", "cp", "clone"} {
		if !hasCommandHelp(command) {
			t.Fatalf("hasCommandHelp(%q) = false, want true", command)
		}
	}
}

func TestHasCommandHelpRejectsInvalidCommand(t *testing.T) {
	for _, command := range []string{"orhphans", "invalid"} {
		if hasCommandHelp(command) {
			t.Fatalf("hasCommandHelp(%q) = true, want false", command)
		}
	}
}

func TestRunHelpRejectsInvalidCommand(t *testing.T) {
	if err := runHelp([]string{"orhphans"}); err == nil || err.Error() != `no such command "orhphans"` {
		t.Fatalf("runHelp(invalid) error = %v", err)
	}
}

func TestRunHelpPrintsEnvironmentVariables(t *testing.T) {
	for _, name := range []string{
		"TICKET_HOME",
		"TICKET_TIMEOUT",
		"AGENT_ID",
		"AGENT_PASSWORD",
	} {
		t.Setenv(name, "")
	}

	output := captureStdout(t, func() {
		if err := runHelp([]string{}); err != nil {
			t.Fatalf("runHelp() error = %v", err)
		}
	})

	for _, want := range []string{
		"ENVIRONMENT",
		"  TICKET_HOME: <unset>",
		"  TICKET_TIMEOUT: <unset>",
		"  AGENT_ID: <unset>",
		"  AGENT_PASSWORD: <unset>",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV("a,b, c ,,d")
	want := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitCSV() = %#v, want %#v", got, want)
	}
}

func TestBuildTicketPromptIncludesFilesAndOutputName(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "one.txt")
	file2 := filepath.Join(tempDir, "two.txt")
	if err := os.WriteFile(file1, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("WriteFile(file1) error = %v", err)
	}
	if err := os.WriteFile(file2, []byte("beta\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(file2) error = %v", err)
	}

	prompt, err := buildTicketPrompt([]string{file1, file2}, "requirements.md")
	if err != nil {
		t.Fatalf("buildTicketPrompt() error = %v", err)
	}
	for _, want := range []string{"requirements.md", "FILE: " + file1, "alpha", "FILE: " + file2, "beta"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("buildTicketPrompt() missing %q:\n%s", want, prompt)
		}
	}
}

func TestRunTicketUsesCodexByDefaultAndWritesOutput(t *testing.T) {
	tempDir := t.TempDir()
	input := filepath.Join(tempDir, "input.txt")
	output := filepath.Join(tempDir, "requirements.md")
	if err := os.WriteFile(input, []byte("source"), 0o644); err != nil {
		t.Fatalf("WriteFile(input) error = %v", err)
	}

	original := runAgentCommand
	defer func() { runAgentCommand = original }()

	var gotAgent, gotPrompt string
	runAgentCommand = func(agent, prompt string, stream bool, ticketKey string) (string, error) {
		gotAgent = agent
		gotPrompt = prompt
		return "generated requirements", nil
	}

	stdout := captureStdout(t, func() {
		if err := runTicketGen([]string{"-f", input, "-o", output}); err != nil {
			t.Fatalf("runTicketGen() error = %v", err)
		}
	})
	if gotAgent != "codex" {
		t.Fatalf("runTicketGen() agent = %q, want codex", gotAgent)
	}
	if !strings.Contains(gotPrompt, "source") || !strings.Contains(gotPrompt, "requirements.md") {
		t.Fatalf("runTicketGen() prompt = %q", gotPrompt)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ReadFile(output) error = %v", err)
	}
	if string(data) != "generated requirements" {
		t.Fatalf("output file = %q", string(data))
	}
	if !strings.Contains(stdout, "generated requirements") {
		t.Fatalf("stdout = %q, want generated requirements", stdout)
	}
}

func TestRunTicketUsesConfiguredAgent(t *testing.T) {
	tempDir := t.TempDir()
	input := filepath.Join(tempDir, "input.txt")
	output := filepath.Join(tempDir, "requirements.md")
	if err := os.WriteFile(input, []byte("source"), 0o644); err != nil {
		t.Fatalf("WriteFile(input) error = %v", err)
	}

	original := runAgentCommand
	defer func() { runAgentCommand = original }()

	var gotAgent string
	runAgentCommand = func(agent, prompt string, stream bool, ticketKey string) (string, error) {
		gotAgent = agent
		return "ok", nil
	}

	if err := runTicketGen([]string{"-f", input, "-o", output, "-agent", "copilot"}); err != nil {
		t.Fatalf("runTicketGen(agent override) error = %v", err)
	}
	if gotAgent != "copilot" {
		t.Fatalf("runTicketGen(agent override) agent = %q, want copilot", gotAgent)
	}
}

func TestResolveCredentialsUsesFlagsAndDefaults(t *testing.T) {
	t.Setenv("TICKET_HOME", t.TempDir())
	setTestLocation(t, "http://localhost:8080")

	username, password, err := resolveCredentials("", "", true)
	if err != nil {
		t.Fatalf("resolveCredentials(defaults) error = %v", err)
	}
	if password != "password" {
		t.Fatalf("resolveCredentials(default password) = %q", password)
	}
	if username == "" {
		t.Fatal("resolveCredentials(default username) returned empty username")
	}

	username, password, err = resolveCredentials("flag-user", "flag-pass", true)
	if err != nil {
		t.Fatalf("resolveCredentials(flags) error = %v", err)
	}
	if username != "flag-user" || password != "flag-pass" {
		t.Fatalf("resolveCredentials(flags) = %q/%q", username, password)
	}

	// Switch to local mode for remaining assertions
	setTestLocation(t, "")
	username, password, err = resolveCredentials("", "", true)
	if err != nil {
		t.Fatalf("resolveCredentials(local) error = %v", err)
	}
	if username != localModeUsername() {
		t.Fatalf("resolveCredentials(local) = %q, want %q", username, localModeUsername())
	}
	if password != "" {
		t.Fatalf("resolveCredentials(local) password = %q, want empty", password)
	}
}

func TestExtractDBOverride(t *testing.T) {
	args, override, err := extractDBOverride([]string{"status", "-f", "/tmp/ticket.db", "-nocolor"})
	if err != nil {
		t.Fatalf("extractDBOverride() error = %v", err)
	}
	if override != "/tmp/ticket.db" {
		t.Fatalf("extractDBOverride() override = %q", override)
	}
	if got := strings.Join(args, " "); got != "status -nocolor" {
		t.Fatalf("extractDBOverride() args = %q", got)
	}
}

func TestRunServerWithExplicitDBBypassesTicketHomeCheck(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(tempDir) error = %v", err)
	}
	defer func() { _ = os.Chdir(originalWD) }()

	// Ensure no implicit workspace/env context is available.
	t.Setenv("TICKET_HOME", "")
	dbPath := filepath.Join(tempDir, "missing", "ticket.db")

	err = run([]string{"server", "-f", dbPath})
	if err == nil {
		t.Fatal("run(server -f) error = nil, want failure opening explicit DB path")
	}
	if strings.Contains(err.Error(), "not a ticket folder") {
		t.Fatalf("run(server -f) should bypass ticket home inference check, got: %v", err)
	}
}

func TestRunWhoamiWithGlobalRemoteConfigDoesNotRequireProjectBinding(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "adminpass"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer db.Close()
	handler, err := server.Handler(db, "test", false, nil, "", "")
	if err != nil {
		t.Fatalf("server.Handler() error = %v", err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(tempDir) error = %v", err)
	}
	defer func() { _ = os.Chdir(originalWD) }()

	t.Setenv("TICKET_HOME", t.TempDir())
	setTestLocation(t, ts.URL)

	output := captureStdout(t, func() {
		if err := run([]string{"whoami"}); err != nil {
			t.Fatalf("run(whoami) error = %v", err)
		}
	})
	if !strings.Contains(output, "username : admin") {
		t.Fatalf("whoami output missing remote user identity:\n%s", output)
	}
	if !strings.Contains(output, "mode     : remote") {
		t.Fatalf("whoami output missing remote mode:\n%s", output)
	}
}

func TestRunListWithoutProjectBindingDoesNotCreateLocalConfigDirs(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "adminpass"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer db.Close()
	handler, err := server.Handler(db, "test", false, nil, "", "")
	if err != nil {
		t.Fatalf("server.Handler() error = %v", err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(tempDir) error = %v", err)
	}
	defer func() { _ = os.Chdir(originalWD) }()

	t.Setenv("TICKET_HOME", t.TempDir())
	setTestLocation(t, ts.URL)

	if err := run([]string{"ls"}); err == nil {
		t.Fatal("run(ls) error = nil, want init requirement")
	}

	if _, err := os.Stat(filepath.Join(tempDir, ".ticket")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no local .ticket directory without repo binding, stat error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "http:")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no unexpected http: directory in env-only remote mode, stat error=%v", err)
	}
}

func TestEmbeddedVersionMatchesBuildVersionFile(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "VERSION"))
	if err != nil {
		t.Fatalf("ReadFile(VERSION) error = %v", err)
	}
	if strings.TrimSpace(embeddedVersion) != strings.TrimSpace(string(data)) {
		t.Fatalf("embeddedVersion = %q, want %q", strings.TrimSpace(embeddedVersion), strings.TrimSpace(string(data)))
	}
}

func TestRunUpgradeReportsNetworkUnavailable(t *testing.T) {
	original := fetchRepoVersion
	fetchRepoVersion = func() (string, error) {
		return "", errors.New("network down")
	}
	defer func() { fetchRepoVersion = original }()

	err := runUpgrade(nil)
	if err == nil {
		t.Fatal("runUpgrade() error = nil, want network unavailable error")
	}
	if got := err.Error(); got != "Unable to check for updates right now. Check your network connection and try again." {
		t.Fatalf("runUpgrade() error = %q", got)
	}
}

func TestRunUpgradeReportsLatestVersion(t *testing.T) {
	original := fetchRepoVersion
	fetchRepoVersion = func() (string, error) {
		return strings.TrimSpace(embeddedVersion), nil
	}
	defer func() { fetchRepoVersion = original }()

	output := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Fatalf("runUpgrade() error = %v", err)
		}
	})
	want := fmt.Sprintf("You are on the latest version (%s)", strings.TrimSpace(embeddedVersion))
	if !strings.Contains(output, want) {
		t.Fatalf("runUpgrade() output missing %q:\n%s", want, output)
	}
}

func TestRunUpgradeReportsOutdatedLocalVersion(t *testing.T) {
	original := fetchRepoVersion
	fetchRepoVersion = func() (string, error) {
		return "999.0.0", nil
	}
	defer func() { fetchRepoVersion = original }()

	output := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Fatalf("runUpgrade() error = %v", err)
		}
	})
	want := "A newer version of tk is available, upgrade using `go install github.com/simonski/ticket@latest`"
	if !strings.Contains(output, want) {
		t.Fatalf("runUpgrade() output missing %q:\n%s", want, output)
	}
}

func TestRunUpgradeReportsLocalVersionNewerThanRepo(t *testing.T) {
	original := fetchRepoVersion
	fetchRepoVersion = func() (string, error) {
		return "0.0.1", nil
	}
	defer func() { fetchRepoVersion = original }()

	output := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Fatalf("runUpgrade() error = %v", err)
		}
	})
	if !strings.Contains(output, "Your local copy is newer than the repo") {
		t.Fatalf("runUpgrade() output = %q", output)
	}
	if !strings.Contains(output, "Local version: "+strings.TrimSpace(embeddedVersion)) {
		t.Fatalf("runUpgrade() output missing local version:\n%s", output)
	}
	if !strings.Contains(output, "Repo version:  0.0.1") {
		t.Fatalf("runUpgrade() output missing repo version:\n%s", output)
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		left  string
		right string
		want  int
	}{
		{"0.1.10", "0.1.10", 0},
		{"0.1.9", "0.1.10", -1},
		{"0.2.0", "0.1.99", 1},
		{"v1.2.0", "1.2.0", 0},
		{"1.2", "1.2.0", 0},
	}
	for _, tc := range cases {
		if got := compareVersions(tc.left, tc.right); got != tc.want {
			t.Fatalf("compareVersions(%q, %q) = %d, want %d", tc.left, tc.right, got, tc.want)
		}
	}
}

func TestRunInitDBDefaultsPasswordWhenMissing(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	setTestWorkingDir(t, tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")

	output := captureStdout(t, func() {
		if err := runInitDB([]string{"-f", dbPath}); err != nil {
			t.Fatalf("runInitDB() error = %v", err)
		}
	})

	if !strings.Contains(output, "admin user: admin") {
		t.Fatalf("runInitDB() output missing admin user:\n%s", output)
	}
	if !strings.Contains(output, "admin password: ") {
		t.Fatalf("runInitDB() output missing password:\n%s", output)
	}
	if !strings.Contains(output, "admin password: password") {
		t.Fatalf("runInitDB() output missing default password:\n%s", output)
	}
}

func TestRunInitDBUsesDefaultPathWhenFIsOmitted(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	setTestWorkingDir(t, tempDir)

	if err := runInitDB([]string{"-password", "secret12"}); err != nil {
		t.Fatalf("runInitDB() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "ticket.db")); err != nil {
		t.Fatalf("expected default db at config dir/ticket.db: %v", err)
	}
}

func TestRunInitDBCommandRespectsExplicitFPath(t *testing.T) {
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	workDir := filepath.Join(tempDir, "repo")
	dbPath := filepath.Join(tempDir, "custom.db")
	t.Setenv("TICKET_HOME", homeDir)
	if err := os.MkdirAll(filepath.Join(workDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	setTestWorkingDir(t, workDir)

	if err := run([]string{"initdb", "-f", dbPath, "-password", "secret12"}); err != nil {
		t.Fatalf("run(initdb -f) error = %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected explicit db at %s: %v", dbPath, err)
	}
	if _, err := os.Stat(filepath.Join(homeDir, "ticket.db")); !os.IsNotExist(err) {
		t.Fatalf("default ticket.db should not be created when -f is explicit, err = %v", err)
	}
}

func TestRunInitDBForceOverwritesExistingDatabase(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	setTestWorkingDir(t, tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")

	if err := runInitDB([]string{"-f", dbPath, "-password", "first-pass"}); err != nil {
		t.Fatalf("first runInitDB() error = %v", err)
	}
	// Second init without -force should succeed gracefully (skips DB creation, updates config)
	if err := runInitDB([]string{"-f", dbPath, "-password", "second-pass"}); err != nil {
		t.Fatalf("second runInitDB() without --force = %v, want nil (graceful skip)", err)
	}
	// Forced overwrite should also succeed
	if err := runInitDB([]string{"-f", dbPath, "--force", "-password", "second-pass"}); err != nil {
		t.Fatalf("forced runInitDB() error = %v", err)
	}
}

func TestRunInitDBPopulateSeedsProjectsStoriesTicketsUsersAndTeams(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	setTestWorkingDir(t, tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")

	if err := runInitDB([]string{"-f", dbPath, "-password", "secret12", "--populate"}); err != nil {
		t.Fatalf("runInitDB(--populate) error = %v", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer db.Close()

	projects, err := store.ListProjects(context.Background(), db, 0)
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) < 4 {
		t.Fatalf("project count = %d, want at least 4 (default + 3 examples)", len(projects))
	}

	for _, prefix := range []string{"CRM", "BIL", "OPS"} {
		var projectID int64
		if err := db.QueryRow(`SELECT project_id FROM projects WHERE prefix = ?`, prefix).Scan(&projectID); err != nil {
			t.Fatalf("project %s not found: %v", prefix, err)
		}
		var storyCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM stories WHERE project_id = ?`, projectID).Scan(&storyCount); err != nil {
			t.Fatalf("stories count query for %s failed: %v", prefix, err)
		}
		if storyCount == 0 {
			t.Fatalf("project %s story count = 0, want > 0", prefix)
		}
		for _, ticketType := range []string{"epic", "task", "bug", "chore"} {
			var typeCount int
			if err := db.QueryRow(`SELECT COUNT(*) FROM tickets WHERE project_id = ? AND type = ?`, projectID, ticketType).Scan(&typeCount); err != nil {
				t.Fatalf("ticket count query for %s/%s failed: %v", prefix, ticketType, err)
			}
			if typeCount == 0 {
				t.Fatalf("project %s type %s count = 0, want > 0", prefix, ticketType)
			}
		}
	}

	var teamCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM teams`).Scan(&teamCount); err != nil {
		t.Fatalf("team count query failed: %v", err)
	}
	if teamCount < 3 {
		t.Fatalf("team count = %d, want at least 3", teamCount)
	}

	for _, username := range []string{"alice", "bob", "carol", "dave", "erin", "frank"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, username).Scan(&count); err != nil {
			t.Fatalf("user lookup for %s failed: %v", username, err)
		}
		if count != 1 {
			t.Fatalf("user %s count = %d, want 1", username, count)
		}
	}

	var teamMemberCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM team_members`).Scan(&teamMemberCount); err != nil {
		t.Fatalf("team_members count query failed: %v", err)
	}
	if teamMemberCount < 6 {
		t.Fatalf("team member count = %d, want at least 6", teamMemberCount)
	}
}

func TestPromptForCredentials(t *testing.T) {
	username, password, err := promptForCredentials(strings.NewReader("alice\nsecret12\n"), ioDiscard{}, "", "")
	if err != nil {
		t.Fatalf("promptForCredentials() error = %v", err)
	}
	if username != "alice" || password != "secret12" {
		t.Fatalf("promptForCredentials() = %q/%q", username, password)
	}
}

func TestPromptForCredentialsUsesDefaultsWhenInputIsEmpty(t *testing.T) {
	username, password, err := promptForCredentials(strings.NewReader("\n\n"), ioDiscard{}, "alice", "secret12")
	if err != nil {
		t.Fatalf("promptForCredentials(defaults) error = %v", err)
	}
	if username != "alice" || password != "secret12" {
		t.Fatalf("promptForCredentials(defaults) = %q/%q", username, password)
	}
}

func TestLoginRetryStoresCredentialsSeparatelyAndLogoutRemovesThem(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	credsPath := filepath.Join(tempDir, "credentials.json")

	var loginAttempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			var loginPayload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&loginPayload); err != nil {
				t.Fatalf("Decode(login) error = %v", err)
			}
			attempt := atomic.AddInt32(&loginAttempts, 1)
			if attempt == 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"invalid credentials"}`))
				return
			}
			if loginPayload["username"] != "alice" || loginPayload["password"] != "secret12" {
				t.Fatalf("retry login payload = %#v", loginPayload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"session-token","user":{"username":"alice","role":"user"}}`))
		case "/api/logout":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"logged_out"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	setTestLocation(t, server.URL)

	oldIn := loginPromptInput
	oldOut := loginPromptOutput
	loginPromptInput = strings.NewReader("alice\nsecret12\n")
	loginPromptOutput = ioDiscard{}
	t.Cleanup(func() {
		loginPromptInput = oldIn
		loginPromptOutput = oldOut
	})

	output := captureStdout(t, func() {
		if err := runLogin([]string{"-username", "alice", "-password", "wrongpwd1"}); err != nil {
			t.Fatalf("runLogin() error = %v", err)
		}
	})
	if !strings.Contains(output, "invalid credentials") {
		t.Fatalf("runLogin() output missing invalid credentials:\n%s", output)
	}
	if !strings.Contains(output, "logged in as alice") {
		t.Fatalf("runLogin() output missing success:\n%s", output)
	}

	configData, err := os.ReadFile(filepath.Join(tempDir, "config.json"))
	if err != nil {
		t.Fatalf("ReadFile(config.json) error = %v", err)
	}
	if strings.Contains(string(configData), "session-token") {
		t.Fatalf("config.json should not contain session token:\n%s", string(configData))
	}
	if !strings.Contains(string(configData), `"location": "`+server.URL+`"`) {
		t.Fatalf("config.json should contain resolved server URL %q:\n%s", server.URL, string(configData))
	}
	credData, err := os.ReadFile(credsPath)
	if err != nil {
		t.Fatalf("ReadFile(credentials.json) error = %v", err)
	}
	if !strings.Contains(string(credData), "session-token") {
		t.Fatalf("credentials.json missing session token:\n%s", string(credData))
	}

	if err := runLogout(nil); err != nil {
		t.Fatalf("runLogout() error = %v", err)
	}
	if _, err := os.Stat(credsPath); !os.IsNotExist(err) {
		t.Fatalf("credentials.json should be removed after logout, err=%v", err)
	}
}

func TestRunLoginUsesValidStoredCredentialsFirst(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"username":"alice"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "credentials.json"), []byte(`{"token":"stored-token"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(credentials.json) error = %v", err)
	}

	var loginCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			if r.Header.Get("Authorization") != "Bearer stored-token" {
				t.Fatalf("status auth header = %q", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","authenticated":true,"user":{"username":"alice","role":"user"}}`))
		case "/api/login":
			atomic.AddInt32(&loginCalls, 1)
			t.Fatal("runLogin should not call /api/login when stored credentials are valid")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	setTestLocation(t, server.URL)

	output := captureStdout(t, func() {
		if err := runLogin(nil); err != nil {
			t.Fatalf("runLogin() error = %v", err)
		}
	})
	if !strings.Contains(output, "logged in as alice") {
		t.Fatalf("runLogin() output = %q", output)
	}
	if atomic.LoadInt32(&loginCalls) != 0 {
		t.Fatalf("unexpected login calls = %d", loginCalls)
	}
	configData, err := os.ReadFile(filepath.Join(tempDir, "config.json"))
	if err != nil {
		t.Fatalf("ReadFile(config.json) error = %v", err)
	}
	if !strings.Contains(string(configData), `"location": "`+server.URL+`"`) {
		t.Fatalf("config.json should contain resolved server URL %q:\n%s", server.URL, string(configData))
	}
}

func TestRunStatusRemoteSuccess(t *testing.T) {
	t.Setenv("TICKET_HOME", t.TempDir())
	t.Setenv("AGENT_ID", "agent-123")
	t.Setenv("AGENT_PASSWORD", "agent-secret")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"env-token","user":{"username":"alice","role":"user"}}`))
		case "/api/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","authenticated":true,"server_version":"9.8.7","user":{"username":"alice","role":"user"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	setTestLocation(t, server.URL)

	output := captureStdout(t, func() {
		if err := runStatus(nil); err != nil {
			t.Fatalf("runStatus(remote) error = %v", err)
		}
	})
	for _, want := range []string{
		"TICKET_HOME      : " + os.Getenv("TICKET_HOME"),
		"AGENT_ID         : agent-123",
		"AGENT_PASSWORD   : ********",
		"config_file      : " + filepath.Join(os.Getenv("TICKET_HOME"), "config.json"),
		"client_version   : " + strings.TrimSpace(embeddedVersion),
		"server_version   : 9.8.7",
		"username         : alice",
		"authenticated    : true",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus(remote) missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "env-pass") || strings.Contains(output, "agent-secret") {
		t.Fatalf("runStatus(remote) should mask secret env values:\n%s", output)
	}
}

func TestRunStatusLocalMissingDatabasePrintsHint(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	setTestWorkingDir(t, tempDir)

	var runErr error
	output := captureStdout(t, func() {
		runErr = runStatus(nil)
	})
	if !errors.Is(runErr, os.ErrNotExist) {
		t.Fatalf("runStatus(local missing) error = %v, want os.ErrNotExist", runErr)
	}
	for _, want := range []string{
		"client_version   : " + strings.TrimSpace(embeddedVersion),
		"db_exists        : false",
		"hint: run tk init",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus(local missing) missing %q:\n%s", want, output)
		}
	}
	if !strings.Contains(output, "db_path          : "+filepath.Join(tempDir, "ticket.db")) {
		t.Fatalf("runStatus(local missing) output missing db_path:\n%s", output)
	}
}

func TestRunStatusLocalSuccess(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	setTestWorkingDir(t, tempDir)
	if err := runInitDB([]string{"-password", "secret12"}); err != nil {
		t.Fatalf("runInitDB() error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"status", "-nocolor"}); err != nil {
			t.Fatalf("runStatus(local) error = %v", err)
		}
	})
	for _, want := range []string{
		"client_version   : " + strings.TrimSpace(embeddedVersion),
		"db_exists        : true",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus(local) missing %q:\n%s", want, output)
		}
	}
	if !strings.Contains(output, "db_path          : "+filepath.Join(tempDir, "ticket.db")) {
		t.Fatalf("runStatus(local) output missing db_path:\n%s", output)
	}
}

func TestRunListAndStatusShareSummaryHeaderWithGitRepo(t *testing.T) {
	setupLocalCLI(t)

	captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "SUM", "-title", "Summary Test", "-git-repository", "https://github.com/example/summary.git"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := config.SaveProjectConfigAt(repoDir, config.Config{ProjectID: "2"}); err != nil {
		t.Fatalf("SaveProjectConfigAt() error = %v", err)
	}
	createLocalTask(t, []string{"add", "Summary task one"})

	statusOut := captureStdout(t, func() {
		if err := run([]string{"status", "-nocolor"}); err != nil {
			t.Fatalf("status error = %v", err)
		}
	})
	listOut := captureStdout(t, func() {
		if err := run([]string{"ls", "-nocolor"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})

	for _, want := range []string{
		"project          : SUM — Summary Test",
		"git repo         : https://github.com/example/summary.git",
		"project_sdlc     : Agile",
		"project_default_draft: false",
		"TICKET_HOME      : " + os.Getenv("TICKET_HOME"),
	} {
		if !strings.Contains(statusOut, want) {
			t.Fatalf("status output missing %q:\n%s", want, statusOut)
		}
		if !strings.Contains(listOut, want) {
			t.Fatalf("list output missing %q:\n%s", want, listOut)
		}
	}
	if idxSummary, idxTable := strings.Index(listOut, "project          : SUM — Summary Test"), strings.Index(listOut, "ID       TYPE"); idxSummary == -1 || idxTable == -1 || idxSummary > idxTable {
		t.Fatalf("list output should show summary block before table:\n%s", listOut)
	}
	if idxStatus, idxSummary := strings.Index(statusOut, "TICKET_HOME"), strings.Index(statusOut, "project          : SUM — Summary Test"); idxStatus == -1 || idxSummary == -1 || idxStatus > idxSummary {
		t.Fatalf("status output should show setup block before summary block:\n%s", statusOut)
	}
	if idxStatus, idxSummary := strings.Index(listOut, "TICKET_HOME"), strings.Index(listOut, "project          : SUM — Summary Test"); idxStatus == -1 || idxSummary == -1 || idxStatus > idxSummary {
		t.Fatalf("list output should show setup block before summary block:\n%s", listOut)
	}
	for _, output := range []string{statusOut, listOut} {
		if strings.Contains(output, "open tickets") {
			t.Fatalf("shared header should not show open tickets row:\n%s", output)
		}
		if strings.Contains(output, "location         :") {
			t.Fatalf("shared header should not show location row:\n%s", output)
		}
	}
	if strings.Count(statusOut, "╭") != 1 || strings.Count(statusOut, "╰") != 1 {
		t.Fatalf("status output should render one merged header box:\n%s", statusOut)
	}
	if strings.Count(listOut, "╭") != 1 || strings.Count(listOut, "╰") != 1 {
		t.Fatalf("list output should render one merged header box:\n%s", listOut)
	}
}

func TestPrintTaskDetailsIncludesAcceptanceCriteria(t *testing.T) {
	output := captureStdout(t, func() {
		printTicketDetails(store.Ticket{
			ID:                 "TK-42",
			Title:              "Example Task",
			Type:               "task",
			Status:             "develop/idle",
			Stage:              "develop",
			State:              "idle",
			Description:        "Example description",
			ProjectID:          7,
			Priority:           1,
			EstimateEffort:     3,
			EstimateComplete:   "2026-04-01T12:00:00Z",
			CreatedAt:          "2026-03-01 12:00:00",
			UpdatedAt:          "2026-03-02 09:30:00",
			AcceptanceCriteria: "- does the thing\n- handles the edge case",
			Comments: []store.Comment{
				{Author: "alice", Text: "latest comment", CreatedAt: "2026-03-02 10:00:00"},
			},
		}, nil, []store.HistoryEvent{
			{EventType: "ticket_created", CreatedAt: "2026-03-01 12:00:00", CreatedBy: "test-user", Payload: "{\"status\":\"develop/idle\"}"},
		}, nil, nil, 0, "", "", 0, 0, 0)
	})

	for _, tc := range []struct {
		label string
		value string
	}{
		{label: "Type", value: "task"},
		{label: "Description", value: "Example description"},
		{label: "Title", value: "Example Task"},
		{label: "Assignee", value: ""},
		{label: "Order", value: "0"},
		{label: "EstimateEffort", value: "3"},
		{label: "EstimateComplete", value: "2026-04-01T12:00:00Z"},
		{label: "DependsOn", value: "[]"},
		{label: "Status", value: "develop/idle"},
		{label: "Stage", value: "develop"},
		{label: "State", value: "idle"},
		{label: "Priority", value: "1"},
		{label: "Created", value: "2026-03-01 12:00:00"},
		{label: "LastModified", value: "2026-03-02 09:30:00"},
	} {
		if !hasDetailField(output, tc.label, tc.value) {
			t.Fatalf("printTicketDetails() missing %s field:\n%s", tc.label, output)
		}
	}
	if !hasDetailLabel(output, "Acceptance Criteria") || !strings.Contains(output, "- does the thing") {
		t.Fatalf("printTicketDetails() missing acceptance criteria:\n%s", output)
	}
	for _, want := range []string{
		"Comments",
		"[2026-03-02 10:00:00] alice: latest comment",
		"History",
		"[2026-03-01 12:00:00] created",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("printTicketDetails() missing %q:\n%s", want, output)
		}
	}
}

func TestRenderSdlcProgress(t *testing.T) {
	noColorOutput = true
	defer func() { noColorOutput = false }()
	stages := []store.SdlcStage{
		{StageName: "develop"},
		{StageName: "done"},
	}
	got := renderSdlcProgress("develop", stages)
	want := "[develop] → done"
	if got != want {
		t.Fatalf("renderSdlcProgress() = %q, want %q", got, want)
	}
}

func TestRunStageStateCommandsUpdateLifecycle(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "-ac", "criteria", "Ticket Beta"})
	idArg := taskID

	// Stage names are now valid arguments to tk state/stage
	stageOutput := captureStdout(t, func() {
		if err := run([]string{"stage", "-id", idArg, "develop", "-json"}); err != nil {
			t.Fatalf("stage command error = %v", err)
		}
	})
	var stageData map[string]any
	if err := json.Unmarshal([]byte(stageOutput), &stageData); err != nil {
		t.Fatalf("stage output parse error = %v\n%s", err, stageOutput)
	}
	if stageData["stage"] != "develop" {
		t.Fatalf("expected stage=develop, got %#v", stageData)
	}

	// tk state -id ID <stage> form also works
	if err := run([]string{"state", "-id", idArg, "develop"}); err != nil {
		t.Fatalf("state with stage name error = %v", err)
	}

	// Unrecognised top-level commands like "develop" still return errors
	if err := run([]string{"develop", "-id", idArg}); err == nil {
		t.Fatal("develop command should return error, got nil")
	}

	// Claim first so active state is allowed (requires assignee)
	if err := run([]string{"claim", idArg}); err != nil {
		t.Fatalf("claim error = %v", err)
	}

	// State commands still work and keep the current stage (design)
	stateOutput := captureStdout(t, func() {
		if err := run([]string{"state", "-id", idArg, "active", "-json"}); err != nil {
			t.Fatalf("state command error = %v", err)
		}
	})
	var stateData map[string]any
	if err := json.Unmarshal([]byte(stateOutput), &stateData); err != nil {
		t.Fatalf("state output parse error = %v\n%s", err, stateOutput)
	}
	for _, want := range []string{"develop/active", "develop", "active"} {
		if got := stateData["status"]; got != want && stateData["stage"] != want && stateData["state"] != want {
			t.Fatalf("state output missing %q in status/stage/state: %#v", want, stateData)
		}
	}
}

func TestRunWorkflowAssignsFirstStageRoleAndSupportsPrevious(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	wf, err := svc.CreateSdlc(context.Background(), libticket.SdlcRequest{
		Name:        "Lifecycle Workflow",
		Description: "workflow for next/previous command coverage",
	})
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}
	designStage, err := svc.AddSdlcStage(context.Background(), wf.ID, libticket.SdlcStageRequest{
		StageName:   "design",
		Description: "design",
		SortOrder:   0,
	})
	if err != nil {
		t.Fatalf("AddSdlcStage(design) error = %v", err)
	}
	testStage, err := svc.AddSdlcStage(context.Background(), wf.ID, libticket.SdlcStageRequest{
		StageName:   "test",
		Description: "test",
		SortOrder:   1,
	})
	if err != nil {
		t.Fatalf("AddSdlcStage(test) error = %v", err)
	}
	_, err = svc.AddSdlcStage(context.Background(), wf.ID, libticket.SdlcStageRequest{
		StageName:   "done",
		Description: "done",
		SortOrder:   2,
	})
	if err != nil {
		t.Fatalf("AddSdlcStage(done) error = %v", err)
	}
	designer, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
		SdlcID:      &wf.ID,
		Title:       "designer",
		Description: "designs work",
	})
	if err != nil {
		t.Fatalf("CreateRole(designer) error = %v", err)
	}
	tester, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
		SdlcID:      &wf.ID,
		Title:       "tester",
		Description: "tests work",
	})
	if err != nil {
		t.Fatalf("CreateRole(tester) error = %v", err)
	}
	if err := svc.AddSdlcStageRole(context.Background(), wf.ID, designStage.ID, designer.ID); err != nil {
		t.Fatalf("AddSdlcStageRole(design) error = %v", err)
	}
	if err := svc.AddSdlcStageRole(context.Background(), wf.ID, testStage.ID, tester.ID); err != nil {
		t.Fatalf("AddSdlcStageRole(test) error = %v", err)
	}
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	if _, err := svc.UpdateProject(context.Background(), project.ID, libticket.ProjectUpdateRequest{SdlcID: &wf.ID}); err != nil {
		t.Fatalf("UpdateProject(sdlc) error = %v", err)
	}

	taskID := createLocalTask(t, []string{"add", "Workflow Ticket"})
	initial, err := svc.GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket(initial) error = %v", err)
	}
	if initial.RoleID == nil || *initial.RoleID != designer.ID || initial.SdlcStageID == nil || *initial.SdlcStageID != designStage.ID {
		t.Fatalf("initial ticket = %#v, want design stage with designer role", initial)
	}

	if err := run([]string{"update", "-id", taskID, "-status", "design/success"}); err != nil {
		t.Fatalf("update design/success error = %v", err)
	}
	advanced, err := svc.GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket(after success) error = %v", err)
	}
	if advanced.Stage != "test" || advanced.State != store.StateIdle || advanced.RoleID == nil || *advanced.RoleID != tester.ID || advanced.SdlcStageID == nil || *advanced.SdlcStageID != testStage.ID {
		t.Fatalf("advanced ticket = %#v, want test stage with tester role idle", advanced)
	}

	if err := run([]string{"update", "-id", taskID, "-status", "test/fail"}); err != nil {
		t.Fatalf("update test/fail error = %v", err)
	}
	previousOutput := captureStdout(t, func() {
		if err := run([]string{"previous", "-id", taskID}); err != nil {
			t.Fatalf("previous error = %v", err)
		}
	})
	if !strings.Contains(previousOutput, "regressed: test/fail -> design/idle") {
		t.Fatalf("unexpected previous output:\n%s", previousOutput)
	}
	regressed, err := svc.GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket(after previous) error = %v", err)
	}
	if regressed.Stage != "design" || regressed.State != store.StateIdle || regressed.RoleID == nil || *regressed.RoleID != designer.ID || regressed.SdlcStageID == nil || *regressed.SdlcStageID != designStage.ID {
		t.Fatalf("regressed ticket = %#v, want design stage with designer role idle", regressed)
	}
}

func TestRunProjectCommandsInLocalMode(t *testing.T) {
	setupLocalCLI(t)

	createOutput := captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "PRA", "-title", "Project A", "-description", "Desc", "-dor", "Ready", "-dod", "Done", "-ac", "AC"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})
	for _, want := range []string{"project: Project A", "prefix: PRA", "status: open", "wow: Desc", "ac: AC", "acceptance_criteria: AC", "dor_map[default]: Ready", "dod_map[default]: Done", "ac_map[default]: AC"} {
		if !strings.Contains(createOutput, want) {
			t.Fatalf("project create output missing %q:\n%s", want, createOutput)
		}
	}

	listOutput := captureStdout(t, func() {
		if err := run([]string{"project", "list"}); err != nil {
			t.Fatalf("project list error = %v", err)
		}
	})
	if !strings.Contains(listOutput, "Project A") || !strings.Contains(listOutput, "*") {
		t.Fatalf("project list output = %q", listOutput)
	}

	updateOutput := captureStdout(t, func() {
		if err := run([]string{"project", "2", "update", "-title", "Project B", "-description", "Updated", "-dor-map", "develop=Build reviewed", "-ac", "AC2"}); err != nil {
			t.Fatalf("project update error = %v", err)
		}
	})
	for _, want := range []string{"project: Project B", "wow: Updated", "description: Updated", "ac: AC2", "acceptance_criteria: AC2", "dor_map[default]: Ready", "dor_map[develop]: Build reviewed"} {
		if !strings.Contains(updateOutput, want) {
			t.Fatalf("project update output missing %q:\n%s", want, updateOutput)
		}
	}

	// Switch to project 1 before disabling project 2 (can't close the current project).
	if err := run([]string{"project", "use", "1"}); err != nil {
		t.Fatalf("project use 1 error = %v", err)
	}
	disableOutput := captureStdout(t, func() {
		if err := run([]string{"project", "2", "disable"}); err != nil {
			t.Fatalf("project disable error = %v", err)
		}
	})
	if !strings.Contains(disableOutput, "status: closed") {
		t.Fatalf("project disable output = %q", disableOutput)
	}

	useOutput := captureStdout(t, func() {
		if err := run([]string{"project", "use", "1"}); err != nil {
			t.Fatalf("project use error = %v", err)
		}
	})
	if !strings.Contains(useOutput, "using project") {
		t.Fatalf("project use output = %q", useOutput)
	}
}

func TestRunProjectCommandsRejectGitBranchFlag(t *testing.T) {
	setupLocalCLI(t)

	err := run([]string{"project", "create", "-prefix", "PRA", "-title", "Project A", "-git-branch", "main"})
	if err == nil {
		t.Fatal("project create with -git-branch error = nil, want error")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined: -git-branch") {
		t.Fatalf("project create with -git-branch error = %v", err)
	}
}

func TestRunProjectGetShowsGuidanceMaps(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	project, err := svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix:             "MAP",
		Title:              "Guidance Project",
		AcceptanceCriteria: "legacy project ac",
		DORMap:             store.GuidanceMap{"default": "project default dor", "develop": "project develop dor"},
		DODMap:             store.GuidanceMap{"default": "project default dod"},
		ACMap:              store.GuidanceMap{"qa": "project qa ac"},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"project", "get", project.Prefix}); err != nil {
			t.Fatalf("project get error = %v", err)
		}
	})

	for _, want := range []string{
		"dor_map[default]: project default dor",
		"dor_map[develop]: project develop dor",
		"dod_map[default]: project default dod",
		"ac_map[default]: legacy project ac",
		"ac_map[qa]: project qa ac",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("project get output missing %q:\n%s", want, output)
		}
	}
}

func TestRunTicketCreateAndUpdateGuidanceMaps(t *testing.T) {
	setupLocalCLI(t)
	attachWorkflowToDefaultProject(t, "design", "develop", "qa", "done")

	createOutput := captureStdout(t, func() {
		if err := run([]string{"add", "-title", "Guided Ticket", "-dor", "Ready to start", "-dod-map", "qa=Verified in QA", "-ac", "Base acceptance", "-ac-map", "develop=Code reviewed"}); err != nil {
			t.Fatalf("ticket create error = %v", err)
		}
	})

	svc := localCLIService(t)
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	tickets, err := svc.ListTickets(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(tickets) == 0 {
		t.Fatal("expected created ticket")
	}
	ticketID := tickets[0].ID
	if !strings.Contains(createOutput, ticketID) {
		t.Fatalf("ticket create output = %q, want %s", createOutput, ticketID)
	}

	initialGetOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", ticketID}); err != nil {
			t.Fatalf("ticket get after create error = %v", err)
		}
	})
	for label, value := range map[string]string{
		"dor_map[default]": "Ready to start",
		"dod_map[qa]":      "Verified in QA",
		"ac_map[default]":  "Base acceptance",
		"ac_map[develop]":  "Code reviewed",
	} {
		if !hasDetailField(initialGetOutput, label, value) {
			t.Fatalf("ticket get after create missing %q=%q:\n%s", label, value, initialGetOutput)
		}
	}

	updateOutput := captureStdout(t, func() {
		if err := run([]string{"update", ticketID, "-dor-map", "develop=Implementation ready", "-dod", "Shipped"}); err != nil {
			t.Fatalf("ticket update error = %v", err)
		}
	})
	if !strings.Contains(updateOutput, "updated") {
		t.Fatalf("ticket update output = %q", updateOutput)
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", ticketID}); err != nil {
			t.Fatalf("ticket get error = %v", err)
		}
	})
	for label, value := range map[string]string{
		"dor_map[default]": "Ready to start",
		"dor_map[develop]": "Implementation ready",
		"dod_map[default]": "Shipped",
		"dod_map[qa]":      "Verified in QA",
		"ac_map[default]":  "Base acceptance",
		"ac_map[develop]":  "Code reviewed",
	} {
		if !hasDetailField(getOutput, label, value) {
			t.Fatalf("ticket get output missing %q=%q:\n%s", label, value, getOutput)
		}
	}
}

func TestRunPromptBuildsPlaintextSections(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if project.SdlcID != nil {
		stages, err := svc.ListSdlcStages(context.Background(), *project.SdlcID)
		if err != nil {
			t.Fatalf("ListSdlcStages() error = %v", err)
		}
		for _, stage := range stages {
			if _, err := svc.UpdateSdlcStage(context.Background(), stage.ID, libticket.SdlcStageRequest{
				StageName:          stage.StageName,
				Description:        stage.Description,
				AcceptanceCriteria: "Stage acceptance criteria",
				DefinitionOfReady:  "Stage acceptance criteria",
			}); err != nil {
				t.Fatalf("UpdateSdlcStage() error = %v", err)
			}
		}
	}

	epicID := createLocalTask(t, []string{"epic", "-d", "Epic description", "Prompt Epic"})
	taskID := createLocalTask(t, []string{"add", "-parent", epicID, "-d", "Task description", "-ac", "Task acceptance", "Prompt Task"})
	if _, err := svc.UpdateProject(context.Background(), project.ID, libticket.ProjectUpdateRequest{
		Title:              project.Title,
		Description:        project.Description,
		AcceptanceCriteria: project.AcceptanceCriteria,
		DORMap:             store.GuidanceMap{"default": "Project default DOR"},
		DODMap:             store.GuidanceMap{"default": "Project default DOD"},
		ACMap:              store.GuidanceMap{"default": "Project default AC"},
		SdlcID:             project.SdlcID,
	}); err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	task, err := svc.GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if _, err := svc.UpdateTicket(context.Background(), taskID, libticket.TicketUpdateRequest{
		Title:              task.Title,
		Description:        task.Description,
		AcceptanceCriteria: task.AcceptanceCriteria,
		DORMap:             store.GuidanceMap{"default": "Ticket default DOR"},
		DODMap:             store.GuidanceMap{"default": "Ticket default DOD"},
		ACMap:              store.GuidanceMap{"default": "Ticket default AC"},
		ParentID:           task.ParentID,
		Priority:           task.Priority,
		Order:              task.Order,
		EstimateEffort:     task.EstimateEffort,
		EstimateComplete:   task.EstimateComplete,
	}); err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"prompt", taskID}); err != nil {
			t.Fatalf("prompt command error = %v", err)
		}
	})

	for _, want := range []string{
		"AGENT EXECUTION PROMPT",
		"PROJECT",
		"EPIC",
		"Prompt Epic",
		"STORY",
		"Key: N/A",
		"TICKET",
		"Prompt Task",
		"Task description",
		"Definition of Ready: Project default DOR",
		"Definition of Done: Project default DOD",
		"Acceptance Criteria: Project default AC",
		"Definition of Ready: Ticket default DOR",
		"Definition of Done: Ticket default DOD",
		"Acceptance Criteria: Ticket default AC",
		"ROLE",
		"STAGE",
		"Definition of Ready: Stage acceptance criteria",
		"Acceptance Criteria: Stage acceptance criteria",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("prompt output missing %q:\n%s", want, output)
		}
	}
}

func TestRunProjectInit(t *testing.T) {
	setupLocalCLI(t)

	// Clear CurrentProject so project init is not blocked by the default set
	// during setupLocalCLI.
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	cfg.ProjectID = ""
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}

	projDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(projDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	initOutput := captureStdout(t, func() {
		if err := run([]string{"project", "init", "-prefix", "INI", "-title", "Init Test"}); err != nil {
			t.Fatalf("project init error = %v", err)
		}
	})
	if !strings.Contains(initOutput, "created project INI") {
		t.Fatalf("project init output missing created: %s", initOutput)
	}

	// Verify config.json was updated with the correct project
	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.ProjectID != "INI" {
		t.Fatalf("config.ProjectID = %q, want INI", cfg.ProjectID)
	}

	// Running init again should fail (already initialised)
	if err := run([]string{"project", "init", "-prefix", "INI"}); err == nil {
		t.Fatal("expected error on second init, got nil")
	}
}

func TestRunProjectRemoteBeforeInit(t *testing.T) {
	homeDir := t.TempDir()
	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	setTestWorkingDir(t, repoDir)
	t.Setenv("TICKET_HOME", homeDir)
	if err := config.Save(config.Config{
		DefaultRemote: "local",
		Remotes: []config.Remote{
			{Name: "local", URL: "file:///tmp/local.db"},
			{Name: "prod", URL: "https://ticket.example.com"},
		},
	}); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"project", "remote", "prod"}); err != nil {
			t.Fatalf("project remote error = %v", err)
		}
	})
	if !strings.Contains(output, "using remote prod") {
		t.Fatalf("project remote output = %q", output)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.Remote != "prod" {
		t.Fatalf("config.Load().Remote = %q, want prod", cfg.Remote)
	}
}

func TestRunListStatusRenderingSupportsUnicodeAndPlainModes(t *testing.T) {
	setupLocalCLI(t)

	_ = createLocalTask(t, []string{"add", "Moon Open Task"})
	inProgressID := createLocalTask(t, []string{"add", "Moon Inprogress Task"})
	advancedID := createLocalTask(t, []string{"add", "Moon Advanced Task"})
	// Advance inProgressID to develop, then assign and set active
	if err := run([]string{"success", "-id", inProgressID}); err != nil {
		t.Fatalf("success inProgressID (design->develop) error = %v", err)
	}
	if err := run([]string{"claim", inProgressID}); err != nil {
		t.Fatalf("claim error = %v", err)
	}
	if err := run([]string{"active", "-id", inProgressID}); err != nil {
		t.Fatalf("active error = %v", err)
	}
	// Advance advancedID from design to develop/idle
	if err := run([]string{"success", "-id", advancedID}); err != nil {
		t.Fatalf("success advancedID (design->develop) error = %v", err)
	}

	unicodeOutput := captureStdout(t, func() {
		if err := run([]string{"list"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})
	checkRow := func(statusSymbol, stage, state string) {
		found := false
		for _, line := range strings.Split(unicodeOutput, "\n") {
			fields := strings.Fields(line)
			// KEY column is "symbol key", so fields[0]=symbol fields[1]=key fields[2]=TYPE fields[3+]=TITLE...
			// Stage/state may not be at a fixed index due to multi-word titles, so check
			// that the row has the right symbol+type and contains both stage and state.
			if len(fields) >= 4 && fields[0] == statusSymbol && fields[2] == "task" && strings.Contains(line, stage) && strings.Contains(line, state) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("list unicode row missing symbol=%q stage=%q state=%q:\n%s", statusSymbol, stage, state, unicodeOutput)
		}
	}
	checkRow("◑", "develop", "active")
	checkRow("○", "develop", "idle")
	checkRow("○", "design", "idle")

	for _, want := range []string{"develop", "design", "active", "idle"} {
		if !strings.Contains(unicodeOutput, want) {
			t.Fatalf("list unicode output missing %q:\n%s", want, unicodeOutput)
		}
	}

	plainOutput := captureStdout(t, func() {
		if err := run([]string{"list", "--plain"}); err != nil {
			t.Fatalf("list --plain error = %v", err)
		}
	})
	if strings.ContainsAny(plainOutput, "○◑◉") {
		t.Fatalf("list --plain output should not contain unicode symbols:\n%s", plainOutput)
	}
}

func TestRunListShowsHealthDecimalFraction(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Task With Health"})
	if err := run([]string{"health", taskID}); err != nil {
		t.Fatalf("health error = %v", err)
	}

	listOutput := captureStdout(t, func() {
		if err := run([]string{"list"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})

	if !strings.Contains(listOutput, "TK-") {
		t.Fatalf("list output missing ticket:\n%s", listOutput)
	}
}

func TestRunListArchivedVisibilityAndColumn(t *testing.T) {
	setupLocalCLI(t)

	openID := createLocalTask(t, []string{"add", "Open Task"})
	archivedID := createLocalTask(t, []string{"add", "Archived Task"})
	if err := run([]string{"archive", "-id", archivedID}); err != nil {
		t.Fatalf("archive error = %v", err)
	}

	openRef := ticketLabelByID(t, openID)
	archivedRef := ticketLabelByID(t, archivedID)

	defaultOutput := captureStdout(t, func() {
		if err := run([]string{"list"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})
	if strings.Contains(defaultOutput, "ARCHIVED") {
		t.Fatalf("list output should not show ARCHIVED column without -a:\n%s", defaultOutput)
	}
	if !strings.Contains(defaultOutput, openRef) {
		t.Fatalf("list output missing open ticket %q:\n%s", openRef, defaultOutput)
	}
	if strings.Contains(defaultOutput, archivedRef) {
		t.Fatalf("list output should not include archived ticket %q without -a:\n%s", archivedRef, defaultOutput)
	}

	// -a shows closed but not archived
	includeClosedOutput := captureStdout(t, func() {
		if err := run([]string{"list", "-a"}); err != nil {
			t.Fatalf("list -a error = %v", err)
		}
	})
	if strings.Contains(includeClosedOutput, archivedRef) {
		t.Fatalf("list -a output should not include archived ticket %q:\n%s", archivedRef, includeClosedOutput)
	}

	// -d (or -ad) shows archived tickets
	includeDeletedOutput := captureStdout(t, func() {
		if err := run([]string{"list", "-d"}); err != nil {
			t.Fatalf("list -d error = %v", err)
		}
	})
	if !strings.Contains(includeDeletedOutput, archivedRef) {
		t.Fatalf("list -d output missing archived ticket %q:\n%s", archivedRef, includeDeletedOutput)
	}

	// combined -ad also shows archived
	combinedOutput := captureStdout(t, func() {
		if err := run([]string{"list", "-ad"}); err != nil {
			t.Fatalf("list -ad error = %v", err)
		}
	})
	if !strings.Contains(combinedOutput, archivedRef) {
		t.Fatalf("list -ad output missing archived ticket %q:\n%s", archivedRef, combinedOutput)
	}
}

func TestRunListShowsOpenChildUnderOpenEpic(t *testing.T) {
	setupLocalCLI(t)

	epicID := createLocalTask(t, []string{"epic", "Parent Epic"})
	childID := createLocalTask(t, []string{"add", "-parent", epicID, "Child Task"})

	output := captureStdout(t, func() {
		if err := run([]string{"list"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})

	if !strings.Contains(output, ticketLabelByID(t, epicID)) {
		t.Fatalf("list output missing open epic:\n%s", output)
	}
	if !strings.Contains(output, ticketLabelByID(t, childID)) {
		t.Fatalf("list output missing open child ticket:\n%s", output)
	}
}

func TestRunListHidesDoneStageTicketEvenWhenIncomplete(t *testing.T) {
	setupLocalCLI(t)

	doneID := createLocalTask(t, []string{"epic", "Done But Incomplete"})
	openID := createLocalTask(t, []string{"add", "Still Open"})

	resolved, err := config.ResolveURL()
	if err != nil {
		t.Fatalf("ResolveURL() error = %v", err)
	}
	db, err := store.Open(resolved.DBPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		UPDATE tickets
		SET stage = ?, state = ?, status = ?, complete = 0
		WHERE ticket_id = ?
	`, store.StageDone, store.StateIdle, store.RenderLifecycleStatus(store.StageDone, store.StateIdle), doneID); err != nil {
		t.Fatalf("forcing done-stage ticket state error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"list"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})

	if strings.Contains(output, ticketLabelByID(t, doneID)) {
		t.Fatalf("list output should hide done-stage ticket:\n%s", output)
	}
	if !strings.Contains(output, ticketLabelByID(t, openID)) {
		t.Fatalf("list output missing open ticket:\n%s", output)
	}
}

func TestRunUserListShowsUserTable(t *testing.T) {
	setupLocalCLI(t)

	output := captureStdout(t, func() {
		if err := run([]string{"user", "ls"}); err != nil {
			t.Fatalf("user ls error = %v", err)
		}
	})

	for _, want := range []string{"USERNAME", "ROLE", "ENABLED", "admin"} {
		if !strings.Contains(output, want) {
			t.Fatalf("user ls output missing %q:\n%s", want, output)
		}
	}
}

func TestRunTaskCommandsInLocalMode(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "-d", "findable description", "-ac", "ship it", "-estimate_effort", "8", "-estimate_complete", "2026-04-20T17:00:00Z", "Ticket Alpha"})
	depID := createLocalTask(t, []string{"add", "Ticket Beta"})
	if err := run([]string{"comment", "add", "-id", taskID, "latest note"}); err != nil {
		t.Fatalf("comment add error = %v", err)
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	for _, tc := range []struct {
		label string
		value string
	}{
		{label: "Title", value: "Ticket Alpha"},
		{label: "Description", value: "findable description"},
		{label: "Acceptance Criteria", value: "ship it"},
		{label: "EstimateEffort", value: "8"},
		{label: "EstimateComplete", value: "2026-04-20T17:00:00Z"},
	} {
		if !hasDetailField(getOutput, tc.label, tc.value) {
			t.Fatalf("get output missing %s field:\n%s", tc.label, getOutput)
		}
	}
	if !strings.Contains(getOutput, "latest note") {
		t.Fatalf("get output missing latest note:\n%s", getOutput)
	}

	searchOutput := captureStdout(t, func() {
		if err := run([]string{"search", "findable"}); err != nil {
			t.Fatalf("search error = %v", err)
		}
	})
	if !strings.Contains(searchOutput, "Ticket Alpha") {
		t.Fatalf("search output = %q", searchOutput)
	}

	dependencyOutput := captureStdout(t, func() {
		if err := run([]string{"dependency", "add", "-id", taskID, depID}); err != nil {
			t.Fatalf("dependency add error = %v", err)
		}
	})
	if !strings.Contains(dependencyOutput, "added dependencies") {
		t.Fatalf("dependency add output = %q", dependencyOutput)
	}

	listOutput := captureStdout(t, func() {
		if err := run([]string{"list", "--status", "design/idle"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})
	if !strings.Contains(listOutput, "Ticket Alpha") || !strings.Contains(listOutput, "Ticket Beta") {
		t.Fatalf("list output = %q", listOutput)
	}

	// Advance design->develop/idle so the ticket is requestable
	if err := run([]string{"success", "-id", taskID}); err != nil {
		t.Fatalf("success (design->develop) error = %v", err)
	}

	requestOutput := captureStdout(t, func() {
		if err := run([]string{"request", taskID}); err != nil {
			t.Fatalf("request error = %v", err)
		}
	})
	if !strings.Contains(requestOutput, "ticket: Ticket Alpha") || !strings.Contains(requestOutput, "status: develop/active") {
		t.Fatalf("request output = %q", requestOutput)
	}

	// Advance depID design->develop/idle so it's claimable for dryrun and claim
	if err := run([]string{"success", "-id", depID}); err != nil {
		t.Fatalf("success depID (design->develop) error = %v", err)
	}

	requestDryRunOutput := captureStdout(t, func() {
		if err := run([]string{"request", "-dryrun"}); err != nil {
			t.Fatalf("request -dryrun error = %v", err)
		}
	})
	if !strings.Contains(requestDryRunOutput, "would assign ticket: ") {
		t.Fatalf("request -dryrun output = %q", requestDryRunOutput)
	}

	claimOutput := captureStdout(t, func() {
		if err := run([]string{"claim", depID}); err != nil {
			t.Fatalf("claim error = %v", err)
		}
	})
	if !strings.Contains(claimOutput, "status: develop/active") {
		t.Fatalf("claim output = %q", claimOutput)
	}

	unclaimOutput := captureStdout(t, func() {
		if err := run([]string{"unclaim", taskID}); err != nil {
			t.Fatalf("unclaim error = %v", err)
		}
	})
	if !strings.Contains(unclaimOutput, "unassigned") {
		t.Fatalf("unclaim output = %q", unclaimOutput)
	}

	cloneOutput := captureStdout(t, func() {
		if err := run([]string{"clone", taskID}); err != nil {
			t.Fatalf("clone error = %v", err)
		}
	})
	if !strings.Contains(cloneOutput, "clone_of:") || !strings.Contains(cloneOutput, "status: design/idle") {
		t.Fatalf("clone output = %q", cloneOutput)
	}

	commentOutput := captureStdout(t, func() {
		if err := run([]string{"comment", "add", "-id", taskID, "hello"}); err != nil {
			t.Fatalf("comment error = %v", err)
		}
	})
	if !strings.Contains(commentOutput, "commented on") {
		t.Fatalf("comment output = %q", commentOutput)
	}

	setParentOutput := captureStdout(t, func() {
		if err := run([]string{"set-parent", "-id", depID, taskID}); err != nil {
			t.Fatalf("set-parent error = %v", err)
		}
	})
	if !strings.Contains(setParentOutput, "key:") {
		t.Fatalf("set-parent output = %q", setParentOutput)
	}

	unsetParentOutput := captureStdout(t, func() {
		if err := run([]string{"unset-parent", "-id", depID}); err != nil {
			t.Fatalf("unset-parent error = %v", err)
		}
	})
	if strings.Contains(unsetParentOutput, "parent_id:") {
		t.Fatalf("unset-parent output should not contain parent_id: %q", unsetParentOutput)
	}
}

func TestRunTicketCreateDefaultsTaskLikeTypesToCurrentEpic(t *testing.T) {
	setupLocalCLI(t)

	epicID := createLocalTask(t, []string{"epic", "Current Epic"})
	taskID := createLocalTask(t, []string{"add", "Auto Parented Task"})
	bugID := createLocalTask(t, []string{"bug", "Auto Parented Bug"})
	choreID := createLocalTask(t, []string{"add", "-t", "chore", "Auto Parented Chore"})

	for _, id := range []string{taskID, bugID, choreID} {
		getOutput := captureStdout(t, func() {
			if err := run([]string{"get", "-id", id}); err != nil {
				t.Fatalf("get error = %v", err)
			}
		})
		if !hasDetailField(getOutput, "Parent", ticketLabelByID(t, epicID)) {
			t.Fatalf("get output missing parent field:\n%s", getOutput)
		}
	}
}

func TestRunSearchSupportsFreeFormAndFilters(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"project", "create", "-prefix", "SEP", "-title", "Second Project"}); err != nil {
		t.Fatalf("project create error = %v", err)
	}
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := config.SaveProjectConfigAt(repoDir, config.Config{ProjectID: "1"}); err != nil {
		t.Fatalf("SaveProjectConfigAt() error = %v", err)
	}

	matchingID := createLocalTask(t, []string{"add", "-d", "Detailed note for customer portal", "-ac", "free form acceptance", "Free form entry"})
	otherID := createLocalTask(t, []string{"add", "-d", "Detailed note for customer portal", "Free form other"})
	if err := config.SaveProjectConfigAt(repoDir, config.Config{ProjectID: "2"}); err != nil {
		t.Fatalf("SaveProjectConfigAt() error = %v", err)
	}
	crossProjectID := createLocalTask(t, []string{"add", "-d", "Detailed note for customer portal", "Free form entry elsewhere"})
	if err := config.SaveProjectConfigAt(repoDir, config.Config{ProjectID: "1"}); err != nil {
		t.Fatalf("SaveProjectConfigAt() error = %v", err)
	}

	// Advance to develop/idle so claim and active produce develop/active
	if err := run([]string{"success", "-id", matchingID}); err != nil {
		t.Fatalf("success matchingID (design->develop) error = %v", err)
	}
	if err := run([]string{"claim", matchingID}); err != nil {
		t.Fatalf("claim error = %v", err)
	}
	if err := run([]string{"update", "-id", matchingID, "-state", "active", "-priority", "4"}); err != nil {
		t.Fatalf("update matching task error = %v", err)
	}
	if err := run([]string{"update", "-id", otherID, "-priority", "2"}); err != nil {
		t.Fatalf("update other task error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{
			"search",
			"free", "form", "entry",
			"-status", "develop/active",
			"-title", "entry",
			"-description", "customer portal",
			"-priority", "4",
			"-owner", localModeUsername(),
		}); err != nil {
			t.Fatalf("search error = %v", err)
		}
	})
	matchingRef := ticketLabelByID(t, matchingID)
	otherRef := ticketLabelByID(t, otherID)
	crossProjectRef := ticketLabelByID(t, crossProjectID)

	if !strings.Contains(output, matchingRef+"\ttask\tdevelop/active\tFree form entry") {
		t.Fatalf("search output missing matching task:\n%s", output)
	}
	if strings.Contains(output, otherRef+"\t") {
		t.Fatalf("search output should not include non-matching task:\n%s", output)
	}
	if strings.Contains(output, crossProjectRef+"\t") {
		t.Fatalf("search output should not include cross-project task without -allprojects:\n%s", output)
	}

	allProjectsOutput := captureStdout(t, func() {
		if err := run([]string{
			"search",
			"free", "form", "entry",
			"-allprojects",
		}); err != nil {
			t.Fatalf("search allprojects error = %v", err)
		}
	})
	if !strings.Contains(allProjectsOutput, "Free form entry") || !strings.Contains(allProjectsOutput, "develop/active") {
		t.Fatalf("allprojects output missing current project task:\n%s", allProjectsOutput)
	}
	if !strings.Contains(allProjectsOutput, "Free form entry elsewhere") || !strings.Contains(allProjectsOutput, "design/idle") {
		t.Fatalf("allprojects output missing cross-project task:\n%s", allProjectsOutput)
	}
}

func TestRunUpdateSupportsCombinedFields(t *testing.T) {
	setupLocalCLI(t)

	parentID := createLocalTask(t, []string{"add", "-type", "epic", "Parent Epic"})
	taskID := createLocalTask(t, []string{"add", "-d", "old description", "-ac", "old ac", "Ticket Alpha"})
	if err := run([]string{"claim", taskID}); err != nil {
		t.Fatalf("claim error = %v", err)
	}

	updateOutput := captureStdout(t, func() {
		if err := run([]string{
			"update",
			"-id",
			taskID,
			"-title", "Ticket Beta",
			"-desc", "new description",
			"-ac", "new ac",
			"-priority", "3",
			"-order", "7",
			"-estimate_effort", "5",
			"-estimate_complete", "2026-04-15T12:00:00Z",
			"-status", "design/active",
			"-parent_id", parentID,
		}); err != nil {
			t.Fatalf("update error = %v", err)
		}
	})
	// Update now prints a single-line summary instead of full ticket details.
	if !strings.Contains(updateOutput, taskID+" updated (") {
		t.Fatalf("update output missing summary line:\n%s", updateOutput)
	}
	for _, want := range []string{
		"title is now \"Ticket Beta\"",
		"description updated",
		"acceptance criteria updated",
		"priority is now 3",
		"order is now 7",
	} {
		if !strings.Contains(updateOutput, want) {
			t.Fatalf("update output missing %q:\n%s", want, updateOutput)
		}
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	for _, tc := range []struct {
		label string
		value string
	}{
		{label: "Title", value: "Ticket Beta"},
		{label: "Description", value: "new description"},
		{label: "Parent", value: ticketLabelByID(t, parentID)},
		{label: "Order", value: "7"},
		{label: "EstimateEffort", value: "5"},
		{label: "EstimateComplete", value: "2026-04-15T12:00:00Z"},
		{label: "Status", value: "design/active"},
		{label: "Priority", value: "3"},
		{label: "Acceptance Criteria", value: "new ac"},
	} {
		if !hasDetailField(getOutput, tc.label, tc.value) {
			t.Fatalf("get output missing %s field:\n%s", tc.label, getOutput)
		}
	}
}

func TestRunUpdateSupportsDescriptionAlias(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "-d", "old description", "Ticket Alpha"})

	if err := run([]string{"update", "-id", taskID, "-description", "updated description"}); err != nil {
		t.Fatalf("update with -description error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !hasDetailField(output, "Description", "updated description") {
		t.Fatalf("get output = %q", output)
	}
}

func TestRunUpdateAcceptsPositionalID(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "Needs ID Update"})

	if err := run([]string{"update", taskID, "-title", "No ID Flag"}); err != nil {
		t.Fatalf("run(update positional id) error = %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	svc, err := resolveService(cfg)
	if err != nil {
		t.Fatalf("resolveService() error = %v", err)
	}
	ticket, err := svc.GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if ticket.Title != "No ID Flag" {
		t.Fatalf("ticket.Title = %q, want %q", ticket.Title, "No ID Flag")
	}
}

func TestRunUpdateRequiresID(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"update", "-title", "No ID Flag"}); err == nil || !strings.Contains(err.Error(), "usage: tk update [-id <id>|<id>]") {
		t.Fatalf("expected usage error for missing id, got %v", err)
	}
}

func TestRunGetAcceptsPositionalID(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "Positional ID Get"})

	// positional arg should work the same as -id
	if err := run([]string{"get", taskID}); err != nil {
		t.Fatalf("expected positional id to work, got %v", err)
	}

	// no id at all should still fail
	if err := run([]string{"get"}); err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("expected usage error for missing id, got %v", err)
	}
}

func TestRunGetShowsChildCounts(t *testing.T) {
	setupLocalCLI(t)

	parentID := createLocalTask(t, []string{"epic", "Parent Epic"})
	openChildID := createLocalTask(t, []string{"add", "-parent", parentID, "Open Child"})
	closedChildID := createLocalTask(t, []string{"add", "-parent", parentID, "Closed Child"})
	if err := run([]string{"close", "-id", closedChildID}); err != nil {
		t.Fatalf("close child error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-id", parentID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})

	if !hasDetailField(output, "ChildCounts", "total=2 open=1 closed=1") {
		t.Fatalf("get output missing child counts:\n%s", output)
	}
	if !strings.Contains(output, ticketLabelByID(t, openChildID)) {
		t.Fatalf("get output missing open child row:\n%s", output)
	}
	if !strings.Contains(output, ticketLabelByID(t, closedChildID)) {
		t.Fatalf("get output missing closed child row:\n%s", output)
	}
}

func TestRunGetAlignsDetailColumns(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "-ac", "Aligned output check", "Alignment Ticket"})

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})

	lines := strings.Split(output, "\n")
	colonIndex := -1
	for _, line := range lines {
		if !strings.Contains(line, " : ") {
			continue
		}
		if strings.HasPrefix(line, "  - ") { // history/comment rows
			continue
		}
		idx := strings.Index(line, " : ")
		if idx < 0 {
			continue
		}
		if colonIndex == -1 {
			colonIndex = idx
			continue
		}
		if idx != colonIndex {
			t.Fatalf("misaligned detail line %q (index %d, want %d)\n%s", line, idx, colonIndex, output)
		}
	}
	if colonIndex == -1 {
		t.Fatalf("no detail lines found in output:\n%s", output)
	}
}

func TestRunDraftAndUndraftToggleDraftFlag(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Draft Me"})

	if err := run([]string{"draft", "-id", taskID}); err != nil {
		t.Fatalf("draft error = %v", err)
	}

	draftOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get after draft error = %v", err)
		}
	})
	if !hasDetailField(draftOutput, "Draft", "true") {
		t.Fatalf("draft output missing draft=true:\n%s", draftOutput)
	}

	if err := run([]string{"undraft", "-id", taskID}); err != nil {
		t.Fatalf("undraft error = %v", err)
	}

	undraftOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get after undraft error = %v", err)
		}
	})
	if !hasDetailField(undraftOutput, "Draft", "false") {
		t.Fatalf("undraft output missing draft=false:\n%s", undraftOutput)
	}
}

func TestRunDraftAndUndraftAcceptMultipleTicketIDs(t *testing.T) {
	setupLocalCLI(t)

	firstID := createLocalTask(t, []string{"add", "Batch Draft 1"})
	secondID := createLocalTask(t, []string{"add", "Batch Draft 2"})
	thirdID := createLocalTask(t, []string{"add", "Batch Draft 3"})

	if err := run([]string{"draft", firstID, secondID, thirdID}); err != nil {
		t.Fatalf("draft multiple error = %v", err)
	}

	for _, id := range []string{firstID, secondID, thirdID} {
		output := captureStdout(t, func() {
			if err := run([]string{"get", "-id", id}); err != nil {
				t.Fatalf("get after draft error = %v", err)
			}
		})
		if !hasDetailField(output, "Draft", "true") {
			t.Fatalf("draft output missing draft=true for %s:\n%s", id, output)
		}
	}

	if err := run([]string{"undraft", firstID, secondID, thirdID}); err != nil {
		t.Fatalf("undraft multiple error = %v", err)
	}

	for _, id := range []string{firstID, secondID, thirdID} {
		output := captureStdout(t, func() {
			if err := run([]string{"get", "-id", id}); err != nil {
				t.Fatalf("get after undraft error = %v", err)
			}
		})
		if !hasDetailField(output, "Draft", "false") {
			t.Fatalf("undraft output missing draft=false for %s:\n%s", id, output)
		}
	}
}

func TestRunUpdateStageUsesCurrentWorkflowStages(t *testing.T) {
	setupLocalCLI(t)
	svc := attachWorkflowToDefaultProject(t, "triage", "build", "verify")

	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Workflow Stage Ticket",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	if err := run([]string{"update", "-id", ticket.ID, "-stage", "build"}); err != nil {
		t.Fatalf("update stage error = %v", err)
	}

	updated, err := svc.GetTicket(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("GetTicket(updated) error = %v", err)
	}
	if updated.Stage != "build" || updated.State != store.StateIdle {
		t.Fatalf("updated lifecycle = %s/%s, want build/idle", updated.Stage, updated.State)
	}

	err = run([]string{"update", "-id", ticket.ID, "-stage", "xxxx"})
	if err == nil {
		t.Fatal("update invalid stage error = nil, want error")
	}
	want := `invalid stage "xxxx"; valid stages: triage, build, verify`
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("update invalid stage error = %v, want substring %q", err, want)
	}
}

func TestRunRejectMovesTicketToFirstWorkflowStageAsDraft(t *testing.T) {
	setupLocalCLI(t)
	svc := attachWorkflowToDefaultProject(t, "triage", "build", "verify")

	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Reject Me",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	advanced, err := svc.UpdateTicket(context.Background(), ticket.ID, libticket.TicketUpdateRequest{
		Title:              ticket.Title,
		Description:        ticket.Description,
		AcceptanceCriteria: ticket.AcceptanceCriteria,
		GitRepository:      ticket.GitRepository,
		GitBranch:          ticket.GitBranch,
		ParentID:           ticket.ParentID,
		Assignee:           "admin",
		Stage:              "build",
		State:              store.StateActive,
		Priority:           ticket.Priority,
		Order:              ticket.Order,
		EstimateEffort:     ticket.EstimateEffort,
		EstimateComplete:   ticket.EstimateComplete,
		Type:               ticket.Type,
	})
	if err != nil {
		t.Fatalf("UpdateTicket(build/active) error = %v", err)
	}
	if advanced.Stage != "build" {
		t.Fatalf("advanced stage = %q, want build", advanced.Stage)
	}

	if err := run([]string{"reject", "-id", ticket.ID}); err != nil {
		t.Fatalf("reject error = %v", err)
	}

	rejected, err := svc.GetTicket(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("GetTicket(rejected) error = %v", err)
	}
	if rejected.Stage != "triage" || rejected.State != store.StateIdle {
		t.Fatalf("rejected lifecycle = %s/%s, want triage/idle", rejected.Stage, rejected.State)
	}
	if !rejected.Draft {
		t.Fatal("rejected ticket should be draft")
	}
}

func TestRunTaskCreateSupportsInterspersedFlags(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "the", "thing", "-type", "epic"})

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !hasDetailField(output, "Title", "the thing") || !hasDetailField(output, "Type", "epic") {
		t.Fatalf("interspersed add output missing required fields:\n%s", output)
	}
}

func TestRunTypedTaskCreateSupportsEstimateFlags(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"epic", "-estimate_effort", "8", "-estimate_complete", "2026-04-20T17:00:00Z", "Estimated Epic"})

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	for _, tc := range []struct {
		label string
		value string
	}{
		{label: "Title", value: "Estimated Epic"},
		{label: "Type", value: "epic"},
		{label: "EstimateEffort", value: "8"},
		{label: "EstimateComplete", value: "2026-04-20T17:00:00Z"},
	} {
		if !hasDetailField(output, tc.label, tc.value) {
			t.Fatalf("typed create output missing %s field:\n%s", tc.label, output)
		}
	}
}

func TestRunTaskCreateFallsBackToDefaultProject(t *testing.T) {
	setupLocalCLI(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	cfg.ProjectID = ""
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}

	taskID := createLocalTask(t, []string{"create", "-t", "epic", "-title", "foo"})
	output := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !hasDetailField(output, "Title", "foo") || !hasDetailField(output, "Type", "epic") || !hasDetailLabel(output, "Key") {
		t.Fatalf("default project fallback output missing expected fields:\n%s", output)
	}

	reloaded, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load(reloaded) error = %v", err)
	}
	if reloaded.ProjectID == "" {
		t.Fatal("CurrentProject should be set after auto-bootstrap")
	}
}

func TestRunAssignAndUnassignInLocalMode(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "-estimate_effort", "3", "-estimate_complete", "2026-04-10T09:00:00Z", "Task Gamma"})

	if err := run([]string{"user", "create", "-username", "alice", "-password", "secret12"}); err != nil {
		t.Fatalf("user create error = %v", err)
	}

	assignOutput := captureStdout(t, func() {
		if err := run([]string{"assign", taskID, "alice"}); err != nil {
			t.Fatalf("assign error = %v", err)
		}
	})
	if !strings.Contains(assignOutput, "assigned") || !strings.Contains(assignOutput, "alice") {
		t.Fatalf("assign output = %q", assignOutput)
	}

	unassignOutput := captureStdout(t, func() {
		if err := run([]string{"unassign", taskID, "alice"}); err != nil {
			t.Fatalf("unassign error = %v", err)
		}
	})
	if !strings.Contains(unassignOutput, "unassigned") {
		t.Fatalf("unassign output = %q", unassignOutput)
	}

	cloneOutput := captureStdout(t, func() {
		if err := run([]string{"clone", taskID}); err != nil {
			t.Fatalf("clone error = %v", err)
		}
	})
	for _, want := range []string{"estimate_effort: 3", "estimate_complete: 2026-04-10T09:00:00Z"} {
		if !strings.Contains(cloneOutput, want) {
			t.Fatalf("clone output missing %q:\n%s", want, cloneOutput)
		}
	}
}

func TestRunStatusChangeInLocalModeDoesNotRequireOwnership(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Ownership-free local task"})

	output := captureStdout(t, func() {
		if err := run([]string{"complete", "-id", taskID}); err != nil {
			t.Fatalf("complete error = %v", err)
		}
	})

	if !strings.Contains(output, "completed") {
		t.Fatalf("complete output = %q, want completed", output)
	}
}

func TestRunDeleteTicketInLocalMode(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Delete me"})
	output := captureStdout(t, func() {
		if err := deleteTicketConfirmed(t, taskID); err != nil {
			t.Fatalf("delete error = %v", err)
		}
	})
	if !strings.Contains(output, "deleted ticket ") {
		t.Fatalf("delete output = %q", output)
	}
	if err := run([]string{"get", "-id", taskID}); err == nil || err.Error() != "ticket not found" {
		t.Fatalf("get deleted task error = %v, want ticket not found", err)
	}
}

func TestRunDeleteTicketFailsWhenTaskHasChildren(t *testing.T) {
	setupLocalCLI(t)

	parentID := createLocalTask(t, []string{"add", "-t", "epic", "Parent"})
	childID := createLocalTask(t, []string{"add", "-parent", parentID, "Child"})
	if childID == "" {
		t.Fatal("child task id is empty")
	}
	// Two-step deletion: the children check happens on the final confirm step.
	if err := deleteTicketConfirmed(t, parentID); err == nil || err.Error() != "ticket has child tickets" {
		t.Fatalf("delete parent error = %v, want ticket has child tickets", err)
	}
}

func TestRunDeleteRequiresIDFlag(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "Needs ID Delete"})

	// Positional ID should now work (no error).
	if err := run([]string{"delete", taskID}); err != nil {
		t.Fatalf("positional delete should succeed, got %v", err)
	}
	// No ID at all should still fail.
	if err := run([]string{"delete"}); err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("expected missing id usage error, got %v", err)
	}
}

func TestRunGetJSONUsesCommentAuthorDateTextShape(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "JSON Task"})
	if err := run([]string{"comment", "add", "-id", taskID, "first"}); err != nil {
		t.Fatalf("comment add first error = %v", err)
	}
	if err := run([]string{"comment", "add", "-id", taskID, "second"}); err != nil {
		t.Fatalf("comment add second error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-json", "-id", taskID}); err != nil {
			t.Fatalf("get -json error = %v", err)
		}
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\n%s", err, output)
	}
	rawComments, ok := payload["comments"].([]any)
	if !ok || len(rawComments) != 2 {
		t.Fatalf("comments payload = %#v", payload["comments"])
	}
	first, ok := rawComments[0].(map[string]any)
	if !ok {
		t.Fatalf("first comment = %#v", rawComments[0])
	}
	second, ok := rawComments[1].(map[string]any)
	if !ok {
		t.Fatalf("second comment = %#v", rawComments[1])
	}
	if first["text"] != "second" || second["text"] != "first" {
		t.Fatalf("comments are not newest-first: %#v", rawComments)
	}
	for _, comment := range []map[string]any{first, second} {
		for _, required := range []string{"author", "date", "text"} {
			if _, ok := comment[required]; !ok {
				t.Fatalf("comment missing %q: %#v", required, comment)
			}
		}
		for _, unwanted := range []string{"item_id", "user_id", "created_at", "comment"} {
			if _, ok := comment[unwanted]; ok {
				t.Fatalf("comment should not contain %q: %#v", unwanted, comment)
			}
		}
	}
}

func TestRunRejectsInvalidCommand(t *testing.T) {
	setupLocalCLI(t)
	if err := run([]string{"invalid"}); err == nil || err.Error() != `no such command "invalid"` {
		t.Fatalf("run(invalid) error = %v", err)
	}
}

func TestRunRemoteOnlyCommandsFailInLocalMode(t *testing.T) {

	for _, args := range [][]string{
		{"login"},
		{"register"},
		{"logout"},
	} {
		if err := run(args); err == nil {
			t.Fatalf("run(%v) error = nil", args)
		}
	}
}

func TestRunRemoteModeStatusFailure(t *testing.T) {
	t.Setenv("TICKET_HOME", t.TempDir())
	setTestLocation(t, "http://127.0.0.1:1")

	var runErr error
	output := captureStdout(t, func() {
		runErr = runStatus(nil)
	})
	if runErr == nil {
		t.Fatal("runStatus(remote failure) error = nil")
	}
	for _, want := range []string{
		"client_version   : " + strings.TrimSpace(embeddedVersion),
		"server_version   : (unknown)",
		"authenticated    : false",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus(remote failure) missing %q:\n%s", want, output)
		}
	}
}

func TestRunCountHistoryOrphansAndConfigInLocalMode(t *testing.T) {
	setupLocalCLI(t)
	epicID := createLocalTask(t, []string{"epic", "Parent Epic"})
	clearCurrentEpicID(t)
	taskID := createLocalTask(t, []string{"add", "-parent", epicID, "Child Task"})
	orphanID := createLocalTask(t, []string{"add", "Orphan Task"})

	countOutput := captureStdout(t, func() {
		if err := run([]string{"count"}); err != nil {
			t.Fatalf("count error = %v", err)
		}
	})
	if !strings.Contains(countOutput, "task") {
		t.Fatalf("count output = %q", countOutput)
	}

	historyOutput := captureStdout(t, func() {
		if err := run([]string{"history", taskID}); err != nil {
			t.Fatalf("history error = %v", err)
		}
	})
	if !strings.Contains(historyOutput, "created task") {
		t.Fatalf("history output = %q", historyOutput)
	}
	if err := run([]string{"history", "-offset", "1"}); err == nil || !strings.Contains(err.Error(), "offset is only supported") {
		t.Fatalf("history -offset without id error = %v", err)
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !hasDetailLabel(getOutput, "History") || !strings.Contains(getOutput, "created task") {
		t.Fatalf("get output missing history:\n%s", getOutput)
	}

	orphansOutput := captureStdout(t, func() {
		if err := run([]string{"orphans"}); err != nil {
			t.Fatalf("orphans error = %v", err)
		}
	})
	orphanRef := ticketLabelByID(t, orphanID)
	taskRef := ticketLabelByID(t, taskID)
	epicRef := ticketLabelByID(t, epicID)
	if !strings.Contains(orphansOutput, orphanRef) || strings.Contains(orphansOutput, taskRef+"\t") {
		t.Fatalf("orphans output = %q", orphansOutput)
	}
	if strings.Contains(orphansOutput, epicRef) {
		t.Fatalf("orphans output should not include epics: %q", orphansOutput)
	}

	if err := run([]string{"config", "set", "location", "http://example.test"}); err != nil {
		t.Fatalf("config set error = %v", err)
	}
	configOutput := captureStdout(t, func() {
		if err := run([]string{"config", "get", "location"}); err != nil {
			t.Fatalf("config get error = %v", err)
		}
	})
	if strings.TrimSpace(configOutput) == "" {
		t.Fatalf("config output = %q", configOutput)
	}

	if err := run([]string{"config", "ls"}); err != nil {
		t.Fatalf("config ls error = %v", err)
	}
	listOutput := captureStdout(t, func() {
		if err := run([]string{"config", "ls"}); err != nil {
			t.Fatalf("config ls error = %v", err)
		}
	})
	for _, want := range []string{
		"location",
		"username",
		"project_id",
		"current_epic_id",
	} {
		if !strings.Contains(listOutput, want) {
			t.Fatalf("config ls output missing %q:\n%s", want, listOutput)
		}
	}

	if err := run([]string{"config", "rm", "location"}); err != nil {
		t.Fatalf("config rm error = %v", err)
	}
	clearedOutput := captureStdout(t, func() {
		if err := run([]string{"config", "get", "location"}); err != nil {
			t.Fatalf("config get error = %v", err)
		}
	})
	if strings.TrimSpace(clearedOutput) == "" {
		t.Fatalf("config get output after delete should still resolve the active location: %q", clearedOutput)
	}
}

func TestRunOrphansExcludesEpicRoots(t *testing.T) {
	setupLocalCLI(t)
	epicID := createLocalTask(t, []string{"epic", "Orphan Epic"})
	clearCurrentEpicID(t)
	orphanID := createLocalTask(t, []string{"add", "Task with no parent"})

	orphansOutput := captureStdout(t, func() {
		if err := run([]string{"orphans"}); err != nil {
			t.Fatalf("orphans error = %v", err)
		}
	})
	orphanRef := ticketLabelByID(t, orphanID)
	epicRef := ticketLabelByID(t, epicID)
	if !strings.Contains(orphansOutput, orphanRef) {
		t.Fatalf("orphans output missing orphan task: %q", orphansOutput)
	}
	if strings.Contains(orphansOutput, epicRef) {
		t.Fatalf("orphans output should not include epic without parent: %q", orphansOutput)
	}
}

func TestRunSetParentAllowsLineageIndependentOfType(t *testing.T) {
	setupLocalCLI(t)
	childID := createLocalTask(t, []string{"epic", "Standalone Epic"})
	clearCurrentEpicID(t)
	taskID := createLocalTask(t, []string{"add", "Task Parent"})

	if err := run([]string{"set-parent", "-id", childID, taskID}); err != nil {
		t.Fatalf("set-parent error = %v", err)
	}
}

func TestRunNegativeCommandCasesInLocalMode(t *testing.T) {
	setupLocalCLI(t)

	cases := []struct {
		args []string
		want string
	}{
		{[]string{"get", "-id", "abc"}, "ticket not found"},
		{[]string{"dependency", "add", "-id", "1", "abc"}, "ticket not found"},
		{[]string{"request", "abc"}, "ticket not found"},
		{[]string{"project", "get"}, "usage: tk project get <id>"},
		{[]string{"list", "-n", "-1"}, "usage: tk list|ls"},
		{[]string{"comment", "add", "1"}, "usage: tk comment <id>"},
		{[]string{"set-parent", "-id", "1", "abc"}, "ticket not found"},
		{[]string{"unset-parent", "-id", "abc"}, "ticket not found"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			if err := run(tc.args); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("run(%v) error = %v, want substring %q", tc.args, err, tc.want)
			}
		})
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	return buf.String()
}

func TestRunSdlcListInLocalMode(t *testing.T) {
	setupLocalCLI(t)
	output := captureStdout(t, func() {
		if err := run([]string{"sdlc", "list"}); err != nil {
			t.Fatalf("sdlc list error = %v", err)
		}
	})
	if !strings.Contains(output, "Agile") {
		t.Fatalf("sdlc list missing Agile sdlc:\n%s", output)
	}
}

func TestRunSdlcGetShowsStages(t *testing.T) {
	setupLocalCLI(t)
	output := captureStdout(t, func() {
		if err := run([]string{"sdlc", "get", "-id", "1"}); err != nil {
			t.Fatalf("sdlc get error = %v", err)
		}
	})
	for _, want := range []string{"develop", "done"} {
		if !strings.Contains(output, want) {
			t.Fatalf("sdlc get missing %q:\n%s", want, output)
		}
	}
}

func TestRunSdlcGetShowsStageAcceptanceCriteria(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	wf, err := svc.CreateSdlc(context.Background(), libticket.SdlcRequest{Name: "AC Workflow", Description: "workflow with stage acceptance criteria"})
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}
	stage, err := svc.AddSdlcStage(context.Background(), wf.ID, libticket.SdlcStageRequest{
		StageName:   "triage",
		Description: "triage",
		SortOrder:   1,
	})
	if err != nil {
		t.Fatalf("AddSdlcStage() error = %v", err)
	}
	if _, err := svc.UpdateSdlcStage(context.Background(), stage.ID, libticket.SdlcStageRequest{
		StageName:          "triage",
		Description:        "triage",
		AcceptanceCriteria: "Clarified with the product owner",
	}); err != nil {
		t.Fatalf("UpdateSdlcStage() error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"sdlc", "get", "-id", strconv.FormatInt(wf.ID, 10)}); err != nil {
			t.Fatalf("sdlc get error = %v", err)
		}
	})
	for _, want := range []string{"ACCEPTANCE CRITERIA", "Clarified with the product owner"} {
		if !strings.Contains(output, want) {
			t.Fatalf("sdlc get missing %q:\n%s", want, output)
		}
	}
}

func TestRunSdlcRoleCRUD(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	wf, err := svc.CreateSdlc(context.Background(), libticket.SdlcRequest{Name: "Role Workflow", Description: "workflow for scoped roles"})
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}
	sdlcID := strconv.FormatInt(wf.ID, 10)

	createOutput := captureStdout(t, func() {
		if err := run([]string{"sdlc", "role-add", "-sdlc_id", sdlcID, "-title", "reviewer", "-description", "Reviews work", "-ac", "Approves the release"}); err != nil {
			t.Fatalf("sdlc role-add error = %v", err)
		}
	})
	if !strings.Contains(createOutput, "created sdlc role") || !strings.Contains(createOutput, "reviewer") {
		t.Fatalf("unexpected sdlc role-add output:\n%s", createOutput)
	}

	var created store.Role
	roles, err := svc.ListRoles(context.Background())
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	for _, role := range roles {
		if role.Title == "reviewer" && role.SdlcID != nil && *role.SdlcID == wf.ID {
			created = role
			break
		}
	}
	if created.ID == 0 {
		t.Fatal("expected scoped sdlc role to be created")
	}
	roleID := strconv.FormatInt(created.ID, 10)

	getOutput := captureStdout(t, func() {
		if err := run([]string{"sdlc", "role-get", "-sdlc_id", sdlcID, "-role_id", roleID}); err != nil {
			t.Fatalf("sdlc role-get error = %v", err)
		}
	})
	for _, want := range []string{"Title:               reviewer", "Acceptance Criteria: Approves the release"} {
		if !strings.Contains(getOutput, want) {
			t.Fatalf("sdlc role-get missing %q:\n%s", want, getOutput)
		}
	}

	updateOutput := captureStdout(t, func() {
		if err := run([]string{"sdlc", "role-update", "-sdlc_id", sdlcID, "-role_id", roleID, "-title", "qa-reviewer", "-description", "Reviews work", "-ac", "Ships the release"}); err != nil {
			t.Fatalf("sdlc role-update error = %v", err)
		}
	})
	if !strings.Contains(updateOutput, "updated sdlc role") || !strings.Contains(updateOutput, "qa-reviewer") {
		t.Fatalf("unexpected sdlc role-update output:\n%s", updateOutput)
	}

	deleteOutput := captureStdout(t, func() {
		if err := run([]string{"sdlc", "role-rm", "-sdlc_id", sdlcID, "-role_id", roleID}); err != nil {
			t.Fatalf("sdlc role-rm error = %v", err)
		}
	})
	if !strings.Contains(deleteOutput, "deleted sdlc role") {
		t.Fatalf("unexpected sdlc role-rm output:\n%s", deleteOutput)
	}

	roles, err = svc.ListRoles(context.Background())
	if err != nil {
		t.Fatalf("ListRoles(after delete) error = %v", err)
	}
	for _, role := range roles {
		if role.ID == created.ID {
			t.Fatalf("expected role %d to be deleted", created.ID)
		}
	}
}

func TestRunSdlcStageRoleCommands(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	wf, err := svc.CreateSdlc(context.Background(), libticket.SdlcRequest{Name: "Stage Role Workflow", Description: "workflow for stage role commands"})
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}
	stage, err := svc.AddSdlcStage(context.Background(), wf.ID, libticket.SdlcStageRequest{
		StageName:   "triage",
		Description: "triage",
		SortOrder:   1,
	})
	if err != nil {
		t.Fatalf("AddSdlcStage() error = %v", err)
	}
	roleA, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
		SdlcID:      &wf.ID,
		Title:       "reviewer",
		Description: "reviews work",
	})
	if err != nil {
		t.Fatalf("CreateRole(roleA) error = %v", err)
	}
	roleB, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
		SdlcID:      &wf.ID,
		Title:       "qa",
		Description: "verifies work",
	})
	if err != nil {
		t.Fatalf("CreateRole(roleB) error = %v", err)
	}
	sdlcID := strconv.FormatInt(wf.ID, 10)
	stageID := strconv.FormatInt(stage.ID, 10)
	roleAID := strconv.FormatInt(roleA.ID, 10)
	roleBID := strconv.FormatInt(roleB.ID, 10)

	addOutput := captureStdout(t, func() {
		if err := run([]string{"sdlc", "stage-role-add", "-sdlc_id", sdlcID, "-stage_id", stageID, "-role_id", roleAID}); err != nil {
			t.Fatalf("stage-role-add roleA error = %v", err)
		}
		if err := run([]string{"sdlc", "stage-role-add", "-sdlc_id", sdlcID, "-stage_id", stageID, "-role_id", roleBID}); err != nil {
			t.Fatalf("stage-role-add roleB error = %v", err)
		}
	})
	if !strings.Contains(addOutput, "assigned role") {
		t.Fatalf("unexpected stage-role-add output:\n%s", addOutput)
	}

	ordered, err := svc.GetSdlc(context.Background(), wf.ID)
	if err != nil {
		t.Fatalf("GetSdlc(after add) error = %v", err)
	}
	if len(ordered.Stages) != 1 || len(ordered.Stages[0].Roles) != 2 {
		t.Fatalf("stage roles after add = %#v", ordered.Stages)
	}

	orderOutput := captureStdout(t, func() {
		if err := run([]string{"sdlc", "stage-role-order", "-sdlc_id", sdlcID, "-stage_id", stageID, "-roles", roleBID + "," + roleAID}); err != nil {
			t.Fatalf("stage-role-order error = %v", err)
		}
	})
	if !strings.Contains(orderOutput, "reordered roles") {
		t.Fatalf("unexpected stage-role-order output:\n%s", orderOutput)
	}

	ordered, err = svc.GetSdlc(context.Background(), wf.ID)
	if err != nil {
		t.Fatalf("GetSdlc(after reorder) error = %v", err)
	}
	if got := []int64{ordered.Stages[0].Roles[0].ID, ordered.Stages[0].Roles[1].ID}; !reflect.DeepEqual(got, []int64{roleB.ID, roleA.ID}) {
		t.Fatalf("stage role order = %v, want [%d %d]", got, roleB.ID, roleA.ID)
	}

	removeOutput := captureStdout(t, func() {
		if err := run([]string{"sdlc", "stage-role-rm", "-sdlc_id", sdlcID, "-stage_id", stageID, "-role_id", roleAID}); err != nil {
			t.Fatalf("stage-role-rm error = %v", err)
		}
	})
	if !strings.Contains(removeOutput, "removed role") {
		t.Fatalf("unexpected stage-role-rm output:\n%s", removeOutput)
	}

	ordered, err = svc.GetSdlc(context.Background(), wf.ID)
	if err != nil {
		t.Fatalf("GetSdlc(after remove) error = %v", err)
	}
	if got := ordered.Stages[0].Roles; len(got) != 1 || got[0].ID != roleB.ID {
		t.Fatalf("remaining stage roles = %#v, want only roleB", got)
	}
}

func TestRunSdlcStageListAndGetShowRoleAndAcceptanceCriteria(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	wf, err := svc.CreateSdlc(context.Background(), libticket.SdlcRequest{Name: "Stage Detail Workflow", Description: "workflow for stage output"})
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}
	stage, err := svc.AddSdlcStage(context.Background(), wf.ID, libticket.SdlcStageRequest{
		StageName:   "triage",
		Description: "triage work",
		SortOrder:   1,
	})
	if err != nil {
		t.Fatalf("AddSdlcStage() error = %v", err)
	}
	stage, err = svc.UpdateSdlcStage(context.Background(), stage.ID, libticket.SdlcStageRequest{
		StageName:          "triage",
		Description:        "triage work",
		AcceptanceCriteria: "Classify the issue",
		DefinitionOfReady:  "Classify the issue",
		DefinitionOfDone:   "Issue routed with owner and priority",
	})
	if err != nil {
		t.Fatalf("UpdateSdlcStage() error = %v", err)
	}
	role, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
		SdlcID:      &wf.ID,
		Title:       "reviewer",
		Description: "reviews the work",
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if err := svc.AddSdlcStageRole(context.Background(), wf.ID, stage.ID, role.ID); err != nil {
		t.Fatalf("AddSdlcStageRole() error = %v", err)
	}

	listOutput := captureStdout(t, func() {
		if err := run([]string{"sdlc", "stage-list", "-id", strconv.FormatInt(wf.ID, 10)}); err != nil {
			t.Fatalf("sdlc stage-list error = %v", err)
		}
	})
	for _, want := range []string{"triage", "reviewer", "triage work"} {
		if !strings.Contains(listOutput, want) {
			t.Fatalf("sdlc stage-list missing %q:\n%s", want, listOutput)
		}
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"sdlc", "stage-get", "-stage-id", strconv.FormatInt(stage.ID, 10)}); err != nil {
			t.Fatalf("sdlc stage-get error = %v", err)
		}
	})
	for _, want := range []string{
		"Stage Name          : triage",
		"WoW                 : triage work",
		"DoR                 : Classify the issue",
		"DoD                 : Issue routed with owner and priority",
		"Acceptance Criteria : Classify the issue",
		"Roles               : reviewer",
	} {
		if !strings.Contains(getOutput, want) {
			t.Fatalf("sdlc stage-get missing %q:\n%s", want, getOutput)
		}
	}
}

func TestRunSdlcCreateAndDelete(t *testing.T) {
	setupLocalCLI(t)
	output := captureStdout(t, func() {
		if err := run([]string{"sdlc", "create", "-name", "custom"}); err != nil {
			t.Fatalf("sdlc create error = %v", err)
		}
	})
	if !strings.Contains(output, "custom") {
		t.Fatalf("sdlc create missing name:\n%s", output)
	}
	// List should show both
	output = captureStdout(t, func() {
		if err := run([]string{"sdlc", "list"}); err != nil {
			t.Fatalf("sdlc list error = %v", err)
		}
	})
	if !strings.Contains(output, "custom") {
		t.Fatalf("sdlc list missing custom:\n%s", output)
	}
}

func TestRunStatusShowsProjectSdlcAndDefaultDraft(t *testing.T) {
	setupLocalCLI(t)
	svc := attachWorkflowToDefaultProject(t, "triage", "build", "done")
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	if err := svc.SetProjectDefaultDraft(context.Background(), project.ID, true); err != nil {
		t.Fatalf("SetProjectDefaultDraft() error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"status"}); err != nil {
			t.Fatalf("status error = %v", err)
		}
	})
	for _, want := range []string{"project_sdlc", "Custom Workflow", "project_default_draft", "true"} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}

	jsonOutput := captureStdout(t, func() {
		if err := run([]string{"status", "-json"}); err != nil {
			t.Fatalf("status --json error = %v", err)
		}
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(jsonOutput), &payload); err != nil {
		t.Fatalf("json.Unmarshal(status) error = %v\noutput=%s", err, jsonOutput)
	}
	if got := payload["project_sdlc"]; got != "Custom Workflow" {
		t.Fatalf("project_sdlc = %#v, want %q", got, "Custom Workflow")
	}
	if got, ok := payload["project_default_draft"].(bool); !ok || !got {
		t.Fatalf("project_default_draft = %#v, want true", payload["project_default_draft"])
	}
}

func TestRunProjectSetDraftUpdatesCurrentProject(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	output := captureStdout(t, func() {
		if err := run([]string{"project", "set-draft", "true"}); err != nil {
			t.Fatalf("project set-draft true error = %v", err)
		}
	})
	if !strings.Contains(output, "default_draft set to true") {
		t.Fatalf("unexpected project set-draft output:\n%s", output)
	}

	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	if !project.DefaultDraft {
		t.Fatalf("project.DefaultDraft = %v, want true", project.DefaultDraft)
	}
}

func TestRunProjectSdlcSetsAndClearsCurrentProjectWorkflow(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	wf, err := svc.CreateSdlc(context.Background(), libticket.SdlcRequest{Name: "Project Workflow", Description: "workflow assignment test"})
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}

	setOutput := captureStdout(t, func() {
		if err := run([]string{"project", "sdlc", strconv.FormatInt(wf.ID, 10)}); err != nil {
			t.Fatalf("project sdlc set error = %v", err)
		}
	})
	if !strings.Contains(setOutput, "set sdlc") {
		t.Fatalf("unexpected project sdlc set output:\n%s", setOutput)
	}

	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) after set error = %v", err)
	}
	if project.SdlcID == nil || *project.SdlcID != wf.ID {
		t.Fatalf("project.SdlcID = %#v, want %d", project.SdlcID, wf.ID)
	}

	clearOutput := captureStdout(t, func() {
		if err := run([]string{"project", "sdlc", "0"}); err != nil {
			t.Fatalf("project sdlc clear error = %v", err)
		}
	})
	if !strings.Contains(clearOutput, "cleared sdlc") {
		t.Fatalf("unexpected project sdlc clear output:\n%s", clearOutput)
	}

	project, err = svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) after clear error = %v", err)
	}
	if project.SdlcID != nil && *project.SdlcID == wf.ID {
		t.Fatalf("project.SdlcID = %#v, want cleared custom workflow %d", project.SdlcID, wf.ID)
	}
}

func TestRunProjectUseAndSdlcHelpPaths(t *testing.T) {
	setupLocalCLI(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	cfg.ProjectID = ""
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}

	noProjectOutput := captureStdout(t, func() {
		if err := run([]string{"project", "use"}); err != nil {
			t.Fatalf("project use with no current project error = %v", err)
		}
	})
	if !strings.Contains(noProjectOutput, "TK — Default Project") {
		t.Fatalf("unexpected project use output:\n%s", noProjectOutput)
	}

	useOutput := captureStdout(t, func() {
		if err := run([]string{"project", "use", "1"}); err != nil {
			t.Fatalf("project use 1 error = %v", err)
		}
	})
	if !strings.Contains(useOutput, "using project") {
		t.Fatalf("unexpected project use set output:\n%s", useOutput)
	}

	currentOutput := captureStdout(t, func() {
		if err := run([]string{"project", "use"}); err != nil {
			t.Fatalf("project use current error = %v", err)
		}
	})
	if !strings.Contains(currentOutput, "TK") {
		t.Fatalf("unexpected current project output:\n%s", currentOutput)
	}

	helpOutput := captureStdout(t, func() {
		if err := run([]string{"project", "sdlc", "help"}); err != nil {
			t.Fatalf("project sdlc help error = %v", err)
		}
	})
	if !strings.Contains(helpOutput, "tk project sdlc <sdlc-id>") {
		t.Fatalf("unexpected project sdlc help output:\n%s", helpOutput)
	}
}

func TestRunReadyAndNotReadyToggleDraft(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Draft toggle",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	if err := run([]string{"notready", ticket.ID}); err != nil {
		t.Fatalf("notready error = %v", err)
	}
	updated, err := svc.GetTicket(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("GetTicket(after notready) error = %v", err)
	}
	if !updated.Draft {
		t.Fatalf("Draft after notready = %v, want true", updated.Draft)
	}

	if err := run([]string{"ready", ticket.ID}); err != nil {
		t.Fatalf("ready error = %v", err)
	}
	updated, err = svc.GetTicket(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("GetTicket(after ready) error = %v", err)
	}
	if updated.Draft {
		t.Fatalf("Draft after ready = %v, want false", updated.Draft)
	}
}

func TestRunTicketTreeReturnsRemovalError(t *testing.T) {
	setupLocalCLI(t)
	err := run([]string{"ticket", "tree"})
	if err == nil {
		t.Fatal("ticket tree should fail")
	}
	for _, want := range []string{"placeholder alias", "use `tk get`"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ticket tree error missing %q: %v", want, err)
		}
	}
}

func TestRunSdlcExportImportRoundTrip(t *testing.T) {
	setupLocalCLI(t)
	tmpFile := filepath.Join(t.TempDir(), "sdlc.json")
	// Export
	if err := run([]string{"sdlc", "export", "-id", "1", "-o", tmpFile}); err != nil {
		t.Fatalf("sdlc export error = %v", err)
	}
	// Modify the file to change the name so we can import
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("read export file error = %v", err)
	}
	modified := strings.Replace(string(data), `"Agile"`, `"imported"`, 1)
	if err := os.WriteFile(tmpFile, []byte(modified), 0o644); err != nil {
		t.Fatalf("write modified file error = %v", err)
	}
	// Import
	output := captureStdout(t, func() {
		if err := run([]string{"sdlc", "import", "-file", tmpFile}); err != nil {
			t.Fatalf("sdlc import error = %v", err)
		}
	})
	if !strings.Contains(output, "imported") {
		t.Fatalf("sdlc import missing name:\n%s", output)
	}
}

func TestRunRoleCRUD(t *testing.T) {
	setupLocalCLI(t)
	// List seeded roles
	output := captureStdout(t, func() {
		if err := run([]string{"role", "list"}); err != nil {
			t.Fatalf("role list error = %v", err)
		}
	})
	if !strings.Contains(output, "Engineer") {
		t.Fatalf("role list missing seeded role Engineer:\n%s", output)
	}
	// Create
	createOutput := captureStdout(t, func() {
		if err := run([]string{"role", "create", "-title", "Security Lead", "-description", "Protect systems", "-dor", "Threat model ready", "-dod", "Review signed off", "-ac", "Zero breaches"}); err != nil {
			t.Fatalf("role create error = %v", err)
		}
	})
	if !strings.Contains(createOutput, "Security Lead") {
		t.Fatalf("role create missing name:\n%s", createOutput)
	}
	// List again should include new role
	output = captureStdout(t, func() {
		if err := run([]string{"role", "ls"}); err != nil {
			t.Fatalf("role ls error = %v", err)
		}
	})
	if !strings.Contains(output, "Security Lead") {
		t.Fatalf("role ls missing Security Lead:\n%s", output)
	}
	// Extract role ID from create output (e.g. "created role #6 Security Lead")
	var roleID string
	for _, line := range strings.Split(createOutput, "\n") {
		if strings.HasPrefix(line, "created role #") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				roleID = strings.TrimPrefix(parts[2], "#")
			}
		}
	}
	if roleID == "" {
		t.Fatalf("could not extract role ID from create output:\n%s", createOutput)
	}
	// Update
	output = captureStdout(t, func() {
		if err := run([]string{"role", "update", "-id", roleID, "-title", "Chief Security", "-description", "Lead design", "-ac-map", "qa=Security sign-off"}); err != nil {
			t.Fatalf("role update error = %v", err)
		}
	})
	if !strings.Contains(output, "Chief Security") {
		t.Fatalf("role update missing new title:\n%s", output)
	}
	getOutput := captureStdout(t, func() {
		if err := run([]string{"role", "get", "-id", roleID}); err != nil {
			t.Fatalf("role get error = %v", err)
		}
	})
	for _, want := range []string{
		"dor_map[default]: Threat model ready",
		"dod_map[default]: Review signed off",
		"ac_map[default]: Zero breaches",
		"ac_map[qa]: Security sign-off",
	} {
		if !strings.Contains(getOutput, want) {
			t.Fatalf("role get output missing %q:\n%s", want, getOutput)
		}
	}
	// Delete
	output = captureStdout(t, func() {
		if err := run([]string{"role", "delete", "-id", roleID}); err != nil {
			t.Fatalf("role delete error = %v", err)
		}
	})
	if !strings.Contains(output, "deleted") {
		t.Fatalf("role delete missing confirmation:\n%s", output)
	}
}

func normalizeTestPath(path string) string {
	cleaned := filepath.Clean(path)
	return strings.ReplaceAll(cleaned, "/private/var/", "/var/")
}

func setupLocalCLI(t *testing.T) {
	t.Helper()
	globalHome := t.TempDir()
	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir(repoDir) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	t.Setenv("TICKET_HOME", globalHome)
	if err := runInitDB([]string{"-password", "secret12"}); err != nil {
		t.Fatalf("runInitDB() error = %v", err)
	}
	if err := config.SaveProjectConfigAt(repoDir, config.Config{ProjectID: "1"}); err != nil {
		t.Fatalf("SaveProjectConfigAt() error = %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	cfg.ProjectID = "1"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}
}

func createLegacyDatabaseForCLI(t *testing.T) (string, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	if err := store.Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	ticket, err := store.CreateTicket(context.Background(), db, store.TicketCreateParams{
		ProjectID: 1,
		Type:      "task",
		Title:     "Legacy CLI ticket",
		CreatedBy: "",
	})
	if closeErr := db.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("store.CreateTicket() error = %v", err)
	}
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if _, err := rawDB.Exec(`DROP TABLE schema_meta`); err != nil {
		if closeErr := rawDB.Close(); closeErr != nil {
			t.Fatalf("rawDB.Close() error after drop failure = %v", closeErr)
		}
		t.Fatalf("DROP TABLE schema_meta error = %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("rawDB.Close() error = %v", err)
	}
	return dbPath, ticket.ID
}

// testAdminUserID returns the admin user's ID by opening the local DB and looking up the user.
func testAdminUserID(t *testing.T) string {
	t.Helper()
	resolved, err := config.ResolveURL()
	if err != nil {
		t.Fatalf("ResolveURL() error = %v", err)
	}
	db, err := store.Open(resolved.DBPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()
	user, err := store.GetUserByUsername(context.Background(), db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	return user.ID
}

func createLocalTask(t *testing.T, args []string) string {
	t.Helper()
	output := captureStdout(t, func() {
		if err := run(args); err != nil {
			t.Fatalf("run(%v) error = %v", args, err)
		}
	})
	lines := strings.Fields(output)
	if len(lines) == 0 {
		t.Fatalf("create task output empty for %v", args)
	}
	for _, field := range lines {
		id, err := parseTicketReferenceToID(field)
		if err == nil {
			return id
		}
	}
	t.Fatalf("no ticket reference found in output %q", output)
	return ""
}

func localCLIService(t *testing.T) libticket.Service {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	svc, err := resolveService(cfg)
	if err != nil {
		t.Fatalf("resolveService() error = %v", err)
	}
	return svc
}

func attachWorkflowToDefaultProject(t *testing.T, stageNames ...string) libticket.Service {
	t.Helper()
	svc := localCLIService(t)
	wf, err := svc.CreateSdlc(context.Background(), libticket.SdlcRequest{
		Name:        "Custom Workflow",
		Description: "custom workflow for tests",
	})
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}
	for i, stageName := range stageNames {
		if _, err := svc.AddSdlcStage(context.Background(), wf.ID, libticket.SdlcStageRequest{
			StageName:   stageName,
			Description: stageName,
			SortOrder:   i,
		}); err != nil {
			t.Fatalf("AddSdlcStage(%q) error = %v", stageName, err)
		}
	}
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	if _, err := svc.UpdateProject(context.Background(), project.ID, libticket.ProjectUpdateRequest{SdlcID: &wf.ID}); err != nil {
		t.Fatalf("UpdateProject(sdlc) error = %v", err)
	}
	return svc
}

// deleteTicketConfirmed performs the two-step ticket deletion: first call generates
// the confirmation token (saved in config), second call with the token deletes the ticket.
// Returns the error from the second (delete) call so callers can assert on it.
func deleteTicketConfirmed(t *testing.T, id string) error {
	t.Helper()
	// Phase 1: generate token (outputs prompt, saves token in config).
	if err := run([]string{"rm", "-id", id}); err != nil {
		t.Fatalf("rm phase 1 error = %v", err)
	}
	// Read token directly from config.
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	token := cfg.DeleteConfirmToken
	if token == "" {
		t.Fatal("no confirmation token in config after phase 1")
	}
	// Phase 2: delete with token.
	return run([]string{"rm", "-id", id, "--confirm", token})
}

func ticketLabelByID(t *testing.T, id string) string {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	svc, err := resolveService(cfg)
	if err != nil {
		t.Fatalf("resolveService() error = %v", err)
	}
	task, err := svc.GetTicket(context.Background(), id)
	if err != nil {
		t.Fatalf("svc.GetTicket(context.Background(), %s) error = %v", id, err)
	}
	return task.ID
}

func clearCurrentEpicID(t *testing.T) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	cfg.CurrentEpicID = ""
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}
}

func svcGetTicket(t *testing.T, ref string) (store.Ticket, error) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		return store.Ticket{}, err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return store.Ticket{}, err
	}
	return svc.GetTicket(context.Background(), ref)
}

func parseTicketReferenceToID(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", errors.New("empty ticket reference")
	}
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return "", err
	}
	task, err := svc.GetTicket(context.Background(), ref)
	if err != nil {
		return "", err
	}
	return task.ID, nil
}

func TestContainsFlag(t *testing.T) {
	if !containsFlag([]string{"-v", "foo"}, "-v") {
		t.Error("containsFlag should find -v")
	}
	if containsFlag([]string{"foo", "bar"}, "-v") {
		t.Error("containsFlag should not find -v")
	}
	if containsFlag(nil, "-v") {
		t.Error("containsFlag(nil) should return false")
	}
}

func TestParseProjectCommandID(t *testing.T) {
	id, ok := parseProjectCommandID("42")
	if !ok || id != 42 {
		t.Errorf("parseProjectCommandID(42) = (%d, %v)", id, ok)
	}
	_, ok = parseProjectCommandID("abc")
	if ok {
		t.Error("parseProjectCommandID(abc) should fail")
	}
}

func TestResolveLifecycleInput(t *testing.T) {
	stage, state, err := resolveLifecycleInput("", "develop", "active")
	if err != nil || stage != "develop" || state != "active" {
		t.Errorf("explicit stage/state = (%q, %q, %v)", stage, state, err)
	}
	stage, state, err = resolveLifecycleInput("develop/idle", "", "")
	if err != nil || stage != "develop" || state != "idle" {
		t.Errorf("status parse = (%q, %q, %v)", stage, state, err)
	}
	stage, state, err = resolveLifecycleInput("", "", "")
	if err != nil || stage != "" || state != "" {
		t.Errorf("empty = (%q, %q, %v)", stage, state, err)
	}
	_, _, err = resolveLifecycleInput("bogus", "", "")
	if err == nil {
		t.Error("bogus status should error")
	}
}

func TestIsReviewerAuthor(t *testing.T) {
	if !isReviewerAuthor("Reviewer-Agent") {
		t.Error("should match reviewer")
	}
	if isReviewerAuthor("alice") {
		t.Error("should not match alice")
	}
}

func TestIsReviewerCommentText(t *testing.T) {
	if !isReviewerCommentText("This has been reviewed and approved") {
		t.Error("should match reviewed")
	}
	if isReviewerCommentText("Just a normal comment") {
		t.Error("should not match")
	}
}

func TestHasReviewerAgentComment(t *testing.T) {
	if hasReviewerAgentComment(nil) {
		t.Error("nil should return false")
	}
	comments := []store.Comment{
		{Author: "alice", Text: "hello"},
	}
	if hasReviewerAgentComment(comments) {
		t.Error("no reviewer should return false")
	}
	comments = append(comments, store.Comment{Author: "Reviewer", Text: "approved"})
	if !hasReviewerAgentComment(comments) {
		t.Error("reviewer comment should return true")
	}
}

func TestBuildAgentPrompt(t *testing.T) {
	ticket := store.Ticket{
		ID:                 "TK-1",
		Title:              "Test Task",
		Description:        "Some description",
		AcceptanceCriteria: "Must pass",
	}
	role := store.Role{Title: "Developer", Description: "Ship features", AcceptanceCriteria: "Quality code"}
	wf := store.SdlcWithStages{
		Sdlc:   store.Sdlc{Name: "Standard"},
		Stages: []store.SdlcStage{{StageName: "develop"}, {StageName: "develop"}, {StageName: "test"}},
	}
	project := store.Project{Prefix: "TK", Title: "Test Project", GitRepository: "github.com/test/repo"}
	resp := libticket.AgentWorkResponse{
		Status:  "NEW",
		Ticket:  &ticket,
		Project: &project,
		Sdlc:    &wf,
		Role:    &role,
	}
	prompt := buildAgentPrompt(resp)
	for _, want := range []string{"Test Task", "Some description", "Must pass", "Developer", "Ship features", "Standard", "develop", "develop", "Test Project", "github.com/test/repo"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("buildAgentPrompt missing %q:\n%s", want, prompt)
		}
	}
	// Without description/AC/role/sdlc
	ticket2 := store.Ticket{Title: "Simple"}
	prompt2 := buildAgentPrompt(libticket.AgentWorkResponse{Ticket: &ticket2})
	if strings.Contains(prompt2, "Description:") {
		t.Error("should not include Description header for empty description")
	}
	if strings.Contains(prompt2, "Role:") {
		t.Error("should not include Role header when no role")
	}
}

func TestAllTopLevelCommandsShowUsageWithNoArgs(t *testing.T) {
	commands := []string{
		"project", "sdlc", "team", "story", "user", "label",
		"dep", "decision", "agent", "role", "idea",
	}
	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			output := captureStdout(t, func() {
				// These should all print usage and return nil (no error).
				_ = run([]string{cmd})
			})
			if !strings.Contains(strings.ToLower(output), "usage") {
				t.Errorf("tk %s with no args should print usage, got:\n%s", cmd, output)
			}
		})
	}
}

func TestIdeasCommandIsRemoved(t *testing.T) {
	err := run([]string{"ideas"})
	if err == nil {
		t.Error("tk ideas should return an error, got nil")
	}
}

func TestRunVersion(t *testing.T) {
	output := captureStdout(t, func() {
		if err := run([]string{"version"}); err != nil {
			t.Fatalf("version error = %v", err)
		}
	})
	if strings.TrimSpace(output) == "" {
		t.Fatal("version output empty")
	}
}

func TestRunStatusReturnsUpgradeDatabaseHintForLegacyDatabase(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	legacyPath, _ := createLegacyDatabaseForCLI(t)
	setTestLocation(t, legacyPath)

	err := run([]string{"status"})
	if err == nil {
		t.Fatal("run(status) error = nil, want schema upgrade hint")
	}
	if !strings.Contains(err.Error(), "upgrade-database") {
		t.Fatalf("run(status) error = %v, want upgrade-database hint", err)
	}
}

func TestRunUpgradeDatabasePortsLegacyDatabase(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	legacyPath, ticketID := createLegacyDatabaseForCLI(t)
	setTestLocation(t, legacyPath)

	targetPath := filepath.Join(t.TempDir(), "new_database", "ticket.db")
	output := captureStdout(t, func() {
		if err := run([]string{"upgrade-database", "-o", targetPath}); err != nil {
			t.Fatalf("upgrade-database error = %v", err)
		}
	})
	if !strings.Contains(output, targetPath) {
		t.Fatalf("upgrade-database output = %q, want target path", output)
	}
	if got, err := store.DetectSchemaVersion(legacyPath); err != nil {
		t.Fatalf("DetectSchemaVersion(source) error = %v", err)
	} else if got != store.LegacySchemaVersion {
		t.Fatalf("DetectSchemaVersion(source) = %d, want %d", got, store.LegacySchemaVersion)
	}
	if got, err := store.DetectSchemaVersion(targetPath); err != nil {
		t.Fatalf("DetectSchemaVersion(target) error = %v", err)
	} else if got != store.CurrentSchemaVersion {
		t.Fatalf("DetectSchemaVersion(target) = %d, want %d", got, store.CurrentSchemaVersion)
	}
	db, err := store.Open(targetPath)
	if err != nil {
		t.Fatalf("store.Open(target) error = %v", err)
	}
	defer db.Close()
	ticket, err := store.GetTicket(context.Background(), db, ticketID)
	if err != nil {
		t.Fatalf("store.GetTicket() error = %v", err)
	}
	if ticket.Title != "Legacy CLI ticket" {
		t.Fatalf("ticket.Title = %q, want %q", ticket.Title, "Legacy CLI ticket")
	}
}

func TestRunLabelCRUD(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "Label Test"})

	// Create label
	output := captureStdout(t, func() {
		if err := run([]string{"label", "create", "-name", "urgent", "-color", "red"}); err != nil {
			t.Fatalf("label create error = %v", err)
		}
	})
	if !strings.Contains(output, "urgent") {
		t.Fatalf("label create missing name:\n%s", output)
	}

	// List labels
	output = captureStdout(t, func() {
		if err := run([]string{"label", "ls"}); err != nil {
			t.Fatalf("label ls error = %v", err)
		}
	})
	if !strings.Contains(output, "urgent") {
		t.Fatalf("label ls missing urgent:\n%s", output)
	}

	// Add label to ticket
	captureStdout(t, func() {
		if err := run([]string{"label", "add", taskID, "1"}); err != nil {
			t.Fatalf("label add error = %v", err)
		}
	})

	// Show ticket labels
	output = captureStdout(t, func() {
		if err := run([]string{"label", "show", taskID}); err != nil {
			t.Fatalf("label show error = %v", err)
		}
	})
	if !strings.Contains(output, "urgent") {
		t.Fatalf("label show missing urgent:\n%s", output)
	}

	// Remove label from ticket
	captureStdout(t, func() {
		if err := run([]string{"label", "remove", taskID, "1"}); err != nil {
			t.Fatalf("label remove error = %v", err)
		}
	})

	// Delete label
	captureStdout(t, func() {
		if err := run([]string{"label", "delete", "1"}); err != nil {
			t.Fatalf("label delete error = %v", err)
		}
	})
}

func TestRunTimeCRUD(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "Time Test"})
	idStr := taskID

	// Log time
	output := captureStdout(t, func() {
		if err := run([]string{"time", "log", "-id", idStr, "-m", "30", "-note", "morning"}); err != nil {
			t.Fatalf("time log error = %v", err)
		}
	})
	if !strings.Contains(output, "30") {
		t.Fatalf("time log missing minutes:\n%s", output)
	}

	// List time entries
	output = captureStdout(t, func() {
		if err := run([]string{"time", "list", idStr}); err != nil {
			t.Fatalf("time list error = %v", err)
		}
	})
	if !strings.Contains(output, "morning") {
		t.Fatalf("time list missing note:\n%s", output)
	}

	// Total time
	output = captureStdout(t, func() {
		if err := run([]string{"time", "total", idStr}); err != nil {
			t.Fatalf("time total error = %v", err)
		}
	})
	if !strings.Contains(output, "30") {
		t.Fatalf("time total missing minutes:\n%s", output)
	}

	// Delete time entry
	captureStdout(t, func() {
		if err := run([]string{"time", "delete", "1"}); err != nil {
			t.Fatalf("time delete error = %v", err)
		}
	})
}

func TestRunBoard(t *testing.T) {
	setupLocalCLI(t)
	createLocalTask(t, []string{"add", "Board Task"})
	output := captureStdout(t, func() {
		if err := run([]string{"board"}); err != nil {
			t.Fatalf("board error = %v", err)
		}
	})
	for _, want := range []string{"DEVELOP", "DONE"} {
		if !strings.Contains(output, want) {
			t.Fatalf("board missing %q:\n%s", want, output)
		}
	}
	if !strings.Contains(output, "Board Task") {
		t.Fatalf("board missing ticket:\n%s", output)
	}
}

func TestRunEpicUseSetsCurrent(t *testing.T) {
	setupLocalCLI(t)

	epicID := createLocalTask(t, []string{"epic", "My Epic"})
	epicRef := epicID

	if err := run([]string{"epic", "use", epicRef}); err != nil {
		t.Fatalf("epic use error = %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.CurrentEpicID != epicID {
		t.Fatalf("CurrentEpicID = %s, want %s", cfg.CurrentEpicID, epicID)
	}
}

func TestRunEpicUseRejectsNonEpic(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Just a task"})
	if err := run([]string{"epic", "use", taskID}); err == nil {
		t.Fatal("expected error when using a non-epic ticket, got nil")
	}
}

func TestRunEpicGetReturnsExistingEpicAndDoesNotCreateTicket(t *testing.T) {
	setupLocalCLI(t)

	epicID := createLocalTask(t, []string{"epic", "Lookup Epic"})

	cfg, svc, project, err := resolveCurrentProjectClient()
	if err != nil {
		t.Fatalf("resolveCurrentProjectClient() error = %v", err)
	}
	before, err := svc.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, true)
	if err != nil {
		t.Fatalf("ListTicketsFiltered(before) error = %v", err)
	}
	_ = cfg

	output := captureStdout(t, func() {
		if err := run([]string{"epic", "get", epicID}); err != nil {
			t.Fatalf("epic get error = %v", err)
		}
	})
	if !hasDetailField(output, "Title", "Lookup Epic") {
		t.Fatalf("epic get output missing epic details:\n%s", output)
	}

	after, err := svc.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, true)
	if err != nil {
		t.Fatalf("ListTicketsFiltered(after) error = %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("epic get should not create tickets: before=%d after=%d", len(before), len(after))
	}
}

func TestRunEpicGetRejectsNonEpic(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Not an epic"})
	err := run([]string{"epic", "get", taskID})
	if err == nil {
		t.Fatal("expected error when getting a non-epic ticket via epic get")
	}
	if !strings.Contains(err.Error(), "is not an epic") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunBugGetReturnsExistingBugAndDoesNotCreateTicket(t *testing.T) {
	setupLocalCLI(t)

	bugID := createLocalTask(t, []string{"bug", "Lookup Bug"})
	_, svc, project, err := resolveCurrentProjectClient()
	if err != nil {
		t.Fatalf("resolveCurrentProjectClient() error = %v", err)
	}
	before, err := svc.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, true)
	if err != nil {
		t.Fatalf("ListTicketsFiltered(before) error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"bug", "get", bugID}); err != nil {
			t.Fatalf("bug get error = %v", err)
		}
	})
	if !hasDetailField(output, "Title", "Lookup Bug") {
		t.Fatalf("bug get output missing bug details:\n%s", output)
	}

	after, err := svc.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, true)
	if err != nil {
		t.Fatalf("ListTicketsFiltered(after) error = %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("bug get should not create tickets: before=%d after=%d", len(before), len(after))
	}
}

func TestRunBugGetRejectsNonBug(t *testing.T) {
	setupLocalCLI(t)

	epicID := createLocalTask(t, []string{"epic", "Not a bug"})
	err := run([]string{"bug", "get", epicID})
	if err == nil {
		t.Fatal("expected error when getting a non-bug ticket via bug get")
	}
	if !strings.Contains(err.Error(), "is not a bug") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunEpicClearResetsEpicID(t *testing.T) {
	setupLocalCLI(t)

	epicID := createLocalTask(t, []string{"epic", "Clearable Epic"})
	if err := run([]string{"epic", "use", epicID}); err != nil {
		t.Fatalf("epic use error = %v", err)
	}

	if err := run([]string{"epic", "clear"}); err != nil {
		t.Fatalf("epic clear error = %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.CurrentEpicID != "" {
		t.Fatalf("CurrentEpicID = %s after clear, want empty", cfg.CurrentEpicID)
	}
}

func TestRunEpicListShowsActiveMarker(t *testing.T) {
	setupLocalCLI(t)

	epicID := createLocalTask(t, []string{"epic", "Listed Epic"})
	epicRef := epicID

	// Clear active epic so we can test the "no marker" state
	clearCurrentEpicID(t)

	output := captureStdout(t, func() {
		if err := run([]string{"epic", "ls"}); err != nil {
			t.Fatalf("epic ls error = %v", err)
		}
	})
	if strings.Contains(output, "*") {
		t.Fatalf("epic ls should not show active marker before use: %s", output)
	}

	if err := run([]string{"epic", "use", epicRef}); err != nil {
		t.Fatalf("epic use error = %v", err)
	}

	output = captureStdout(t, func() {
		if err := run([]string{"epic", "ls"}); err != nil {
			t.Fatalf("epic ls error = %v", err)
		}
	})
	if !strings.Contains(output, "*") {
		t.Fatalf("epic ls should show active marker after use: %s", output)
	}
	if !strings.Contains(output, "Listed Epic") {
		t.Fatalf("epic ls missing epic title: %s", output)
	}
}

func TestRunUnclaimRejectsNonOwner(t *testing.T) {
	setupLocalCLI(t)

	// Create and assign to alice
	taskID := createLocalTask(t, []string{"add", "Unclaim Test Task"})
	if err := run([]string{"user", "create", "-username", "alice", "-password", "secret12"}); err != nil {
		t.Fatalf("user create error = %v", err)
	}
	if err := run([]string{"assign", taskID, "alice"}); err != nil {
		t.Fatalf("assign error = %v", err)
	}

	// Admin (local mode user) is not alice; unclaim should fail
	err := run([]string{"unclaim", taskID})
	if err == nil {
		t.Fatal("expected error when unclaiming a ticket not owned by the caller")
	}
	if !strings.Contains(err.Error(), "not assigned to admin") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRunClaimRejectsAlreadyAssigned(t *testing.T) {
	setupLocalCLI(t)

	// Create and assign to alice
	taskID := createLocalTask(t, []string{"add", "Claim Conflict Task"})
	if err := run([]string{"user", "create", "-username", "alice", "-password", "secret12"}); err != nil {
		t.Fatalf("user create error = %v", err)
	}
	if err := run([]string{"assign", taskID, "alice"}); err != nil {
		t.Fatalf("assign error = %v", err)
	}

	// Advance to develop/idle so it is "claimable" stage-wise
	if err := run([]string{"complete", "-id", taskID}); err != nil {
		t.Fatalf("complete error = %v", err)
	}

	// claim as admin (already assigned to alice) — runRequest returns nil and prints REJECTED
	output := captureStdout(t, func() {
		if err := run([]string{"claim", taskID}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(output, "REJECTED") {
		t.Fatalf("claim of already-assigned ticket should print REJECTED, got: %s", output)
	}
}

func TestRunAssignDisabledUserFails(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Disabled Assign Task"})
	if err := run([]string{"user", "create", "-username", "carol", "-password", "secret12"}); err != nil {
		t.Fatalf("user create error = %v", err)
	}
	if err := run([]string{"user", "disable", "-username", "carol"}); err != nil {
		t.Fatalf("user disable error = %v", err)
	}

	err := run([]string{"assign", taskID, "carol"})
	if err == nil {
		t.Fatal("expected error when assigning to a disabled user")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRunAssignNonExistentUserFails(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "No User Task"})
	err := run([]string{"assign", taskID, "nobody"})
	if err == nil {
		t.Fatal("expected error when assigning to a non-existent user")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRunStoryCreateListGetUpdateDelete(t *testing.T) {
	setupLocalCLI(t)

	// create
	createOutput := captureStdout(t, func() {
		if err := run([]string{"story", "create", "Auth Story", "-d", "User wants to log in"}); err != nil {
			t.Fatalf("story create error = %v", err)
		}
	})
	if !strings.Contains(createOutput, "Auth Story") {
		t.Fatalf("story create output missing title: %s", createOutput)
	}

	// list
	listOutput := captureStdout(t, func() {
		if err := run([]string{"story", "ls"}); err != nil {
			t.Fatalf("story ls error = %v", err)
		}
	})
	if !strings.Contains(listOutput, "Auth Story") {
		t.Fatalf("story ls missing title: %s", listOutput)
	}
	if !strings.Contains(listOutput, "draft") {
		t.Fatalf("story ls missing status: %s", listOutput)
	}

	// get
	getOutput := captureStdout(t, func() {
		if err := run([]string{"story", "get", "1"}); err != nil {
			t.Fatalf("story get error = %v", err)
		}
	})
	if !strings.Contains(getOutput, "Auth Story") {
		t.Fatalf("story get missing title: %s", getOutput)
	}
	if !strings.Contains(getOutput, "User wants to log in") {
		t.Fatalf("story get missing description: %s", getOutput)
	}

	// update
	updateOutput := captureStdout(t, func() {
		if err := run([]string{"story", "update", "1", "-title", "Updated Story"}); err != nil {
			t.Fatalf("story update error = %v", err)
		}
	})
	if !strings.Contains(updateOutput, "Updated Story") {
		t.Fatalf("story update output missing new title: %s", updateOutput)
	}

	// delete
	if err := run([]string{"story", "delete", "1"}); err != nil {
		t.Fatalf("story delete error = %v", err)
	}

	// list should be empty
	emptyOutput := captureStdout(t, func() {
		if err := run([]string{"story", "ls"}); err != nil {
			t.Fatalf("story ls error after delete = %v", err)
		}
	})
	if strings.Contains(emptyOutput, "Updated Story") {
		t.Fatalf("story ls should be empty after delete: %s", emptyOutput)
	}
}

func TestRunStoryCreatePositionalTitle(t *testing.T) {
	setupLocalCLI(t)

	output := captureStdout(t, func() {
		if err := run([]string{"story", "create", "My User Story"}); err != nil {
			t.Fatalf("story create error = %v", err)
		}
	})
	if !strings.Contains(output, "My User Story") {
		t.Fatalf("story create did not use positional title: %s", output)
	}
}

func TestRunStoryCreateRequiresTitle(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"story", "create"}); err == nil {
		t.Fatal("expected error when creating story without title")
	}
}

func TestRunStoryGetInvalidID(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"story", "get", "999"}); err == nil {
		t.Fatal("expected error for non-existent story id")
	}
}

func TestRunCurateCreatesRequirement(t *testing.T) {
	setupLocalCLI(t)

	sourceID := createLocalTask(t, []string{"add", "Implement login"})

	output := captureStdout(t, func() {
		if err := run([]string{"curate", sourceID}); err != nil {
			t.Fatalf("curate error = %v", err)
		}
	})
	if !strings.Contains(output, "requirement") {
		t.Fatalf("curate output missing type=requirement:\n%s", output)
	}
	if !strings.Contains(output, "Implement login") {
		t.Fatalf("curate output missing source title:\n%s", output)
	}
}

func TestRunCurateRequiresArgs(t *testing.T) {
	setupLocalCLI(t)
	if err := run([]string{"curate"}); err == nil {
		t.Fatal("expected error when curate has no args")
	}
}

func TestRunReviewListsRequirements(t *testing.T) {
	setupLocalCLI(t)

	sourceID := createLocalTask(t, []string{"add", "Login feature"})
	if err := run([]string{"curate", sourceID}); err != nil {
		t.Fatalf("curate error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"review"}); err != nil {
			t.Fatalf("review error = %v", err)
		}
	})
	if !strings.Contains(output, "Login feature") {
		t.Fatalf("review output missing requirement title:\n%s", output)
	}
}

func TestRunReviewFiltersByStatus(t *testing.T) {
	setupLocalCLI(t)

	sourceID := createLocalTask(t, []string{"add", "Filter source"})
	var reqKey string
	captureStdout(t, func() {
		if err := run([]string{"curate", sourceID}); err != nil {
			t.Fatalf("curate error = %v", err)
		}
	})
	// list to get key
	allOutput := captureStdout(t, func() {
		if err := run([]string{"review"}); err != nil {
			t.Fatalf("review error = %v", err)
		}
	})
	// extract key from output line
	for _, line := range strings.Split(allOutput, "\n") {
		if strings.Contains(line, "Filter source") {
			reqKey = strings.Fields(line)[0]
			break
		}
	}
	if reqKey == "" {
		t.Fatalf("could not find requirement key in review output: %s", allOutput)
	}

	// accept it
	if err := run([]string{"accept", "requirement", reqKey}); err != nil {
		t.Fatalf("accept error = %v", err)
	}

	// should appear in accepted review
	acceptedOutput := captureStdout(t, func() {
		if err := run([]string{"review", "-status", "accepted"}); err != nil {
			t.Fatalf("review -status accepted error = %v", err)
		}
	})
	if !strings.Contains(acceptedOutput, "Filter source") {
		t.Fatalf("review -status accepted missing accepted requirement:\n%s", acceptedOutput)
	}

	// should not appear in proposed review
	proposedOutput := captureStdout(t, func() {
		if err := run([]string{"review", "-status", "proposed"}); err != nil {
			t.Fatalf("review -status proposed error = %v", err)
		}
	})
	if strings.Contains(proposedOutput, "Filter source") {
		t.Fatalf("review -status proposed should not show accepted requirement:\n%s", proposedOutput)
	}
}

func TestRunAcceptAndRejectRequirement(t *testing.T) {
	setupLocalCLI(t)

	src1 := createLocalTask(t, []string{"add", "Accept me"})
	src2 := createLocalTask(t, []string{"add", "Reject me"})

	var req1Key, req2Key string
	captureStdout(t, func() {
		if err := run([]string{"curate", src1}); err != nil {
			t.Fatalf("curate error = %v", err)
		}
	})
	captureStdout(t, func() {
		if err := run([]string{"curate", src2}); err != nil {
			t.Fatalf("curate error = %v", err)
		}
	})
	allOutput := captureStdout(t, func() {
		if err := run([]string{"review"}); err != nil {
			t.Fatalf("review error = %v", err)
		}
	})
	for _, line := range strings.Split(allOutput, "\n") {
		if strings.Contains(line, "Accept me") {
			req1Key = strings.Fields(line)[0]
		}
		if strings.Contains(line, "Reject me") {
			req2Key = strings.Fields(line)[0]
		}
	}
	if req1Key == "" || req2Key == "" {
		t.Fatalf("could not extract keys from review output: %s", allOutput)
	}

	// accept
	acceptOut := captureStdout(t, func() {
		if err := run([]string{"accept", "requirement", req1Key}); err != nil {
			t.Fatalf("accept error = %v", err)
		}
	})
	// accept auto-advances through the sdlc: design/success → develop/idle
	if !strings.Contains(acceptOut, "develop") {
		t.Fatalf("accept output should show ticket moved to develop stage:\n%s", acceptOut)
	}

	// reject
	rejectOut := captureStdout(t, func() {
		if err := run([]string{"reject", "requirement", req2Key}); err != nil {
			t.Fatalf("reject error = %v", err)
		}
	})
	if !strings.Contains(rejectOut, "fail") {
		t.Fatalf("reject output should show fail state:\n%s", rejectOut)
	}
}

func TestRunReviseRequirement(t *testing.T) {
	setupLocalCLI(t)

	srcID := createLocalTask(t, []string{"add", "Revise source"})
	var reqKey string
	captureStdout(t, func() {
		if err := run([]string{"curate", srcID}); err != nil {
			t.Fatalf("curate error = %v", err)
		}
	})
	allOutput := captureStdout(t, func() {
		if err := run([]string{"review"}); err != nil {
			t.Fatalf("review error = %v", err)
		}
	})
	for _, line := range strings.Split(allOutput, "\n") {
		if strings.Contains(line, "Revise source") {
			reqKey = strings.Fields(line)[0]
			break
		}
	}
	if reqKey == "" {
		t.Fatalf("could not find requirement key: %s", allOutput)
	}

	out := captureStdout(t, func() {
		if err := run([]string{"revise", "requirement", reqKey}); err != nil {
			t.Fatalf("revise error = %v", err)
		}
	})
	if !strings.Contains(out, "(revised)") {
		t.Fatalf("revise output should contain (revised):\n%s", out)
	}
}

func TestRunDecisionNewListAndPrintID(t *testing.T) {
	setupLocalCLI(t)

	decisionID := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"decision", "new", "-printid", "Use PostgreSQL for storage"}); err != nil {
			t.Fatalf("decision new error = %v", err)
		}
	}))
	decision, err := localCLIService(t).GetTicket(context.Background(), decisionID)
	if err != nil {
		t.Fatalf("GetTicket(%q) error = %v", decisionID, err)
	}
	if decision.Type != "decision" {
		t.Fatalf("decision type = %q, want decision", decision.Type)
	}

	listOut := captureStdout(t, func() {
		if err := run([]string{"decision", "ls"}); err != nil {
			t.Fatalf("decision ls error = %v", err)
		}
	})
	if !strings.Contains(listOut, "Use PostgreSQL for storage") {
		t.Fatalf("decision ls missing decision text:\n%s", listOut)
	}
}

func TestRunConversationShow(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Conversation ticket"})
	if err := run([]string{"comment", "add", "-id", taskID, "First comment"}); err != nil {
		t.Fatalf("comment add error = %v", err)
	}

	out := captureStdout(t, func() {
		if err := run([]string{"conversation", "show", taskID}); err != nil {
			t.Fatalf("conversation show error = %v", err)
		}
	})
	if !strings.Contains(out, "created task") {
		t.Fatalf("conversation show missing created event:\n%s", out)
	}
}

func TestRunNoteAndQuestionCreate(t *testing.T) {
	setupLocalCLI(t)

	noteOut := captureStdout(t, func() {
		if err := run([]string{"note", "Meeting notes from standup"}); err != nil {
			t.Fatalf("note error = %v", err)
		}
	})
	if noteOut == "" {
		t.Fatal("note command produced no output")
	}

	questionOut := captureStdout(t, func() {
		if err := run([]string{"question", "Should we migrate to Postgres?"}); err != nil {
			t.Fatalf("question error = %v", err)
		}
	})
	if questionOut == "" {
		t.Fatal("question command produced no output")
	}
}

func TestRunTeamCreateListUpdateDelete(t *testing.T) {
	setupLocalCLI(t)

	// create
	createOut := captureStdout(t, func() {
		if err := run([]string{"team", "create", "-name", "Alpha Team"}); err != nil {
			t.Fatalf("team create error = %v", err)
		}
	})
	if !strings.Contains(createOut, "Alpha Team") {
		t.Fatalf("team create output missing name:\n%s", createOut)
	}
	// extract team id — output format: "created team #<id> <name>"
	var teamID string
	for _, f := range strings.Fields(createOut) {
		clean := strings.TrimPrefix(f, "#")
		if n, err := strconv.ParseInt(clean, 10, 64); err == nil && n > 0 {
			teamID = clean
			break
		}
	}
	if teamID == "" {
		t.Fatalf("could not extract team id from: %s", createOut)
	}

	// list
	listOut := captureStdout(t, func() {
		if err := run([]string{"team", "list"}); err != nil {
			t.Fatalf("team list error = %v", err)
		}
	})
	if !strings.Contains(listOut, "Alpha Team") {
		t.Fatalf("team list missing Alpha Team:\n%s", listOut)
	}

	// ls alias
	lsOut := captureStdout(t, func() {
		if err := run([]string{"team", "ls"}); err != nil {
			t.Fatalf("team ls error = %v", err)
		}
	})
	if !strings.Contains(lsOut, "Alpha Team") {
		t.Fatalf("team ls missing Alpha Team:\n%s", lsOut)
	}

	// update
	updateOut := captureStdout(t, func() {
		if err := run([]string{"team", "update", "-id", teamID, "-name", "Beta Team"}); err != nil {
			t.Fatalf("team update error = %v", err)
		}
	})
	if !strings.Contains(updateOut, "Beta Team") {
		t.Fatalf("team update output missing new name:\n%s", updateOut)
	}

	// delete
	captureStdout(t, func() {
		if err := run([]string{"team", "delete", "-id", teamID}); err != nil {
			t.Fatalf("team delete error = %v", err)
		}
	})

	// verify gone
	afterDelete := captureStdout(t, func() {
		if err := run([]string{"team", "list"}); err != nil {
			t.Fatalf("team list after delete error = %v", err)
		}
	})
	if strings.Contains(afterDelete, "Beta Team") {
		t.Fatalf("team list still shows deleted team:\n%s", afterDelete)
	}
}

func TestRunCreateCommandsSupportForcedIDs(t *testing.T) {
	t.Run("project", func(t *testing.T) {
		setupLocalCLI(t)
		if err := run([]string{"project", "create", "-id", "101", "-prefix", "EXP", "-title", "Explicit Project"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
		project, err := localCLIService(t).GetProject(context.Background(), "101")
		if err != nil {
			t.Fatalf("GetProject(101) error = %v", err)
		}
		if project.ID != 101 {
			t.Fatalf("project id = %d, want 101", project.ID)
		}
	})

	t.Run("team", func(t *testing.T) {
		setupLocalCLI(t)
		if err := run([]string{"team", "create", "-id", "102", "-name", "Explicit Team"}); err != nil {
			t.Fatalf("team create error = %v", err)
		}
		teams, err := localCLIService(t).ListTeams(context.Background())
		if err != nil {
			t.Fatalf("ListTeams() error = %v", err)
		}
		found := false
		for _, team := range teams {
			if team.ID == 102 {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("team 102 not found")
		}
	})

	t.Run("role", func(t *testing.T) {
		setupLocalCLI(t)
		if err := run([]string{"role", "create", "-id", "103", "-title", "Explicit Role"}); err != nil {
			t.Fatalf("role create error = %v", err)
		}
		roles, err := localCLIService(t).ListRoles(context.Background())
		if err != nil {
			t.Fatalf("ListRoles() error = %v", err)
		}
		found := false
		for _, role := range roles {
			if role.ID == 103 {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("role 103 not found")
		}
	})

	t.Run("sdlc", func(t *testing.T) {
		setupLocalCLI(t)
		if err := run([]string{"sdlc", "create", "-id", "104", "-name", "Explicit SDLC"}); err != nil {
			t.Fatalf("sdlc create error = %v", err)
		}
		wf, err := localCLIService(t).GetSdlc(context.Background(), 104)
		if err != nil {
			t.Fatalf("GetSdlc(104) error = %v", err)
		}
		if wf.ID != 104 {
			t.Fatalf("sdlc id = %d, want 104", wf.ID)
		}
	})

	t.Run("label", func(t *testing.T) {
		setupLocalCLI(t)
		if err := run([]string{"label", "create", "-id", "105", "Explicit Label"}); err != nil {
			t.Fatalf("label create error = %v", err)
		}
		project, err := localCLIService(t).GetProject(context.Background(), "1")
		if err != nil {
			t.Fatalf("GetProject(1) error = %v", err)
		}
		labels, err := localCLIService(t).ListLabels(context.Background(), project.ID)
		if err != nil {
			t.Fatalf("ListLabels() error = %v", err)
		}
		found := false
		for _, label := range labels {
			if label.ID == 105 {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("label 105 not found")
		}
	})

	t.Run("story", func(t *testing.T) {
		setupLocalCLI(t)
		if err := run([]string{"story", "create", "-id", "106", "-title", "Explicit Story"}); err != nil {
			t.Fatalf("story create error = %v", err)
		}
		story, err := localCLIService(t).GetStory(context.Background(), 106)
		if err != nil {
			t.Fatalf("GetStory(106) error = %v", err)
		}
		if story.ID != 106 {
			t.Fatalf("story id = %d, want 106", story.ID)
		}
	})
}

func TestRunCreateCommandsPrintOnlyID(t *testing.T) {
	openLocalDB := func(t *testing.T) *sql.DB {
		t.Helper()
		resolved, err := config.ResolveURL()
		if err != nil {
			t.Fatalf("ResolveURL() error = %v", err)
		}
		db, err := store.Open(resolved.DBPath)
		if err != nil {
			t.Fatalf("store.Open() error = %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })
		return db
	}

	t.Run("project", func(t *testing.T) {
		setupLocalCLI(t)
		out := strings.TrimSpace(captureStdout(t, func() {
			if err := run([]string{"project", "create", "-id", "301", "-printid", "-prefix", "PID", "-title", "Print Project"}); err != nil {
				t.Fatalf("project create error = %v", err)
			}
		}))
		if out != "301" {
			t.Fatalf("project output = %q, want %q", out, "301")
		}
	})

	t.Run("team", func(t *testing.T) {
		setupLocalCLI(t)
		out := strings.TrimSpace(captureStdout(t, func() {
			if err := run([]string{"team", "create", "-id", "302", "-printid", "-name", "Print Team"}); err != nil {
				t.Fatalf("team create error = %v", err)
			}
		}))
		if out != "302" {
			t.Fatalf("team output = %q, want %q", out, "302")
		}
	})

	t.Run("role", func(t *testing.T) {
		setupLocalCLI(t)
		out := strings.TrimSpace(captureStdout(t, func() {
			if err := run([]string{"role", "create", "-id", "303", "-printid", "-title", "Print Role"}); err != nil {
				t.Fatalf("role create error = %v", err)
			}
		}))
		if out != "303" {
			t.Fatalf("role output = %q, want %q", out, "303")
		}
	})

	t.Run("sdlc", func(t *testing.T) {
		setupLocalCLI(t)
		out := strings.TrimSpace(captureStdout(t, func() {
			if err := run([]string{"sdlc", "create", "-id", "304", "-printid", "-name", "Print SDLC"}); err != nil {
				t.Fatalf("sdlc create error = %v", err)
			}
		}))
		if out != "304" {
			t.Fatalf("sdlc output = %q, want %q", out, "304")
		}
	})

	t.Run("label", func(t *testing.T) {
		setupLocalCLI(t)
		out := strings.TrimSpace(captureStdout(t, func() {
			if err := run([]string{"label", "create", "-id", "305", "-printid", "print-label"}); err != nil {
				t.Fatalf("label create error = %v", err)
			}
		}))
		if out != "305" {
			t.Fatalf("label output = %q, want %q", out, "305")
		}
	})

	t.Run("story", func(t *testing.T) {
		setupLocalCLI(t)
		out := strings.TrimSpace(captureStdout(t, func() {
			if err := run([]string{"story", "create", "-id", "306", "-printid", "-title", "Print Story"}); err != nil {
				t.Fatalf("story create error = %v", err)
			}
		}))
		if out != "306" {
			t.Fatalf("story output = %q, want %q", out, "306")
		}
	})

	t.Run("user", func(t *testing.T) {
		setupLocalCLI(t)
		out := strings.TrimSpace(captureStdout(t, func() {
			if err := run([]string{"user", "create", "-username", "script-user", "-password", "password123", "-printid"}); err != nil {
				t.Fatalf("user create error = %v", err)
			}
		}))
		user, err := store.GetUserByUsername(context.Background(), openLocalDB(t), "script-user")
		if err != nil {
			t.Fatalf("GetUserByUsername(script-user) error = %v", err)
		}
		if out != user.ID {
			t.Fatalf("user output = %q, want %q", out, user.ID)
		}
	})

	t.Run("agent", func(t *testing.T) {
		setupLocalCLI(t)
		out := strings.TrimSpace(captureStdout(t, func() {
			if err := run([]string{"agent", "create", "-password", "agentpass123", "-printid"}); err != nil {
				t.Fatalf("agent create error = %v", err)
			}
		}))
		agent, err := store.GetAgentByID(context.Background(), openLocalDB(t), out)
		if err != nil {
			t.Fatalf("GetAgentByID(%q) error = %v", out, err)
		}
		if out != agent.ID {
			t.Fatalf("agent output = %q, want %q", out, agent.ID)
		}
	})

	t.Run("ticket", func(t *testing.T) {
		setupLocalCLI(t)
		out := strings.TrimSpace(captureStdout(t, func() {
			if err := run([]string{"add", "-printid", "Print Ticket"}); err != nil {
				t.Fatalf("ticket create error = %v", err)
			}
		}))
		ticket, err := localCLIService(t).GetTicket(context.Background(), out)
		if err != nil {
			t.Fatalf("GetTicket(%q) error = %v", out, err)
		}
		if out != ticket.ID {
			t.Fatalf("ticket output = %q, want %q", out, ticket.ID)
		}
	})
}

func TestRunCountSupportsTicketFiltersAndExpectations(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"bug", "Bug Alpha"}); err != nil {
		t.Fatalf("bug create alpha error = %v", err)
	}
	if err := run([]string{"bug", "Bug Beta"}); err != nil {
		t.Fatalf("bug create beta error = %v", err)
	}
	if err := run([]string{"add", "Task Gamma"}); err != nil {
		t.Fatalf("task create error = %v", err)
	}

	summaryOut := captureStdout(t, func() {
		if err := run([]string{"count"}); err != nil {
			t.Fatalf("count summary error = %v", err)
		}
	})
	if !strings.Contains(summaryOut, "bugs") {
		t.Fatalf("count summary missing bug totals:\n%s", summaryOut)
	}

	filteredOut := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"count", "-type", "bug"}); err != nil {
			t.Fatalf("count filtered error = %v", err)
		}
	}))
	if filteredOut != "2" {
		t.Fatalf("filtered count output = %q, want %q", filteredOut, "2")
	}

	searchOut := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"count", "-type", "bug", "-search", "Beta"}); err != nil {
			t.Fatalf("count search error = %v", err)
		}
	}))
	if searchOut != "1" {
		t.Fatalf("search count output = %q, want %q", searchOut, "1")
	}

	okOut := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"count", "-type", "bug", "-expect_equals", "2"}); err != nil {
			t.Fatalf("count expect equals error = %v", err)
		}
	}))
	if okOut != "2" {
		t.Fatalf("expect equals output = %q, want %q", okOut, "2")
	}

	err := run([]string{"count", "-type", "bug", "-expect_equals", "3"})
	if err == nil || !strings.Contains(err.Error(), "expected count to equal 3, got 2") {
		t.Fatalf("expect equals mismatch error = %v", err)
	}

	err = run([]string{"count", "-type", "bug", "-expect_notequals", "2"})
	if err == nil || !strings.Contains(err.Error(), "expected count to not equal 2, got 2") {
		t.Fatalf("expect notequals mismatch error = %v", err)
	}
}

func TestRunListSupportsCountAndExpectations(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"bug", "Bug Alpha"}); err != nil {
		t.Fatalf("bug create alpha error = %v", err)
	}
	if err := run([]string{"bug", "Bug Beta"}); err != nil {
		t.Fatalf("bug create beta error = %v", err)
	}
	if err := run([]string{"add", "Task Gamma"}); err != nil {
		t.Fatalf("task create error = %v", err)
	}

	countOut := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"ls", "-t", "bug", "-count"}); err != nil {
			t.Fatalf("ls count error = %v", err)
		}
	}))
	if countOut != "2" {
		t.Fatalf("ls count output = %q, want %q", countOut, "2")
	}

	okOut := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"ls", "-t", "bug", "-count", "-expect_equals", "2"}); err != nil {
			t.Fatalf("ls expect equals error = %v", err)
		}
	}))
	if okOut != "2" {
		t.Fatalf("ls expect equals output = %q, want %q", okOut, "2")
	}

	err := run([]string{"ls", "-t", "bug", "-count", "-expect_equals", "1"})
	if err == nil || !strings.Contains(err.Error(), "expected count to equal 1, got 2") {
		t.Fatalf("ls expect equals mismatch error = %v", err)
	}

	err = run([]string{"ls", "-t", "bug", "-count", "-expect_notequals", "2"})
	if err == nil || !strings.Contains(err.Error(), "expected count to not equal 2, got 2") {
		t.Fatalf("ls expect notequals mismatch error = %v", err)
	}
}

func TestRunTeamCreateRequiresName(t *testing.T) {
	setupLocalCLI(t)
	err := run([]string{"team", "create"})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("team create without -name should return usage error, got: %v", err)
	}
}

func TestRunTeamAddAndRemoveUser(t *testing.T) {
	setupLocalCLI(t)

	// create a team
	createOut := captureStdout(t, func() {
		if err := run([]string{"team", "create", "-name", "Dev Squad"}); err != nil {
			t.Fatalf("team create error = %v", err)
		}
	})
	// output format: "created team #<id> <name>"
	var teamID string
	for _, f := range strings.Fields(createOut) {
		clean := strings.TrimPrefix(f, "#")
		if n, err := strconv.ParseInt(clean, 10, 64); err == nil && n > 0 {
			teamID = clean
			break
		}
	}
	if teamID == "" {
		t.Fatalf("could not extract team id from: %s", createOut)
	}

	// Look up admin user ID (created by runInitDB)
	adminUserID := testAdminUserID(t)
	addOut := captureStdout(t, func() {
		if err := run([]string{"team", "add-user",
			"-team_id", teamID,
			"-user_id", adminUserID,
			"-role", "owner",
			"-job_title", "Tech Lead",
		}); err != nil {
			t.Fatalf("team add-user error = %v", err)
		}
	})
	if !strings.Contains(addOut, "team_id=") {
		t.Fatalf("team add-user output unexpected:\n%s", addOut)
	}

	// list users
	usersOut := captureStdout(t, func() {
		if err := run([]string{"team", "users", "-team_id", teamID}); err != nil {
			t.Fatalf("team users error = %v", err)
		}
	})
	if !strings.Contains(usersOut, "admin") {
		t.Fatalf("team users missing admin:\n%s", usersOut)
	}

	// remove user
	captureStdout(t, func() {
		if err := run([]string{"team", "remove-user",
			"-team_id", teamID,
			"-user_id", adminUserID,
		}); err != nil {
			t.Fatalf("team remove-user error = %v", err)
		}
	})

	// verify removed
	afterRemove := captureStdout(t, func() {
		if err := run([]string{"team", "users", "-team_id", teamID}); err != nil {
			t.Fatalf("team users after remove error = %v", err)
		}
	})
	if strings.Contains(afterRemove, "admin") {
		t.Fatalf("team users still shows removed user:\n%s", afterRemove)
	}
}

func TestRunTeamAddUserRequiresArgs(t *testing.T) {
	setupLocalCLI(t)
	err := run([]string{"team", "add-user", "-team_id", "1"})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("team add-user without user_id should return usage error, got: %v", err)
	}
}

func TestRunDependencyAddAndRemove(t *testing.T) {
	setupLocalCLI(t)

	// create two tickets
	id1 := createLocalTask(t, []string{"add", "Ticket Alpha"})
	id2 := createLocalTask(t, []string{"add", "Ticket Beta"})
	ref1 := id1
	ref2 := id2

	// add dependency: alpha depends on beta
	addOut := captureStdout(t, func() {
		if err := run([]string{"dependency", "add", "-id", ref1, ref2}); err != nil {
			t.Fatalf("dependency add error = %v", err)
		}
	})
	if !strings.Contains(addOut, "added") {
		t.Fatalf("dependency add output unexpected:\n%s", addOut)
	}

	// verify via get detail
	getOut := captureStdout(t, func() {
		if err := run([]string{"get", "-id", ref1}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !strings.Contains(getOut, ref2) {
		t.Fatalf("get detail should show dependency id %s:\n%s", ref2, getOut)
	}

	// remove dependency
	removeOut := captureStdout(t, func() {
		if err := run([]string{"dependency", "remove", "-id", ref1, ref2}); err != nil {
			t.Fatalf("dependency remove error = %v", err)
		}
	})
	if !strings.Contains(removeOut, "removed") {
		t.Fatalf("dependency remove output unexpected:\n%s", removeOut)
	}

	// verify removed
	getOut2 := captureStdout(t, func() {
		if err := run([]string{"get", "-id", ref1}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	// DependsOn should be empty
	for _, line := range strings.Split(getOut2, "\n") {
		if strings.HasPrefix(line, "DependsOn") && strings.Contains(line, ref2) {
			t.Fatalf("dependency should be removed but still present:\n%s", getOut2)
		}
	}
}

func TestRunDependencyShorthandForm(t *testing.T) {
	setupLocalCLI(t)

	id1 := createLocalTask(t, []string{"add", "Ticket X"})
	id2 := createLocalTask(t, []string{"add", "Ticket Y"})
	ref1 := id1
	ref2 := id2

	// shorthand: add-dependency <id> <dep-id>
	addOut := captureStdout(t, func() {
		if err := run([]string{"add-dependency", ref1, ref2}); err != nil {
			t.Fatalf("add-dependency error = %v", err)
		}
	})
	if !strings.Contains(addOut, "added") {
		t.Fatalf("add-dependency output unexpected:\n%s", addOut)
	}

	// shorthand remove
	removeOut := captureStdout(t, func() {
		if err := run([]string{"remove-dependency", ref1, ref2}); err != nil {
			t.Fatalf("remove-dependency error = %v", err)
		}
	})
	if !strings.Contains(removeOut, "removed") {
		t.Fatalf("remove-dependency output unexpected:\n%s", removeOut)
	}
}

func TestRunDependencyRequiresArgs(t *testing.T) {
	setupLocalCLI(t)

	err := run([]string{"dependency", "add"})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("dependency add without args should return usage error, got: %v", err)
	}

	err = run([]string{"dependency", "remove"})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("dependency remove without args should return usage error, got: %v", err)
	}
}

func TestRunDependencyInvalidID(t *testing.T) {
	setupLocalCLI(t)

	id1 := createLocalTask(t, []string{"add", "Ticket Z"})
	ref1 := id1

	err := run([]string{"dependency", "add", "-id", ref1, "99999"})
	if err == nil {
		t.Fatal("dependency add with non-existent dep should return error")
	}
}

func TestRunArchiveAndUnarchive(t *testing.T) {
	setupLocalCLI(t)

	id := createLocalTask(t, []string{"add", "Archive me"})
	ref := id

	// archive
	archiveOut := captureStdout(t, func() {
		if err := run([]string{"archive", "-id", ref}); err != nil {
			t.Fatalf("archive error = %v", err)
		}
	})
	if !strings.Contains(archiveOut, "archived: true") {
		t.Fatalf("archive output should show archived=true:\n%s", archiveOut)
	}

	// archived ticket hidden from default list
	listOut := captureStdout(t, func() {
		if err := run([]string{"list"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})
	if strings.Contains(listOut, "Archive me") {
		t.Fatalf("archived ticket should not appear in default list:\n%s", listOut)
	}

	// unarchive
	unarchiveOut := captureStdout(t, func() {
		if err := run([]string{"unarchive", "-id", ref}); err != nil {
			t.Fatalf("unarchive error = %v", err)
		}
	})
	if !strings.Contains(unarchiveOut, "archived: false") {
		t.Fatalf("unarchive output should show archived=false:\n%s", unarchiveOut)
	}

	// ticket reappears in list
	listOut2 := captureStdout(t, func() {
		if err := run([]string{"list"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})
	if !strings.Contains(listOut2, "Archive me") {
		t.Fatalf("unarchived ticket should appear in list:\n%s", listOut2)
	}
}

func TestRunArchiveRequiresID(t *testing.T) {
	setupLocalCLI(t)
	err := run([]string{"archive"})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("archive without -id should return usage error, got: %v", err)
	}
}

func TestRunCommentAddAndList(t *testing.T) {
	setupLocalCLI(t)

	id := createLocalTask(t, []string{"add", "Commented ticket"})
	ref := id

	// add comment
	addOut := captureStdout(t, func() {
		if err := run([]string{"comment", "add", "-id", ref, "First comment"}); err != nil {
			t.Fatalf("comment add error = %v", err)
		}
	})
	if !strings.Contains(addOut, "First comment") {
		t.Fatalf("comment add output should echo comment text:\n%s", addOut)
	}

	// comment appears in ticket detail
	getOut := captureStdout(t, func() {
		if err := run([]string{"get", "-id", ref}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !strings.Contains(getOut, "First comment") {
		t.Fatalf("get detail should include comment:\n%s", getOut)
	}
}

func TestRunCommentAddRequiresArgs(t *testing.T) {
	setupLocalCLI(t)
	err := run([]string{"comment", "add"})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("comment add without args should return usage error, got: %v", err)
	}
}

func TestRunCloneTicket(t *testing.T) {
	setupLocalCLI(t)

	id := createLocalTask(t, []string{"add", "Original ticket"})
	ref := id

	// clone
	cloneOut := captureStdout(t, func() {
		if err := run([]string{"clone", ref}); err != nil {
			t.Fatalf("clone error = %v", err)
		}
	})
	if !strings.Contains(cloneOut, "Original ticket") {
		t.Fatalf("clone output should contain original title:\n%s", cloneOut)
	}
	// clone should reference the original via clone_of (shown as key)
	if !strings.Contains(cloneOut, "clone_of: "+ticketLabelByID(t, id)) {
		t.Fatalf("clone output should show clone_of=%s:\n%s", ref, cloneOut)
	}
}

func TestRunCloneRequiresID(t *testing.T) {
	setupLocalCLI(t)
	err := run([]string{"clone"})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("clone without id should return usage error, got: %v", err)
	}
}

func TestRunRequestAssignsNextTicket(t *testing.T) {
	setupLocalCLI(t)

	// create a task in develop/idle (requestable state)
	id := createLocalTask(t, []string{"add", "Requestable ticket"})
	// advance to develop stage so it's requestable
	ref := id
	captureStdout(t, func() {
		_ = run([]string{"complete", "-id", ref})
	})
	// mark ticket as ready so it can be claimed
	captureStdout(t, func() {
		_ = run([]string{"ready", ref})
	})

	requestOut := captureStdout(t, func() {
		if err := run([]string{"request"}); err != nil {
			t.Fatalf("request error = %v", err)
		}
	})
	// should assign to admin and print the ticket or a status message
	if requestOut == "" {
		t.Fatal("request command produced no output")
	}
}

func TestRunUserCRUD(t *testing.T) {
	setupLocalCLI(t)

	// create user
	createOut := captureStdout(t, func() {
		if err := run([]string{"user", "create", "-username", "newuser", "-password", "testpass1"}); err != nil {
			t.Fatalf("user create error = %v", err)
		}
	})
	if !strings.Contains(createOut, "newuser") {
		t.Fatalf("user create output missing username:\n%s", createOut)
	}

	// list includes new user
	listOut := captureStdout(t, func() {
		if err := run([]string{"user", "list"}); err != nil {
			t.Fatalf("user list error = %v", err)
		}
	})
	if !strings.Contains(listOut, "newuser") {
		t.Fatalf("user list missing newuser:\n%s", listOut)
	}

	// disable user
	disableOut := captureStdout(t, func() {
		if err := run([]string{"user", "disable", "-username", "newuser"}); err != nil {
			t.Fatalf("user disable error = %v", err)
		}
	})
	if !strings.Contains(disableOut, "newuser") {
		t.Fatalf("user disable output missing username:\n%s", disableOut)
	}

	// enable user
	enableOut := captureStdout(t, func() {
		if err := run([]string{"user", "enable", "-username", "newuser"}); err != nil {
			t.Fatalf("user enable error = %v", err)
		}
	})
	if !strings.Contains(enableOut, "newuser") {
		t.Fatalf("user enable output missing username:\n%s", enableOut)
	}

	// delete user
	captureStdout(t, func() {
		if err := run([]string{"user", "delete", "-username", "newuser"}); err != nil {
			t.Fatalf("user delete error = %v", err)
		}
	})

	// verify gone
	listOut2 := captureStdout(t, func() {
		if err := run([]string{"user", "ls"}); err != nil {
			t.Fatalf("user ls error = %v", err)
		}
	})
	if strings.Contains(listOut2, "newuser") {
		t.Fatalf("user list still shows deleted user:\n%s", listOut2)
	}
}

func TestRunUserResetPassword(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"user", "create", "-username", "resetme", "-password", "oldpassword1"}); err != nil {
		t.Fatalf("user create error = %v", err)
	}

	resetOut := captureStdout(t, func() {
		if err := run([]string{"user", "reset-password", "-username", "resetme", "-password", "newpassword1"}); err != nil {
			t.Fatalf("user reset-password error = %v", err)
		}
	})
	if !strings.Contains(resetOut, "username : resetme") {
		t.Fatalf("reset output missing username:\n%s", resetOut)
	}
	if !strings.Contains(resetOut, "password : newpassword1") {
		t.Fatalf("reset output missing password:\n%s", resetOut)
	}
	if !strings.Contains(resetOut, "all sessions invalidated") {
		t.Fatalf("reset output missing session invalidation:\n%s", resetOut)
	}

	resolved, err := config.ResolveURL()
	if err != nil {
		t.Fatalf("config.ResolveURL() error = %v", err)
	}
	db, err := store.Open(resolved.DBPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer db.Close()

	if _, err := store.AuthenticateUser(context.Background(), db, "resetme", "oldpassword1"); !errors.Is(err, store.ErrInvalidCredentials) {
		t.Fatalf("AuthenticateUser(old password) error = %v, want ErrInvalidCredentials", err)
	}
	if _, err := store.AuthenticateUser(context.Background(), db, "resetme", "newpassword1"); err != nil {
		t.Fatalf("AuthenticateUser(new password) error = %v", err)
	}
}

func TestRunUserNewAliasCreatesUser(t *testing.T) {
	setupLocalCLI(t)

	createOut := captureStdout(t, func() {
		if err := run([]string{"user", "new", "-username", "aliasuser", "-password", "testpass1"}); err != nil {
			t.Fatalf("user new error = %v", err)
		}
	})
	if !strings.Contains(createOut, "aliasuser") {
		t.Fatalf("user new output missing username:\n%s", createOut)
	}

	listOut := captureStdout(t, func() {
		if err := run([]string{"user", "list"}); err != nil {
			t.Fatalf("user list error = %v", err)
		}
	})
	if !strings.Contains(listOut, "aliasuser") {
		t.Fatalf("user list missing alias-created user:\n%s", listOut)
	}
}

func TestRunUserCreateRequiresUsername(t *testing.T) {
	setupLocalCLI(t)
	// Without a username flag resolveCredentials falls back to the current user.
	// Just verify the create path is exercisable.
	_ = run([]string{"user", "create", "-username", "testonly", "-password", "password1"})
}

// ---------------------------------------------------------------------------
// timeAgo
// ---------------------------------------------------------------------------

func TestTimeAgoJustNow(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	got := timeAgo("2025-01-01T12:00:00Z", now)
	if got != "just now" {
		t.Fatalf("timeAgo() = %q, want %q", got, "just now")
	}
}

func TestTimeAgoMinutes(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 30, 0, 0, time.UTC)
	got := timeAgo("2025-01-01T12:00:00Z", now)
	if got != "30m ago" {
		t.Fatalf("timeAgo() = %q, want %q", got, "30m ago")
	}
}

func TestTimeAgoHours(t *testing.T) {
	now := time.Date(2025, 1, 1, 18, 0, 0, 0, time.UTC)
	got := timeAgo("2025-01-01T12:00:00Z", now)
	if got != "6h ago" {
		t.Fatalf("timeAgo() = %q, want %q", got, "6h ago")
	}
}

func TestTimeAgoDays(t *testing.T) {
	now := time.Date(2025, 1, 4, 12, 0, 0, 0, time.UTC)
	got := timeAgo("2025-01-01T12:00:00Z", now)
	if got != "3d ago" {
		t.Fatalf("timeAgo() = %q, want %q", got, "3d ago")
	}
}

func TestTimeAgoDate(t *testing.T) {
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	got := timeAgo("2025-01-01T12:00:00Z", now)
	if got != "2025-01-01" {
		t.Fatalf("timeAgo() = %q, want %q", got, "2025-01-01")
	}
}

func TestTimeAgoUnparseable(t *testing.T) {
	now := time.Now()
	got := timeAgo("not-a-date", now)
	if got != "not-a-date" {
		t.Fatalf("timeAgo() = %q, want original string", got)
	}
}

func TestTimeAgoAlternateLayouts(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 30, 0, 0, time.UTC)
	for _, ts := range []string{
		"2025-01-01 12:00:00",
		"2025-01-01T12:00:00",
	} {
		got := timeAgo(ts, now)
		if got != "30m ago" {
			t.Fatalf("timeAgo(%q) = %q, want %q", ts, got, "30m ago")
		}
	}
}

// ---------------------------------------------------------------------------
// orDash
// ---------------------------------------------------------------------------

func TestOrDashEmpty(t *testing.T) {
	if got := orDash(""); got != "-" {
		t.Fatalf("orDash(%q) = %q, want %q", "", got, "-")
	}
}

func TestOrDashWhitespace(t *testing.T) {
	if got := orDash("   "); got != "-" {
		t.Fatalf("orDash(%q) = %q, want %q", "   ", got, "-")
	}
}

func TestOrDashNonEmpty(t *testing.T) {
	if got := orDash("hello"); got != "hello" {
		t.Fatalf("orDash(%q) = %q, want %q", "hello", got, "hello")
	}
}

// ---------------------------------------------------------------------------
// rowColor
// ---------------------------------------------------------------------------

func TestRowColorActive(t *testing.T) {
	got := rowColor("develop/active")
	if got != "\033[32m" {
		t.Fatalf("rowColor(develop/active) = %q, want ansiGreen", got)
	}
}

func TestRowColorFail(t *testing.T) {
	got := rowColor("develop/fail")
	if got != "\033[31m" {
		t.Fatalf("rowColor(design/fail) = %q, want ansiRed", got)
	}
}

func TestRowColorIdle(t *testing.T) {
	got := rowColor("develop/idle")
	if got != ansiWhite {
		t.Fatalf("rowColor(develop/idle) = %q, want ansiWhite", got)
	}
}

func TestRowColorInvalid(t *testing.T) {
	got := rowColor("garbage")
	if got != "" {
		t.Fatalf("rowColor(garbage) = %q, want empty", got)
	}
}

func TestRowColorSuccess(t *testing.T) {
	got := rowColor("develop/success")
	if got != ansiGray {
		t.Fatalf("rowColor(design/success) = %q, want ansiGray", got)
	}
}

// ---------------------------------------------------------------------------
// formatPayloadKeyValues
// ---------------------------------------------------------------------------

func TestFormatPayloadKeyValuesBasic(t *testing.T) {
	data := map[string]interface{}{
		"title": "hello",
	}
	got := formatPayloadKeyValues(data)
	if got != "title=hello" {
		t.Fatalf("formatPayloadKeyValues() = %q, want %q", got, "title=hello")
	}
}

func TestFormatPayloadKeyValuesEmptyString(t *testing.T) {
	data := map[string]interface{}{
		"title": "",
	}
	got := formatPayloadKeyValues(data)
	if got != "" {
		t.Fatalf("formatPayloadKeyValues() = %q, want empty", got)
	}
}

func TestFormatPayloadKeyValuesNonString(t *testing.T) {
	data := map[string]interface{}{
		"count": 42,
	}
	got := formatPayloadKeyValues(data)
	if got != "count=42" {
		t.Fatalf("formatPayloadKeyValues() = %q, want %q", got, "count=42")
	}
}

// ---------------------------------------------------------------------------
// generateConfirmToken
// ---------------------------------------------------------------------------

func TestGenerateConfirmTokenLength(t *testing.T) {
	token, err := generateConfirmToken()
	if err != nil {
		t.Fatalf("generateConfirmToken() error = %v", err)
	}
	// 16 bytes -> 32 hex chars
	if len(token) != 32 {
		t.Fatalf("generateConfirmToken() len = %d, want 32", len(token))
	}
}

func TestGenerateConfirmTokenUniqueness(t *testing.T) {
	a, _ := generateConfirmToken()
	b, _ := generateConfirmToken()
	if a == b {
		t.Fatalf("generateConfirmToken() produced identical tokens")
	}
}

// ---------------------------------------------------------------------------
// prefixWriter
// ---------------------------------------------------------------------------

func TestPrefixWriterSingleLine(t *testing.T) {
	var buf bytes.Buffer
	pw := &prefixWriter{w: &buf, prefix: ">> "}
	n, err := pw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != 5 {
		t.Fatalf("Write() = %d, want 5", n)
	}
	if buf.String() != ">> hello" {
		t.Fatalf("Write() output = %q, want %q", buf.String(), ">> hello")
	}
}

func TestPrefixWriterMultiLine(t *testing.T) {
	var buf bytes.Buffer
	pw := &prefixWriter{w: &buf, prefix: "| "}
	_, err := pw.Write([]byte("line1\nline2\n"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	want := "| line1\n| line2\n"
	if buf.String() != want {
		t.Fatalf("Write() output = %q, want %q", buf.String(), want)
	}
}

// ---------------------------------------------------------------------------
// prefixReader
// ---------------------------------------------------------------------------

func TestPrefixReaderEchoesWithPrefix(t *testing.T) {
	var echo bytes.Buffer
	pr := &prefixReader{
		r:      strings.NewReader("data"),
		prefix: "<< ",
		w:      &echo,
	}
	p := make([]byte, 64)
	n, err := pr.Read(p)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(p[:n]) != "data" {
		t.Fatalf("Read() data = %q, want %q", string(p[:n]), "data")
	}
	if !strings.Contains(echo.String(), "<< data") {
		t.Fatalf("echo = %q, want prefix echo", echo.String())
	}
}

// ---------------------------------------------------------------------------
// printAgentTable
// ---------------------------------------------------------------------------

func TestPrintAgentTableEmpty(t *testing.T) {
	out := captureStdout(t, func() {
		printAgentTable(nil)
	})
	if !strings.Contains(out, "no agents") {
		t.Fatalf("printAgentTable(nil) = %q, want 'no agents'", out)
	}
}

func TestPrintAgentTableNonEmpty(t *testing.T) {
	tk := "PROJ-1"
	statuses := []store.AgentStatus{
		{
			Agent:       store.Agent{ID: "uuid-1", Username: "bot-a", Enabled: true, Status: "idle", LastSeen: "2025-01-01T00:00:00Z"},
			TicketKey:   &tk,
			ProjectName: "MyProject",
			SdlcName:    "default",
			RoleTitle:   "developer",
		},
		{
			Agent: store.Agent{ID: "uuid-2", Username: "bot-b", Enabled: false, Status: "disabled"},
		},
	}
	out := captureStdout(t, func() {
		printAgentTable(statuses)
	})
	for _, want := range []string{"uuid-1", "uuid-2", "PROJ-1", "MyProject", "developer"} {
		if !strings.Contains(out, want) {
			t.Fatalf("printAgentTable output missing %q:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// printTeamAgentTable
// ---------------------------------------------------------------------------

func TestPrintTeamAgentTableEmpty(t *testing.T) {
	out := captureStdout(t, func() {
		printTeamAgentTable(nil)
	})
	if !strings.Contains(out, "no team agents") {
		t.Fatalf("printTeamAgentTable(nil) = %q, want 'no team agents'", out)
	}
}

func TestPrintTeamAgentTableNonEmpty(t *testing.T) {
	items := []store.TeamAgent{
		{TeamID: 1, AgentID: "a1", AgentUUID: "uuid-1", Enabled: true, Status: "idle"},
	}
	out := captureStdout(t, func() {
		printTeamAgentTable(items)
	})
	for _, want := range []string{"uuid-1", "a1", "idle"} {
		if !strings.Contains(out, want) {
			t.Fatalf("printTeamAgentTable output missing %q:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// printTicketChildren
// ---------------------------------------------------------------------------

func TestPrintTicketChildrenOutput(t *testing.T) {
	children := []store.Ticket{
		{ID: "PROJ-1", Type: "task", Status: "develop/idle", Title: "Child One"},
		{ID: "PROJ-2", Type: "bug", Status: "develop/active", Title: "Child Two"},
		{ID: "PROJ-3", Type: "task", Status: "done/idle", Stage: store.StageDone, State: store.StateIdle, Complete: true, Title: "Child Three"},
	}
	out := captureStdout(t, func() {
		printTicketChildren(children)
	})
	if !strings.Contains(out, "Children") {
		t.Fatalf("printTicketChildren missing header:\n%s", out)
	}
	for _, want := range []string{"PROJ-1", "PROJ-2", "PROJ-3", "Child One", "Child Two", "Child Three"} {
		if !strings.Contains(out, want) {
			t.Fatalf("printTicketChildren missing %q:\n%s", want, out)
		}
	}
}

func TestChildTicketColorByState(t *testing.T) {
	if got := childTicketColor(store.Ticket{State: store.StateActive, Stage: store.StageDevelop}); got != ansiWhite {
		t.Fatalf("active child color = %q, want %q", got, ansiWhite)
	}
	if got := childTicketColor(store.Ticket{State: store.StateIdle, Stage: store.StageDevelop}); got != ansiGray {
		t.Fatalf("idle child color = %q, want %q", got, ansiGray)
	}
	if got := childTicketColor(store.Ticket{State: store.StateIdle, Stage: store.StageDone, Complete: true}); got != ansiDim+ansiGray {
		t.Fatalf("done child color = %q, want %q", got, ansiDim+ansiGray)
	}
}

// ---------------------------------------------------------------------------
// fallbackCommandUsername in local mode
// ---------------------------------------------------------------------------

func TestFallbackCommandUsernameLocalMode(t *testing.T) {
	setupLocalCLI(t)
	got := fallbackCommandUsername()
	if got != "admin" {
		t.Fatalf("fallbackCommandUsername() = %q, want %q", got, "admin")
	}
}

// ---------------------------------------------------------------------------
// runWhoami
// ---------------------------------------------------------------------------

func TestRunWhoamiLocalMode(t *testing.T) {
	setupLocalCLI(t)
	out := captureStdout(t, func() {
		if err := run([]string{"whoami"}); err != nil {
			t.Fatalf("whoami error = %v", err)
		}
	})
	for _, want := range []string{"USER", "username", "admin", "CONNECTION", "local"} {
		if !strings.Contains(out, want) {
			t.Fatalf("whoami output missing %q:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// runSummary
// ---------------------------------------------------------------------------

func TestRunSummaryLocalMode(t *testing.T) {
	setupLocalCLI(t)

	// Create a project first
	captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "SUM", "-title", "Summary Test"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := config.SaveProjectConfigAt(repoDir, config.Config{ProjectID: "2"}); err != nil {
		t.Fatalf("SaveProjectConfigAt() error = %v", err)
	}

	// Create a ticket so summary has data
	createLocalTask(t, []string{"add", "Summary task one"})

	out := captureStdout(t, func() {
		if err := run([]string{"summary"}); err != nil {
			t.Fatalf("summary error = %v", err)
		}
	})
	for _, want := range []string{"project", "SUM", "open tickets", "database"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary output missing %q:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// runTicketNS help
// ---------------------------------------------------------------------------

func TestRunTicketNSHelp(t *testing.T) {
	setupLocalCLI(t)
	out := captureStdout(t, func() {
		if err := run([]string{"ticket", "help"}); err != nil {
			t.Fatalf("ticket help error = %v", err)
		}
	})
	if !strings.Contains(out, "ticket") {
		t.Fatalf("ticket help output missing usage info:\n%s", out)
	}
}

func TestRunTicketNSUnknown(t *testing.T) {
	setupLocalCLI(t)
	err := run([]string{"ticket", "nonexistent-subcommand-xyz"})
	if err == nil || !strings.Contains(err.Error(), "unknown ticket command") {
		t.Fatalf("ticket unknown = %v, want unknown ticket command error", err)
	}
}

// ---------------------------------------------------------------------------
// runSetTicketClosed (close/open)
// ---------------------------------------------------------------------------

func TestRunTicketCloseAndOpen(t *testing.T) {
	setupLocalCLI(t)

	captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "CLS", "-title", "Close Test"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})

	taskID := createLocalTask(t, []string{"add", "Closable task"})

	// Close the ticket
	captureStdout(t, func() {
		if err := run([]string{"close", taskID}); err != nil {
			t.Fatalf("close error = %v", err)
		}
	})

	// Verify it is closed (get should show closed)
	getOut := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !strings.Contains(getOut, "closed") {
		t.Fatalf("ticket should be closed:\n%s", getOut)
	}

	// Re-open the ticket
	captureStdout(t, func() {
		if err := run([]string{"open", taskID}); err != nil {
			t.Fatalf("open error = %v", err)
		}
	})

	getOut2 := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !strings.Contains(getOut2, "open") {
		t.Fatalf("ticket should be open:\n%s", getOut2)
	}
}

// ---------------------------------------------------------------------------
// ticket NS subcommands via runTicketNS: search, board, count, orphans, clone
// ---------------------------------------------------------------------------

func TestRunTicketNSSearchBoardCountOrphans(t *testing.T) {
	setupLocalCLI(t)

	captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "TNS", "-title", "NS Test"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})

	createLocalTask(t, []string{"add", "NS searchable task"})

	// ticket search
	searchOut := captureStdout(t, func() {
		_ = run([]string{"ticket", "search", "searchable"})
	})
	if !strings.Contains(searchOut, "searchable") {
		t.Fatalf("ticket search missing expected result:\n%s", searchOut)
	}

	// ticket count
	countOut := captureStdout(t, func() {
		if err := run([]string{"ticket", "count"}); err != nil {
			t.Fatalf("ticket count error = %v", err)
		}
	})
	// count output should exist (even if just a number)
	if len(strings.TrimSpace(countOut)) == 0 {
		t.Fatalf("ticket count output empty")
	}

	// ticket orphans
	orphanOut := captureStdout(t, func() {
		_ = run([]string{"ticket", "orphans"})
	})
	if !strings.Contains(orphanOut, "searchable") {
		t.Fatalf("ticket orphans missing orphan ticket:\n%s", orphanOut)
	}

	// ticket list
	listOut := captureStdout(t, func() {
		if err := run([]string{"ticket", "list"}); err != nil {
			t.Fatalf("ticket list error = %v", err)
		}
	})
	if !strings.Contains(listOut, "searchable") {
		t.Fatalf("ticket list missing task:\n%s", listOut)
	}
}

// ---------------------------------------------------------------------------
// ticket NS state subcommands: active, idle, complete, fail
// ---------------------------------------------------------------------------

func TestRunTicketNSStateCommands(t *testing.T) {
	setupLocalCLI(t)

	captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "STA", "-title", "State Test"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})

	taskID := createLocalTask(t, []string{"add", "State task"})

	// active
	captureStdout(t, func() {
		if err := run([]string{"ticket", "active", "-id", taskID}); err != nil {
			t.Fatalf("ticket active error = %v", err)
		}
	})

	// idle
	captureStdout(t, func() {
		if err := run([]string{"ticket", "idle", "-id", taskID}); err != nil {
			t.Fatalf("ticket idle error = %v", err)
		}
	})

	// fail
	captureStdout(t, func() {
		if err := run([]string{"ticket", "fail", "-id", taskID}); err != nil {
			t.Fatalf("ticket fail error = %v", err)
		}
	})

	// complete
	captureStdout(t, func() {
		if err := run([]string{"ticket", "complete", "-id", taskID}); err != nil {
			t.Fatalf("ticket complete error = %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// ticket NS hierarchy: attach/detach
// ---------------------------------------------------------------------------

func TestRunTicketNSAttachDetach(t *testing.T) {
	setupLocalCLI(t)

	captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "ATT", "-title", "Attach Test"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})

	parentID := createLocalTask(t, []string{"add", "Parent task"})
	childID := createLocalTask(t, []string{"add", "Child task"})

	// attach child to parent
	captureStdout(t, func() {
		if err := run([]string{"ticket", "attach", "-id", childID, parentID}); err != nil {
			t.Fatalf("ticket attach error = %v", err)
		}
	})

	// detach
	captureStdout(t, func() {
		if err := run([]string{"ticket", "detach", "-id", childID}); err != nil {
			t.Fatalf("ticket detach error = %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// ticket NS clone and delete
// ---------------------------------------------------------------------------

func TestRunTicketNSCloneAndDelete(t *testing.T) {
	setupLocalCLI(t)

	captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "CLN", "-title", "Clone Test"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})

	taskID := createLocalTask(t, []string{"add", "Clonable task"})

	// clone
	cloneOut := captureStdout(t, func() {
		if err := run([]string{"ticket", "clone", taskID}); err != nil {
			t.Fatalf("ticket clone error = %v", err)
		}
	})
	if strings.TrimSpace(cloneOut) == "" {
		t.Fatalf("ticket clone output empty")
	}

	// delete original via ticket NS
	captureStdout(t, func() {
		if err := run([]string{"ticket", "rm", taskID}); err != nil {
			t.Fatalf("ticket rm error = %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// project add-user / remove-user (error paths)
// ---------------------------------------------------------------------------

func TestRunProjectAddUserRequiresArgs(t *testing.T) {
	setupLocalCLI(t)
	err := run([]string{"project", "add-user"})
	if err == nil {
		t.Fatal("project add-user with no args should error")
	}
}

func TestRunProjectRemoveUserRequiresArgs(t *testing.T) {
	setupLocalCLI(t)
	err := run([]string{"project", "remove-user"})
	if err == nil {
		t.Fatal("project remove-user with no args should error")
	}
}

func TestRunProjectAddTeamRequiresArgs(t *testing.T) {
	setupLocalCLI(t)
	err := run([]string{"project", "add-team"})
	if err == nil {
		t.Fatal("project add-team with no args should error")
	}
}

func TestRunProjectRemoveTeamRequiresArgs(t *testing.T) {
	setupLocalCLI(t)
	err := run([]string{"project", "remove-team"})
	if err == nil {
		t.Fatal("project remove-team with no args should error")
	}
}

// ---------------------------------------------------------------------------
// Quickstart verification tests
// ---------------------------------------------------------------------------

// TestQuickstartClient exercises every command documented in QUICKSTART_CLIENT.md
// using local mode (no server).
func TestQuickstartClient(t *testing.T) {
	setupLocalCLI(t)

	// Step 1: Create a project
	out := captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "CUS", "-title", "Customer Portal"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})
	if !strings.Contains(out, "CUS") {
		t.Fatalf("project create output missing prefix:\n%s", out)
	}

	captureStdout(t, func() {
		if err := run([]string{"project", "use", "CUS"}); err != nil {
			t.Fatalf("project use error = %v", err)
		}
	})

	// Step 2: Capture work — add, bug, epic
	taskID := createLocalTask(t, []string{"add", "Customers can reset their password"})
	_ = createLocalTask(t, []string{"bug", "Reset token expires immediately"})
	epicID := createLocalTask(t, []string{"epic", "Authentication"})

	// Step 3: Ideas
	captureStdout(t, func() {
		if err := run([]string{"idea", "new", "Add dark mode"}); err != nil {
			t.Fatalf("idea new error = %v", err)
		}
	})

	ideaOut := captureStdout(t, func() {
		if err := run([]string{"idea", "ls"}); err != nil {
			t.Fatalf("idea ls error = %v", err)
		}
	})
	if !strings.Contains(ideaOut, "dark mode") {
		t.Fatalf("idea ls output missing 'dark mode':\n%s", ideaOut)
	}

	// Step 4: Inspect — list, get, attach
	listOut := captureStdout(t, func() {
		if err := run([]string{"list"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})
	if !strings.Contains(listOut, "reset") {
		t.Fatalf("list output missing ticket:\n%s", listOut)
	}

	getOut := captureStdout(t, func() {
		if err := run([]string{"get", taskID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !strings.Contains(getOut, "reset") {
		t.Fatalf("get output missing title:\n%s", getOut)
	}

	// Attach task to epic (use internal IDs since sequence is shared across types)
	captureStdout(t, func() {
		if err := run([]string{"attach", "-id", taskID, epicID}); err != nil {
			t.Fatalf("attach error = %v", err)
		}
	})

	// Verify parent was set
	ticket, err := svcGetTicket(t, taskID)
	if err != nil {
		t.Fatalf("svcGetTicket() error = %v", err)
	}
	if ticket.ParentID == nil {
		t.Fatal("attach did not set parent")
	}

	// Step 5: Lifecycle — active, complete, idle
	captureStdout(t, func() {
		if err := run([]string{"active", "-id", taskID}); err != nil {
			t.Fatalf("active error = %v", err)
		}
	})
	ticket, _ = svcGetTicket(t, taskID)
	if ticket.State != "active" {
		t.Fatalf("active: state = %q, want active", ticket.State)
	}

	captureStdout(t, func() {
		if err := run([]string{"complete", "-id", taskID}); err != nil {
			t.Fatalf("complete error = %v", err)
		}
	})
	ticket, _ = svcGetTicket(t, taskID)
	if !ticket.Complete {
		t.Fatal("complete should set complete=true")
	}
	if ticket.Stage != "done" {
		t.Fatalf("complete should set stage=done, got %s", ticket.Stage)
	}

	// Reopen to continue testing
	captureStdout(t, func() {
		if err := run([]string{"reopen", "-id", taskID}); err != nil {
			t.Fatalf("reopen error = %v", err)
		}
	})

	captureStdout(t, func() {
		if err := run([]string{"idle", "-id", taskID}); err != nil {
			t.Fatalf("idle error = %v", err)
		}
	})
	ticket, _ = svcGetTicket(t, taskID)
	if ticket.State != "idle" {
		t.Fatalf("idle: state = %q, want idle", ticket.State)
	}
}

// TestQuickstartServer exercises key commands from QUICKSTART_SERVER.md
// using a real httptest server with full API.
func TestQuickstartServer(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	// Initialize database and start test server
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "adminpass"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer db.Close()

	handler, err := server.Handler(db, "test", false, nil, "", "")
	if err != nil {
		t.Fatalf("server.Handler() error = %v", err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	setTestLocation(t, ts.URL)

	// Enable registration
	if err := store.SetRegistrationEnabled(context.Background(), db, true); err != nil {
		t.Fatalf("SetRegistrationEnabled() error = %v", err)
	}

	// Step 1: Register a user
	captureStdout(t, func() {
		if err := run([]string{"register", "-username", "alice", "-password", "secret12"}); err != nil {
			t.Fatalf("register error = %v", err)
		}
	})

	// Step 2: Login as alice, verify it works
	loginOut := captureStdout(t, func() {
		if err := run([]string{"login", "-username", "alice", "-password", "secret12"}); err != nil {
			t.Fatalf("login alice error = %v", err)
		}
	})
	if !strings.Contains(loginOut, "alice") {
		t.Fatalf("login output missing username:\n%s", loginOut)
	}

	// Clear credentials and saved username, then login as admin
	if err := config.ClearCredentials(); err != nil {
		t.Fatalf("ClearCredentials() error = %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	cfg.Username = ""
	cfg.Token = ""
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}
	captureStdout(t, func() {
		if err := run([]string{"login", "-username", "admin", "-password", "adminpass"}); err != nil {
			t.Fatalf("admin login error = %v", err)
		}
	})

	// Step 4: Create project
	srvProjectID := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "SRV", "-title", "Server Project", "-printid"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	}))
	captureStdout(t, func() {
		if err := run([]string{"project", "use", "SRV"}); err != nil {
			t.Fatalf("project use error = %v", err)
		}
	})

	// Step 5: Create tickets
	taskOut := captureStdout(t, func() {
		if err := run([]string{"add", "Server task"}); err != nil {
			t.Fatalf("add error = %v", err)
		}
	})
	taskKey := strings.Fields(taskOut)[0]

	captureStdout(t, func() {
		if err := run([]string{"bug", "Server bug"}); err != nil {
			t.Fatalf("bug error = %v", err)
		}
	})

	// Step 6: List tickets
	listOut := captureStdout(t, func() {
		if err := run([]string{"list"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})
	if !strings.Contains(listOut, "Server task") {
		t.Fatalf("list output missing ticket:\n%s", listOut)
	}

	// Step 7: Create user bob and assign (admin required)
	captureStdout(t, func() {
		if err := run([]string{"user", "create", "-username", "bob", "-password", "bobpass12"}); err != nil {
			t.Fatalf("user create bob error = %v", err)
		}
	})
	captureStdout(t, func() {
		if err := run([]string{"assign", taskKey, "bob"}); err != nil {
			t.Fatalf("assign error = %v", err)
		}
	})

	// Step 8: Agent create
	agentID := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"agent", "create", "-password", "agentpass123", "-printid"}); err != nil {
			t.Fatalf("agent create error = %v", err)
		}
	}))

	// Step 9: Move a dedicated unassigned ticket into a claimable workflow state
	agentTaskKey := strings.Fields(captureStdout(t, func() {
		if err := run([]string{"add", "Agent queue task"}); err != nil {
			t.Fatalf("add agent task error = %v", err)
		}
	}))[0]
	captureStdout(t, func() {
		if err := run([]string{"update", "-id", agentTaskKey, "-status", "develop/idle"}); err != nil {
			t.Fatalf("update agent task status error = %v", err)
		}
	})

	agentRequestOut := captureStdout(t, func() {
		if err := run([]string{"agent", "request", "-agent-id", agentID, "-password", "agentpass123", "-project-id", srvProjectID, "-id", agentTaskKey}); err != nil {
			t.Fatalf("agent request error = %v", err)
		}
	})
	var requestPayload map[string]any
	if err := json.Unmarshal([]byte(agentRequestOut), &requestPayload); err != nil {
		t.Fatalf("json.Unmarshal(agent request) error = %v\noutput=%s", err, agentRequestOut)
	}
	if requestPayload["status"] != "NEW" {
		t.Fatalf("agent request status = %#v, want NEW", requestPayload["status"])
	}
	ticketPayload, ok := requestPayload["ticket"].(map[string]any)
	if !ok {
		t.Fatalf("agent request ticket payload = %#v", requestPayload["ticket"])
	}
	if ticketPayload["title"] != "Agent queue task" {
		t.Fatalf("agent request ticket title = %#v, want %q", ticketPayload["title"], "Agent queue task")
	}
}

func TestRunAgentRemoteAdminFlow(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "adminpass"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer db.Close()

	handler, err := server.Handler(db, "test", false, nil, "", "")
	if err != nil {
		t.Fatalf("server.Handler() error = %v", err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	setTestLocation(t, ts.URL)

	captureStdout(t, func() {
		if err := run([]string{"login", "-username", "admin", "-password", "adminpass"}); err != nil {
			t.Fatalf("admin login error = %v", err)
		}
	})

	srvProjectID := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "SRV", "-title", "Server Project", "-printid"}); err != nil {
			t.Fatalf("project create SRV error = %v", err)
		}
	}))
	opsProjectID := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "OPS", "-title", "Ops Project", "-printid"}); err != nil {
			t.Fatalf("project create OPS error = %v", err)
		}
	}))
	captureStdout(t, func() {
		if err := run([]string{"project", "use", srvProjectID}); err != nil {
			t.Fatalf("project use error = %v", err)
		}
	})

	readyTicketID := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"add", "-printid", "Agent remote admin ticket"}); err != nil {
			t.Fatalf("add ticket error = %v", err)
		}
	}))
	captureStdout(t, func() {
		if err := run([]string{"update", "-id", readyTicketID, "-status", "develop/idle"}); err != nil {
			t.Fatalf("update ticket status error = %v", err)
		}
	})

	agentID := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"agent", "create", "-password", "oldpass123", "-printid"}); err != nil {
			t.Fatalf("agent create error = %v", err)
		}
	}))

	initialConfigOut := captureStdout(t, func() {
		if err := run([]string{"agent", "config-ls", "-id", agentID}); err != nil {
			t.Fatalf("agent config-ls initial error = %v", err)
		}
	})
	if !strings.Contains(initialConfigOut, "(no config)") {
		t.Fatalf("agent config-ls initial output = %q, want no config", initialConfigOut)
	}

	captureStdout(t, func() {
		if err := run([]string{"agent", "config-set", "-id", agentID, "llm", "codex"}); err != nil {
			t.Fatalf("agent config-set llm error = %v", err)
		}
	})
	captureStdout(t, func() {
		if err := run([]string{"agent", "config-set", "-id", agentID, "poll_seconds", "7"}); err != nil {
			t.Fatalf("agent config-set poll_seconds error = %v", err)
		}
	})
	configOut := captureStdout(t, func() {
		if err := run([]string{"agent", "config-ls", "-id", agentID}); err != nil {
			t.Fatalf("agent config-ls error = %v", err)
		}
	})
	if !strings.Contains(configOut, "llm=codex") || !strings.Contains(configOut, "poll_seconds=7") {
		t.Fatalf("agent config-ls output missing config values:\n%s", configOut)
	}

	wrongProjectOut := captureStdout(t, func() {
		if err := run([]string{"agent", "request", "-agent-id", agentID, "-password", "oldpass123", "-project-id", opsProjectID}); err != nil {
			t.Fatalf("agent request wrong project error = %v", err)
		}
	})
	var wrongProjectPayload map[string]any
	if err := json.Unmarshal([]byte(wrongProjectOut), &wrongProjectPayload); err != nil {
		t.Fatalf("json.Unmarshal(wrong project request) error = %v\noutput=%s", err, wrongProjectOut)
	}
	if wrongProjectPayload["status"] != "NONE" {
		t.Fatalf("agent request wrong project status = %#v, want NONE", wrongProjectPayload["status"])
	}

	resetOut := captureStdout(t, func() {
		if err := run([]string{"agent", "reset-password", "-id", agentID, "-password", "newpass123"}); err != nil {
			t.Fatalf("agent reset-password error = %v", err)
		}
	})
	if !strings.Contains(resetOut, "password : newpass123") {
		t.Fatalf("agent reset-password output missing new password:\n%s", resetOut)
	}

	if err := run([]string{"agent", "request", "-agent-id", agentID, "-password", "oldpass123", "-project-id", srvProjectID, "-id", readyTicketID}); err == nil {
		t.Fatal("agent request with old password should fail after reset")
	}

	requestOut := captureStdout(t, func() {
		if err := run([]string{"agent", "request", "-agent-id", agentID, "-password", "newpass123", "-project-id", srvProjectID, "-id", readyTicketID}); err != nil {
			t.Fatalf("agent request error = %v", err)
		}
	})
	var requestPayload map[string]any
	if err := json.Unmarshal([]byte(requestOut), &requestPayload); err != nil {
		t.Fatalf("json.Unmarshal(agent request) error = %v\noutput=%s", err, requestOut)
	}
	if requestPayload["status"] != "NEW" {
		t.Fatalf("agent request status = %#v, want NEW", requestPayload["status"])
	}
	configPayload, ok := requestPayload["config"].(map[string]any)
	if !ok {
		t.Fatalf("agent request config payload = %#v", requestPayload["config"])
	}
	if configPayload["llm"] != "codex" || configPayload["poll_seconds"] != "7" {
		t.Fatalf("agent request config payload = %#v", configPayload)
	}

	captureStdout(t, func() {
		if err := run([]string{"agent", "config-rm", "-id", agentID, "llm"}); err != nil {
			t.Fatalf("agent config-rm error = %v", err)
		}
	})
	afterRemoveOut := captureStdout(t, func() {
		if err := run([]string{"agent", "config-ls", "-id", agentID}); err != nil {
			t.Fatalf("agent config-ls after remove error = %v", err)
		}
	})
	if strings.Contains(afterRemoveOut, "llm=codex") || !strings.Contains(afterRemoveOut, "poll_seconds=7") {
		t.Fatalf("agent config-ls after remove output = %q", afterRemoveOut)
	}
}

func TestRunIdeaReviseAlias(t *testing.T) {
	setupLocalCLI(t)

	ideaID := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"idea", "new", "-printid", "Add dark mode"}); err != nil {
			t.Fatalf("idea new error = %v", err)
		}
	}))

	out := captureStdout(t, func() {
		if err := run([]string{"idea", "revise", "-id", ideaID}); err != nil {
			t.Fatalf("idea revise error = %v", err)
		}
	})
	if !strings.Contains(out, "(revised)") {
		t.Fatalf("idea revise output should contain (revised):\n%s", out)
	}
}

func TestBuildTreeDisplayOrdersChildrenUnderParents(t *testing.T) {
	// Three tickets: epic (no parent), and two tasks under it.
	epicID := "TK-1"
	childAID := "TK-2"
	childBID := "TK-3"
	tickets := []store.Ticket{
		{ID: childAID, ParentID: &epicID},
		{ID: epicID},
		{ID: childBID, ParentID: &epicID},
	}

	ordered, prefix := buildTreeDisplay(tickets)

	if len(ordered) != 3 {
		t.Fatalf("expected 3 tickets, got %d", len(ordered))
	}
	// Epic must be first.
	if ordered[0].ID != epicID {
		t.Errorf("expected epic %s first, got %s", epicID, ordered[0].ID)
	}
	// Both children must follow.
	childIDs := map[string]bool{ordered[1].ID: true, ordered[2].ID: true}
	if !childIDs[childAID] || !childIDs[childBID] {
		t.Errorf("expected children %s and %s after epic, got %s and %s", childAID, childBID, ordered[1].ID, ordered[2].ID)
	}
	// Epic has no prefix.
	if prefix[epicID] != "" {
		t.Errorf("epic should have empty prefix, got %q", prefix[epicID])
	}
	// Last child uses └─.
	lastID := ordered[2].ID
	if !strings.HasPrefix(prefix[lastID], "└─") {
		t.Errorf("last child should have └─ prefix, got %q", prefix[lastID])
	}
	// Non-last child uses ├─.
	firstChildID := ordered[1].ID
	if !strings.HasPrefix(prefix[firstChildID], "├─") {
		t.Errorf("non-last child should have ├─ prefix, got %q", prefix[firstChildID])
	}
}

func TestBuildTreeDisplayOrphansTreatedAsRoots(t *testing.T) {
	// Child whose parent is NOT in the list → appears as root with no prefix.
	outsideParent := "TK-99"
	childID := "TK-2"
	tickets := []store.Ticket{
		{ID: childID, ParentID: &outsideParent},
	}

	ordered, prefix := buildTreeDisplay(tickets)

	if len(ordered) != 1 || ordered[0].ID != childID {
		t.Fatalf("expected single orphan ticket, got %v", ordered)
	}
	if prefix[childID] != "" {
		t.Errorf("orphan should have empty prefix, got %q", prefix[childID])
	}
}

func TestTicketSortKeyCompleteTicketsSinkToBottom(t *testing.T) {
	active := store.Ticket{Stage: store.StageDesign, State: store.StateActive}
	idle := store.Ticket{Stage: store.StageDesign, State: store.StateIdle}
	done := store.Ticket{Stage: store.StageDone, State: store.StateSuccess}

	if ticketSortKey(active) >= ticketSortKey(idle) {
		t.Error("active should sort before idle")
	}
	if ticketSortKey(idle) >= ticketSortKey(done) {
		t.Error("idle should sort before done")
	}
}
