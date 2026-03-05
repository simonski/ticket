package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBuildAgentCommandRequiresArgs(t *testing.T) {
	t.Parallel()

	if _, err := buildAgentCommand(nil); err == nil {
		t.Fatal("expected an error for missing args")
	}
}

func TestBuildAgentCommandUsesShellForSingleArgument(t *testing.T) {
	t.Parallel()

	cmd, err := buildAgentCommand([]string{"codex --help"})
	if err != nil {
		t.Fatalf("buildAgentCommand returned error: %v", err)
	}

	gotArgs := cmd.Args
	wantArgs := []string{"sh", "-c", "codex --help"}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("Args len = %d, want %d (%v)", len(gotArgs), len(wantArgs), gotArgs)
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Fatalf("Args[%d] = %q, want %q (all args: %v)", i, gotArgs[i], wantArgs[i], gotArgs)
		}
	}
}

func TestBuildAgentCommandUsesDirectExecForMultipleArguments(t *testing.T) {
	t.Parallel()

	cmd, err := buildAgentCommand([]string{"codex", "--help"})
	if err != nil {
		t.Fatalf("buildAgentCommand returned error: %v", err)
	}

	gotArgs := cmd.Args
	wantArgs := []string{"codex", "--help"}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("Args len = %d, want %d (%v)", len(gotArgs), len(wantArgs), gotArgs)
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Fatalf("Args[%d] = %q, want %q (all args: %v)", i, gotArgs[i], wantArgs[i], gotArgs)
		}
	}
}

func TestRunLoopDryRunFlagRequiresInteger(t *testing.T) {
	t.Parallel()

	err := runLoop([]string{"-name", "ralph", "-dryrun"})
	if err == nil {
		t.Fatal("runLoop() error = nil, want non-nil")
	}
}

func TestDryRunCommandUsesProvidedSeconds(t *testing.T) {
	t.Parallel()

	got := dryRunCommand(7)
	want := "echo 'dry-run; sleep 7'"
	if got != want {
		t.Fatalf("dryRunCommand() = %q, want %q", got, want)
	}
}

func TestRenderPromptUsesDefaultTemplate(t *testing.T) {
	t.Setenv("WIGGUM_PROMPT_TEMPLATE", "")

	got := renderPrompt("BEAD")
	want := "Perform the following:\nBEAD"
	if got != want {
		t.Fatalf("renderPrompt() = %q, want %q", got, want)
	}
}

func TestRenderPromptUsesTemplatePlaceholder(t *testing.T) {
	t.Setenv("WIGGUM_PROMPT_TEMPLATE", "Please do this:\n<BEAD>\nThanks")

	got := renderPrompt("BEAD")
	want := "Please do this:\nBEAD\nThanks"
	if got != want {
		t.Fatalf("renderPrompt() = %q, want %q", got, want)
	}
}

func TestSplitAcceptanceUsesNewlines(t *testing.T) {
	t.Parallel()

	got := splitAcceptance("first criterion\nsecond criterion")
	if len(got) != 2 || got[0] != "first criterion" || got[1] != "second criterion" {
		t.Fatalf("splitAcceptance() = %#v", got)
	}
}

func TestSplitAcceptanceStillToleratesPipes(t *testing.T) {
	t.Parallel()

	got := splitAcceptance("first criterion|second criterion")
	if len(got) != 2 || got[0] != "first criterion" || got[1] != "second criterion" {
		t.Fatalf("splitAcceptance() = %#v", got)
	}
}

func TestRenderWorkPacketIsPromptWrapped(t *testing.T) {
	t.Setenv("WIGGUM_PROMPT_TEMPLATE", "")

	item := issue{
		ID:          "bd-123",
		Title:       "Tidy parser help",
		Description: "Update the help output",
		Acceptance:  "help prints usage\ntests pass",
		Status:      "open",
		Priority:    2,
		IssueType:   "task",
	}

	got := renderPrompt(renderWorkPacket(item, "ralph", "feature/ralph/tidy-parser-help", false, false, false))
	if !strings.HasPrefix(got, "Perform the following:\nWiggum: ralph\n") {
		t.Fatalf("wrapped packet missing default prompt prefix: %q", got)
	}
	if !strings.Contains(got, "ID: bd-123\n") {
		t.Fatalf("wrapped packet missing bead contents: %q", got)
	}
}

func TestBranchNameForUsesFeaturePrefix(t *testing.T) {
	t.Parallel()

	item := issue{
		ID:          "bd-123",
		Title:       "Tidy parser help",
		ExternalRef: "e2-s2",
	}

	got := branchNameFor("ralph", item)
	want := "feature/ralph/e2-s2-tidy-parser-help"
	if got != want {
		t.Fatalf("branchNameFor() = %q, want %q", got, want)
	}
}

func TestWorkDirForSanitizesPathLikeCharacters(t *testing.T) {
	t.Parallel()

	got := workDirFor(issue{ID: "bd/123", Title: "Fix: thing"})
	want := filepath.Join("logs", "bd-123-Fix-thing")
	if got != want {
		t.Fatalf("workDirFor() = %q, want %q", got, want)
	}
}

func TestWriteWorkArtifactsIncludesExpectedFiles(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	item := issue{ID: "bd/123", Title: "Fix: thing"}
	workDir := workDirFor(item)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", workDir, err)
	}

	startedAt := "2026-02-28T11:00:00Z"
	completedAt := "2026-02-28T11:02:00Z"
	if err := writeWorkArtifacts(
		workDir,
		item,
		"feature/ralph/fix-thing",
		"codex exec - < logs/bd-123-Fix-thing/prompt.md",
		"agent output\n",
		mustParseRFC3339(t, startedAt),
		mustParseRFC3339(t, completedAt),
		7,
		"in_progress",
		"closed",
	); err != nil {
		t.Fatalf("writeWorkArtifacts() error = %v", err)
	}

	outputPath := filepath.Join(tempDir, workDir, "output.md")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", outputPath, err)
	}
	if got := string(data); got != "agent output\n" {
		t.Fatalf("output.md = %q, want %q", got, "agent output\n")
	}

	statusPath := filepath.Join(tempDir, workDir, "status.json")
	statusBytes, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", statusPath, err)
	}

	var got workStatus
	if err := json.Unmarshal(statusBytes, &got); err != nil {
		t.Fatalf("json.Unmarshal(status.json) error = %v", err)
	}

	if got.StartedAt != startedAt || got.CompletedAt != completedAt || got.ExitCode != 7 || got.Branch != "feature/ralph/fix-thing" || got.BeadID != "bd/123" || got.Title != "Fix: thing" || got.Instruction != "codex exec - < logs/bd-123-Fix-thing/prompt.md" || got.BeadStatusStart != "in_progress" || got.BeadStatusEnd != "closed" {
		t.Fatalf("status.json = %+v", got)
	}
}

func TestPerformWorkWritesArtifactsOnWorkerFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	t.Setenv("WIGGUM_PROMPT_TEMPLATE", "")
	installFakeCodex(t, tempDir, 7, "agent says hi\n")

	item := issue{
		ID:          "bd-123",
		Title:       "Tidy parser help",
		Description: "Update the help output",
		Acceptance:  "help prints usage\ntests pass",
		Status:      "open",
		Priority:    2,
		IssueType:   "task",
	}

	err := performWork(item, "ralph", "feature/ralph/tidy-parser-help", true)
	if err == nil {
		t.Fatal("performWork() error = nil, want non-nil")
	}

	workDir := filepath.Join(tempDir, workDirFor(item))
	outputPath := filepath.Join(workDir, "output.md")
	data, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", outputPath, readErr)
	}

	got := string(data)
	if !strings.Contains(got, "agent says hi\n") {
		t.Fatalf("output missing worker output: %q", got)
	}

	promptBytes, readErr := os.ReadFile(filepath.Join(workDir, "prompt.md"))
	if readErr != nil {
		t.Fatalf("ReadFile(prompt.md) error = %v", readErr)
	}
	if !strings.Contains(string(promptBytes), "Perform the following:\nWiggum: ralph\n") {
		t.Fatalf("prompt missing prompt contents: %q", string(promptBytes))
	}

	statusBytes, readErr := os.ReadFile(filepath.Join(workDir, "status.json"))
	if readErr != nil {
		t.Fatalf("ReadFile(status.json) error = %v", readErr)
	}
	var status workStatus
	if err := json.Unmarshal(statusBytes, &status); err != nil {
		t.Fatalf("json.Unmarshal(status.json) error = %v", err)
	}
	if status.ExitCode != 7 {
		t.Fatalf("status exit code = %d, want 7", status.ExitCode)
	}
	if status.Instruction != "codex exec - < logs/bd-123-Tidy-parser-help/prompt.md" {
		t.Fatalf("status instruction = %q", status.Instruction)
	}
	if status.Branch != "feature/ralph/tidy-parser-help" {
		t.Fatalf("status branch = %q", status.Branch)
	}
	if status.BeadStatusStart != "open" || status.BeadStatusEnd != "open" {
		t.Fatalf("status bead statuses = %q -> %q", status.BeadStatusStart, status.BeadStatusEnd)
	}
}

func TestPerformDryRunWorkWritesArtifacts(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	t.Setenv("WIGGUM_PROMPT_TEMPLATE", "")

	item := issue{
		ID:          "bd-456",
		Title:       "Simulate parser help",
		Description: "Update the help output",
		Acceptance:  "help prints usage\ntests pass",
		Status:      "open",
		Priority:    2,
		IssueType:   "task",
	}

	if err := performDryRunWork(item, "jane", "feature/jane/simulate-parser-help", false, 7); err != nil {
		t.Fatalf("performDryRunWork() error = %v", err)
	}

	workDir := filepath.Join(tempDir, workDirFor(item))
	data, err := os.ReadFile(filepath.Join(workDir, "output.md"))
	if err != nil {
		t.Fatalf("ReadFile(output.md) error = %v", err)
	}

	got := string(data)
	if !strings.Contains(got, "dry-run; sleep 7\n") {
		t.Fatalf("output missing dry-run process output: %q", got)
	}

	inputBytes, err := os.ReadFile(filepath.Join(workDir, "input.md"))
	if err != nil {
		t.Fatalf("ReadFile(input.md) error = %v", err)
	}
	if !strings.Contains(string(inputBytes), "Dry Run: true\n") {
		t.Fatalf("input missing dry-run packet contents: %q", string(inputBytes))
	}

	statusBytes, err := os.ReadFile(filepath.Join(workDir, "status.json"))
	if err != nil {
		t.Fatalf("ReadFile(status.json) error = %v", err)
	}
	var status workStatus
	if err := json.Unmarshal(statusBytes, &status); err != nil {
		t.Fatalf("json.Unmarshal(status.json) error = %v", err)
	}
	if status.ExitCode != 0 {
		t.Fatalf("status exit code = %d, want 0", status.ExitCode)
	}
	if status.Branch != "feature/jane/simulate-parser-help" {
		t.Fatalf("status branch = %q", status.Branch)
	}
	if status.Instruction != "echo 'dry-run; sleep 7'" {
		t.Fatalf("status instruction = %q", status.Instruction)
	}
	if status.BeadStatusStart != "open" || status.BeadStatusEnd != "open" {
		t.Fatalf("status bead statuses = %q -> %q", status.BeadStatusStart, status.BeadStatusEnd)
	}
}

func TestUpdateWorkStatusEndRewritesStatus(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	item := issue{ID: "bd-123", Title: "Fix: thing"}
	workDir := workDirFor(item)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", workDir, err)
	}

	if err := writeWorkArtifacts(
		workDir,
		item,
		"feature/ralph/fix-thing",
		"codex exec - < logs/bd-123-Fix-thing/prompt.md",
		"agent output\n",
		mustParseRFC3339(t, "2026-02-28T11:00:00Z"),
		mustParseRFC3339(t, "2026-02-28T11:02:00Z"),
		0,
		"in_progress",
		"in_progress",
	); err != nil {
		t.Fatalf("writeWorkArtifacts() error = %v", err)
	}

	if err := updateWorkStatusEnd(item, "feature/ralph/fix-thing", "closed"); err != nil {
		t.Fatalf("updateWorkStatusEnd() error = %v", err)
	}

	statusBytes, err := os.ReadFile(filepath.Join(workDir, "status.json"))
	if err != nil {
		t.Fatalf("ReadFile(status.json) error = %v", err)
	}

	var status workStatus
	if err := json.Unmarshal(statusBytes, &status); err != nil {
		t.Fatalf("json.Unmarshal(status.json) error = %v", err)
	}
	if status.BeadStatusStart != "in_progress" || status.BeadStatusEnd != "closed" {
		t.Fatalf("status bead statuses = %q -> %q", status.BeadStatusStart, status.BeadStatusEnd)
	}
}

func installFakeCodex(t *testing.T, dir string, exitCode int, output string) {
	t.Helper()

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", binDir, err)
	}

	scriptPath := filepath.Join(binDir, "codex")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" != \"exec\" ] || [ \"$2\" != \"-\" ]; then\n" +
		"  echo \"unexpected args: $*\" >&2\n" +
		"  exit 99\n" +
		"fi\n" +
		"cat >/dev/null\n" +
		"printf '%s' " + shellQuote(output) + "\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", scriptPath, err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func mustParseRFC3339(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", value, err)
	}
	return parsed
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
