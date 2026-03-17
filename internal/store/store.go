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

	var defaultWorkflowID *int64
	var wfID int64
	if err := db.QueryRow(`SELECT workflow_id FROM workflows WHERE name = 'default'`).Scan(&wfID); err == nil {
		defaultWorkflowID = &wfID
	}
	if _, err := CreateProjectWithParams(db, ProjectCreateParams{
		Prefix:             defaultProjectPrefix,
		Title:              "Default Project",
		Description:        "Bootstrap project created during init.",
		AcceptanceCriteria: "",
		CreatedBy:          1,
		WorkflowID:         defaultWorkflowID,
	}); err != nil {
		return err
	}
	return nil
}

func createSchema(db *sql.DB) error {
	// Pre-schema migration: rename tasks→tickets BEFORE the CREATE TABLE IF
	// NOT EXISTS block runs, so we don't end up with both tables.
	if tableExists(db, "tasks") && !tableExists(db, "tickets") {
		if _, err := db.Exec(`ALTER TABLE tasks RENAME TO tickets`); err != nil {
			return err
		}
	}
	// If both tables exist (e.g. a previous run created an empty tickets table
	// while tasks still held data), merge tasks data into tickets and drop tasks.
	if tableExists(db, "tasks") && tableExists(db, "tickets") {
		// Map old column names to new ones for the tasks→tickets transfer.
		renames := map[string]string{"task_id": "ticket_id"}
		taskCols, _ := tableColumnNames(db, "tasks")
		ticketCols, _ := tableColumnNames(db, "tickets")
		ticketSet := make(map[string]bool, len(ticketCols))
		for _, c := range ticketCols {
			ticketSet[c] = true
		}
		var dstCols, srcCols []string
		for _, src := range taskCols {
			dst := src
			if mapped, ok := renames[src]; ok {
				dst = mapped
			}
			if ticketSet[dst] {
				dstCols = append(dstCols, dst)
				srcCols = append(srcCols, src)
			}
		}
		if len(dstCols) > 0 {
			if _, err := db.Exec(fmt.Sprintf(`INSERT OR IGNORE INTO tickets (%s) SELECT %s FROM tasks`,
				strings.Join(dstCols, ", "), strings.Join(srcCols, ", "))); err != nil {
				return err
			}
		}
		if _, err := db.Exec(`DROP TABLE tasks`); err != nil {
			return err
		}
	}

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

CREATE TABLE IF NOT EXISTS agents (
	agent_id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	password_hash TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	status TEXT NOT NULL DEFAULT 'idle',
	last_seen TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS roles (
	role_id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL UNIQUE,
	motivation TEXT NOT NULL DEFAULT '',
	goals TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS projects (
	project_id INTEGER PRIMARY KEY AUTOINCREMENT,
	prefix TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	git_repository TEXT NOT NULL DEFAULT '',
	git_branch TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'open',
	visibility TEXT NOT NULL DEFAULT 'public',
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	ticket_sequence INTEGER NOT NULL DEFAULT 0,
	workflow_id INTEGER,
	FOREIGN KEY(created_by) REFERENCES users(user_id),
	FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id)
);

CREATE TABLE IF NOT EXISTS tickets (
	ticket_id INTEGER PRIMARY KEY AUTOINCREMENT,
	key TEXT NOT NULL DEFAULT '',
	project_id INTEGER NOT NULL,
	parent_id INTEGER,
	clone_of INTEGER,
	type TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	git_repository TEXT NOT NULL DEFAULT '',
	git_branch TEXT NOT NULL DEFAULT '',
	workflow_stage_id INTEGER,
	stage TEXT NOT NULL DEFAULT 'design',
	state TEXT NOT NULL DEFAULT 'idle',
	status TEXT NOT NULL DEFAULT 'open',
	priority INTEGER NOT NULL DEFAULT 3,
	sort_order INTEGER NOT NULL DEFAULT 0,
	estimate_effort INTEGER NOT NULL DEFAULT 0,
	estimate_complete TEXT NOT NULL DEFAULT '',
	health_score INTEGER NOT NULL DEFAULT 0,
	assignee TEXT NOT NULL DEFAULT '',
	open INTEGER NOT NULL DEFAULT 1,
	archived INTEGER NOT NULL DEFAULT 0,
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(parent_id) REFERENCES tickets(ticket_id),
	FOREIGN KEY(clone_of) REFERENCES tickets(ticket_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id),
	FOREIGN KEY(workflow_stage_id) REFERENCES workflow_stages(workflow_stage_id)
);

CREATE TABLE IF NOT EXISTS stories (
	story_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'draft',
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS story_ticket_links (
	story_id INTEGER NOT NULL,
	ticket_id INTEGER NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(story_id, ticket_id),
	FOREIGN KEY(story_id) REFERENCES stories(story_id),
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id)
);

CREATE TABLE IF NOT EXISTS history_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	ticket_id INTEGER NOT NULL,
	event_type TEXT NOT NULL,
	payload TEXT NOT NULL DEFAULT '{}',
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS ticket_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	ticket_id INTEGER NOT NULL,
	event_type TEXT NOT NULL,
	payload TEXT NOT NULL DEFAULT '{}',
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS comments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	item_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	comment TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(item_id) REFERENCES tickets(ticket_id),
	FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS dependencies (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	ticket_id INTEGER NOT NULL,
	depends_on INTEGER NOT NULL,
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id),
	FOREIGN KEY(depends_on) REFERENCES tickets(ticket_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS app_settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS project_members (
	project_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	role TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(project_id, user_id),
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS teams (
	team_id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	parent_team_id INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(parent_team_id) REFERENCES teams(team_id)
);

CREATE TABLE IF NOT EXISTS team_members (
	team_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	role TEXT NOT NULL,
	job_title TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(team_id, user_id),
	FOREIGN KEY(team_id) REFERENCES teams(team_id),
	FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS team_agents (
	team_id INTEGER NOT NULL,
	agent_id INTEGER NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(team_id, agent_id),
	FOREIGN KEY(team_id) REFERENCES teams(team_id),
	FOREIGN KEY(agent_id) REFERENCES agents(agent_id)
);

CREATE TABLE IF NOT EXISTS project_teams (
	project_id INTEGER NOT NULL,
	team_id INTEGER NOT NULL,
	role TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(project_id, team_id),
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(team_id) REFERENCES teams(team_id)
);

CREATE TABLE IF NOT EXISTS workflows (
	workflow_id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS workflow_stages (
	workflow_stage_id INTEGER PRIMARY KEY AUTOINCREMENT,
	workflow_id INTEGER NOT NULL,
	stage_name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	role_id INTEGER,
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id),
	FOREIGN KEY(role_id) REFERENCES roles(role_id),
	UNIQUE(workflow_id, stage_name)
);
`

	if _, err := db.Exec(schema); err != nil {
		return err
	}
	return migrateSchema(db)
}

func migrateSchema(db *sql.DB) error {
	// Rename task_id columns to use ticket terminology.
	if columnExists(db, "tickets", "task_id") {
		if _, err := db.Exec(`ALTER TABLE tickets RENAME COLUMN task_id TO ticket_id`); err != nil {
			return err
		}
	}
	if columnExists(db, "history_events", "task_id") {
		if _, err := db.Exec(`ALTER TABLE history_events RENAME COLUMN task_id TO ticket_id`); err != nil {
			return err
		}
	}
	if columnExists(db, "ticket_history", "task_id") {
		if _, err := db.Exec(`ALTER TABLE ticket_history RENAME COLUMN task_id TO ticket_id`); err != nil {
			return err
		}
	}
	if columnExists(db, "dependencies", "task_id") {
		if _, err := db.Exec(`ALTER TABLE dependencies RENAME COLUMN task_id TO ticket_id`); err != nil {
			return err
		}
	}

	// Fix FK references that still point at the old "tasks" table after the rename.
	// Must run before any DML that would trigger FK checks on the affected tables.
	if err := fixStaleForeignKeys(db); err != nil {
		return err
	}

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
	if !columnExists(db, "projects", "git_repository") {
		if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN git_repository TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(db, "projects", "git_branch") {
		if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN git_branch TEXT NOT NULL DEFAULT ''`); err != nil {
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
	if !columnExists(db, "projects", "visibility") {
		if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public'`); err != nil {
			return err
		}
	}
	if !columnExists(db, "projects", "workflow_id") {
		if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN workflow_id INTEGER REFERENCES workflows(workflow_id)`); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`UPDATE projects SET status = 'open' WHERE status = 'active'`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE projects SET visibility = 'public' WHERE TRIM(COALESCE(visibility,'')) = ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE projects SET status = 'closed' WHERE status = 'disabled'`); err != nil {
		return err
	}

	if !columnExists(db, "tickets", "key") {
		if _, err := db.Exec(`ALTER TABLE tickets ADD COLUMN key TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tickets", "sort_order") {
		if _, err := db.Exec(`ALTER TABLE tickets ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tickets", "estimate_effort") {
		if _, err := db.Exec(`ALTER TABLE tickets ADD COLUMN estimate_effort INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tickets", "estimate_complete") {
		if _, err := db.Exec(`ALTER TABLE tickets ADD COLUMN estimate_complete TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tickets", "git_repository") {
		if _, err := db.Exec(`ALTER TABLE tickets ADD COLUMN git_repository TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tickets", "git_branch") {
		if _, err := db.Exec(`ALTER TABLE tickets ADD COLUMN git_branch TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tickets", "health_score") {
		if _, err := db.Exec(`ALTER TABLE tickets ADD COLUMN health_score INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tickets", "open") {
		if _, err := db.Exec(`ALTER TABLE tickets ADD COLUMN open INTEGER NOT NULL DEFAULT 1`); err != nil {
			return err
		}
		// Legacy behavior used archived for close/open. Preserve closed state and reset archived.
		if _, err := db.Exec(`UPDATE tickets SET open = CASE WHEN archived = 1 THEN 0 ELSE 1 END`); err != nil {
			return err
		}
		if _, err := db.Exec(`UPDATE tickets SET archived = 0`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tickets", "stage") {
		if _, err := db.Exec(`ALTER TABLE tickets ADD COLUMN stage TEXT NOT NULL DEFAULT 'design'`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tickets", "state") {
		if _, err := db.Exec(`ALTER TABLE tickets ADD COLUMN state TEXT NOT NULL DEFAULT 'idle'`); err != nil {
			return err
		}
	}
	if !columnExists(db, "tickets", "workflow_stage_id") {
		if _, err := db.Exec(`ALTER TABLE tickets ADD COLUMN workflow_stage_id INTEGER REFERENCES workflow_stages(workflow_stage_id)`); err != nil {
			return err
		}
	}
	if !tableExists(db, "stories") {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS stories (
				story_id INTEGER PRIMARY KEY AUTOINCREMENT,
				project_id INTEGER NOT NULL,
				title TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'draft',
				created_by INTEGER,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY(project_id) REFERENCES projects(project_id),
				FOREIGN KEY(created_by) REFERENCES users(user_id)
			)
		`); err != nil {
			return err
		}
	}
	if !tableExists(db, "story_ticket_links") {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS story_ticket_links (
				story_id INTEGER NOT NULL,
				ticket_id INTEGER NOT NULL,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY(story_id, ticket_id),
				FOREIGN KEY(story_id) REFERENCES stories(story_id),
				FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id)
			)
		`); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`
		UPDATE tickets
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
				WHEN LOWER(TRIM(state)) = 'complete' THEN 'success'
				WHEN LOWER(TRIM(state)) = 'fail' THEN 'fail'
				WHEN status IN ('notready', 'open') THEN 'idle'
				WHEN status = 'inprogress' THEN 'active'
				WHEN status = 'complete' THEN 'success'
				WHEN status = 'fail' THEN 'fail'
				ELSE COALESCE(NULLIF(TRIM(state), ''), 'idle')
			END
		WHERE COALESCE(NULLIF(TRIM(stage), ''), '') = '' OR COALESCE(NULLIF(TRIM(state), ''), '') = ''
		   OR stage NOT IN ('design', 'develop', 'test', 'done')
		   OR state NOT IN ('idle', 'active', 'success', 'fail')
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
		INSERT INTO ticket_history (id, project_id, ticket_id, event_type, payload, created_by, created_at)
		SELECT h.id, h.project_id, h.ticket_id, h.event_type, h.payload, h.created_by, h.created_at
		FROM history_events h
		WHERE NOT EXISTS (SELECT 1 FROM ticket_history th WHERE th.id = h.id)
	`); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO app_settings (key, value) VALUES ('registration_enabled', '1')`); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO app_settings (key, value) VALUES ('chat_max_connections', '2')`); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO app_settings (key, value) VALUES ('chat_max_duration_minutes', '3')`); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO app_settings (key, value) VALUES ('chat_enabled', '1')`); err != nil {
		return err
	}
	if _, err := db.Exec(`
		INSERT OR IGNORE INTO project_members (project_id, user_id, role)
		SELECT project_id, created_by, 'owner'
		FROM projects
		WHERE created_by IS NOT NULL AND created_by > 0
	`); err != nil {
		return err
	}
	if err := seedDefaultRoles(db); err != nil {
		return err
	}
	if err := seedDefaultWorkflow(db); err != nil {
		return err
	}
	if err := backfillTicketWorkflowStages(db); err != nil {
		return err
	}
	return nil
}

func backfillTicketWorkflowStages(db *sql.DB) error {
	// For tickets that have a stage name but no workflow_stage_id,
	// resolve the stage from the project's workflow.
	_, err := db.Exec(`
		UPDATE tickets
		SET workflow_stage_id = (
			SELECT ws.workflow_stage_id
			FROM projects p
			JOIN workflow_stages ws ON ws.workflow_id = p.workflow_id AND ws.stage_name = tickets.stage
			WHERE p.project_id = tickets.project_id
		)
		WHERE workflow_stage_id IS NULL
		  AND EXISTS (
			SELECT 1 FROM projects p
			JOIN workflow_stages ws ON ws.workflow_id = p.workflow_id AND ws.stage_name = tickets.stage
			WHERE p.project_id = tickets.project_id
		)
	`)
	return err
}

func seedDefaultWorkflow(db *sql.DB) error {
	result, err := db.Exec(`INSERT OR IGNORE INTO workflows (name, description, updated_at) VALUES ('default', 'Standard engineering lifecycle', CURRENT_TIMESTAMP)`)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil // already seeded
	}
	wfID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	stages := []struct {
		name      string
		roleTitle string
		order     int
	}{
		{"design", "BA", 0},
		{"develop", "Lead Engineer", 1},
		{"test", "QA/Tester", 2},
		{"done", "Product Owner", 3},
	}
	for _, s := range stages {
		var roleID *int64
		role, err := GetRoleByTitle(db, s.roleTitle)
		if err == nil {
			roleID = &role.ID
		}
		if _, err := db.Exec(`
			INSERT INTO workflow_stages (workflow_id, stage_name, role_id, sort_order, updated_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, wfID, s.name, roleID, s.order); err != nil {
			return err
		}
	}
	return nil
}

func seedDefaultRoles(db *sql.DB) error {
	type defaultRole struct {
		Title            string
		LegacyMotivation string
		LegacyGoals      string
		Motivation       string
		Goals            string
	}

	defaultRoles := []defaultRole{
		{
			Title:            "Product Owner",
			LegacyMotivation: "Maximize customer value and clarity.",
			LegacyGoals:      "Prioritize backlog and validate outcomes.",
			Motivation: `The Product Owner is the steward of value. This role gathers the many voices around a product and turns them into one accountable direction that the team can execute without ambiguity.

In classical delivery practice, the Product Owner protects both customer outcomes and engineering focus by deciding what matters now, what can wait, and what should never be built.`,
			Goals: `Shape and maintain a prioritized backlog whose ordering reflects business value, risk reduction, and delivery reality.

Define acceptance criteria that make success testable, and continuously validate delivered work with users and stakeholders so the team learns quickly and adjusts course deliberately.`,
		},
		{
			Title:            "Architect",
			LegacyMotivation: "Maintain coherent system design.",
			LegacyGoals:      "Define architecture guardrails and reduce complexity.",
			Motivation: `The Architect preserves structural integrity while change is constant. This role balances immediate delivery pressure against the long-term fitness of the system so that each release does not mortgage the next.

Classically, the Architect translates strategy into technical boundaries: where variation is welcome, where standards are required, and where simplification must be enforced.`,
			Goals: `Set architecture principles, interfaces, and constraints that allow teams to move independently without creating fragmentation.

Drive intentional reduction of complexity through clear decomposition, dependable integration points, and regular review of hotspots where cost, risk, and coupling are rising.`,
		},
		{
			Title:            "DevOps",
			LegacyMotivation: "Improve delivery reliability and speed.",
			LegacyGoals:      "Automate build/deploy and keep systems stable.",
			Motivation: `DevOps exists to eliminate the false trade-off between speed and stability. The role champions delivery systems that are repeatable, observable, and safe under real operational load.

In the classical sense, DevOps is a reliability discipline as much as a tooling discipline: remove toil, shorten feedback loops, and make production behavior visible to everyone.`,
			Goals: `Build and maintain automated pipelines for build, test, release, and rollback so deployment becomes routine rather than exceptional.

Improve operational excellence through monitoring, alerting, incident learning, and capacity planning, keeping service health and change lead time in continuous balance.`,
		},
		{
			Title:            "QA/Tester",
			LegacyMotivation: "Protect product quality.",
			LegacyGoals:      "Design tests and prevent regressions.",
			Motivation: `QA/Tester represents disciplined skepticism in the delivery process. This role asks how a feature can fail in practice and ensures quality is demonstrated, not merely asserted.

Classical testing practice positions QA as a partner in design and risk management, helping teams discover defects early and prevent classes of failure before release.`,
			Goals: `Create layered test strategies across functional, integration, and exploratory testing that reflect product risk and user impact.

Strengthen regression safety by improving test automation, test data quality, and defect feedback loops so each release increases confidence instead of uncertainty.`,
		},
		{
			Title:            "BA",
			LegacyMotivation: "Turn business needs into implementable requirements.",
			LegacyGoals:      "Clarify scope and acceptance criteria.",
			Motivation: `The Business Analyst translates intent into precision. This role turns broad business goals into clear problem statements, measurable outcomes, and decisions that teams can implement without rework.

Classically, BA work reduces waste by exposing assumptions early, surfacing dependencies, and creating shared understanding before expensive development begins.`,
			Goals: `Elicit and document requirements in language that is concrete for engineering and meaningful for stakeholders, including explicit boundaries and constraints.

Maintain alignment between scope, process, and outcomes by refining acceptance criteria, tracking requirement changes, and resolving ambiguity before it reaches implementation.`,
		},
		{
			Title:            "Lead Engineer",
			LegacyMotivation: "Deliver technically sound features.",
			LegacyGoals:      "Guide implementation and unblock execution.",
			Motivation: `The Lead Engineer is accountable for turning plans into high-quality software under real timeline pressure. This role keeps the team moving while preserving technical standards and delivery discipline.

In classical engineering leadership, the lead is both builder and coordinator: clarifying trade-offs, sequencing work, and intervening quickly when execution drifts.`,
			Goals: `Guide implementation across design, coding, and review practices so feature delivery remains coherent, testable, and maintainable.

Actively unblock execution by resolving technical uncertainty, coordinating cross-role decisions, and maintaining a predictable cadence from development through handoff.`,
		},
		{
			Title:            "Staff Engineer",
			LegacyMotivation: "Raise cross-team technical quality.",
			LegacyGoals:      "Drive long-term engineering improvements.",
			Motivation: `The Staff Engineer operates at system and organization scale. This role focuses on problems that exceed a single team boundary and improves how engineering works across the whole product surface.

Classically, staff-level impact is measured in leverage: better defaults, better standards, and better technical decisions that make many teams more effective over time.`,
			Goals: `Lead cross-team initiatives that improve architecture consistency, platform capability, and engineering effectiveness without centralizing all decision-making.

Advance long-term technical quality by identifying systemic risks, shaping roadmaps for foundational work, and mentoring engineers in high-leverage design and execution patterns.`,
		},
		{
			Title:            "StoryReview",
			LegacyMotivation: "",
			LegacyGoals:      "",
			Motivation: `StoryReview converts high-level requirements into a coherent implementation shape. The role ensures scope is represented as outcome-focused epics and delivery-focused tickets that can be executed incrementally.

This role emphasizes completeness, clear boundaries, and actionable decomposition that engineering teams can review and implement with minimal ambiguity.`,
			Goals: `Break each story into a small set of epics that represent major capability slices, then derive concrete implementation tickets for each epic.

Ensure generated work items have clear intent, practical scope, and traceability back to the parent story for review and approval.`,
		},
		{
			Title:            "EpicReview",
			LegacyMotivation: "",
			LegacyGoals:      "",
			Motivation: `EpicReview translates a strategic epic into executable tickets. The role focuses on identifying practical delivery steps that preserve architectural integrity while enabling fast implementation.

This role acts as the bridge between planning and coding by turning broad capability statements into clear, testable work units.`,
			Goals: `Decompose epics into implementation tickets with well-defined titles and descriptions that support estimation and assignment.

Produce tickets that are specific enough for immediate development while maintaining linkage to the parent epic and story context.`,
		},
	}
	for _, role := range defaultRoles {
		if _, err := db.Exec(`
			INSERT OR IGNORE INTO roles (title, motivation, goals, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		`, role.Title, role.Motivation, role.Goals); err != nil {
			return err
		}
		if _, err := db.Exec(`
			UPDATE roles
			SET motivation = ?, goals = ?, updated_at = CURRENT_TIMESTAMP
			WHERE title = ?
			  AND (
			    (motivation = ? AND goals = ?)
			    OR (TRIM(motivation) = '' AND TRIM(goals) = '')
			  )
		`, role.Motivation, role.Goals, role.Title, role.LegacyMotivation, role.LegacyGoals); err != nil {
			return err
		}
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
				var ticketCount int
				if err := db.QueryRow(`SELECT COUNT(*) FROM tickets WHERE project_id = ?`, project.id).Scan(&ticketCount); err != nil {
					return err
				}
				if ticketCount == 0 {
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
		SELECT t.ticket_id, t.project_id, t.type, t.key, p.prefix
		FROM tickets t
		JOIN projects p ON p.project_id = t.project_id
		ORDER BY t.project_id, t.ticket_id
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		ticketID   int64
		projectID  int64
		ticketType string
		key        string
		prefix     string
	}
	var tickets []row
	maxSeq := map[int64]int64{}
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.ticketID, &item.projectID, &item.ticketType, &item.key, &item.prefix); err != nil {
			return err
		}
		tickets = append(tickets, item)
		if seq := parseTicketSequence(item.key); seq > maxSeq[item.projectID] {
			maxSeq[item.projectID] = seq
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, ticket := range tickets {
		if strings.TrimSpace(ticket.key) == "" {
			maxSeq[ticket.projectID]++
			key, err := generateTicketKey(ticket.prefix, ticket.ticketType, maxSeq[ticket.projectID])
			if err != nil {
				return err
			}
			if _, err := db.Exec(`UPDATE tickets SET key = ? WHERE ticket_id = ?`, key, ticket.ticketID); err != nil {
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

// fixStaleForeignKeys recreates tables whose FOREIGN KEY references still
// point at the old "tasks" table after the tasks→tickets rename. SQLite does
// not support ALTER TABLE to change FK constraints, so we must recreate.
func fixStaleForeignKeys(db *sql.DB) error {
	if !tableHsFKTo(db, "history_events", "tasks") {
		return nil // already migrated
	}

	// Temporarily disable FK checks so we can drop/recreate without cascading errors.
	if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer db.Exec(`PRAGMA foreign_keys = ON`) //nolint:errcheck

	type tableMigration struct {
		name   string
		create string
	}
	migrations := []tableMigration{
		{
			name: "history_events",
			create: `CREATE TABLE history_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				project_id INTEGER NOT NULL,
				ticket_id INTEGER NOT NULL,
				event_type TEXT NOT NULL,
				payload TEXT NOT NULL DEFAULT '{}',
				created_by INTEGER,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY(project_id) REFERENCES projects(project_id),
				FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id),
				FOREIGN KEY(created_by) REFERENCES users(user_id)
			)`,
		},
		{
			name: "ticket_history",
			create: `CREATE TABLE ticket_history (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				project_id INTEGER NOT NULL,
				ticket_id INTEGER NOT NULL,
				event_type TEXT NOT NULL,
				payload TEXT NOT NULL DEFAULT '{}',
				created_by INTEGER,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY(project_id) REFERENCES projects(project_id),
				FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id),
				FOREIGN KEY(created_by) REFERENCES users(user_id)
			)`,
		},
		{
			name: "comments",
			create: `CREATE TABLE comments (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				item_id INTEGER NOT NULL,
				user_id INTEGER NOT NULL,
				comment TEXT NOT NULL,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY(item_id) REFERENCES tickets(ticket_id),
				FOREIGN KEY(user_id) REFERENCES users(user_id)
			)`,
		},
		{
			name: "dependencies",
			create: `CREATE TABLE dependencies (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				project_id INTEGER NOT NULL,
				ticket_id INTEGER NOT NULL,
				depends_on INTEGER NOT NULL,
				created_by INTEGER,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY(project_id) REFERENCES projects(project_id),
				FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id),
				FOREIGN KEY(depends_on) REFERENCES tickets(ticket_id),
				FOREIGN KEY(created_by) REFERENCES users(user_id)
			)`,
		},
	}

	for _, m := range migrations {
		if !tableHsFKTo(db, m.name, "tasks") {
			continue
		}
		tmpName := m.name + "_migrate_tmp"
		if _, err := db.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, m.name, tmpName)); err != nil {
			return fmt.Errorf("rename %s: %w", m.name, err)
		}
		if _, err := db.Exec(m.create); err != nil {
			return fmt.Errorf("create %s: %w", m.name, err)
		}
		if _, err := db.Exec(fmt.Sprintf(`INSERT INTO %s SELECT * FROM %s`, m.name, tmpName)); err != nil {
			return fmt.Errorf("copy %s: %w", m.name, err)
		}
		if _, err := db.Exec(fmt.Sprintf(`DROP TABLE %s`, tmpName)); err != nil {
			return fmt.Errorf("drop %s: %w", tmpName, err)
		}
	}
	return nil
}

// tableHsFKTo returns true if the table has a foreign key referencing the given table.
func tableHsFKTo(db *sql.DB, tableName, refTable string) bool {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA foreign_key_list(%s)`, tableName))
	if err != nil {
		return false
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return false
		}
		// Column index 2 is the referenced table name.
		if ref, ok := vals[2].(string); ok && ref == refTable {
			return true
		}
	}
	return false
}

func tableExists(db *sql.DB, tableName string) bool {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tableName).Scan(&count)
	return err == nil && count > 0
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
