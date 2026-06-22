package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// countTickets opens path read-only-ish and returns the number of rows in tickets.
func countTickets(t *testing.T, path string) int {
	t.Helper()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer raw.Close()
	var n int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM tickets`).Scan(&n); err != nil {
		t.Fatalf("count tickets error = %v", err)
	}
	return n
}

func currentVersionDBWithTicket(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(path, "admin", "secret12"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	_, err = CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: 1, Type: "task", Title: "safety ticket",
	})
	if closeErr := db.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	return path
}

// TestUpgradeInPlaceRollsBackOnFailedMigration injects a migration that mutates the
// live database and then fails; the original data and schema version must be
// recovered from the verified backup, and the error must be surfaced.
func TestUpgradeInPlaceRollsBackOnFailedMigration(t *testing.T) {
	// Not parallel: mutates the runSchemaUpgrade package seam.
	path := currentVersionDBWithTicket(t)
	if got := countTickets(t, path); got != 1 {
		t.Fatalf("precondition: countTickets = %d, want 1", got)
	}
	wantVersion, err := DetectSchemaVersion(path)
	if err != nil {
		t.Fatalf("DetectSchemaVersion() error = %v", err)
	}

	prev := runSchemaUpgrade
	runSchemaUpgrade = func(ctx context.Context, p string, found int) error {
		// Destructive change BEFORE failing, to prove rollback restores data.
		raw, openErr := sql.Open("sqlite", p)
		if openErr != nil {
			return openErr
		}
		if _, execErr := raw.Exec(`DELETE FROM tickets`); execErr != nil {
			_ = raw.Close()
			return execErr
		}
		_ = raw.Close()
		return errors.New("injected migration failure")
	}
	defer func() { runSchemaUpgrade = prev }()

	_, upErr := UpgradeInPlace(context.Background(), path)
	if upErr == nil {
		t.Fatal("UpgradeInPlace() error = nil, want injected failure")
	}

	// Data recovered.
	if got := countTickets(t, path); got != 1 {
		t.Fatalf("after rollback countTickets = %d, want 1 (data not recovered)", got)
	}
	// Schema version unchanged.
	if got, vErr := DetectSchemaVersion(path); vErr != nil {
		t.Fatalf("DetectSchemaVersion() after rollback error = %v", vErr)
	} else if got != wantVersion {
		t.Fatalf("schema version after rollback = %d, want %d", got, wantVersion)
	}
}

// TestUpgradeInPlaceTakesVerifiedBackup confirms a verifiable backup is produced
// before the migration runs.
func TestUpgradeInPlaceTakesVerifiedBackup(t *testing.T) {
	t.Parallel()
	path := currentVersionDBWithTicket(t)

	res, err := UpgradeInPlaceWithBackup(context.Background(), path)
	if err != nil {
		t.Fatalf("UpgradeInPlaceWithBackup() error = %v", err)
	}
	if res.BackupPath == "" {
		t.Fatal("BackupPath is empty; no backup was taken")
	}
	if _, statErr := os.Stat(res.BackupPath); statErr != nil {
		t.Fatalf("backup file missing: %v", statErr)
	}
	if vErr := verifyDatabaseBackup(context.Background(), res.BackupPath, res.From); vErr != nil {
		t.Fatalf("backup did not verify: %v", vErr)
	}
}

// TestVerifyDatabaseBackupRejectsBadBackups ensures verification fails for a
// corrupt file and for a wrong expected version, so a bad backup cannot precede a
// migration.
func TestVerifyDatabaseBackupRejectsBadBackups(t *testing.T) {
	t.Parallel()
	path := currentVersionDBWithTicket(t)
	version, err := DetectSchemaVersion(path)
	if err != nil {
		t.Fatalf("DetectSchemaVersion() error = %v", err)
	}

	// Good backup verifies at the right version.
	if vErr := verifyDatabaseBackup(context.Background(), path, version); vErr != nil {
		t.Fatalf("good backup failed to verify: %v", vErr)
	}
	// Wrong expected version is rejected.
	if vErr := verifyDatabaseBackup(context.Background(), path, version+1); vErr == nil {
		t.Fatal("verifyDatabaseBackup() error = nil for wrong version, want error")
	}
	// A garbage (non-SQLite) file is rejected.
	bad := filepath.Join(t.TempDir(), "garbage.db")
	if writeErr := os.WriteFile(bad, []byte("this is not a sqlite database"), 0o600); writeErr != nil {
		t.Fatalf("write garbage error = %v", writeErr)
	}
	if vErr := verifyDatabaseBackup(context.Background(), bad, version); vErr == nil {
		t.Fatal("verifyDatabaseBackup() error = nil for corrupt file, want error")
	}
}

// TestPruneOldBackupsRetainsNewest verifies the retention policy keeps only the
// newest N backups.
func TestPruneOldBackupsRetainsNewest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base := filepath.Join(dir, "ticket.db")
	if err := os.WriteFile(base, []byte("live"), 0o600); err != nil {
		t.Fatalf("write base error = %v", err)
	}
	// Create 7 timestamped backups with sortable suffixes.
	stamps := []string{
		"20260101-000001.000000000",
		"20260101-000002.000000000",
		"20260101-000003.000000000",
		"20260101-000004.000000000",
		"20260101-000005.000000000",
		"20260101-000006.000000000",
		"20260101-000007.000000000",
	}
	for _, s := range stamps {
		if err := os.WriteFile(base+".bak-"+s, []byte("b"), 0o600); err != nil {
			t.Fatalf("write backup error = %v", err)
		}
	}
	if err := pruneOldBackups(base, 3); err != nil {
		t.Fatalf("pruneOldBackups() error = %v", err)
	}
	matches, err := filepath.Glob(base + ".bak-*")
	if err != nil {
		t.Fatalf("glob error = %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("after prune kept %d backups, want 3: %v", len(matches), matches)
	}
	// The three newest must remain.
	for _, s := range stamps[len(stamps)-3:] {
		if _, statErr := os.Stat(base + ".bak-" + s); statErr != nil {
			t.Fatalf("expected newest backup %s to remain: %v", s, statErr)
		}
	}
}
