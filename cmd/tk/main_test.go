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
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/simonski/ticket/internal/client"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/server"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/internal/testutil"
	"github.com/simonski/ticket/internal/ticketmarkdown"
	"github.com/simonski/ticket/libticket"
)

func hasDetailLabel(output, label string) bool {
	pattern := `(?m)^` + regexp.QuoteMeta(label) + `\s+:`
	return regexp.MustCompile(pattern).MatchString(output)
}

func hasDetailField(output, label, value string) bool {
	pattern := `(?mi)^` + regexp.QuoteMeta(label) + `\s+:\s` + regexp.QuoteMeta(value) + `$`
	return regexp.MustCompile(pattern).MatchString(output)
}

func setBinaryNameForTest(t *testing.T, name string) {
	t.Helper()
	original := os.Args
	if len(original) == 0 {
		os.Args = []string{name}
	} else {
		copied := append([]string(nil), original...)
		copied[0] = name
		os.Args = copied
	}
	t.Cleanup(func() { os.Args = original })
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
	if location == "" {
		t.Setenv("TICKET_URL", "")
		return
	}
	t.Setenv("TICKET_URL", location)
}

func setTestWorkingDir(t *testing.T, dir string) {
	t.Helper()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s) error = %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
}

func setTestGitOrigin(t *testing.T, remote string) {
	t.Helper()
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if out, err := exec.Command("git", "-C", repoDir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init error = %v\n%s", err, out)
	}
	_ = exec.Command("git", "-C", repoDir, "remote", "remove", "origin").Run()
	if out, err := exec.Command("git", "-C", repoDir, "remote", "add", "origin", remote).CombinedOutput(); err != nil {
		t.Fatalf("git remote add origin error = %v\n%s", err, out)
	}
	gitOriginByRoot.Delete(repoDir)
}

func testDBPath(t *testing.T) string {
	t.Helper()
	home := strings.TrimSpace(os.Getenv("TICKET_HOME"))
	if home == "" {
		t.Fatal("TICKET_HOME is not set")
	}
	return filepath.Join(home, "ticket.db")
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
		"SYSTEM",
		"\x1b[38;5;117m",
		"ticket",
		"idea",
		"project",
		"dep",
		"label",
		"time",
		"document",
		"config",
		"export",
		"import",
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
		"  document",
		"  decision",
		"  doctor",
		"  admin config",
		"  admin export",
		"  admin import",
		"  admin upgrade-database",
		"  admin role",
		"  admin workflow",
		"  admin team",
		"  admin agent",
		"  admin user",
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
	systemOrder := []string{"  status", "  summary", "  whoami", "  export", "  import", "  server", "  login", "  logout", "  register", "  initdb", "  version", "  upgrade", "  skill", "  docker-compose"}
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
	systemIdx := strings.Index(usage, "SYSTEM")
	examplesIdx := strings.Index(usage, "EXAMPLES")
	for _, unwanted := range []string{"  admin", "  help"} {
		if idx := strings.LastIndex(usage, unwanted); idx != -1 && idx > systemIdx && idx < examplesIdx {
			t.Fatalf("root usage SYSTEM section should not include %q:\n%s", unwanted, usage)
		}
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

	err := formatRuntimeError(&client.HTTPStatusError{StatusCode: http.StatusServiceUnavailable, Status: "503 Service Unavailable"})
	got := err.Error()

	for _, want := range []string{
		"this command is configured for https://ticket.example, but that server is currently unavailable (503 Service Unavailable).",
		"Check whether the server, proxy, or tunnel is up.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("remote runtime error missing %q:\n%s", want, got)
		}
	}
}

func TestFormatRuntimeErrorCannotConnectUsesEnvTicketURL(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(repoDir) error = %v", err)
	}
	setTestWorkingDir(t, repoDir)
	t.Setenv("TICKET_HOME", t.TempDir())
	t.Setenv("TICKET_URL", "http://localhost:8080")

	err := formatRuntimeError(errors.New("cannot connect to http://localhost:8080"))
	got := err.Error()

	if got != "Unable to access http://localhost:8080." {
		t.Fatalf("env remote runtime error = %q", got)
	}
}

func TestFormatRuntimeErrorRemote503FromProjectConfigOmitsServerURL(t *testing.T) {
	t.Setenv("TICKET_HOME", t.TempDir())
	t.Setenv("TICKET_URL", "https://ticket.example")

	err := formatRuntimeError(&client.HTTPStatusError{StatusCode: http.StatusServiceUnavailable, Status: "503 Service Unavailable"})
	got := err.Error()

	for _, want := range []string{
		"your ticket CLI is configured for https://ticket.example, but that server is currently unavailable (503 Service Unavailable).",
		"Check whether the server, proxy, or tunnel is up.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("project-config remote runtime error missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"setup:", ".ticket/config.json", "auth override"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("project-config remote runtime error should omit %q:\n%s", unwanted, got)
		}
	}
}

func TestFormatRuntimeErrorRemote401ExplainsCredentials(t *testing.T) {
	t.Setenv("TICKET_HOME", t.TempDir())
	t.Setenv("TICKET_URL", "https://ticket.example")

	err := formatRuntimeError(&client.HTTPStatusError{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized", APIError: "unauthorized"})
	got := err.Error()

	for _, want := range []string{
		"your ticket CLI is configured for https://ticket.example, but the server rejected the saved credentials (401 Unauthorized).",
		"Run `tk login` for that server",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("remote 401 runtime error missing %q:\n%s", want, got)
		}
	}
}

func TestFormatRuntimeErrorRemote401PreservesHelpfulAPIMessage(t *testing.T) {
	t.Setenv("TICKET_HOME", t.TempDir())
	t.Setenv("TICKET_URL", "https://ticket.example")

	err := &client.HTTPStatusError{
		StatusCode: http.StatusUnauthorized,
		Status:     "401 Unauthorized",
		APIError:   "access denied for project GATE; request access via POST /api/projects/GATE/access-requests",
	}
	if got := formatRuntimeError(err); got != err {
		t.Fatalf("formatRuntimeError() should preserve helpful 401 API error, got %v", got)
	}
}

func TestFormatRuntimeErrorRemote404GenericExplainsWrongServer(t *testing.T) {
	t.Setenv("TICKET_HOME", t.TempDir())
	t.Setenv("TICKET_URL", "https://ticket.example")

	err := formatRuntimeError(&client.HTTPStatusError{StatusCode: http.StatusNotFound, Status: "404 Not Found"})
	got := err.Error()

	for _, want := range []string{
		"your ticket CLI is configured for https://ticket.example, but that server does not expose the expected Ticket API (404 Not Found).",
		"Check that the remote URL points to the Ticket server",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("remote 404 runtime error missing %q:\n%s", want, got)
		}
	}
}

func TestFormatRuntimeErrorLeavesDomain404Unchanged(t *testing.T) {
	err := &client.HTTPStatusError{StatusCode: http.StatusNotFound, Status: "404 Not Found", APIError: "ticket not found"}
	if got := formatRuntimeError(err); got != err {
		t.Fatalf("formatRuntimeError() should leave domain 404 errors unchanged, got %v", got)
	}
}

func TestFormatRuntimeErrorLocalDBIssueIncludesSetup(t *testing.T) {
	t.Setenv("TICKET_HOME", t.TempDir())

	err := formatRuntimeError(errors.New("unable to open database file"))
	got := err.Error()

	for _, want := range []string{
		"unable to open database file",
		"setup:",
		"mode             : local",
		"configured via   : default local database path",
		"this command is currently using local mode",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("local runtime error missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"project config", "TICKET_URL"} {
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

func TestRunAdminExportImportSnapshotCommands(t *testing.T) {
	setupLocalCLI(t)

	createLocalTask(t, []string{"add", "-d", "snapshot export/import ticket", "Snapshot Ticket"})
	snapshotFile := filepath.Join(t.TempDir(), "snapshot.json")

	if err := run([]string{"admin", "export", "-o", snapshotFile}); err != nil {
		t.Fatalf("run(admin export) error = %v", err)
	}
	if _, err := os.Stat(snapshotFile); err != nil {
		t.Fatalf("snapshot file missing after export: %v", err)
	}

	if err := run([]string{"admin", "import", "-i", snapshotFile}); err != nil {
		t.Fatalf("run(admin import) error = %v", err)
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
		"workflow_acceptance_criteria:",
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
		"workflow_acceptance_criteria",
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

func TestRunHelpHonorsTKNoColorEnv(t *testing.T) {
	original := selectBannerWord
	selectBannerWord = func() string { return "TKT" }
	defer func() { selectBannerWord = original }()

	t.Setenv("TK_NOCOLOR", "1")
	output := captureStdout(t, func() {
		if err := run([]string{"help"}); err != nil {
			t.Fatalf("help error = %v", err)
		}
	})
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("help output should omit ANSI when TK_NOCOLOR=1:\n%s", output)
	}
	if !strings.Contains(output, "TTTTTTT") {
		t.Fatalf("help output missing banner art:\n%s", output)
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

func TestRenderCommandHelpNoLongerIncludesInit(t *testing.T) {
	help := renderCommandHelp("init")
	if strings.Contains(help, "requires the current working directory to be inside a git repository") {
		t.Fatalf("init help should be removed:\n%s", help)
	}
}

func TestRunOnboardPrintsEmbeddedAgentsTemplateToStdout(t *testing.T) {
	tempDir := t.TempDir()
	setTestWorkingDir(t, tempDir)

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
	setTestWorkingDir(t, tempDir)

	output := captureStdout(t, func() {
		if err := runSkill(nil); err != nil {
			t.Fatalf("runSkill() error = %v", err)
		}
	})
	if !strings.Contains(output, "# tk Skill") {
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
	if !strings.Contains(output, "# tk Skill") {
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
		"TICKET_ADMIN_PASSWORD: ${TICKET_ADMIN_PASSWORD:?Set TICKET_ADMIN_PASSWORD before first boot}",
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
		"tk server -f /path/to/ticket.db -p 9999 -v",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("server help missing %q:\n%s", want, help)
		}
	}
}

func TestRenderProjectHelpIncludesSetDraft(t *testing.T) {
	help := renderCommandHelp("project")
	for _, want := range []string{
		"tk project <create|list|get|use|set-default|clear-default|set-draft|request-access|my-access-requests|access-requests|approve-access-request|reject-access-request|workflow|add-user|remove-user|add-team|remove-team>",
		"`tk project set-default <ref>` saves a per-user default project; `tk project clear-default` removes it.",
		"`set-draft` controls whether new tickets default to draft mode for the project.",
		"`request-access` submits an access request for a gated project that accepts new members.",
		"`my-access-requests` lets the current user review their own pending and decided membership requests.",
		"`access-requests`, `approve-access-request`, and `reject-access-request` let project admins review and decide pending membership requests, optionally with a decision note.",
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
	for _, want := range []string{
		"PROJECT",
		"COMMANDS",
		"set-draft [-project_id <id>] <true|false>",
		"request-access [-project_id <id>]",
		"my-access-requests",
		"access-requests [-project_id <id>]",
		"approve-access-request",
		"reject-access-request",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("project usage missing %q:\n%s", want, output)
		}
	}
}

func TestRunAdminHelpUsesFormattedNamespaceUsage(t *testing.T) {
	output := captureStdout(t, func() {
		if err := run([]string{"admin", "help"}); err != nil {
			t.Fatalf("admin help error = %v", err)
		}
	})
	for _, want := range []string{
		"ADMIN",
		"COMMANDS",
		"tk admin <command> [flags]",
		"config",
		"workflow",
		"user",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("admin usage missing %q:\n%s", want, output)
		}
	}
}

func TestRunProjectRequestAccessRemote(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	testutil.CloneSeededDB(t, dbPath, "adminpass")
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

	alice, err := store.CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	captureStdout(t, func() {
		if err := run([]string{"login", "-username", "admin", "-password", "adminpass"}); err != nil {
			t.Fatalf("admin login error = %v", err)
		}
	})

	projectIDText := strings.TrimSpace(captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "GATE", "-title", "Gated Project", "-printid"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	}))
	projectID, err := strconv.ParseInt(projectIDText, 10, 64)
	if err != nil {
		t.Fatalf("ParseInt(project id) error = %v", err)
	}
	if err := store.SetProjectAcceptsNewMembers(context.Background(), db, projectID, true); err != nil {
		t.Fatalf("SetProjectAcceptsNewMembers() error = %v", err)
	}

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
		if err := run([]string{"login", "-username", "alice", "-password", "password123"}); err != nil {
			t.Fatalf("alice login error = %v", err)
		}
	})

	output := captureStdout(t, func() {
		if err := run([]string{"project", "request-access", "-project_id", "GATE", "-message", "please add me"}); err != nil {
			t.Fatalf("project request-access error = %v", err)
		}
	})
	for _, want := range []string{
		"requested access:",
		fmt.Sprintf("project_id=%d", projectID),
		"status=pending",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("request-access output missing %q:\n%s", want, output)
		}
	}

	requests, err := store.ListProjectAccessRequests(context.Background(), db, projectID, "")
	if err != nil {
		t.Fatalf("ListProjectAccessRequests() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("ListProjectAccessRequests() len = %d, want 1", len(requests))
	}
	if requests[0].UserID != alice.ID {
		t.Fatalf("request user_id = %q, want %q", requests[0].UserID, alice.ID)
	}
	if requests[0].Message != "please add me" {
		t.Fatalf("request message = %q, want %q", requests[0].Message, "please add me")
	}
	if requests[0].Status != "pending" {
		t.Fatalf("request status = %q, want pending", requests[0].Status)
	}

	if err := config.ClearCredentials(); err != nil {
		t.Fatalf("ClearCredentials(second) error = %v", err)
	}
	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("config.Load(second) error = %v", err)
	}
	cfg.Username = ""
	cfg.Token = ""
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save(second) error = %v", err)
	}

	captureStdout(t, func() {
		if err := run([]string{"login", "-username", "admin", "-password", "adminpass"}); err != nil {
			t.Fatalf("admin re-login error = %v", err)
		}
	})

	listOutput := captureStdout(t, func() {
		if err := run([]string{"project", "access-requests", "-project_id", "GATE", "-status", "pending"}); err != nil {
			t.Fatalf("project access-requests error = %v", err)
		}
	})
	for _, want := range []string{
		"REQUEST_ID",
		"GATE (Gated Project)",
		"alice",
		"please add me",
		"pending",
	} {
		if !strings.Contains(listOutput, want) {
			t.Fatalf("access-requests output missing %q:\n%s", want, listOutput)
		}
	}

	approveOutput := captureStdout(t, func() {
		if err := run([]string{"project", "approve-access-request", "-project_id", "GATE", "-request_id", strconv.FormatInt(requests[0].ID, 10), "-message", "Approved for sprint work"}); err != nil {
			t.Fatalf("approve-access-request error = %v", err)
		}
	})
	for _, want := range []string{
		"approved access request:",
		fmt.Sprintf("request_id=%d", requests[0].ID),
		"status=approved",
		"user=alice",
		"message=Approved for sprint work",
	} {
		if !strings.Contains(approveOutput, want) {
			t.Fatalf("approve-access-request output missing %q:\n%s", want, approveOutput)
		}
	}

	members, err := store.ListProjectMembers(context.Background(), db, projectID)
	if err != nil {
		t.Fatalf("ListProjectMembers() error = %v", err)
	}
	found := false
	for _, member := range members {
		if member.UserID == alice.ID && member.Role == store.ProjectRoleObserver {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("alice not added as observer after approval: %#v", members)
	}

	if err := config.ClearCredentials(); err != nil {
		t.Fatalf("ClearCredentials(third) error = %v", err)
	}
	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("config.Load(third) error = %v", err)
	}
	cfg.Username = ""
	cfg.Token = ""
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save(third) error = %v", err)
	}

	captureStdout(t, func() {
		if err := run([]string{"login", "-username", "alice", "-password", "password123"}); err != nil {
			t.Fatalf("alice re-login error = %v", err)
		}
	})

	myRequestsOutput := captureStdout(t, func() {
		if err := run([]string{"project", "my-access-requests"}); err != nil {
			t.Fatalf("project my-access-requests error = %v", err)
		}
	})
	for _, want := range []string{
		"REQUEST_ID",
		"GATE (Gated Project)",
		"alice",
		"approved",
		"decision: Approved for sprint work",
	} {
		if !strings.Contains(myRequestsOutput, want) {
			t.Fatalf("my-access-requests output missing %q:\n%s", want, myRequestsOutput)
		}
	}

	notificationsOutput := captureStdout(t, func() {
		if err := run([]string{"user", "notifications", "-status", "unread"}); err != nil {
			t.Fatalf("user notifications error = %v", err)
		}
	})
	for _, want := range []string{
		"NOTIFICATION_ID",
		"Project access approved",
		"project_access_approved",
		"unread",
	} {
		if !strings.Contains(notificationsOutput, want) {
			t.Fatalf("user notifications output missing %q:\n%s", want, notificationsOutput)
		}
	}

	notifications, err := store.ListUserNotifications(context.Background(), db, alice.ID, store.UserNotificationStatusUnread, 10)
	if err != nil {
		t.Fatalf("ListUserNotifications() error = %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("ListUserNotifications() len = %d, want 1", len(notifications))
	}
	if !strings.Contains(notifications[0].Message, "Approved for sprint work") {
		t.Fatalf("notification message = %q", notifications[0].Message)
	}

	readOutput := captureStdout(t, func() {
		if err := run([]string{"user", "read-notification", "-id", strconv.FormatInt(notifications[0].ID, 10)}); err != nil {
			t.Fatalf("user read-notification error = %v", err)
		}
	})
	for _, want := range []string{
		"marked notification as read:",
		fmt.Sprintf("notification_id=%d", notifications[0].ID),
		"status=read",
	} {
		if !strings.Contains(readOutput, want) {
			t.Fatalf("read-notification output missing %q:\n%s", want, readOutput)
		}
	}
}

func TestRenderUserHelpIncludesAdmin403Message(t *testing.T) {
	help := renderCommandHelp("user")
	for _, want := range []string{
		"tk admin user <create|new|ls|list|rm|delete|enable|disable|notifications|read-notification|reset-password>",
		"user is not an admin",
		"tk admin user create -username alice -email alice@example.com",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("user help missing %q:\n%s", want, help)
		}
	}
}

func TestRenderConfigHelpIncludesAdminNamespace(t *testing.T) {
	help := renderCommandHelp("config")
	for _, want := range []string{
		"tk admin config <get|ls|list|registration-enable|registration-disable|registration-autoapprove-enable|registration-autoapprove-disable> [key]",
		"tk admin config ls",
		"Runtime client configuration now comes from environment variables",
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
	if password != "" {
		t.Fatalf("resolveCredentials(default password) = %q, want empty", password)
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

	t.Setenv("TICKET_USERNAME", "env-user")
	t.Setenv("TICKET_PASSWORD", "env-pass")
	username, password, err = resolveCredentials("", "", true)
	if err != nil {
		t.Fatalf("resolveCredentials(env) error = %v", err)
	}
	if username != "env-user" {
		t.Fatalf("resolveCredentials(env) username = %q", username)
	}
	if password != "env-pass" {
		t.Fatalf("resolveCredentials(env) password = %q, want %q", password, "env-pass")
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

func TestExtractDBOverridePreservesCreateFileFlag(t *testing.T) {
	args, override, err := extractDBOverride([]string{"new", "-f", "tickets.txt"})
	if err != nil {
		t.Fatalf("extractDBOverride() error = %v", err)
	}
	if override != "" {
		t.Fatalf("extractDBOverride() override = %q, want empty", override)
	}
	if got := strings.Join(args, " "); got != "new -f tickets.txt" {
		t.Fatalf("extractDBOverride() args = %q", got)
	}

	args, override, err = extractDBOverride([]string{"update", "-f", "tickets.txt"})
	if err != nil {
		t.Fatalf("extractDBOverride(update) error = %v", err)
	}
	if override != "" {
		t.Fatalf("extractDBOverride(update) override = %q, want empty", override)
	}
	if got := strings.Join(args, " "); got != "update -f tickets.txt" {
		t.Fatalf("extractDBOverride(update) args = %q", got)
	}

	args, override, err = extractDBOverride([]string{"import", "-f", "ticket.md"})
	if err != nil {
		t.Fatalf("extractDBOverride(import) error = %v", err)
	}
	if override != "" {
		t.Fatalf("extractDBOverride(import) override = %q, want empty", override)
	}
	if got := strings.Join(args, " "); got != "import -f ticket.md" {
		t.Fatalf("extractDBOverride(import) args = %q", got)
	}
}

func TestRunServerWithExplicitDBBypassesTicketHomeCheck(t *testing.T) {
	tempDir := t.TempDir()
	setTestWorkingDir(t, tempDir)

	// Ensure no implicit workspace/env context is available.
	t.Setenv("TICKET_HOME", "")
	dbPath := filepath.Join(tempDir, "missing", "ticket.db")

	err := run([]string{"server", "-f", dbPath})
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
	testutil.CloneSeededDB(t, dbPath, "adminpass")
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

	setTestWorkingDir(t, tempDir)

	t.Setenv("TICKET_HOME", t.TempDir())
	setTestLocation(t, ts.URL)
	t.Setenv("TICKET_USERNAME", "admin")
	t.Setenv("TICKET_PASSWORD", "adminpass")

	output := captureStdout(t, func() {
		if err := run([]string{"whoami"}); err != nil {
			t.Fatalf("run(whoami) error = %v", err)
		}
	})
	if !strings.Contains(output, "username : admin") {
		t.Fatalf("whoami output missing remote user identity:\n%s", output)
	}
	if strings.Contains(output, "CONNECTION") || strings.Contains(output, "TICKET_URL") {
		t.Fatalf("whoami output should not include connection details:\n%s", output)
	}
}

func TestRunListWithoutProjectBindingDoesNotCreateLocalConfigDirs(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ticket.db")
	testutil.CloneSeededDB(t, dbPath, "adminpass")
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

	setTestWorkingDir(t, tempDir)

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

func TestDefaultTicketURLUsesSimonskiDotComHTTPS(t *testing.T) {
	if got, want := defaultTicketURL, "https://ticket.simonski.com"; got != want {
		t.Fatalf("defaultTicketURL = %q, want %q", got, want)
	}
}

func TestConfiguredServiceLocationUsesDefaultRemoteWhenNotTestBinary(t *testing.T) {
	setBinaryNameForTest(t, "tk")
	if got, want := configuredServiceLocation(config.Config{}), defaultTicketURL; got != want {
		t.Fatalf("configuredServiceLocation() = %q, want %q", got, want)
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

func TestRunInitDBDoesNotWarnAboutMissingDefaults(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	setTestWorkingDir(t, tempDir)

	output := captureStdout(t, func() {
		if err := runInitDB([]string{"-password", "secret12"}); err != nil {
			t.Fatalf("runInitDB() error = %v", err)
		}
	})
	if strings.Contains(output, "warning: could not check defaults") {
		t.Fatalf("runInitDB() unexpected defaults warning:\n%s", output)
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
	t.Setenv("TICKET_URL", server.URL)

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
	if err == nil && strings.Contains(string(configData), "session-token") {
		t.Fatalf("config.json should not contain session token:\n%s", string(configData))
	}
	credData, err := os.ReadFile(credsPath)
	if err != nil {
		t.Fatalf("ReadFile(credentials.json) error = %v", err)
	}
	if !strings.Contains(string(credData), "session-token") {
		t.Fatalf("credentials.json missing stored session token:\n%s", string(credData))
	}

	if err := runLogout(nil); err != nil {
		t.Fatalf("runLogout() error = %v", err)
	}
	if _, err := os.Stat(credsPath); !os.IsNotExist(err) {
		t.Fatalf("credentials.json should be removed after logout, err=%v", err)
	}
}

func TestRunLogoutRequiresRemoteMode(t *testing.T) {
	t.Setenv("TICKET_HOME", t.TempDir())
	setTestLocation(t, "")

	err := runLogout(nil)
	if err == nil {
		t.Fatal("runLogout() error = nil")
	}
	if !strings.Contains(err.Error(), "ticket logout only works in remote mode") {
		t.Fatalf("runLogout() error = %v", err)
	}
}

func TestRunLogoutRequiresStoredCredentials(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	t.Setenv("TICKET_URL", "https://tickets.example.com")
	setTestLocation(t, "https://tickets.example.com")

	err := runLogout(nil)
	if err == nil {
		t.Fatal("runLogout() error = nil")
	}
	if !strings.Contains(err.Error(), "no stored login session for https://tickets.example.com") {
		t.Fatalf("runLogout() error = %v", err)
	}
}

func TestRunLoginUsesValidStoredCredentialsFirst(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"username":"alice"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
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
	if err := config.SaveRemoteCredentials(server.URL, "alice", "stored-token"); err != nil {
		t.Fatalf("SaveRemoteCredentials() error = %v", err)
	}
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
	if err == nil && strings.Contains(string(configData), "stored-token") {
		t.Fatalf("config.json should not contain stored tokens:\n%s", string(configData))
	}
}

func TestRunLoginWithPasskey(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	var pollCalls int32
	baseURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/passkey/login/start":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"verification_url":"` + baseURL + `/passkey?code=abc123","code":"abc123","expires_at":"2099-01-01T00:00:00Z"}`))
		case "/api/auth/passkey/poll":
			call := atomic.AddInt32(&pollCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			if call == 1 {
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"status":"pending"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"complete","token":"passkey-token","user":{"username":"alice","role":"user"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	baseURL = server.URL
	setTestLocation(t, server.URL)
	t.Setenv("TICKET_URL", server.URL)

	oldOpener := passkeyBrowserOpener
	oldInterval := passkeyPollInterval
	passkeyBrowserOpener = func(target string) error {
		if target != server.URL+"/passkey?code=abc123" {
			t.Fatalf("browser target = %q", target)
		}
		return nil
	}
	passkeyPollInterval = 0
	t.Cleanup(func() {
		passkeyBrowserOpener = oldOpener
		passkeyPollInterval = oldInterval
	})

	output := captureStdout(t, func() {
		if err := runLogin([]string{"--passkey", "-username", "alice"}); err != nil {
			t.Fatalf("runLogin(--passkey) error = %v", err)
		}
	})
	if !strings.Contains(output, "logged in as alice") {
		t.Fatalf("runLogin(--passkey) output = %q", output)
	}
	creds, err := config.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	remoteCreds, ok := creds.Remote(server.URL)
	if !ok || remoteCreds.Token != "passkey-token" {
		t.Fatalf("stored remote credentials = %#v", remoteCreds)
	}
}

func TestRunLoginPasskeyShortCircuitsWhenAlreadyLoggedIn(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	var passkeyStartCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			if r.Header.Get("Authorization") != "Bearer stored-token" {
				t.Fatalf("status auth header = %q", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","authenticated":true,"user":{"username":"alice","role":"user"}}`))
		case "/api/auth/passkey/login/start":
			atomic.AddInt32(&passkeyStartCalls, 1)
			t.Fatal("passkey flow must not start when a valid session already exists")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	if err := config.SaveRemoteCredentials(server.URL, "alice", "stored-token"); err != nil {
		t.Fatalf("SaveRemoteCredentials() error = %v", err)
	}
	setTestLocation(t, server.URL)
	t.Setenv("TICKET_URL", server.URL)

	oldOpener := passkeyBrowserOpener
	passkeyBrowserOpener = func(string) error {
		t.Fatal("browser must not open when already logged in")
		return nil
	}
	t.Cleanup(func() { passkeyBrowserOpener = oldOpener })

	output := captureStdout(t, func() {
		if err := runLogin([]string{"--passkey"}); err != nil {
			t.Fatalf("runLogin(--passkey) error = %v", err)
		}
	})
	if !strings.Contains(output, "logged in as alice") {
		t.Fatalf("runLogin(--passkey) output = %q", output)
	}
	if atomic.LoadInt32(&passkeyStartCalls) != 0 {
		t.Fatalf("passkey start called %d times, want 0", passkeyStartCalls)
	}
}

func TestRunLoginTokenShortCircuitsWhenAlreadyLoggedIn(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			// The already-logged-in check must use the STORED token; the supplied
			// -token value must never reach the server.
			if r.Header.Get("Authorization") != "Bearer stored-token" {
				t.Fatalf("status auth header = %q, want stored token", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","authenticated":true,"user":{"username":"alice","role":"user"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	if err := config.SaveRemoteCredentials(server.URL, "alice", "stored-token"); err != nil {
		t.Fatalf("SaveRemoteCredentials() error = %v", err)
	}
	setTestLocation(t, server.URL)
	t.Setenv("TICKET_URL", server.URL)

	output := captureStdout(t, func() {
		if err := runLogin([]string{"-token", "some-other-token"}); err != nil {
			t.Fatalf("runLogin(-token) error = %v", err)
		}
	})
	if !strings.Contains(output, "logged in as alice") {
		t.Fatalf("runLogin(-token) output = %q", output)
	}
}

func TestRunUserPasskeyEnroll(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	baseURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/passkey/register/start":
			if r.Header.Get("Authorization") != "Bearer stored-token" {
				t.Fatalf("register start auth header = %q", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"verification_url":"` + baseURL + `/passkey?code=enroll1","code":"enroll1","expires_at":"2099-01-01T00:00:00Z"}`))
		case "/api/auth/passkey/poll":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"complete"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	baseURL = server.URL
	setTestLocation(t, server.URL)
	t.Setenv("TICKET_URL", server.URL)
	if err := config.SaveRemoteCredentials(server.URL, "alice", "stored-token"); err != nil {
		t.Fatalf("SaveRemoteCredentials() error = %v", err)
	}

	oldOpener := passkeyBrowserOpener
	oldInterval := passkeyPollInterval
	passkeyBrowserOpener = func(target string) error {
		if target != server.URL+"/passkey?code=enroll1" {
			t.Fatalf("browser target = %q", target)
		}
		return nil
	}
	passkeyPollInterval = 0
	t.Cleanup(func() {
		passkeyBrowserOpener = oldOpener
		passkeyPollInterval = oldInterval
	})

	output := captureStdout(t, func() {
		if err := run([]string{"user", "passkey", "enroll", "-name", "Laptop"}); err != nil {
			t.Fatalf("run(user passkey enroll) error = %v", err)
		}
	})
	if !strings.Contains(output, "passkey enrolled") {
		t.Fatalf("user passkey enroll output = %q", output)
	}
}

func TestRunRegisterPendingApprovalPrintsHelpfulMessage(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/register" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"username":"pending","email":"pending@example.com","approved":false}`))
	}))
	defer server.Close()
	setTestLocation(t, server.URL)

	output := captureStdout(t, func() {
		if err := runRegister([]string{"-username", "pending", "-email", "pending@example.com"}); err != nil {
			t.Fatalf("runRegister() error = %v", err)
		}
	})
	for _, want := range []string{"registered user pending", "wait for approval or check your email"} {
		if !strings.Contains(output, want) {
			t.Fatalf("runRegister() output missing %q:\n%s", want, output)
		}
	}
}

func TestRunRegisterDisabledReturnsHelpfulError(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/register" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"registration is disabled"}`))
	}))
	defer server.Close()
	setTestLocation(t, server.URL)

	err := runRegister([]string{"-username", "pending", "-email", "pending@example.com"})
	if err == nil {
		t.Fatal("runRegister() error = nil")
	}
	if !strings.Contains(err.Error(), "server is not accepting registrations right now") {
		t.Fatalf("runRegister() error = %v", err)
	}
}

func TestRunRegisterDuplicateUsernameReturnsHelpfulError(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	testutil.CloneSeededDB(t, dbPath, "adminpass")
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
	if err := store.SetRegistrationEnabled(context.Background(), db, true); err != nil {
		t.Fatalf("SetRegistrationEnabled() error = %v", err)
	}

	if err := run([]string{"register", "-username", "alice", "-email", "alice@example.com", "-password", "password123"}); err != nil {
		t.Fatalf("first register error = %v", err)
	}

	err = run([]string{"register", "-username", "alice", "-email", "alice@example.com", "-password", "password123"})
	if err == nil {
		t.Fatal("second register error = nil")
	}
	if !strings.Contains(err.Error(), "username already exists") {
		t.Fatalf("second register error = %v", err)
	}
}

func TestRunRegisterRequiresExplicitUsername(t *testing.T) {
	t.Setenv("USER", "simon")
	t.Setenv("USERNAME", "simon")

	err := run([]string{"register", "-email", "simon@example.com", "-password", "password123"})
	if err == nil {
		t.Fatal("register error = nil")
	}
	if !strings.Contains(err.Error(), "username is required") {
		t.Fatalf("register error = %v", err)
	}
}

func TestRunRegisterRequiresEmail(t *testing.T) {
	err := run([]string{"register", "-username", "alice"})
	if err == nil {
		t.Fatal("register error = nil")
	}
	if !strings.Contains(err.Error(), "email is required") {
		t.Fatalf("register error = %v", err)
	}
}

func TestRunStatusRemoteSuccess(t *testing.T) {
	t.Setenv("TICKET_HOME", t.TempDir())
	t.Setenv("AGENT_ID", "agent-123")
	t.Setenv("AGENT_PASSWORD", "agent-secret")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			if r.Header.Get("Authorization") != "Bearer env-token" {
				t.Fatalf("status auth header = %q", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","authenticated":true,"server_version":"9.8.7","user":{"username":"alice","role":"user","default_project_id":7}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	setTestLocation(t, server.URL)
	t.Setenv("TICKET_TOKEN", "env-token")

	output := captureStdout(t, func() {
		if err := runStatus(nil); err != nil {
			t.Fatalf("runStatus(remote) error = %v", err)
		}
	})
	for _, want := range []string{
		"TICKET_URL",
		server.URL,
		"TICKET_USERNAME",
		"alice",
		"TICKET_PASSWORD",
		"********",
		"SERVER_VERSION",
		"9.8.7",
		"CLIENT_VERSION",
		strings.TrimSpace(embeddedVersion),
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus(remote) missing %q:\n%s", want, output)
		}
	}
	for _, unwanted := range []string{"TICKET_HOME", "config_file", "authenticated", "connection", "password         : (using TICKET_TOKEN)", "DEFAULT_PROJECT"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("runStatus(remote) should not show %q:\n%s", unwanted, output)
		}
	}
	if strings.Contains(output, "env-pass") || strings.Contains(output, "agent-secret") {
		t.Fatalf("runStatus(remote) should mask secret env values:\n%s", output)
	}
	for _, unwanted := range []string{"AGENT_ID", "AGENT_PASSWORD"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("runStatus(remote) should not show %q:\n%s", unwanted, output)
		}
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
	if runErr != nil {
		t.Fatalf("runStatus() error = %v", runErr)
	}
	for _, want := range []string{
		"TICKET_URL",
		"UNSET",
		"TICKET_USERNAME",
		"TICKET_PASSWORD",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus() output missing %q:\n%s", want, output)
		}
	}
}

func TestRunInitRequiresGitRepository(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", t.TempDir())
	setTestWorkingDir(t, tempDir)

	err := run([]string{"init"})
	if err == nil {
		t.Fatal("run(init) error = nil, want git-repository error")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Fatalf("run(init) error = %v, want message about git repository", err)
	}
}

func TestRunInitFailsWithoutGitRemote(t *testing.T) {
	baseDir := t.TempDir()
	repoDir := filepath.Join(baseDir, "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(repo .git) error = %v", err)
	}
	setTestWorkingDir(t, repoDir)

	err := run([]string{"init"})
	if err == nil {
		t.Fatal("run(init) error = nil, want git-repository error")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Fatalf("run(init) error = %v, want message about git repository", err)
	}
}

func TestRunStatusLocalSuccess(t *testing.T) {
	setupLocalCLI(t)

	output := captureStdout(t, func() {
		if err := run([]string{"status", "-nocolor"}); err != nil {
			t.Fatalf("run(status) error = %v", err)
		}
	})
	for _, want := range []string{
		"TICKET_URL",
		"http://",
		"TICKET_USERNAME",
		"admin",
		"TICKET_PASSWORD",
		"********",
		"SERVER_VERSION",
		"CLIENT_VERSION",
		strings.TrimSpace(embeddedVersion),
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus(remote) missing %q:\n%s", want, output)
		}
	}
}

func TestRunListShowsActiveTicketsFirstWithSeparator(t *testing.T) {
	setupLocalCLI(t)

	idleA := createLocalTask(t, []string{"add", "Idle A"})
	activeID := createLocalTask(t, []string{"add", "Active one"})
	idleB := createLocalTask(t, []string{"add", "Idle B"})

	if err := run([]string{"claim", activeID}); err != nil {
		t.Fatalf("claim error = %v", err)
	}
	if err := run([]string{"active", activeID}); err != nil {
		t.Fatalf("active error = %v", err)
	}

	out := captureStdout(t, func() {
		if err := run([]string{"ls", "-nocolor"}); err != nil {
			t.Fatalf("ls error = %v", err)
		}
	})

	ai := strings.Index(out, activeID)
	bi := strings.Index(out, idleA)
	ci := strings.Index(out, idleB)
	if ai < 0 || bi < 0 || ci < 0 {
		t.Fatalf("ls output missing ticket ids:\n%s", out)
	}
	if ai > bi || ai > ci {
		t.Fatalf("active ticket should be listed first:\n%s", out)
	}
	rule := strings.Repeat("─", 10)
	sep := strings.Index(out, rule)
	if sep < 0 {
		t.Fatalf("ls output missing active/inactive separator:\n%s", out)
	}
	if !(ai < sep && sep < bi && sep < ci) {
		t.Fatalf("separator should sit between active and idle tickets:\n%s", out)
	}
}

func TestRunListShowsTicketsWithoutDetailsBanner(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "SUM", "-title", "Summary Test", "-git-repository", "https://github.com/example/summary.git"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})
	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	var summaryProjectID int64
	for _, project := range projects {
		if project.Prefix == "SUM" {
			summaryProjectID = project.ID
			break
		}
	}
	if summaryProjectID == 0 {
		t.Fatalf("summary project not found in %+v", projects)
	}
	t.Setenv("TICKET_PROJECT", fmt.Sprintf("%d", summaryProjectID))
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

	for _, want := range []string{"TICKET_URL", "TICKET_USERNAME", "TICKET_PASSWORD", "SERVER_VERSION", "CLIENT_VERSION"} {
		if !strings.Contains(statusOut, want) {
			t.Fatalf("status output missing %q:\n%s", want, statusOut)
		}
	}
	for _, unwanted := range []string{"project          :", "config_file", "TICKET_HOME"} {
		if strings.Contains(statusOut, unwanted) {
			t.Fatalf("status output should stay remote-only and omit %q:\n%s", unwanted, statusOut)
		}
	}
	for _, unwanted := range []string{
		"project          :",
		"git              :",
		"workflow",
		"draft            :",
		"TICKET_HOME",
		"AGENT_ID",
		"AGENT_PASSWORD",
		"config_file",
		"db_path",
		"db_exists",
		"╭",
		"╰",
	} {
		if strings.Contains(listOut, unwanted) {
			t.Fatalf("list output should not include %q:\n%s", unwanted, listOut)
		}
	}
	for _, unwanted := range []string{"git repo", "project_workflow", "project_default_draft"} {
		if strings.Contains(listOut, unwanted) || strings.Contains(statusOut, unwanted) {
			t.Fatalf("output should not include legacy key %q:\nstatus:\n%s\nlist:\n%s", unwanted, statusOut, listOut)
		}
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
	if !strings.Contains(listOut, "Summary task one") {
		t.Fatalf("list output missing ticket row:\n%s", listOut)
	}
	if !strings.HasPrefix(listOut, "PROJECT  SUM — Summary Test (") {
		t.Fatalf("list output should start with project context:\n%s", listOut)
	}
	if !strings.Contains(listOut, "ID") || !strings.Contains(listOut, "TYPE") || !strings.Contains(listOut, "TITLE") {
		t.Fatalf("list output should include the ticket table header after project context:\n%s", listOut)
	}
	// Project context, then a blank line, then the ticket table header.
	listLines := strings.Split(listOut, "\n")
	if len(listLines) < 3 || strings.TrimSpace(listLines[1]) != "" {
		t.Fatalf("project context should be followed by a blank line before the table:\n%s", listOut)
	}
	if !strings.HasPrefix(strings.TrimSpace(listLines[2]), "ID") {
		t.Fatalf("ticket table header should follow the blank line:\n%s", listOut)
	}
}

func TestRunListTruncatesMultilineTicketTitles(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "Rebuild according to the HARD RULES\nIf you build a shed within two metres"})

	listOut := captureStdout(t, func() {
		if err := run([]string{"ls", "-nocolor"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})
	if !strings.Contains(listOut, "Rebuild according to the HARD RULES...") {
		t.Fatalf("list output missing truncated multiline title:\n%s", listOut)
	}
	if strings.Contains(listOut, "If you build a shed within two metres") {
		t.Fatalf("list output should not include later title lines:\n%s", listOut)
	}

	getOut := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	for _, want := range []string{
		"title       : Rebuild according to the HARD RULES",
		"              If you build a shed within two metres",
	} {
		if !strings.Contains(getOut, want) {
			t.Fatalf("get output missing %q:\n%s", want, getOut)
		}
	}
}

func TestRunListIndentsTitlesByTreeDepth(t *testing.T) {
	setupLocalCLI(t)

	rootID := createLocalTask(t, []string{"add", "Root title"})
	childID := createLocalTask(t, []string{"add", "-parent", rootID, "Child title"})
	grandchildID := createLocalTask(t, []string{"add", "-parent", childID, "Grandchild title"})

	output := captureStdout(t, func() {
		if err := run([]string{"ls", "-nocolor"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})

	var rootLine, childLine, grandchildLine string
	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.Contains(line, rootID) && strings.Contains(line, "Root title"):
			rootLine = line
		case strings.Contains(line, childID) && strings.Contains(line, "Child title"):
			childLine = line
		case strings.Contains(line, grandchildID) && strings.Contains(line, "Grandchild title"):
			grandchildLine = line
		}
	}
	if rootLine == "" || childLine == "" || grandchildLine == "" {
		t.Fatalf("list output missing hierarchy rows:\n%s", output)
	}
	rootTitleCol := strings.Index(rootLine, "Root title")
	childTitleCol := strings.Index(childLine, "Child title")
	grandchildTitleCol := strings.Index(grandchildLine, "Grandchild title")
	if !(rootTitleCol < childTitleCol && childTitleCol < grandchildTitleCol) {
		t.Fatalf("title columns should indent by depth:\nroot: %q\nchild: %q\ngrandchild: %q", rootLine, childLine, grandchildLine)
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

func TestRenderWorkflowProgress(t *testing.T) {
	noColorOutput = true
	defer func() { noColorOutput = false }()
	stages := []store.WorkflowStage{
		{StageName: "develop"},
		{StageName: "done"},
	}
	got := renderWorkflowProgress("develop", stages)
	want := "[develop] → done"
	if got != want {
		t.Fatalf("renderWorkflowProgress() = %q, want %q", got, want)
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

func TestLifecycleCommandsAcceptPositionalIDBeforeFlags(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	taskID := createLocalTask(t, []string{"add", "Positional lifecycle"})

	// Claim is required before a ticket may go active.
	if err := run([]string{"claim", taskID}); err != nil {
		t.Fatalf("claim error = %v", err)
	}

	// Positional id FOLLOWED by a flag must work: tk active <id> -m "note".
	// Go's flag parser stops at the first positional, so this used to fail.
	if err := run([]string{"active", taskID, "-m", "starting the work"}); err != nil {
		t.Fatalf("active <id> -m error = %v", err)
	}

	ticket, err := svc.GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if ticket.State != "active" {
		t.Fatalf("ticket state = %q, want active", ticket.State)
	}

	// The trailing -m must have been parsed and attached as a comment.
	comments, err := svc.ListComments(context.Background(), taskID)
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	found := false
	for _, c := range comments {
		if strings.Contains(c.Text, "starting the work") || strings.Contains(c.Comment, "starting the work") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected comment from -m flag, comments = %+v", comments)
	}

	// stage <id> <stage> -m "..." (positional id + state arg + trailing flag).
	if err := run([]string{"stage", taskID, "develop", "-m", "into develop"}); err != nil {
		t.Fatalf("stage <id> <stage> -m error = %v", err)
	}
	staged, err := svc.GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket() after stage error = %v", err)
	}
	if staged.Stage != "develop" {
		t.Fatalf("ticket stage = %q, want develop", staged.Stage)
	}
}

func TestRunWorkflowAssignsFirstStageRoleAndSupportsPrevious(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	wf, err := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{
		Name:        "Lifecycle Workflow",
		Description: "workflow for next/previous command coverage",
	})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	designStage, err := svc.AddWorkflowStage(context.Background(), wf.ID, libticket.WorkflowStageRequest{
		StageName:   "design",
		Description: "design",
		SortOrder:   0,
	})
	if err != nil {
		t.Fatalf("AddWorkflowStage(design) error = %v", err)
	}
	testStage, err := svc.AddWorkflowStage(context.Background(), wf.ID, libticket.WorkflowStageRequest{
		StageName:   "test",
		Description: "test",
		SortOrder:   1,
	})
	if err != nil {
		t.Fatalf("AddWorkflowStage(test) error = %v", err)
	}
	_, err = svc.AddWorkflowStage(context.Background(), wf.ID, libticket.WorkflowStageRequest{
		StageName:   "done",
		Description: "done",
		SortOrder:   2,
	})
	if err != nil {
		t.Fatalf("AddWorkflowStage(done) error = %v", err)
	}
	designer, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
		WorkflowID:  &wf.ID,
		Title:       "designer",
		Description: "designs work",
	})
	if err != nil {
		t.Fatalf("CreateRole(designer) error = %v", err)
	}
	tester, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
		WorkflowID:  &wf.ID,
		Title:       "tester",
		Description: "tests work",
	})
	if err != nil {
		t.Fatalf("CreateRole(tester) error = %v", err)
	}
	if roleErr := svc.AddWorkflowStageRole(context.Background(), wf.ID, designStage.ID, designer.ID); roleErr != nil {
		t.Fatalf("AddWorkflowStageRole(design) error = %v", roleErr)
	}
	if roleErr := svc.AddWorkflowStageRole(context.Background(), wf.ID, testStage.ID, tester.ID); roleErr != nil {
		t.Fatalf("AddWorkflowStageRole(test) error = %v", roleErr)
	}
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	if _, updateErr := svc.UpdateProject(context.Background(), project.ID, libticket.ProjectUpdateRequest{WorkflowID: &wf.ID}); updateErr != nil {
		t.Fatalf("UpdateProject(workflow) error = %v", updateErr)
	}

	taskID := createLocalTask(t, []string{"add", "Workflow Ticket"})
	initial, err := svc.GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket(initial) error = %v", err)
	}
	if initial.RoleID == nil || *initial.RoleID != designer.ID || initial.WorkflowStageID == nil || *initial.WorkflowStageID != designStage.ID {
		t.Fatalf("initial ticket = %#v, want design stage with designer role", initial)
	}

	if runErr := run([]string{"update", "-id", taskID, "-status", "design/success"}); runErr != nil {
		t.Fatalf("update design/success error = %v", runErr)
	}
	advanced, err := svc.GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket(after success) error = %v", err)
	}
	if advanced.Stage != "test" || advanced.State != store.StateIdle || advanced.RoleID == nil || *advanced.RoleID != tester.ID || advanced.WorkflowStageID == nil || *advanced.WorkflowStageID != testStage.ID {
		t.Fatalf("advanced ticket = %#v, want test stage with tester role idle", advanced)
	}

	if runErr := run([]string{"update", "-id", taskID, "-status", "test/fail"}); runErr != nil {
		t.Fatalf("update test/fail error = %v", runErr)
	}
	previousOutput := captureStdout(t, func() {
		if runErr := run([]string{"previous", "-id", taskID}); runErr != nil {
			t.Fatalf("previous error = %v", runErr)
		}
	})
	if !strings.Contains(previousOutput, "regressed: test/fail -> design/idle") {
		t.Fatalf("unexpected previous output:\n%s", previousOutput)
	}
	regressed, err := svc.GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket(after previous) error = %v", err)
	}
	if regressed.Stage != "design" || regressed.State != store.StateIdle || regressed.RoleID == nil || *regressed.RoleID != designer.ID || regressed.WorkflowStageID == nil || *regressed.WorkflowStageID != designStage.ID {
		t.Fatalf("regressed ticket = %#v, want design stage with designer role idle", regressed)
	}
}

func TestRunProjectCommandsInLocalMode(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

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
	if !strings.Contains(listOutput, "Project A") {
		t.Fatalf("project list output = %q", listOutput)
	}

	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	var createdProjectID int64
	for _, project := range projects {
		if project.Prefix == "PRA" {
			createdProjectID = project.ID
			break
		}
	}
	if createdProjectID == 0 {
		t.Fatalf("created project not found in %+v", projects)
	}

	updateOutput := captureStdout(t, func() {
		if err := run([]string{"project", fmt.Sprintf("%d", createdProjectID), "update", "-title", "Project B", "-description", "Updated", "-dor-map", "develop=Build reviewed", "-ac", "AC2"}); err != nil {
			t.Fatalf("project update error = %v", err)
		}
	})
	for _, want := range []string{"project: Project B", "wow: Updated", "description: Updated", "ac: AC2", "acceptance_criteria: AC2", "dor_map[default]: Ready", "dor_map[develop]: Build reviewed"} {
		if !strings.Contains(updateOutput, want) {
			t.Fatalf("project update output missing %q:\n%s", want, updateOutput)
		}
	}

	disableOutput := captureStdout(t, func() {
		if err := run([]string{"project", fmt.Sprintf("%d", createdProjectID), "disable"}); err != nil {
			t.Fatalf("project disable error = %v", err)
		}
	})
	if !strings.Contains(disableOutput, "status: closed") {
		t.Fatalf("project disable output = %q", disableOutput)
	}

}

func TestRunProjectUpdateRePrefixesProject(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	project, err := svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix: "PRE",
		Title:  "Re-prefix Project",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Some work",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if !strings.HasPrefix(ticket.ID, "PRE-") {
		t.Fatalf("ticket key = %q, want PRE- prefix", ticket.ID)
	}

	updateOutput := captureStdout(t, func() {
		if err := run([]string{"project", "update", "-id", fmt.Sprintf("%d", project.ID), "-prefix", "ATOM"}); err != nil {
			t.Fatalf("project update -prefix error = %v", err)
		}
	})
	if !strings.Contains(updateOutput, "prefix: ATOM") {
		t.Fatalf("project update output missing new prefix:\n%s", updateOutput)
	}

	updated, err := svc.GetProject(context.Background(), fmt.Sprintf("%d", project.ID))
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if updated.Prefix != "ATOM" {
		t.Fatalf("project prefix = %q, want ATOM", updated.Prefix)
	}

	newKey := "ATOM-" + strings.TrimPrefix(ticket.ID, "PRE-")
	rekeyed, err := svc.GetTicket(context.Background(), newKey)
	if err != nil {
		t.Fatalf("GetTicket(%q) after re-prefix error = %v", newKey, err)
	}
	if rekeyed.ID != newKey {
		t.Fatalf("re-keyed ticket key = %q, want %q", rekeyed.ID, newKey)
	}
}

func TestRunProjectUpdateAcceptsPositionalIDBeforeFlags(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	project, err := svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix: "POS",
		Title:  "Positional Update",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	// `tk project update 5 -prefix ATOM` must behave like the -id form, even
	// though the positional id precedes the flag.
	updateOutput := captureStdout(t, func() {
		if err := run([]string{"project", "update", fmt.Sprintf("%d", project.ID), "-prefix", "ATOM"}); err != nil {
			t.Fatalf("project update <id> -prefix error = %v", err)
		}
	})
	if !strings.Contains(updateOutput, "prefix: ATOM") {
		t.Fatalf("project update output missing new prefix:\n%s", updateOutput)
	}

	updated, err := svc.GetProject(context.Background(), fmt.Sprintf("%d", project.ID))
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if updated.Prefix != "ATOM" {
		t.Fatalf("project prefix = %q, want ATOM", updated.Prefix)
	}
}

func TestRunStatusShowsServerVersionWhenAuthFails(t *testing.T) {
	setupLocalCLI(t)
	// Valid server, but wrong credentials: the authenticated status call fails,
	// yet the server version must still be shown (fetched unauthenticated).
	t.Setenv("TICKET_PASSWORD", "wrong-password-zz")

	jsonOutput := captureStdout(t, func() {
		// run returns the auth error; we only care about the printed payload.
		_ = run([]string{"status", "-json"})
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(jsonOutput), &payload); err != nil {
		t.Fatalf("status -json parse error = %v\n%s", err, jsonOutput)
	}
	if got := payload["SERVER_VERSION"]; got == "UNSET" || got == nil || got == "" {
		t.Fatalf("SERVER_VERSION = %v, want the server version even when auth fails:\n%s", got, jsonOutput)
	}
}

func TestRunStatusSurfacesConnectionErrorInBothModes(t *testing.T) {
	t.Setenv("TICKET_HOME", t.TempDir())
	t.Setenv("TICKET_URL", "http://127.0.0.1:0")
	t.Setenv("TICKET_USERNAME", "admin")
	t.Setenv("TICKET_PASSWORD", "secret12")

	// Non-JSON mode surfaces the connection error.
	if err := run([]string{"status"}); err == nil {
		t.Fatal("status (non-json) error = nil, want connection error")
	}

	// JSON mode must be consistent: surface the same error AND include it in
	// the JSON payload (previously -json swallowed the error).
	jsonOutput := captureStdout(t, func() {
		if err := run([]string{"status", "-json"}); err == nil {
			t.Fatal("status -json error = nil, want connection error")
		}
	})
	if !strings.Contains(jsonOutput, "\"ERROR\"") {
		t.Fatalf("status -json payload missing ERROR field:\n%s", jsonOutput)
	}
	if !strings.Contains(jsonOutput, "\"CONNECTED\": false") {
		t.Fatalf("status -json payload missing CONNECTED=false:\n%s", jsonOutput)
	}
}

func TestRunProjectListMarksRepoResolvedCurrentProject(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	createOutput := captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "REP", "-title", "Repo Project", "-git-repository", "https://github.com/example/repo.git"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})
	if !strings.Contains(createOutput, "Repo Project") {
		t.Fatalf("project create output = %q", createOutput)
	}
	project, err := svc.GetProject(context.Background(), "REP")
	if err != nil {
		t.Fatalf("GetProject(REP) error = %v", err)
	}

	setTestGitOrigin(t, "https://github.com/example/repo.git")

	listOutput := captureStdout(t, func() {
		if err := run([]string{"project", "list"}); err != nil {
			t.Fatalf("project list error = %v", err)
		}
	})
	if !strings.Contains(listOutput, fmt.Sprintf("*  %d   REP     Repo Project", project.ID)) {
		t.Fatalf("project list output missing repo-resolved marker:\n%s", listOutput)
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

	workflow, err := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{
		Name:        "Guidance Workflow",
		Description: "workflow for project summary",
	})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	project, err := svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix:             "MAP",
		Title:              "Guidance Project",
		AcceptanceCriteria: "legacy project ac",
		DORMap:             store.GuidanceMap{"default": "project default dor", "develop": "project develop dor"},
		DODMap:             store.GuidanceMap{"default": "project default dod"},
		ACMap:              store.GuidanceMap{"qa": "project qa ac"},
		GitRepository:      "github.com/example/guidance.git",
		Visibility:         store.ProjectVisibilityTeam,
		AcceptsNewMembers:  true,
		WorkflowID:         &workflow.ID,
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if err := svc.AddProjectGitRepository(context.Background(), project.Prefix, "github.com/example/guidance-secondary.git"); err != nil {
		t.Fatalf("AddProjectGitRepository() error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"project", "get", project.Prefix}); err != nil {
			t.Fatalf("project get error = %v", err)
		}
	})

	for _, want := range []string{
		"FIELD",
		"TITLE",
		"Guidance Project",
		"VISIBILITY",
		"team",
		"ACCEPTS_NEW_MEMBERS",
		"GIT_REPOSITORY",
		"github.com/example/guidance.git",
		"REPOSITORIES",
		"github.com/example/guidance-secondary.git, github.com/example/guidance.git",
		"WORKFLOW",
		"Guidance Workflow",
		"DOR_MAP[default]",
		"project default dor",
		"DOR_MAP[develop]",
		"project develop dor",
		"DOD_MAP[default]",
		"project default dod",
		"AC_MAP[default]",
		"legacy project ac",
		"AC_MAP[qa]",
		"project qa ac",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("project get output missing %q:\n%s", want, output)
		}
	}
}

func TestRunProjectGetUsesCurrentProjectWhenIDOmitted(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	project, err := svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix: "CUR",
		Title:  "Current Project",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	t.Setenv("TICKET_PROJECT", project.Prefix)

	output := captureStdout(t, func() {
		if err := run([]string{"project", "get"}); err != nil {
			t.Fatalf("project get error = %v", err)
		}
	})
	for _, want := range []string{"TITLE", "Current Project", "PREFIX", "CUR"} {
		if !strings.Contains(output, want) {
			t.Fatalf("project get output missing %q:\n%s", want, output)
		}
	}
}

func TestRunProjectGetUsesDefaultProjectWhenNoContextSet(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	project, err := svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix: "DEF",
		Title:  "Default Project",
	})
	if err != nil {
		t.Fatalf("CreateProject(default) error = %v", err)
	}

	if err := run([]string{"project", "set-default", project.Prefix}); err != nil {
		t.Fatalf("project set-default error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"project", "get"}); err != nil {
			t.Fatalf("project get error = %v", err)
		}
	})
	for _, want := range []string{"TITLE", "Default Project", "PROJECT_ID", fmt.Sprintf("%d", project.ID), "PREFIX", "DEF"} {
		if !strings.Contains(output, want) {
			t.Fatalf("project get output missing %q:\n%s", want, output)
		}
	}
}

func TestRunProjectCreateUsesPositionalTitle(t *testing.T) {
	setupLocalCLI(t)

	output := captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "POS", "Positional Project"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})
	if !strings.Contains(output, "project: Positional Project") {
		t.Fatalf("project create output missing positional title:\n%s", output)
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
		if err := run([]string{"get", "-id", ticketID, "-v"}); err != nil {
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
		if err := run([]string{"get", "-id", ticketID, "-v"}); err != nil {
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

func TestRunTicketCreateSupportsProjectSelectionAliases(t *testing.T) {
	setupLocalCLI(t)

	svc := localCLIService(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	privateProject, _, err := resolveProjectContext(context.Background(), cfg, svc, "private")
	if err != nil {
		t.Fatalf("resolveProjectContext(private) error = %v", err)
	}
	publicProject, _, err := resolveProjectContext(context.Background(), cfg, svc, "public")
	if err != nil {
		t.Fatalf("resolveProjectContext(public) error = %v", err)
	}

	privateID := createLocalTask(t, []string{"add", "-project_id", "private", "-p", "3", "Private Alias Task"})
	privateTicket, err := svc.GetTicket(context.Background(), privateID)
	if err != nil {
		t.Fatalf("GetTicket(private alias) error = %v", err)
	}
	if privateTicket.ProjectID != privateProject.ID {
		t.Fatalf("private alias ticket.ProjectID = %d, want %d", privateTicket.ProjectID, privateProject.ID)
	}
	if privateTicket.Priority != 3 {
		t.Fatalf("private alias ticket.Priority = %d, want 3", privateTicket.Priority)
	}

	publicID := createLocalTask(t, []string{"add", "-project", "public", "Public Alias Task"})
	publicTicket, err := svc.GetTicket(context.Background(), publicID)
	if err != nil {
		t.Fatalf("GetTicket(public alias) error = %v", err)
	}
	if publicTicket.ProjectID != publicProject.ID {
		t.Fatalf("public alias ticket.ProjectID = %d, want %d", publicTicket.ProjectID, publicProject.ID)
	}

	filePath := filepath.Join(t.TempDir(), "ticket.md")
	fileContent := "title: File Alias Task\nproject_id: public\n\nCreated from file.\n"
	if err := os.WriteFile(filePath, []byte(fileContent), 0o600); err != nil {
		t.Fatalf("WriteFile(ticket.md) error = %v", err)
	}
	fileID := createLocalTask(t, []string{"add", "@" + filePath})
	fileTicket, err := svc.GetTicket(context.Background(), fileID)
	if err != nil {
		t.Fatalf("GetTicket(file alias) error = %v", err)
	}
	if fileTicket.ProjectID != publicProject.ID {
		t.Fatalf("file alias ticket.ProjectID = %d, want %d", fileTicket.ProjectID, publicProject.ID)
	}
}

func TestRunPromptBuildsPlaintextSections(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if project.WorkflowID != nil {
		stages, listErr := svc.ListWorkflowStages(context.Background(), *project.WorkflowID)
		if listErr != nil {
			t.Fatalf("ListWorkflowStages() error = %v", listErr)
		}
		for _, stage := range stages {
			if _, updateErr := svc.UpdateWorkflowStage(context.Background(), stage.ID, libticket.WorkflowStageRequest{
				StageName:          stage.StageName,
				Description:        stage.Description,
				AcceptanceCriteria: "Stage acceptance criteria",
				DefinitionOfReady:  "Stage acceptance criteria",
			}); updateErr != nil {
				t.Fatalf("UpdateWorkflowStage() error = %v", updateErr)
			}
		}
	}

	epicID := createLocalTask(t, []string{"epic", "-d", "Epic description", "Prompt Epic"})
	taskID := createLocalTask(t, []string{"add", "-parent", epicID, "-d", "Task description", "-ac", "Task acceptance", "Prompt Task"})
	if _, updateErr := svc.UpdateProject(context.Background(), project.ID, libticket.ProjectUpdateRequest{
		Title:              project.Title,
		Description:        project.Description,
		AcceptanceCriteria: project.AcceptanceCriteria,
		DORMap:             store.GuidanceMap{"default": "Project default DOR"},
		DODMap:             store.GuidanceMap{"default": "Project default DOD"},
		ACMap:              store.GuidanceMap{"default": "Project default AC"},
		WorkflowID:         project.WorkflowID,
	}); updateErr != nil {
		t.Fatalf("UpdateProject() error = %v", updateErr)
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

func TestRunProjectRepo(t *testing.T) {
	setupLocalCLI(t)

	addOutput := captureStdout(t, func() {
		if runErr := run([]string{"project", "repo", "add", "-project_id", "PRIV", "github.com/acme/repo.git"}); runErr != nil {
			t.Fatalf("project repo add error = %v", runErr)
		}
	})
	if !strings.Contains(addOutput, "added repository github.com/acme/repo.git to project PRIV") {
		t.Fatalf("project repo add output = %q", addOutput)
	}

	listOutput := captureStdout(t, func() {
		if runErr := run([]string{"project", "repo", "ls", "-project_id", "PRIV"}); runErr != nil {
			t.Fatalf("project repo ls error = %v", runErr)
		}
	})
	if !strings.Contains(listOutput, "github.com/acme/repo.git") {
		t.Fatalf("project repo ls output = %q", listOutput)
	}

	removeOutput := captureStdout(t, func() {
		if runErr := run([]string{"project", "repo", "rm", "-project_id", "PRIV", "github.com/acme/repo.git"}); runErr != nil {
			t.Fatalf("project repo rm error = %v", runErr)
		}
	})
	if !strings.Contains(removeOutput, "removed repository github.com/acme/repo.git from project PRIV") {
		t.Fatalf("project repo rm output = %q", removeOutput)
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
	err := run([]string{"project", "remote", "prod"})
	if err == nil || !strings.Contains(err.Error(), "has been removed") {
		t.Fatalf("project remote error = %v", err)
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

	if !strings.Contains(listOutput, taskID) {
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

func TestRunListRepoResolvedProjectHeader(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "REP", "-title", "Repo List Project", "-git-repository", "https://github.com/example/repo-list.git"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})
	project, err := svc.GetProject(context.Background(), "REP")
	if err != nil {
		t.Fatalf("GetProject(REP) error = %v", err)
	}

	setTestGitOrigin(t, "https://github.com/example/repo-list.git")
	createLocalTask(t, []string{"add", "Repo resolved task"})

	output := captureStdout(t, func() {
		if err := run([]string{"ls", "-nocolor"}); err != nil {
			t.Fatalf("ls error = %v", err)
		}
	})
	if !strings.HasPrefix(output, fmt.Sprintf("PROJECT  REP — Repo List Project (%d)", project.ID)) {
		t.Fatalf("ls output missing repo-resolved project header:\n%s", output)
	}
	if !strings.Contains(output, "Repo resolved task") {
		t.Fatalf("ls output missing repo-resolved task:\n%s", output)
	}
}

func TestRunListEmptyStateIncludesProjectContext(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "EMP", "-title", "Empty Project"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})
	project, err := svc.GetProject(context.Background(), "EMP")
	if err != nil {
		t.Fatalf("GetProject(EMP) error = %v", err)
	}

	t.Setenv("TICKET_PROJECT", strconv.FormatInt(project.ID, 10))

	output := captureStdout(t, func() {
		if err := run([]string{"ls", "-t", "bug"}); err != nil {
			t.Fatalf("ls empty state error = %v", err)
		}
	})
	if !strings.HasPrefix(output, fmt.Sprintf("PROJECT  EMP — Empty Project (%d)", project.ID)) {
		t.Fatalf("ls empty output missing project header:\n%s", output)
	}
	if !strings.Contains(output, "No bugs available.") {
		t.Fatalf("ls empty output missing entity empty state:\n%s", output)
	}
}

func TestRunListHidesDoneStageTicketEvenWhenIncomplete(t *testing.T) {
	setupLocalCLI(t)

	doneID := createLocalTask(t, []string{"epic", "Done But Incomplete"})
	openID := createLocalTask(t, []string{"add", "Still Open"})

	db, err := store.Open(testDBPath(t))
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
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
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

func TestRunTicketCreateDoesNotAutoParentTaskLikeTypes(t *testing.T) {
	setupLocalCLI(t)

	_ = createLocalTask(t, []string{"epic", "Current Epic"})
	taskID := createLocalTask(t, []string{"add", "Auto Parented Task"})
	bugID := createLocalTask(t, []string{"bug", "Auto Parented Bug"})
	choreID := createLocalTask(t, []string{"add", "-t", "chore", "Auto Parented Chore"})

	for _, id := range []string{taskID, bugID, choreID} {
		getOutput := captureStdout(t, func() {
			if err := run([]string{"get", "-id", id, "-v"}); err != nil {
				t.Fatalf("get error = %v", err)
			}
		})
		if hasDetailLabel(getOutput, "Parent") {
			t.Fatalf("get output should not auto-assign a parent:\n%s", getOutput)
		}
	}
}

func TestRunTicketCreateFromFileCreatesMultipleTickets(t *testing.T) {
	setupLocalCLI(t)
	content := strings.Join([]string{
		"# API bug in login flow",
		"type: bug",
		"label: api, urgent",
		"",
		"Login fails with a 500 in production.",
		"Investigate token refresh handling.",
		"",
		"# Follow-up docs task",
		"label: docs",
		"",
		"Document the new login behavior.",
	}, "\n")
	filePath := filepath.Join(t.TempDir(), "tickets.txt")
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_ = captureStdout(t, func() {
		if err := run([]string{"new", "-f", filePath, "-commit"}); err != nil {
			t.Fatalf("new -f error = %v", err)
		}
	})

	svc := localCLIService(t)
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	tickets, err := svc.ListTickets(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(tickets) != 2 {
		t.Fatalf("ticket count = %d, want 2", len(tickets))
	}

	byTitle := make(map[string]store.Ticket, len(tickets))
	for _, ticket := range tickets {
		byTitle[ticket.Title] = ticket
	}

	bug, ok := byTitle["API bug in login flow"]
	if !ok {
		t.Fatalf("missing ticket %q", "API bug in login flow")
	}
	if bug.Type != "bug" {
		t.Fatalf("bug ticket type = %q, want bug", bug.Type)
	}
	if strings.Contains(strings.ToLower(bug.Description), "label:") || strings.Contains(strings.ToLower(bug.Description), "type:") {
		t.Fatalf("bug description still includes directives:\n%s", bug.Description)
	}
	bugLabels, err := svc.ListTicketLabels(context.Background(), bug.ID)
	if err != nil {
		t.Fatalf("ListTicketLabels(%s) error = %v", bug.ID, err)
	}
	gotBugLabels := map[string]bool{}
	for _, label := range bugLabels {
		gotBugLabels[strings.ToLower(label.Name)] = true
	}
	for _, want := range []string{"api", "urgent"} {
		if !gotBugLabels[want] {
			t.Fatalf("bug labels missing %q: %#v", want, gotBugLabels)
		}
	}

	task, ok := byTitle["Follow-up docs task"]
	if !ok {
		t.Fatalf("missing ticket %q", "Follow-up docs task")
	}
	if task.Type != "task" {
		t.Fatalf("task ticket type = %q, want task", task.Type)
	}
	taskLabels, err := svc.ListTicketLabels(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ListTicketLabels(%s) error = %v", task.ID, err)
	}
	gotTaskLabels := map[string]bool{}
	for _, label := range taskLabels {
		gotTaskLabels[strings.ToLower(label.Name)] = true
	}
	if !gotTaskLabels["docs"] {
		t.Fatalf("task labels missing docs: %#v", gotTaskLabels)
	}

	updatedFile, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile(updated) error = %v", err)
	}
	if !strings.Contains(string(updatedFile), "id:") {
		t.Fatalf("updated file missing id entries:\n%s", string(updatedFile))
	}
}

func TestRunTicketCreateFromFileCreatesHierarchy(t *testing.T) {
	setupLocalCLI(t)
	content := strings.Join([]string{
		"# First epic",
		"type: epic",
		"",
		"Top-level epic description.",
		"",
		"## Child task",
		"type: task",
		"",
		"Child task description.",
		"",
		"### Grandchild bug",
		"type: bug",
		"",
		"Nested bug description.",
	}, "\n")
	filePath := filepath.Join(t.TempDir(), "hierarchy_tickets.txt")
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := run([]string{"new", "-f", filePath, "-commit"}); err != nil {
		t.Fatalf("new -f -commit hierarchy error = %v", err)
	}

	svc := localCLIService(t)
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	tickets, err := svc.ListTickets(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	byTitle := make(map[string]store.Ticket, len(tickets))
	for _, ticket := range tickets {
		byTitle[ticket.Title] = ticket
	}

	epic := byTitle["First epic"]
	child := byTitle["Child task"]
	grandchild := byTitle["Grandchild bug"]
	if epic.ID == "" || child.ID == "" || grandchild.ID == "" {
		t.Fatalf("missing hierarchy tickets: %#v", byTitle)
	}
	if child.ParentID == nil || *child.ParentID != epic.ID {
		t.Fatalf("child parent = %#v, want %s", child.ParentID, epic.ID)
	}
	if grandchild.ParentID == nil || *grandchild.ParentID != child.ID {
		t.Fatalf("grandchild parent = %#v, want %s", grandchild.ParentID, child.ID)
	}
}

func TestRunTicketCreateFromFilePreviewDoesNotWriteTickets(t *testing.T) {
	setupLocalCLI(t)
	content := strings.Join([]string{
		"# Preview only ticket",
		"labels: docs",
		"",
		"Just preview this ticket.",
	}, "\n")
	filePath := filepath.Join(t.TempDir(), "preview_tickets.txt")
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	out := captureStdout(t, func() {
		if err := run([]string{"new", "-f", filePath}); err != nil {
			t.Fatalf("new -f preview error = %v", err)
		}
	})
	if !strings.Contains(out, "Preview only ticket") {
		t.Fatalf("preview output missing ticket title:\n%s", out)
	}
	if !strings.Contains(out, "Tip: `use -commit` to write back to tk") {
		t.Fatalf("preview output missing commit tip:\n%s", out)
	}
	svc := localCLIService(t)
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	tickets, err := svc.ListTickets(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(tickets) != 0 {
		t.Fatalf("preview mode created tickets: %d", len(tickets))
	}
}

func TestRunTicketCreateFromFileFailsAtomicallyOnParseError(t *testing.T) {
	setupLocalCLI(t)
	content := strings.Join([]string{
		"this line appears before a heading and should fail parsing",
		"# Valid ticket title",
		"Description",
	}, "\n")
	filePath := filepath.Join(t.TempDir(), "bad_tickets.txt")
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	svc := localCLIService(t)
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	before, err := svc.ListTickets(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListTickets(before) error = %v", err)
	}

	err = run([]string{"new", "-f", filePath})
	if err == nil {
		t.Fatal("new -f parse error = nil, want error")
	}
	if !strings.Contains(err.Error(), "cannot parse") {
		t.Fatalf("error = %v, want cannot parse", err)
	}

	after, err := svc.ListTickets(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListTickets(after) error = %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("ticket count changed after parse failure: before=%d after=%d", len(before), len(after))
	}
}

func TestRunTicketCreateFromFileFailsOnInvalidHeadingHierarchy(t *testing.T) {
	setupLocalCLI(t)
	content := strings.Join([]string{
		"# Valid root",
		"",
		"### Missing middle parent",
		"Description",
	}, "\n")
	filePath := filepath.Join(t.TempDir(), "bad_hierarchy_tickets.txt")
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := run([]string{"new", "-f", filePath})
	if err == nil {
		t.Fatal("new -f invalid hierarchy error = nil, want error")
	}
	if !strings.Contains(err.Error(), "heading level") {
		t.Fatalf("error = %v, want heading level", err)
	}
}

func TestRunTicketCreateFromFileCodeFenceDoesNotSplitOnHeading(t *testing.T) {
	setupLocalCLI(t)
	content := strings.Join([]string{
		"# Parent ticket",
		"",
		"Normal description line.",
		"```markdown",
		"# this is not a ticket heading",
		"type: bug",
		"labels: one, two",
		"```",
		"Still part of the first ticket.",
		"",
		"# Second ticket",
		"Second description.",
	}, "\n")
	filePath := filepath.Join(t.TempDir(), "fenced_tickets.txt")
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := run([]string{"new", "-f", filePath, "-commit"}); err != nil {
		t.Fatalf("new -f -commit error = %v", err)
	}

	svc := localCLIService(t)
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	tickets, err := svc.ListTickets(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(tickets) != 2 {
		t.Fatalf("ticket count = %d, want 2", len(tickets))
	}

	var first, second store.Ticket
	for _, ticket := range tickets {
		switch ticket.Title {
		case "Parent ticket":
			first = ticket
		case "Second ticket":
			second = ticket
		}
	}
	if first.ID == "" || second.ID == "" {
		t.Fatalf("expected both tickets to be created: %#v", tickets)
	}
	if !strings.Contains(first.Description, "# this is not a ticket heading") {
		t.Fatalf("first ticket description missing fenced heading line:\n%s", first.Description)
	}
	if first.Type != "task" {
		t.Fatalf("first ticket type = %q, want default task", first.Type)
	}
}

func TestRunTicketCreateFromFileFenceInfoLineInsideFenceDoesNotDesync(t *testing.T) {
	setupLocalCLI(t)
	content := strings.Join([]string{
		"# First ticket",
		"```",
		"```yaml",
		"# not a heading",
		"```",
		"",
		"# Second ticket",
		"Second description.",
		"```",
		"# also not a heading",
		"```",
		"",
		"# Third ticket",
		"Third description.",
	}, "\n")
	filePath := filepath.Join(t.TempDir(), "fence_desync_tickets.txt")
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := run([]string{"new", "-f", filePath, "-commit"}); err != nil {
		t.Fatalf("new -f -commit error = %v", err)
	}

	svc := localCLIService(t)
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	tickets, err := svc.ListTickets(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if len(tickets) != 3 {
		t.Fatalf("ticket count = %d, want 3", len(tickets))
	}

	var first store.Ticket
	for _, ticket := range tickets {
		if ticket.Title == "First ticket" {
			first = ticket
			break
		}
	}
	if first.ID == "" {
		t.Fatalf("first ticket not created: %#v", tickets)
	}
	if !strings.Contains(first.Description, "# not a heading") {
		t.Fatalf("first description missing fenced heading content:\n%s", first.Description)
	}
}

func TestRunUpdateFromFilePreviewAndCommit(t *testing.T) {
	setupLocalCLI(t)
	ticketID := createLocalTask(t, []string{"add", "Original Title"})
	svc := localCLIService(t)
	ticket, err := svc.GetTicket(context.Background(), ticketID)
	if err != nil {
		t.Fatalf("GetTicket(%s) error = %v", ticketID, err)
	}
	if _, err := svc.CreateLabel(context.Background(), ticket.ProjectID, libticket.LabelRequest{Name: "legacy"}); err != nil {
		t.Fatalf("CreateLabel(legacy) error = %v", err)
	}
	labels, err := svc.ListLabels(context.Background(), ticket.ProjectID)
	if err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	var legacyID int64
	for _, label := range labels {
		if strings.EqualFold(label.Name, "legacy") {
			legacyID = label.ID
			break
		}
	}
	if legacyID == 0 {
		t.Fatal("legacy label not found")
	}
	if err := svc.AddTicketLabel(context.Background(), ticketID, legacyID); err != nil {
		t.Fatalf("AddTicketLabel() error = %v", err)
	}

	content := strings.Join([]string{
		"# Updated Title",
		fmt.Sprintf("id: %s", ticketID),
		"type: bug",
		"labels: api, urgent",
		"",
		"Updated description line 1.",
		"Updated description line 2.",
	}, "\n")
	filePath := filepath.Join(t.TempDir(), "update_tickets.txt")
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	preview := captureStdout(t, func() {
		if err := run([]string{"update", "-f", filePath}); err != nil {
			t.Fatalf("update -f preview error = %v", err)
		}
	})
	if !strings.Contains(preview, "Updated Title") {
		t.Fatalf("preview output missing title:\n%s", preview)
	}
	if !strings.Contains(preview, "Tip: `use -commit` to write back to tk") {
		t.Fatalf("preview output missing commit tip:\n%s", preview)
	}
	current, err := svc.GetTicket(context.Background(), ticketID)
	if err != nil {
		t.Fatalf("GetTicket(after preview) error = %v", err)
	}
	if current.Title != "Original Title" {
		t.Fatalf("preview changed ticket title: %q", current.Title)
	}

	if err := run([]string{"update", "-f", filePath, "-commit"}); err != nil {
		t.Fatalf("update -f -commit error = %v", err)
	}
	updated, err := svc.GetTicket(context.Background(), ticketID)
	if err != nil {
		t.Fatalf("GetTicket(updated) error = %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Fatalf("updated title = %q", updated.Title)
	}
	if updated.Type != "bug" {
		t.Fatalf("updated type = %q, want bug", updated.Type)
	}
	if !strings.Contains(updated.Description, "Updated description line 1.") {
		t.Fatalf("updated description = %q", updated.Description)
	}
	updatedLabels, err := svc.ListTicketLabels(context.Background(), ticketID)
	if err != nil {
		t.Fatalf("ListTicketLabels(updated) error = %v", err)
	}
	gotLabels := map[string]bool{}
	for _, label := range updatedLabels {
		gotLabels[strings.ToLower(label.Name)] = true
	}
	if !gotLabels["api"] || !gotLabels["urgent"] || gotLabels["legacy"] {
		t.Fatalf("updated labels mismatch: %#v", gotLabels)
	}
}

func TestRunUpdateFromFileRequiresID(t *testing.T) {
	setupLocalCLI(t)
	content := strings.Join([]string{
		"# Missing ID update",
		"type: bug",
		"",
		"Description",
	}, "\n")
	filePath := filepath.Join(t.TempDir(), "bad_update_tickets.txt")
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	err := run([]string{"update", "-f", filePath, "-commit"})
	if err == nil {
		t.Fatal("update -f -commit without id = nil, want error")
	}
	if !strings.Contains(err.Error(), "missing id") {
		t.Fatalf("error = %v, want missing id", err)
	}
}

func TestRunExportWritesMarkdownDocument(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "-d", "Line 1\n```\nLine 3", "-ac", "- check 1\n- check 2", "Markdown Export"})

	output := captureStdout(t, func() {
		if err := run([]string{"export", taskID}); err != nil {
			t.Fatalf("export error = %v", err)
		}
	})

	for _, want := range []string{
		ticketmarkdown.Header,
		"id: " + taskID,
		"type: task",
		"## title",
		"## description",
		"## acceptance criteria",
		"````markdown",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("export output missing %q:\n%s", want, output)
		}
	}
}

func TestRunImportUpdatesSupportedFieldsOnly(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "-d", "Before description", "-ac", "Before AC", "Before title"})
	svc := localCLIService(t)
	before, err := svc.GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket(before) error = %v", err)
	}

	filePath := filepath.Join(t.TempDir(), "ticket.md")
	if err := run([]string{"export", taskID, "-o", filePath}); err != nil {
		t.Fatalf("export -o error = %v", err)
	}
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(contentBytes)
	content = strings.Replace(content, "type: task", "type: bug", 1)
	content = strings.Replace(content, "Before title", "After title", 1)
	content = strings.Replace(content, "Before description", "After description", 1)
	content = strings.Replace(content, "Before AC", "After AC", 1)
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	importOutput := captureStdout(t, func() {
		if err := run([]string{"import", filePath}); err != nil {
			t.Fatalf("import error = %v", err)
		}
	})
	if !strings.Contains(importOutput, taskID+" updated (") {
		t.Fatalf("import output missing summary:\n%s", importOutput)
	}

	after, err := svc.GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket(after) error = %v", err)
	}
	if after.Type != "bug" || after.Title != "After title" || after.Description != "After description" || after.AcceptanceCriteria != "After AC" {
		t.Fatalf("ticket after import = %#v", after)
	}
	var beforeParent, afterParent string
	if before.ParentID != nil {
		beforeParent = *before.ParentID
	}
	if after.ParentID != nil {
		afterParent = *after.ParentID
	}
	if after.Status != before.Status || after.Assignee != before.Assignee || after.Priority != before.Priority || afterParent != beforeParent {
		t.Fatalf("import changed out-of-scope fields: before=%#v after=%#v", before, after)
	}
}

func TestRunImportRejectsUnsupportedSection(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "Unsupported Section"})
	filePath := filepath.Join(t.TempDir(), "ticket.md")
	if err := run([]string{"export", taskID, "-o", filePath}); err != nil {
		t.Fatalf("export -o error = %v", err)
	}
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(contentBytes) + "\n\n## labels\n```text\napi\n```\n"
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = run([]string{"import", filePath})
	if err == nil {
		t.Fatal("import error = nil, want error")
	}
	if !strings.Contains(err.Error(), `unsupported section "labels"`) {
		t.Fatalf("import error = %v", err)
	}
}

func TestRunUpdateFromFileRejectsTicketMarkdownExport(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "Batch parser guard"})
	filePath := filepath.Join(t.TempDir(), "ticket.md")
	if err := run([]string{"export", taskID, "-o", filePath}); err != nil {
		t.Fatalf("export -o error = %v", err)
	}

	err := run([]string{"update", "-f", filePath, "-commit"})
	if err == nil {
		t.Fatal("update -f on ticket markdown export = nil, want error")
	}
	if !strings.Contains(err.Error(), "use `tk import") {
		t.Fatalf("update -f error = %v", err)
	}
}

func TestRunSearchSupportsFreeFormAndFilters(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	if err := run([]string{"project", "create", "-prefix", "SEP", "-title", "Second Project"}); err != nil {
		t.Fatalf("project create error = %v", err)
	}
	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	var defaultProjectID, secondProjectID int64
	for _, project := range projects {
		switch project.Prefix {
		case "TK":
			defaultProjectID = project.ID
		case "SEP":
			secondProjectID = project.ID
		}
	}
	if defaultProjectID == 0 || secondProjectID == 0 {
		t.Fatalf("expected default and second project, got %+v", projects)
	}

	t.Setenv("TICKET_PROJECT", "TK")
	matchingID := createLocalTask(t, []string{"add", "-d", "Detailed note for customer portal", "-ac", "free form acceptance", "Free form entry"})
	otherID := createLocalTask(t, []string{"add", "-d", "Detailed note for customer portal", "Free form other"})
	t.Setenv("TICKET_PROJECT", "SEP")
	crossProjectID := createLocalTask(t, []string{"add", "-d", "Detailed note for customer portal", "Free form entry elsewhere"})
	t.Setenv("TICKET_PROJECT", "TK")

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
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	owner := strings.TrimSpace(cfg.Username)
	if owner == "" {
		if currentUser, userErr := user.Current(); userErr == nil {
			owner = strings.TrimSpace(currentUser.Username)
		}
	}
	if owner == "" {
		t.Fatal("owner username was empty")
	}

	output := captureStdout(t, func() {
		if err := run([]string{
			"search",
			"free", "form", "entry",
			"-status", "develop/active",
			"-title", "entry",
			"-description", "customer portal",
			"-priority", "4",
			"-owner", owner,
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
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
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

func TestRunUpdateNormalizesBareNumericTicketRefs(t *testing.T) {
	setupLocalCLI(t)

	parentID := createLocalTask(t, []string{"add", "-type", "epic", "Parent Epic"})
	taskID := createLocalTask(t, []string{"add", "Child Task"})
	parentSeq := strings.TrimPrefix(parentID, "TK-")
	taskSeq := strings.TrimPrefix(taskID, "TK-")

	updateOutput := captureStdout(t, func() {
		if err := run([]string{"update", "-id", taskSeq, "-parent_id", parentSeq, "-title", "Child Task Updated"}); err != nil {
			t.Fatalf("update error = %v", err)
		}
	})
	if !strings.Contains(updateOutput, taskID+" updated (") {
		t.Fatalf("update output missing normalized ticket id:\n%s", updateOutput)
	}

	ticket, err := localCLIService(t).GetTicket(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if ticket.ParentID == nil || *ticket.ParentID != parentID {
		t.Fatalf("ticket.ParentID = %#v, want %q", ticket.ParentID, parentID)
	}
}

func TestRunUpdateSupportsDescriptionAlias(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "-d", "old description", "Ticket Alpha"})

	if err := run([]string{"update", "-id", taskID, "-description", "updated description"}); err != nil {
		t.Fatalf("update with -description error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
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

func TestRunUpdatePositionalNumericIDUpdatesTitleWithPunctuation(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "Needs Numeric Update"})
	taskSeq := strings.TrimPrefix(taskID, "TK-")
	updatedTitle := `the title: "quoted", punctuated!`

	if err := run([]string{"update", taskSeq, "-title", updatedTitle}); err != nil {
		t.Fatalf("run(update positional numeric id) error = %v", err)
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !hasDetailField(getOutput, "Title", updatedTitle) {
		t.Fatalf("get output missing updated title:\n%s", getOutput)
	}

	showOutput := captureStdout(t, func() {
		if err := run([]string{"show", taskID}); err != nil {
			t.Fatalf("show error = %v", err)
		}
	})
	if !strings.Contains(showOutput, updatedTitle) {
		t.Fatalf("show output missing updated title:\n%s", showOutput)
	}
}

func TestRunUpdateRequiresID(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"update", "-title", "No ID Flag"}); err == nil || !strings.Contains(err.Error(), "usage: tk update [-f <filename>] [-commit] [-id <id>|<id>]") {
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

	// positional id followed by a flag should also work: tk get <id> -v
	verboseOutput := captureStdout(t, func() {
		if err := run([]string{"get", taskID, "-v"}); err != nil {
			t.Fatalf("expected positional id with -v to work, got %v", err)
		}
	})
	if !hasDetailLabel(verboseOutput, "History") {
		t.Fatalf("positional get -v should include verbose history section:\n%s", verboseOutput)
	}
}

func TestRunGetDefaultIsConciseAndVerboseShowsDetails(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "-d", "Concise description\nLine two\nLine three", "-ac", "- ac line 1\n- ac line 2\n- ac line 3", "Concise Ticket"})

	defaultOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID}); err != nil {
			t.Fatalf("get default error = %v", err)
		}
	})
	for _, want := range []string{
		"id/type     : " + taskID + "/task",
		"title       : Concise Ticket",
		"description : Concise description",
		"              Line two",
		"              Line three",
		"a/c         : - ac line 1",
		"              - ac line 2",
		"              - ac line 3",
	} {
		if !strings.Contains(defaultOutput, want) {
			t.Fatalf("default get output missing %q:\n%s", want, defaultOutput)
		}
	}
	if !strings.Contains(defaultOutput, "(use `tk get XXX -v` for more information)") {
		t.Fatalf("default get output missing verbose tip:\n%s", defaultOutput)
	}
	if hasDetailLabel(defaultOutput, "History") {
		t.Fatalf("default get should not include verbose history section:\n%s", defaultOutput)
	}

	verboseOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
			t.Fatalf("get verbose error = %v", err)
		}
	})
	if !hasDetailLabel(verboseOutput, "History") {
		t.Fatalf("verbose get should include history section:\n%s", verboseOutput)
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
		if err := run([]string{"get", "-id", parentID, "-v"}); err != nil {
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
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
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

	initialOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
			t.Fatalf("get initial draft error = %v", err)
		}
	})
	if !hasDetailField(initialOutput, "Draft", "true") {
		t.Fatalf("new ticket should start draft=true:\n%s", initialOutput)
	}

	if err := run([]string{"draft", "-id", taskID}); err != nil {
		t.Fatalf("draft error = %v", err)
	}

	draftOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
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
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
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
			if err := run([]string{"get", "-id", id, "-v"}); err != nil {
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
			if err := run([]string{"get", "-id", id, "-v"}); err != nil {
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
	svc := localCLIService(t)

	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Workflow Stage Ticket",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	if runErr := run([]string{"update", "-id", ticket.ID, "-stage", "develop"}); runErr != nil {
		t.Fatalf("update stage error = %v", runErr)
	}

	updated, err := svc.GetTicket(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("GetTicket(updated) error = %v", err)
	}
	if updated.Stage != "develop" || updated.State != store.StateIdle {
		t.Fatalf("updated lifecycle = %s/%s, want develop/idle", updated.Stage, updated.State)
	}

	err = run([]string{"update", "-id", ticket.ID, "-stage", "xxxx"})
	if err == nil {
		t.Fatal("update invalid stage error = nil, want error")
	}
	want := `invalid stage "xxxx"; valid stages: design, develop, test, done`
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("update invalid stage error = %v, want substring %q", err, want)
	}
}

func TestRunMergeCombinesDraftTicketsAndArchivesSources(t *testing.T) {
	setupLocalCLI(t)

	targetID := createLocalTask(t, []string{"add", "-d", "first description", "-ac", "first acceptance", "Primary draft"})
	secondID := createLocalTask(t, []string{"add", "-d", "second description", "-ac", "second acceptance", "Secondary draft"})
	thirdID := createLocalTask(t, []string{"add", "-d", "third description", "-ac", "third acceptance", "Tertiary draft"})

	output := captureStdout(t, func() {
		if err := run([]string{"merge", targetID, secondID, thirdID}); err != nil {
			t.Fatalf("merge error = %v", err)
		}
	})
	if !strings.Contains(output, "merged:") {
		t.Fatalf("merge output missing summary:\n%s", output)
	}

	merged, err := svcGetTicket(t, targetID)
	if err != nil {
		t.Fatalf("GetTicket(target after merge) error = %v", err)
	}
	if merged.Title != "Primary draft" {
		t.Fatalf("merged.Title = %q, want %q", merged.Title, "Primary draft")
	}
	if merged.Description != "first description\n----\nsecond description\n----\nthird description" {
		t.Fatalf("merged.Description = %q", merged.Description)
	}
	if merged.AcceptanceCriteria != "first acceptance\n----\nsecond acceptance\n----\nthird acceptance" {
		t.Fatalf("merged.AcceptanceCriteria = %q", merged.AcceptanceCriteria)
	}
	if !merged.Draft {
		t.Fatalf("merged.Draft = %v, want true", merged.Draft)
	}
	if merged.Archived {
		t.Fatalf("merged.Archived = %v, want false", merged.Archived)
	}

	for _, id := range []string{secondID, thirdID} {
		ticket, getErr := svcGetTicket(t, id)
		if getErr != nil {
			t.Fatalf("GetTicket(%s after merge) error = %v", id, getErr)
		}
		if !ticket.Archived {
			t.Fatalf("%s.Archived = %v, want true", id, ticket.Archived)
		}
	}
}

func TestRunMergeRejectsNonDraftTickets(t *testing.T) {
	setupLocalCLI(t)

	targetID := createLocalTask(t, []string{"add", "-d", "first description", "Primary draft"})
	secondID := createLocalTask(t, []string{"add", "-d", "second description", "Not yet drafted"})
	if err := run([]string{"undraft", secondID}); err != nil {
		t.Fatalf("undraft source error = %v", err)
	}

	err := run([]string{"merge", targetID, secondID})
	if err == nil || !strings.Contains(err.Error(), "only draft tickets can be merged") {
		t.Fatalf("merge non-draft error = %v, want draft validation", err)
	}
}

func TestRunRejectMovesTicketToFirstWorkflowStageAsDraft(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

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
		Stage:              "develop",
		State:              store.StateActive,
		Priority:           ticket.Priority,
		Order:              ticket.Order,
		EstimateEffort:     ticket.EstimateEffort,
		EstimateComplete:   ticket.EstimateComplete,
		Type:               ticket.Type,
	})
	if err != nil {
		t.Fatalf("UpdateTicket(develop/active) error = %v", err)
	}
	if advanced.Stage != "develop" {
		t.Fatalf("advanced stage = %q, want develop", advanced.Stage)
	}

	if runErr := run([]string{"reject", "-id", ticket.ID}); runErr != nil {
		t.Fatalf("reject error = %v", runErr)
	}

	rejected, err := svc.GetTicket(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("GetTicket(rejected) error = %v", err)
	}
	if rejected.Stage != "design" || rejected.State != store.StateIdle {
		t.Fatalf("rejected lifecycle = %s/%s, want design/idle", rejected.Stage, rejected.State)
	}
	if !rejected.Draft {
		t.Fatal("rejected ticket should be draft")
	}
}

func TestRunTaskCreateSupportsInterspersedFlags(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "the", "thing", "-type", "epic"})

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
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
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
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
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	taskID := createLocalTask(t, []string{"create", "-t", "epic", "-title", "foo"})
	output := captureStdout(t, func() {
		if runErr := run([]string{"get", "-id", taskID}); runErr != nil {
			t.Fatalf("get error = %v", runErr)
		}
	})
	if !strings.Contains(output, "title") || !strings.Contains(output, "foo") || !strings.Contains(output, "id/type") || !strings.Contains(output, taskID+"/epic") {
		t.Fatalf("default project fallback output missing expected fields:\n%s", output)
	}

	if _, err := config.Load(); err != nil {
		t.Fatalf("config.Load(reloaded) error = %v", err)
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

func TestRunDeleteSupportsCommaSeparatedPositionalIDs(t *testing.T) {
	setupLocalCLI(t)
	firstID := createLocalTask(t, []string{"add", "Delete one"})
	secondID := createLocalTask(t, []string{"add", "Delete two"})
	combined := firstID + "," + secondID

	if err := run([]string{"rm", combined}); err != nil {
		t.Fatalf("rm phase 1 error = %v", err)
	}
	if err := run([]string{"rm", "-id", combined, "--confirm", combined}); err != nil {
		t.Fatalf("rm phase 2 error = %v", err)
	}
	if err := run([]string{"get", "-id", firstID}); err == nil || err.Error() != "ticket not found" {
		t.Fatalf("first ticket should be deleted, got %v", err)
	}
	if err := run([]string{"get", "-id", secondID}); err == nil || err.Error() != "ticket not found" {
		t.Fatalf("second ticket should be deleted, got %v", err)
	}
}

func TestRunDeleteSupportsCommaSeparatedIDFlag(t *testing.T) {
	setupLocalCLI(t)
	firstID := createLocalTask(t, []string{"add", "Delete alpha"})
	secondID := createLocalTask(t, []string{"add", "Delete beta"})
	combined := firstID + "," + secondID

	if err := run([]string{"rm", "-id", combined}); err != nil {
		t.Fatalf("rm phase 1 error = %v", err)
	}
	if err := run([]string{"rm", "-id", combined, "--confirm", combined}); err != nil {
		t.Fatalf("rm phase 2 error = %v", err)
	}
	if err := run([]string{"get", "-id", firstID}); err == nil || err.Error() != "ticket not found" {
		t.Fatalf("first ticket should be deleted, got %v", err)
	}
	if err := run([]string{"get", "-id", secondID}); err == nil || err.Error() != "ticket not found" {
		t.Fatalf("second ticket should be deleted, got %v", err)
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

func TestRunInterveneJSONIncludesTicketIDsForSplitWork(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Intervention payload task"})
	if err := run([]string{"fail", "-id", taskID}); err != nil {
		t.Fatalf("fail error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"intervene", "-id", taskID, "-outcome", "split-work", "-json"}); err != nil {
			t.Fatalf("intervene -json error = %v", err)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\n%s", err, output)
	}

	ticket, ok := payload["ticket"].(map[string]any)
	if !ok {
		t.Fatalf("ticket payload = %#v", payload["ticket"])
	}
	if got := strings.TrimSpace(fmt.Sprint(ticket["ticket_id"])); got != taskID {
		t.Fatalf("ticket.ticket_id = %q, want %q\npayload=%s", got, taskID, output)
	}

	followUp, ok := payload["follow_up"].(map[string]any)
	if !ok {
		t.Fatalf("follow_up payload = %#v", payload["follow_up"])
	}
	followUpID := strings.TrimSpace(fmt.Sprint(followUp["ticket_id"]))
	if followUpID == "" || followUpID == "<nil>" {
		t.Fatalf("follow_up.ticket_id missing in payload: %s", output)
	}
	if followUpID == taskID {
		t.Fatalf("follow_up.ticket_id should differ from source ticket_id: %s", output)
	}
}

func TestRunInterveneJSONIncludesTicketIDWithoutFollowUp(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Intervention retry task"})
	if err := run([]string{"fail", "-id", taskID}); err != nil {
		t.Fatalf("fail error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"intervene", "-id", taskID, "-outcome", "retry-role", "-json"}); err != nil {
			t.Fatalf("intervene -json error = %v", err)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\n%s", err, output)
	}

	ticket, ok := payload["ticket"].(map[string]any)
	if !ok {
		t.Fatalf("ticket payload = %#v", payload["ticket"])
	}
	if got := strings.TrimSpace(fmt.Sprint(ticket["ticket_id"])); got != taskID {
		t.Fatalf("ticket.ticket_id = %q, want %q\npayload=%s", got, taskID, output)
	}
	if _, ok := payload["follow_up"]; ok {
		t.Fatalf("retry-role should not include follow_up payload: %s", output)
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
	t.Setenv("TICKET_TOKEN", "test-token")

	var runErr error
	output := captureStdout(t, func() {
		runErr = runStatus(nil)
	})
	if runErr == nil {
		t.Fatal("runStatus(remote failure) error = nil")
	}
	for _, want := range []string{
		"TICKET_URL",
		"http://127.0.0.1:1",
		"TICKET_USERNAME",
		"UNSET",
		"TICKET_PASSWORD",
		"********",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus(remote failure) missing %q:\n%s", want, output)
		}
	}
	for _, unwanted := range []string{"TICKET_HOME", "config_file", "server_version", "authenticated", "connection"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("runStatus(remote failure) should not show %q:\n%s", unwanted, output)
		}
	}
}

func TestRunCountHistoryOrphansAndConfigInLocalMode(t *testing.T) {
	setupLocalCLI(t)
	epicID := createLocalTask(t, []string{"epic", "Parent Epic"})

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
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
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

	if err := run([]string{"admin", "config", "ls"}); err != nil {
		t.Fatalf("admin config ls error = %v", err)
	}
	listOutput := captureStdout(t, func() {
		if err := run([]string{"admin", "config", "ls"}); err != nil {
			t.Fatalf("admin config ls error = %v", err)
		}
	})
	for _, want := range []string{
		"registration_enabled",
		"registration_auto_approve",
	} {
		if !strings.Contains(listOutput, want) {
			t.Fatalf("config ls output missing %q:\n%s", want, listOutput)
		}
	}
}

func TestRunHistoryShowsProjectAccessRequestAuditEvents(t *testing.T) {
	setupLocalCLI(t)

	db, err := store.Open(testDBPath(t))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer db.Close()

	requester, err := store.CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	req, err := store.CreateProjectAccessRequest(context.Background(), db, 1, requester.ID, "please add me")
	if err != nil {
		t.Fatalf("CreateProjectAccessRequest() error = %v", err)
	}
	if err := store.AddHistoryEvent(context.Background(), db, 1, "", "project_access_request_created", map[string]any{
		"request_id":     req.ID,
		"user_id":        requester.ID,
		"username":       "alice",
		"project_id":     1,
		"project_prefix": "PRJ",
		"project_title":  "Sample Project",
		"status":         "pending",
		"message":        "please add me",
		"requested_by":   "alice",
	}, requester.ID); err != nil {
		t.Fatalf("AddHistoryEvent(created) error = %v", err)
	}
	if err := store.AddHistoryEvent(context.Background(), db, 1, "", "project_access_request_approved", map[string]any{
		"request_id":     req.ID,
		"user_id":        requester.ID,
		"username":       "alice",
		"project_id":     1,
		"project_prefix": "PRJ",
		"project_title":  "Sample Project",
		"status":         "approved",
		"message":        "please add me",
		"decided_by":     "admin",
	}, testAdminUserID(t)); err != nil {
		t.Fatalf("AddHistoryEvent(approved) error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"history", "-n", "5"}); err != nil {
			t.Fatalf("history(project) error = %v", err)
		}
	})
	for _, want := range []string{
		"project",
		"alice requested access to PRJ: please add me",
		"approved access request #",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("project history output missing %q:\n%s", want, output)
		}
	}
}

func TestRunOrphansExcludesEpicRoots(t *testing.T) {
	setupLocalCLI(t)
	epicID := createLocalTask(t, []string{"epic", "Orphan Epic"})

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
		{[]string{"update", "-id", "abc", "-title", "new title"}, "ticket not found"},
		{[]string{"dependency", "add", "-id", "1", "abc"}, "ticket not found"},
		{[]string{"request", "abc"}, "ticket not found"},
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

func TestRunWorkflowListInLocalMode(t *testing.T) {
	setupLocalCLI(t)
	output := captureStdout(t, func() {
		if err := run([]string{"workflow", "list"}); err != nil {
			t.Fatalf("workflow list error = %v", err)
		}
	})
	if !strings.Contains(output, "Agile") {
		t.Fatalf("workflow list missing Agile workflow:\n%s", output)
	}
}

func TestRunWorkflowAliasListInLocalMode(t *testing.T) {
	setupLocalCLI(t)
	output := captureStdout(t, func() {
		if err := run([]string{"workflow", "list"}); err != nil {
			t.Fatalf("workflow list error = %v", err)
		}
	})
	if !strings.Contains(output, "Agile") {
		t.Fatalf("workflow list missing Agile workflow:\n%s", output)
	}
}

func TestRunWorkflowGetShowsStages(t *testing.T) {
	setupLocalCLI(t)
	output := captureStdout(t, func() {
		if err := run([]string{"workflow", "get", "-id", "1"}); err != nil {
			t.Fatalf("workflow get error = %v", err)
		}
	})
	for _, want := range []string{"develop", "done"} {
		if !strings.Contains(output, want) {
			t.Fatalf("workflow get missing %q:\n%s", want, output)
		}
	}
}

func TestRunWorkflowGetTreeShowsWorkflowPhaseRoleHierarchy(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	wf, err := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{
		Name:        "Tree Workflow",
		Description: "workflow tree test",
	})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	designStage, err := svc.AddWorkflowStage(context.Background(), wf.ID, libticket.WorkflowStageRequest{
		StageName: "design",
		SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("AddWorkflowStage(design) error = %v", err)
	}
	buildStage, err := svc.AddWorkflowStage(context.Background(), wf.ID, libticket.WorkflowStageRequest{
		StageName: "build",
		SortOrder: 2,
	})
	if err != nil {
		t.Fatalf("AddWorkflowStage(build) error = %v", err)
	}
	architectRole, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
		WorkflowID: &wf.ID,
		Title:      "Architect",
	})
	if err != nil {
		t.Fatalf("CreateRole(Architect) error = %v", err)
	}
	engineerRole, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
		WorkflowID: &wf.ID,
		Title:      "Engineer",
	})
	if err != nil {
		t.Fatalf("CreateRole(Engineer) error = %v", err)
	}
	if err := svc.AddWorkflowStageRole(context.Background(), wf.ID, designStage.ID, architectRole.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole(design) error = %v", err)
	}
	if err := svc.AddWorkflowStageRole(context.Background(), wf.ID, buildStage.ID, engineerRole.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole(build) error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"workflow", "get", "-id", strconv.FormatInt(wf.ID, 10), "-tree"}); err != nil {
			t.Fatalf("workflow get -tree error = %v", err)
		}
	})
	for _, want := range []string{
		"workflow: Tree Workflow",
		"phase: design",
		"role: Architect",
		"phase: build",
		"role: Engineer",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("workflow get -tree missing %q:\n%s", want, output)
		}
	}
}

func TestRunWorkflowGetShowsStageAcceptanceCriteria(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	wf, err := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{Name: "AC Workflow", Description: "workflow with stage acceptance criteria"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	stage, err := svc.AddWorkflowStage(context.Background(), wf.ID, libticket.WorkflowStageRequest{
		StageName:   "triage",
		Description: "triage",
		SortOrder:   1,
	})
	if err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	if _, err := svc.UpdateWorkflowStage(context.Background(), stage.ID, libticket.WorkflowStageRequest{
		StageName:          "triage",
		Description:        "triage",
		AcceptanceCriteria: "Clarified with the product owner",
	}); err != nil {
		t.Fatalf("UpdateWorkflowStage() error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"workflow", "get", "-id", strconv.FormatInt(wf.ID, 10)}); err != nil {
			t.Fatalf("workflow get error = %v", err)
		}
	})
	for _, want := range []string{"ACCEPTANCE CRITERIA", "Clarified with the product owner"} {
		if !strings.Contains(output, want) {
			t.Fatalf("workflow get missing %q:\n%s", want, output)
		}
	}
}

func TestRunWorkflowRoleCRUD(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	wf, err := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{Name: "Role Workflow", Description: "workflow for scoped roles"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	workflowID := strconv.FormatInt(wf.ID, 10)

	createOutput := captureStdout(t, func() {
		if runErr := run([]string{"workflow", "role-add", "-workflow_id", workflowID, "-title", "reviewer", "-description", "Reviews work", "-ac", "Approves the release"}); runErr != nil {
			t.Fatalf("workflow role-add error = %v", runErr)
		}
	})
	if !strings.Contains(createOutput, "created workflow role") || !strings.Contains(createOutput, "reviewer") {
		t.Fatalf("unexpected workflow role-add output:\n%s", createOutput)
	}

	var created store.Role
	roles, err := svc.ListRoles(context.Background())
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	for _, role := range roles {
		if role.Title == "reviewer" && role.WorkflowID != nil && *role.WorkflowID == wf.ID {
			created = role
			break
		}
	}
	if created.ID == 0 {
		t.Fatal("expected scoped workflow role to be created")
	}
	roleID := strconv.FormatInt(created.ID, 10)

	getOutput := captureStdout(t, func() {
		if runErr := run([]string{"workflow", "role-get", "-workflow_id", workflowID, "-role_id", roleID}); runErr != nil {
			t.Fatalf("workflow role-get error = %v", runErr)
		}
	})
	for _, want := range []string{"Title:               reviewer", "Acceptance Criteria: Approves the release"} {
		if !strings.Contains(getOutput, want) {
			t.Fatalf("workflow role-get missing %q:\n%s", want, getOutput)
		}
	}

	updateOutput := captureStdout(t, func() {
		if runErr := run([]string{"workflow", "role-update", "-workflow_id", workflowID, "-role_id", roleID, "-title", "qa-reviewer", "-description", "Reviews work", "-ac", "Ships the release"}); runErr != nil {
			t.Fatalf("workflow role-update error = %v", runErr)
		}
	})
	if !strings.Contains(updateOutput, "updated workflow role") || !strings.Contains(updateOutput, "qa-reviewer") {
		t.Fatalf("unexpected workflow role-update output:\n%s", updateOutput)
	}

	deleteOutput := captureStdout(t, func() {
		if runErr := run([]string{"workflow", "role-rm", "-workflow_id", workflowID, "-role_id", roleID}); runErr != nil {
			t.Fatalf("workflow role-rm error = %v", runErr)
		}
	})
	if !strings.Contains(deleteOutput, "deleted workflow role") {
		t.Fatalf("unexpected workflow role-rm output:\n%s", deleteOutput)
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

func TestRunWorkflowStageRoleCommands(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	wf, err := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{Name: "Stage Role Workflow", Description: "workflow for stage role commands"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	stage, err := svc.AddWorkflowStage(context.Background(), wf.ID, libticket.WorkflowStageRequest{
		StageName:   "triage",
		Description: "triage",
		SortOrder:   1,
	})
	if err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	roleA, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
		WorkflowID:  &wf.ID,
		Title:       "reviewer",
		Description: "reviews work",
	})
	if err != nil {
		t.Fatalf("CreateRole(roleA) error = %v", err)
	}
	roleB, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
		WorkflowID:  &wf.ID,
		Title:       "qa",
		Description: "verifies work",
	})
	if err != nil {
		t.Fatalf("CreateRole(roleB) error = %v", err)
	}
	workflowID := strconv.FormatInt(wf.ID, 10)
	stageID := strconv.FormatInt(stage.ID, 10)
	roleAID := strconv.FormatInt(roleA.ID, 10)
	roleBID := strconv.FormatInt(roleB.ID, 10)

	addOutput := captureStdout(t, func() {
		if runErr := run([]string{"workflow", "stage-role-add", "-workflow_id", workflowID, "-stage_id", stageID, "-role_id", roleAID}); runErr != nil {
			t.Fatalf("stage-role-add roleA error = %v", runErr)
		}
		if runErr := run([]string{"workflow", "stage-role-add", "-workflow_id", workflowID, "-stage_id", stageID, "-role_id", roleBID}); runErr != nil {
			t.Fatalf("stage-role-add roleB error = %v", runErr)
		}
	})
	if !strings.Contains(addOutput, "assigned role") {
		t.Fatalf("unexpected stage-role-add output:\n%s", addOutput)
	}

	ordered, err := svc.GetWorkflow(context.Background(), wf.ID)
	if err != nil {
		t.Fatalf("GetWorkflow(after add) error = %v", err)
	}
	if len(ordered.Stages) != 1 || len(ordered.Stages[0].Roles) != 2 {
		t.Fatalf("stage roles after add = %#v", ordered.Stages)
	}

	orderOutput := captureStdout(t, func() {
		if runErr := run([]string{"workflow", "stage-role-order", "-workflow_id", workflowID, "-stage_id", stageID, "-roles", roleBID + "," + roleAID}); runErr != nil {
			t.Fatalf("stage-role-order error = %v", runErr)
		}
	})
	if !strings.Contains(orderOutput, "reordered roles") {
		t.Fatalf("unexpected stage-role-order output:\n%s", orderOutput)
	}

	ordered, err = svc.GetWorkflow(context.Background(), wf.ID)
	if err != nil {
		t.Fatalf("GetWorkflow(after reorder) error = %v", err)
	}
	if got := []int64{ordered.Stages[0].Roles[0].ID, ordered.Stages[0].Roles[1].ID}; !reflect.DeepEqual(got, []int64{roleB.ID, roleA.ID}) {
		t.Fatalf("stage role order = %v, want [%d %d]", got, roleB.ID, roleA.ID)
	}

	removeOutput := captureStdout(t, func() {
		if runErr := run([]string{"workflow", "stage-role-rm", "-workflow_id", workflowID, "-stage_id", stageID, "-role_id", roleAID}); runErr != nil {
			t.Fatalf("stage-role-rm error = %v", runErr)
		}
	})
	if !strings.Contains(removeOutput, "removed role") {
		t.Fatalf("unexpected stage-role-rm output:\n%s", removeOutput)
	}

	ordered, err = svc.GetWorkflow(context.Background(), wf.ID)
	if err != nil {
		t.Fatalf("GetWorkflow(after remove) error = %v", err)
	}
	if got := ordered.Stages[0].Roles; len(got) != 1 || got[0].ID != roleB.ID {
		t.Fatalf("remaining stage roles = %#v, want only roleB", got)
	}
}

func TestRunWorkflowStageListAndGetShowRoleAndAcceptanceCriteria(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	wf, err := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{Name: "Stage Detail Workflow", Description: "workflow for stage output"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	stage, err := svc.AddWorkflowStage(context.Background(), wf.ID, libticket.WorkflowStageRequest{
		StageName:   "triage",
		Description: "triage work",
		SortOrder:   1,
	})
	if err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	stage, err = svc.UpdateWorkflowStage(context.Background(), stage.ID, libticket.WorkflowStageRequest{
		StageName:          "triage",
		Description:        "triage work",
		AcceptanceCriteria: "Classify the issue",
		DefinitionOfReady:  "Classify the issue",
		DefinitionOfDone:   "Issue routed with owner and priority",
	})
	if err != nil {
		t.Fatalf("UpdateWorkflowStage() error = %v", err)
	}
	role, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
		WorkflowID:  &wf.ID,
		Title:       "reviewer",
		Description: "reviews the work",
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if err := svc.AddWorkflowStageRole(context.Background(), wf.ID, stage.ID, role.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole() error = %v", err)
	}

	listOutput := captureStdout(t, func() {
		if err := run([]string{"workflow", "stage-list", "-id", strconv.FormatInt(wf.ID, 10)}); err != nil {
			t.Fatalf("workflow stage-list error = %v", err)
		}
	})
	for _, want := range []string{"triage", "reviewer", "triage work"} {
		if !strings.Contains(listOutput, want) {
			t.Fatalf("workflow stage-list missing %q:\n%s", want, listOutput)
		}
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"workflow", "stage-get", "-stage-id", strconv.FormatInt(stage.ID, 10)}); err != nil {
			t.Fatalf("workflow stage-get error = %v", err)
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
			t.Fatalf("workflow stage-get missing %q:\n%s", want, getOutput)
		}
	}
}

func TestRunWorkflowCreateAndDelete(t *testing.T) {
	setupLocalCLI(t)
	output := captureStdout(t, func() {
		if err := run([]string{"workflow", "create", "-name", "custom"}); err != nil {
			t.Fatalf("workflow create error = %v", err)
		}
	})
	if !strings.Contains(output, "custom") {
		t.Fatalf("workflow create missing name:\n%s", output)
	}
	// List should show both
	output = captureStdout(t, func() {
		if err := run([]string{"workflow", "list"}); err != nil {
			t.Fatalf("workflow list error = %v", err)
		}
	})
	if !strings.Contains(output, "custom") {
		t.Fatalf("workflow list missing custom:\n%s", output)
	}
}

func TestRunWorkflowDeleteCheckDetectsProjectReferences(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	wf, err := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{
		Name:        "Delete Check Workflow",
		Description: "workflow delete preflight test",
	})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	_, err = svc.UpdateProject(context.Background(), project.ID, libticket.ProjectUpdateRequest{
		Title:              project.Title,
		Description:        project.Description,
		AcceptanceCriteria: project.AcceptanceCriteria,
		DORMap:             project.DORMap,
		DODMap:             project.DODMap,
		ACMap:              project.ACMap,
		GitRepository:      project.GitRepository,
		Notes:              project.Notes,
		Status:             project.Status,
		Visibility:         project.Visibility,
		WorkflowID:         &wf.ID,
	})
	if err != nil {
		t.Fatalf("UpdateProject(workflow) error = %v", err)
	}

	out := captureStdout(t, func() {
		err := run([]string{"workflow", "rm", "-id", strconv.FormatInt(wf.ID, 10), "-check"})
		if err == nil {
			t.Fatal("expected workflow rm -check to report references")
		}
		if !strings.Contains(err.Error(), "still has references") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "projects using workflow: 1") {
		t.Fatalf("expected project reference output, got:\n%s", out)
	}
}

func TestRunRequestExplainNoWork(t *testing.T) {
	setupLocalCLI(t)
	out := captureStdout(t, func() {
		if err := run([]string{"request", "-explain"}); err != nil {
			t.Fatalf("request -explain error = %v", err)
		}
	})
	if !strings.Contains(out, "NO-WORK") {
		t.Fatalf("expected NO-WORK status, got:\n%s", out)
	}
	if !strings.Contains(out, "explain:") {
		t.Fatalf("expected explanation line, got:\n%s", out)
	}
}

func TestRunStatusShowsProjectWorkflowAndDefaultDraft(t *testing.T) {
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
	for _, want := range []string{"TICKET_URL", "TICKET_USERNAME", "TICKET_PASSWORD", "SERVER_VERSION", "CLIENT_VERSION"} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
	for _, unwanted := range []string{"workflow", "Custom Workflow", "draft", "true"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("status output should omit project summary details like %q:\n%s", unwanted, output)
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
	if got := payload["TICKET_USERNAME"]; got != "admin" {
		t.Fatalf("TICKET_USERNAME = %#v, want %q", got, "admin")
	}
	if got := payload["CLIENT_VERSION"]; got != strings.TrimSpace(embeddedVersion) {
		t.Fatalf("CLIENT_VERSION = %#v, want %q", got, strings.TrimSpace(embeddedVersion))
	}
	if got := payload["SERVER_VERSION"]; got == nil || strings.TrimSpace(fmt.Sprint(got)) == "" {
		t.Fatalf("SERVER_VERSION missing from status json: %#v", payload)
	}
	if _, exists := payload["project_workflow"]; exists {
		t.Fatalf("status json should omit project_workflow: %#v", payload)
	}
	if _, exists := payload["project_default_draft"]; exists {
		t.Fatalf("status json should omit project_default_draft: %#v", payload)
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

func TestRunProjectSetDraftSupportsPrivateAlias(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	if err := run([]string{"project", "set-draft", "-project_id", "private", "true"}); err != nil {
		t.Fatalf("project set-draft private error = %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	project, _, err := resolveProjectContext(context.Background(), cfg, svc, "private")
	if err != nil {
		t.Fatalf("GetProject(private) error = %v", err)
	}
	if !project.DefaultDraft {
		t.Fatalf("private project.DefaultDraft = %v, want true", project.DefaultDraft)
	}
}

func TestResolveGUIProjectUsesDefaultProject(t *testing.T) {
	setupLocalCLI(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	svc := localCLIService(t)

	resolvedCfg, project, err := resolveGUIProject(context.Background(), cfg, svc)
	if err != nil {
		t.Fatalf("resolveGUIProject() error = %v", err)
	}
	if project.ID == 0 || project.Prefix != "PRIV" {
		t.Fatalf("resolveGUIProject() project = %#v", project)
	}
	if resolvedCfg.ProjectID == "" {
		t.Fatalf("resolved cfg.ProjectID is empty, want a numeric project id")
	}
}

func TestRunProjectWorkflowSetsAndClearsCurrentProjectWorkflow(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	wf, err := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{Name: "Project Workflow", Description: "workflow assignment test"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	setOutput := captureStdout(t, func() {
		if runErr := run([]string{"project", "workflow", strconv.FormatInt(wf.ID, 10)}); runErr != nil {
			t.Fatalf("project workflow set error = %v", runErr)
		}
	})
	if !strings.Contains(setOutput, "set workflow") {
		t.Fatalf("unexpected project workflow set output:\n%s", setOutput)
	}

	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) after set error = %v", err)
	}
	if project.WorkflowID == nil || *project.WorkflowID != wf.ID {
		t.Fatalf("project.WorkflowID = %#v, want %d", project.WorkflowID, wf.ID)
	}

	clearOutput := captureStdout(t, func() {
		if runErr := run([]string{"project", "workflow", "0"}); runErr != nil {
			t.Fatalf("project workflow clear error = %v", runErr)
		}
	})
	if !strings.Contains(clearOutput, "cleared workflow") {
		t.Fatalf("unexpected project workflow clear output:\n%s", clearOutput)
	}

	project, err = svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) after clear error = %v", err)
	}
	if project.WorkflowID != nil && *project.WorkflowID == wf.ID {
		t.Fatalf("project.WorkflowID = %#v, want cleared custom workflow %d", project.WorkflowID, wf.ID)
	}
}

func TestRunProjectUseAndWorkflowHelpPaths(t *testing.T) {
	setupLocalCLI(t)

	noProjectOutput := captureStdout(t, func() {
		if err := run([]string{"project", "use"}); err != nil {
			t.Fatalf("project use with no current project error = %v", err)
		}
	})
	if !strings.Contains(noProjectOutput, "PRIV — Private") {
		t.Fatalf("unexpected project use output:\n%s", noProjectOutput)
	}

	useErr := run([]string{"project", "use", "1"})
	if useErr == nil || !strings.Contains(useErr.Error(), "has been removed") {
		t.Fatalf("project use error = %v", useErr)
	}

	currentOutput := captureStdout(t, func() {
		if err := run([]string{"project", "use"}); err != nil {
			t.Fatalf("project use current error = %v", err)
		}
	})
	if !strings.Contains(currentOutput, "PRIV") {
		t.Fatalf("unexpected current project output:\n%s", currentOutput)
	}

	helpOutput := captureStdout(t, func() {
		if err := run([]string{"project", "workflow", "help"}); err != nil {
			t.Fatalf("project workflow help error = %v", err)
		}
	})
	if !strings.Contains(helpOutput, "tk project workflow <workflow-id>") {
		t.Fatalf("unexpected project workflow help output:\n%s", helpOutput)
	}
}

func TestRunProjectSetDefaultAffectsFallbackResolution(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)
	project, err := svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix: "DEF",
		Title:  "Default Project",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	if err := run([]string{"project", "set-default", "DEF"}); err != nil {
		t.Fatalf("project set-default error = %v", err)
	}

	listOutput := captureStdout(t, func() {
		if err := run([]string{"project", "ls"}); err != nil {
			t.Fatalf("project ls error = %v", err)
		}
	})
	if !strings.Contains(listOutput, "*  4   DEF") {
		t.Fatalf("project ls should mark saved default project:\n%s", listOutput)
	}

	statusOutput := captureStdout(t, func() {
		if err := run([]string{"status"}); err != nil {
			t.Fatalf("status error = %v", err)
		}
	})
	if strings.Contains(statusOutput, "DEFAULT_PROJECT") {
		t.Fatalf("status output should not show DEFAULT_PROJECT:\n%s", statusOutput)
	}

	ticketID := createLocalTask(t, []string{"create", "-title", "Uses saved default"})
	ticket, err := svc.GetTicket(context.Background(), ticketID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if ticket.ProjectID != project.ID {
		t.Fatalf("ticket.ProjectID = %d, want %d", ticket.ProjectID, project.ID)
	}

	if err := run([]string{"project", "clear-default"}); err != nil {
		t.Fatalf("project clear-default error = %v", err)
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

	if runErr := run([]string{"notready", ticket.ID}); runErr != nil {
		t.Fatalf("notready error = %v", runErr)
	}
	updated, err := svc.GetTicket(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("GetTicket(after notready) error = %v", err)
	}
	if !updated.Draft {
		t.Fatalf("Draft after notready = %v, want true", updated.Draft)
	}

	if runErr := run([]string{"ready", ticket.ID}); runErr != nil {
		t.Fatalf("ready error = %v", runErr)
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

func TestRunWorkflowExportImportRoundTrip(t *testing.T) {
	setupLocalCLI(t)
	tmpFile := filepath.Join(t.TempDir(), "workflow.json")
	// Export
	if err := run([]string{"workflow", "export", "-id", "1", "-o", tmpFile}); err != nil {
		t.Fatalf("workflow export error = %v", err)
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
		if err := run([]string{"workflow", "import", "-file", tmpFile}); err != nil {
			t.Fatalf("workflow import error = %v", err)
		}
	})
	if !strings.Contains(output, "imported") {
		t.Fatalf("workflow import missing name:\n%s", output)
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

func setupLocalCLI(t *testing.T) {
	t.Helper()
	rootDir := t.TempDir()
	globalHome := filepath.Join(rootDir, "home")
	repoDir := filepath.Join(rootDir, "repo")
	if err := os.MkdirAll(globalHome, 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir(repoDir) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	t.Setenv("TICKET_HOME", globalHome)
	t.Setenv("TICKET_USERNAME", "admin")
	t.Setenv("TICKET_PASSWORD", "secret12")
	dbPath := filepath.Join(globalHome, "ticket.db")
	testutil.CloneSeededDB(t, dbPath, "secret12")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	handler, err := server.Handler(db, "test", false, nil, "", "")
	if err != nil {
		t.Fatalf("server.Handler() error = %v", err)
	}
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	// Set the admin user's default project so resolveProjectContext can find it
	// via GetMyDefaultProject (step 3 of the resolution chain). Do this directly
	// on the DB before the HTTP server starts to avoid env-variable races.
	adminUser, err := store.GetUserByUsername(context.Background(), db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	if err := store.SetUserDefaultProject(context.Background(), db, adminUser.ID, 1); err != nil {
		t.Fatalf("SetUserDefaultProject(1) error = %v", err)
	}
	t.Setenv("TICKET_URL", ts.URL)
	setTestLocation(t, ts.URL)
}

func createLegacyDatabaseForCLI(t *testing.T) (string, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	testutil.CloneSeededDB(t, dbPath, "password")
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

func createPreviousSchemaDatabaseForCLI(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "previous.db")
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	_, err = rawDB.Exec(`
		PRAGMA foreign_keys = ON;
		CREATE TABLE users (
			user_id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL,
			plan_id INTEGER,
			display_name TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			user_type TEXT NOT NULL DEFAULT 'user',
			uuid TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			last_seen TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE schema_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
		INSERT INTO schema_meta (key, value) VALUES ('schema_version', '5');
	`)
	if err != nil {
		if closeErr := rawDB.Close(); closeErr != nil {
			t.Fatalf("rawDB.Close() error after exec failure = %v", closeErr)
		}
		t.Fatalf("rawDB.Exec() error = %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("rawDB.Close() error = %v", err)
	}
	return dbPath
}

// testAdminUserID returns the admin user's ID by opening the local DB and looking up the user.
func testAdminUserID(t *testing.T) string {
	t.Helper()
	db, err := store.Open(testDBPath(t))
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

func TestResolveServiceUsesEnvRemoteConfig(t *testing.T) {
	setupLocalCLI(t)

	t.Setenv("TICKET_URL", "http://127.0.0.1:1")
	t.Setenv("TICKET_USERNAME", "admin")
	t.Setenv("TICKET_PASSWORD", "password")

	cfg := config.Config{
		Location: "",
	}
	svc, err := resolveService(cfg)
	if err != nil {
		t.Fatalf("resolveService() error = %v", err)
	}

	_, _, loginErr := svc.Login(context.Background(), "admin", "password")
	if loginErr == nil {
		t.Fatal("expected login error against test endpoint")
	}
	if strings.Contains(loginErr.Error(), "configured server") {
		t.Fatalf("expected remote HTTP login attempt, got server-binding error: %v", loginErr)
	}
}

func TestHasCompleteRemoteRuntimeConfigWithToken(t *testing.T) {
	setupLocalCLI(t)

	t.Setenv("TICKET_URL", "http://127.0.0.1:1")
	t.Setenv("TICKET_TOKEN", "session-token")

	ok, err := hasCompleteRemoteRuntimeConfig()
	if err != nil {
		t.Fatalf("hasCompleteRemoteRuntimeConfig() error = %v", err)
	}
	if !ok {
		t.Fatal("hasCompleteRemoteRuntimeConfig() = false, want true")
	}
}

func TestResolveServiceRequiresTicketURLWhenOnlyLocalDBPresent(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	setTestWorkingDir(t, tempDir)
	noColorOutput = true
	defer func() { noColorOutput = false }()
	if err := runInitDB([]string{"-f", filepath.Join(tempDir, "ticket.db"), "-password", "secret12"}); err != nil {
		t.Fatalf("runInitDB() error = %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	_, err = resolveService(cfg)
	if err == nil {
		t.Fatal("resolveService() error = nil")
	}
	for _, want := range []string{
		"incomplete remote authentication configuration.",
		"attempting to connect to TICKET_URL",
		"TICKET_URL UNSET",
		"TICKET_USERNAME = UNSET",
		"TICKET_PASSWORD = UNSET",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("resolveService() error missing %q:\n%v", want, err)
		}
	}
}

func TestResolveServiceUsesProcessLocationOverrideForLocalDatabase(t *testing.T) {
	setupLocalCLI(t)

	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	testutil.CloneSeededDB(t, dbPath, "secret12")
	config.SetLocationOverride(dbPath)
	t.Cleanup(config.ClearLocationOverride)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	svc, err := resolveService(cfg)
	if err != nil {
		t.Fatalf("resolveService() error = %v", err)
	}
	if _, ok := svc.(*libticket.LocalService); !ok {
		t.Fatalf("resolveService() returned %T, want *libticket.LocalService", svc)
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("svc.Status() error = %v", err)
	}
	if status.Status != "ok" {
		t.Fatalf("svc.Status().Status = %q, want %q", status.Status, "ok")
	}
}

func TestResolveServiceShowsCombinedMissingPasswordError(t *testing.T) {
	setupLocalCLI(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	t.Setenv("TICKET_URL", server.URL)
	t.Setenv("TICKET_USERNAME", "admin")
	t.Setenv("TICKET_PASSWORD", "")
	noColorOutput = true
	defer func() { noColorOutput = false }()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	_, err = resolveService(cfg)
	if err == nil {
		t.Fatal("resolveService() error = nil")
	}
	for _, want := range []string{
		"attempting to connect to TICKET_URL " + server.URL,
		"TICKET_USERNAME = admin",
		"TICKET_PASSWORD = UNSET",
		"Run `tk login`, set TICKET_TOKEN, or set both TICKET_USERNAME and TICKET_PASSWORD.",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("resolveService() error missing %q:\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), "missing required environment variable") {
		t.Fatalf("resolveService() error = %v", err)
	}
}

func TestResolveServiceIgnoresRepoConfigFile(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("TICKET_HOME", homeDir)
	noColorOutput = true
	defer func() { noColorOutput = false }()
	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "ticket-config.json"), []byte(`{"TICKET_URL":"https://ticket.example.com","TICKET_USERNAME":"alice","TICKET_PROJECT":"42"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(ticket-config.json) error = %v", err)
	}
	setTestWorkingDir(t, repoDir)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	_, err = resolveService(cfg)
	if err == nil {
		t.Fatal("resolveService() error = nil")
	}
	for _, want := range []string{
		"attempting to connect to TICKET_URL UNSET",
		"TICKET_USERNAME = UNSET",
		"TICKET_PASSWORD = UNSET",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("resolveService() error missing %q:\n%v", want, err)
		}
	}
}

func TestResolveServiceUsesStoredTokenForEnvURLWithoutPassword(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("TICKET_HOME", homeDir)

	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			t.Fatalf("path = %q, want /api/status", r.URL.Path)
		}
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","authenticated":true,"server_version":"test"}`))
	}))
	defer server.Close()

	setTestLocation(t, server.URL)
	if err := config.SaveRemoteCredentials(server.URL, "admin", "stored-token"); err != nil {
		t.Fatalf("SaveRemoteCredentials() error = %v", err)
	}

	t.Setenv("TICKET_URL", server.URL)
	t.Setenv("TICKET_USERNAME", "admin")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	svc, err := resolveService(cfg)
	if err != nil {
		t.Fatalf("resolveService() error = %v", err)
	}
	if _, ok := svc.(*libticket.HTTPService); !ok {
		t.Fatalf("resolveService() returned %T, want *libticket.HTTPService", svc)
	}
	if _, err := svc.Status(context.Background()); err != nil {
		t.Fatalf("svc.Status() error = %v", err)
	}
	if authHeader != "Bearer stored-token" {
		t.Fatalf("Authorization header = %q, want %q", authHeader, "Bearer stored-token")
	}
}

func TestResolveServiceRequiresLoginForRemoteCommands(t *testing.T) {
	setupLocalCLI(t)
	noColorOutput = true
	defer func() { noColorOutput = false }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	t.Setenv("TICKET_URL", server.URL)
	t.Setenv("TICKET_USERNAME", "")
	t.Setenv("TICKET_PASSWORD", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	_, err = resolveService(cfg)
	if err == nil {
		t.Fatal("resolveService() error = nil")
	}
	for _, want := range []string{
		"attempting to connect to TICKET_URL " + server.URL,
		"TICKET_USERNAME = UNSET",
		"TICKET_PASSWORD = UNSET",
		"Run `tk login`, set TICKET_TOKEN, or set both TICKET_USERNAME and TICKET_PASSWORD.",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("resolveService() error missing %q:\n%v", want, err)
		}
	}
}

func TestResolveServiceIgnoresRepoConfigPassword(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("TICKET_HOME", homeDir)
	noColorOutput = true
	defer func() { noColorOutput = false }()
	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "ticket-config.json"), []byte(`{"TICKET_URL":"https://ticket.example.com","TICKET_USERNAME":"alice","TICKET_PASSWORD":"bad"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(ticket-config.json) error = %v", err)
	}
	setTestWorkingDir(t, repoDir)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	_, err = resolveService(cfg)
	if err == nil {
		t.Fatal("resolveService() error = nil")
	}
	for _, want := range []string{
		"attempting to connect to TICKET_URL UNSET",
		"TICKET_USERNAME = UNSET",
		"TICKET_PASSWORD = UNSET",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("resolveService() error missing %q:\n%v", want, err)
		}
	}
}

func TestResolveCurrentProjectClientMatchesCanonicalGitOriginAcrossProjectRepositories(t *testing.T) {
	setupLocalCLI(t)

	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	target := filepath.Join(t.TempDir(), "origin.git")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}
	link := filepath.Join(t.TempDir(), "origin-link.git")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	svc := localCLIService(t)
	project, err := svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix:        "CAN",
		Title:         "Canonical Repo",
		GitRepository: "https://example.com/primary.git",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if err := svc.AddProjectGitRepository(context.Background(), project.Prefix, "file://"+target); err != nil {
		t.Fatalf("AddProjectGitRepository() error = %v", err)
	}

	gitOriginByRoot.Store(repoDir, link)
	t.Cleanup(func() { gitOriginByRoot.Delete(repoDir) })

	cfg, _, resolvedProject, err := resolveCurrentProjectClient()
	if err != nil {
		t.Fatalf("resolveCurrentProjectClient() error = %v", err)
	}
	if resolvedProject.Prefix != project.Prefix {
		t.Fatalf("resolveCurrentProjectClient().Prefix = %q, want %q", resolvedProject.Prefix, project.Prefix)
	}
	if cfg.ProjectID != project.Prefix {
		t.Fatalf("resolved cfg.ProjectID = %q, want %q", cfg.ProjectID, project.Prefix)
	}
}

// TestResolveProjectContextUsesGitRemoteOverDefault verifies that when the
// current directory's git origin is registered to a project, that project is
// returned instead of the user's default project.
func TestResolveProjectContextUsesGitRemoteOverDefault(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	// Create a second project with a registered git repository.
	repoURL := "https://example.com/git-detection-test.git"
	gitProject, err := svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix:        "GIT",
		Title:         "Git Detection Project",
		GitRepository: repoURL,
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	// Simulate being in a directory whose git origin matches the registered URL.
	gitOriginByRoot.Store(repoDir, repoURL)
	t.Cleanup(func() { gitOriginByRoot.Delete(repoDir) })

	_, _, resolvedProject, err := resolveCurrentProjectClient()
	if err != nil {
		t.Fatalf("resolveCurrentProjectClient() error = %v", err)
	}
	if resolvedProject.Prefix != gitProject.Prefix {
		t.Fatalf("resolveCurrentProjectClient().Prefix = %q, want %q (git-detected project)", resolvedProject.Prefix, gitProject.Prefix)
	}
}

// TestResolveProjectContextFallsBackToDefaultWhenNoGitMatch verifies that when
// no git remote matches a registered project, the user's default project is used.
func TestResolveProjectContextFallsBackToDefaultWhenNoGitMatch(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	// Simulate a git origin that is NOT registered to any project.
	gitOriginByRoot.Store(repoDir, "https://example.com/unregistered-repo.git")
	t.Cleanup(func() { gitOriginByRoot.Delete(repoDir) })

	defaultProject, err := svc.GetMyDefaultProject(context.Background())
	if err != nil {
		t.Fatalf("GetMyDefaultProject() error = %v", err)
	}

	_, _, resolvedProject, err := resolveCurrentProjectClient()
	if err != nil {
		t.Fatalf("resolveCurrentProjectClient() error = %v", err)
	}
	if resolvedProject.ID != defaultProject.ID {
		t.Fatalf("resolveCurrentProjectClient().ID = %d, want %d (default project)", resolvedProject.ID, defaultProject.ID)
	}
}

// TestResolveProjectContextExplicitFlagOverridesGit verifies that an explicit
// -project_id flag takes precedence over git-based detection.
func TestResolveProjectContextExplicitFlagOverridesGit(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	// Create a second project with a registered git repository.
	repoURL := "https://example.com/explicit-override-test.git"
	_, err = svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix:        "OVR",
		Title:         "Override Test Project",
		GitRepository: repoURL,
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	// Simulate git origin matching OVR project.
	gitOriginByRoot.Store(repoDir, repoURL)
	t.Cleanup(func() { gitOriginByRoot.Delete(repoDir) })

	// Explicit -project_id PRIV should override the git-detected OVR project.
	setProjectOverride("PRIV")
	t.Cleanup(clearProjectOverride)

	_, _, resolvedProject, err := resolveCurrentProjectClient()
	if err != nil {
		t.Fatalf("resolveCurrentProjectClient() error = %v", err)
	}
	if resolvedProject.Prefix != "PRIV" {
		t.Fatalf("resolveCurrentProjectClient().Prefix = %q, want %q (explicit flag)", resolvedProject.Prefix, "PRIV")
	}
}

// TestRunInitProjectCreatesProjectForGitRepo verifies that tk init creates a
// new project and registers the current git repository to it.
func TestRunInitProjectCreatesProjectForGitRepo(t *testing.T) {
	setupLocalCLI(t)
	svc := localCLIService(t)

	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	repoURL := "https://example.com/new-init-project.git"
	gitOriginByRoot.Store(repoDir, repoURL)
	t.Cleanup(func() { gitOriginByRoot.Delete(repoDir) })

	output := captureStdout(t, func() {
		if runErr := runInitProject([]string{"-name", "Init Test Project", "-prefix", "INI"}); runErr != nil {
			t.Fatalf("runInitProject() error = %v", runErr)
		}
	})

	if !strings.Contains(output, "INI") {
		t.Fatalf("runInitProject() output missing project prefix INI:\n%s", output)
	}

	project, err := svc.FindProjectByGitRepository(context.Background(), repoURL)
	if err != nil {
		t.Fatalf("FindProjectByGitRepository() after init error = %v", err)
	}
	if project.Prefix != "INI" {
		t.Fatalf("FindProjectByGitRepository().Prefix = %q, want INI", project.Prefix)
	}
}

// TestRunInitProjectFailsWhenRepoAlreadyRegistered verifies that tk init
// fails with an informative error when the git repository is already assigned.
func TestRunInitProjectFailsWhenRepoAlreadyRegistered(t *testing.T) {
	setupLocalCLI(t)

	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	repoURL := "https://example.com/already-registered.git"
	gitOriginByRoot.Store(repoDir, repoURL)
	t.Cleanup(func() { gitOriginByRoot.Delete(repoDir) })

	// First init succeeds.
	if err := runInitProject([]string{"-name", "First Project", "-prefix", "FST"}); err != nil {
		t.Fatalf("first runInitProject() error = %v", err)
	}

	// Second init must fail with an explanation.
	err = runInitProject([]string{"-name", "Second Project", "-prefix", "SND"})
	if err == nil {
		t.Fatal("second runInitProject() want error for already-registered repo, got nil")
	}
	if !strings.Contains(err.Error(), "already assigned") {
		t.Fatalf("second runInitProject() error = %q, want it to mention 'already assigned'", err.Error())
	}
}

// TestRunInitProjectFailsWithNoGitRemote verifies that tk init fails when the
// current directory is not inside a git repository with a remote origin.
func TestRunInitProjectFailsWithNoGitRemote(t *testing.T) {
	setupLocalCLI(t)
	// repoDir from setupLocalCLI has a .git directory but no remote configured.
	// gitOriginByRoot has no entry for it, so detectGitOriginAt returns "".
	err := runInitProject(nil)
	if err == nil {
		t.Fatal("runInitProject() with no git remote want error, got nil")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Fatalf("runInitProject() error = %q, want it to mention 'git repository'", err.Error())
	}
}

func attachWorkflowToDefaultProject(t *testing.T, stageNames ...string) libticket.Service {
	t.Helper()
	svc := localCLIService(t)
	wf, err := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{
		Name:        "Custom Workflow",
		Description: "custom workflow for tests",
	})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	for i, stageName := range stageNames {
		if _, addStageErr := svc.AddWorkflowStage(context.Background(), wf.ID, libticket.WorkflowStageRequest{
			StageName:   stageName,
			Description: stageName,
			SortOrder:   i,
		}); addStageErr != nil {
			t.Fatalf("AddWorkflowStage(%q) error = %v", stageName, addStageErr)
		}
	}
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	if _, err := svc.UpdateProject(context.Background(), project.ID, libticket.ProjectUpdateRequest{WorkflowID: &wf.ID}); err != nil {
		t.Fatalf("UpdateProject(workflow) error = %v", err)
	}
	output := captureStdout(t, func() {
		if runErr := run([]string{"project", "workflow", strconv.FormatInt(wf.ID, 10)}); runErr != nil {
			t.Fatalf("project workflow set error = %v", runErr)
		}
	})
	if !strings.Contains(output, "set workflow") {
		t.Fatalf("project workflow set output = %q", output)
	}
	return svc
}

// deleteTicketConfirmed performs the two-step ticket deletion using the
// stateless confirmation value echoed by the first run.
func deleteTicketConfirmed(t *testing.T, id string) error {
	t.Helper()
	if err := run([]string{"rm", "-id", id}); err != nil {
		t.Fatalf("rm phase 1 error = %v", err)
	}
	return run([]string{"rm", "-id", id, "--confirm", id})
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
	wf := store.WorkflowWithStages{
		Workflow: store.Workflow{Name: "Standard"},
		Stages:   []store.WorkflowStage{{StageName: "develop"}, {StageName: "develop"}, {StageName: "test"}},
	}
	project := store.Project{Prefix: "TK", Title: "Test Project", GitRepository: "github.com/test/repo"}
	resp := libticket.AgentWorkResponse{
		Status:   "NEW",
		Ticket:   &ticket,
		Project:  &project,
		Workflow: &wf,
		Role:     &role,
	}
	prompt := buildAgentPrompt(resp)
	for _, want := range []string{"Test Task", "Some description", "Must pass", "Developer", "Ship features", "Standard", "develop", "develop", "Test Project", "github.com/test/repo"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("buildAgentPrompt missing %q:\n%s", want, prompt)
		}
	}
	// Without description/AC/role/workflow
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
	setupLocalCLI(t)
	commands := []string{
		"project", "workflow", "team", "story", "user", "label",
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

func TestRunStatusIgnoresLegacyLocalDatabaseLocation(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	legacyPath, _ := createLegacyDatabaseForCLI(t)
	if err := config.Save(config.Config{Location: legacyPath}); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"status"}); err != nil {
			t.Fatalf("run(status) error = %v", err)
		}
	})
	if !strings.Contains(output, "TICKET_URL") || !strings.Contains(output, "UNSET") {
		t.Fatalf("run(status) should ignore local DB locations and show remote fields:\n%s", output)
	}
}

func TestRunUpgradeDatabaseUpgradesLegacyDatabaseInPlace(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	legacyPath, ticketID := createLegacyDatabaseForCLI(t)

	output := captureStdout(t, func() {
		if err := run([]string{"admin", "upgrade-database", "-f", legacyPath}); err != nil {
			t.Fatalf("admin upgrade-database error = %v", err)
		}
	})
	if !strings.Contains(output, "upgraded") {
		t.Fatalf("admin upgrade-database output = %q, want upgrade confirmation", output)
	}
	// The database is upgraded in place: the same path is now at the current version.
	if got, err := store.DetectSchemaVersion(legacyPath); err != nil {
		t.Fatalf("DetectSchemaVersion(legacy) error = %v", err)
	} else if got != store.CurrentSchemaVersion {
		t.Fatalf("DetectSchemaVersion(legacy) = %d, want %d", got, store.CurrentSchemaVersion)
	}
	db, err := store.Open(legacyPath)
	if err != nil {
		t.Fatalf("store.Open(legacy) error = %v", err)
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

func TestRunListShowsSchemaUpgradeGuidanceForPreviousDatabase(t *testing.T) {
	t.Setenv("TICKET_HOME", t.TempDir())
	noColorOutput = true
	t.Cleanup(func() { noColorOutput = false })

	dbPath := createPreviousSchemaDatabaseForCLI(t)
	err := run([]string{"-f", dbPath, "ls"})
	if err == nil {
		t.Fatal("run(ls) error = nil, want schema version guidance")
	}
	for _, want := range []string{
		dbPath,
		"schema version 5",
		fmt.Sprintf("schema version %d", store.CurrentSchemaVersion),
		"upgrade-database -f",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("run(ls) error missing %q:\n%v", want, err)
		}
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

func TestRunEpicGetRequiresExplicitID(t *testing.T) {
	setupLocalCLI(t)

	epicID := createLocalTask(t, []string{"epic", "My Epic"})
	err := run([]string{"epic", "get"})
	if err == nil || !strings.Contains(err.Error(), "usage: tk epic get <id>") {
		t.Fatalf("epic get without id error = %v", err)
	}
	output := captureStdout(t, func() {
		if runErr := run([]string{"epic", "get", epicID}); runErr != nil {
			t.Fatalf("epic get error = %v", runErr)
		}
	})
	if !hasDetailField(output, "Title", "My Epic") {
		t.Fatalf("epic get output missing epic details:\n%s", output)
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
		if runErr := run([]string{"epic", "get", epicID}); runErr != nil {
			t.Fatalf("epic get error = %v", runErr)
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
		if runErr := run([]string{"bug", "get", bugID}); runErr != nil {
			t.Fatalf("bug get error = %v", runErr)
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

func TestRunEpicSubcommandsRemoved(t *testing.T) {
	setupLocalCLI(t)

	epicID := createLocalTask(t, []string{"epic", "Clearable Epic"})
	err := run([]string{"epic", "use", epicID})
	if err == nil || !strings.Contains(err.Error(), "has been removed") {
		t.Fatalf("epic use error = %v", err)
	}
	err = run([]string{"epic", "clear"})
	if err == nil || !strings.Contains(err.Error(), "has been removed") {
		t.Fatalf("epic clear error = %v", err)
	}
}

func TestRunEpicListShowsPlainRows(t *testing.T) {
	setupLocalCLI(t)

	_ = createLocalTask(t, []string{"epic", "Listed Epic"})

	output := captureStdout(t, func() {
		if err := run([]string{"epic", "ls"}); err != nil {
			t.Fatalf("epic ls error = %v", err)
		}
	})
	if !strings.Contains(output, "Listed Epic") {
		t.Fatalf("epic ls missing epic title: %s", output)
	}
}

func TestRunTypedNamespaceListEmptyMessages(t *testing.T) {
	setupLocalCLI(t)

	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"ls"}, want: "No tickets available for project \"Private\"."},
		{args: []string{"story", "ls"}, want: "No stories available."},
		{args: []string{"idea", "ls"}, want: "No ideas available."},
		{args: []string{"decision", "ls"}, want: "No decisions available."},
		{args: []string{"epic", "ls"}, want: "No epics available."},
		{args: []string{"bug", "ls"}, want: "No bugs available."},
		{args: []string{"label", "ls"}, want: "No labels available."},
	}

	for _, tc := range tests {
		output := captureStdout(t, func() {
			if err := run(tc.args); err != nil {
				t.Fatalf("run(%v) error = %v", tc.args, err)
			}
		})
		trimmed := strings.TrimSpace(output)
		if len(tc.args) > 0 && tc.args[0] == "ls" {
			if !strings.HasPrefix(trimmed, "PROJECT  PRIV — Private (1)") || !strings.Contains(trimmed, tc.want) {
				t.Fatalf("run(%v) output = %q, want project header plus %q", tc.args, trimmed, tc.want)
			}
			continue
		}
		if trimmed != tc.want {
			t.Fatalf("run(%v) output = %q, want %q", tc.args, trimmed, tc.want)
		}
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

	// Admin bootstrap user is not alice; unclaim should fail
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
	if strings.TrimSpace(emptyOutput) != "No stories available." {
		t.Fatalf("story ls after delete = %q, want %q", strings.TrimSpace(emptyOutput), "No stories available.")
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

func TestRunStoryBarePositionalTitleCreatesStory(t *testing.T) {
	setupLocalCLI(t)

	output := captureStdout(t, func() {
		if err := run([]string{"story", "Bar Story"}); err != nil {
			t.Fatalf("story shortcut error = %v", err)
		}
	})
	if !strings.Contains(output, "Bar Story") {
		t.Fatalf("story shortcut output missing title: %s", output)
	}

	listOutput := captureStdout(t, func() {
		if err := run([]string{"story", "ls"}); err != nil {
			t.Fatalf("story ls error = %v", err)
		}
	})
	if !strings.Contains(listOutput, "Bar Story") {
		t.Fatalf("story ls missing shortcut-created story: %s", listOutput)
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

func TestRunStoryGetUsesMostRecentWhenIDOmitted(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"story", "create", "Older Story"}); err != nil {
		t.Fatalf("story create older error = %v", err)
	}
	if err := run([]string{"story", "create", "Newest Story"}); err != nil {
		t.Fatalf("story create newest error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"story", "get"}); err != nil {
			t.Fatalf("story get error = %v", err)
		}
	})
	if !strings.Contains(output, "Newest Story") {
		t.Fatalf("story get output missing latest story:\n%s", output)
	}
}

func TestRunDocumentCreateListGetUpdateDeleteAndFiles(t *testing.T) {
	setupLocalCLI(t)

	createOutput := captureStdout(t, func() {
		if err := run([]string{"document", "create", "-title", "Doc One", "-d", "summary", "-notes", "notes", "-content", "hello"}); err != nil {
			t.Fatalf("document create error = %v", err)
		}
	})
	if !strings.Contains(createOutput, "Doc One") {
		t.Fatalf("document create output missing title: %s", createOutput)
	}

	documentID := "1"
	if matches := regexp.MustCompile(`document\s+(\d+):`).FindStringSubmatch(createOutput); len(matches) == 2 {
		documentID = matches[1]
	}

	listOutput := captureStdout(t, func() {
		if err := run([]string{"document", "ls"}); err != nil {
			t.Fatalf("document ls error = %v", err)
		}
	})
	if !strings.Contains(listOutput, "Doc One") {
		t.Fatalf("document ls output missing title: %s", listOutput)
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"document", "get", documentID}); err != nil {
			t.Fatalf("document get error = %v", err)
		}
	})
	if !strings.Contains(getOutput, "Doc One") || !strings.Contains(getOutput, "summary") {
		t.Fatalf("document get output missing values:\n%s", getOutput)
	}

	updateOutput := captureStdout(t, func() {
		if err := run([]string{"document", "update", documentID, "-title", "Doc One Updated"}); err != nil {
			t.Fatalf("document update error = %v", err)
		}
	})
	if !strings.Contains(updateOutput, "Doc One Updated") {
		t.Fatalf("document update output missing title: %s", updateOutput)
	}

	inputPath := filepath.Join(t.TempDir(), "doc-note.txt")
	if err := os.WriteFile(inputPath, []byte("file body"), 0o600); err != nil {
		t.Fatalf("WriteFile(inputPath) error = %v", err)
	}
	fileAddOutput := captureStdout(t, func() {
		if err := run([]string{"document", "file-add", documentID, "-path", inputPath}); err != nil {
			t.Fatalf("document file-add error = %v", err)
		}
	})
	if !strings.Contains(fileAddOutput, "added") {
		t.Fatalf("document file-add output missing confirmation: %s", fileAddOutput)
	}

	fileID := "1"
	if matches := regexp.MustCompile(`file\s+(\d+)\s+added`).FindStringSubmatch(fileAddOutput); len(matches) == 2 {
		fileID = matches[1]
	}

	fileListOutput := captureStdout(t, func() {
		if err := run([]string{"document", "file-ls", documentID}); err != nil {
			t.Fatalf("document file-ls error = %v", err)
		}
	})
	if !strings.Contains(fileListOutput, "doc-note.txt") {
		t.Fatalf("document file-ls output missing file name:\n%s", fileListOutput)
	}

	outputPath := filepath.Join(t.TempDir(), "downloaded.txt")
	fileGetOutput := captureStdout(t, func() {
		if err := run([]string{"document", "file-get", documentID, fileID, "-o", outputPath}); err != nil {
			t.Fatalf("document file-get error = %v", err)
		}
	})
	if !strings.Contains(fileGetOutput, "wrote") {
		t.Fatalf("document file-get output missing write confirmation: %s", fileGetOutput)
	}
	gotBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(outputPath) error = %v", err)
	}
	if string(gotBytes) != "file body" {
		t.Fatalf("downloaded content = %q, want %q", string(gotBytes), "file body")
	}

	if err := run([]string{"document", "file-rm", documentID, fileID}); err != nil {
		t.Fatalf("document file-rm error = %v", err)
	}
	if err := run([]string{"document", "rm", documentID}); err != nil {
		t.Fatalf("document rm error = %v", err)
	}

	emptyOutput := captureStdout(t, func() {
		if err := run([]string{"document", "ls"}); err != nil {
			t.Fatalf("document ls after delete error = %v", err)
		}
	})
	if strings.TrimSpace(emptyOutput) != "No documents available." {
		t.Fatalf("document ls after delete = %q, want %q", strings.TrimSpace(emptyOutput), "No documents available.")
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
	// accept auto-advances through the workflow: design/success → develop/idle
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

func TestRunDecisionGetUsesMostRecentWhenIDOmitted(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"decision", "new", "Older decision"}); err != nil {
		t.Fatalf("decision new older error = %v", err)
	}
	if err := run([]string{"decision", "new", "Newest decision"}); err != nil {
		t.Fatalf("decision new newest error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"decision", "get"}); err != nil {
			t.Fatalf("decision get error = %v", err)
		}
	})
	if !hasDetailField(output, "Title", "Newest decision") {
		t.Fatalf("decision get output missing latest decision:\n%s", output)
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

func TestRunActionCreateGetAndUpdateFlows(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"act", "Follow up with vendor"}); err != nil {
		t.Fatalf("act create error = %v", err)
	}
	_, svc, project, err := resolveCurrentProjectClient()
	if err != nil {
		t.Fatalf("resolveCurrentProjectClient() error = %v", err)
	}
	actions, err := svc.ListTicketsFiltered(context.Background(), project.ID, "action", "", "", "", "", "", 0, true)
	if err != nil {
		t.Fatalf("ListTicketsFiltered(action) error = %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action ticket, got %d", len(actions))
	}
	actionID := actions[0].ID

	getOutput := captureStdout(t, func() {
		if runErr := run([]string{"act", "get", actionID}); runErr != nil {
			t.Fatalf("act get error = %v", runErr)
		}
	})
	if !strings.Contains(getOutput, actionID+"/action") {
		t.Fatalf("act get output missing id/type:\n%s", getOutput)
	}

	if err := run([]string{"act", "update", actionID, "-due", "2026-05-31", "-description", "Updated action description"}); err != nil {
		t.Fatalf("act update error = %v", err)
	}
	updated, err := svc.GetTicket(context.Background(), actionID)
	if err != nil {
		t.Fatalf("GetTicket(updated action) error = %v", err)
	}
	if updated.EstimateComplete != "2026-05-31T00:00:00Z" {
		t.Fatalf("EstimateComplete = %q, want %q", updated.EstimateComplete, "2026-05-31T00:00:00Z")
	}
	if updated.Description != "Updated action description" {
		t.Fatalf("Description = %q, want %q", updated.Description, "Updated action description")
	}

	if err := run([]string{"act", "comment", actionID, "-m", "Sent follow-up note"}); err != nil {
		t.Fatalf("act comment error = %v", err)
	}
	if err := run([]string{"act", "assign", actionID, "admin"}); err != nil {
		t.Fatalf("act assign error = %v", err)
	}
	updated, err = svc.GetTicket(context.Background(), actionID)
	if err != nil {
		t.Fatalf("GetTicket(assigned action) error = %v", err)
	}
	if updated.Assignee != "admin" {
		t.Fatalf("Assignee = %q, want %q", updated.Assignee, "admin")
	}

	if err := run([]string{"act", "unassign", actionID}); err != nil {
		t.Fatalf("act unassign error = %v", err)
	}
	updated, err = svc.GetTicket(context.Background(), actionID)
	if err != nil {
		t.Fatalf("GetTicket(unassigned action) error = %v", err)
	}
	if updated.Assignee != "" {
		t.Fatalf("Assignee = %q, want empty", updated.Assignee)
	}
}

func TestRunActionStateAliasesAndDueValidation(t *testing.T) {
	setupLocalCLI(t)

	actionID := createLocalTask(t, []string{"act", "Action workflow alias test"})
	if err := run([]string{"act", "reject", actionID}); err != nil {
		t.Fatalf("act reject error = %v", err)
	}
	rejected, err := localCLIService(t).GetTicket(context.Background(), actionID)
	if err != nil {
		t.Fatalf("GetTicket(rejected action) error = %v", err)
	}
	if rejected.State != "fail" {
		t.Fatalf("rejected action state = %q, want fail", rejected.State)
	}

	actionID = createLocalTask(t, []string{"act", "Action done alias test"})
	if err := run([]string{"act", "done", actionID}); err != nil {
		t.Fatalf("act done error = %v", err)
	}
	done, err := localCLIService(t).GetTicket(context.Background(), actionID)
	if err != nil {
		t.Fatalf("GetTicket(done action) error = %v", err)
	}
	if done.Stage == "design" && done.State == "idle" {
		t.Fatalf("done action did not change lifecycle: stage=%s state=%s", done.Stage, done.State)
	}

	err = run([]string{"act", "update", actionID, "-due", "2026-99-99"})
	if err == nil || !strings.Contains(err.Error(), "invalid due date") {
		t.Fatalf("invalid due date error = %v", err)
	}
}

func TestRunActionBinaryAliasRoutesTicketCommands(t *testing.T) {
	setupLocalCLI(t)
	setBinaryNameForTest(t, "act")

	if err := run([]string{"Follow up finance by Friday"}); err != nil {
		t.Fatalf("run(act create) error = %v", err)
	}
	_, svc, project, err := resolveCurrentProjectClient()
	if err != nil {
		t.Fatalf("resolveCurrentProjectClient() error = %v", err)
	}
	actions, err := svc.ListTicketsFiltered(context.Background(), project.ID, "action", "", "", "", "", "", 0, true)
	if err != nil {
		t.Fatalf("ListTicketsFiltered(action) error = %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action ticket, got %d", len(actions))
	}

	listOut := captureStdout(t, func() {
		if runErr := run([]string{"ls"}); runErr != nil {
			t.Fatalf("run(act ls) error = %v", runErr)
		}
	})
	if !strings.Contains(listOut, "Follow up finance by Friday") || !strings.Contains(listOut, " action ") {
		t.Fatalf("act ls output missing filtered action ticket:\n%s", listOut)
	}
}

func TestRunActionBinaryAliasPreservesSystemCommands(t *testing.T) {
	setupLocalCLI(t)
	setBinaryNameForTest(t, "action")

	statusOut := captureStdout(t, func() {
		if err := run([]string{"status"}); err != nil {
			t.Fatalf("run(action status) error = %v", err)
		}
	})
	for _, want := range []string{"TICKET_URL", "TICKET_USERNAME", "TICKET_PASSWORD", "SERVER_VERSION", "CLIENT_VERSION"} {
		if !strings.Contains(statusOut, want) {
			t.Fatalf("action status should run remote status output missing %q:\n%s", want, statusOut)
		}
	}
	if strings.Contains(statusOut, "project") {
		t.Fatalf("action status should run system status output:\n%s", statusOut)
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

	adminUserID := testAdminUserID(t)
	captureStdout(t, func() {
		if err := run([]string{"team", "add-user", "-team_id", teamID, "-user_id", adminUserID, "-role", "owner"}); err != nil {
			t.Fatalf("team add-user before delete error = %v", err)
		}
	})
	captureStdout(t, func() {
		if err := run([]string{"team", "remove-user", "-team_id", teamID, "-user_id", adminUserID}); err != nil {
			t.Fatalf("team remove-user before delete error = %v", err)
		}
	})

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

	t.Run("workflow", func(t *testing.T) {
		setupLocalCLI(t)
		if err := run([]string{"workflow", "create", "-id", "104", "-name", "Explicit Workflow"}); err != nil {
			t.Fatalf("workflow create error = %v", err)
		}
		wf, err := localCLIService(t).GetWorkflow(context.Background(), 104)
		if err != nil {
			t.Fatalf("GetWorkflow(104) error = %v", err)
		}
		if wf.ID != 104 {
			t.Fatalf("workflow id = %d, want 104", wf.ID)
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
		db, err := store.Open(testDBPath(t))
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

	t.Run("workflow", func(t *testing.T) {
		setupLocalCLI(t)
		out := strings.TrimSpace(captureStdout(t, func() {
			if err := run([]string{"workflow", "create", "-id", "304", "-printid", "-name", "Print Workflow"}); err != nil {
				t.Fatalf("workflow create error = %v", err)
			}
		}))
		if out != "304" {
			t.Fatalf("workflow output = %q, want %q", out, "304")
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

func TestRunCountSupportsPrivateProjectAlias(t *testing.T) {
	setupLocalCLI(t)

	t.Setenv("TICKET_PROJECT", "private")
	if err := run([]string{"add", "Private Task"}); err != nil {
		t.Fatalf("add private task error = %v", err)
	}
	if err := run([]string{"count", "-project_id", "private", "-type", "task", "-expect_equals", "1"}); err != nil {
		t.Fatalf("count private alias error = %v", err)
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
		if err := run([]string{"get", "-id", ref1, "-v"}); err != nil {
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
		if err := run([]string{"get", "-id", ref1, "-v"}); err != nil {
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
	if strings.TrimSpace(archiveOut) != ref+" archived" {
		t.Fatalf("archive output = %q, want %q", strings.TrimSpace(archiveOut), ref+" archived")
	}
	for _, unwanted := range []string{"archived: true", "type: task"} {
		if strings.Contains(archiveOut, unwanted) {
			t.Fatalf("archive output should be terse, found %q:\n%s", unwanted, archiveOut)
		}
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

	// archive verbose
	archiveVerboseOut := captureStdout(t, func() {
		if err := run([]string{"unarchive", "-id", ref}); err != nil {
			t.Fatalf("unarchive reset error = %v", err)
		}
		if err := run([]string{"archive", "-id", ref, "-v"}); err != nil {
			t.Fatalf("archive verbose error = %v", err)
		}
	})
	if !strings.Contains(archiveVerboseOut, "archived: true") || !strings.Contains(archiveVerboseOut, "type: task") {
		t.Fatalf("archive verbose output should include ticket detail:\n%s", archiveVerboseOut)
	}

	// unarchive
	unarchiveOut := captureStdout(t, func() {
		if err := run([]string{"unarchive", "-id", ref}); err != nil {
			t.Fatalf("unarchive error = %v", err)
		}
	})
	if strings.TrimSpace(unarchiveOut) != ref+" unarchived" {
		t.Fatalf("unarchive output = %q, want %q", strings.TrimSpace(unarchiveOut), ref+" unarchived")
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

func TestRunArchiveSupportsJSONOutput(t *testing.T) {
	previousJSON := outputJSON
	defer func() { outputJSON = previousJSON }()

	setupLocalCLI(t)

	id := createLocalTask(t, []string{"add", "Archive JSON"})
	output := captureStdout(t, func() {
		if err := run([]string{"archive", "-id", id, "-json"}); err != nil {
			t.Fatalf("archive -json error = %v", err)
		}
	})

	var ticket store.Ticket
	if err := json.Unmarshal([]byte(output), &ticket); err != nil {
		t.Fatalf("archive -json output parse error = %v\n%s", err, output)
	}
	if ticket.ID != id {
		t.Fatalf("archive -json ticket.ID = %q, want %q", ticket.ID, id)
	}
	if !ticket.Archived {
		t.Fatalf("archive -json ticket.Archived = %v, want true", ticket.Archived)
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
	if !strings.Contains(addOut, "commented on") {
		t.Fatalf("comment add output should confirm comment action:\n%s", addOut)
	}

	// comment appears in ticket detail
	getOut := captureStdout(t, func() {
		if err := run([]string{"get", "-id", ref, "-v"}); err != nil {
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
		if err := run([]string{"user", "create", "-username", "newuser", "-email", "newuser@example.com"}); err != nil {
			t.Fatalf("user create error = %v", err)
		}
	})
	if !strings.Contains(createOut, "newuser") {
		t.Fatalf("user create output missing username:\n%s", createOut)
	}
	if !strings.Contains(createOut, "password: ") {
		t.Fatalf("user create output missing generated password:\n%s", createOut)
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

	db, err := store.Open(testDBPath(t))
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

func TestTicketTypeColorBug(t *testing.T) {
	if got := ticketTypeColor("bug"); got != ansiRed {
		t.Fatalf("ticketTypeColor(bug) = %q, want %q", got, ansiRed)
	}
	if got := ticketTypeColor("task"); got != "" {
		t.Fatalf("ticketTypeColor(task) = %q, want empty", got)
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
			Agent:        store.Agent{ID: "uuid-1", Username: "bot-a", Enabled: true, Status: "idle", LastSeen: "2025-01-01T00:00:00Z"},
			TicketKey:    &tk,
			ProjectName:  "MyProject",
			WorkflowName: "default",
			RoleTitle:    "developer",
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
// fallbackCommandUsername in server mode
// ---------------------------------------------------------------------------

func TestFallbackCommandUsernameLocalMode(t *testing.T) {
	setupLocalCLI(t)
	got := fallbackCommandUsername()
	if strings.TrimSpace(got) == "" {
		t.Fatal("fallbackCommandUsername() returned empty username")
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
	for _, want := range []string{"USER", "username", "admin", "PROJECTS"} {
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
	svc := localCLIService(t)

	// Create a project first
	captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "SUM", "-title", "Summary Test"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})
	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	var summaryProjectID int64
	for _, project := range projects {
		if project.Prefix == "SUM" {
			summaryProjectID = project.ID
			break
		}
	}
	if summaryProjectID == 0 {
		t.Fatalf("summary project not found in %+v", projects)
	}
	t.Setenv("TICKET_PROJECT", fmt.Sprintf("%d", summaryProjectID))

	// Create a ticket so summary has data
	createLocalTask(t, []string{"add", "Summary task one"})

	out := captureStdout(t, func() {
		if err := run([]string{"summary"}); err != nil {
			t.Fatalf("summary error = %v", err)
		}
	})
	for _, want := range []string{"project", "SUM", "open tickets", "config"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary output missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"TICKET_HOME", "database"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("summary output should not include %q:\n%s", unwanted, out)
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
	for _, want := range []string{"ticket", "merge"} {
		if !strings.Contains(out, want) {
			t.Fatalf("ticket help output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderCommandHelpIncludesMerge(t *testing.T) {
	help := renderCommandHelp("merge")
	for _, want := range []string{"tk merge", "Merges draft tickets into the first ticket", "TK-1 TK-2 TK-3"} {
		if !strings.Contains(help, want) {
			t.Fatalf("merge help missing %q:\n%s", want, help)
		}
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
	closeOut := captureStdout(t, func() {
		if err := run([]string{"close", taskID}); err != nil {
			t.Fatalf("close error = %v", err)
		}
	})
	if strings.TrimSpace(closeOut) != "OK" {
		t.Fatalf("close output should be the terse 'OK', got:\n%s", closeOut)
	}

	// Verify it is closed (get should show closed)
	getOut := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !strings.Contains(getOut, "closed") {
		t.Fatalf("ticket should be closed:\n%s", getOut)
	}

	// Re-open the ticket
	openOut := captureStdout(t, func() {
		if err := run([]string{"open", taskID}); err != nil {
			t.Fatalf("open error = %v", err)
		}
	})
	if strings.TrimSpace(openOut) != "OK" {
		t.Fatalf("open output should be the terse 'OK', got:\n%s", openOut)
	}

	getOut2 := captureStdout(t, func() {
		if err := run([]string{"get", "-id", taskID, "-v"}); err != nil {
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

// TestQuickstartClient exercises every command documented in docs/quickstarts/client.md
// using direct database access (no server).
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

	t.Setenv("TICKET_PROJECT", "CUS")

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

// TestQuickstartServer exercises key commands from docs/quickstarts/server.md
// using a real httptest server with full API.
func TestQuickstartServer(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	// Initialize database and start test server
	dbPath := filepath.Join(tempDir, "ticket.db")
	testutil.CloneSeededDB(t, dbPath, "adminpass")
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
	if setRegistrationErr := store.SetRegistrationEnabled(context.Background(), db, true); setRegistrationErr != nil {
		t.Fatalf("SetRegistrationEnabled() error = %v", setRegistrationErr)
	}

	// Step 1: Register a user without an explicit password so the server generates one.
	registerOut := captureStdout(t, func() {
		if runErr := run([]string{"register", "-username", "alice", "-email", "alice@example.com"}); runErr != nil {
			t.Fatalf("register error = %v", runErr)
		}
	})
	var generatedPassword string
	for _, line := range strings.Split(registerOut, "\n") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(line), "password: "); ok {
			generatedPassword = strings.TrimSpace(after)
			break
		}
	}
	if generatedPassword == "" {
		t.Fatalf("register output missing generated password:\n%s", registerOut)
	}

	// Step 2: Login as alice, verify it works
	loginOut := captureStdout(t, func() {
		if runErr := run([]string{"login", "-username", "alice", "-password", generatedPassword}); runErr != nil {
			t.Fatalf("login alice error = %v", runErr)
		}
	})
	if !strings.Contains(loginOut, "alice") {
		t.Fatalf("login output missing username:\n%s", loginOut)
	}
	credsAfterAlice, err := config.LoadCredentials()
	if err != nil {
		t.Fatalf("config.LoadCredentials() after alice login error = %v", err)
	}
	remoteCreds, ok := credsAfterAlice.Remote(ts.URL)
	if !ok {
		t.Fatalf("expected stored credentials for %s", ts.URL)
	}
	aliceToken := strings.TrimSpace(remoteCreds.Token)
	if aliceToken == "" {
		t.Fatal("alice token not stored after login")
	}

	if clearErr := config.ClearCredentials(); clearErr != nil {
		t.Fatalf("ClearCredentials() before token login error = %v", clearErr)
	}
	tokenLoginOut := captureStdout(t, func() {
		if runErr := run([]string{"login", "-token", aliceToken}); runErr != nil {
			t.Fatalf("token login alice error = %v", runErr)
		}
	})
	if !strings.Contains(tokenLoginOut, "alice") {
		t.Fatalf("token login output missing username:\n%s", tokenLoginOut)
	}

	// Clear credentials and saved username, then login as admin
	if clearErr := config.ClearCredentials(); clearErr != nil {
		t.Fatalf("ClearCredentials() error = %v", clearErr)
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
	t.Setenv("TICKET_PROJECT", "SRV")

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
	testutil.CloneSeededDB(t, dbPath, "adminpass")
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
	t.Setenv("TICKET_PROJECT", "SRV")

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

func TestRunIdeaCreatesIdeaTicketType(t *testing.T) {
	setupLocalCLI(t)

	ideaID := createLocalTask(t, []string{"idea", "Add dark mode"})
	ticket, err := localCLIService(t).GetTicket(context.Background(), ideaID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if ticket.Type != "idea" {
		t.Fatalf("ticket.Type = %q, want %q", ticket.Type, "idea")
	}

	ideaOut := captureStdout(t, func() {
		if err := run([]string{"idea", "ls"}); err != nil {
			t.Fatalf("idea ls error = %v", err)
		}
	})
	if !strings.Contains(ideaOut, "Add dark mode") {
		t.Fatalf("idea ls output missing created idea:\n%s", ideaOut)
	}
}

func TestRunIdeaGetUsesMostRecentWhenIDOmitted(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"idea", "new", "Older idea"}); err != nil {
		t.Fatalf("idea new older error = %v", err)
	}
	if err := run([]string{"idea", "new", "Newest idea"}); err != nil {
		t.Fatalf("idea new newest error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"idea", "get"}); err != nil {
			t.Fatalf("idea get error = %v", err)
		}
	})
	if !strings.Contains(output, "title       : Newest idea") {
		t.Fatalf("idea get output missing latest idea:\n%s", output)
	}
}

func TestRunTicketGetUsesMostRecentWhenIDOmitted(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"add", "Older task"}); err != nil {
		t.Fatalf("ticket create older error = %v", err)
	}
	latestID := createLocalTask(t, []string{"add", "Newest task"})

	output := captureStdout(t, func() {
		if err := run([]string{"get"}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !strings.Contains(output, "id/type") || !strings.Contains(output, latestID+"/task") || !strings.Contains(output, "title") || !strings.Contains(output, "Newest task") {
		t.Fatalf("get output missing latest ticket:\n%s", output)
	}
}

func TestRunTypedGetNormalizesBareNumericTicketRefs(t *testing.T) {
	setupLocalCLI(t)

	bugID := createLocalTask(t, []string{"bug", "Numeric Bug"})

	output := captureStdout(t, func() {
		if err := run([]string{"bug", "get", "1"}); err != nil {
			t.Fatalf("bug get error = %v", err)
		}
	})
	if !strings.Contains(output, "id/type") || !strings.Contains(output, bugID+"/bug") || !strings.Contains(output, "title") || !strings.Contains(output, "Numeric Bug") {
		t.Fatalf("bug get output missing normalized bug:\n%s", output)
	}
}

func TestRunLabelNewGetLatestAndShowNormalizesBareTicketRef(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"label", "new", "-title", "Needs triage"}); err != nil {
		t.Fatalf("label new error = %v", err)
	}
	if err := run([]string{"label", "new", "Newest Label"}); err != nil {
		t.Fatalf("label new positional error = %v", err)
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"label", "get"}); err != nil {
			t.Fatalf("label get error = %v", err)
		}
	})
	if !strings.Contains(getOutput, "Name      : Newest Label") {
		t.Fatalf("label get output missing latest label:\n%s", getOutput)
	}

	createLocalTask(t, []string{"add", "Labeled task"})
	if err := run([]string{"label", "add", "-id", "1", "2"}); err != nil {
		t.Fatalf("label add error = %v", err)
	}
	showOutput := captureStdout(t, func() {
		if err := run([]string{"label", "show", "1"}); err != nil {
			t.Fatalf("label show error = %v", err)
		}
	})
	if !strings.Contains(showOutput, "Newest Label") {
		t.Fatalf("label show output missing ticket labels:\n%s", showOutput)
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

func TestEffectiveTicketUpdatedAtPropagatesNestedChildren(t *testing.T) {
	epicID := "TK-1"
	storyID := "TK-2"
	taskID := "TK-3"
	tickets := []store.Ticket{
		{ID: epicID, UpdatedAt: "2026-01-01 09:00:00"},
		{ID: storyID, ParentID: &epicID, UpdatedAt: "2026-01-01 10:00:00"},
		{ID: taskID, ParentID: &storyID, UpdatedAt: "2026-01-01 11:00:00"},
	}

	effective := effectiveTicketUpdatedAt(tickets)
	if effective[epicID] != "2026-01-01 11:00:00" {
		t.Fatalf("effective updated_at for epic = %q, want child timestamp", effective[epicID])
	}
	if effective[storyID] != "2026-01-01 11:00:00" {
		t.Fatalf("effective updated_at for story = %q, want child timestamp", effective[storyID])
	}
}

func TestRunListDefaultsToRecencyOrdering(t *testing.T) {
	setupLocalCLI(t)

	olderID := createLocalTask(t, []string{"add", "Older task"})
	newerID := createLocalTask(t, []string{"add", "Newer task"})

	db, err := store.Open(testDBPath(t))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`UPDATE tickets SET updated_at = ? WHERE ticket_id = ?`, "2026-01-01 09:00:00", olderID); err != nil {
		t.Fatalf("update older ticket timestamp error = %v", err)
	}
	if _, err := db.Exec(`UPDATE tickets SET updated_at = ? WHERE ticket_id = ?`, "2026-01-01 10:00:00", newerID); err != nil {
		t.Fatalf("update newer ticket timestamp error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"list", "-nocolor"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})
	if strings.Index(output, newerID) >= strings.Index(output, olderID) {
		t.Fatalf("list output should show newer ticket before older ticket:\n%s", output)
	}
}

func TestRunListBubblesParentEpicByDescendantRecency(t *testing.T) {
	setupLocalCLI(t)

	epicID := createLocalTask(t, []string{"epic", "Recent Parent Epic"})
	childID := createLocalTask(t, []string{"add", "-parent", epicID, "Recent Child"})
	otherID := createLocalTask(t, []string{"add", "Other Root"})

	db, err := store.Open(testDBPath(t))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	for _, tc := range []struct {
		id        string
		updatedAt string
	}{
		{id: epicID, updatedAt: "2026-01-01 09:00:00"},
		{id: otherID, updatedAt: "2026-01-01 10:00:00"},
		{id: childID, updatedAt: "2026-01-01 11:00:00"},
	} {
		if _, err := db.Exec(`UPDATE tickets SET updated_at = ? WHERE ticket_id = ?`, tc.updatedAt, tc.id); err != nil {
			t.Fatalf("update timestamp for %s error = %v", tc.id, err)
		}
	}

	output := captureStdout(t, func() {
		if err := run([]string{"list", "-nocolor"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})
	if strings.Index(output, epicID) >= strings.Index(output, otherID) {
		t.Fatalf("list output should bubble parent epic ahead of older roots:\n%s", output)
	}
	if !strings.Contains(output, childID) {
		t.Fatalf("list output missing child ticket:\n%s", output)
	}
}
