package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	LegacySchemaVersion  = 1
	CurrentSchemaVersion = 11
	schemaMetaTable      = "schema_meta"
	schemaVersionKey     = "schema_version"
	// DefaultBackupRetention is the number of timestamped pre-upgrade backups
	// kept alongside a database; older ones are pruned after a successful upgrade.
	DefaultBackupRetention = 5
)

type SchemaVersionError struct {
	Path          string
	Found         int
	Current       int
	UpgradeNeeded bool
}

func (e *SchemaVersionError) Error() string {
	path := strings.TrimSpace(e.Path)
	displayPath := path
	if path == "" {
		displayPath = "database"
	}
	if e.UpgradeNeeded {
		command := "tk admin upgrade-database"
		if path != "" {
			command = fmt.Sprintf("tk admin upgrade-database -f %s", shellQuotePath(path))
		}
		return fmt.Sprintf("%s is schema version %d; this tk binary expects schema version %d; run `%s` to upgrade it (the server also upgrades automatically on startup)", displayPath, e.Found, e.Current, command)
	}
	return fmt.Sprintf("%s is schema version %d; this tk binary expects schema version %d; upgrade the tk binary before using this database", displayPath, e.Found, e.Current)
}

func shellQuotePath(path string) string {
	return "'" + strings.ReplaceAll(strings.TrimSpace(path), "'", `'\"'\"'`) + "'"
}

func openSQLite(path string) (*sql.DB, error) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 5000;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func enableWAL(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `PRAGMA journal_mode = WAL;`)
	return err
}

func databaseHasNoUserTables(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table'
		  AND name NOT LIKE 'sqlite_%'
	`).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

func readSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	if !tableExists(ctx, db, schemaMetaTable) {
		return LegacySchemaVersion, nil
	}
	var raw string
	err := db.QueryRowContext(ctx, `SELECT value FROM `+schemaMetaTable+` WHERE key = ?`, schemaVersionKey).Scan(&raw)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return LegacySchemaVersion, nil
	case err != nil:
		return 0, err
	}
	version, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid schema version %q", raw)
	}
	return version, nil
}

func writeSchemaVersion(ctx context.Context, db *sql.DB, version int) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO schema_meta (key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, schemaVersionKey, strconv.Itoa(version))
	return err
}

func DetectSchemaVersion(path string) (int, error) {
	db, err := openSQLite(path)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	empty, err := databaseHasNoUserTables(context.Background(), db)
	if err != nil {
		return 0, err
	}
	if empty {
		return 0, errors.New("database is empty")
	}
	return readSchemaVersion(context.Background(), db)
}

func UpgradeDatabase(ctx context.Context, sourcePath, targetPath string) error {
	sourcePath = strings.TrimSpace(sourcePath)
	targetPath = strings.TrimSpace(targetPath)
	if sourcePath == "" {
		return errors.New("source database path is required")
	}
	if targetPath == "" {
		return errors.New("target database path is required")
	}
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return err
	}
	if absSource == absTarget {
		return errors.New("target database path must be different from the source database path")
	}
	if _, statErr := os.Stat(absSource); statErr != nil {
		return statErr
	}
	if _, statErr := os.Stat(absTarget); statErr == nil {
		return fmt.Errorf("target database already exists at %s", absTarget)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	sourceVersion, err := DetectSchemaVersion(absSource)
	if err != nil {
		return err
	}
	if sourceVersion > CurrentSchemaVersion {
		return &SchemaVersionError{
			Path:          absSource,
			Found:         sourceVersion,
			Current:       CurrentSchemaVersion,
			UpgradeNeeded: false,
		}
	}

	snapshotPath := absSource
	cleanup := func() error { return nil }
	if sourceVersion < CurrentSchemaVersion {
		tempDir, tempErr := os.MkdirTemp("", "ticket-db-upgrade-*")
		if tempErr != nil {
			return tempErr
		}
		defer os.RemoveAll(tempDir)
		workingPath := filepath.Join(tempDir, "ticket.db")
		if copyErr := copySQLiteDatabase(absSource, workingPath); copyErr != nil {
			return copyErr
		}
		workingDB, openErr := openSQLite(workingPath)
		if openErr != nil {
			return openErr
		}
		if schemaErr := createSchema(ctx, workingDB); schemaErr != nil {
			_ = workingDB.Close()
			return schemaErr
		}
		if closeErr := workingDB.Close(); closeErr != nil {
			return closeErr
		}
		snapshotPath = workingPath
	}
	defer func() { _ = cleanup() }()

	sourceDB, err := openSQLite(snapshotPath)
	if err != nil {
		return err
	}
	snapshot, err := ExportSnapshot(ctx, sourceDB)
	if closeErr := sourceDB.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(absTarget), 0o700); mkdirErr != nil && filepath.Dir(absTarget) != "." {
		return mkdirErr
	}
	if initErr := Init(absTarget, "admin", "upgrade-temp-password"); initErr != nil {
		return initErr
	}
	targetDB, err := Open(absTarget)
	if err != nil {
		return err
	}
	defer targetDB.Close()
	return ImportSnapshot(ctx, targetDB, snapshot)
}

// UpgradeResult describes the outcome of an in-place upgrade.
type UpgradeResult struct {
	From       int    // schema version found before the upgrade
	To         int    // schema version after the upgrade (CurrentSchemaVersion)
	BackupPath string // path to the verified pre-upgrade backup that was taken
}

// runSchemaUpgrade performs the actual schema migration for a database whose
// detected version is found (<= CurrentSchemaVersion). It is held in a package
// variable so tests can inject a failing migration to exercise rollback; production
// always uses runSchemaUpgradeDefault.
var runSchemaUpgrade = runSchemaUpgradeDefault

func runSchemaUpgradeDefault(ctx context.Context, path string, found int) error {
	if found == CurrentSchemaVersion {
		db, openErr := openSQLite(path)
		if openErr != nil {
			return openErr
		}
		defer db.Close()
		if schemaErr := createSchema(ctx, db); schemaErr != nil {
			return schemaErr
		}
		return enableWAL(ctx, db)
	}

	// Older schema: rebuild into a fresh database (which brings the schema fully
	// current and re-imports the data) and atomically swap it into place.
	tmpDir, err := os.MkdirTemp(filepath.Dir(path), ".tk-upgrade-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	rebuilt := filepath.Join(tmpDir, "ticket.db")
	if err := UpgradeDatabase(ctx, path, rebuilt); err != nil {
		return err
	}
	return replaceSQLiteDatabase(rebuilt, path)
}

// UpgradeInPlace upgrades the database at path so its schema matches the current
// version, leaving the database at the same path. It returns the schema version
// found before the upgrade. See UpgradeInPlaceWithBackup for the backup details.
func UpgradeInPlace(ctx context.Context, path string) (int, error) {
	res, err := UpgradeInPlaceWithBackup(ctx, path)
	return res.From, err
}

// UpgradeInPlaceWithBackup upgrades the database at path in place, taking a
// WAL-checkpointed, integrity-verified backup BEFORE any mutation and rolling the
// database back from that backup if the migration fails. Every migration path is
// protected, not just server startup.
//
//   - If the database is already at the current version, it re-applies the
//     idempotent additive migrations in place (createSchema creates any missing
//     tables and migrateSchema adds any missing columns). This repairs a database
//     that is missing a column introduced without a schema-version bump.
//   - If the database is at an older version, it rebuilds a fresh database from a
//     snapshot of the upgraded data and atomically swaps it into place.
//   - If the database is newer than this binary, it returns a SchemaVersionError.
//
// On success, backups older than DefaultBackupRetention are pruned.
func UpgradeInPlaceWithBackup(ctx context.Context, path string) (UpgradeResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return UpgradeResult{}, errors.New("database path is required")
	}
	found, err := DetectSchemaVersion(path)
	if err != nil {
		return UpgradeResult{}, err
	}
	res := UpgradeResult{From: found, To: CurrentSchemaVersion}
	if found > CurrentSchemaVersion {
		return res, &SchemaVersionError{Path: path, Found: found, Current: CurrentSchemaVersion, UpgradeNeeded: false}
	}

	// Take a checkpointed, integrity-verified backup before mutating the live DB.
	backupPath, err := takeVerifiedBackup(ctx, path, found)
	if err != nil {
		return res, fmt.Errorf("pre-upgrade backup failed: %w", err)
	}
	res.BackupPath = backupPath

	if migErr := runSchemaUpgrade(ctx, path, found); migErr != nil {
		if restoreErr := restoreFromBackup(backupPath, path); restoreErr != nil {
			return res, fmt.Errorf("migration failed (%v) AND automatic rollback failed (%v); restore manually from %s", migErr, restoreErr, backupPath)
		}
		return res, fmt.Errorf("migration failed and was rolled back from backup %s: %w", backupPath, migErr)
	}

	if pruneErr := pruneOldBackups(path, DefaultBackupRetention); pruneErr != nil {
		// Pruning is best-effort housekeeping; do not fail a successful upgrade.
		return res, nil //nolint:nilerr // intentional: prune failures must not fail the upgrade
	}
	return res, nil
}

// backupPathFor returns a unique timestamped backup path for a database. The
// nanosecond suffix keeps backups taken in quick succession distinct and sortable.
func backupPathFor(path string) string {
	return fmt.Sprintf("%s.bak-%s", path, time.Now().Format("20060102-150405.000000000"))
}

// takeVerifiedBackup checkpoints the WAL of the live database, copies it (and its
// sidecars) to a timestamped backup, and verifies that backup with an integrity
// check and a schema-version read before returning. The live database is not
// modified.
func takeVerifiedBackup(ctx context.Context, path string, expectedVersion int) (string, error) {
	if err := checkpointWAL(ctx, path); err != nil {
		return "", fmt.Errorf("wal checkpoint failed: %w", err)
	}
	backupPath := backupPathFor(path)
	if err := copySQLiteDatabase(path, backupPath); err != nil {
		return "", err
	}
	if err := verifyDatabaseBackup(ctx, backupPath, expectedVersion); err != nil {
		return "", fmt.Errorf("backup verification failed: %w", err)
	}
	return backupPath, nil
}

// checkpointWAL flushes the write-ahead log into the main database file so a file
// copy of the main file is a self-contained, consistent backup. It is harmless on
// a database that is not in WAL mode.
func checkpointWAL(ctx context.Context, path string) error {
	db, err := openSQLite(path)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE);`)
	return err
}

// verifyDatabaseBackup opens a backup database, runs PRAGMA integrity_check, and
// confirms it reports the expected schema version. It returns an error if the
// backup is unreadable, corrupt, or at an unexpected version.
func verifyDatabaseBackup(ctx context.Context, path string, expectedVersion int) error {
	db, err := openSQLite(path)
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if scanErr := db.QueryRowContext(ctx, `PRAGMA integrity_check;`).Scan(&result); scanErr != nil {
		return scanErr
	}
	if !strings.EqualFold(strings.TrimSpace(result), "ok") {
		return fmt.Errorf("integrity_check returned %q", result)
	}
	got, err := readSchemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if got != expectedVersion {
		return fmt.Errorf("backup schema version is %d, expected %d", got, expectedVersion)
	}
	return nil
}

// restoreFromBackup restores the live database (and its sidecars) from a backup,
// used to roll back a failed migration. Stale live sidecars are removed first so
// the restored main file is authoritative.
func restoreFromBackup(backupPath, livePath string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(livePath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return copySQLiteDatabase(backupPath, livePath)
}

// pruneOldBackups removes all but the newest keep timestamped backups (and their
// sidecars) for a database. Backups sort chronologically by their zero-padded
// timestamp suffix.
func pruneOldBackups(path string, keep int) error {
	if keep <= 0 {
		return nil
	}
	matches, err := filepath.Glob(path + ".bak-*")
	if err != nil {
		return err
	}
	backups := make([]string, 0, len(matches))
	for _, m := range matches {
		if strings.HasSuffix(m, "-wal") || strings.HasSuffix(m, "-shm") {
			continue
		}
		backups = append(backups, m)
	}
	if len(backups) <= keep {
		return nil
	}
	sort.Strings(backups)
	for _, b := range backups[:len(backups)-keep] {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if err := os.Remove(b + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

// replaceSQLiteDatabase atomically replaces the database at dst (and its WAL/SHM
// sidecars) with the database at src. src and dst must be on the same filesystem.
func replaceSQLiteDatabase(src, dst string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(dst + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		source := src + suffix
		if _, err := os.Stat(source); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		if err := os.Rename(source, dst+suffix); err != nil {
			return err
		}
	}
	return nil
}

// BackupDatabase copies the SQLite database (and any -wal/-shm sidecar files) at
// sourcePath to targetPath. It is intended for taking a safety snapshot before
// an in-place upgrade.
func BackupDatabase(sourcePath, targetPath string) error {
	sourcePath = strings.TrimSpace(sourcePath)
	targetPath = strings.TrimSpace(targetPath)
	if sourcePath == "" || targetPath == "" {
		return errors.New("source and target database paths are required")
	}
	if _, err := os.Stat(sourcePath); err != nil {
		return err
	}
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("backup target already exists at %s", targetPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return copySQLiteDatabase(sourcePath, targetPath)
}

func copySQLiteDatabase(sourcePath, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil && filepath.Dir(targetPath) != "." {
		return err
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		source := sourcePath + suffix
		target := targetPath + suffix
		if err := copyFileIfExists(source, target); err != nil {
			return err
		}
	}
	return nil
}

func copyFileIfExists(source, target string) error {
	in, err := os.Open(source) // #nosec G304 -- source path is resolved by the application, not arbitrary untrusted input
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- target path is application-controlled during migration
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
