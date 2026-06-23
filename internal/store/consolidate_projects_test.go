package store

import (
	"context"
	"database/sql"
	"testing"
)

// TestConsolidateProjectsMigrationNoDataLoss simulates the pre-consolidation
// projects shape (agent_model_* columns present and populated) and verifies the
// upgrade moves the values into attrs with no loss, drops the columns, and the
// values remain readable via the typed Project fields.
func TestConsolidateProjectsMigrationNoDataLoss(t *testing.T) {
	// Not parallel: manipulates schema_version and raw columns.
	ctx := context.Background()
	db, path := attrsTestDB(t)
	p, err := CreateProjectWithParams(ctx, db, ProjectCreateParams{Title: "P", Prefix: "PMG"})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE projects ADD COLUMN agent_model_provider TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN agent_model_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN agent_model_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN agent_model_api_key TEXT NOT NULL DEFAULT ''`,
	} {
		if _, execErr := raw.Exec(stmt); execErr != nil {
			_ = raw.Close()
			t.Fatalf("setup %q error = %v", stmt, execErr)
		}
	}
	if _, execErr := raw.Exec(
		`UPDATE projects SET agent_model_provider=?, agent_model_name=?, agent_model_url=?, agent_model_api_key=?, attrs='{}' WHERE project_id=?`,
		"anthropic", "claude-opus-4-8", "https://api", "sk-secret", p.ID,
	); execErr != nil {
		_ = raw.Close()
		t.Fatalf("populate error = %v", execErr)
	}
	if _, execErr := raw.Exec(`UPDATE schema_meta SET value='11' WHERE key='schema_version'`); execErr != nil {
		_ = raw.Close()
		t.Fatalf("set version error = %v", execErr)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("raw.Close() error = %v", err)
	}

	if _, err := UpgradeInPlace(ctx, path); err != nil {
		t.Fatalf("UpgradeInPlace() error = %v", err)
	}

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Open() after upgrade error = %v", err)
	}
	defer db2.Close()

	got, err := GetProjectByID(ctx, db2, p.ID)
	if err != nil {
		t.Fatalf("GetProjectByID() error = %v", err)
	}
	if got.AgentModelProvider != "anthropic" || got.AgentModelName != "claude-opus-4-8" ||
		got.AgentModelURL != "https://api" || got.AgentModelAPIKey != "sk-secret" {
		t.Fatalf("agent-model config not preserved: %+v", got)
	}
	for _, col := range []string{"agent_model_provider", "agent_model_name", "agent_model_url", "agent_model_api_key"} {
		if columnExists(ctx, db2, "projects", col) {
			t.Errorf("projects.%s still exists after consolidation", col)
		}
	}
}

// TestConsolidatedProjectAgentModelRoundTrip verifies the agent-model config still
// works through the typed fields and the dedicated setter after consolidation.
func TestConsolidatedProjectAgentModelRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, _ := attrsTestDB(t)

	p, err := CreateProjectWithParams(ctx, db, ProjectCreateParams{
		Title: "P", Prefix: "PRT",
		AgentModelProvider: "anthropic", AgentModelName: "claude",
		Attrs: Attrs{"jira": "X-1"},
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	got, err := GetProjectByID(ctx, db, p.ID)
	if err != nil {
		t.Fatalf("GetProjectByID() error = %v", err)
	}
	if got.AgentModelProvider != "anthropic" || got.AgentModelName != "claude" {
		t.Fatalf("agent-model fields not round-tripped: %+v", got)
	}
	if got.Attrs.GetString("jira") != "X-1" {
		t.Fatalf("extra attr lost: %#v", got.Attrs)
	}
	if _, dup := got.Attrs["agent_model_provider"]; dup {
		t.Fatalf("attrs duplicates a typed field: %#v", got.Attrs)
	}

	updated, err := SetProjectAgentModelConfig(ctx, db, p.ID, AgentModelConfig{Provider: "openai", Model: "gpt", URL: "u", APIKey: "k"})
	if err != nil {
		t.Fatalf("SetProjectAgentModelConfig() error = %v", err)
	}
	if updated.AgentModelProvider != "openai" || updated.AgentModelName != "gpt" || updated.AgentModelURL != "u" || updated.AgentModelAPIKey != "k" {
		t.Fatalf("setter did not write through bag: %+v", updated)
	}
}
