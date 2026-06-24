package store

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
)

// Agent-model configuration resolves in layers (TK-149): per-user overrides
// per-project overrides the system config, which itself carries the built-in
// default. Empty fields at a layer fall through to the next.

func userAgentModelKey(userID, field string) string {
	return "agent_model_user_" + strings.TrimSpace(userID) + "_" + field
}

// UserAgentModelConfig returns a user's per-user overrides (empty = not set).
func UserAgentModelConfig(ctx context.Context, db *sql.DB, userID string) (AgentModelConfig, error) {
	var cfg AgentModelConfig
	fields := []struct {
		field string
		dst   *string
	}{
		{"provider", &cfg.Provider}, {"name", &cfg.Model}, {"url", &cfg.URL}, {"api_key", &cfg.APIKey},
	}
	for _, f := range fields {
		v, err := getAppSettingValue(ctx, db, userAgentModelKey(userID, f.field))
		if err != nil {
			return cfg, err
		}
		*f.dst = v
	}
	return cfg, nil
}

// SetUserAgentModelConfig stores a user's per-user overrides.
func SetUserAgentModelConfig(ctx context.Context, db *sql.DB, userID string, cfg AgentModelConfig) error {
	pairs := []struct{ field, val string }{
		{"provider", cfg.Provider}, {"name", cfg.Model}, {"url", cfg.URL}, {"api_key", cfg.APIKey},
	}
	for _, p := range pairs {
		if err := SetAppSetting(ctx, db, userAgentModelKey(userID, p.field), strings.TrimSpace(p.val)); err != nil {
			return err
		}
	}
	return nil
}

func overlayAgentModel(base *AgentModelConfig, o AgentModelConfig) {
	if strings.TrimSpace(o.Provider) != "" {
		base.Provider = o.Provider
	}
	if strings.TrimSpace(o.Model) != "" {
		base.Model = o.Model
	}
	if strings.TrimSpace(o.URL) != "" {
		base.URL = o.URL
	}
	if strings.TrimSpace(o.APIKey) != "" {
		base.APIKey = o.APIKey
	}
}

// ResolveAgentModelConfig computes the effective agent-model config for a user
// working in a project: system (with built-in default + provider list) overlaid
// by the project, overlaid by the user. The API key falls back to the matching
// provider entry's stored key when not set explicitly.
func ResolveAgentModelConfig(ctx context.Context, db *sql.DB, userID string, projectID *int64) (AgentModelConfig, error) {
	cfg, err := SystemAgentModelConfig(ctx, db)
	if err != nil {
		return cfg, err
	}
	if projectID != nil {
		if proj, perr := GetProject(ctx, db, strconv.FormatInt(*projectID, 10)); perr == nil {
			overlayAgentModel(&cfg, AgentModelConfig{
				Provider: proj.AgentModelProvider,
				Model:    proj.AgentModelName,
				URL:      proj.AgentModelURL,
				APIKey:   proj.AgentModelAPIKey,
			})
		}
	}
	if strings.TrimSpace(userID) != "" {
		uc, uerr := UserAgentModelConfig(ctx, db, userID)
		if uerr != nil {
			return cfg, uerr
		}
		overlayAgentModel(&cfg, uc)
	}
	// Fall back to the configured provider's stored key/URL when not set inline.
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.URL) == "" {
		for _, p := range cfg.Providers {
			if p.ID == cfg.Provider {
				if strings.TrimSpace(cfg.APIKey) == "" {
					cfg.APIKey = p.APIKey
				}
				if strings.TrimSpace(cfg.URL) == "" {
					cfg.URL = p.BaseURL
				}
				if strings.TrimSpace(cfg.Model) == "" {
					cfg.Model = p.DefaultModel
				}
				break
			}
		}
	}
	return cfg, nil
}
