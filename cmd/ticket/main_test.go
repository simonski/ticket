package main

import (
	"bytes"
	"encoding/json"
	"errors"
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
		"ADMIN COMMANDS",
		"initdb",
		"server",
		"version",
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
		"  add",
		"  active",
		"  claim",
		"  clone",
		"  comment",
		"  complete",
		"  count",
		"  design",
		"  dependency",
		"  delete",
		"  develop",
		"  done",
		"  get",
		"  help",
		"  health",
		"  idle",
		"  list",
		"  login",
		"  logout",
		"  onboard",
		"  orphans",
		"  project",
		"  register",
		"\n  ticket          ",
		"  request",
		"  request-dryrun",
		"  search",
		"  set-parent",
		"  status",
		"  test",
		"  unset-parent",
		"  unclaim",
		"  update",
		"  version",
	}
	last := -1
	for _, item := range clientOrder {
		idx := strings.Index(usage, item)
		if idx == -1 {
			t.Fatalf("root usage missing ordered client command %q:\n%s", item, usage)
		}
		if idx <= last {
			t.Fatalf("root usage client commands not alphabetical around %q:\n%s", item, usage)
		}
		last = idx
	}

	adminOrder := []string{"  assign", "  initdb", "  server", "  unassign", "  user"}
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
		if err := run([]string{"comment", "add", strconv.FormatInt(taskID, 10), "Reviewer approved this ticket."}); err != nil {
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
	if err := run([]string{"comment", "add", strconv.FormatInt(secondID, 10), "Approved by reviewer"}); err != nil {
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
	help := renderCommandHelp("initdb")

	for _, want := range []string{
		"USAGE",
		"ticket initdb",
		"DETAILS",
		"EXAMPLE",
		"ticket initdb -f $TICKET_HOME/ticket.db --force -password secret",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("command help missing %q:\n%s", want, help)
		}
	}
}

func TestRunOnboardAppendsEmbeddedAgentsTemplate(t *testing.T) {
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

	target := filepath.Join(tempDir, "AGENTS.md")
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatalf("WriteFile(existing AGENTS.md) error = %v", err)
	}

	if err := runOnboard(nil); err != nil {
		t.Fatalf("runOnboard() error = %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "existing\n# Agent Instructions") {
		t.Fatalf("runOnboard() did not append embedded template correctly:\n%s", content)
	}
}

func TestRenderServerHelpIncludesTaskHomeDefault(t *testing.T) {
	help := renderCommandHelp("server")
	for _, want := range []string{
		"ticket server [-f <db-path>] [-addr :8080]",
		"If `-f` is omitted, the server uses `$TICKET_HOME/ticket.db`.",
		"ticket server -f $TICKET_HOME/ticket.db -addr :8080",
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
	t.Setenv("TICKET_MODE", "remote")

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

	t.Setenv("TICKET_MODE", "local")
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

func TestRunInitDBGeneratesPasswordWhenMissing(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
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

func TestRunInitDBUsesTaskHomeWhenFIsOmitted(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	if err := runInitDB([]string{"-password", "secret"}); err != nil {
		t.Fatalf("runInitDB() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "ticket.db")); err != nil {
		t.Fatalf("expected default db at TICKET_HOME/ticket.db: %v", err)
	}
}

func TestRunInitDBForceOverwritesExistingDatabase(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
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
	t.Setenv("TICKET_HOME", tempDir)
	credsPath := filepath.Join(tempDir, "credentials.json")
	t.Setenv("TICKET_MODE", "remote")
	t.Setenv("TICKET_SERVER", "")
	t.Setenv("TICKET_SERVER", "")

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
	t.Setenv("TICKET_SERVER", server.URL)
	t.Setenv("TICKET_URL", "")

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
	t.Setenv("TICKET_HOME", tempDir)
	t.Setenv("TICKET_MODE", "remote")
	t.Setenv("TICKET_SERVER", "")
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
	t.Setenv("TICKET_SERVER", server.URL)
	t.Setenv("TICKET_URL", "")

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
	t.Setenv("TICKET_MODE", "remote")
	t.Setenv("TICKET_HOME", t.TempDir())
	t.Setenv("TICKET_SERVER", "")
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
	t.Setenv("TICKET_SERVER", server.URL)

	output := captureStdout(t, func() {
		if err := runStatus(nil); err != nil {
			t.Fatalf("runStatus(remote) error = %v", err)
		}
	})
	for _, want := range []string{
		"mode: remote",
		"server: " + server.URL,
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
	t.Setenv("TICKET_MODE", "local")
	t.Setenv("TICKET_HOME", tempDir)

	var runErr error
	output := captureStdout(t, func() {
		runErr = runStatus(nil)
	})
	if !errors.Is(runErr, os.ErrNotExist) {
		t.Fatalf("runStatus(local missing) error = %v, want os.ErrNotExist", runErr)
	}
	expectedPath := normalizeTestPath(filepath.Join(tempDir, "ticket.db"))
	for _, want := range []string{
		"mode: local",
		"db_exists: false",
		"failure",
		"hint: run ticket initdb",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus(local missing) missing %q:\n%s", want, output)
		}
	}
	if !strings.Contains(normalizeTestPath(output), "db_path: "+expectedPath) {
		t.Fatalf("runStatus(local missing) output missing db_path %q:\n%s", expectedPath, output)
	}
}

func TestRunStatusLocalSuccess(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_MODE", "local")
	t.Setenv("TICKET_HOME", tempDir)
	if err := runInitDB([]string{"-password", "secret"}); err != nil {
		t.Fatalf("runInitDB() error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"status", "-nocolor"}); err != nil {
			t.Fatalf("runStatus(local) error = %v", err)
		}
	})
	expectedPath := normalizeTestPath(filepath.Join(tempDir, "ticket.db"))
	for _, want := range []string{
		"mode: local",
		"db_exists: true",
		"connection: success",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus(local) missing %q:\n%s", want, output)
		}
	}
	if !strings.Contains(normalizeTestPath(output), "db_path: "+expectedPath) {
		t.Fatalf("runStatus(local) output missing db_path %q:\n%s", expectedPath, output)
	}
}

func TestPrintTaskDetailsIncludesAcceptanceCriteria(t *testing.T) {
	output := captureStdout(t, func() {
		printTicketDetails(store.Ticket{
			ID:                 42,
			Title:              "Example Task",
			Type:               "task",
			Status:             "design/idle",
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
		}, nil)
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
		"Priority     : 1",
		"Created      : 2026-03-01 12:00:00",
		"LastModified : 2026-03-02 09:30:00",
		"Acceptance Criteria : - does the thing",
		"Comments     :",
		"[2026-03-02 10:00:00] alice: latest comment",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("printTicketDetails() missing %q:\n%s", want, output)
		}
	}
}

func TestRunProjectCommandsInLocalMode(t *testing.T) {
	setupLocalCLI(t)

	createOutput := captureStdout(t, func() {
		if err := run([]string{"project", "create", "-prefix", "PRA", "-description", "Desc", "-ac", "AC", "Project A"}); err != nil {
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
	if !strings.Contains(listOutput, "Project A") || !strings.Contains(listOutput, "(current)") {
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

func TestRunListStatusRenderingSupportsUnicodeAndPlainModes(t *testing.T) {
	setupLocalCLI(t)

	openID := createLocalTask(t, []string{"add", "Moon Open Task"})
	inProgressID := createLocalTask(t, []string{"add", "Moon Inprogress Task"})
	completeID := createLocalTask(t, []string{"add", "Moon Complete Task"})
	if err := run([]string{"develop", strconv.FormatInt(inProgressID, 10)}); err != nil {
		t.Fatalf("develop error = %v", err)
	}
	if err := run([]string{"active", strconv.FormatInt(inProgressID, 10)}); err != nil {
		t.Fatalf("active error = %v", err)
	}
	if err := run([]string{"complete", strconv.FormatInt(completeID, 10)}); err != nil {
		t.Fatalf("complete error = %v", err)
	}

	if err := run([]string{"develop", strconv.FormatInt(openID, 10)}); err != nil {
		t.Fatalf("develop open task error = %v", err)
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
	checkRow("◑", "develop/active")
	checkRow("◉", "design/complete")
	checkRow("○", "develop/idle")

	for _, want := range []string{"develop/active", "develop/idle", "design/complete"} {
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
	if err := run([]string{"develop", strconv.FormatInt(taskID, 10)}); err != nil {
		t.Fatalf("develop task alpha error = %v", err)
	}
	if err := run([]string{"develop", strconv.FormatInt(depID, 10)}); err != nil {
		t.Fatalf("develop task beta error = %v", err)
	}
	if err := run([]string{"comment", "add", strconv.FormatInt(taskID, 10), "latest note"}); err != nil {
		t.Fatalf("comment add error = %v", err)
	}

	getOutput := captureStdout(t, func() {
		if err := run([]string{"get", strconv.FormatInt(taskID, 10)}); err != nil {
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
		if err := run([]string{"dependency", "add", strconv.FormatInt(taskID, 10), strconv.FormatInt(depID, 10)}); err != nil {
			t.Fatalf("dependency add error = %v", err)
		}
	})
	if !strings.Contains(dependencyOutput, "added dependencies") {
		t.Fatalf("dependency add output = %q", dependencyOutput)
	}

	listOutput := captureStdout(t, func() {
		if err := run([]string{"list", "--status", "develop/idle"}); err != nil {
			t.Fatalf("list error = %v", err)
		}
	})
	if !strings.Contains(listOutput, "Ticket Alpha") || !strings.Contains(listOutput, "Ticket Beta") {
		t.Fatalf("list output = %q", listOutput)
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
		if err := run([]string{"comment", "add", strconv.FormatInt(taskID, 10), "hello"}); err != nil {
			t.Fatalf("comment error = %v", err)
		}
	})
	if !strings.Contains(commentOutput, "commented on") {
		t.Fatalf("comment output = %q", commentOutput)
	}

	setParentOutput := captureStdout(t, func() {
		if err := run([]string{"set-parent", strconv.FormatInt(depID, 10), strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("set-parent error = %v", err)
		}
	})
	if !strings.Contains(setParentOutput, "parent_id: "+strconv.FormatInt(taskID, 10)) {
		t.Fatalf("set-parent output = %q", setParentOutput)
	}

	unsetParentOutput := captureStdout(t, func() {
		if err := run([]string{"unset-parent", strconv.FormatInt(depID, 10)}); err != nil {
			t.Fatalf("unset-parent error = %v", err)
		}
	})
	if strings.Contains(unsetParentOutput, "parent_id:") {
		t.Fatalf("unset-parent output should not contain parent_id: %q", unsetParentOutput)
	}
}

func TestRunSearchSupportsFreeFormAndFilters(t *testing.T) {
	setupLocalCLI(t)

	if err := run([]string{"project", "create", "-prefix", "SEP", "Second Project"}); err != nil {
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

	if err := run([]string{"claim", strconv.FormatInt(matchingID, 10)}); err != nil {
		t.Fatalf("claim error = %v", err)
	}
	if err := run([]string{"update", strconv.FormatInt(matchingID, 10), "-stage", "develop", "-state", "active", "-priority", "4"}); err != nil {
		t.Fatalf("update matching task error = %v", err)
	}
	if err := run([]string{"update", strconv.FormatInt(otherID, 10), "-priority", "2"}); err != nil {
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
	if err := run([]string{"claim", strconv.FormatInt(taskID, 10)}); err != nil {
		t.Fatalf("claim error = %v", err)
	}

	updateOutput := captureStdout(t, func() {
		if err := run([]string{
			"update",
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
		if err := run([]string{"get", strconv.FormatInt(taskID, 10)}); err != nil {
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

	if err := run([]string{"update", strconv.FormatInt(taskID, 10), "-description", "updated description"}); err != nil {
		t.Fatalf("update with -description error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"get", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("get error = %v", err)
		}
	})
	if !strings.Contains(output, "Description  : updated description") {
		t.Fatalf("get output = %q", output)
	}
}

func TestRunTaskCreateSupportsInterspersedFlags(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "the", "thing", "-type", "epic"})

	output := captureStdout(t, func() {
		if err := run([]string{"get", strconv.FormatInt(taskID, 10)}); err != nil {
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
		if err := run([]string{"get", strconv.FormatInt(taskID, 10)}); err != nil {
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
		if err := run([]string{"get", strconv.FormatInt(taskID, 10)}); err != nil {
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
		if err := run([]string{"complete", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("complete error = %v", err)
		}
	})

	if !strings.Contains(output, "status: design/complete") {
		t.Fatalf("complete output = %q", output)
	}
}

func TestRunDeleteTicketInLocalMode(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "Delete me"})
	output := captureStdout(t, func() {
		if err := run([]string{"delete", strconv.FormatInt(taskID, 10)}); err != nil {
			t.Fatalf("delete error = %v", err)
		}
	})
	if !strings.Contains(output, "deleted ticket ") {
		t.Fatalf("delete output = %q", output)
	}
	if err := run([]string{"get", strconv.FormatInt(taskID, 10)}); err == nil || err.Error() != "ticket not found" {
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
	if err := run([]string{"rm", strconv.FormatInt(parentID, 10)}); err == nil || err.Error() != "ticket has child tickets" {
		t.Fatalf("delete parent error = %v, want ticket has child tickets", err)
	}
}

func TestRunGetJSONUsesCommentAuthorDateTextShape(t *testing.T) {
	setupLocalCLI(t)

	taskID := createLocalTask(t, []string{"add", "JSON Task"})
	if err := run([]string{"comment", "add", strconv.FormatInt(taskID, 10), "first"}); err != nil {
		t.Fatalf("comment add first error = %v", err)
	}
	if err := run([]string{"comment", "add", strconv.FormatInt(taskID, 10), "second"}); err != nil {
		t.Fatalf("comment add second error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := run([]string{"get", "-json", strconv.FormatInt(taskID, 10)}); err != nil {
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
	t.Setenv("TICKET_MODE", "local")

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
	t.Setenv("TICKET_MODE", "remote")
	t.Setenv("TICKET_SERVER", "http://127.0.0.1:1")
	t.Setenv("TICKET_URL", "")
	t.Setenv("TICKET_HOME", t.TempDir())

	var runErr error
	output := captureStdout(t, func() {
		runErr = runStatus(nil)
	})
	if runErr == nil {
		t.Fatal("runStatus(remote failure) error = nil")
	}
	for _, want := range []string{
		"mode: remote",
		"server: http://127.0.0.1:1",
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

func TestRunSetParentDisallowsEpicUnderTask(t *testing.T) {
	setupLocalCLI(t)
	epicID := createLocalTask(t, []string{"epic", "Orphan Epic"})
	taskID := createLocalTask(t, []string{"add", "Task Parent"})

	if err := run([]string{"set-parent", strconv.FormatInt(epicID, 10), strconv.FormatInt(taskID, 10)}); err == nil {
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
		{[]string{"get", "abc"}, "ticket not found"},
		{[]string{"dependency", "add", "1", "abc"}, "ticket not found"},
		{[]string{"request", "abc"}, "ticket not found"},
		{[]string{"project", "get"}, "usage: ticket project get <id>"},
		{[]string{"list", "-n", "-1"}, "usage: ticket list|ls"},
		{[]string{"comment", "add", "1"}, "usage: ticket comment add <id> \"comment\""},
		{[]string{"set-parent", "1", "abc"}, "ticket not found"},
		{[]string{"unset-parent", "abc"}, "ticket not found"},
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

func normalizeTestPath(path string) string {
	cleaned := filepath.Clean(path)
	return strings.ReplaceAll(cleaned, "/private/var/", "/var/")
}

func setupLocalCLI(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("TICKET_MODE", "local")
	t.Setenv("TICKET_HOME", tempDir)
	t.Setenv("TICKET_SERVER", "")
	t.Setenv("TICKET_SERVER", "")
	t.Setenv("TICKET_DB_OVERRIDE", "")
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
