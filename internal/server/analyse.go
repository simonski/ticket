package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

type storyAnalysisTask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type storyAnalysisEpic struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Tasks       []storyAnalysisTask `json:"tasks"`
}

type storyAnalysisResult struct {
	Epics []storyAnalysisEpic `json:"epics"`
}

type epicAnalysisResult struct {
	Tickets []storyAnalysisTask `json:"tickets"`
}

func analyseCommandLine() string {
	if raw := strings.TrimSpace(os.Getenv("TICKET_ANALYSE_CMD")); raw != "" {
		return raw
	}
	return "codex exec"
}

func resolveAnalyseCommandArgs() []string {
	raw := strings.TrimSpace(analyseCommandLine())
	if raw == "" {
		return []string{"codex", "exec"}
	}
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return []string{"codex", "exec"}
	}
	if strings.EqualFold(parts[0], "codex") && (len(parts) == 1 || strings.HasPrefix(parts[1], "-")) {
		return append([]string{parts[0], "exec"}, parts[1:]...)
	}
	return parts
}

func storyAnalyseProcessEnv() []string {
	url := "http://localhost:8080"
	if r, err := config.ResolveURL(); err == nil && r.Mode == config.ModeRemote {
		url = r.ServerURL
	}
	username := strings.TrimSpace(os.Getenv("TICKET_USERNAME"))
	if username == "" {
		username = "admin"
	}
	password := strings.TrimSpace(os.Getenv("TICKET_PASSWORD"))
	if password == "" {
		password = "password"
	}
	env := append([]string{}, os.Environ()...)
	env = append(env,
		"TICKET_URL="+url,
		"TICKET_USERNAME="+username,
		"TICKET_PASSWORD="+password,
	)
	return env
}

func buildStoryAnalyseCLIInstructions(story store.Story, project store.Project, role store.Role) string {
	projectRef := fmt.Sprintf("%d", project.ID)
	return fmt.Sprintf(
		`You are role %s.
Description:
%s

AcceptanceCriteria:
%s

Break down the following story into implementation epics and tasks using the local "ticket" CLI binary.

Story:
- id: %d
- title: %s
- description: %s

Project:
- id: %d
- prefix: %s
- title: %s

Requirements:
1) Run non-interactively.
2) Use environment variables already provided:
   TICKET_URL
   TICKET_USERNAME
   TICKET_PASSWORD
3) Login first:
   ticket login -url "$TICKET_URL" -username "$TICKET_USERNAME" -password "$TICKET_PASSWORD"
4) Create 1-4 epics in this project using:
   ticket create -project %s -t epic -title "<epic title>" -d "<epic description>"
5) For each epic, create 2-5 tasks in this project associated to that epic using:
   ticket create -project %s -t task -parent <epic-ref> -title "<task title>" -d "<task description>"
6) Keep titles concrete and delivery-focused.
7) Print a short final summary of created epic/task refs.
`,
		role.Title,
		role.Description,
		role.AcceptanceCriteria,
		story.ID,
		strings.TrimSpace(story.Title),
		strings.TrimSpace(story.Description),
		project.ID,
		strings.TrimSpace(project.Prefix),
		strings.TrimSpace(project.Title),
		projectRef,
		projectRef,
	)
}

func runStoryBreakdownViaTicketCLI(db *sql.DB, project store.Project, story store.Story) error {
	role, err := store.GetRoleByTitle(context.Background(), db, "StoryReview")
	if err != nil {
		return err
	}
	prompt := buildStoryAnalyseCLIInstructions(story, project, role)
	commandArgs := resolveAnalyseCommandArgs()
	if len(commandArgs) == 0 {
		return errors.New("analysis command is empty")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, commandArgs[0], commandArgs[1:]...) // #nosec G204 -- commandArgs is resolved from trusted server configuration, not user input
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := stdin.Write([]byte(prompt + "\n")); err != nil {
		if closeErr := stdin.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	return nil
}

func runRoleJSONAnalysis(db *sql.DB, roleTitle, prompt string, target any) error {
	role, err := store.GetRoleByTitle(context.Background(), db, roleTitle)
	if err != nil {
		return err
	}
	fullPrompt := fmt.Sprintf(
		"You are role %s.\nDescription:\n%s\n\nAcceptanceCriteria:\n%s\n\nReturn JSON only.\n%s\n",
		role.Title,
		role.Description,
		role.AcceptanceCriteria,
		prompt,
	)

	commandArgs := resolveAnalyseCommandArgs()
	if len(commandArgs) == 0 {
		return errors.New("analysis command is empty")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, commandArgs[0], commandArgs[1:]...) // #nosec G204 -- commandArgs from trusted server config
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := stdin.Write([]byte(fullPrompt + "\n")); err != nil {
		if closeErr := stdin.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return errors.New("empty analysis output")
	}
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start >= 0 && end > start {
		output = output[start : end+1]
	}
	if err := json.Unmarshal([]byte(output), target); err != nil {
		return err
	}
	return nil
}

func fallbackStoryAnalysis(story store.Story) storyAnalysisResult {
	epicTitle := strings.TrimSpace(story.Title)
	if epicTitle == "" {
		epicTitle = "Story Breakdown"
	}
	return storyAnalysisResult{
		Epics: []storyAnalysisEpic{
			{
				Title:       epicTitle + " Core Delivery",
				Description: "Core implementation stream for the story.",
				Tasks: []storyAnalysisTask{
					{Title: "Define implementation plan", Description: "Break down technical approach and dependencies."},
					{Title: "Implement core flow", Description: "Build primary user and system behavior for the story."},
					{Title: "Validate and review", Description: "Test outcomes and prepare story for stakeholder review."},
				},
			},
		},
	}
}

func fallbackEpicAnalysis(epic store.Ticket) epicAnalysisResult {
	base := strings.TrimSpace(epic.Title)
	if base == "" {
		base = "Epic"
	}
	return epicAnalysisResult{
		Tickets: []storyAnalysisTask{
			{Title: base + " - implementation", Description: "Implement primary functionality for the epic."},
			{Title: base + " - test coverage", Description: "Add tests and validation for epic behavior."},
			{Title: base + " - operational polish", Description: "Finalize docs, edge cases, and release readiness."},
		},
	}
}
