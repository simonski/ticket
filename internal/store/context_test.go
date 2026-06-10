package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestContextEdgeCRUDAndGraph(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "Context Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID, Type: "story", Title: "Goal with context",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	document, err := CreateDocument(ctx, db, project.ID, "Design Notes", "the design", "", "graph contents here")
	if err != nil {
		t.Fatalf("CreateDocument() error = %v", err)
	}

	// Ticket → document edge.
	edge, err := AddContextEdge(ctx, db, project.ID, ContextNodeTicket, ticket.ID, ContextNodeDocument, fmt.Sprintf("%d", document.ID), "", "", "")
	if err != nil {
		t.Fatalf("AddContextEdge(ticket→document) error = %v", err)
	}
	if edge.Relation != "references" {
		t.Fatalf("AddContextEdge().Relation = %q, want references", edge.Relation)
	}

	// Duplicate edge is rejected.
	if _, err := AddContextEdge(ctx, db, project.ID, ContextNodeTicket, ticket.ID, ContextNodeDocument, fmt.Sprintf("%d", document.ID), "", "", ""); err == nil {
		t.Fatal("AddContextEdge() duplicate should fail")
	}

	// Ticket → URL edge.
	urlEdge, err := AddContextEdge(ctx, db, project.ID, ContextNodeTicket, ticket.ID, ContextNodeURL, "https://example.com/spec", "", "External spec", "")
	if err != nil {
		t.Fatalf("AddContextEdge(ticket→url) error = %v", err)
	}
	if urlEdge.Title != "External spec" {
		t.Fatalf("AddContextEdge().Title = %q, want External spec", urlEdge.Title)
	}

	// Invalid nodes are rejected.
	if _, err := AddContextEdge(ctx, db, project.ID, ContextNodeTicket, "NOPE-1", ContextNodeURL, "https://example.com", "", "", ""); err == nil {
		t.Fatal("AddContextEdge() with unknown ticket should fail")
	}
	if _, err := AddContextEdge(ctx, db, project.ID, ContextNodeTicket, ticket.ID, ContextNodeURL, "ftp://example.com", "", "", ""); err == nil {
		t.Fatal("AddContextEdge() with non-http url should fail")
	}
	if _, err := AddContextEdge(ctx, db, project.ID, "banana", ticket.ID, ContextNodeURL, "https://example.com", "", "", ""); err == nil {
		t.Fatal("AddContextEdge() with bad node type should fail")
	}

	// Edges for the ticket node.
	edges, err := ListContextEdgesForNode(ctx, db, ContextNodeTicket, ticket.ID)
	if err != nil {
		t.Fatalf("ListContextEdgesForNode() error = %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("ListContextEdgesForNode() len = %d, want 2", len(edges))
	}

	// Graph includes the document (always), the ticket, and the URL.
	graph, err := BuildContextGraph(ctx, db, project.ID)
	if err != nil {
		t.Fatalf("BuildContextGraph() error = %v", err)
	}
	if len(graph.Edges) != 2 {
		t.Fatalf("BuildContextGraph() edges = %d, want 2", len(graph.Edges))
	}
	types := map[string]int{}
	for _, node := range graph.Nodes {
		types[node.Type]++
	}
	if types[ContextNodeDocument] != 1 || types[ContextNodeTicket] != 1 || types[ContextNodeURL] != 1 {
		t.Fatalf("BuildContextGraph() node types = %v, want one of each", types)
	}

	// Remove an edge; graph shrinks.
	if err := RemoveContextEdge(ctx, db, project.ID, urlEdge.ID); err != nil {
		t.Fatalf("RemoveContextEdge() error = %v", err)
	}
	graph, err = BuildContextGraph(ctx, db, project.ID)
	if err != nil {
		t.Fatalf("BuildContextGraph() after remove error = %v", err)
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("BuildContextGraph() after remove edges = %d, want 1", len(graph.Edges))
	}
	// Removing through the wrong project fails.
	if err := RemoveContextEdge(ctx, db, project.ID+999, edge.ID); err == nil {
		t.Fatal("RemoveContextEdge() with wrong project should fail")
	}
}

func TestSearchContext(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "Search Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateDocument(ctx, db, project.ID, "Payments runbook", "", "", "the flux capacitor handles retries"); err != nil {
		t.Fatalf("CreateDocument() error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID, Type: "story", Title: "Wire up the flux capacitor",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// Matches document content and ticket title.
	nodes, err := SearchContext(ctx, db, project.ID, "flux capacitor")
	if err != nil {
		t.Fatalf("SearchContext() error = %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("SearchContext() len = %d, want 2 (%#v)", len(nodes), nodes)
	}
	foundTicket := false
	for _, node := range nodes {
		if node.Type == ContextNodeTicket && node.ID == ticket.ID {
			foundTicket = true
		}
	}
	if !foundTicket {
		t.Fatalf("SearchContext() did not return ticket %s", ticket.ID)
	}

	// Empty query returns nothing.
	nodes, err = SearchContext(ctx, db, project.ID, "  ")
	if err != nil {
		t.Fatalf("SearchContext() empty error = %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("SearchContext() empty len = %d, want 0", len(nodes))
	}

	// No cross-project leakage.
	other, err := CreateProject(ctx, db, "Other Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject(other) error = %v", err)
	}
	nodes, err = SearchContext(ctx, db, other.ID, "flux")
	if err != nil {
		t.Fatalf("SearchContext(other) error = %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("SearchContext(other) len = %d, want 0", len(nodes))
	}
}

func TestReorderChildTickets(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "Reorder Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	parent, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID, Type: "story", Title: "Idea being broken down",
	})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	makeChild := func(title string) Ticket {
		t.Helper()
		pid := parent.ID
		child, cErr := CreateTicket(ctx, db, TicketCreateParams{
			ProjectID: project.ID, ParentID: &pid, Type: "story", Title: title,
		})
		if cErr != nil {
			t.Fatalf("CreateTicket(%s) error = %v", title, cErr)
		}
		return child
	}
	first := makeChild("First proposed story")
	second := makeChild("Second proposed story")
	third := makeChild("Third proposed story")

	// Reverse the order.
	children, err := ReorderChildTickets(ctx, db, parent.ID, []string{third.ID, second.ID, first.ID}, "admin", "")
	if err != nil {
		t.Fatalf("ReorderChildTickets() error = %v", err)
	}
	got := []string{children[0].ID, children[1].ID, children[2].ID}
	want := []string{third.ID, second.ID, first.ID}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ReorderChildTickets() order = %v, want %v", got, want)
		}
	}
	if children[0].Order != 1 || children[2].Order != 3 {
		t.Fatalf("ReorderChildTickets() sort_order = %d,%d, want 1,3", children[0].Order, children[2].Order)
	}

	// Partial list: listed children go first, the rest keep relative order after.
	children, err = ReorderChildTickets(ctx, db, parent.ID, []string{first.ID}, "admin", "")
	if err != nil {
		t.Fatalf("ReorderChildTickets(partial) error = %v", err)
	}
	if children[0].ID != first.ID {
		t.Fatalf("ReorderChildTickets(partial) first = %s, want %s", children[0].ID, first.ID)
	}
	if children[1].ID != third.ID || children[2].ID != second.ID {
		t.Fatalf("ReorderChildTickets(partial) rest = %s,%s, want %s,%s", children[1].ID, children[2].ID, third.ID, second.ID)
	}

	// Errors: non-child, duplicate, empty, childless parent.
	if _, err := ReorderChildTickets(ctx, db, parent.ID, []string{"NOPE-1"}, "admin", ""); err == nil {
		t.Fatal("ReorderChildTickets() with non-child should fail")
	}
	if _, err := ReorderChildTickets(ctx, db, parent.ID, []string{first.ID, first.ID}, "admin", ""); err == nil {
		t.Fatal("ReorderChildTickets() with duplicate should fail")
	}
	if _, err := ReorderChildTickets(ctx, db, parent.ID, nil, "admin", ""); err == nil {
		t.Fatal("ReorderChildTickets() with empty order should fail")
	}
	if _, err := ReorderChildTickets(ctx, db, first.ID, []string{first.ID}, "admin", ""); err == nil ||
		!strings.Contains(err.Error(), "no children") {
		t.Fatalf("ReorderChildTickets() on leaf error = %v, want no-children error", err)
	}

	// History event recorded on the parent.
	events, err := ListHistoryEvents(ctx, db, parent.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListHistoryEvents() error = %v", err)
	}
	foundReorder := false
	for _, event := range events {
		if event.EventType == "ticket_children_reordered" {
			foundReorder = true
		}
	}
	if !foundReorder {
		t.Fatal("expected ticket_children_reordered history event")
	}
}
