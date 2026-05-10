package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

type GoalChatMessage struct {
	ID        int64  `json:"message_id"`
	GoalID    int64  `json:"goal_id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

type GoalStoryLink struct {
	GoalID    int64  `json:"goal_id"`
	StoryID   int64  `json:"story_id"`
	CreatedAt string `json:"created_at"`
}

type Goal struct {
	ID                  int64  `json:"goal_id"`
	ProjectID           int64  `json:"project_id"`
	Title               string `json:"title"`
	Description         string `json:"description"`
	Notes               string `json:"notes"`
	ETA                 string `json:"eta"`
	Priority            int    `json:"priority"`
	Status              string `json:"status"`
	RefinedGoal         string `json:"refined_goal"`
	Decompose           string `json:"decomposition"`
	RefinementConfirmed bool   `json:"refinement_confirmed"`
	AgentModelProvider  string `json:"agent_model_provider"`
	AgentModelName      string `json:"agent_model_name"`
	AgentModelURL       string `json:"agent_model_url"`
	AgentModelAPIKey    string `json:"agent_model_api_key"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

type GoalDecompositionItem struct {
	ID        int64  `json:"item_id"`
	GoalID    int64  `json:"goal_id"`
	Kind      string `json:"kind"`
	Text      string `json:"text"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type GoalClarification struct {
	ID        int64  `json:"clarification_id"`
	GoalID    int64  `json:"goal_id"`
	Question  string `json:"question"`
	Resolved  bool   `json:"resolved"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type GoalInboxItem struct {
	GoalID                   int64  `json:"goal_id"`
	ProjectID                int64  `json:"project_id"`
	Title                    string `json:"title"`
	Status                   string `json:"status"`
	Priority                 int    `json:"priority"`
	UpdatedAt                string `json:"updated_at"`
	RefinementConfirmed      bool   `json:"refinement_confirmed"`
	DecompositionDepth       int    `json:"decomposition_depth"`
	UnresolvedClarifications int    `json:"unresolved_clarifications"`
}

func CreateGoal(ctx context.Context, db *sql.DB, projectID int64, title, description, notes, eta string, priority int) (Goal, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Goal{}, errors.New("goal title is required")
	}
	if priority == 0 {
		priority = 1
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO goals (project_id, title, description, notes, eta, priority, status, refinement_confirmed, agent_model_provider, agent_model_name, agent_model_url, agent_model_api_key, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'draft', 0, '', '', '', '', CURRENT_TIMESTAMP)
	`, projectID, title, strings.TrimSpace(description), strings.TrimSpace(notes), strings.TrimSpace(eta), priority)
	if err != nil {
		return Goal{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Goal{}, err
	}
	return GetGoal(ctx, db, id)
}

func GetGoal(ctx context.Context, db *sql.DB, id int64) (Goal, error) {
	row := db.QueryRowContext(ctx, `
		SELECT goal_id, project_id, title, description, notes, eta, priority, status, refined_goal, decomposition, refinement_confirmed, agent_model_provider, agent_model_name, agent_model_url, agent_model_api_key, created_at, updated_at
		FROM goals WHERE goal_id = ?
	`, id)
	var g Goal
	if err := row.Scan(&g.ID, &g.ProjectID, &g.Title, &g.Description, &g.Notes, &g.ETA, &g.Priority, &g.Status, &g.RefinedGoal, &g.Decompose, &g.RefinementConfirmed, &g.AgentModelProvider, &g.AgentModelName, &g.AgentModelURL, &g.AgentModelAPIKey, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return Goal{}, err
	}
	return g, nil
}

func ListGoals(ctx context.Context, db *sql.DB, projectID int64) ([]Goal, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT goal_id, project_id, title, description, notes, eta, priority, status, refined_goal, decomposition, refinement_confirmed, agent_model_provider, agent_model_name, agent_model_url, agent_model_api_key, created_at, updated_at
		FROM goals WHERE project_id = ? ORDER BY priority, created_at
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var goals []Goal
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.ProjectID, &g.Title, &g.Description, &g.Notes, &g.ETA, &g.Priority, &g.Status, &g.RefinedGoal, &g.Decompose, &g.RefinementConfirmed, &g.AgentModelProvider, &g.AgentModelName, &g.AgentModelURL, &g.AgentModelAPIKey, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

func UpdateGoal(ctx context.Context, db *sql.DB, id int64, title, description, notes, eta string, priority int) (Goal, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Goal{}, errors.New("goal title is required")
	}
	if priority == 0 {
		priority = 1
	}
	result, err := db.ExecContext(ctx, `
		UPDATE goals
		SET title = ?, description = ?, notes = ?, eta = ?, priority = ?, updated_at = CURRENT_TIMESTAMP
		WHERE goal_id = ?
	`, title, strings.TrimSpace(description), strings.TrimSpace(notes), strings.TrimSpace(eta), priority, id)
	if err != nil {
		return Goal{}, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return Goal{}, errors.New("goal not found")
	}
	return GetGoal(ctx, db, id)
}

func SetGoalStatus(ctx context.Context, db *sql.DB, id int64, status string) (Goal, error) {
	normalized := strings.TrimSpace(strings.ToLower(status))
	switch normalized {
	case "draft", "refining", "ready":
	default:
		return Goal{}, errors.New("invalid goal status")
	}
	if normalized == "ready" {
		goal, err := GetGoal(ctx, db, id)
		if err != nil {
			return Goal{}, err
		}
		if strings.TrimSpace(goal.RefinedGoal) == "" {
			return Goal{}, errors.New("goal refined_goal is required before setting ready")
		}
		if strings.TrimSpace(goal.Decompose) == "" {
			return Goal{}, errors.New("goal decomposition is required before setting ready")
		}
		if !goal.RefinementConfirmed {
			return Goal{}, errors.New("goal refinement must be explicitly confirmed before setting ready")
		}
		depth, err := goalDecompositionDepth(ctx, db, id, goal.Decompose)
		if err != nil {
			return Goal{}, err
		}
		if depth < 3 {
			return Goal{}, errors.New("goal decomposition must contain at least 3 ordered items before setting ready")
		}
		unresolved, err := unresolvedGoalClarificationCount(ctx, db, id)
		if err != nil {
			return Goal{}, err
		}
		if unresolved > 0 {
			return Goal{}, errors.New("goal has unresolved clarification questions")
		}
	}
	result, err := db.ExecContext(ctx, `
		UPDATE goals
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE goal_id = ?
	`, normalized, id)
	if err != nil {
		return Goal{}, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return Goal{}, errors.New("goal not found")
	}
	return GetGoal(ctx, db, id)
}

func DeleteGoal(ctx context.Context, db *sql.DB, id int64) error {
	// Unlink any tickets from this goal
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET goal_id = NULL WHERE goal_id = ?`, id); err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `DELETE FROM goals WHERE goal_id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("goal not found")
	}
	return nil
}

func UpdateGoalRefinement(ctx context.Context, db *sql.DB, id int64, refinedGoal, decomposition string) (Goal, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Goal{}, err
	}
	trimmedGoal := strings.TrimSpace(refinedGoal)
	trimmedDecomposition := strings.TrimSpace(decomposition)
	result, err := tx.ExecContext(ctx, `
		UPDATE goals
		SET refined_goal = ?, decomposition = ?, refinement_confirmed = 0, updated_at = CURRENT_TIMESTAMP
		WHERE goal_id = ?
	`, trimmedGoal, trimmedDecomposition, id)
	if err != nil {
		_ = tx.Rollback()
		return Goal{}, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		_ = tx.Rollback()
		return Goal{}, errors.New("goal not found")
	}
	if err := replaceGoalDecompositionItemsFromText(ctx, tx, id, trimmedDecomposition); err != nil {
		_ = tx.Rollback()
		return Goal{}, err
	}
	if err := tx.Commit(); err != nil {
		return Goal{}, err
	}
	return GetGoal(ctx, db, id)
}

func ConfirmGoalRefinement(ctx context.Context, db *sql.DB, id int64, confirmed bool) (Goal, error) {
	confirmValue := 0
	if confirmed {
		confirmValue = 1
	}
	result, err := db.ExecContext(ctx, `
		UPDATE goals
		SET refinement_confirmed = ?, updated_at = CURRENT_TIMESTAMP
		WHERE goal_id = ?
	`, confirmValue, id)
	if err != nil {
		return Goal{}, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return Goal{}, errors.New("goal not found")
	}
	return GetGoal(ctx, db, id)
}

func ListGoalDecompositionItems(ctx context.Context, db *sql.DB, goalID int64) ([]GoalDecompositionItem, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT item_id, goal_id, kind, text, sort_order, created_at, updated_at
		FROM goal_decomposition_items
		WHERE goal_id = ?
		ORDER BY sort_order ASC, item_id ASC
	`, goalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GoalDecompositionItem, 0)
	for rows.Next() {
		var item GoalDecompositionItem
		if err := rows.Scan(&item.ID, &item.GoalID, &item.Kind, &item.Text, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func CreateGoalDecompositionItem(ctx context.Context, db *sql.DB, goalID int64, kind, text string, sortOrder *int) (GoalDecompositionItem, error) {
	kind = normalizeGoalDecompositionKind(kind)
	if kind == "" {
		return GoalDecompositionItem{}, errors.New("invalid decomposition kind")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return GoalDecompositionItem{}, errors.New("decomposition text is required")
	}
	nextSortOrder := 0
	if sortOrder == nil {
		if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM goal_decomposition_items WHERE goal_id = ?`, goalID).Scan(&nextSortOrder); err != nil {
			return GoalDecompositionItem{}, err
		}
	} else {
		nextSortOrder = *sortOrder
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO goal_decomposition_items (goal_id, kind, text, sort_order, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, goalID, kind, text, nextSortOrder)
	if err != nil {
		return GoalDecompositionItem{}, err
	}
	itemID, err := result.LastInsertId()
	if err != nil {
		return GoalDecompositionItem{}, err
	}
	if err := syncGoalDecompositionTextFromItems(ctx, db, goalID); err != nil {
		return GoalDecompositionItem{}, err
	}
	return GetGoalDecompositionItem(ctx, db, goalID, itemID)
}

func GetGoalDecompositionItem(ctx context.Context, db *sql.DB, goalID, itemID int64) (GoalDecompositionItem, error) {
	row := db.QueryRowContext(ctx, `
		SELECT item_id, goal_id, kind, text, sort_order, created_at, updated_at
		FROM goal_decomposition_items
		WHERE goal_id = ? AND item_id = ?
	`, goalID, itemID)
	var item GoalDecompositionItem
	if err := row.Scan(&item.ID, &item.GoalID, &item.Kind, &item.Text, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GoalDecompositionItem{}, errors.New("goal decomposition item not found")
		}
		return GoalDecompositionItem{}, err
	}
	return item, nil
}

func UpdateGoalDecompositionItem(ctx context.Context, db *sql.DB, goalID, itemID int64, kind, text string) (GoalDecompositionItem, error) {
	kind = normalizeGoalDecompositionKind(kind)
	if kind == "" {
		return GoalDecompositionItem{}, errors.New("invalid decomposition kind")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return GoalDecompositionItem{}, errors.New("decomposition text is required")
	}
	result, err := db.ExecContext(ctx, `
		UPDATE goal_decomposition_items
		SET kind = ?, text = ?, updated_at = CURRENT_TIMESTAMP
		WHERE goal_id = ? AND item_id = ?
	`, kind, text, goalID, itemID)
	if err != nil {
		return GoalDecompositionItem{}, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return GoalDecompositionItem{}, errors.New("goal decomposition item not found")
	}
	if err := syncGoalDecompositionTextFromItems(ctx, db, goalID); err != nil {
		return GoalDecompositionItem{}, err
	}
	return GetGoalDecompositionItem(ctx, db, goalID, itemID)
}

func DeleteGoalDecompositionItem(ctx context.Context, db *sql.DB, goalID, itemID int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM goal_decomposition_items WHERE goal_id = ? AND item_id = ?`, goalID, itemID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("goal decomposition item not found")
	}
	if err := syncGoalDecompositionTextFromItems(ctx, db, goalID); err != nil {
		return err
	}
	return nil
}

func ReorderGoalDecompositionItems(ctx context.Context, db *sql.DB, goalID int64, itemIDs []int64) error {
	if len(itemIDs) == 0 {
		return errors.New("item_ids are required")
	}
	existing, err := ListGoalDecompositionItems(ctx, db, goalID)
	if err != nil {
		return err
	}
	if len(existing) != len(itemIDs) {
		return errors.New("item_ids must contain every decomposition item exactly once")
	}
	existingIDs := make([]int64, 0, len(existing))
	for _, item := range existing {
		existingIDs = append(existingIDs, item.ID)
	}
	slices.Sort(existingIDs)
	requested := append([]int64(nil), itemIDs...)
	slices.Sort(requested)
	if !slices.Equal(existingIDs, requested) {
		return errors.New("item_ids must contain every decomposition item exactly once")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for idx, itemID := range itemIDs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE goal_decomposition_items
			SET sort_order = ?, updated_at = CURRENT_TIMESTAMP
			WHERE goal_id = ? AND item_id = ?
		`, idx, goalID, itemID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := syncGoalDecompositionTextFromItems(ctx, tx, goalID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func ListGoalClarifications(ctx context.Context, db *sql.DB, goalID int64) ([]GoalClarification, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT clarification_id, goal_id, question, resolved, created_at, updated_at
		FROM goal_clarifications
		WHERE goal_id = ?
		ORDER BY clarification_id ASC
	`, goalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GoalClarification, 0)
	for rows.Next() {
		var item GoalClarification
		if err := rows.Scan(&item.ID, &item.GoalID, &item.Question, &item.Resolved, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func AddGoalClarification(ctx context.Context, db *sql.DB, goalID int64, question string) (GoalClarification, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return GoalClarification{}, errors.New("clarification question is required")
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO goal_clarifications (goal_id, question, resolved, updated_at)
		VALUES (?, ?, 0, CURRENT_TIMESTAMP)
	`, goalID, question)
	if err != nil {
		return GoalClarification{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return GoalClarification{}, err
	}
	return GetGoalClarification(ctx, db, goalID, id)
}

func GetGoalClarification(ctx context.Context, db *sql.DB, goalID, clarificationID int64) (GoalClarification, error) {
	row := db.QueryRowContext(ctx, `
		SELECT clarification_id, goal_id, question, resolved, created_at, updated_at
		FROM goal_clarifications
		WHERE goal_id = ? AND clarification_id = ?
	`, goalID, clarificationID)
	var clarification GoalClarification
	if err := row.Scan(&clarification.ID, &clarification.GoalID, &clarification.Question, &clarification.Resolved, &clarification.CreatedAt, &clarification.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GoalClarification{}, errors.New("goal clarification not found")
		}
		return GoalClarification{}, err
	}
	return clarification, nil
}

func SetGoalClarificationResolved(ctx context.Context, db *sql.DB, goalID, clarificationID int64, resolved bool) (GoalClarification, error) {
	resolvedValue := 0
	if resolved {
		resolvedValue = 1
	}
	result, err := db.ExecContext(ctx, `
		UPDATE goal_clarifications
		SET resolved = ?, updated_at = CURRENT_TIMESTAMP
		WHERE goal_id = ? AND clarification_id = ?
	`, resolvedValue, goalID, clarificationID)
	if err != nil {
		return GoalClarification{}, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return GoalClarification{}, errors.New("goal clarification not found")
	}
	return GetGoalClarification(ctx, db, goalID, clarificationID)
}

func ListGoalInbox(ctx context.Context, db *sql.DB, projectID int64, statusFilter, sort string) ([]GoalInboxItem, error) {
	statusFilter = strings.TrimSpace(strings.ToLower(statusFilter))
	if statusFilter != "" && statusFilter != "draft" && statusFilter != "refining" && statusFilter != "ready" {
		return nil, errors.New("invalid goal status filter")
	}
	args := []any{projectID}
	var query string
	switch strings.TrimSpace(strings.ToLower(sort)) {
	case "", "updated_desc":
		if statusFilter == "" {
			query = `
				SELECT g.goal_id, g.project_id, g.title, g.status, g.priority, g.updated_at, g.refinement_confirmed,
					(SELECT COUNT(1) FROM goal_decomposition_items d WHERE d.goal_id = g.goal_id) AS decomposition_depth,
					(SELECT COUNT(1) FROM goal_clarifications c WHERE c.goal_id = g.goal_id AND c.resolved = 0) AS unresolved_clarifications
				FROM goals g
				WHERE g.project_id = ?
				ORDER BY g.updated_at DESC, g.priority ASC, g.goal_id ASC`
		} else {
			query = `
				SELECT g.goal_id, g.project_id, g.title, g.status, g.priority, g.updated_at, g.refinement_confirmed,
					(SELECT COUNT(1) FROM goal_decomposition_items d WHERE d.goal_id = g.goal_id) AS decomposition_depth,
					(SELECT COUNT(1) FROM goal_clarifications c WHERE c.goal_id = g.goal_id AND c.resolved = 0) AS unresolved_clarifications
				FROM goals g
				WHERE g.project_id = ? AND g.status = ?
				ORDER BY g.updated_at DESC, g.priority ASC, g.goal_id ASC`
			args = append(args, statusFilter)
		}
	case "priority_asc":
		if statusFilter == "" {
			query = `
				SELECT g.goal_id, g.project_id, g.title, g.status, g.priority, g.updated_at, g.refinement_confirmed,
					(SELECT COUNT(1) FROM goal_decomposition_items d WHERE d.goal_id = g.goal_id) AS decomposition_depth,
					(SELECT COUNT(1) FROM goal_clarifications c WHERE c.goal_id = g.goal_id AND c.resolved = 0) AS unresolved_clarifications
				FROM goals g
				WHERE g.project_id = ?
				ORDER BY g.priority ASC, g.updated_at DESC, g.goal_id ASC`
		} else {
			query = `
				SELECT g.goal_id, g.project_id, g.title, g.status, g.priority, g.updated_at, g.refinement_confirmed,
					(SELECT COUNT(1) FROM goal_decomposition_items d WHERE d.goal_id = g.goal_id) AS decomposition_depth,
					(SELECT COUNT(1) FROM goal_clarifications c WHERE c.goal_id = g.goal_id AND c.resolved = 0) AS unresolved_clarifications
				FROM goals g
				WHERE g.project_id = ? AND g.status = ?
				ORDER BY g.priority ASC, g.updated_at DESC, g.goal_id ASC`
			args = append(args, statusFilter)
		}
	case "status":
		if statusFilter == "" {
			query = `
				SELECT g.goal_id, g.project_id, g.title, g.status, g.priority, g.updated_at, g.refinement_confirmed,
					(SELECT COUNT(1) FROM goal_decomposition_items d WHERE d.goal_id = g.goal_id) AS decomposition_depth,
					(SELECT COUNT(1) FROM goal_clarifications c WHERE c.goal_id = g.goal_id AND c.resolved = 0) AS unresolved_clarifications
				FROM goals g
				WHERE g.project_id = ?
				ORDER BY CASE g.status WHEN 'refining' THEN 0 WHEN 'draft' THEN 1 WHEN 'ready' THEN 2 ELSE 3 END, g.updated_at DESC, g.goal_id ASC`
		} else {
			query = `
				SELECT g.goal_id, g.project_id, g.title, g.status, g.priority, g.updated_at, g.refinement_confirmed,
					(SELECT COUNT(1) FROM goal_decomposition_items d WHERE d.goal_id = g.goal_id) AS decomposition_depth,
					(SELECT COUNT(1) FROM goal_clarifications c WHERE c.goal_id = g.goal_id AND c.resolved = 0) AS unresolved_clarifications
				FROM goals g
				WHERE g.project_id = ? AND g.status = ?
				ORDER BY CASE g.status WHEN 'refining' THEN 0 WHEN 'draft' THEN 1 WHEN 'ready' THEN 2 ELSE 3 END, g.updated_at DESC, g.goal_id ASC`
			args = append(args, statusFilter)
		}
	default:
		return nil, errors.New("invalid goal inbox sort")
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GoalInboxItem, 0)
	for rows.Next() {
		var item GoalInboxItem
		if err := rows.Scan(
			&item.GoalID,
			&item.ProjectID,
			&item.Title,
			&item.Status,
			&item.Priority,
			&item.UpdatedAt,
			&item.RefinementConfirmed,
			&item.DecompositionDepth,
			&item.UnresolvedClarifications,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func unresolvedGoalClarificationCount(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, goalID int64) (int, error) {
	var unresolved int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM goal_clarifications WHERE goal_id = ? AND resolved = 0`, goalID).Scan(&unresolved); err != nil {
		return 0, err
	}
	return unresolved, nil
}

func goalDecompositionDepth(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, goalID int64, fallbackText string) (int, error) {
	var depth int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM goal_decomposition_items WHERE goal_id = ?`, goalID).Scan(&depth); err != nil {
		return 0, err
	}
	if depth > 0 {
		return depth, nil
	}
	return len(parseGoalDecompositionLines(fallbackText)), nil
}

func normalizeGoalDecompositionKind(kind string) string {
	normalized := strings.TrimSpace(strings.ToLower(kind))
	switch normalized {
	case "objective", "epic", "story", "task":
		return normalized
	default:
		return ""
	}
}

func parseGoalDecompositionLines(text string) []string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed == "" {
			continue
		}
		trimmed = strings.TrimLeft(trimmed, "0123456789.)- ")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func replaceGoalDecompositionItemsFromText(ctx context.Context, db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, goalID int64, decomposition string) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM goal_decomposition_items WHERE goal_id = ?`, goalID); err != nil {
		return err
	}
	items := parseGoalDecompositionLines(decomposition)
	for idx, text := range items {
		kind := "task"
		switch {
		case strings.HasPrefix(strings.ToLower(text), "objective:"):
			kind = "objective"
		case strings.HasPrefix(strings.ToLower(text), "epic:"):
			kind = "epic"
		case strings.HasPrefix(strings.ToLower(text), "story:"):
			kind = "story"
		case strings.HasPrefix(strings.ToLower(text), "task:"):
			kind = "task"
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO goal_decomposition_items (goal_id, kind, text, sort_order, updated_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, goalID, kind, text, idx); err != nil {
			return err
		}
	}
	return nil
}

func syncGoalDecompositionTextFromItems(ctx context.Context, db interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, goalID int64) error {
	rows, err := db.QueryContext(ctx, `
		SELECT text
		FROM goal_decomposition_items
		WHERE goal_id = ?
		ORDER BY sort_order ASC, item_id ASC
	`, goalID)
	if err != nil {
		return err
	}
	defer rows.Close()
	lines := make([]string, 0)
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return err
		}
		lines = append(lines, strings.TrimSpace(text))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rendered := ""
	for idx, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if rendered != "" {
			rendered += "\n"
		}
		rendered += fmt.Sprintf("%d. %s", idx+1, line)
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE goals
		SET decomposition = ?, refinement_confirmed = 0, updated_at = CURRENT_TIMESTAMP
		WHERE goal_id = ?
	`, rendered, goalID); err != nil {
		return err
	}
	return nil
}

func AddGoalChatMessage(ctx context.Context, db *sql.DB, goalID int64, author, text string) (GoalChatMessage, error) {
	author = strings.TrimSpace(strings.ToLower(author))
	switch author {
	case "user", "agent", "system":
	default:
		return GoalChatMessage{}, errors.New("invalid chat author")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return GoalChatMessage{}, errors.New("chat text is required")
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO goal_chat_messages (goal_id, author, text)
		VALUES (?, ?, ?)
	`, goalID, author, text)
	if err != nil {
		return GoalChatMessage{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return GoalChatMessage{}, err
	}
	return GetGoalChatMessage(ctx, db, id)
}

func GetGoalChatMessage(ctx context.Context, db *sql.DB, id int64) (GoalChatMessage, error) {
	row := db.QueryRowContext(ctx, `
		SELECT message_id, goal_id, author, text, created_at
		FROM goal_chat_messages
		WHERE message_id = ?
	`, id)
	var msg GoalChatMessage
	if err := row.Scan(&msg.ID, &msg.GoalID, &msg.Author, &msg.Text, &msg.CreatedAt); err != nil {
		return GoalChatMessage{}, err
	}
	return msg, nil
}

func ListGoalChatMessages(ctx context.Context, db *sql.DB, goalID int64, limit int) ([]GoalChatMessage, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := db.QueryContext(ctx, `
		SELECT message_id, goal_id, author, text, created_at
		FROM goal_chat_messages
		WHERE goal_id = ?
		ORDER BY message_id ASC
		LIMIT ?
	`, goalID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GoalChatMessage, 0)
	for rows.Next() {
		var msg GoalChatMessage
		if err := rows.Scan(&msg.ID, &msg.GoalID, &msg.Author, &msg.Text, &msg.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, rows.Err()
}

func GoalChatMessagesAsJSON(messages []GoalChatMessage) string {
	encoded, err := json.Marshal(messages)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func LinkGoalToStory(ctx context.Context, db *sql.DB, goalID, storyID int64) error {
	if goalID < 1 || storyID < 1 {
		return errors.New("goal and story are required")
	}
	_, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO goal_story_links (goal_id, story_id)
		VALUES (?, ?)
	`, goalID, storyID)
	return err
}

func UnlinkGoalFromStory(ctx context.Context, db *sql.DB, goalID, storyID int64) error {
	result, err := db.ExecContext(ctx, `
		DELETE FROM goal_story_links
		WHERE goal_id = ? AND story_id = ?
	`, goalID, storyID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("goal/story link not found")
	}
	return nil
}

func ListStoriesForGoal(ctx context.Context, db *sql.DB, goalID int64) ([]Story, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.story_id, s.project_id, s.title, s.description, s.status, COALESCE(s.created_by, ''), s.created_at, s.updated_at
		FROM stories s
		JOIN goal_story_links l ON l.story_id = s.story_id
		WHERE l.goal_id = ?
		ORDER BY s.created_at DESC, s.story_id DESC
	`, goalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stories := make([]Story, 0)
	for rows.Next() {
		var story Story
		if err := rows.Scan(&story.ID, &story.ProjectID, &story.Title, &story.Description, &story.Status, &story.CreatedBy, &story.CreatedAt, &story.UpdatedAt); err != nil {
			return nil, err
		}
		stories = append(stories, story)
	}
	return stories, rows.Err()
}

func SetGoalAgentModelConfig(ctx context.Context, db *sql.DB, goalID int64, cfg AgentModelConfig) (Goal, error) {
	result, err := db.ExecContext(ctx, `
		UPDATE goals
		SET agent_model_provider = ?, agent_model_name = ?, agent_model_url = ?, agent_model_api_key = ?, updated_at = CURRENT_TIMESTAMP
		WHERE goal_id = ?
	`, strings.TrimSpace(cfg.Provider), strings.TrimSpace(cfg.Model), strings.TrimSpace(cfg.URL), strings.TrimSpace(cfg.APIKey), goalID)
	if err != nil {
		return Goal{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Goal{}, err
	}
	if affected == 0 {
		return Goal{}, sql.ErrNoRows
	}
	return GetGoal(ctx, db, goalID)
}

func ResolveGoalAgentModelConfig(ctx context.Context, db *sql.DB, goalID int64) (AgentModelConfig, error) {
	goal, err := GetGoal(ctx, db, goalID)
	if err != nil {
		return AgentModelConfig{}, err
	}
	project, err := GetProjectByID(ctx, db, goal.ProjectID)
	if err != nil {
		return AgentModelConfig{}, err
	}
	system, err := SystemAgentModelConfig(ctx, db)
	if err != nil {
		return AgentModelConfig{}, err
	}
	cfg := system
	if strings.TrimSpace(project.AgentModelProvider) != "" {
		cfg.Provider = strings.TrimSpace(project.AgentModelProvider)
	}
	if strings.TrimSpace(project.AgentModelName) != "" {
		cfg.Model = strings.TrimSpace(project.AgentModelName)
	}
	if strings.TrimSpace(project.AgentModelURL) != "" {
		cfg.URL = strings.TrimSpace(project.AgentModelURL)
	}
	if strings.TrimSpace(project.AgentModelAPIKey) != "" {
		cfg.APIKey = strings.TrimSpace(project.AgentModelAPIKey)
	}
	if strings.TrimSpace(goal.AgentModelProvider) != "" {
		cfg.Provider = strings.TrimSpace(goal.AgentModelProvider)
	}
	if strings.TrimSpace(goal.AgentModelName) != "" {
		cfg.Model = strings.TrimSpace(goal.AgentModelName)
	}
	if strings.TrimSpace(goal.AgentModelURL) != "" {
		cfg.URL = strings.TrimSpace(goal.AgentModelURL)
	}
	if strings.TrimSpace(goal.AgentModelAPIKey) != "" {
		cfg.APIKey = strings.TrimSpace(goal.AgentModelAPIKey)
	}
	return cfg, nil
}
