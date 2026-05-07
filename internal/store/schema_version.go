package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	LegacySchemaVersion  = 1
	CurrentSchemaVersion = 2
	schemaMetaTable      = "schema_meta"
	schemaVersionKey     = "schema_version"
)

type SchemaVersionError struct {
	Path          string
	Found         int
	Current       int
	UpgradeNeeded bool
}

func (e *SchemaVersionError) Error() string {
	path := strings.TrimSpace(e.Path)
	if path == "" {
		path = "database"
	}
	if e.UpgradeNeeded {
		return fmt.Sprintf("%s schema version %d is older than this binary's schema version %d; run `tk upgrade-database` to port it to a new database", path, e.Found, e.Current)
	}
	return fmt.Sprintf("%s schema version %d is newer than this binary's schema version %d; upgrade the tk binary before using this database", path, e.Found, e.Current)
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
	if _, err := os.Stat(absSource); err != nil {
		return err
	}
	if _, err := os.Stat(absTarget); err == nil {
		return fmt.Errorf("target database already exists at %s", absTarget)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
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
		tempDir, err := os.MkdirTemp("", "ticket-db-upgrade-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempDir)
		workingPath := filepath.Join(tempDir, "ticket.db")
		if err := copySQLiteDatabase(absSource, workingPath); err != nil {
			return err
		}
		workingDB, err := openSQLite(workingPath)
		if err != nil {
			return err
		}
		if err := createSchema(ctx, workingDB); err != nil {
			_ = workingDB.Close()
			return err
		}
		if err := workingDB.Close(); err != nil {
			return err
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

	if err := os.MkdirAll(filepath.Dir(absTarget), 0o700); err != nil && filepath.Dir(absTarget) != "." {
		return err
	}
	if err := Init(absTarget, "admin", "upgrade-temp-password"); err != nil {
		return err
	}
	targetDB, err := Open(absTarget)
	if err != nil {
		return err
	}
	defer targetDB.Close()
	return ImportSnapshot(ctx, targetDB, snapshot)
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
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
