package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/internal/testutil"
)

func TestResolveAnalyseCommandArgsDefaultsToCodexExec(t *testing.T) {
	t.Setenv("TICKET_ANALYSE_CMD", "")
	got := resolveAnalyseCommandArgs()
	want := []string{"codex", "exec"}
	if len(got) != len(want) {
		t.Fatalf("resolveAnalyseCommandArgs() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("resolveAnalyseCommandArgs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func seededAnalyseDBPath(t *testing.T) string {
	t.Helper()
	return testutil.SeededDBPath(t, "password")
}

func TestResolveAnalyseCommandArgsInjectsExecForCodexFlags(t *testing.T) {
	t.Setenv("TICKET_ANALYSE_CMD", "codex --model gpt-5.3-codex")
	got := resolveAnalyseCommandArgs()
	want := []string{"codex", "exec", "--model", "gpt-5.3-codex"}
	if len(got) != len(want) {
		t.Fatalf("resolveAnalyseCommandArgs() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("resolveAnalyseCommandArgs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildStoryAnalyseCLIInstructionsContainsTicketCommands(t *testing.T) {
	t.Parallel()
	story := store.Story{
		ID:          7,
		ProjectID:   3,
		Title:       "Checkout upgrade",
		Description: "Improve checkout journey",
	}
	project := store.Project{
		ID:     3,
		Prefix: "APP",
		Title:  "App",
	}
	role := store.Role{
		Title:              "StoryReview",
		Description:        "Find coherent breakdowns.",
		AcceptanceCriteria: "Produce actionable tickets.",
	}
	prompt := buildStoryAnalyseCLIInstructions(story, project, role)
	for _, want := range []string{
		"You are role StoryReview",
		"Do not rely on TICKET_URL, TICKET_USERNAME, or TICKET_PASSWORD.",
		"Assume this environment is already configured for the correct backend.",
		"ticket create -project 3 -t epic",
		"ticket create -project 3 -t task -parent <epic-ref>",
		"Story:",
		"Checkout upgrade",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q in %q", want, prompt)
		}
	}
}

func TestRunStoryBreakdownViaTicketCLIUsesConfiguredCommand(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "analyse-ok.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\ncat >/dev/null\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(script) error = %v", err)
	}
	t.Setenv("TICKET_ANALYSE_CMD", scriptPath)

	dbPath := seededAnalyseDBPath(t)
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	project, err := store.GetProject(context.Background(), db, "1")
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if _, createRoleErr := store.CreateRole(context.Background(), db, nil, "StoryReview", "Break stories down.", "Create useful tickets."); createRoleErr != nil {
		t.Fatalf("CreateRole(StoryReview) error = %v", createRoleErr)
	}
	story, err := store.CreateStory(context.Background(), db, project.ID, "Checkout", "Improve checkout", "")
	if err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}

	if err := runStoryBreakdownViaTicketCLI(db, project, story); err != nil {
		t.Fatalf("runStoryBreakdownViaTicketCLI() error = %v", err)
	}
}

func TestRunRoleJSONAnalysisExtractsJSONFromCommandOutput(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "analyse-json.sh")
	script := `#!/bin/sh
cat >/dev/null
printf 'noise before {"tickets":[{"title":"Implement checkout","description":"Build flow"}]} noise after\n'
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(script) error = %v", err)
	}
	t.Setenv("TICKET_ANALYSE_CMD", scriptPath)

	dbPath := seededAnalyseDBPath(t)
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()
	if _, err := store.CreateRole(context.Background(), db, nil, "EpicReview", "Break epics down.", "Create implementation tasks."); err != nil {
		t.Fatalf("CreateRole(EpicReview) error = %v", err)
	}

	var result epicAnalysisResult
	if err := runRoleJSONAnalysis(db, "EpicReview", "Return tasks.", &result); err != nil {
		t.Fatalf("runRoleJSONAnalysis() error = %v", err)
	}
	if len(result.Tickets) != 1 || result.Tickets[0].Title != "Implement checkout" {
		t.Fatalf("analysis result = %#v", result)
	}
}

func TestRunRoleJSONAnalysisReturnsCommandAndJSONErrors(t *testing.T) {
	dbPath := seededAnalyseDBPath(t)
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()
	if _, err := store.CreateRole(context.Background(), db, nil, "EpicReview", "Break epics down.", "Create implementation tasks."); err != nil {
		t.Fatalf("CreateRole(EpicReview) error = %v", err)
	}

	failScript := filepath.Join(t.TempDir(), "analyse-fail.sh")
	if err := os.WriteFile(failScript, []byte("#!/bin/sh\ncat >/dev/null\nprintf 'nope' >&2\nexit 7\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(fail script) error = %v", err)
	}
	t.Setenv("TICKET_ANALYSE_CMD", failScript)
	var result epicAnalysisResult
	if err := runRoleJSONAnalysis(db, "EpicReview", "Return tasks.", &result); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("runRoleJSONAnalysis(command failure) error = %v, want stderr", err)
	}

	badJSONScript := filepath.Join(t.TempDir(), "analyse-bad-json.sh")
	if err := os.WriteFile(badJSONScript, []byte("#!/bin/sh\ncat >/dev/null\nprintf '{not-json}\\n'\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(bad json script) error = %v", err)
	}
	t.Setenv("TICKET_ANALYSE_CMD", badJSONScript)
	if err := runRoleJSONAnalysis(db, "EpicReview", "Return tasks.", &result); err == nil {
		t.Fatal("runRoleJSONAnalysis(bad JSON) error = nil, want JSON error")
	}

	emptyScript := filepath.Join(t.TempDir(), "analyse-empty.sh")
	if err := os.WriteFile(emptyScript, []byte("#!/bin/sh\ncat >/dev/null\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(empty script) error = %v", err)
	}
	t.Setenv("TICKET_ANALYSE_CMD", emptyScript)
	if err := runRoleJSONAnalysis(db, "EpicReview", "Return tasks.", &result); err == nil || !strings.Contains(err.Error(), "empty analysis output") {
		t.Fatalf("runRoleJSONAnalysis(empty output) error = %v, want empty output error", err)
	}
}

func TestFallbackAnalysisUsesDefaultTitlesWhenInputTitleBlank(t *testing.T) {
	t.Parallel()

	storyResult := fallbackStoryAnalysis(store.Story{})
	if len(storyResult.Epics) != 1 || !strings.Contains(storyResult.Epics[0].Title, "Story Breakdown") {
		t.Fatalf("fallbackStoryAnalysis() = %#v", storyResult)
	}

	epicResult := fallbackEpicAnalysis(store.Ticket{})
	if len(epicResult.Tickets) != 3 || !strings.Contains(epicResult.Tickets[0].Title, "Epic") {
		t.Fatalf("fallbackEpicAnalysis() = %#v", epicResult)
	}
}
