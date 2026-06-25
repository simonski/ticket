package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Room agent task states. Tasks are ephemeral: they move queued → running →
// done/failed and are then auto-purged, never reaching the ticket backlog.
const (
	RoomAgentTaskQueued  = "queued"
	RoomAgentTaskRunning = "running"
	RoomAgentTaskDone    = "done"
	RoomAgentTaskFailed  = "failed"
)

// RoomAgentTask is a single ephemeral work-item enqueued for an agent in a room
// via "@agent do X" / "@agent queue X" (TK-168). It is intentionally separate
// from WorkItem (ticket execution packets).
type RoomAgentTask struct {
	ID          int64  `json:"task_id"`
	RoomID      int64  `json:"room_id"`
	AgentID     string `json:"agent_id"`
	AgentName   string `json:"agent_name"`
	RequesterID string `json:"requester_id"`
	Instruction string `json:"instruction"`
	State       string `json:"state"`
	Result      string `json:"result"`
	Ephemeral   bool   `json:"ephemeral"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

const roomAgentTaskColumns = `task_id, room_id, agent_id, agent_name, requester_id, instruction, state, result, ephemeral, created_at, updated_at`

func scanRoomAgentTask(s interface{ Scan(...any) error }) (RoomAgentTask, error) {
	var (
		t         RoomAgentTask
		ephemeral int64
	)
	if err := s.Scan(&t.ID, &t.RoomID, &t.AgentID, &t.AgentName, &t.RequesterID, &t.Instruction, &t.State, &t.Result, &ephemeral, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return RoomAgentTask{}, err
	}
	t.Ephemeral = ephemeral != 0
	return t, nil
}

// EnqueueRoomAgentTask adds a queued task for an agent in a room.
func EnqueueRoomAgentTask(ctx context.Context, db *sql.DB, task RoomAgentTask) (RoomAgentTask, error) {
	if task.RoomID == 0 || strings.TrimSpace(task.AgentID) == "" {
		return RoomAgentTask{}, fmt.Errorf("room id and agent id are required")
	}
	instruction := strings.TrimSpace(task.Instruction)
	if instruction == "" {
		return RoomAgentTask{}, fmt.Errorf("instruction is required")
	}
	// These queue items are always ephemeral — they exist to keep throwaway agent
	// work out of the permanent ticket backlog. The column is kept for clarity and
	// future use, but enqueue always sets it.
	res, err := db.ExecContext(ctx,
		`INSERT INTO room_agent_tasks (room_id, agent_id, agent_name, requester_id, instruction, state, ephemeral) VALUES (?, ?, ?, ?, ?, ?, 1)`,
		task.RoomID, strings.TrimSpace(task.AgentID), task.AgentName, task.RequesterID, instruction, RoomAgentTaskQueued)
	if err != nil {
		return RoomAgentTask{}, fmt.Errorf("enqueue room agent task: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return RoomAgentTask{}, err
	}
	return GetRoomAgentTask(ctx, db, id)
}

// GetRoomAgentTask returns a single task by id.
func GetRoomAgentTask(ctx context.Context, db *sql.DB, id int64) (RoomAgentTask, error) {
	row := db.QueryRowContext(ctx, `SELECT `+roomAgentTaskColumns+` FROM room_agent_tasks WHERE task_id = ?`, id)
	return scanRoomAgentTask(row)
}

// ListRoomAgentTasks returns the tasks for a room in creation order. This drives
// the live queue panel, so it includes queued/running and recently finished items
// (the worker purges old finished ones separately).
func ListRoomAgentTasks(ctx context.Context, db *sql.DB, roomID int64) ([]RoomAgentTask, error) {
	rows, err := db.QueryContext(ctx, `SELECT `+roomAgentTaskColumns+` FROM room_agent_tasks WHERE room_id = ? ORDER BY task_id ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []RoomAgentTask
	for rows.Next() {
		t, err := scanRoomAgentTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ClaimNextRoomAgentTask atomically marks the oldest eligible queued task as
// running and returns it. A task is eligible only when no other task for the same
// (room, agent) is already running — this enforces serial, concurrency-1 execution
// per (room, agent) while letting different agents/rooms proceed in parallel.
// Returns ok=false when nothing is eligible.
func ClaimNextRoomAgentTask(ctx context.Context, db *sql.DB) (RoomAgentTask, bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return RoomAgentTask{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var id int64
	row := tx.QueryRowContext(ctx, `
		SELECT t.task_id FROM room_agent_tasks t
		WHERE t.state = ?
		  AND NOT EXISTS (
		      SELECT 1 FROM room_agent_tasks r
		      WHERE r.room_id = t.room_id AND r.agent_id = t.agent_id AND r.state = ?
		  )
		ORDER BY t.task_id ASC LIMIT 1`, RoomAgentTaskQueued, RoomAgentTaskRunning)
	if scanErr := row.Scan(&id); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return RoomAgentTask{}, false, nil
		}
		return RoomAgentTask{}, false, scanErr
	}
	if _, execErr := tx.ExecContext(ctx, `UPDATE room_agent_tasks SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE task_id = ?`, RoomAgentTaskRunning, id); execErr != nil {
		return RoomAgentTask{}, false, execErr
	}
	if commitErr := tx.Commit(); commitErr != nil {
		return RoomAgentTask{}, false, commitErr
	}
	t, err := GetRoomAgentTask(ctx, db, id)
	if err != nil {
		return RoomAgentTask{}, false, err
	}
	return t, true, nil
}

// FinishRoomAgentTask records the terminal state (done/failed) and result.
func FinishRoomAgentTask(ctx context.Context, db *sql.DB, id int64, state, result string) (RoomAgentTask, error) {
	if state != RoomAgentTaskDone && state != RoomAgentTaskFailed {
		return RoomAgentTask{}, fmt.Errorf("invalid terminal state %q", state)
	}
	if _, err := db.ExecContext(ctx, `UPDATE room_agent_tasks SET state = ?, result = ?, updated_at = CURRENT_TIMESTAMP WHERE task_id = ?`, state, result, id); err != nil {
		return RoomAgentTask{}, err
	}
	return GetRoomAgentTask(ctx, db, id)
}

// DeleteRoomAgentTask removes a single task (e.g. after promotion to a ticket).
func DeleteRoomAgentTask(ctx context.Context, db *sql.DB, id int64) error {
	_, err := db.ExecContext(ctx, `DELETE FROM room_agent_tasks WHERE task_id = ?`, id)
	return err
}

// HasPendingRoomAgentTasks reports whether any task is queued or running (used to
// decide whether the worker should keep polling).
func HasPendingRoomAgentTasks(ctx context.Context, db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM room_agent_tasks WHERE state IN (?, ?)`, RoomAgentTaskQueued, RoomAgentTaskRunning).Scan(&n)
	return n > 0, err
}

// PurgeFinishedRoomAgentTasks deletes ephemeral done/failed tasks older than the
// given age (in seconds), keeping the queue from accumulating. Non-ephemeral tasks
// are retained. Returns the number removed.
func PurgeFinishedRoomAgentTasks(ctx context.Context, db *sql.DB, olderThanSeconds int) (int64, error) {
	if olderThanSeconds < 0 {
		olderThanSeconds = 0
	}
	cutoff := fmt.Sprintf("-%d seconds", olderThanSeconds)
	res, err := db.ExecContext(ctx, `
		DELETE FROM room_agent_tasks
		WHERE ephemeral = 1
		  AND state IN (?, ?)
		  AND updated_at <= datetime('now', ?)`, RoomAgentTaskDone, RoomAgentTaskFailed, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
