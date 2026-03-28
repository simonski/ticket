package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotExportImportPreservesIDs(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source.db")
	if err := Init(sourcePath, "admin", "password"); err != nil {
		t.Fatalf("Init(source) error = %v", err)
	}
	sourceDB, err := Open(sourcePath)
	if err != nil {
		t.Fatalf("Open(source) error = %v", err)
	}
	defer sourceDB.Close()

	member, err := CreateUser(sourceDB, "member1", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	project, err := CreateProjectWithParams(sourceDB, ProjectCreateParams{
		Prefix:             "EXP",
		Title:              "Export Project",
		Description:        "Export/import coverage",
		AcceptanceCriteria: "retain IDs",
		CreatedBy:          "",
		Visibility:         ProjectVisibilityPrivate,
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	if _, err := AddProjectMember(sourceDB, project.ID, member.ID, ProjectRoleEditor); err != nil {
		t.Fatalf("AddProjectMember() error = %v", err)
	}
	epic, err := CreateTicket(sourceDB, TicketCreateParams{
		ProjectID:   project.ID,
		Type:        "epic",
		Title:       "Export Epic",
		Description: "Epic for snapshot",
		CreatedBy:   "",
		State:       StateIdle,
	})
	if err != nil {
		t.Fatalf("CreateTicket(epic) error = %v", err)
	}
	parentID := epic.ID
	task, err := CreateTicket(sourceDB, TicketCreateParams{
		ProjectID:   project.ID,
		ParentID:    &parentID,
		Type:        "task",
		Title:       "Export Task",
		Description: "Task for snapshot",
		Assignee:    "admin",
		CreatedBy:   "",
		State:       StateActive,
	})
	if err != nil {
		t.Fatalf("CreateTicket(task) error = %v", err)
	}
	story, err := CreateStory(sourceDB, project.ID, "Snapshot Story", "Story description", "")
	if err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}
	if err := LinkStoryToTicket(sourceDB, story.ID, epic.ID); err != nil {
		t.Fatalf("LinkStoryToTicket(epic) error = %v", err)
	}
	if err := LinkStoryToTicket(sourceDB, story.ID, task.ID); err != nil {
		t.Fatalf("LinkStoryToTicket(task) error = %v", err)
	}

	snapshot, err := ExportSnapshot(sourceDB)
	if err != nil {
		t.Fatalf("ExportSnapshot() error = %v", err)
	}
	if snapshot.SchemaVersion != SnapshotSchemaVersion {
		t.Fatalf("snapshot schema_version = %q, want %q", snapshot.SchemaVersion, SnapshotSchemaVersion)
	}

	targetPath := filepath.Join(t.TempDir(), "target.db")
	if err := Init(targetPath, "admin", "password"); err != nil {
		t.Fatalf("Init(target) error = %v", err)
	}
	targetDB, err := Open(targetPath)
	if err != nil {
		t.Fatalf("Open(target) error = %v", err)
	}
	defer targetDB.Close()

	if err := ImportSnapshot(targetDB, snapshot); err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}

	importedProject, err := GetProjectByID(targetDB, project.ID)
	if err != nil {
		t.Fatalf("GetProjectByID(imported) error = %v", err)
	}
	if importedProject.Prefix != project.Prefix || importedProject.Title != project.Title {
		t.Fatalf("imported project mismatch = %#v, want %#v", importedProject, project)
	}
	importedEpic, err := GetTicket(targetDB, epic.ID)
	if err != nil {
		t.Fatalf("GetTicket(imported epic) error = %v", err)
	}
	if importedEpic.Type != "epic" || importedEpic.Title != epic.Title {
		t.Fatalf("imported epic mismatch = %#v, want %#v", importedEpic, epic)
	}
	importedTask, err := GetTicket(targetDB, task.ID)
	if err != nil {
		t.Fatalf("GetTicket(imported task) error = %v", err)
	}
	if importedTask.ParentID == nil || *importedTask.ParentID != epic.ID {
		t.Fatalf("imported task parent = %#v, want %s", importedTask.ParentID, epic.ID)
	}
	importedStory, err := GetStory(targetDB, story.ID)
	if err != nil {
		t.Fatalf("GetStory(imported) error = %v", err)
	}
	if importedStory.Title != story.Title {
		t.Fatalf("imported story title = %q, want %q", importedStory.Title, story.Title)
	}
	var links int
	if err := targetDB.QueryRow(`SELECT COUNT(*) FROM story_ticket_links WHERE story_id = ?`, story.ID).Scan(&links); err != nil {
		t.Fatalf("story_ticket_links count query error = %v", err)
	}
	if links != 2 {
		t.Fatalf("story links count = %d, want 2", links)
	}
	if _, err := GetUserByUsername(targetDB, "member1"); err != nil {
		t.Fatalf("GetUserByUsername(member1) error = %v", err)
	}
}

func TestNormalizeExportValue(t *testing.T) {
	// []byte -> string
	if got := normalizeExportValue([]byte("hello")); got != "hello" {
		t.Fatalf("normalizeExportValue([]byte) = %v, want hello", got)
	}
	// time.Time -> RFC3339
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if got, ok := normalizeExportValue(ts).(string); !ok || got == "" {
		t.Fatalf("normalizeExportValue(time.Time) = %v", got)
	}
	// string passes through
	if got := normalizeExportValue("foo"); got != "foo" {
		t.Fatalf("normalizeExportValue(string) = %v, want foo", got)
	}
}

func TestNormalizeImportValue(t *testing.T) {
	// json.Number integer
	n := json.Number("42")
	if got := normalizeImportValue(n); got != int64(42) {
		t.Fatalf("normalizeImportValue(json.Number int) = %v (%T)", got, got)
	}
	// json.Number float
	f := json.Number("3.14")
	if got := normalizeImportValue(f); got != 3.14 {
		t.Fatalf("normalizeImportValue(json.Number float) = %v (%T)", got, got)
	}
	// float64 that is integer
	if got := normalizeImportValue(float64(5.0)); got != int64(5) {
		t.Fatalf("normalizeImportValue(float64 int) = %v (%T)", got, got)
	}
	// float64 that is not integer
	if got := normalizeImportValue(float64(3.14)); got != float64(3.14) {
		t.Fatalf("normalizeImportValue(float64) = %v (%T)", got, got)
	}
	// string passes through
	if got := normalizeImportValue("foo"); got != "foo" {
		t.Fatalf("normalizeImportValue(string) = %v", got)
	}
}
