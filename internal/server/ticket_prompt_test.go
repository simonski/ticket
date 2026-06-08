package server

import (
	"context"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/store"
)

func TestBuildTicketWorkPromptIncludesTicketAndProject(t *testing.T) {
	t.Parallel()
	db, err := store.Open(seededAnalyseDBPath(t))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	project, err := store.GetProject(ctx, db, "1")
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	ticket, err := store.CreateTicket(ctx, db, store.TicketCreateParams{
		ProjectID:   project.ID,
		Type:        "task",
		Title:       "Wire up the export endpoint",
		Description: "Stream a zip archive of the user's data.",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	prompt := buildTicketWorkPrompt(ctx, db, ticket)
	for _, want := range []string{
		"Wire up the export endpoint",
		"Stream a zip archive",
		"Ticket: " + ticket.ID,
		"Project:",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("buildTicketWorkPrompt() missing %q in:\n%s", want, prompt)
		}
	}
}
