package store

import "testing"

func TestCountEverything(t *testing.T) {
	db := testDB(t)

	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	otherProject, err := CreateProject(db, "Internal Tools", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() other error = %v", err)
	}

	if _, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Task A",
		Stage:     StageDesign,
		State:     StateIdle,
		CreatedBy: 1,
	}); err != nil {
		t.Fatalf("CreateTicket(task design/idle) error = %v", err)
	}
	if _, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Task B",
		Stage:     StageDone,
		State:     StateComplete,
		CreatedBy: 1,
	}); err != nil {
		t.Fatalf("CreateTicket(task done/complete) error = %v", err)
	}
	if _, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Epic A",
		Stage:     StageDone,
		State:     StateComplete,
		CreatedBy: 1,
	}); err != nil {
		t.Fatalf("CreateTicket(epic done/complete) error = %v", err)
	}
	if _, err := CreateTicket(db, TicketCreateParams{
		ProjectID: otherProject.ID,
		Type:      "bug",
		Title:     "Bug A",
		Stage:     StageDevelop,
		State:     StateActive,
		Assignee:  "alice",
		CreatedBy: 1,
	}); err != nil {
		t.Fatalf("CreateTicket(bug develop/active) error = %v", err)
	}

	all, err := CountEverything(db, nil)
	if err != nil {
		t.Fatalf("CountEverything(all) error = %v", err)
	}
	if all.Users != 1 {
		t.Fatalf("CountEverything(all).Users = %d, want 1", all.Users)
	}
	if all.Projects != 3 {
		t.Fatalf("CountEverything(all).Projects = %d, want 3", all.Projects)
	}
	if len(all.Types) != 3 {
		t.Fatalf("CountEverything(all).Types len = %d, want 3", len(all.Types))
	}

	projectOnly, err := CountEverything(db, &project.ID)
	if err != nil {
		t.Fatalf("CountEverything(project) error = %v", err)
	}
	if projectOnly.Projects != 0 {
		t.Fatalf("CountEverything(project).Projects = %d, want 0", projectOnly.Projects)
	}
	if len(projectOnly.Types) != 2 {
		t.Fatalf("CountEverything(project).Types len = %d, want 2", len(projectOnly.Types))
	}
	if projectOnly.Types[0].Type != "epic" || projectOnly.Types[0].Total != 1 {
		t.Fatalf("CountEverything(project).Types[0] = %#v", projectOnly.Types[0])
	}
	if projectOnly.Types[1].Type != "task" || projectOnly.Types[1].Total != 2 {
		t.Fatalf("CountEverything(project).Types[1] = %#v", projectOnly.Types[1])
	}
	if projectOnly.Types[1].Statuses["done/complete"] != 1 || projectOnly.Types[1].Statuses["design/idle"] != 1 {
		t.Fatalf("CountEverything(project).Types[1].Statuses = %#v", projectOnly.Types[1].Statuses)
	}
}
