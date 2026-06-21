package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

const defaultProjectPrefix = "TK"

func Open(path string) (*sql.DB, error) {
	ctx := context.Background()
	existed := path == ":memory:"
	if !existed {
		if _, err := os.Stat(path); err == nil {
			existed = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	db, err := openSQLite(path)
	if err != nil {
		return nil, err
	}

	empty, err := databaseHasNoUserTables(ctx, db)
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("store: close db after empty-database check failure: %v", closeErr)
		}
		return nil, err
	}
	if !existed || empty {
		if err := createSchema(ctx, db); err != nil {
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("store: close db after schema creation failure: %v", closeErr)
			}
			return nil, err
		}
	} else {
		version, err := readSchemaVersion(ctx, db)
		if err != nil {
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("store: close db after schema version check failure: %v", closeErr)
			}
			return nil, err
		}
		if version != CurrentSchemaVersion {
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("store: close db after schema version mismatch: %v", closeErr)
			}
			return nil, &SchemaVersionError{
				Path:          path,
				Found:         version,
				Current:       CurrentSchemaVersion,
				UpgradeNeeded: version < CurrentSchemaVersion,
			}
		}
	}
	if err := enableWAL(ctx, db); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("store: close db after journal_mode pragma failure: %v", closeErr)
		}
		return nil, err
	}
	return db, nil
}

// SeedFunc is called during Init to seed the database with Workflows and roles.
// It receives the opened database and should create at least one Workflow.
// The first Workflow found after seeding is assigned to the default project.
type SeedFunc func(ctx context.Context, db *sql.DB) error

// Init creates a new database at path with seeded plans/resources and an admin user.
// The seedFn (if non-nil) is called to populate Workflows and roles from embedded
// static files. If nil, no Workflows are created and the project has no lifecycle.
func Init(path, adminUsername, adminPassword string, seedFn ...SeedFunc) error {
	ctx := context.Background()
	if adminUsername == "" || adminPassword == "" {
		return errors.New("admin username and password are required")
	}

	if path != ":memory:" {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("database already exists at %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil && filepath.Dir(path) != "." {
			return err
		}
	}

	db, err := Open(path)
	if err != nil {
		return err
	}
	defer db.Close()

	if schemaErr := createSchema(ctx, db); schemaErr != nil {
		return schemaErr
	}

	// Run the seed function to populate Workflows and roles.
	// If no seed function is provided, create a minimal develop→done Workflow
	// so tickets always have valid stages.
	if len(seedFn) > 0 && seedFn[0] != nil {
		if seedErr := seedFn[0](ctx, db); seedErr != nil {
			return seedErr
		}
	} else {
		workflow, sErr := CreateWorkflow(ctx, db, "default", "Minimal bootstrap Workflow")
		if sErr == nil {
			for i, name := range []string{StageIdea, StageRefine, StageReady, StageDevelop, StageComplete} {
				isBacklog := name == StageIdea || name == StageRefine || name == StageReady
				stage, stageErr := AddWorkflowStage(ctx, db, workflow.ID, name, "", "", i)
				if stageErr != nil {
					return stageErr
				}
				if isBacklog {
					if backlogErr := SetWorkflowStageBacklog(ctx, db, stage.ID, true); backlogErr != nil {
						return backlogErr
					}
				}
			}
		}
	}

	if planErr := ensureDefaultPlans(ctx, db); planErr != nil {
		return planErr
	}
	adminUser, err := CreateUserWithParams(ctx, db, UserCreateParams{
		Username:               adminUsername,
		PlainPassword:          adminPassword,
		Role:                   "admin",
		Enabled:                true,
		PlanSlug:               EnterprisePlanSlug,
		SkipPasswordValidation: true,
	})
	if err != nil {
		return err
	}
	if _, _, err := ensurePublicResources(ctx, db, adminUser.ID); err != nil {
		return err
	}
	if _, err := ensureBootstrapTicketProject(ctx, db, adminUser.ID); err != nil {
		return err
	}
	return nil
}

func createSchema(ctx context.Context, db *sql.DB) error {
	// Pre-schema migration: rename tasks→tickets BEFORE the CREATE TABLE IF
	// NOT EXISTS block runs, so we don't end up with both tables.
	if tableExists(ctx, db, "tasks") && !tableExists(ctx, db, "tickets") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tasks RENAME TO tickets`); err != nil {
			return err
		}
	}
	// If both tables exist (e.g. a previous run created an empty tickets table
	// while tasks still held data), merge tasks data into tickets and drop tasks.
	if tableExists(ctx, db, "tasks") && tableExists(ctx, db, "tickets") {
		// Map old column names to new ones for the tasks→tickets transfer.
		renames := map[string]string{"task_id": "ticket_id"}
		taskCols, _ := tableColumnNames(ctx, db, "tasks")
		ticketCols, _ := tableColumnNames(ctx, db, "tickets")
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
			if _, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT OR IGNORE INTO tickets (%s) SELECT %s FROM tasks`,
				strings.Join(dstCols, ", "), strings.Join(srcCols, ", "))); err != nil {
				return err
			}
		}
		if _, err := db.ExecContext(ctx, `DROP TABLE tasks`); err != nil {
			return err
		}
	}

	schema := `
CREATE TABLE IF NOT EXISTS users (
	user_id TEXT PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL,
	plan_id INTEGER,
	default_project_id INTEGER,
	display_name TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	user_type TEXT NOT NULL DEFAULT 'user',
	uuid TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT '',
	last_seen TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS plans (
	plan_id INTEGER PRIMARY KEY AUTOINCREMENT,
	slug TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	max_projects INTEGER NOT NULL DEFAULT 0,
	max_private_projects INTEGER NOT NULL DEFAULT 0,
	max_tickets INTEGER NOT NULL DEFAULT 0,
	max_tickets_per_project INTEGER NOT NULL DEFAULT 0,
	max_team_memberships INTEGER NOT NULL DEFAULT 0,
	max_api_calls_per_day INTEGER NOT NULL DEFAULT 0,
	default_project_alias TEXT NOT NULL DEFAULT '',
	registration_actions TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
	session_id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	token TEXT NOT NULL UNIQUE,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at TEXT,
	FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS passkey_credentials (
	credential_id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	credential_json TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_used_at TEXT,
	FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS passkey_flows (
	flow_code TEXT PRIMARY KEY,
	purpose TEXT NOT NULL,
	user_id TEXT NOT NULL,
	credential_name TEXT NOT NULL DEFAULT '',
	session_json TEXT NOT NULL,
	options_json TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	token TEXT NOT NULL DEFAULT '',
	error_message TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at TEXT NOT NULL,
	completed_at TEXT,
	consumed_at TEXT,
	FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS roles (
	role_id INTEGER PRIMARY KEY AUTOINCREMENT,
	workflow_id INTEGER,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	dor_map TEXT NOT NULL DEFAULT '{}',
	dod_map TEXT NOT NULL DEFAULT '{}',
	ac_map TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id),
	UNIQUE(workflow_id, title)
);

CREATE TABLE IF NOT EXISTS projects (
	project_id INTEGER PRIMARY KEY AUTOINCREMENT,
	prefix TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	dor_map TEXT NOT NULL DEFAULT '{}',
	dod_map TEXT NOT NULL DEFAULT '{}',
	ac_map TEXT NOT NULL DEFAULT '{}',
	git_repository TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'open',
	visibility TEXT NOT NULL DEFAULT 'public',
	created_by TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	ticket_sequence INTEGER NOT NULL DEFAULT 0,
	default_draft INTEGER NOT NULL DEFAULT 0,
	workflow_id INTEGER,
	agent_model_provider TEXT NOT NULL DEFAULT '',
	agent_model_name TEXT NOT NULL DEFAULT '',
	agent_model_url TEXT NOT NULL DEFAULT '',
	agent_model_api_key TEXT NOT NULL DEFAULT '',
	FOREIGN KEY(created_by) REFERENCES users(user_id),
	FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id)
);

CREATE TABLE IF NOT EXISTS releases (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	purpose TEXT NOT NULL DEFAULT '',
	target_date TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'in_design',
	designed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	started_at TEXT NOT NULL DEFAULT '',
	completed_at TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id)
);

CREATE TABLE IF NOT EXISTS tickets (
	ticket_id TEXT PRIMARY KEY,
	project_id INTEGER NOT NULL,
	parent_id TEXT,
	clone_of TEXT,
	type TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	dor_map TEXT NOT NULL DEFAULT '{}',
	dod_map TEXT NOT NULL DEFAULT '{}',
	ac_map TEXT NOT NULL DEFAULT '{}',
	git_repository TEXT NOT NULL DEFAULT '',
	git_branch TEXT NOT NULL DEFAULT '',
	workflow_stage_id INTEGER,
	role_id INTEGER,
	stage TEXT NOT NULL DEFAULT 'develop',
	state TEXT NOT NULL DEFAULT 'idle',
	status TEXT NOT NULL DEFAULT 'develop/idle',
	priority INTEGER NOT NULL DEFAULT 3,
	sort_order INTEGER NOT NULL DEFAULT 0,
	estimate_effort INTEGER NOT NULL DEFAULT 0,
	estimate_complete TEXT NOT NULL DEFAULT '',
	health_score INTEGER NOT NULL DEFAULT 0,
	assignee TEXT NOT NULL DEFAULT '',
	draft INTEGER NOT NULL DEFAULT 1,
	complete INTEGER NOT NULL DEFAULT 0,
	archived INTEGER NOT NULL DEFAULT 0,
	deleted INTEGER NOT NULL DEFAULT 0,
	previous_workflow_stage_id INTEGER,
	previous_role_id INTEGER,
	release_id INTEGER,
	created_by TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(parent_id) REFERENCES tickets(ticket_id),
	FOREIGN KEY(clone_of) REFERENCES tickets(ticket_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id),
	FOREIGN KEY(workflow_stage_id) REFERENCES workflow_stages(workflow_stage_id),
	FOREIGN KEY(role_id) REFERENCES roles(role_id),
	FOREIGN KEY(release_id) REFERENCES releases(id)
);

CREATE TABLE IF NOT EXISTS stories (
	story_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'draft',
	created_by TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS story_ticket_links (
	story_id INTEGER NOT NULL,
	ticket_id TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(story_id, ticket_id),
	FOREIGN KEY(story_id) REFERENCES stories(story_id) ON DELETE CASCADE,
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS history_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	ticket_id TEXT NOT NULL,
	event_type TEXT NOT NULL,
	payload TEXT NOT NULL DEFAULT '{}',
	created_by TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE,
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE,
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS ticket_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	ticket_id TEXT,
	event_type TEXT NOT NULL,
	payload TEXT NOT NULL DEFAULT '{}',
	created_by TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE,
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE,
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS work_items (
	work_item_id TEXT PRIMARY KEY,
	ticket_id TEXT NOT NULL,
	project_id INTEGER NOT NULL,
	workflow_id INTEGER,
	workflow_stage_id INTEGER,
	role_id INTEGER,
	status TEXT NOT NULL DEFAULT 'active',
	assignee_type TEXT NOT NULL DEFAULT 'human',
	assignee_id TEXT NOT NULL DEFAULT '',
	objective_snapshot TEXT NOT NULL DEFAULT '',
	prompt_snapshot TEXT NOT NULL DEFAULT '',
	feedback TEXT NOT NULL DEFAULT '',
	commit_ref TEXT NOT NULL DEFAULT '',
	started_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	completed_at TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE,
	FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE,
	FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id),
	FOREIGN KEY(workflow_stage_id) REFERENCES workflow_stages(workflow_stage_id),
	FOREIGN KEY(role_id) REFERENCES roles(role_id)
);

CREATE TABLE IF NOT EXISTS comments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	item_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	comment TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(item_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE,
	FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS dependencies (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	ticket_id TEXT NOT NULL,
	depends_on TEXT NOT NULL,
	created_by TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE,
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE,
	FOREIGN KEY(depends_on) REFERENCES tickets(ticket_id) ON DELETE CASCADE,
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS app_settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS project_members (
	project_id INTEGER NOT NULL,
	user_id TEXT NOT NULL,
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
	user_id TEXT NOT NULL,
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
	user_id TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(team_id, user_id),
	FOREIGN KEY(team_id) REFERENCES teams(team_id),
	FOREIGN KEY(user_id) REFERENCES users(user_id)
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

CREATE TABLE IF NOT EXISTS project_aliases (
	alias_name TEXT NOT NULL,
	user_id TEXT,
	project_id INTEGER NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(alias_name, user_id),
	FOREIGN KEY(user_id) REFERENCES users(user_id),
	FOREIGN KEY(project_id) REFERENCES projects(project_id)
);

CREATE TABLE IF NOT EXISTS workflows (
	workflow_id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	approval_policy TEXT NOT NULL DEFAULT 'single_role',
	progression_mode TEXT NOT NULL DEFAULT 'linear',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS labels (
	label_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	color TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	UNIQUE(project_id, name)
);

CREATE TABLE IF NOT EXISTS ticket_labels (
	ticket_id TEXT NOT NULL,
	label_id INTEGER NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(ticket_id, label_id),
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE,
	FOREIGN KEY(label_id) REFERENCES labels(label_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS time_entries (
	time_entry_id INTEGER PRIMARY KEY AUTOINCREMENT,
	ticket_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	minutes INTEGER NOT NULL,
	note TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE,
	FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS workflow_stages (
	workflow_stage_id INTEGER PRIMARY KEY AUTOINCREMENT,
	workflow_id INTEGER NOT NULL,
	stage_name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL DEFAULT 0,
	is_backlog_stage INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id),
	UNIQUE(workflow_id, stage_name)
);

CREATE TABLE IF NOT EXISTS workflow_stage_roles (
	workflow_id INTEGER NOT NULL,
	stage_id INTEGER NOT NULL,
	role_id INTEGER NOT NULL,
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(workflow_id, stage_id, role_id),
	FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id),
	FOREIGN KEY(stage_id) REFERENCES workflow_stages(workflow_stage_id),
	FOREIGN KEY(role_id) REFERENCES roles(role_id)
);

CREATE TABLE IF NOT EXISTS messages (
	message_id INTEGER PRIMARY KEY AUTOINCREMENT,
	from_user_id TEXT NOT NULL,
	to_user_id TEXT NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	body TEXT NOT NULL DEFAULT '',
	medium TEXT NOT NULL DEFAULT 'dm',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	sent_at TEXT NOT NULL DEFAULT '',
	received_at TEXT NOT NULL DEFAULT '',
	FOREIGN KEY(from_user_id) REFERENCES users(user_id),
	FOREIGN KEY(to_user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS documents (
	document_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id)
);

CREATE TABLE IF NOT EXISTS document_labels (
	document_id INTEGER NOT NULL,
	label_id INTEGER NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(document_id, label_id),
	FOREIGN KEY(document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY(label_id) REFERENCES labels(label_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS document_files (
	file_id INTEGER PRIMARY KEY AUTOINCREMENT,
	document_id INTEGER NOT NULL,
	file_name TEXT NOT NULL,
	content_type TEXT NOT NULL DEFAULT '',
	size_bytes INTEGER NOT NULL DEFAULT 0,
	content BLOB NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(document_id) REFERENCES documents(document_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS context_edges (
	edge_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	source_type TEXT NOT NULL,
	source_id TEXT NOT NULL,
	target_type TEXT NOT NULL,
	target_id TEXT NOT NULL,
	relation TEXT NOT NULL DEFAULT 'references',
	title TEXT NOT NULL DEFAULT '',
	created_by TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(project_id, source_type, source_id, target_type, target_id, relation),
	FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS ticket_phase_signoffs (
	ticket_id TEXT NOT NULL,
	phase TEXT NOT NULL,
	approved INTEGER NOT NULL DEFAULT 0,
	approved_by TEXT,
	note TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(ticket_id, phase),
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE,
	FOREIGN KEY(approved_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS inbox_entries (
	inbox_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	ticket_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'open',
	recommendations_json TEXT NOT NULL DEFAULT '[]',
	decision TEXT NOT NULL DEFAULT '',
	message TEXT NOT NULL DEFAULT '',
	created_by TEXT,
	decided_by TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE,
	FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE,
	FOREIGN KEY(created_by) REFERENCES users(user_id),
	FOREIGN KEY(decided_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS user_notifications (
	notification_id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'unread',
	title TEXT NOT NULL,
	message TEXT NOT NULL DEFAULT '',
	payload_json TEXT NOT NULL DEFAULT '{}',
	read_at TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS org (
	id INTEGER PRIMARY KEY DEFAULT 1 CHECK(id = 1),
	name TEXT NOT NULL DEFAULT '',
	domain TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	logo_url TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS programmes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_passkey_credentials_user_id ON passkey_credentials(user_id);
CREATE INDEX IF NOT EXISTS idx_passkey_flows_user_id ON passkey_flows(user_id);
CREATE INDEX IF NOT EXISTS idx_passkey_flows_expires_at ON passkey_flows(expires_at);

CREATE INDEX IF NOT EXISTS idx_stories_project_id ON stories(project_id);

CREATE INDEX IF NOT EXISTS idx_story_ticket_links_ticket_id ON story_ticket_links(ticket_id);

CREATE INDEX IF NOT EXISTS idx_history_events_project_id ON history_events(project_id);
CREATE INDEX IF NOT EXISTS idx_history_events_ticket_id ON history_events(ticket_id);

CREATE INDEX IF NOT EXISTS idx_ticket_history_project_id ON ticket_history(project_id);
CREATE INDEX IF NOT EXISTS idx_ticket_history_ticket_id ON ticket_history(ticket_id);
CREATE INDEX IF NOT EXISTS idx_work_items_ticket_id ON work_items(ticket_id);
CREATE INDEX IF NOT EXISTS idx_work_items_project_id ON work_items(project_id);
CREATE INDEX IF NOT EXISTS idx_work_items_status ON work_items(status);

CREATE INDEX IF NOT EXISTS idx_comments_item_id ON comments(item_id);
CREATE INDEX IF NOT EXISTS idx_comments_user_id ON comments(user_id);

CREATE INDEX IF NOT EXISTS idx_dependencies_project_id ON dependencies(project_id);
CREATE INDEX IF NOT EXISTS idx_dependencies_ticket_id ON dependencies(ticket_id);
CREATE INDEX IF NOT EXISTS idx_dependencies_depends_on ON dependencies(depends_on);

CREATE INDEX IF NOT EXISTS idx_labels_project_id ON labels(project_id);

CREATE INDEX IF NOT EXISTS idx_ticket_labels_label_id ON ticket_labels(label_id);
CREATE INDEX IF NOT EXISTS idx_ticket_labels_ticket_id ON ticket_labels(ticket_id);

CREATE INDEX IF NOT EXISTS idx_project_members_user_id ON project_members(user_id);
CREATE INDEX IF NOT EXISTS idx_team_members_user_id ON team_members(user_id);
CREATE INDEX IF NOT EXISTS idx_team_agents_user_id ON team_agents(user_id);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_messages_from_user_id ON messages(from_user_id);
CREATE INDEX IF NOT EXISTS idx_messages_to_user_id ON messages(to_user_id);
CREATE INDEX IF NOT EXISTS idx_documents_project_id ON documents(project_id);
CREATE INDEX IF NOT EXISTS idx_document_labels_label_id ON document_labels(label_id);
CREATE INDEX IF NOT EXISTS idx_document_files_document_id ON document_files(document_id);
CREATE INDEX IF NOT EXISTS idx_context_edges_project_id ON context_edges(project_id);
CREATE INDEX IF NOT EXISTS idx_context_edges_source ON context_edges(source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_context_edges_target ON context_edges(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_ticket_phase_signoffs_ticket_id ON ticket_phase_signoffs(ticket_id);
CREATE INDEX IF NOT EXISTS idx_ticket_phase_signoffs_phase ON ticket_phase_signoffs(phase);
CREATE INDEX IF NOT EXISTS idx_inbox_entries_project_id ON inbox_entries(project_id);
CREATE INDEX IF NOT EXISTS idx_inbox_entries_ticket_id ON inbox_entries(ticket_id);
CREATE INDEX IF NOT EXISTS idx_inbox_entries_status ON inbox_entries(status);
CREATE INDEX IF NOT EXISTS idx_user_notifications_user_id ON user_notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_user_notifications_status ON user_notifications(status);

CREATE INDEX IF NOT EXISTS idx_time_entries_ticket_id ON time_entries(ticket_id);
CREATE INDEX IF NOT EXISTS idx_time_entries_user_id ON time_entries(user_id);

`

	if _, err := db.ExecContext(ctx, schema); err != nil {
		return err
	}
	if err := migrateSchema(ctx, db); err != nil {
		return err
	}
	return writeSchemaVersion(ctx, db, CurrentSchemaVersion)
}

func migrateSchema(ctx context.Context, db *sql.DB) error {
	// Disable FK checks for all migrations; re-enable at the end.
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer db.ExecContext(ctx, `PRAGMA foreign_keys = ON`) //nolint:errcheck // best-effort restore of FK enforcement

	// Rename task_id columns to use ticket terminology.
	if columnExists(ctx, db, "tickets", "task_id") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets RENAME COLUMN task_id TO ticket_id`); err != nil {
			return err
		}
	}
	if columnExists(ctx, db, "history_events", "task_id") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE history_events RENAME COLUMN task_id TO ticket_id`); err != nil {
			return err
		}
	}
	if columnExists(ctx, db, "ticket_history", "task_id") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE ticket_history RENAME COLUMN task_id TO ticket_id`); err != nil {
			return err
		}
	}
	if columnExists(ctx, db, "dependencies", "task_id") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE dependencies RENAME COLUMN task_id TO ticket_id`); err != nil {
			return err
		}
	}

	// Fix FK references that still point at the old "tasks" table after the rename.
	// Must run before any DML that would trigger FK checks on the affected tables.
	if err := fixStaleForeignKeys(ctx, db); err != nil {
		return err
	}

	if !columnExists(ctx, db, "projects", "prefix") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN prefix TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !tableExists(ctx, db, "documents") {
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE documents (
				document_id INTEGER PRIMARY KEY AUTOINCREMENT,
				project_id INTEGER NOT NULL,
				title TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				notes TEXT NOT NULL DEFAULT '',
				content TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY(project_id) REFERENCES projects(project_id)
			)
		`); err != nil {
			return err
		}
	}
	if !tableExists(ctx, db, "document_labels") {
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE document_labels (
				document_id INTEGER NOT NULL,
				label_id INTEGER NOT NULL,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY(document_id, label_id),
				FOREIGN KEY(document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
				FOREIGN KEY(label_id) REFERENCES labels(label_id) ON DELETE CASCADE
			)
		`); err != nil {
			return err
		}
	}
	if !tableExists(ctx, db, "document_files") {
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE document_files (
				file_id INTEGER PRIMARY KEY AUTOINCREMENT,
				document_id INTEGER NOT NULL,
				file_name TEXT NOT NULL,
				content_type TEXT NOT NULL DEFAULT '',
				size_bytes INTEGER NOT NULL DEFAULT 0,
				content BLOB NOT NULL,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY(document_id) REFERENCES documents(document_id) ON DELETE CASCADE
			)
		`); err != nil {
			return err
		}
	}
	if !tableExists(ctx, db, "context_edges") {
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE context_edges (
				edge_id INTEGER PRIMARY KEY AUTOINCREMENT,
				project_id INTEGER NOT NULL,
				source_type TEXT NOT NULL,
				source_id TEXT NOT NULL,
				target_type TEXT NOT NULL,
				target_id TEXT NOT NULL,
				relation TEXT NOT NULL DEFAULT 'references',
				title TEXT NOT NULL DEFAULT '',
				created_by TEXT,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(project_id, source_type, source_id, target_type, target_id, relation),
				FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE
			)
		`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "notes") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN notes TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "git_repository") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN git_repository TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "dor_map") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN dor_map TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "dod_map") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN dod_map TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "ac_map") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN ac_map TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if columnExists(ctx, db, "projects", "git_branch") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects DROP COLUMN git_branch`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "updated_at") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "ticket_sequence") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN ticket_sequence INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "visibility") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public'`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "workflow_id") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN workflow_id INTEGER REFERENCES workflows(workflow_id)`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "agent_model_provider") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN agent_model_provider TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "agent_model_name") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN agent_model_name TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "agent_model_url") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN agent_model_url TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "agent_model_api_key") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN agent_model_api_key TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_projects_workflow_id ON projects(workflow_id)`); err != nil {
		return err
	}
	if !columnExists(ctx, db, "workflows", "approval_policy") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE workflows ADD COLUMN approval_policy TEXT NOT NULL DEFAULT 'single_role'`); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE workflows SET approval_policy = 'single_role' WHERE TRIM(COALESCE(approval_policy, '')) = ''`); err != nil {
		return err
	}
	if !columnExists(ctx, db, "workflows", "progression_mode") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE workflows ADD COLUMN progression_mode TEXT NOT NULL DEFAULT 'linear'`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "work_items", "commit_ref") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE work_items ADD COLUMN commit_ref TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE projects SET status = 'open' WHERE status = 'active'`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE projects SET visibility = 'public' WHERE TRIM(COALESCE(visibility,'')) = ''`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE projects SET status = 'closed' WHERE status = 'disabled'`); err != nil {
		return err
	}

	// Migrate ticket_id from INTEGER to TEXT (using the key value as the new ticket_id).
	if err := migrateTicketIDToText(ctx, db); err != nil {
		return err
	}
	// Fix dependent tables whose FK constraints still reference tickets_old_int.
	if err := fixStaleFKsAfterTicketIDMigration(ctx, db); err != nil {
		return err
	}
	if err := ensureCascadeTicketChildren(ctx, db); err != nil {
		return err
	}
	if err := ensureProjectHistoryAllowsNullTicketID(ctx, db); err != nil {
		return err
	}
	if err := ensureProjectAccessTables(ctx, db); err != nil {
		return err
	}
	if !tableExists(ctx, db, "user_notifications") {
		if _, err := db.ExecContext(ctx, `
CREATE TABLE user_notifications (
	notification_id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'unread',
	title TEXT NOT NULL,
	message TEXT NOT NULL DEFAULT '',
	payload_json TEXT NOT NULL DEFAULT '{}',
	read_at TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES users(user_id) ON DELETE CASCADE
)`); err != nil {
			return err
		}
	}

	if !columnExists(ctx, db, "tickets", "sort_order") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "estimate_effort") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN estimate_effort INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "estimate_complete") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN estimate_complete TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "git_repository") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN git_repository TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "dor_map") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN dor_map TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "dod_map") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN dod_map TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "ac_map") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN ac_map TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "git_branch") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN git_branch TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "health_score") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN health_score INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "open") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN open INTEGER NOT NULL DEFAULT 1`); err != nil {
			return err
		}
		// Legacy behavior used archived for close/open. Preserve closed state and reset archived.
		if _, err := db.ExecContext(ctx, `UPDATE tickets SET open = CASE WHEN archived = 1 THEN 0 ELSE 1 END`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `UPDATE tickets SET archived = 0`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "deleted") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "stage") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN stage TEXT NOT NULL DEFAULT 'design'`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "state") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN state TEXT NOT NULL DEFAULT 'idle'`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "workflow_stage_id") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN workflow_stage_id INTEGER REFERENCES workflow_stages(workflow_stage_id)`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "workflow_id") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN workflow_id INTEGER REFERENCES workflows(workflow_id)`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "roles", "dor_map") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE roles ADD COLUMN dor_map TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "roles", "dod_map") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE roles ADD COLUMN dod_map TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "roles", "ac_map") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE roles ADD COLUMN ac_map TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tickets_workflow_stage_id ON tickets(workflow_stage_id)`); err != nil {
		return err
	}
	if !tableExists(ctx, db, "stories") {
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS stories (
				story_id INTEGER PRIMARY KEY AUTOINCREMENT,
				project_id INTEGER NOT NULL,
				title TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'draft',
				created_by TEXT,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY(project_id) REFERENCES projects(project_id),
				FOREIGN KEY(created_by) REFERENCES users(user_id)
			)
		`); err != nil {
			return err
		}
	}
	if !tableExists(ctx, db, "story_ticket_links") {
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS story_ticket_links (
				story_id INTEGER NOT NULL,
				ticket_id TEXT NOT NULL,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY(story_id, ticket_id),
				FOREIGN KEY(story_id) REFERENCES stories(story_id),
				FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id)
			)
		`); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, `
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
	if err := backfillProjectPrefixes(ctx, db); err != nil {
		return err
	}
	if err := backfillTicketKeys(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO ticket_history (id, project_id, ticket_id, event_type, payload, created_by, created_at)
		SELECT h.id, h.project_id, h.ticket_id, h.event_type, h.payload, h.created_by, h.created_at
		FROM history_events h
		WHERE NOT EXISTS (SELECT 1 FROM ticket_history th WHERE th.id = h.id)
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO app_settings (key, value) VALUES ('registration_enabled', '1')`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO app_settings (key, value) VALUES ('registration_auto_approve', '1')`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO app_settings (key, value) VALUES ('default_plan_slug', ?)`, DefaultPlanSlug); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO app_settings (key, value) VALUES ('chat_max_connections', '2')`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO app_settings (key, value) VALUES ('chat_max_duration_minutes', '3')`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO app_settings (key, value) VALUES ('chat_enabled', '1')`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO app_settings (key, value) VALUES ('agent_model_provider', 'openai')`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO app_settings (key, value) VALUES ('agent_model_name', 'gpt-5.3-codex')`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO app_settings (key, value) VALUES ('agent_model_url', '')`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO app_settings (key, value) VALUES ('agent_model_api_key', '')`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO app_settings (key, value) VALUES ('agent_model_providers', ?)`, DefaultAgentModelProvidersJSON); err != nil {
		return err
	}
	if err := EnsureDefaultAgentModelProviders(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO project_members (project_id, user_id, role)
		SELECT project_id, created_by, 'owner'
		FROM projects
		WHERE created_by IS NOT NULL AND created_by != ''
	`); err != nil {
		return err
	}
	// Roles and Workflows are now seeded from embedded static files during bootstrap.
	// The legacy seed functions are retained for backward compatibility with existing databases
	// but are no longer called on new databases.
	if err := backfillTicketWorkflowStages(ctx, db); err != nil {
		return err
	}
	// Add DoR/DoD to workflow stages
	if !columnExists(ctx, db, "workflow_stages", "definition_of_ready") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE workflow_stages ADD COLUMN definition_of_ready TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "workflow_stages", "definition_of_done") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE workflow_stages ADD COLUMN definition_of_done TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	// Add new columns to users for merged agent support
	if !columnExists(ctx, db, "users", "user_type") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN user_type TEXT NOT NULL DEFAULT 'user'`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "users", "uuid") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN uuid TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "users", "plan_id") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN plan_id INTEGER`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "users", "default_project_id") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN default_project_id INTEGER`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "default_draft") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN default_draft INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !tableExists(ctx, db, "plans") {
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS plans (
				plan_id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE,
				name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				max_projects INTEGER NOT NULL DEFAULT 0,
				max_private_projects INTEGER NOT NULL DEFAULT 0,
				max_tickets INTEGER NOT NULL DEFAULT 0,
				max_tickets_per_project INTEGER NOT NULL DEFAULT 0,
				max_team_memberships INTEGER NOT NULL DEFAULT 0,
				max_api_calls_per_day INTEGER NOT NULL DEFAULT 0,
				default_project_alias TEXT NOT NULL DEFAULT '',
				registration_actions TEXT NOT NULL DEFAULT '{}',
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "plans", "max_tickets_per_project") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE plans ADD COLUMN max_tickets_per_project INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "plans", "max_api_calls_per_day") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE plans ADD COLUMN max_api_calls_per_day INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "plans", "default_project_alias") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE plans ADD COLUMN default_project_alias TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !tableExists(ctx, db, "project_aliases") {
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS project_aliases (
				alias_name TEXT NOT NULL,
				user_id TEXT,
				project_id INTEGER NOT NULL,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY(alias_name, user_id),
				FOREIGN KEY(user_id) REFERENCES users(user_id),
				FOREIGN KEY(project_id) REFERENCES projects(project_id)
			)
		`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "users", "description") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN description TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "users", "status") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "users", "last_seen") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN last_seen TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "users", "updated_at") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	// Migrate agents table into users if it still exists
	if tableExists(ctx, db, "agents") {
		// Add UUID column to agents if missing (for very old DBs)
		if !columnExists(ctx, db, "agents", "uuid") {
			if _, err := db.ExecContext(ctx, `ALTER TABLE agents ADD COLUMN uuid TEXT NOT NULL DEFAULT ''`); err != nil {
				return err
			}
		}
		// Backfill UUIDs for agents that don't have one
		if agentRows, err := db.QueryContext(ctx, `SELECT agent_id FROM agents WHERE TRIM(COALESCE(uuid, '')) = ''`); err == nil {
			var agentIDs []int64
			for agentRows.Next() {
				var id int64
				if scanErr := agentRows.Scan(&id); scanErr != nil {
					if closeErr := agentRows.Close(); closeErr != nil {
						log.Printf("store: close agent rows after scan failure: %v", closeErr)
					}
					return scanErr
				}
				agentIDs = append(agentIDs, id)
			}
			if rowsErr := agentRows.Err(); rowsErr != nil {
				if closeErr := agentRows.Close(); closeErr != nil {
					log.Printf("store: close agent rows after iteration failure: %v", closeErr)
				}
				return rowsErr
			}
			if closeErr := agentRows.Close(); closeErr != nil {
				log.Printf("store: close agent rows: %v", closeErr)
			}
			for _, id := range agentIDs {
				u := generateAgentUUID()
				if _, err := db.ExecContext(ctx, `UPDATE agents SET uuid = ? WHERE agent_id = ?`, u, id); err != nil {
					return fmt.Errorf("migrate agent uuid: %w", err)
				}
			}
		}
		// Migrate agent data into users table
		if _, err := db.ExecContext(ctx, `
			INSERT OR IGNORE INTO users (username, password_hash, role, display_name, enabled, user_type, uuid, description, status, last_seen, created_at, updated_at)
			SELECT name, password_hash, 'agent', name, enabled, 'agent', COALESCE(uuid, ''), COALESCE(description, ''), COALESCE(status, 'idle'), COALESCE(last_seen, ''), created_at, updated_at
			FROM agents
		`); err != nil {
			return err
		}
		// Migrate team_agents references from old agent_id to new user_id
		if columnExists(ctx, db, "team_agents", "agent_id") {
			if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
				return err
			}
			// Create a mapping from old agent_id to new user_id
			if _, err := db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS team_agents_new (
					team_id INTEGER NOT NULL,
					user_id TEXT NOT NULL,
					created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY(team_id, user_id),
					FOREIGN KEY(team_id) REFERENCES teams(team_id),
					FOREIGN KEY(user_id) REFERENCES users(user_id)
				)
			`); err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `
				INSERT OR IGNORE INTO team_agents_new (team_id, user_id, created_at)
				SELECT ta.team_id, u.user_id, ta.created_at
				FROM team_agents ta
				JOIN agents a ON a.agent_id = ta.agent_id
				JOIN users u ON u.username = a.name AND u.user_type = 'agent'
			`); err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `DROP TABLE team_agents`); err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `ALTER TABLE team_agents_new RENAME TO team_agents`); err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
				return err
			}
		}
		// Drop the old agents table
		if _, err := db.ExecContext(ctx, `DROP TABLE agents`); err != nil {
			return err
		}
	}
	// Add email to users
	if !columnExists(ctx, db, "users", "email") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "users", "email_confirmed_at") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN email_confirmed_at TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	// Programmes table and projects.programme_id must exist before any code that
	// reads projects runs (ensurePublicResources below queries p.programme_id via
	// GetProjectByAlias). On databases that already have an admin user this path
	// is reached during migration, so adding the column late caused
	// "no such column: p.programme_id" upgrade failures.
	if !tableExists(ctx, db, "programmes") {
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE programmes (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "projects", "programme_id") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN programme_id INTEGER REFERENCES programmes(id)`); err != nil {
			return err
		}
	}

	// Agent config key-value store
	if !tableExists(ctx, db, "agent_config") {
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE agent_config (
				user_id TEXT NOT NULL,
				key TEXT NOT NULL,
				value TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY(user_id, key),
				FOREIGN KEY(user_id) REFERENCES users(user_id)
			)
		`); err != nil {
			return err
		}
	}
	// Migrate agent_config from agent_id to user_id if needed
	if columnExists(ctx, db, "agent_config", "agent_id") && !columnExists(ctx, db, "agent_config", "user_id") {
		if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE agent_config_new (
				user_id TEXT NOT NULL,
				key TEXT NOT NULL,
				value TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY(user_id, key),
				FOREIGN KEY(user_id) REFERENCES users(user_id)
			)
		`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `
			INSERT OR IGNORE INTO agent_config_new (user_id, key, value, created_at, updated_at)
			SELECT agent_id, key, value, created_at, updated_at FROM agent_config
		`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `DROP TABLE agent_config`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `ALTER TABLE agent_config_new RENAME TO agent_config`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "tickets", "ready") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN ready INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	// Add author column to store the username of who created the ticket
	if !columnExists(ctx, db, "tickets", "author") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN author TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
		// Backfill author from created_by user ID
		if _, err := db.ExecContext(ctx, `UPDATE tickets SET author = COALESCE((SELECT u.username FROM users u WHERE u.user_id = tickets.created_by), '') WHERE author = ''`); err != nil {
			return err
		}
	}
	// Account lockout columns for brute-force protection.
	if !columnExists(ctx, db, "users", "failed_login_attempts") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN failed_login_attempts INTEGER DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columnExists(ctx, db, "users", "locked_until") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN locked_until TEXT`); err != nil {
			return err
		}
	}

	// Add missing indexes for frequently-queried columns.
	type conditionalIndex struct {
		table  string
		column string
		stmt   string
	}
	missingIndexes := []conditionalIndex{
		{table: "tickets", column: "project_id", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_project_id ON tickets(project_id)`},
		{table: "tickets", column: "parent_id", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_parent_id ON tickets(parent_id)`},
		{table: "tickets", column: "assignee", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_assignee ON tickets(assignee)`},
		{table: "tickets", column: "stage", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_stage ON tickets(stage)`},
		{table: "tickets", column: "state", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_state ON tickets(state)`},
		{table: "tickets", column: "open", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_open ON tickets(open)`},
		{table: "tickets", column: "draft", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_draft ON tickets(draft)`},
		{table: "tickets", column: "complete", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_complete ON tickets(complete)`},
		{table: "tickets", column: "archived", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_archived ON tickets(archived)`},
		{table: "tickets", column: "deleted", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_deleted ON tickets(deleted)`},
		{table: "tickets", column: "status", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status)`},
		{table: "tickets", column: "type", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_type ON tickets(type)`},
		{table: "tickets", column: "role_id", stmt: `CREATE INDEX IF NOT EXISTS idx_tickets_role_id ON tickets(role_id)`},
		{table: "project_members", column: "user_id", stmt: `CREATE INDEX IF NOT EXISTS idx_project_members_user_id ON project_members(user_id)`},
		{table: "team_members", column: "user_id", stmt: `CREATE INDEX IF NOT EXISTS idx_team_members_user_id ON team_members(user_id)`},
		{table: "team_agents", column: "user_id", stmt: `CREATE INDEX IF NOT EXISTS idx_team_agents_user_id ON team_agents(user_id)`},
		{table: "ticket_labels", column: "ticket_id", stmt: `CREATE INDEX IF NOT EXISTS idx_ticket_labels_ticket_id ON ticket_labels(ticket_id)`},
		{table: "users", column: "username", stmt: `CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`},
		{table: "roles", column: "workflow_id", stmt: `CREATE INDEX IF NOT EXISTS idx_roles_workflow_id ON roles(workflow_id)`},
		{table: "workflow_stages", column: "workflow_id", stmt: `CREATE INDEX IF NOT EXISTS idx_workflow_stages_workflow_id ON workflow_stages(workflow_id)`},
		{table: "workflow_stage_roles", column: "stage_id", stmt: `CREATE INDEX IF NOT EXISTS idx_workflow_stage_roles_stage_id ON workflow_stage_roles(stage_id)`},
		{table: "workflow_stage_roles", column: "role_id", stmt: `CREATE INDEX IF NOT EXISTS idx_workflow_stage_roles_role_id ON workflow_stage_roles(role_id)`},
		{table: "documents", column: "project_id", stmt: `CREATE INDEX IF NOT EXISTS idx_documents_project_id ON documents(project_id)`},
		{table: "document_labels", column: "label_id", stmt: `CREATE INDEX IF NOT EXISTS idx_document_labels_label_id ON document_labels(label_id)`},
		{table: "document_files", column: "document_id", stmt: `CREATE INDEX IF NOT EXISTS idx_document_files_document_id ON document_files(document_id)`},
	}
	for _, idx := range missingIndexes {
		if !columnExists(ctx, db, idx.table, idx.column) {
			continue
		}
		if _, err := db.ExecContext(ctx, idx.stmt); err != nil {
			return err
		}
	}

	// Add releases table if it doesn't exist yet (existing databases).
	if !tableExists(ctx, db, "releases") {
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE releases (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				project_id INTEGER NOT NULL,
				title TEXT NOT NULL DEFAULT '',
				purpose TEXT NOT NULL DEFAULT '',
				target_date TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'in_design',
				designed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				started_at TEXT NOT NULL DEFAULT '',
				completed_at TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY(project_id) REFERENCES projects(project_id)
			)
		`); err != nil {
			return err
		}
	}

	// Add release_id column to tickets if it doesn't exist yet.
	if !columnExists(ctx, db, "tickets", "release_id") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN release_id INTEGER REFERENCES releases(id)`); err != nil {
			return err
		}
	}

	// Add org table if it doesn't exist yet.
	if !tableExists(ctx, db, "org") {
		if _, err := db.ExecContext(ctx, `
			CREATE TABLE org (
				id INTEGER PRIMARY KEY DEFAULT 1 CHECK(id = 1),
				name TEXT NOT NULL DEFAULT '',
				domain TEXT NOT NULL DEFAULT '',
				description TEXT NOT NULL DEFAULT '',
				logo_url TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`); err != nil {
			return err
		}
	}

	// Add recommended_ready to tickets (refiner agent signals readiness).
	if !columnExists(ctx, db, "tickets", "recommended_ready") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN recommended_ready INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}

	// Add pr_url to tickets (developer agent stores PR link after completing).
	if !columnExists(ctx, db, "tickets", "pr_url") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tickets ADD COLUMN pr_url TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}

	// Add agent_role to users (refiner, developer, tester etc.).
	if !columnExists(ctx, db, "users", "agent_role") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN agent_role TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}

	// Data backfills that read whole rows must run only after every additive
	// column above has been applied, otherwise shared SELECT column lists (for
	// example users.agent_role, projects.programme_id) reference columns that do
	// not exist yet and the migration aborts with "no such column".
	if err := ensureDefaultPlans(ctx, db); err != nil {
		return err
	}
	var bootstrapAdminID string
	if err := db.QueryRowContext(ctx, `SELECT user_id FROM users WHERE role = 'admin' ORDER BY created_at LIMIT 1`).Scan(&bootstrapAdminID); err == nil {
		if _, _, ensureErr := ensurePublicResources(ctx, db, bootstrapAdminID); ensureErr != nil {
			return ensureErr
		}
	}

	return nil
}

func backfillTicketWorkflowStages(ctx context.Context, db *sql.DB) error {
	// For tickets that have a stage name but no workflow_stage_id,
	// resolve the stage from the project's workflow.
	_, err := db.ExecContext(ctx, `
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

func backfillProjectPrefixes(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT project_id, title, prefix FROM projects ORDER BY project_id`)
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
				if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets WHERE project_id = ?`, project.id).Scan(&ticketCount); err != nil {
					return err
				}
				if ticketCount == 0 {
					prefix := defaultProjectPrefix
					if prefix != strings.TrimSpace(project.prefix) {
						nextPrefix, prefixErr := nextUniqueProjectPrefix(ctx, db, prefix)
						if prefixErr != nil {
							return prefixErr
						}
						if _, execErr := db.ExecContext(ctx, `UPDATE projects SET prefix = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, nextPrefix, project.id); execErr != nil {
							return execErr
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
		prefix, err := nextUniqueProjectPrefix(ctx, db, desiredPrefix)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `UPDATE projects SET prefix = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, prefix, project.id); err != nil {
			return err
		}
	}
	return nil
}

func backfillTicketKeys(ctx context.Context, db *sql.DB) error {
	// ticket_id is now the key string; no separate key column to backfill.
	return nil
}

// migrateTicketIDToText converts ticket_id from INTEGER to TEXT PRIMARY KEY,
// using the key column value as the new ticket_id, then drops the key column.
// This is a one-time migration for databases created before the refactor.
func migrateTicketIDToText(ctx context.Context, db *sql.DB) error {
	// Detect: if the key column still exists, migration is needed (or was interrupted).
	if !columnExists(ctx, db, "tickets", "key") {
		return nil
	}

	// If ticket_id is already TEXT the data migration completed but the key
	// column was never dropped (interrupted run). Just drop the column and return.
	if columnType(ctx, db, "tickets", "ticket_id") == "TEXT" {
		_, err := db.ExecContext(ctx, `ALTER TABLE tickets DROP COLUMN key`)
		return err
	}

	// Build a mapping of old integer ticket_id → key string.
	// Recreate tickets table with TEXT PRIMARY KEY, without key column.
	if _, err := db.ExecContext(ctx, `ALTER TABLE tickets RENAME TO tickets_old_int`); err != nil {
		return fmt.Errorf("migrateTicketIDToText: rename tickets: %w", err)
	}

	// Get the current column list (minus key column) to build the INSERT.
	// We know the columns from the schema.
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE tickets (
			ticket_id TEXT PRIMARY KEY,
			project_id INTEGER NOT NULL,
			parent_id TEXT,
			clone_of TEXT,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			acceptance_criteria TEXT NOT NULL DEFAULT '',
			dor_map TEXT NOT NULL DEFAULT '{}',
			dod_map TEXT NOT NULL DEFAULT '{}',
			ac_map TEXT NOT NULL DEFAULT '{}',
			git_repository TEXT NOT NULL DEFAULT '',
			git_branch TEXT NOT NULL DEFAULT '',
			workflow_id INTEGER,
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
			ready INTEGER NOT NULL DEFAULT 0,
			open INTEGER NOT NULL DEFAULT 1,
			archived INTEGER NOT NULL DEFAULT 0,
			deleted INTEGER NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(project_id) REFERENCES projects(project_id),
			FOREIGN KEY(parent_id) REFERENCES tickets(ticket_id),
			FOREIGN KEY(clone_of) REFERENCES tickets(ticket_id),
			FOREIGN KEY(created_by) REFERENCES users(user_id),
			FOREIGN KEY(workflow_stage_id) REFERENCES workflow_stages(workflow_stage_id)
		)
	`); err != nil {
		return fmt.Errorf("migrateTicketIDToText: create new tickets: %w", err)
	}

	// Determine which columns exist in the old table (some may be missing in very old DBs).
	oldCols := []string{
		"project_id", "type", "title", "description", "acceptance_criteria", "dor_map", "dod_map", "ac_map",
		"git_repository", "git_branch", "stage", "state", "status",
		"priority", "sort_order", "estimate_effort", "estimate_complete",
		"health_score", "assignee", "open", "archived", "deleted", "created_by",
		"created_at", "updated_at",
	}
	optionalCols := []string{"workflow_id", "workflow_stage_id", "ready"}
	var presentCols []string
	for _, c := range oldCols {
		if columnExists(ctx, db, "tickets_old_int", c) {
			presentCols = append(presentCols, c)
		}
	}
	for _, c := range optionalCols {
		if columnExists(ctx, db, "tickets_old_int", c) {
			presentCols = append(presentCols, c)
		}
	}

	// Copy data using key as the new ticket_id.
	// parent_id and clone_of need to be mapped from old int to key string.
	prefixedCols := make([]string, len(presentCols))
	for i, c := range presentCols {
		prefixedCols[i] = "o." + c
	}
	insertSQL := fmt.Sprintf( // #nosec G201 -- columns are enumerated from the live schema, not user input
		`INSERT INTO tickets (ticket_id, %s, parent_id, clone_of)
		SELECT
			o.key,
			%s,
			(SELECT p.key FROM tickets_old_int p WHERE p.ticket_id = o.parent_id),
			(SELECT c.key FROM tickets_old_int c WHERE c.ticket_id = o.clone_of)
		FROM tickets_old_int o
		WHERE o.key != ''
	`, strings.Join(presentCols, ", "), strings.Join(prefixedCols, ", "))
	if _, err := db.ExecContext(ctx, insertSQL); err != nil {
		return fmt.Errorf("migrateTicketIDToText: copy data: %w", err)
	}

	// Update FK references in dependent tables: map old integer IDs to key strings.
	fkTables := []struct {
		table  string
		column string
	}{
		{"history_events", "ticket_id"},
		{"ticket_history", "ticket_id"},
		{"comments", "item_id"},
		{"dependencies", "ticket_id"},
		{"dependencies", "depends_on"},
		{"story_ticket_links", "ticket_id"},
		{"ticket_labels", "ticket_id"},
		{"time_entries", "ticket_id"},
	}
	for _, fk := range fkTables {
		if !tableExists(ctx, db, fk.table) || !columnExists(ctx, db, fk.table, fk.column) {
			continue
		}
		updateSQL := fmt.Sprintf( // #nosec G201 -- table and column names come from a hardcoded internal list, not user input
			`UPDATE %s SET %s = (
				SELECT key FROM tickets_old_int WHERE ticket_id = CAST(%s.%s AS INTEGER)
			) WHERE EXISTS (
				SELECT 1 FROM tickets_old_int WHERE ticket_id = CAST(%s.%s AS INTEGER)
			)
		`, fk.table, fk.column, fk.table, fk.column, fk.table, fk.column)
		if _, err := db.ExecContext(ctx, updateSQL); err != nil {
			return fmt.Errorf("migrateTicketIDToText: update %s.%s: %w", fk.table, fk.column, err)
		}
	}

	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS tickets_old_int`); err != nil {
		return fmt.Errorf("migrateTicketIDToText: drop old table: %w", err)
	}

	// Recreate dependent tables so FK constraints point at tickets(ticket_id) instead of tickets_old_int.
	if err := fixStaleFKsAfterTicketIDMigration(ctx, db); err != nil {
		return fmt.Errorf("migrateTicketIDToText: fix dependent FKs: %w", err)
	}

	return nil
}

// fixStaleFKsAfterTicketIDMigration recreates tables whose FK constraints still
// reference tickets_old_int after the ticket_id TEXT migration.
func fixStaleFKsAfterTicketIDMigration(ctx context.Context, db *sql.DB) error {
	if !tableHsFKTo(ctx, db, "history_events", "tickets_old_int") {
		return nil // already fixed
	}
	type tableMigration struct {
		name   string
		create string
	}
	migrations := []tableMigration{
		{name: "history_events", create: `CREATE TABLE history_events (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id INTEGER NOT NULL, ticket_id TEXT NOT NULL, event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}', created_by TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(project_id) REFERENCES projects(project_id), FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id), FOREIGN KEY(created_by) REFERENCES users(user_id))`},
		{name: "ticket_history", create: `CREATE TABLE ticket_history (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id INTEGER NOT NULL, ticket_id TEXT, event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}', created_by TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(project_id) REFERENCES projects(project_id), FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id), FOREIGN KEY(created_by) REFERENCES users(user_id))`},
		{name: "comments", create: `CREATE TABLE comments (id INTEGER PRIMARY KEY AUTOINCREMENT, item_id TEXT NOT NULL, user_id TEXT NOT NULL, comment TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(item_id) REFERENCES tickets(ticket_id), FOREIGN KEY(user_id) REFERENCES users(user_id))`},
		{name: "dependencies", create: `CREATE TABLE dependencies (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id INTEGER NOT NULL, ticket_id TEXT NOT NULL, depends_on TEXT NOT NULL, created_by TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(project_id) REFERENCES projects(project_id), FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id), FOREIGN KEY(depends_on) REFERENCES tickets(ticket_id), FOREIGN KEY(created_by) REFERENCES users(user_id))`},
		{name: "story_ticket_links", create: `CREATE TABLE story_ticket_links (story_id INTEGER NOT NULL, ticket_id TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(story_id, ticket_id), FOREIGN KEY(story_id) REFERENCES stories(story_id), FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id))`},
		{name: "ticket_labels", create: `CREATE TABLE ticket_labels (ticket_id TEXT NOT NULL, label_id INTEGER NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(ticket_id, label_id), FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id), FOREIGN KEY(label_id) REFERENCES labels(label_id))`},
		{name: "time_entries", create: `CREATE TABLE time_entries (time_entry_id INTEGER PRIMARY KEY AUTOINCREMENT, ticket_id TEXT NOT NULL, user_id TEXT NOT NULL, minutes INTEGER NOT NULL, note TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id), FOREIGN KEY(user_id) REFERENCES users(user_id))`},
	}
	for _, m := range migrations {
		if !tableExists(ctx, db, m.name) {
			continue
		}
		if !tableHsFKTo(ctx, db, m.name, "tickets_old_int") {
			continue
		}
		tmp := m.name + "_fk_tmp"
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, m.name, tmp)); err != nil {
			return fmt.Errorf("rename %s: %w", m.name, err)
		}
		if _, err := db.ExecContext(ctx, m.create); err != nil {
			return fmt.Errorf("create %s: %w", m.name, err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s SELECT * FROM %s`, m.name, tmp)); err != nil {
			return fmt.Errorf("copy %s: %w", m.name, err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, tmp)); err != nil {
			return fmt.Errorf("drop %s: %w", tmp, err)
		}
	}
	return nil
}

func ensureCascadeTicketChildren(ctx context.Context, db *sql.DB) error {
	type tableMigration struct {
		name     string
		refTable string
		create   string
	}
	migrations := []tableMigration{
		{
			name:     "history_events",
			refTable: "tickets",
			create:   `CREATE TABLE history_events (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id INTEGER NOT NULL, ticket_id TEXT NOT NULL, event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}', created_by TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE, FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE, FOREIGN KEY(created_by) REFERENCES users(user_id))`,
		},
		{
			name:     "ticket_history",
			refTable: "tickets",
			create:   `CREATE TABLE ticket_history (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id INTEGER NOT NULL, ticket_id TEXT, event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}', created_by TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE, FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE, FOREIGN KEY(created_by) REFERENCES users(user_id))`,
		},
		{
			name:     "comments",
			refTable: "tickets",
			create:   `CREATE TABLE comments (id INTEGER PRIMARY KEY AUTOINCREMENT, item_id TEXT NOT NULL, user_id TEXT NOT NULL, comment TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(item_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE, FOREIGN KEY(user_id) REFERENCES users(user_id))`,
		},
		{
			name:     "dependencies",
			refTable: "tickets",
			create:   `CREATE TABLE dependencies (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id INTEGER NOT NULL, ticket_id TEXT NOT NULL, depends_on TEXT NOT NULL, created_by TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE, FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE, FOREIGN KEY(depends_on) REFERENCES tickets(ticket_id) ON DELETE CASCADE, FOREIGN KEY(created_by) REFERENCES users(user_id))`,
		},
		{
			name:     "story_ticket_links",
			refTable: "tickets",
			create:   `CREATE TABLE story_ticket_links (story_id INTEGER NOT NULL, ticket_id TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(story_id, ticket_id), FOREIGN KEY(story_id) REFERENCES stories(story_id) ON DELETE CASCADE, FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE)`,
		},
		{
			name:     "ticket_labels",
			refTable: "tickets",
			create:   `CREATE TABLE ticket_labels (ticket_id TEXT NOT NULL, label_id INTEGER NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(ticket_id, label_id), FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE, FOREIGN KEY(label_id) REFERENCES labels(label_id) ON DELETE CASCADE)`,
		},
		{
			name:     "time_entries",
			refTable: "tickets",
			create:   `CREATE TABLE time_entries (time_entry_id INTEGER PRIMARY KEY AUTOINCREMENT, ticket_id TEXT NOT NULL, user_id TEXT NOT NULL, minutes INTEGER NOT NULL, note TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE, FOREIGN KEY(user_id) REFERENCES users(user_id))`,
		},
	}
	for _, m := range migrations {
		if !tableExists(ctx, db, m.name) || tableHasFKDeleteAction(ctx, db, m.name, m.refTable, "CASCADE") {
			continue
		}
		tmp := m.name + "_cascade_tmp"
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, m.name, tmp)); err != nil {
			return fmt.Errorf("rename %s: %w", m.name, err)
		}
		if _, err := db.ExecContext(ctx, m.create); err != nil {
			return fmt.Errorf("create %s: %w", m.name, err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s SELECT * FROM %s`, m.name, tmp)); err != nil {
			return fmt.Errorf("copy %s: %w", m.name, err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, tmp)); err != nil {
			return fmt.Errorf("drop %s: %w", tmp, err)
		}
	}
	return nil
}

func ensureProjectHistoryAllowsNullTicketID(ctx context.Context, db *sql.DB) error {
	if !tableExists(ctx, db, "ticket_history") || !columnIsNotNull(ctx, db, "ticket_history", "ticket_id") {
		return nil
	}
	tmp := "ticket_history_nullable_tmp"
	if _, err := db.ExecContext(ctx, `ALTER TABLE ticket_history RENAME TO `+tmp); err != nil {
		return fmt.Errorf("rename ticket_history: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE ticket_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER NOT NULL,
		ticket_id TEXT,
		event_type TEXT NOT NULL,
		payload TEXT NOT NULL DEFAULT '{}',
		created_by TEXT,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE,
		FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE,
		FOREIGN KEY(created_by) REFERENCES users(user_id)
	)`); err != nil {
		return fmt.Errorf("create ticket_history: %w", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO ticket_history (id, project_id, ticket_id, event_type, payload, created_by, created_at)
		SELECT id, project_id, NULLIF(ticket_id, ''), event_type, payload, created_by, created_at
		FROM `+tmp); err != nil {
		return fmt.Errorf("copy ticket_history: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE `+tmp); err != nil {
		return fmt.Errorf("drop ticket_history temp: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_ticket_history_project_id ON ticket_history(project_id)`); err != nil {
		return fmt.Errorf("index ticket_history project_id: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_ticket_history_ticket_id ON ticket_history(ticket_id)`); err != nil {
		return fmt.Errorf("index ticket_history ticket_id: %w", err)
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
func fixStaleForeignKeys(ctx context.Context, db *sql.DB) error {
	if !tableHsFKTo(ctx, db, "history_events", "tasks") {
		return nil // already migrated
	}

	// Temporarily disable FK checks so we can drop/recreate without cascading errors.
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer db.ExecContext(ctx, `PRAGMA foreign_keys = ON`) //nolint:errcheck // best-effort restore of FK enforcement

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
				ticket_id TEXT,
				event_type TEXT NOT NULL,
				payload TEXT NOT NULL DEFAULT '{}',
				created_by TEXT,
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
				ticket_id TEXT NOT NULL,
				event_type TEXT NOT NULL,
				payload TEXT NOT NULL DEFAULT '{}',
				created_by TEXT,
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
				item_id TEXT NOT NULL,
				user_id TEXT NOT NULL,
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
				ticket_id TEXT NOT NULL,
				depends_on TEXT NOT NULL,
				created_by TEXT,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY(project_id) REFERENCES projects(project_id),
				FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id),
				FOREIGN KEY(depends_on) REFERENCES tickets(ticket_id),
				FOREIGN KEY(created_by) REFERENCES users(user_id)
			)`,
		},
	}

	for _, m := range migrations {
		if !tableHsFKTo(ctx, db, m.name, "tasks") {
			continue
		}
		tmpName := m.name + "_migrate_tmp"
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, m.name, tmpName)); err != nil {
			return fmt.Errorf("rename %s: %w", m.name, err)
		}
		if _, err := db.ExecContext(ctx, m.create); err != nil {
			return fmt.Errorf("create %s: %w", m.name, err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s SELECT * FROM %s`, m.name, tmpName)); err != nil {
			return fmt.Errorf("copy %s: %w", m.name, err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, tmpName)); err != nil {
			return fmt.Errorf("drop %s: %w", tmpName, err)
		}
	}
	return nil
}

// tableHsFKTo returns true if the table has a foreign key referencing the given table.
func tableHsFKTo(ctx context.Context, db *sql.DB, tableName, refTable string) bool {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA foreign_key_list(%s)`, quoteIdentifier(tableName)))
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

func tableHasFKDeleteAction(ctx context.Context, db *sql.DB, tableName, refTable, action string) bool {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA foreign_key_list(%s)`, quoteIdentifier(tableName)))
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
		ref, refOK := vals[2].(string)
		onDelete, deleteOK := vals[6].(string)
		if refOK && deleteOK && ref == refTable && strings.EqualFold(onDelete, action) {
			return true
		}
	}
	return false
}

func tableExists(ctx context.Context, db *sql.DB, tableName string) bool {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tableName).Scan(&count)
	return err == nil && count > 0
}

func columnExists(ctx context.Context, db *sql.DB, tableName, columnName string) bool {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+quoteIdentifier(tableName)+`)`)
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

func columnIsNotNull(ctx context.Context, db *sql.DB, tableName, columnName string) bool {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+quoteIdentifier(tableName)+`)`)
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
			return notNull != 0
		}
	}
	return false
}

// columnType returns the declared type of a column (e.g. "TEXT", "INTEGER").
// Returns "" if the table or column does not exist.
func columnType(ctx context.Context, db *sql.DB, tableName, columnName string) string {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+quoteIdentifier(tableName)+`)`)
	if err != nil {
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return ""
		}
		if name == columnName {
			return ctype
		}
	}
	return ""
}
