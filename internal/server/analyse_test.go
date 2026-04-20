package server

import (
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/store"
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

func TestStoryAnalyseProcessEnvDefaults(t *testing.T) {
	t.Setenv("TICKET_URL", "")
	t.Setenv("TICKET_USERNAME", "")
	t.Setenv("TICKET_PASSWORD", "")
	env := strings.Join(storyAnalyseProcessEnv(), "\n")
	for _, want := range []string{
		"TICKET_URL=https://ticket.exe.xyz",
		"TICKET_USERNAME=admin",
		"TICKET_PASSWORD=password",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("storyAnalyseProcessEnv() missing %q in %q", want, env)
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
		"ticket login -url \"$TICKET_URL\" -username \"$TICKET_USERNAME\" -password \"$TICKET_PASSWORD\"",
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
