package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ReorderChildTickets sets the sort_order of a parent's live children to match
// orderedIDs (1-based). Children omitted from orderedIDs keep their relative
// order and are placed after the listed ones. Every listed ID must be a live
// child of the parent and may appear only once.
//
// This backs the decomposition-reordering contract in FACTORY.md §5.3 (FR-14): during
// refinement the human can reprioritize the proposed breakdown before sign-off.
func ReorderChildTickets(ctx context.Context, db *sql.DB, parentID string, orderedIDs []string, actorUsername, actorID string) ([]Ticket, error) {
	parent, err := GetTicket(ctx, db, parentID)
	if err != nil {
		return nil, err
	}
	children, err := ListChildTicketsByOrder(ctx, db, parentID)
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return nil, fmt.Errorf("ticket %s has no children to reorder", parentID)
	}
	childSet := make(map[string]bool, len(children))
	for _, child := range children {
		childSet[child.ID] = true
	}
	seen := make(map[string]bool, len(orderedIDs))
	cleaned := make([]string, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !childSet[id] {
			return nil, fmt.Errorf("ticket %s is not a child of %s", id, parentID)
		}
		if seen[id] {
			return nil, fmt.Errorf("ticket %s appears more than once in the order", id)
		}
		seen[id] = true
		cleaned = append(cleaned, id)
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("order must list at least one child ticket")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	position := 0
	apply := func(id string) error {
		position++
		_, execErr := tx.ExecContext(ctx, `
			UPDATE tickets SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE ticket_id = ?
		`, position, id)
		return execErr
	}
	for _, id := range cleaned {
		if err := apply(id); err != nil {
			return nil, err
		}
	}
	for _, child := range children {
		if seen[child.ID] {
			continue
		}
		if err := apply(child.ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	_ = AddHistoryEvent(ctx, db, parent.ProjectID, parentID, "ticket_children_reordered", map[string]any{
		"order": cleaned,
		"actor": actorUsername,
	}, actorID)

	return ListChildTicketsByOrder(ctx, db, parentID)
}

// ListChildTicketsByOrder returns a parent's live children sorted by their
// explicit sort_order (creation order as tie-break).
func ListChildTicketsByOrder(ctx context.Context, db *sql.DB, parentID string) ([]Ticket, error) {
	// #nosec G202 -- ticketSelectColumns is a fixed, code-controlled column list (no user input); values are bound parameters.
	rows, err := db.QueryContext(ctx, `
		SELECT `+ticketSelectColumns("")+`
		FROM tickets
		WHERE parent_id = ? AND deleted = 0
		ORDER BY sort_order, created_at, ticket_id
	`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tickets := make([]Ticket, 0)
	for rows.Next() {
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}
