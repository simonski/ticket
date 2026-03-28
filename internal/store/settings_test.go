package store

import (
	"path/filepath"
	"testing"
)

func TestChatLimitsConfigDefaults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	limits, err := ChatLimitsConfig(db)
	if err != nil {
		t.Fatalf("ChatLimitsConfig() error = %v", err)
	}
	if limits.MaxConnections != DefaultChatMaxConnections {
		t.Fatalf("MaxConnections = %d, want %d", limits.MaxConnections, DefaultChatMaxConnections)
	}
	if limits.MaxDurationMin != DefaultChatMaxDurationMinutes {
		t.Fatalf("MaxDurationMin = %d, want %d", limits.MaxDurationMin, DefaultChatMaxDurationMinutes)
	}
}

func TestSetChatLimitsConfigPersistsValues(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := SetChatLimitsConfig(db, 5, 12); err != nil {
		t.Fatalf("SetChatLimitsConfig() error = %v", err)
	}
	limits, err := ChatLimitsConfig(db)
	if err != nil {
		t.Fatalf("ChatLimitsConfig() error = %v", err)
	}
	if limits.MaxConnections != 5 {
		t.Fatalf("MaxConnections = %d, want 5", limits.MaxConnections)
	}
	if limits.MaxDurationMin != 12 {
		t.Fatalf("MaxDurationMin = %d, want 12", limits.MaxDurationMin)
	}
}

func TestSetChatLimitsConfigFallsBackToDefaults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := SetChatLimitsConfig(db, 0, -2); err != nil {
		t.Fatalf("SetChatLimitsConfig() error = %v", err)
	}
	limits, err := ChatLimitsConfig(db)
	if err != nil {
		t.Fatalf("ChatLimitsConfig() error = %v", err)
	}
	if limits.MaxConnections != DefaultChatMaxConnections {
		t.Fatalf("MaxConnections = %d, want %d", limits.MaxConnections, DefaultChatMaxConnections)
	}
	if limits.MaxDurationMin != DefaultChatMaxDurationMinutes {
		t.Fatalf("MaxDurationMin = %d, want %d", limits.MaxDurationMin, DefaultChatMaxDurationMinutes)
	}
}

func TestChatEnabledDefaultsToTrue(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	enabled, err := ChatEnabled(db)
	if err != nil {
		t.Fatalf("ChatEnabled() error = %v", err)
	}
	if !enabled {
		t.Fatalf("ChatEnabled() = false, want true")
	}
}

func TestRegistrationEnabledDefaultsToTrue(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	enabled, err := RegistrationEnabled(db)
	if err != nil {
		t.Fatalf("RegistrationEnabled() error = %v", err)
	}
	if !enabled {
		t.Fatalf("RegistrationEnabled() = false, want true")
	}
}

func TestSetRegistrationEnabledPersistsValues(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := SetRegistrationEnabled(db, false); err != nil {
		t.Fatalf("SetRegistrationEnabled(false) error = %v", err)
	}
	enabled, err := RegistrationEnabled(db)
	if err != nil {
		t.Fatalf("RegistrationEnabled() error = %v", err)
	}
	if enabled {
		t.Fatalf("RegistrationEnabled() = true, want false")
	}

	if err := SetRegistrationEnabled(db, true); err != nil {
		t.Fatalf("SetRegistrationEnabled(true) error = %v", err)
	}
	enabled, err = RegistrationEnabled(db)
	if err != nil {
		t.Fatalf("RegistrationEnabled() error = %v", err)
	}
	if !enabled {
		t.Fatalf("RegistrationEnabled() = false, want true")
	}
}

func TestSetChatEnabledPersistsValues(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := SetChatEnabled(db, false); err != nil {
		t.Fatalf("SetChatEnabled(false) error = %v", err)
	}
	enabled, err := ChatEnabled(db)
	if err != nil {
		t.Fatalf("ChatEnabled() error = %v", err)
	}
	if enabled {
		t.Fatalf("ChatEnabled() = true, want false")
	}

	if err := SetChatEnabled(db, true); err != nil {
		t.Fatalf("SetChatEnabled(true) error = %v", err)
	}
	enabled, err = ChatEnabled(db)
	if err != nil {
		t.Fatalf("ChatEnabled() error = %v", err)
	}
	if !enabled {
		t.Fatalf("ChatEnabled() = false, want true")
	}
}
