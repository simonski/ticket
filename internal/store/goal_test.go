package store

import "testing"

func TestGoalCRUD(t *testing.T) {
	db := testDB(t)

	project, err := CreateProject(db, "Goal Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	// Create a goal
	goal, err := CreateGoal(db, project.ID, "Launch v1", "Ship it", "some notes", "2026-06-01", 2)
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

	// Get the goal
	fetched, err := GetGoal(db, goal.ID)
	if err != nil {
		t.Fatalf("GetGoal() error = %v", err)
	}
	if fetched.Title != "Launch v1" {
		t.Fatalf("GetGoal().Title = %q, want Launch v1", fetched.Title)
	}

	// List goals
	goals, err := ListGoals(db, project.ID)
	if err != nil {
		t.Fatalf("ListGoals() error = %v", err)
	}
	if len(goals) != 1 {
		t.Fatalf("ListGoals() len = %d, want 1", len(goals))
	}

	// Delete the goal
	if err := DeleteGoal(db, goal.ID); err != nil {
		t.Fatalf("DeleteGoal() error = %v", err)
	}

	// Verify deleted
	goals, err = ListGoals(db, project.ID)
	if err != nil {
		t.Fatalf("ListGoals() after delete error = %v", err)
	}
	if len(goals) != 0 {
		t.Fatalf("ListGoals() after delete len = %d, want 0", len(goals))
	}
}

func TestCreateGoalEmptyTitle(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Goal Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateGoal(db, project.ID, "", "", "", "", 0); err == nil {
		t.Fatal("CreateGoal(empty title) error = nil, want error")
	}
}

func TestCreateGoalDefaultPriority(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Goal Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	goal, err := CreateGoal(db, project.ID, "Some goal", "", "", "", 0)
	if err != nil {
		t.Fatalf("CreateGoal() error = %v", err)
	}
	if goal.Priority != 1 {
		t.Fatalf("CreateGoal(priority=0).Priority = %d, want 1", goal.Priority)
	}
}

func TestDeleteGoalNotFound(t *testing.T) {
	db := testDB(t)
	if err := DeleteGoal(db, 999); err == nil {
		t.Fatal("DeleteGoal(nonexistent) error = nil, want error")
	}
}
