package store

import (
	"context"
	"testing"
)

func TestGoalCRUD(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	project, err := CreateProject(context.Background(), db, "Goal Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	// Create a goal
	goal, err := CreateGoal(context.Background(), db, project.ID, "Launch v1", "Ship it", "some notes", "2026-06-01", 2)
	if err != nil {
		t.Fatalf("CreateGoal() error = %v", err)
	}
	if goal.ID == 0 {
		t.Fatal("CreateGoal() goal.ID = 0")
	}
	if goal.Title != "Launch v1" {
		t.Fatalf("CreateGoal().Title = %q, want Launch v1", goal.Title)
	}
	if goal.Priority != 2 {
		t.Fatalf("CreateGoal().Priority = %d, want 2", goal.Priority)
	}
	if goal.Status != "draft" {
		t.Fatalf("CreateGoal().Status = %q, want draft", goal.Status)
	}
	if goal.RefinedGoal != "" || goal.Decompose != "" {
		t.Fatalf("CreateGoal().refinement fields = %#v, want empty", goal)
	}
	if goal.RefinementConfirmed {
		t.Fatalf("CreateGoal().RefinementConfirmed = true, want false")
	}

	// Get the goal
	fetched, err := GetGoal(context.Background(), db, goal.ID)
	if err != nil {
		t.Fatalf("GetGoal() error = %v", err)
	}
	if fetched.Title != "Launch v1" {
		t.Fatalf("GetGoal().Title = %q, want Launch v1", fetched.Title)
	}

	// List goals
	goals, err := ListGoals(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("ListGoals() error = %v", err)
	}
	if len(goals) != 1 {
		t.Fatalf("ListGoals() len = %d, want 1", len(goals))
	}

	// Delete the goal
	if err := DeleteGoal(context.Background(), db, goal.ID); err != nil {
		t.Fatalf("DeleteGoal() error = %v", err)
	}

	// Verify deleted
	goals, err = ListGoals(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("ListGoals() after delete error = %v", err)
	}
	if len(goals) != 0 {
		t.Fatalf("ListGoals() after delete len = %d, want 0", len(goals))
	}
}

func TestCreateGoalEmptyTitle(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Goal Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateGoal(context.Background(), db, project.ID, "", "", "", "", 0); err == nil {
		t.Fatal("CreateGoal(empty title) error = nil, want error")
	}
}

func TestCreateGoalDefaultPriority(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Goal Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	goal, err := CreateGoal(context.Background(), db, project.ID, "Some goal", "", "", "", 0)
	if err != nil {
		t.Fatalf("CreateGoal() error = %v", err)
	}
	if goal.Priority != 1 {
		t.Fatalf("CreateGoal(priority=0).Priority = %d, want 1", goal.Priority)
	}
}

func TestDeleteGoalNotFound(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	if err := DeleteGoal(context.Background(), db, 999); err == nil {
		t.Fatal("DeleteGoal(nonexistent) error = nil, want error")
	}
}

func TestGoalUpdateAndStatus(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	project, err := CreateProject(context.Background(), db, "Goal Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	goal, err := CreateGoal(context.Background(), db, project.ID, "Launch v1", "Ship it", "some notes", "2026-06-01", 2)
	if err != nil {
		t.Fatalf("CreateGoal() error = %v", err)
	}

	updated, err := UpdateGoal(context.Background(), db, goal.ID, "Launch v2", "Ship more", "new notes", "2026-07-01", 3)
	if err != nil {
		t.Fatalf("UpdateGoal() error = %v", err)
	}
	if updated.Title != "Launch v2" || updated.Priority != 3 {
		t.Fatalf("UpdateGoal() = %+v, want updated title/priority", updated)
	}

	refining, err := SetGoalStatus(context.Background(), db, goal.ID, "refining")
	if err != nil {
		t.Fatalf("SetGoalStatus(refining) error = %v", err)
	}
	if refining.Status != "refining" {
		t.Fatalf("SetGoalStatus(refining).Status = %q, want refining", refining.Status)
	}

	if _, err := SetGoalStatus(context.Background(), db, goal.ID, "ready"); err == nil {
		t.Fatal("SetGoalStatus(ready) before refinement error = nil, want error")
	}

	if _, err := UpdateGoalRefinement(context.Background(), db, goal.ID, "Refined goal", "1. Objective: MVP\n2. Epic: Build\n3. Story: Deliver"); err != nil {
		t.Fatalf("UpdateGoalRefinement() error = %v", err)
	}

	if _, err := ConfirmGoalRefinement(context.Background(), db, goal.ID, true); err != nil {
		t.Fatalf("ConfirmGoalRefinement(true) error = %v", err)
	}

	ready, err := SetGoalStatus(context.Background(), db, goal.ID, "ready")
	if err != nil {
		t.Fatalf("SetGoalStatus(ready) error = %v", err)
	}
	if ready.Status != "ready" {
		t.Fatalf("SetGoalStatus(ready).Status = %q, want ready", ready.Status)
	}
}

func TestGoalRefinementAndChat(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	project, err := CreateProject(context.Background(), db, "Goal Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	goal, err := CreateGoal(context.Background(), db, project.ID, "Build app", "", "", "", 1)
	if err != nil {
		t.Fatalf("CreateGoal() error = %v", err)
	}

	refined, err := UpdateGoalRefinement(context.Background(), db, goal.ID,
		"Clean goal: deliver multiplayer todo app",
		"- Epic: Auth\n- Story: Registration page\n- Story: Realtime collaboration")
	if err != nil {
		t.Fatalf("UpdateGoalRefinement() error = %v", err)
	}
	if refined.RefinedGoal == "" || refined.Decompose == "" {
		t.Fatalf("UpdateGoalRefinement() = %+v, expected populated refinement fields", refined)
	}
	items, err := ListGoalDecompositionItems(context.Background(), db, goal.ID)
	if err != nil {
		t.Fatalf("ListGoalDecompositionItems() error = %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("ListGoalDecompositionItems() len=%d, want 3", len(items))
	}

	if _, err := AddGoalChatMessage(context.Background(), db, goal.ID, "user", "Please refine this goal"); err != nil {
		t.Fatalf("AddGoalChatMessage(user) error = %v", err)
	}
	if _, err := AddGoalChatMessage(context.Background(), db, goal.ID, "agent", "Here is a cleaner goal and decomposition"); err != nil {
		t.Fatalf("AddGoalChatMessage(agent) error = %v", err)
	}
	messages, err := ListGoalChatMessages(context.Background(), db, goal.ID, 50)
	if err != nil {
		t.Fatalf("ListGoalChatMessages() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("ListGoalChatMessages() len=%d, want 2", len(messages))
	}

	story, err := CreateStory(context.Background(), db, project.ID, "Story one", "desc", "")
	if err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}
	if err := LinkGoalToStory(context.Background(), db, goal.ID, story.ID); err != nil {
		t.Fatalf("LinkGoalToStory() error = %v", err)
	}
	stories, err := ListStoriesForGoal(context.Background(), db, goal.ID)
	if err != nil {
		t.Fatalf("ListStoriesForGoal() error = %v", err)
	}
	if len(stories) != 1 || stories[0].ID != story.ID {
		t.Fatalf("ListStoriesForGoal() = %#v, want linked story", stories)
	}
	if err := UnlinkGoalFromStory(context.Background(), db, goal.ID, story.ID); err != nil {
		t.Fatalf("UnlinkGoalFromStory() error = %v", err)
	}
}

func TestGoalDecompositionCRUDAndReorder(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Goal Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	goal, err := CreateGoal(context.Background(), db, project.ID, "Build app", "", "", "", 1)
	if err != nil {
		t.Fatalf("CreateGoal() error = %v", err)
	}
	objective, err := CreateGoalDecompositionItem(context.Background(), db, goal.ID, "objective", "Objective: MVP", nil)
	if err != nil {
		t.Fatalf("CreateGoalDecompositionItem(objective) error = %v", err)
	}
	epic, err := CreateGoalDecompositionItem(context.Background(), db, goal.ID, "epic", "Epic: Auth", nil)
	if err != nil {
		t.Fatalf("CreateGoalDecompositionItem(epic) error = %v", err)
	}
	story, err := CreateGoalDecompositionItem(context.Background(), db, goal.ID, "story", "Story: Login", nil)
	if err != nil {
		t.Fatalf("CreateGoalDecompositionItem(story) error = %v", err)
	}

	items, err := ListGoalDecompositionItems(context.Background(), db, goal.ID)
	if err != nil {
		t.Fatalf("ListGoalDecompositionItems() error = %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("ListGoalDecompositionItems() len=%d, want 3", len(items))
	}

	if err := ReorderGoalDecompositionItems(context.Background(), db, goal.ID, []int64{story.ID, objective.ID, epic.ID}); err != nil {
		t.Fatalf("ReorderGoalDecompositionItems() error = %v", err)
	}
	items, err = ListGoalDecompositionItems(context.Background(), db, goal.ID)
	if err != nil {
		t.Fatalf("ListGoalDecompositionItems() after reorder error = %v", err)
	}
	if items[0].ID != story.ID {
		t.Fatalf("first item after reorder = %d, want %d", items[0].ID, story.ID)
	}

	updated, err := UpdateGoalDecompositionItem(context.Background(), db, goal.ID, objective.ID, "objective", "Objective: MVP plus analytics")
	if err != nil {
		t.Fatalf("UpdateGoalDecompositionItem() error = %v", err)
	}
	if updated.Text != "Objective: MVP plus analytics" {
		t.Fatalf("UpdateGoalDecompositionItem().Text = %q", updated.Text)
	}

	if err := DeleteGoalDecompositionItem(context.Background(), db, goal.ID, epic.ID); err != nil {
		t.Fatalf("DeleteGoalDecompositionItem() error = %v", err)
	}
	items, err = ListGoalDecompositionItems(context.Background(), db, goal.ID)
	if err != nil {
		t.Fatalf("ListGoalDecompositionItems() after delete error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("ListGoalDecompositionItems() len=%d, want 2 after delete", len(items))
	}
}

func TestResolveGoalAgentModelConfigInheritance(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	if err := SetSystemAgentModelConfig(ctx, db, AgentModelConfig{
		Provider: "openai",
		Model:    "gpt-5.3-codex",
		URL:      "https://api.openai.com/v1",
		APIKey:   "system-key",
	}); err != nil {
		t.Fatalf("SetSystemAgentModelConfig() error = %v", err)
	}

	project, err := CreateProjectWithParams(ctx, db, ProjectCreateParams{
		Title:              "Agent Config Project",
		Prefix:             "AC",
		AgentModelProvider: "anthropic",
		AgentModelName:     "claude-sonnet-4.5",
		AgentModelURL:      "https://api.anthropic.com",
		AgentModelAPIKey:   "project-key",
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	goal, err := CreateGoal(ctx, db, project.ID, "Refine backlog", "", "", "", 1)
	if err != nil {
		t.Fatalf("CreateGoal() error = %v", err)
	}

	cfg, err := ResolveGoalAgentModelConfig(ctx, db, goal.ID)
	if err != nil {
		t.Fatalf("ResolveGoalAgentModelConfig() error = %v", err)
	}
	if cfg.Provider != "anthropic" || cfg.Model != "claude-sonnet-4.5" {
		t.Fatalf("project override not applied: %+v", cfg)
	}

	if _, err := SetGoalAgentModelConfig(ctx, db, goal.ID, AgentModelConfig{
		Provider: "openrouter",
		Model:    "openai/gpt-5",
		URL:      "https://openrouter.ai/api/v1",
		APIKey:   "goal-key",
	}); err != nil {
		t.Fatalf("SetGoalAgentModelConfig() error = %v", err)
	}

	cfg, err = ResolveGoalAgentModelConfig(ctx, db, goal.ID)
	if err != nil {
		t.Fatalf("ResolveGoalAgentModelConfig() error = %v", err)
	}
	if cfg.Provider != "openrouter" || cfg.Model != "openai/gpt-5" || cfg.APIKey != "goal-key" {
		t.Fatalf("goal override not applied: %+v", cfg)
	}
}

func TestGoalReadyRequiresConfirmationDepthAndResolvedClarifications(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Goal Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	goal, err := CreateGoal(context.Background(), db, project.ID, "Build app", "", "", "", 1)
	if err != nil {
		t.Fatalf("CreateGoal() error = %v", err)
	}
	if _, err := UpdateGoalRefinement(context.Background(), db, goal.ID, "Refined goal", "1. Objective: MVP\n2. Epic: Auth"); err != nil {
		t.Fatalf("UpdateGoalRefinement() error = %v", err)
	}
	if _, err := ConfirmGoalRefinement(context.Background(), db, goal.ID, true); err != nil {
		t.Fatalf("ConfirmGoalRefinement(true) error = %v", err)
	}
	if _, err := SetGoalStatus(context.Background(), db, goal.ID, "ready"); err == nil {
		t.Fatal("SetGoalStatus(ready) with depth<3 error = nil, want error")
	}
	if _, err := CreateGoalDecompositionItem(context.Background(), db, goal.ID, "story", "Story: Login", nil); err != nil {
		t.Fatalf("CreateGoalDecompositionItem(story) error = %v", err)
	}
	if _, err := ConfirmGoalRefinement(context.Background(), db, goal.ID, true); err != nil {
		t.Fatalf("ConfirmGoalRefinement(true) after decomposition edit error = %v", err)
	}
	clarification, err := AddGoalClarification(context.Background(), db, goal.ID, "Should guests be allowed?")
	if err != nil {
		t.Fatalf("AddGoalClarification() error = %v", err)
	}
	if _, err := SetGoalStatus(context.Background(), db, goal.ID, "ready"); err == nil {
		t.Fatal("SetGoalStatus(ready) with unresolved clarification error = nil, want error")
	}
	if _, err := SetGoalClarificationResolved(context.Background(), db, goal.ID, clarification.ID, true); err != nil {
		t.Fatalf("SetGoalClarificationResolved(true) error = %v", err)
	}
	ready, err := SetGoalStatus(context.Background(), db, goal.ID, "ready")
	if err != nil {
		t.Fatalf("SetGoalStatus(ready) final error = %v", err)
	}
	if ready.Status != "ready" {
		t.Fatalf("ready.Status = %q, want ready", ready.Status)
	}
}
