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
	return "codex"
}

func runRoleJSONAnalysis(db *sql.DB, roleTitle, prompt string, target any) error {
	role, err := store.GetRoleByTitle(db, roleTitle)
	if err != nil {
		return err
	}
	fullPrompt := fmt.Sprintf(
		"You are role %s.\nMotivation:\n%s\n\nGoals:\n%s\n\nReturn JSON only.\n%s\n",
		role.Title,
		role.Motivation,
		role.Goals,
		prompt,
	)

	shellPath := strings.TrimSpace(os.Getenv("SHELL"))
	if shellPath == "" {
		shellPath = "/bin/sh"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shellPath, "-lc", analyseCommandLine())
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
		_ = stdin.Close()
		return err
	}
	_ = stdin.Close()
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
