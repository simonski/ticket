package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/simonski/ticket/internal/password"
)

const defaultProjectPrefix = "TK"

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		db.Close()
		return nil, err
	}
	if err := createSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func Init(path, adminUsername, adminPassword string) error {
	if adminUsername == "" || adminPassword == "" {
		return errors.New("admin username and password are required")
	}

	if path != ":memory:" {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("database already exists at %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
			return err
		}
	}

	db, err := Open(path)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := createSchema(db); err != nil {
		return err
	}

	hash, err := password.Hash(adminPassword)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT INTO users (username, password_hash, role, display_name, enabled)
		VALUES (?, ?, 'admin', ?, 1)
	`, adminUsername, hash, adminUsername)
	if err != nil {
		return err
	}

	if _, err := CreateProjectWithParams(db, ProjectCreateParams{
		Prefix:             defaultProjectPrefix,
		Title:              "Default Project",
		Description:        "Bootstrap project created during initdb.",
		AcceptanceCriteria: "",
		CreatedBy:          1,
	}); err != nil {
		return err
	}
	return nil
}

func createSchema(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
	user_id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL,
	display_name TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
	session_id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	token TEXT NOT NULL UNIQUE,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at TEXT,
	FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS projects (
	project_id INTEGER PRIMARY KEY AUTOINCREMENT,
	prefix TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'open',
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	ticket_sequence INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS tasks (
	task_id INTEGER PRIMARY KEY AUTOINCREMENT,
	key TEXT NOT NULL DEFAULT '',
	project_id INTEGER NOT NULL,
	parent_id INTEGER,
	clone_of INTEGER,
	type TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	stage TEXT NOT NULL DEFAULT 'design',
	state TEXT NOT NULL DEFAULT 'idle',
	status TEXT NOT NULL DEFAULT 'open',
	priority INTEGER NOT NULL DEFAULT 3,
	sort_order INTEGER NOT NULL DEFAULT 0,
	estimate_effort INTEGER NOT NULL DEFAULT 0,
	estimate_complete TEXT NOT NULL DEFAULT '',
	health_score INTEGER NOT NULL DEFAULT 0,
	assignee TEXT NOT NULL DEFAULT '',
	archived INTEGER NOT NULL DEFAULT 0,
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(parent_id) REFERENCES tasks(task_id),
	FOREIGN KEY(clone_of) REFERENCES tasks(task_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS history_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	task_id INTEGER NOT NULL,
	event_type TEXT NOT NULL,
	payload TEXT NOT NULL DEFAULT '{}',
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(task_id) REFERENCES tasks(task_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS ticket_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	task_id INTEGER NOT NULL,
	event_type TEXT NOT NULL,
	payload TEXT NOT NULL DEFAULT '{}',
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(task_id) REFERENCES tasks(task_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS comments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	item_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	comment TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(item_id) REFERENCES tasks(task_id),
	FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS dependencies (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	task_id INTEGER NOT NULL,
	depends_on INTEGER NOT NULL,
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(task_id) REFERENCES tasks(task_id),
	FOREIGN KEY(depends_on) REFERENCES tasks(task_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);
`

	if _, err := db.Exec(schema); err != nil {
		return err
	}
	return migrateSchema(db)
}

func migrateSchema(db *sql.DB) error {
	if !columnExists(db, "projects", "prefix") {
		if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN prefix TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(db, "projects", "notes") {
		if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN notes TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(db, "projects", "updated_at") {
		if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`); err != nil {
			return err
		}
	}
	if !columnExists(db, "projects", "ticket_sequence") {
		if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN ticket_sequence INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`UPDATE projects SET status = 'open' WHERE status = 'active'`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE projects SET status = 'closed' WHERE status = 'disabled'`); err != nil {
		return err
	}

	if !columnExists(db, "tasks", "key") {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN key TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tasks", "sort_order") {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tasks", "estimate_effort") {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN estimate_effort INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tasks", "estimate_complete") {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN estimate_complete TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tasks", "health_score") {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN health_score INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tasks", "stage") {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN stage TEXT NOT NULL DEFAULT 'design'`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tasks", "state") {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN state TEXT NOT NULL DEFAULT 'idle'`); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`
		UPDATE tasks
		SET
			stage = CASE
				WHEN status = 'notready' THEN 'design'
				WHEN status = 'open' THEN 'develop'
				WHEN status = 'inprogress' THEN 'develop'
				WHEN status = 'complete' THEN 'done'
				WHEN status = 'fail' THEN 'test'
				ELSE COALESCE(NULLIF(TRIM(stage), ''), 'design')
			END,
			state = CASE
				WHEN status IN ('notready', 'open') THEN 'idle'
				WHEN status = 'inprogress' THEN 'active'
				WHEN status IN ('complete', 'fail') THEN 'complete'
				ELSE COALESCE(NULLIF(TRIM(state), ''), 'idle')
			END
		WHERE COALESCE(NULLIF(TRIM(stage), ''), '') = '' OR COALESCE(NULLIF(TRIM(state), ''), '') = ''
		   OR stage NOT IN ('design', 'develop', 'test', 'done')
		   OR state NOT IN ('idle', 'active', 'complete')
	`); err != nil {
		return err
	}
	if err := backfillProjectPrefixes(db); err != nil {
		return err
	}
	if err := backfillTicketKeys(db); err != nil {
		return err
	}
	if _, err := db.Exec(`
		INSERT INTO ticket_history (id, project_id, task_id, event_type, payload, created_by, created_at)
		SELECT h.id, h.project_id, h.task_id, h.event_type, h.payload, h.created_by, h.created_at
		FROM history_events h
		WHERE NOT EXISTS (SELECT 1 FROM ticket_history th WHERE th.id = h.id)
	`); err != nil {
		return err
	}
	return nil
}

func backfillProjectPrefixes(db *sql.DB) error {
	rows, err := db.Query(`SELECT project_id, title, prefix FROM projects ORDER BY project_id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id     int64
		title  string
		prefix string
	}
	var projects []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.title, &item.prefix); err != nil {
			return err
		}
		projects = append(projects, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, project := range projects {
		if strings.TrimSpace(project.prefix) != "" {
			if project.id == 1 && strings.TrimSpace(project.title) == "Default Project" {
				var taskCount int
				if err := db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id = ?`, project.id).Scan(&taskCount); err != nil {
					return err
				}
				if taskCount == 0 {
					prefix := defaultProjectPrefix
					if prefix != strings.TrimSpace(project.prefix) {
						prefix, err := nextUniqueProjectPrefix(db, prefix)
						if err != nil {
							return err
						}
						if _, err := db.Exec(`UPDATE projects SET prefix = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, prefix, project.id); err != nil {
							return err
						}
					}
				}
			}
			continue
		}
		desiredPrefix := deriveProjectPrefix(project.title)
		if project.id == 1 && strings.TrimSpace(project.title) == "Default Project" {
			desiredPrefix = defaultProjectPrefix
		}
		prefix, err := nextUniqueProjectPrefix(db, desiredPrefix)
		if err != nil {
			return err
		}
		if _, err := db.Exec(`UPDATE projects SET prefix = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, prefix, project.id); err != nil {
			return err
		}
	}
	return nil
}

func backfillTicketKeys(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT t.task_id, t.project_id, t.type, t.key, p.prefix
		FROM tasks t
		JOIN projects p ON p.project_id = t.project_id
		ORDER BY t.project_id, t.task_id
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		taskID    int64
		projectID int64
		taskType  string
		key       string
		prefix    string
	}
	var tasks []row
	maxSeq := map[int64]int64{}
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.taskID, &item.projectID, &item.taskType, &item.key, &item.prefix); err != nil {
			return err
		}
		tasks = append(tasks, item)
		if seq := parseTicketSequence(item.key); seq > maxSeq[item.projectID] {
			maxSeq[item.projectID] = seq
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, task := range tasks {
		if strings.TrimSpace(task.key) == "" {
			maxSeq[task.projectID]++
			key, err := generateTicketKey(task.prefix, task.taskType, maxSeq[task.projectID])
			if err != nil {
				return err
			}
			if _, err := db.Exec(`UPDATE tasks SET key = ? WHERE task_id = ?`, key, task.taskID); err != nil {
				return err
			}
		}
	}
	for projectID, seq := range maxSeq {
		if _, err := db.Exec(`UPDATE projects SET ticket_sequence = CASE WHEN ticket_sequence < ? THEN ? ELSE ticket_sequence END WHERE project_id = ?`, seq, seq, projectID); err != nil {
			return err
		}
	}
	return nil
}

func parseTicketSequence(key string) int64 {
	parts := strings.Split(strings.TrimSpace(key), "-")
	switch len(parts) {
	case 2:
		seq, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || seq < 0 {
			return 0
		}
		return seq
	case 3:
		seq, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || seq < 0 {
			return 0
		}
		return seq
	default:
		return 0
	}
}

func columnExists(db *sql.DB, tableName, columnName string) bool {
	rows, err := db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return false
		}
		if name == columnName {
			return true
		}
	}
	return false
}
