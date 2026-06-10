package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/simonski/ticket/internal/store"
)

// contextTestFixture creates a project, a parent ticket, and a document, and
// returns the admin token alongside them.
func contextTestFixture(t *testing.T, handler http.Handler) (string, store.Project, store.Ticket, store.Document) {
	t.Helper()
	adminToken := loginAdmin(t, handler)

	projectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{
		"title": "Context API Project",
	}, adminToken)
	if projectResp.Code != http.StatusCreated {
		t.Fatalf("create project status = %d body=%s", projectResp.Code, projectResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, projectResp, &project)

	ticketResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "story",
		"title":      "Goal needing context",
	}, adminToken)
	if ticketResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d body=%s", ticketResp.Code, ticketResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, ticketResp, &ticket)

	documentResp := doJSONRequest(t, handler, http.MethodPost, fmt.Sprintf("/api/projects/%d/documents", project.ID), map[string]any{
		"title":   "Spec document",
		"content": "all about the flux capacitor",
	}, adminToken)
	if documentResp.Code != http.StatusCreated {
		t.Fatalf("create document status = %d body=%s", documentResp.Code, documentResp.Body.String())
	}
	var document store.Document
	decodeResponse(t, documentResp, &document)

	return adminToken, project, ticket, document
}

func TestTicketContextEndpoints(t *testing.T) {
	t.Parallel()
	handler, _ := testHandler(t)
	adminToken, project, ticket, document := contextTestFixture(t, handler)

	// Attach a document to the ticket.
	addResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/context", map[string]any{
		"target_type": "document",
		"target_id":   fmt.Sprintf("%d", document.ID),
	}, adminToken)
	if addResp.Code != http.StatusCreated {
		t.Fatalf("add context status = %d body=%s", addResp.Code, addResp.Body.String())
	}
	var edge store.ContextEdge
	decodeResponse(t, addResp, &edge)
	if edge.Relation != "references" || edge.ProjectID != int64(project.ID) {
		t.Fatalf("add context edge = %#v", edge)
	}

	// Attach an external URL.
	urlResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/context", map[string]any{
		"target_type": "url",
		"target_id":   "https://example.com/design",
		"title":       "Design doc",
	}, adminToken)
	if urlResp.Code != http.StatusCreated {
		t.Fatalf("add url context status = %d body=%s", urlResp.Code, urlResp.Body.String())
	}
	var urlEdge store.ContextEdge
	decodeResponse(t, urlResp, &urlEdge)

	// Invalid target is a 400.
	badResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/context", map[string]any{
		"target_type": "url",
		"target_id":   "not-a-url",
	}, adminToken)
	if badResp.Code != http.StatusBadRequest {
		t.Fatalf("add invalid context status = %d body=%s", badResp.Code, badResp.Body.String())
	}

	// List context for the ticket.
	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/context", nil, adminToken)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list context status = %d body=%s", listResp.Code, listResp.Body.String())
	}
	var edges []store.ContextEdge
	decodeResponse(t, listResp, &edges)
	if len(edges) != 2 {
		t.Fatalf("list context len = %d, want 2", len(edges))
	}

	// Unauthenticated requests are rejected.
	anonResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/context", nil, "")
	if anonResp.Code != http.StatusUnauthorized {
		t.Fatalf("anon list context status = %d, want 401", anonResp.Code)
	}

	// Remove an edge.
	removeResp := doJSONRequest(t, handler, http.MethodDelete, fmt.Sprintf("/api/tickets/%s/context/%d", ticket.ID, urlEdge.ID), nil, adminToken)
	if removeResp.Code != http.StatusOK {
		t.Fatalf("remove context status = %d body=%s", removeResp.Code, removeResp.Body.String())
	}
	listResp = doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/context", nil, adminToken)
	decodeResponse(t, listResp, &edges)
	if len(edges) != 1 {
		t.Fatalf("list context after remove len = %d, want 1", len(edges))
	}
}

func TestProjectContextGraphAndSearch(t *testing.T) {
	t.Parallel()
	handler, _ := testHandler(t)
	adminToken, project, ticket, document := contextTestFixture(t, handler)

	addResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/context", map[string]any{
		"target_type": "document",
		"target_id":   fmt.Sprintf("%d", document.ID),
	}, adminToken)
	if addResp.Code != http.StatusCreated {
		t.Fatalf("add context status = %d body=%s", addResp.Code, addResp.Body.String())
	}

	// Whole-project graph: document + ticket nodes, one edge.
	graphResp := doJSONRequest(t, handler, http.MethodGet, fmt.Sprintf("/api/projects/%d/context", project.ID), nil, adminToken)
	if graphResp.Code != http.StatusOK {
		t.Fatalf("graph status = %d body=%s", graphResp.Code, graphResp.Body.String())
	}
	var graph store.ContextGraph
	decodeResponse(t, graphResp, &graph)
	if len(graph.Edges) != 1 || len(graph.Nodes) != 2 {
		t.Fatalf("graph nodes=%d edges=%d, want 2/1 (%#v)", len(graph.Nodes), len(graph.Edges), graph)
	}

	// Search across node content (document content matches).
	searchResp := doJSONRequest(t, handler, http.MethodGet, fmt.Sprintf("/api/projects/%d/context/search?q=flux", project.ID), nil, adminToken)
	if searchResp.Code != http.StatusOK {
		t.Fatalf("search status = %d body=%s", searchResp.Code, searchResp.Body.String())
	}
	var nodes []store.ContextNode
	decodeResponse(t, searchResp, &nodes)
	if len(nodes) != 1 || nodes[0].Type != store.ContextNodeDocument {
		t.Fatalf("search nodes = %#v, want one document node", nodes)
	}

	// Anonymous access is rejected.
	anonResp := doJSONRequest(t, handler, http.MethodGet, fmt.Sprintf("/api/projects/%d/context", project.ID), nil, "")
	if anonResp.Code != http.StatusUnauthorized {
		t.Fatalf("anon graph status = %d, want 401", anonResp.Code)
	}
}

func TestTicketChildrenReorderEndpoint(t *testing.T) {
	t.Parallel()
	handler, _ := testHandler(t)
	adminToken, project, parent, _ := contextTestFixture(t, handler)

	makeChild := func(title string) store.Ticket {
		t.Helper()
		resp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
			"project_id": project.ID,
			"type":       "story",
			"title":      title,
			"parent_id":  parent.ID,
		}, adminToken)
		if resp.Code != http.StatusCreated {
			t.Fatalf("create child status = %d body=%s", resp.Code, resp.Body.String())
		}
		var child store.Ticket
		decodeResponse(t, resp, &child)
		return child
	}
	first := makeChild("Story A")
	second := makeChild("Story B")
	third := makeChild("Story C")

	// Reverse the breakdown order.
	reorderResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+parent.ID+"/children/reorder", map[string]any{
		"order": []string{third.ID, second.ID, first.ID},
	}, adminToken)
	if reorderResp.Code != http.StatusOK {
		t.Fatalf("reorder status = %d body=%s", reorderResp.Code, reorderResp.Body.String())
	}
	var children []store.Ticket
	decodeResponse(t, reorderResp, &children)
	if len(children) != 3 || children[0].ID != third.ID || children[2].ID != first.ID {
		t.Fatalf("reorder children = %#v, want %s..%s", children, third.ID, first.ID)
	}

	// Reordering with a non-child is a 400.
	badResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+parent.ID+"/children/reorder", map[string]any{
		"order": []string{"NOPE-1"},
	}, adminToken)
	if badResp.Code != http.StatusBadRequest {
		t.Fatalf("reorder bad child status = %d body=%s", badResp.Code, badResp.Body.String())
	}

	// Unauthenticated reorder is rejected.
	anonResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+parent.ID+"/children/reorder", map[string]any{
		"order": []string{first.ID},
	}, "")
	if anonResp.Code != http.StatusUnauthorized {
		t.Fatalf("anon reorder status = %d, want 401", anonResp.Code)
	}
}
