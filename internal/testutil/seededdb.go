package testutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/simonski/ticket/internal/static"
	"github.com/simonski/ticket/internal/store"
)

var seededDBSnapshots sync.Map

// SeededDBPath clones a cached seeded SQLite database into a fresh test temp dir.
func SeededDBPath(t *testing.T, adminPassword string) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	CloneSeededDB(t, dbPath, adminPassword)
	return dbPath
}

// CloneSeededDB copies a cached seeded SQLite database to dbPath.
func CloneSeededDB(t *testing.T, dbPath, adminPassword string) {
	t.Helper()

	snapshotPath, err := seededSnapshotPath(adminPassword)
	if err != nil {
		t.Fatalf("seededSnapshotPath() error = %v", err)
	}
	if err := copySQLiteDatabase(snapshotPath, dbPath); err != nil {
		t.Fatalf("copySQLiteDatabase(%q, %q) error = %v", snapshotPath, dbPath, err)
	}
}

func seededSnapshotPath(adminPassword string) (string, error) {
	if cached, ok := seededDBSnapshots.Load(adminPassword); ok {
		return cached.(string), nil
	}

	snapshotDir, err := os.MkdirTemp("", "ticket-seeded-db-*")
	if err != nil {
		return "", err
	}
	snapshotPath := filepath.Join(snapshotDir, "ticket.db")
	if err := store.Init(snapshotPath, "admin", adminPassword, static.SeedDatabase); err != nil {
		return "", fmt.Errorf("store.Init snapshot: %w", err)
	}
	actual, loaded := seededDBSnapshots.LoadOrStore(adminPassword, snapshotPath)
	if loaded {
		_ = os.Remove(filepath.Join(snapshotDir, "ticket.db-shm"))
		_ = os.Remove(filepath.Join(snapshotDir, "ticket.db-wal"))
		_ = os.Remove(snapshotPath)
		_ = os.Remove(snapshotDir)
		return actual.(string), nil
	}
	return snapshotPath, nil
}

func copySQLiteDatabase(srcPath, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil {
		return err
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := copyIfExists(srcPath+suffix, dstPath+suffix); err != nil {
			return err
		}
	}
	return nil
}

func copyIfExists(srcPath, dstPath string) error {
	src, err := os.Open(srcPath) // #nosec G304 -- test helper copies known local snapshot files created within the test process
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- test helper writes cloned SQLite fixtures into caller-provided temp paths
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Close()
}
