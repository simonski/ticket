package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Context graph (GOAL.md "context graph").
//
// Tickets, documents, and external URLs form a project-scoped graph: nodes are
// the entities themselves, edges are typed links recorded in context_edges.
// Uploaded documents contribute their content to the graph and everything is
// queryable — by node (what context is attached to this ticket?), by project
// (the whole graph), or by text search across node content.

// Context node types.
const (
	ContextNodeTicket   = "ticket"
	ContextNodeDocument = "document"
	ContextNodeURL      = "url"
)

// ContextEdge links two context nodes within a project.
type ContextEdge struct {
	ID         int64  `json:"edge_id"`
	ProjectID  int64  `json:"project_id"`
	SourceType string `json:"source_type"`
	SourceID   string `json:"source_id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Relation   string `json:"relation"`
	Title      string `json:"title,omitempty"` // display title for url targets
	CreatedBy  string `json:"created_by,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// ContextNode is a node in the context graph: a ticket, a document, or an
// external URL.
type ContextNode struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ContextGraph is the project-scoped graph of context nodes and edges.
type ContextGraph struct {
	Nodes []ContextNode `json:"nodes"`
	Edges []ContextEdge `json:"edges"`
}

// ErrInvalidContextNode is returned when an edge references an unknown node
// type or a node that does not exist in the project.
var ErrInvalidContextNode = errors.New("invalid context node")

func validContextNodeType(t string) bool {
	switch t {
	case ContextNodeTicket, ContextNodeDocument, ContextNodeURL:
		return true
	default:
		return false
	}
}

// resolveContextNode validates that a node reference exists inside the project
// and returns its display title.
func resolveContextNode(ctx context.Context, db *sql.DB, projectID int64, nodeType, nodeID string) (string, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return "", fmt.Errorf("%w: id is required", ErrInvalidContextNode)
	}
	switch nodeType {
	case ContextNodeTicket:
		ticket, err := GetTicket(ctx, db, nodeID)
		if err != nil {
			return "", fmt.Errorf("%w: ticket %s not found", ErrInvalidContextNode, nodeID)
		}
		if ticket.ProjectID != projectID {
			return "", fmt.Errorf("%w: ticket %s is not in this project", ErrInvalidContextNode, nodeID)
		}
		return ticket.Title, nil
	case ContextNodeDocument:
		var docID int64
		if _, err := fmt.Sscan(nodeID, &docID); err != nil {
			return "", fmt.Errorf("%w: document id must be numeric", ErrInvalidContextNode)
		}
		document, err := GetDocument(ctx, db, docID)
		if err != nil {
			return "", fmt.Errorf("%w: document %s not found", ErrInvalidContextNode, nodeID)
		}
		if document.ProjectID != projectID {
			return "", fmt.Errorf("%w: document %s is not in this project", ErrInvalidContextNode, nodeID)
		}
		return document.Title, nil
	case ContextNodeURL:
		if !strings.HasPrefix(nodeID, "http://") && !strings.HasPrefix(nodeID, "https://") {
			return "", fmt.Errorf("%w: url must start with http:// or https://", ErrInvalidContextNode)
		}
		return nodeID, nil
	default:
		return "", fmt.Errorf("%w: unknown node type %q", ErrInvalidContextNode, nodeType)
	}
}

// AddContextEdge links a source node to a target node within a project. The
// relation defaults to "references". Duplicate edges are rejected.
func AddContextEdge(ctx context.Context, db *sql.DB, projectID int64, sourceType, sourceID, targetType, targetID, relation, title, createdBy string) (ContextEdge, error) {
	if !validContextNodeType(sourceType) || !validContextNodeType(targetType) {
		return ContextEdge{}, fmt.Errorf("%w: node type must be one of ticket, document, url", ErrInvalidContextNode)
	}
	if _, err := resolveContextNode(ctx, db, projectID, sourceType, sourceID); err != nil {
		return ContextEdge{}, err
	}
	resolvedTitle, err := resolveContextNode(ctx, db, projectID, targetType, targetID)
	if err != nil {
		return ContextEdge{}, err
	}
	relation = strings.TrimSpace(relation)
	if relation == "" {
		relation = "references"
	}
	title = strings.TrimSpace(title)
	if title == "" && targetType == ContextNodeURL {
		title = resolvedTitle
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO context_edges (project_id, source_type, source_id, target_type, target_id, relation, title, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, projectID, sourceType, strings.TrimSpace(sourceID), targetType, strings.TrimSpace(targetID), relation, title, createdBy)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ContextEdge{}, errors.New("context link already exists")
		}
		return ContextEdge{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return ContextEdge{}, err
	}
	return GetContextEdge(ctx, db, id)
}

// GetContextEdge returns one edge by ID.
func GetContextEdge(ctx context.Context, db *sql.DB, edgeID int64) (ContextEdge, error) {
	row := db.QueryRowContext(ctx, `
		SELECT edge_id, project_id, source_type, source_id, target_type, target_id, relation, title, COALESCE(created_by, ''), created_at
		FROM context_edges
		WHERE edge_id = ?
	`, edgeID)
	var edge ContextEdge
	if err := row.Scan(&edge.ID, &edge.ProjectID, &edge.SourceType, &edge.SourceID, &edge.TargetType, &edge.TargetID, &edge.Relation, &edge.Title, &edge.CreatedBy, &edge.CreatedAt); err != nil {
		return ContextEdge{}, err
	}
	return edge, nil
}

// RemoveContextEdge deletes an edge. The projectID guard prevents removing
// edges through another project's authorization scope.
func RemoveContextEdge(ctx context.Context, db *sql.DB, projectID, edgeID int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM context_edges WHERE edge_id = ? AND project_id = ?`, edgeID, projectID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListContextEdgesForNode returns every edge that touches the given node,
// whether as source or target.
func ListContextEdgesForNode(ctx context.Context, db *sql.DB, nodeType, nodeID string) ([]ContextEdge, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT edge_id, project_id, source_type, source_id, target_type, target_id, relation, title, COALESCE(created_by, ''), created_at
		FROM context_edges
		WHERE (source_type = ? AND source_id = ?) OR (target_type = ? AND target_id = ?)
		ORDER BY edge_id
	`, nodeType, nodeID, nodeType, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContextEdges(rows)
}

func scanContextEdges(rows *sql.Rows) ([]ContextEdge, error) {
	edges := make([]ContextEdge, 0)
	for rows.Next() {
		var edge ContextEdge
		if err := rows.Scan(&edge.ID, &edge.ProjectID, &edge.SourceType, &edge.SourceID, &edge.TargetType, &edge.TargetID, &edge.Relation, &edge.Title, &edge.CreatedBy, &edge.CreatedAt); err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	return edges, rows.Err()
}

// BuildContextGraph assembles the project's context graph: every document in
// the project is a node (uploaded documents always contribute), plus every
// node referenced by an edge.
func BuildContextGraph(ctx context.Context, db *sql.DB, projectID int64) (ContextGraph, error) {
	graph := ContextGraph{Nodes: make([]ContextNode, 0), Edges: make([]ContextEdge, 0)}

	rows, err := db.QueryContext(ctx, `
		SELECT edge_id, project_id, source_type, source_id, target_type, target_id, relation, title, COALESCE(created_by, ''), created_at
		FROM context_edges
		WHERE project_id = ?
		ORDER BY edge_id
	`, projectID)
	if err != nil {
		return ContextGraph{}, err
	}
	defer rows.Close()
	edges, err := scanContextEdges(rows)
	if err != nil {
		return ContextGraph{}, err
	}
	graph.Edges = edges

	seen := make(map[string]bool)
	addNode := func(node ContextNode) {
		key := node.Type + "\x00" + node.ID
		if seen[key] {
			return
		}
		seen[key] = true
		graph.Nodes = append(graph.Nodes, node)
	}

	documents, err := ListDocumentsByProject(ctx, db, projectID)
	if err != nil {
		return ContextGraph{}, err
	}
	for _, doc := range documents {
		addNode(ContextNode{Type: ContextNodeDocument, ID: fmt.Sprintf("%d", doc.ID), Title: doc.Title})
	}

	for _, edge := range edges {
		for _, ref := range [][2]string{{edge.SourceType, edge.SourceID}, {edge.TargetType, edge.TargetID}} {
			nodeType, nodeID := ref[0], ref[1]
			if seen[nodeType+"\x00"+nodeID] {
				continue
			}
			title := nodeID
			switch nodeType {
			case ContextNodeTicket:
				if ticket, tErr := GetTicket(ctx, db, nodeID); tErr == nil {
					title = ticket.Title
				}
			case ContextNodeURL:
				if edge.Title != "" {
					title = edge.Title
				}
			}
			addNode(ContextNode{Type: nodeType, ID: nodeID, Title: title})
		}
	}
	return graph, nil
}

// SearchContext finds context nodes by text. Documents match on title,
// description, notes, and content (uploaded content contributes to the graph);
// tickets match on key, title, and description.
func SearchContext(ctx context.Context, db *sql.DB, projectID int64, query string) ([]ContextNode, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []ContextNode{}, nil
	}
	like := "%" + query + "%"
	nodes := make([]ContextNode, 0)

	docRows, err := db.QueryContext(ctx, `
		SELECT document_id, title
		FROM documents
		WHERE project_id = ? AND (title LIKE ? OR description LIKE ? OR notes LIKE ? OR content LIKE ?)
		ORDER BY document_id
	`, projectID, like, like, like, like)
	if err != nil {
		return nil, err
	}
	defer docRows.Close()
	for docRows.Next() {
		var id int64
		var title string
		if scanErr := docRows.Scan(&id, &title); scanErr != nil {
			return nil, scanErr
		}
		nodes = append(nodes, ContextNode{Type: ContextNodeDocument, ID: fmt.Sprintf("%d", id), Title: title})
	}
	if rowsErr := docRows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	ticketRows, err := db.QueryContext(ctx, `
		SELECT ticket_id, title
		FROM tickets
		WHERE project_id = ? AND deleted = 0 AND (ticket_id LIKE ? OR title LIKE ? OR description LIKE ?)
		ORDER BY ticket_id
	`, projectID, like, like, like)
	if err != nil {
		return nil, err
	}
	defer ticketRows.Close()
	for ticketRows.Next() {
		var id, title string
		if err := ticketRows.Scan(&id, &title); err != nil {
			return nil, err
		}
		nodes = append(nodes, ContextNode{Type: ContextNodeTicket, ID: id, Title: title})
	}
	return nodes, ticketRows.Err()
}
