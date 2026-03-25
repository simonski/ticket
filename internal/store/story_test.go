package store

import (
	"testing"
)

func TestStoryCRUDAndLinking(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Stories Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	story, err := CreateStory(db, project.ID, "Customer onboarding", "High-level onboarding requirement", "")
	if err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}
	if story.ID == 0 {
		t.Fatalf("CreateStory() story id = 0")
	}
	list, err := ListStoriesByProject(db, project.ID)
	if err != nil {
		t.Fatalf("ListStoriesByProject() error = %v", err)
	}
	if len(list) != 1 || list[0].ID != story.ID {
		t.Fatalf("ListStoriesByProject() = %#v", list)
	}

	updated, err := UpdateStoryStatus(db, story.ID, "ready_for_review")
	if err != nil {
		t.Fatalf("UpdateStoryStatus() error = %v", err)
	}
	if updated.Status != "ready_for_review" {
		t.Fatalf("UpdateStoryStatus() status = %q, want ready_for_review", updated.Status)
	}

	ticket, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Onboarding epic",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := LinkStoryToTicket(db, story.ID, ticket.ID); err != nil {
		t.Fatalf("LinkStoryToTicket() error = %v", err)
	}
	storyID, ok, err := StoryIDForTicket(db, ticket.ID)
	if err != nil {
		t.Fatalf("StoryIDForTicket() error = %v", err)
	}
	if !ok || storyID != story.ID {
		t.Fatalf("StoryIDForTicket() = (%d,%v), want (%d,true)", storyID, ok, story.ID)
	}
}
