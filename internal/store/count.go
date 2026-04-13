package store

import (
	"context"
	"database/sql"
)

type TypeCount struct {
	Type     string         `json:"type"`
	Total    int            `json:"total"`
	Statuses map[string]int `json:"statuses"`
}

type CountSummary struct {
	Users    int         `json:"users"`
	Projects int         `json:"projects,omitempty"`
	Types    []TypeCount `json:"types"`
}

func CountEverything(ctx context.Context, db *sql.DB, projectID *int64) (CountSummary, error) {
	var summary CountSummary

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&summary.Users); err != nil {
		return CountSummary{}, err
	}

	if projectID == nil {
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects`).Scan(&summary.Projects); err != nil {
			return CountSummary{}, err
		}
	}

	query := `
		SELECT type, stage, state, COUNT(*)
		FROM tickets
	`
	var rows *sql.Rows
	var err error
	if projectID != nil {
		query += ` WHERE project_id = ?`
		rows, err = db.QueryContext(ctx, query+` GROUP BY type, stage, state ORDER BY type, stage, state`, *projectID)
	} else {
		rows, err = db.QueryContext(ctx, query+` GROUP BY type, stage, state ORDER BY type, stage, state`)
	}
	if err != nil {
		return CountSummary{}, err
	}
	defer rows.Close()

	byType := map[string]*TypeCount{}
	for rows.Next() {
		var ticketType string
		var stage string
		var state string
		var count int
		if err := rows.Scan(&ticketType, &stage, &state, &count); err != nil {
			return CountSummary{}, err
		}
		entry, ok := byType[ticketType]
		if !ok {
			entry = &TypeCount{
				Type:     ticketType,
				Statuses: map[string]int{},
			}
			byType[ticketType] = entry
		}
		entry.Total += count
		entry.Statuses[RenderLifecycleStatus(stage, state)] = count
	}
	if err := rows.Err(); err != nil {
		return CountSummary{}, err
	}

	for _, ticketType := range []string{"epic", "task", "bug", "spike", "chore"} {
		if entry, ok := byType[ticketType]; ok {
			summary.Types = append(summary.Types, *entry)
		}
	}
	return summary, nil
}
