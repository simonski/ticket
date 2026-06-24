package store

import (
	"context"
	"testing"
)

func TestResolveAgentModelConfigLayers(t *testing.T) {
	ctx := context.Background()
	db := openAccessRoleTestDB(t)

	// Built-in system default.
	cfg, err := ResolveAgentModelConfig(ctx, db, "u1", nil)
	if err != nil {
		t.Fatalf("resolve default: %v", err)
	}
	if cfg.Provider != DefaultAgentModelProvider || cfg.Model != DefaultAgentModelName {
		t.Fatalf("default resolve = %+v, want %s/%s", cfg, DefaultAgentModelProvider, DefaultAgentModelName)
	}

	// System override.
	if err := SetSystemAgentModelConfig(ctx, db, AgentModelConfig{Provider: "anthropic", Model: "claude-sys", URL: "https://api.anthropic.com", APIKey: "sys-key"}); err != nil {
		t.Fatalf("set system: %v", err)
	}
	cfg, _ = ResolveAgentModelConfig(ctx, db, "u1", nil)
	if cfg.Provider != "anthropic" || cfg.Model != "claude-sys" || cfg.APIKey != "sys-key" {
		t.Fatalf("system override = %+v", cfg)
	}

	// Per-user override only changes the fields it sets (model), inheriting the rest.
	if err := SetUserAgentModelConfig(ctx, db, "u1", AgentModelConfig{Model: "claude-user"}); err != nil {
		t.Fatalf("set user: %v", err)
	}
	cfg, _ = ResolveAgentModelConfig(ctx, db, "u1", nil)
	if cfg.Model != "claude-user" || cfg.Provider != "anthropic" || cfg.APIKey != "sys-key" {
		t.Fatalf("user override = %+v, want model claude-user inheriting provider/key", cfg)
	}

	// A different user without overrides still sees the system config.
	other, _ := ResolveAgentModelConfig(ctx, db, "u2", nil)
	if other.Model != "claude-sys" {
		t.Fatalf("u2 should inherit system model, got %q", other.Model)
	}
}
