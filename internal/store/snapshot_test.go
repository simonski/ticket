package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSnapshotExportImportPreservesIDs(t *testing.T) {
	t.Parallel()
	sourcePath := filepath.Join(t.TempDir(), "source.db")
	if err := Init(sourcePath, "admin", "password"); err != nil {
		t.Fatalf("Init(source) error = %v", err)
	}
	sourceDB, err := Open(sourcePath)
	if err != nil {
		t.Fatalf("Open(source) error = %v", err)
	}
	defer sourceDB.Close()

	member, err := CreateUser(context.Background(), sourceDB, "member1", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	project, err := CreateProjectWithParams(context.Background(), sourceDB, ProjectCreateParams{
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
	if _, err := AddProjectMember(context.Background(), sourceDB, project.ID, member.ID, ProjectRoleEditor); err != nil {
		t.Fatalf("AddProjectMember() error = %v", err)
	}
	epic, err := CreateTicket(context.Background(), sourceDB, TicketCreateParams{
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
	task, err := CreateTicket(context.Background(), sourceDB, TicketCreateParams{
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
	story, err := CreateStory(context.Background(), sourceDB, project.ID, "Snapshot Story", "Story description", "")
	if err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}
	if err := LinkStoryToTicket(context.Background(), sourceDB, story.ID, epic.ID); err != nil {
		t.Fatalf("LinkStoryToTicket(epic) error = %v", err)
	}
	if err := LinkStoryToTicket(context.Background(), sourceDB, story.ID, task.ID); err != nil {
		t.Fatalf("LinkStoryToTicket(task) error = %v", err)
	}

	snapshot, err := ExportSnapshot(context.Background(), sourceDB)
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

	if err := ImportSnapshot(context.Background(), targetDB, snapshot); err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}

	importedProject, err := GetProjectByID(context.Background(), targetDB, project.ID)
	if err != nil {
		t.Fatalf("GetProjectByID(imported) error = %v", err)
	}
	if importedProject.Prefix != project.Prefix || importedProject.Title != project.Title {
		t.Fatalf("imported project mismatch = %#v, want %#v", importedProject, project)
	}
	importedEpic, err := GetTicket(context.Background(), targetDB, epic.ID)
	if err != nil {
		t.Fatalf("GetTicket(imported epic) error = %v", err)
	}
	if importedEpic.Type != "epic" || importedEpic.Title != epic.Title {
		t.Fatalf("imported epic mismatch = %#v, want %#v", importedEpic, epic)
	}
	importedTask, err := GetTicket(context.Background(), targetDB, task.ID)
	if err != nil {
		t.Fatalf("GetTicket(imported task) error = %v", err)
	}
	if importedTask.ParentID == nil || *importedTask.ParentID != epic.ID {
		t.Fatalf("imported task parent = %#v, want %s", importedTask.ParentID, epic.ID)
	}
	importedStory, err := GetStory(context.Background(), targetDB, story.ID)
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
	if _, err := GetUserByUsername(context.Background(), targetDB, "member1"); err != nil {
		t.Fatalf("GetUserByUsername(member1) error = %v", err)
	}
}

func TestNormalizeExportValue(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestExportSnapshotSignsWhenEncryptionKeyIsConfigured(t *testing.T) {
	t.Setenv("TICKET_ENCRYPTION_KEY", "test-key-for-encryption-32bytes!")

	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Signed Export", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Signed Ticket",
		CreatedBy: "",
	}); err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	snapshot, err := ExportSnapshot(context.Background(), db)
	if err != nil {
		t.Fatalf("ExportSnapshot() error = %v", err)
	}
	if strings.TrimSpace(snapshot.Signature) == "" {
		t.Fatal("expected signed snapshot when TICKET_ENCRYPTION_KEY is set")
	}
}

func TestImportSnapshotRejectsTamperedSignedSnapshot(t *testing.T) {
	t.Setenv("TICKET_ENCRYPTION_KEY", "test-key-for-encryption-32bytes!")

	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Signed Import", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Original Title",
		CreatedBy: "",
	}); err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	snapshot, err := ExportSnapshot(context.Background(), db)
	if err != nil {
		t.Fatalf("ExportSnapshot() error = %v", err)
	}
	ticketsTable := snapshot.Tables["tickets"]
	titleColumn := -1
	for i, column := range ticketsTable.Columns {
		if column == "title" {
			titleColumn = i
			break
		}
	}
	if titleColumn == -1 || len(ticketsTable.Rows) == 0 {
		t.Fatalf("tickets snapshot missing title column or rows: %#v", ticketsTable.Columns)
	}
	ticketsTable.Rows[0][titleColumn] = "Tampered Title"
	snapshot.Tables["tickets"] = ticketsTable

	target := testDB(t)
	err = ImportSnapshot(context.Background(), target, snapshot)
	if err == nil || !strings.Contains(err.Error(), "snapshot signature verification failed") {
		t.Fatalf("ImportSnapshot(tampered) error = %v, want signature verification failure", err)
	}
}

func TestImportSnapshotIgnoresUnknownLegacyColumns(t *testing.T) {
	t.Parallel()

	sourceDB := testDB(t)
	role, err := CreateRole(context.Background(), sourceDB, nil, "Legacy Role", "", "")
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}

	snapshot, err := ExportSnapshot(context.Background(), sourceDB)
	if err != nil {
		t.Fatalf("ExportSnapshot() error = %v", err)
	}
	rolesTable := snapshot.Tables["roles"]
	rolesTable.Columns = append(rolesTable.Columns, "motivation")
	for i := range rolesTable.Rows {
		rolesTable.Rows[i] = append(rolesTable.Rows[i], "legacy field")
	}
	snapshot.Tables["roles"] = rolesTable

	targetDB := testDB(t)
	if err := ImportSnapshot(context.Background(), targetDB, snapshot); err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}
	imported, err := GetRoleByID(context.Background(), targetDB, role.ID)
	if err != nil {
		t.Fatalf("GetRoleByID() error = %v", err)
	}
	if imported.Title != role.Title {
		t.Fatalf("imported.Title = %q, want %q", imported.Title, role.Title)
	}
}

func TestImportSnapshotPreservesRoleSdlcForeignKeys(t *testing.T) {
	t.Parallel()

	sourceDB := testDB(t)
	sdlc, err := CreateSdlc(context.Background(), sourceDB, "FK SDLC", "foreign key coverage")
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}
	role, err := CreateRole(context.Background(), sourceDB, &sdlc.ID, "FK Role", "", "")
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}

	snapshot, err := ExportSnapshot(context.Background(), sourceDB)
	if err != nil {
		t.Fatalf("ExportSnapshot() error = %v", err)
	}

	targetDB := testDB(t)
	if err := ImportSnapshot(context.Background(), targetDB, snapshot); err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}
	imported, err := GetRoleByID(context.Background(), targetDB, role.ID)
	if err != nil {
		t.Fatalf("GetRoleByID() error = %v", err)
	}
	if imported.SdlcID == nil || *imported.SdlcID != sdlc.ID {
		t.Fatalf("imported.SdlcID = %#v, want %d", imported.SdlcID, sdlc.ID)
	}
}

func TestImportSnapshotPrunesOrphanTicketHistoryRows(t *testing.T) {
	t.Parallel()

	sourceDB := testDB(t)
	project, err := CreateProject(context.Background(), sourceDB, "History Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), sourceDB, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "History ticket",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	snapshot, err := ExportSnapshot(context.Background(), sourceDB)
	if err != nil {
		t.Fatalf("ExportSnapshot() error = %v", err)
	}
	historyTable := snapshot.Tables["ticket_history"]
	historyTable.Rows = append(historyTable.Rows, []any{999999, project.ID, "MISSING-TICKET", "legacy", "{}", "", "2026-01-01T00:00:00Z"})
	snapshot.Tables["ticket_history"] = historyTable

	targetDB := testDB(t)
	if err := ImportSnapshot(context.Background(), targetDB, snapshot); err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}
	imported, err := GetTicket(context.Background(), targetDB, ticket.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if imported.Title != ticket.Title {
		t.Fatalf("imported.Title = %q, want %q", imported.Title, ticket.Title)
	}
	var count int
	if err := targetDB.QueryRow(`SELECT COUNT(*) FROM ticket_history WHERE ticket_id = 'MISSING-TICKET'`).Scan(&count); err != nil {
		t.Fatalf("ticket_history count query error = %v", err)
	}
	if count != 0 {
		t.Fatalf("orphan ticket_history row count = %d, want 0", count)
	}
}
