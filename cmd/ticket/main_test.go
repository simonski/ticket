package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

func TestRenderRootUsageShowsMainCommandsOnly(t *testing.T) {
	original := selectBannerWord
	selectBannerWord = func() string { return "TICKET" }
	defer func() { selectBannerWord = original }()

	usage := renderRootUsage()

	for _, want := range []string{
		"TTTTTTT",
		"USAGE",
		"CLIENT COMMANDS",
		"LIFECYCLE COMMANDS",
		"STAGE COMMANDS",
		"STATE COMMANDS",
		"ADMIN COMMANDS",
		"\x1b[38;5;117m",
		"config",
		"init",
		"server",
		"version",
		"upgrade",
		"login",
		"project",
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

	clientOrder := []string{
		"  login",
		"  register",
		"  logout",
		"  status",
		"  config",
		"  project",
		"  team",
		"  agent",
		"  workflow",
		"  label",
		"  time",
		"  add",
		"  get",
		"  board",
		"  list",
		"  search",
		"  update",
		"  delete",
		"  clone",
		"  claim",
		"  unclaim",
		"  request",
		"  request-dryrun",
		"  set-parent",
		"  attach",
		"  unset-parent",
		"  detach",
		"  comment",
		"  dependency",
		"  health",
		"  count",
		"  orphans",
		"\n  ticket          ",
		"  onboard",
		"  help",
		"  upgrade",
		"  version",
	}
	last := -1
	for _, item := range clientOrder {
		idx := strings.Index(usage, item)
		if idx == -1 {
			t.Fatalf("root usage missing ordered client command %q:\n%s", item, usage)
		}
		if idx <= last {
			t.Fatalf("root usage client commands not in expected order around %q:\n%s", item, usage)
		}
		last = idx
	}

	adminOrder := []string{"  assign", "  export", "  import", "  init", "  server", "  unassign", "  user"}
	last = -1
	for _, item := range adminOrder {
		idx := strings.Index(usage, item)
		if idx == -1 {
			t.Fatalf("root usage missing ordered admin command %q:\n%s", item, usage)
		}
		if idx <= last {
			t.Fatalf("root usage admin commands not alphabetical around %q:\n%s", item, usage)
		}
		last = idx
	}

	for _, unwanted := range []string{"ALIASES", "create,new", "del,delete", "  ls", "  show"} {
		if strings.Contains(usage, unwanted) {
			t.Fatalf("root usage should not include aliases %q:\n%s", unwanted, usage)
		}
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

	if err := run([]string{"rm", "-id", strconv.FormatInt(ticketID, 10)}); err != nil {
		t.Fatalf("run(rm) error = %v", err)
	}
	if err := run([]string{"get", "-id", strconv.FormatInt(ticketID, 10)}); err == nil || err.Error() != "ticket not found" {
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
		if err := run([]string{"get", "-id", strconv.FormatInt(ticketID, 10)}); err != nil {
			t.Fatalf("run(get restored) error = %v", err)
		}
	})
	if !strings.Contains(getOutput, fmt.Sprintf("ID           : %d", ticketID)) {
		t.Fatalf("restored get output missing ticket id %d:\n%s", ticketID, getOutput)
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
	taskID := createLocalTask(t, []string{"add", "-parent", strconv.FormatInt(parentID, 10), "-ac", "Child has AC", "-title", "Child Task"})

	output := captureStdout(t, func() {
		if err := run([]string{"comment", "add", "-id", strconv.FormatInt(taskID, 10), "Reviewer approved this ticket."}); err != nil {
			t.Fatalf("comment add error = %v", err)
		}
		if err := run([]string{"health", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("health error = %v", err)
		}
	})

	for _, want := range []string{
		"TICKET HEALTH",
		"score: 1.00",
		"not_an_orphan: true",
		"has_acceptance_criteria: true",
		"reviewed_by_reviewer_agent: true",
		"definition_of_ready: true",
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
	output := captureStdout(t, func() {
		if err := run([]string{"health", strconv.FormatInt(taskID, 10), "-json"}); err != nil {
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
	if section["score"] != float64(1) {
		t.Fatalf("health score = %#v", got["score"])
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
}

func TestRunHealthExecutePersistsScores(t *testing.T) {
	previousJSON := outputJSON
	defer func() { outputJSON = previousJSON }()

	setupLocalCLI(t)

	firstID := createLocalTask(t, []string{"add", "Task One"})
	secondID := createLocalTask(t, []string{"add", "Task Two", "-ac", "criteria", "-parent", strconv.FormatInt(firstID, 10)})
	if err := run([]string{"comment", "add", "-id", strconv.FormatInt(secondID, 10), "Approved by reviewer"}); err != nil {
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

	tasks := []int64{firstID, secondID}
	for _, id := range tasks {
		task, err := svcGetTicket(t, strconv.FormatInt(id, 10))
		if err != nil {
			t.Fatalf("svc.GetTicket(%d) error = %v", id, err)
		}
		if task.HealthScore == 0 {
			t.Fatalf("ticket %d health score = %d", id, task.HealthScore)
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
	help := renderCommandHelp("init")

	for _, want := range []string{
		"USAGE",
		"ticket init",
		"DETAILS",
		"EXAMPLE",
		"ticket init -f /path/to/ticket.db --force -password secret --populate",
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

func TestRenderServerHelpIncludesTaskHomeDefault(t *testing.T) {
	help := renderCommandHelp("server")
	for _, want := range []string{
		"ticket server [-f <db-path>] [-p <port>] [-addr <host:port>] [-v]",
		"the server uses the database path from TICKET_URL",
		"ticket server -f /path/to/ticket.db -p 9999 -v",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("server help missing %q:\n%s", want, help)
		}
	}
}

func TestRenderUserHelpIncludesAdmin403Message(t *testing.T) {
	help := renderCommandHelp("user")
	for _, want := range []string{
		"ticket user <create|ls|list|rm|delete|enable|disable>",
		"user is not an admin",
		"ticket user create --username alice --password secret",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("user help missing %q:\n%s", want, help)
		}
	}
}

func TestRenderConfigHelpIncludesListAndDelete(t *testing.T) {
	help := renderCommandHelp("config")
	for _, want := range []string{
		"ticket config <set|get|ls|list|rm|delete|registration-enable|registration-disable> [key] [value]",
		"ticket config ls",
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
		"TICKET_URL",
		"TICKET_CONFIG_DIR",
		"TICKET_USERNAME",
		"TICKET_PASSWORD",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("TICKET_URL", "file:///tmp/test.db")

	output := captureStdout(t, func() {
		if err := runHelp([]string{}); err != nil {
			t.Fatalf("runHelp() error = %v", err)
		}
	})

	for _, want := range []string{
		"ENVIRONMENT",
		"  TICKET_URL: file:///tmp/test.db",
		"  TICKET_CONFIG_DIR: <unset>",
		"  TICKET_USERNAME: <unset>",
		"  TICKET_PASSWORD: <unset>",
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
	runAgentCommand = func(agent, prompt string) (string, error) {
		gotAgent = agent
		gotPrompt = prompt
		return "generated requirements", nil
	}

	stdout := captureStdout(t, func() {
		if err := runTicket([]string{"-f", input, "-o", output}); err != nil {
			t.Fatalf("runTicket() error = %v", err)
		}
	})
	if gotAgent != "codex" {
		t.Fatalf("runTicket() agent = %q, want codex", gotAgent)
	}
	if !strings.Contains(gotPrompt, "source") || !strings.Contains(gotPrompt, "requirements.md") {
		t.Fatalf("runTicket() prompt = %q", gotPrompt)
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
	runAgentCommand = func(agent, prompt string) (string, error) {
		gotAgent = agent
		return "ok", nil
	}

	if err := runTicket([]string{"-f", input, "-o", output, "-agent", "copilot"}); err != nil {
		t.Fatalf("runTicket(agent override) error = %v", err)
	}
	if gotAgent != "copilot" {
		t.Fatalf("runTicket(agent override) agent = %q, want copilot", gotAgent)
	}
}

func TestResolveCredentialsUsesFlagsEnvAndDefaults(t *testing.T) {
	t.Setenv("TICKET_URL", "http://localhost:8080")

	t.Setenv("TICKET_USERNAME", "env-user")
	t.Setenv("TICKET_PASSWORD", "env-pass")

	username, password, err := resolveCredentials("", "", true)
	if err != nil {
		t.Fatalf("resolveCredentials(env) error = %v", err)
	}
	if username != "env-user" || password != "env-pass" {
		t.Fatalf("resolveCredentials(env) = %q/%q", username, password)
	}

	username, password, err = resolveCredentials("flag-user", "flag-pass", true)
	if err != nil {
		t.Fatalf("resolveCredentials(flags) error = %v", err)
	}
	if username != "flag-user" || password != "flag-pass" {
		t.Fatalf("resolveCredentials(flags) = %q/%q", username, password)
	}

	t.Setenv("TICKET_USERNAME", "")
	t.Setenv("TICKET_PASSWORD", "")
	username, password, err = resolveCredentials("", "", true)
	if err != nil {
		t.Fatalf("resolveCredentials(defaults) error = %v", err)
	}
	if password != "password" {
		t.Fatalf("resolveCredentials(default password) = %q", password)
	}
	if username == "" {
		t.Fatal("resolveCredentials(default username) returned empty username")
	}

	t.Setenv("TICKET_URL", "")
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

func TestExtractURLOverride(t *testing.T) {
	args, override, err := extractURLOverride([]string{"login", "-username", "simon", "-url", "http://example.test:9000"})
	if err != nil {
		t.Fatalf("extractURLOverride() error = %v", err)
	}
	if override != "http://example.test:9000" {
		t.Fatalf("extractURLOverride() override = %q", override)
	}
	got := strings.Join(args, " ")
	if got != "login -username simon" {
		t.Fatalf("extractURLOverride() args = %q", got)
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

func TestEmbeddedVersionMatchesBuildVersionFile(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("VERSION"))
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
	want := "A newer version of ticket is available, upgrade using `go install github.com/simonski/ticket@latest`"
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

func TestRunInitDBGeneratesPasswordWhenMissing(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_CONFIG_DIR", tempDir)
	t.Setenv("TICKET_URL", "")
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
	if !strings.Contains(output, "generated because -password was not provided") {
		t.Fatalf("runInitDB() output missing generated-password note:\n%s", output)
	}
}

func TestRunInitDBUsesDefaultPathWhenFIsOmitted(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_CONFIG_DIR", tempDir)
	t.Setenv("TICKET_URL", "")

	if err := runInitDB([]string{"-password", "secret"}); err != nil {
		t.Fatalf("runInitDB() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "ticket.db")); err != nil {
		t.Fatalf("expected default db at config dir/ticket.db: %v", err)
	}
}

func TestRunInitDBForceOverwritesExistingDatabase(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_CONFIG_DIR", tempDir)
	t.Setenv("TICKET_URL", "")
	dbPath := filepath.Join(tempDir, "ticket.db")

	if err := runInitDB([]string{"-f", dbPath, "-password", "first-pass"}); err != nil {
		t.Fatalf("first runInitDB() error = %v", err)
	}
	if err := runInitDB([]string{"-f", dbPath, "-password", "second-pass"}); err == nil {
		t.Fatal("second runInitDB() without --force = nil, want error")
	}
	if err := runInitDB([]string{"-f", dbPath, "--force", "-password", "second-pass"}); err != nil {
		t.Fatalf("forced runInitDB() error = %v", err)
	}
}

func TestRunInitDBPopulateSeedsProjectsStoriesTicketsUsersAndTeams(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_CONFIG_DIR", tempDir)
	t.Setenv("TICKET_URL", "")
	dbPath := filepath.Join(tempDir, "ticket.db")

	if err := runInitDB([]string{"-f", dbPath, "-password", "secret", "--populate"}); err != nil {
		t.Fatalf("runInitDB(--populate) error = %v", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer db.Close()

	projects, err := store.ListProjects(db)
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
	username, password, err := promptForCredentials(strings.NewReader("alice\nsecret\n"), ioDiscard{}, "", "")
	if err != nil {
		t.Fatalf("promptForCredentials() error = %v", err)
	}
	if username != "alice" || password != "secret" {
		t.Fatalf("promptForCredentials() = %q/%q", username, password)
	}
}

func TestPromptForCredentialsUsesDefaultsWhenInputIsEmpty(t *testing.T) {
	username, password, err := promptForCredentials(strings.NewReader("\n\n"), ioDiscard{}, "alice", "secret")
	if err != nil {
		t.Fatalf("promptForCredentials(defaults) error = %v", err)
	}
	if username != "alice" || password != "secret" {
		t.Fatalf("promptForCredentials(defaults) = %q/%q", username, password)
	}
}

func TestLoginRetryStoresCredentialsSeparatelyAndLogoutRemovesThem(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_CONFIG_DIR", tempDir)
	credsPath := filepath.Join(tempDir, "credentials.json")
	t.Setenv("TICKET_URL", "")

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
			if loginPayload["username"] != "alice" || loginPayload["password"] != "secret" {
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
	t.Setenv("TICKET_URL", server.URL)

	oldIn := loginPromptInput
	oldOut := loginPromptOutput
	loginPromptInput = strings.NewReader("alice\nsecret\n")
	loginPromptOutput = ioDiscard{}
	t.Cleanup(func() {
		loginPromptInput = oldIn
		loginPromptOutput = oldOut
	})

	output := captureStdout(t, func() {
		if err := runLogin([]string{"-username", "alice", "-password", "wrong"}); err != nil {
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
	if !strings.Contains(string(configData), `"username": "alice"`) {
		t.Fatalf("config.json should contain username alice:\n%s", string(configData))
	}
	if !strings.Contains(string(configData), `"server_url": "`+server.URL+`"`) {
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
	t.Setenv("TICKET_CONFIG_DIR", tempDir)
	t.Setenv("TICKET_URL", "")

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
	t.Setenv("TICKET_URL", server.URL)

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
	if !strings.Contains(string(configData), `"server_url": "`+server.URL+`"`) {
		t.Fatalf("config.json should contain resolved server URL %q:\n%s", server.URL, string(configData))
	}
}

func TestRunStatusRemoteSuccess(t *testing.T) {
	t.Setenv("TICKET_CONFIG_DIR", t.TempDir())
	t.Setenv("TICKET_URL", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","authenticated":true,"user":{"username":"alice","role":"user"}}`))
	}))
	defer server.Close()
	t.Setenv("TICKET_URL", server.URL)

	output := captureStdout(t, func() {
		if err := runStatus(nil); err != nil {
			t.Fatalf("runStatus(remote) error = %v", err)
		}
	})
	for _, want := range []string{
		"TICKET_URL: " + server.URL,
		"username: alice",
		"authenticated: true",
		"connection: ",
		"success",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus(remote) missing %q:\n%s", want, output)
		}
	}
}

func TestRunStatusLocalMissingDatabasePrintsHint(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_URL", "file://"+filepath.Join(tempDir, "ticket.db"))
	t.Setenv("TICKET_CONFIG_DIR", tempDir)

	var runErr error
	output := captureStdout(t, func() {
		runErr = runStatus(nil)
	})
	if !errors.Is(runErr, os.ErrNotExist) {
		t.Fatalf("runStatus(local missing) error = %v, want os.ErrNotExist", runErr)
	}
	for _, want := range []string{
		"db_exists: false",
		"failure",
		"hint: run ticket init",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus(local missing) missing %q:\n%s", want, output)
		}
	}
	if !strings.Contains(output, "TICKET_URL: file://"+filepath.Join(tempDir, "ticket.db")) {
		t.Fatalf("runStatus(local missing) output missing TICKET_URL:\n%s", output)
	}
}

func TestRunStatusLocalSuccess(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_URL", "file://"+filepath.Join(tempDir, "ticket.db"))
	t.Setenv("TICKET_CONFIG_DIR", tempDir)
	if err := runInitDB([]string{"-password", "secret"}); err != nil {
		t.Fatalf("runInitDB() error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"status", "-nocolor"}); err != nil {
			t.Fatalf("runStatus(local) error = %v", err)
		}
	})
	for _, want := range []string{
		"db_exists: true",
		"connection: success",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus(local) missing %q:\n%s", want, output)
		}
	}
	if !strings.Contains(output, "TICKET_URL: file://"+filepath.Join(tempDir, "ticket.db")) {
		t.Fatalf("runStatus(local) output missing TICKET_URL:\n%s", output)
	}
}

func TestPrintTaskDetailsIncludesAcceptanceCriteria(t *testing.T) {
	output := captureStdout(t, func() {
		printTicketDetails(store.Ticket{
			ID:                 42,
			Title:              "Example Task",
			Type:               "task",
			Status:             "design/idle",
			Stage:              "design",
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
			{EventType: "ticket_created", CreatedAt: "2026-03-01 12:00:00", CreatedBy: 1, Payload: "{\"status\":\"design/idle\"}"},
		}, nil, nil, 0)
	})

	for _, want := range []string{
		"ID           : 42",
		"Type         : task",
		"Description  : Example description",
		"ParentID     : ",
		"ProjectID    : 7",
		"Title        : Example Task",
		"Assignee     : ",
		"Order        : 0",
		"EstimateEffort   : 3",
		"EstimateComplete : 2026-04-01T12:00:00Z",
		"DependsOn    : []",
		"Status       : design/idle",
		"Stage        : design",
		"State        : idle",
		"Priority     : 1",
		"Created      : 2026-03-01 12:00:00",
		"LastModified : 2026-03-02 09:30:00",
		"Acceptance Criteria : - does the thing",
		"Comments     :",
		"[2026-03-02 10:00:00] alice: latest comment",
		"History      :",
		"[2026-03-01 12:00:00] ticket_created by 1: {\"status\":\"design/idle\"}",
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
		{StageName: "design"},
		{StageName: "develop"},
		{StageName: "test"},
		{StageName: "done"},
	}
	got := renderWorkflowProgress("develop", stages)
	want := "design → [develop] → test → done"
	if got != want {
		t.Fatalf("renderWorkflowProgress() = %q, want %q", got, want)
	}
}

func TestRunStageStateCommandsUpdateLifecycle(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "-ac", "criteria", "Ticket Beta"})

	// Stage commands now return errors (stage is workflow-driven)
	if err := run([]string{"stage", "-id", strconv.FormatInt(taskID, 10), "develop"}); err == nil {
		t.Fatal("stage command should return error, got nil")
	}
	if err := run([]string{"develop", "-id", strconv.FormatInt(taskID, 10)}); err == nil {
		t.Fatal("develop command should return error, got nil")
	}

	// Claim first so active state is allowed (requires assignee)
	if err := run([]string{"claim", strconv.FormatInt(taskID, 10)}); err != nil {
		t.Fatalf("claim error = %v", err)
	}

	// State commands still work and keep the current stage (design)
	stateOutput := captureStdout(t, func() {
		if err := run([]string{"state", "-id", strconv.FormatInt(taskID, 10), "active", "-json"}); err != nil {
			t.Fatalf("state command error = %v", err)
		}
	})
	var stateData map[string]any
	if err := json.Unmarshal([]byte(stateOutput), &stateData); err != nil {
		t.Fatalf("state output parse error = %v\n%s", err, stateOutput)
	}
	for _, want := range []string{"design/active", "design", "active"} {
		if got := stateData["status"]; got != want && stateData["stage"] != want && stateData["state"] != want {
			t.Fatalf("state output missing %q in status/stage/state: %#v", want, stateData)
		}
	}
}

func TestRunProjectCommandsInLocalMode(t *testing.T) {
	setupLocalCLI(t)

	createOutput := captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "PRA", "-title", "Project A", "-description", "Desc", "-ac", "AC"}); err != nil {
			t.Fatalf("project create error = %v", err)
		}
	})
	for _, want := range []string{"project: Project A", "prefix: PRA", "status: open", "acceptance_criteria: AC"} {
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
		if err := run([]string{"project", "2", "update", "-title", "Project B", "-description", "Updated", "-ac", "AC2"}); err != nil {
			t.Fatalf("project update error = %v", err)
		}
	})
	for _, want := range []string{"project: Project B", "description: Updated", "acceptance_criteria: AC2"} {
		if !strings.Contains(updateOutput, want) {
			t.Fatalf("project update output missing %q:\n%s", want, updateOutput)
		}
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

func TestRunProjectInit(t *testing.T) {
	setupLocalCLI(t)

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
	if !strings.Contains(initOutput, ".ticket.json") {
		t.Fatalf("project init output missing .ticket.json: %s", initOutput)
	}

	// Verify .ticket.json was created
	data, err := os.ReadFile(filepath.Join(projDir, ".ticket.json"))
	if err != nil {
		t.Fatalf("reading .ticket.json: %v", err)
	}
	if !strings.Contains(string(data), "INI") {
		t.Fatalf(".ticket.json does not contain INI: %s", data)
	}

	// Running init again should fail (already exists)
	if err := run([]string{"project", "init", "-prefix", "INI"}); err == nil {
		t.Fatal("expected error on second init, got nil")
	}
}

func TestRunListStatusRenderingSupportsUnicodeAndPlainModes(t *testing.T) {
	setupLocalCLI(t)

	_ = createLocalTask(t, []string{"add", "Moon Open Task"})
	inProgressID := createLocalTask(t, []string{"add", "Moon Inprogress Task"})
	completeID := createLocalTask(t, []string{"add", "Moon Complete Task"})
	// Claim to assign, then set active (stays at design stage)
	if err := run([]string{"claim", strconv.FormatInt(inProgressID, 10)}); err != nil {
		t.Fatalf("claim error = %v", err)
	}
	if err := run([]string{"active", "-id", strconv.FormatInt(inProgressID, 10)}); err != nil {
		t.Fatalf("active error = %v", err)
	}
	// complete auto-advances from design to develop/idle
	if err := run([]string{"complete", "-id", strconv.FormatInt(completeID, 10)}); err != nil {
		t.Fatalf("complete error = %v", err)
	}

	unicodeOutput := captureStdout(t, func() {
		if err := run([]string{"list"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})
	checkRow := func(statusSymbol, statusText string) {
		found := false
		for _, line := range strings.Split(unicodeOutput, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 4 && fields[0] == statusSymbol && fields[2] == "task" && fields[3] == statusText {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("list unicode row missing symbol=%q status=%q:\n%s", statusSymbol, statusText, unicodeOutput)
		}
	}
	checkRow("◑", "design/active")
	checkRow("○", "develop/idle")
	checkRow("○", "design/idle")

	for _, want := range []string{"design/active", "develop/idle", "design/idle"} {
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
	if err := run([]string{"health", strconv.FormatInt(taskID, 10)}); err != nil {
		t.Fatalf("health error = %v", err)
	}

	listOutput := captureStdout(t, func() {
		if err := run([]string{"list"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})

	if !strings.Contains(listOutput, "HEALTH") {
		t.Fatalf("list output missing health header:\n%s", listOutput)
	}
	if !strings.Contains(listOutput, "0.25") {
		t.Fatalf("list output missing health fraction value 0.25:\n%s", listOutput)
	}
}

func TestRunListArchivedVisibilityAndColumn(t *testing.T) {
	setupLocalCLI(t)

	openID := createLocalTask(t, []string{"add", "Open Task"})
	archivedID := createLocalTask(t, []string{"add", "Archived Task"})
	if err := run([]string{"archive", "-id", strconv.FormatInt(archivedID, 10)}); err != nil {
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

	includeArchivedOutput := captureStdout(t, func() {
		if err := run([]string{"list", "-a"}); err != nil {
			t.Fatalf("list -a error = %v", err)
		}
	})
	if !strings.Contains(includeArchivedOutput, "ARCHIVED") {
		t.Fatalf("list -a output missing ARCHIVED column:\n%s", includeArchivedOutput)
	}
	if !strings.Contains(includeArchivedOutput, archivedRef) {
		t.Fatalf("list -a output missing archived ticket %q:\n%s", archivedRef, includeArchivedOutput)
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
	if err := run([]string{"comment", "add", "-id", strconv.FormatInt(taskID, 10), "latest note"}); err != nil {
		t.Fatalf("comment add error = %v", err)
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	for _, want := range []string{"Title        : Ticket Alpha", "Description  : findable description", "Acceptance Criteria : ship it", "EstimateEffort   : 8", "EstimateComplete : 2026-04-20T17:00:00Z", "latest note"} {
		if !strings.Contains(getOutput, want) {
			t.Fatalf("get output missing %q:\n%s", want, getOutput)
		}
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
		if err := run([]string{"dependency", "add", "-id", strconv.FormatInt(taskID, 10), strconv.FormatInt(depID, 10)}); err != nil {
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

	// Advance ticket to develop/idle so it's claimable via specific request
	if err := run([]string{"complete", "-id", strconv.FormatInt(taskID, 10)}); err != nil {
		t.Fatalf("complete (advance to develop) error = %v", err)
	}

	requestOutput := captureStdout(t, func() {
		if err := run([]string{"request", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("request error = %v", err)
		}
	})
	if !strings.Contains(requestOutput, "ticket: Ticket Alpha") || !strings.Contains(requestOutput, "status: develop/active") {
		t.Fatalf("request output = %q", requestOutput)
	}

	requestDryRunOutput := captureStdout(t, func() {
		if err := run([]string{"request", "-dryrun"}); err != nil {
			t.Fatalf("request -dryrun error = %v", err)
		}
	})
	if !strings.Contains(requestDryRunOutput, "would assign ticket: ") {
		t.Fatalf("request -dryrun output = %q", requestDryRunOutput)
	}

	// Advance depID to develop/idle so it's claimable
	if err := run([]string{"complete", "-id", strconv.FormatInt(depID, 10)}); err != nil {
		t.Fatalf("complete (advance dep to develop) error = %v", err)
	}

	claimOutput := captureStdout(t, func() {
		if err := run([]string{"claim", strconv.FormatInt(depID, 10)}); err != nil {
			t.Fatalf("claim error = %v", err)
		}
	})
	if !strings.Contains(claimOutput, "status: develop/active") {
		t.Fatalf("claim output = %q", claimOutput)
	}

	unclaimOutput := captureStdout(t, func() {
		if err := run([]string{"unclaim", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("unclaim error = %v", err)
		}
	})
	if !strings.Contains(unclaimOutput, "unassigned") {
		t.Fatalf("unclaim output = %q", unclaimOutput)
	}

	cloneOutput := captureStdout(t, func() {
		if err := run([]string{"clone", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("clone error = %v", err)
		}
	})
	if !strings.Contains(cloneOutput, "clone_of: "+strconv.FormatInt(taskID, 10)) || !strings.Contains(cloneOutput, "status: design/idle") {
		t.Fatalf("clone output = %q", cloneOutput)
	}

	commentOutput := captureStdout(t, func() {
		if err := run([]string{"comment", "add", "-id", strconv.FormatInt(taskID, 10), "hello"}); err != nil {
			t.Fatalf("comment error = %v", err)
		}
	})
	if !strings.Contains(commentOutput, "commented on") {
		t.Fatalf("comment output = %q", commentOutput)
	}

	setParentOutput := captureStdout(t, func() {
		if err := run([]string{"set-parent", "-id", strconv.FormatInt(depID, 10), strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("set-parent error = %v", err)
		}
	})
	if !strings.Contains(setParentOutput, "parent_id: "+strconv.FormatInt(taskID, 10)) {
		t.Fatalf("set-parent output = %q", setParentOutput)
	}

	unsetParentOutput := captureStdout(t, func() {
		if err := run([]string{"unset-parent", "-id", strconv.FormatInt(depID, 10)}); err != nil {
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

	for _, id := range []int64{taskID, bugID, choreID} {
		getOutput := captureStdout(t, func() {
			if err := run([]string{"get", "-id", strconv.FormatInt(id, 10)}); err != nil {
				t.Fatalf("get error = %v", err)
			}
		})
		want := "ParentID     : " + strconv.FormatInt(epicID, 10)
		if !strings.Contains(getOutput, want) {
			t.Fatalf("get output missing %q:\n%s", want, getOutput)
		}
	}
}

func TestRunSearchSupportsFreeFormAndFilters(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"project", "create", "-prefix", "SEP", "-title", "Second Project"}); err != nil {
		t.Fatalf("project create error = %v", err)
	}
	if err := run([]string{"project", "use", "1"}); err != nil {
		t.Fatalf("project use error = %v", err)
	}

	matchingID := createLocalTask(t, []string{"add", "-d", "Detailed note for customer portal", "-ac", "free form acceptance", "Free form entry"})
	otherID := createLocalTask(t, []string{"add", "-d", "Detailed note for customer portal", "Free form other"})
	if err := run([]string{"project", "use", "2"}); err != nil {
		t.Fatalf("project use error = %v", err)
	}
	crossProjectID := createLocalTask(t, []string{"add", "-d", "Detailed note for customer portal", "Free form entry elsewhere"})
	if err := run([]string{"project", "use", "1"}); err != nil {
		t.Fatalf("project use error = %v", err)
	}

	// Advance to develop/idle so ticket is claimable
	if err := run([]string{"complete", "-id", strconv.FormatInt(matchingID, 10)}); err != nil {
		t.Fatalf("complete (advance to develop) error = %v", err)
	}
	if err := run([]string{"claim", strconv.FormatInt(matchingID, 10)}); err != nil {
		t.Fatalf("claim error = %v", err)
	}
	if err := run([]string{"update", "-id", strconv.FormatInt(matchingID, 10), "-state", "active", "-priority", "4"}); err != nil {
		t.Fatalf("update matching task error = %v", err)
	}
	if err := run([]string{"update", "-id", strconv.FormatInt(otherID, 10), "-priority", "2"}); err != nil {
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
	// Advance to develop/idle so ticket is claimable
	if err := run([]string{"complete", "-id", strconv.FormatInt(taskID, 10)}); err != nil {
		t.Fatalf("complete (advance to develop) error = %v", err)
	}
	if err := run([]string{"claim", strconv.FormatInt(taskID, 10)}); err != nil {
		t.Fatalf("claim error = %v", err)
	}

	updateOutput := captureStdout(t, func() {
		if err := run([]string{
			"update",
			"-id",
			strconv.FormatInt(taskID, 10),
			"-title", "Ticket Beta",
			"-desc", "new description",
			"-ac", "new ac",
			"-priority", "3",
			"-order", "7",
			"-estimate_effort", "5",
			"-estimate_complete", "2026-04-15T12:00:00Z",
			"-status", "develop/active",
			"-parent_id", strconv.FormatInt(parentID, 10),
		}); err != nil {
			t.Fatalf("update error = %v", err)
		}
	})
	for _, want := range []string{
		"Title        : Ticket Beta",
		"Description  : new description",
		"ParentID     : " + strconv.FormatInt(parentID, 10),
		"Order        : 7",
		"EstimateEffort   : 5",
		"EstimateComplete : 2026-04-15T12:00:00Z",
		"Status       : develop/active",
		"Priority     : 3",
		"Acceptance Criteria : new ac",
	} {
		if !strings.Contains(updateOutput, want) {
			t.Fatalf("update output missing %q:\n%s", want, updateOutput)
		}
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	for _, want := range []string{
		"Title        : Ticket Beta",
		"Description  : new description",
		"ParentID     : " + strconv.FormatInt(parentID, 10),
		"Order        : 7",
		"EstimateEffort   : 5",
		"EstimateComplete : 2026-04-15T12:00:00Z",
		"Status       : develop/active",
		"Priority     : 3",
		"Acceptance Criteria : new ac",
	} {
		if !strings.Contains(getOutput, want) {
			t.Fatalf("get output missing %q:\n%s", want, getOutput)
		}
	}
}

func TestRunUpdateSupportsDescriptionAlias(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "-d", "old description", "Ticket Alpha"})

	if err := run([]string{"update", "-id", strconv.FormatInt(taskID, 10), "-description", "updated description"}); err != nil {
		t.Fatalf("update with -description error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-id", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !strings.Contains(output, "Description  : updated description") {
		t.Fatalf("get output = %q", output)
	}
}

func TestRunUpdateRequiresIDFlag(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "Needs ID Update"})

	if err := run([]string{"update", strconv.FormatInt(taskID, 10), "-title", "No ID Flag"}); err == nil || !strings.Contains(err.Error(), "usage: ticket update -id") {
		t.Fatalf("expected usage error for positional id, got %v", err)
	}

	if err := run([]string{"update", "-title", "No ID Flag"}); err == nil || !strings.Contains(err.Error(), "usage: ticket update -id") {
		t.Fatalf("expected usage error for missing -id, got %v", err)
	}
}

func TestRunGetRequiresIDFlag(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "Needs ID Get"})

	if err := run([]string{"get", strconv.FormatInt(taskID, 10)}); err == nil || !strings.Contains(err.Error(), "usage: ticket get -id") {
		t.Fatalf("expected usage error for positional id, got %v", err)
	}

	if err := run([]string{"get"}); err == nil || !strings.Contains(err.Error(), "usage: ticket get -id") {
		t.Fatalf("expected usage error for missing -id, got %v", err)
	}
}

func TestRunTaskCreateSupportsInterspersedFlags(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "the", "thing", "-type", "epic"})

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-id", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	for _, want := range []string{
		"Title        : the thing",
		"Type         : epic",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("interspersed add output missing %q:\n%s", want, output)
		}
	}
}

func TestRunTypedTaskCreateSupportsEstimateFlags(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"epic", "-estimate_effort", "8", "-estimate_complete", "2026-04-20T17:00:00Z", "Estimated Epic"})

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-id", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	for _, want := range []string{
		"Title        : Estimated Epic",
		"Type         : epic",
		"EstimateEffort   : 8",
		"EstimateComplete : 2026-04-20T17:00:00Z",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("typed create output missing %q:\n%s", want, output)
		}
	}
}

func TestRunTaskCreateFallsBackToDefaultProject(t *testing.T) {
	setupLocalCLI(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	cfg.CurrentProject = ""
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}

	taskID := createLocalTask(t, []string{"create", "-t", "epic", "-title", "foo"})
	output := captureStdout(t, func() {
		if err := run([]string{"get", "-id", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	for _, want := range []string{
		"Title        : foo",
		"Type         : epic",
		"ProjectID    : 1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("default project fallback output missing %q:\n%s", want, output)
		}
	}

	reloaded, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load(reloaded) error = %v", err)
	}
	if reloaded.CurrentProject != "1" {
		t.Fatalf("CurrentProject = %q, want 1", reloaded.CurrentProject)
	}
}

func TestRunAssignAndUnassignInLocalMode(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "-estimate_effort", "3", "-estimate_complete", "2026-04-10T09:00:00Z", "Task Gamma"})

	if err := run([]string{"user", "create", "-username", "alice", "-password", "secret"}); err != nil {
		t.Fatalf("user create error = %v", err)
	}

	assignOutput := captureStdout(t, func() {
		if err := run([]string{"assign", strconv.FormatInt(taskID, 10), "alice"}); err != nil {
			t.Fatalf("assign error = %v", err)
		}
	})
	if !strings.Contains(assignOutput, "assigned") || !strings.Contains(assignOutput, "alice") {
		t.Fatalf("assign output = %q", assignOutput)
	}

	unassignOutput := captureStdout(t, func() {
		if err := run([]string{"unassign", strconv.FormatInt(taskID, 10), "alice"}); err != nil {
			t.Fatalf("unassign error = %v", err)
		}
	})
	if !strings.Contains(unassignOutput, "unassigned") {
		t.Fatalf("unassign output = %q", unassignOutput)
	}

	cloneOutput := captureStdout(t, func() {
		if err := run([]string{"clone", strconv.FormatInt(taskID, 10)}); err != nil {
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
		if err := run([]string{"complete", "-id", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("complete error = %v", err)
		}
	})

	if !strings.Contains(output, "status: develop/idle") {
		t.Fatalf("complete output = %q, want status: develop/idle (auto-advanced from design)", output)
	}
}

func TestRunDeleteTicketInLocalMode(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Delete me"})
	output := captureStdout(t, func() {
		if err := run([]string{"delete", "-id", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("delete error = %v", err)
		}
	})
	if !strings.Contains(output, "deleted ticket ") {
		t.Fatalf("delete output = %q", output)
	}
	if err := run([]string{"get", "-id", strconv.FormatInt(taskID, 10)}); err == nil || err.Error() != "ticket not found" {
		t.Fatalf("get deleted task error = %v, want ticket not found", err)
	}
}

func TestRunDeleteTicketFailsWhenTaskHasChildren(t *testing.T) {
	setupLocalCLI(t)

	parentID := createLocalTask(t, []string{"add", "-t", "epic", "Parent"})
	childID := createLocalTask(t, []string{"add", "-parent", strconv.FormatInt(parentID, 10), "Child"})
	if childID == 0 {
		t.Fatal("child task id = 0")
	}
	if err := run([]string{"rm", "-id", strconv.FormatInt(parentID, 10)}); err == nil || err.Error() != "ticket has child tickets" {
		t.Fatalf("delete parent error = %v, want ticket has child tickets", err)
	}
}

func TestRunDeleteRequiresIDFlag(t *testing.T) {
	setupLocalCLI(t)
	taskID := createLocalTask(t, []string{"add", "Needs ID Delete"})

	if err := run([]string{"delete", strconv.FormatInt(taskID, 10)}); err == nil || !strings.Contains(err.Error(), "usage: ticket rm|delete -id") {
		t.Fatalf("expected positional delete usage error, got %v", err)
	}
	if err := run([]string{"delete"}); err == nil || !strings.Contains(err.Error(), "usage: ticket rm|delete -id") {
		t.Fatalf("expected missing -id usage error, got %v", err)
	}
}

func TestRunGetJSONUsesCommentAuthorDateTextShape(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "JSON Task"})
	if err := run([]string{"comment", "add", "-id", strconv.FormatInt(taskID, 10), "first"}); err != nil {
		t.Fatalf("comment add first error = %v", err)
	}
	if err := run([]string{"comment", "add", "-id", strconv.FormatInt(taskID, 10), "second"}); err != nil {
		t.Fatalf("comment add second error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-json", "-id", strconv.FormatInt(taskID, 10)}); err != nil {
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
	if err := run([]string{"invalid"}); err == nil || err.Error() != `no such command "invalid"` {
		t.Fatalf("run(invalid) error = %v", err)
	}
}

func TestRunRemoteOnlyCommandsFailInLocalMode(t *testing.T) {
	t.Setenv("TICKET_URL", "")

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
	t.Setenv("TICKET_URL", "http://127.0.0.1:1")
	t.Setenv("TICKET_CONFIG_DIR", t.TempDir())

	var runErr error
	output := captureStdout(t, func() {
		runErr = runStatus(nil)
	})
	if runErr == nil {
		t.Fatal("runStatus(remote failure) error = nil")
	}
	for _, want := range []string{
		"TICKET_URL: http://127.0.0.1:1",
		"authenticated: false",
		"failure",
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
	taskID := createLocalTask(t, []string{"add", "-parent", strconv.FormatInt(epicID, 10), "Child Task"})
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
		if err := run([]string{"history", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("history error = %v", err)
		}
	})
	if !strings.Contains(historyOutput, "Event      : ticket_created") {
		t.Fatalf("history output = %q", historyOutput)
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"get", "-id", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	for _, want := range []string{"History      :", "ticket_created"} {
		if !strings.Contains(getOutput, want) {
			t.Fatalf("get output missing %q:\n%s", want, getOutput)
		}
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

	if err := run([]string{"config", "set", "server", "http://example.test"}); err != nil {
		t.Fatalf("config set error = %v", err)
	}
	configOutput := captureStdout(t, func() {
		if err := run([]string{"config", "get", "server"}); err != nil {
			t.Fatalf("config get error = %v", err)
		}
	})
	if !strings.Contains(configOutput, "http://example.test") {
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
		"server=http://example.test",
		"username=",
		"current_project=",
		"current_epic_id=",
	} {
		if !strings.Contains(listOutput, want) {
			t.Fatalf("config ls output missing %q:\n%s", want, listOutput)
		}
	}

	if err := run([]string{"config", "rm", "server"}); err != nil {
		t.Fatalf("config rm error = %v", err)
	}
	clearedOutput := captureStdout(t, func() {
		if err := run([]string{"config", "get", "server"}); err != nil {
			t.Fatalf("config get error = %v", err)
		}
	})
	if strings.Contains(clearedOutput, "http://example.test") {
		t.Fatalf("config get output after delete still contains old server: %q", clearedOutput)
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

func TestRunSetParentDisallowsEpicUnderTask(t *testing.T) {
	setupLocalCLI(t)
	epicID := createLocalTask(t, []string{"epic", "Orphan Epic"})
	taskID := createLocalTask(t, []string{"add", "Task Parent"})

	if err := run([]string{"set-parent", "-id", strconv.FormatInt(epicID, 10), strconv.FormatInt(taskID, 10)}); err == nil {
		t.Fatalf("set-parent should reject epic parenting by task")
	} else if !strings.Contains(err.Error(), "task cannot parent epic") {
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
		{[]string{"project", "get"}, "usage: ticket project get <id>"},
		{[]string{"list", "-n", "-1"}, "usage: ticket list|ls"},
		{[]string{"comment", "add", "1"}, "usage: ticket comment add -id <id> \"comment\""},
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
	if !strings.Contains(output, "default") {
		t.Fatalf("workflow list missing default workflow:\n%s", output)
	}
}

func TestRunWorkflowGetShowsStages(t *testing.T) {
	setupLocalCLI(t)
	output := captureStdout(t, func() {
		if err := run([]string{"workflow", "get", "-id", "1"}); err != nil {
			t.Fatalf("workflow get error = %v", err)
		}
	})
	for _, want := range []string{"design", "develop", "test", "done", "BA", "Lead Engineer", "QA/Tester", "Product Owner"} {
		if !strings.Contains(output, want) {
			t.Fatalf("workflow get missing %q:\n%s", want, output)
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
	modified := strings.Replace(string(data), `"default"`, `"imported"`, 1)
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

func normalizeTestPath(path string) string {
	cleaned := filepath.Clean(path)
	return strings.ReplaceAll(cleaned, "/private/var/", "/var/")
}

func setupLocalCLI(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("TICKET_URL", "file://"+filepath.Join(tempDir, "ticket.db"))
	t.Setenv("TICKET_CONFIG_DIR", tempDir)
	if err := runInitDB([]string{"-password", "secret"}); err != nil {
		t.Fatalf("runInitDB() error = %v", err)
	}
}

func createLocalTask(t *testing.T, args []string) int64 {
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
	id, err := parseTicketReferenceToID(lines[0])
	if err != nil {
		t.Fatalf("ParseInt(%q) error = %v", lines[0], err)
	}
	return id
}

func ticketLabelByID(t *testing.T, id int64) string {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	svc, err := resolveService(cfg)
	if err != nil {
		t.Fatalf("resolveService() error = %v", err)
	}
	task, err := svc.GetTicket(strconv.FormatInt(id, 10))
	if err != nil {
		t.Fatalf("svc.GetTicket(%d) error = %v", id, err)
	}
	if task.Key != "" {
		return task.Key
	}
	return strconv.FormatInt(id, 10)
}

func clearCurrentEpicID(t *testing.T) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	cfg.CurrentEpicID = 0
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
	return svc.GetTicket(ref)
}

func parseTicketReferenceToID(ref string) (int64, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, errors.New("empty ticket reference")
	}
	cfg, err := config.Load()
	if err != nil {
		return 0, err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return 0, err
	}
	task, err := svc.GetTicket(ref)
	if err != nil {
		return 0, err
	}
	return task.ID, nil
}
