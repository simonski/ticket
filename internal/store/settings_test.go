package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestChatLimitsConfigDefaults(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	limits, err := ChatLimitsConfig(context.Background(), db)
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
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := SetChatLimitsConfig(context.Background(), db, 5, 12); err != nil {
		t.Fatalf("SetChatLimitsConfig() error = %v", err)
	}
	limits, err := ChatLimitsConfig(context.Background(), db)
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
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := SetChatLimitsConfig(context.Background(), db, 0, -2); err != nil {
		t.Fatalf("SetChatLimitsConfig() error = %v", err)
	}
	limits, err := ChatLimitsConfig(context.Background(), db)
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
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	enabled, err := ChatEnabled(context.Background(), db)
	if err != nil {
		t.Fatalf("ChatEnabled() error = %v", err)
	}
	if !enabled {
		t.Fatalf("ChatEnabled() = false, want true")
	}
}

func TestRegistrationEnabledDefaultsToTrue(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	enabled, err := RegistrationEnabled(context.Background(), db)
	if err != nil {
		t.Fatalf("RegistrationEnabled() error = %v", err)
	}
	if !enabled {
		t.Fatalf("RegistrationEnabled() = false, want true")
	}
}

func TestSetRegistrationEnabledPersistsValues(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := SetRegistrationEnabled(context.Background(), db, false); err != nil {
		t.Fatalf("SetRegistrationEnabled(false) error = %v", err)
	}
	enabled, err := RegistrationEnabled(context.Background(), db)
	if err != nil {
		t.Fatalf("RegistrationEnabled() error = %v", err)
	}
	if enabled {
		t.Fatalf("RegistrationEnabled() = true, want false")
	}

	if err := SetRegistrationEnabled(context.Background(), db, true); err != nil {
		t.Fatalf("SetRegistrationEnabled(true) error = %v", err)
	}
	enabled, err = RegistrationEnabled(context.Background(), db)
	if err != nil {
		t.Fatalf("RegistrationEnabled() error = %v", err)
	}
	if !enabled {
		t.Fatalf("RegistrationEnabled() = false, want true")
	}
}

func TestSetChatEnabledPersistsValues(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := SetChatEnabled(context.Background(), db, false); err != nil {
		t.Fatalf("SetChatEnabled(false) error = %v", err)
	}
	enabled, err := ChatEnabled(context.Background(), db)
	if err != nil {
		t.Fatalf("ChatEnabled() error = %v", err)
	}
	if enabled {
		t.Fatalf("ChatEnabled() = true, want false")
	}

	if err := SetChatEnabled(context.Background(), db, true); err != nil {
		t.Fatalf("SetChatEnabled(true) error = %v", err)
	}
	enabled, err = ChatEnabled(context.Background(), db)
	if err != nil {
		t.Fatalf("ChatEnabled() error = %v", err)
	}
	if !enabled {
		t.Fatalf("ChatEnabled() = false, want true")
	}
}

func TestSystemAgentModelConfigIncludesProviderCatalogWithCopilot(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	cfg, err := SystemAgentModelConfig(context.Background(), db)
	if err != nil {
		t.Fatalf("SystemAgentModelConfig() error = %v", err)
	}
	if len(cfg.Providers) == 0 {
		t.Fatalf("SystemAgentModelConfig().Providers empty")
	}

	var foundCopilot bool
	var foundCopilotEnterprise bool
	for _, provider := range cfg.Providers {
		switch provider.ID {
		case "github-copilot":
			foundCopilot = true
			if provider.DefaultModel == "" {
				t.Fatalf("GitHub Copilot provider missing default model")
			}
			if len(provider.Models) == 0 {
				t.Fatalf("GitHub Copilot provider missing models catalog")
			}
			if provider.AuthType != "api_key" {
				t.Fatalf("GitHub Copilot auth_type = %q, want %q", provider.AuthType, "api_key")
			}
		case "github-copilot-enterprise":
			foundCopilotEnterprise = true
			if provider.DefaultModel == "" {
				t.Fatalf("GitHub Copilot Enterprise provider missing default model")
			}
			if len(provider.Models) == 0 {
				t.Fatalf("GitHub Copilot Enterprise provider missing models catalog")
			}
			if provider.AuthType != "api_key" {
				t.Fatalf("GitHub Copilot Enterprise auth_type = %q, want %q", provider.AuthType, "api_key")
			}
		}
	}
	if !foundCopilot {
		t.Fatalf("GitHub Copilot provider missing from catalog")
	}
	if !foundCopilotEnterprise {
		t.Fatalf("GitHub Copilot Enterprise provider missing from catalog")
	}
}
