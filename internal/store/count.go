package store

import (
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

func CountEverything(db *sql.DB, projectID *int64) (CountSummary, error) {
	var summary CountSummary

	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&summary.Users); err != nil {
		return CountSummary{}, err
	}

	if projectID == nil {
		if err := db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&summary.Projects); err != nil {
			return CountSummary{}, err
		}
	}

	query := `
		SELECT type, stage, state, COUNT(*)
		FROM tasks
	`
	var rows *sql.Rows
	var err error
	if projectID != nil {
		query += ` WHERE project_id = ?`
		rows, err = db.Query(query+` GROUP BY type, stage, state ORDER BY type, stage, state`, *projectID)
	} else {
		rows, err = db.Query(query + ` GROUP BY type, stage, state ORDER BY type, stage, state`)
	}
	if err != nil {
		return CountSummary{}, err
	}
	defer rows.Close()

	byType := map[string]*TypeCount{}
	for rows.Next() {
		var taskType string
		var stage string
		var state string
		var count int
		if err := rows.Scan(&taskType, &stage, &state, &count); err != nil {
			return CountSummary{}, err
		}
		entry, ok := byType[taskType]
		if !ok {
			entry = &TypeCount{
				Type:     taskType,
				Statuses: map[string]int{},
			}
			byType[taskType] = entry
		}
		entry.Total += count
		entry.Statuses[RenderLifecycleStatus(stage, state)] = count
	}
	if err := rows.Err(); err != nil {
		return CountSummary{}, err
	}

	for _, taskType := range []string{"epic", "task", "bug", "spike", "chore"} {
		if entry, ok := byType[taskType]; ok {
			summary.Types = append(summary.Types, *entry)
		}
	}
	return summary, nil
}
