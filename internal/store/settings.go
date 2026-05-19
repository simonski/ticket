package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	DefaultChatMaxConnections      = 2
	DefaultChatMaxDurationMinutes  = 3
	DefaultChatEnabled             = true
	DefaultAgentModelProvider      = "openai"
	DefaultAgentModelName          = "gpt-5.3-codex"
	DefaultAgentModelURL           = ""
	DefaultAgentModelAPIKey        = ""
	DefaultAgentModelProvidersJSON = `[
  {"id":"openai","label":"OpenAI","base_url":"https://api.openai.com/v1","default_model":"gpt-5.3-codex","models":["gpt-5.3-codex","gpt-5","o3"],"auth_type":"api_key","requires_url":false},
  {"id":"anthropic","label":"Anthropic","base_url":"https://api.anthropic.com","default_model":"claude-sonnet-4.5","models":["claude-sonnet-4.5","claude-opus-4.7","claude-haiku-4.5"],"auth_type":"api_key","requires_url":false},
  {"id":"google","label":"Google Gemini","base_url":"https://generativelanguage.googleapis.com","default_model":"gemini-2.5-pro","models":["gemini-2.5-pro","gemini-2.5-flash"],"auth_type":"api_key","requires_url":false},
  {"id":"openrouter","label":"OpenRouter","base_url":"https://openrouter.ai/api/v1","default_model":"openai/gpt-5","models":["openai/gpt-5","anthropic/claude-sonnet-4.5","google/gemini-2.5-pro"],"auth_type":"api_key","requires_url":false},
  {"id":"github-copilot","label":"GitHub Copilot","base_url":"https://api.githubcopilot.com","default_model":"gpt-5.3-codex","models":["gpt-5.3-codex","gpt-5","claude-sonnet-4.6","gemini-2.5-pro"],"auth_type":"api_key","requires_url":false},
  {"id":"github-copilot-enterprise","label":"GitHub Copilot Enterprise","base_url":"https://api.githubcopilot.com","default_model":"gpt-5.3-codex","models":["gpt-5.3-codex","gpt-5","claude-sonnet-4.6","gemini-2.5-pro"],"auth_type":"api_key","requires_url":false}
]`
)

type ChatLimits struct {
	MaxConnections int `json:"chat_max_connections"`
	MaxDurationMin int `json:"chat_max_duration_minutes"`
}

type AgentModelProvider struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	BaseURL      string   `json:"base_url"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models,omitempty"`
	AuthType     string   `json:"auth_type,omitempty"`
	RequiresURL  bool     `json:"requires_url,omitempty"`
	APIKey       string   `json:"api_key,omitempty"`
}

type AgentModelConfig struct {
	Provider  string               `json:"provider"`
	Model     string               `json:"model"`
	URL       string               `json:"url"`
	APIKey    string               `json:"api_key"`
	Providers []AgentModelProvider `json:"providers,omitempty"`
}

type AppSetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func ListAppSettings(ctx context.Context, db *sql.DB) ([]AppSetting, error) {
	rows, err := db.QueryContext(ctx, `SELECT key, value FROM app_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make([]AppSetting, 0)
	for rows.Next() {
		var entry AppSetting
		if err := rows.Scan(&entry.Key, &entry.Value); err != nil {
			return nil, err
		}
		settings = append(settings, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return settings, nil
}

func SetAppSetting(ctx context.Context, db *sql.DB, key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("config key is required")
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

func DeleteAppSetting(ctx context.Context, db *sql.DB, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("config key is required")
	}
	_, err := db.ExecContext(ctx, `DELETE FROM app_settings WHERE key = ?`, key)
	return err
}

func defaultAgentModelProviders() []AgentModelProvider {
	var providers []AgentModelProvider
	if err := json.Unmarshal([]byte(DefaultAgentModelProvidersJSON), &providers); err != nil {
		return nil
	}
	return providers
}

func normalizeAgentModelProvider(provider AgentModelProvider) AgentModelProvider {
	provider.ID = strings.TrimSpace(provider.ID)
	provider.Label = strings.TrimSpace(provider.Label)
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	provider.DefaultModel = strings.TrimSpace(provider.DefaultModel)
	provider.APIKey = strings.TrimSpace(provider.APIKey)
	if provider.AuthType == "" {
		provider.AuthType = "api_key"
	}
	nextModels := make([]string, 0, len(provider.Models))
	seen := map[string]bool{}
	for _, raw := range provider.Models {
		model := strings.TrimSpace(raw)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		nextModels = append(nextModels, model)
	}
	if provider.DefaultModel != "" && !seen[provider.DefaultModel] {
		nextModels = append([]string{provider.DefaultModel}, nextModels...)
	}
	provider.Models = nextModels
	return provider
}

func EnsureDefaultAgentModelProviders(ctx context.Context, db *sql.DB) error {
	defaults := defaultAgentModelProviders()
	if len(defaults) == 0 {
		return nil
	}
	for index := range defaults {
		defaults[index] = normalizeAgentModelProvider(defaults[index])
	}
	existing := make([]AgentModelProvider, 0)
	var raw string
	if err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'agent_model_providers'`).Scan(&raw); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	} else if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &existing)
	}
	merged := make([]AgentModelProvider, 0, len(defaults))
	existingByID := map[string]AgentModelProvider{}
	for _, provider := range existing {
		normalized := normalizeAgentModelProvider(provider)
		if normalized.ID == "" {
			continue
		}
		existingByID[normalized.ID] = normalized
	}
	for _, def := range defaults {
		next := def
		if current, ok := existingByID[def.ID]; ok {
			if current.Label != "" {
				next.Label = current.Label
			}
			if current.BaseURL != "" {
				next.BaseURL = current.BaseURL
			}
			if current.DefaultModel != "" {
				next.DefaultModel = current.DefaultModel
			}
			if len(current.Models) > 0 {
				next.Models = current.Models
			}
			if current.AuthType != "" {
				next.AuthType = current.AuthType
			}
			next.RequiresURL = current.RequiresURL
			if current.APIKey != "" {
				next.APIKey = current.APIKey
			}
		}
		merged = append(merged, normalizeAgentModelProvider(next))
	}
	// #nosec G117 -- provider profiles intentionally persist optional per-profile API keys.
	encoded, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO app_settings (key, value) VALUES ('agent_model_providers', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, string(encoded)); err != nil {
		return err
	}
	return nil
}

func SystemAgentModelConfig(ctx context.Context, db *sql.DB) (AgentModelConfig, error) {
	cfg := AgentModelConfig{
		Provider: DefaultAgentModelProvider,
		Model:    DefaultAgentModelName,
		URL:      DefaultAgentModelURL,
		APIKey:   DefaultAgentModelAPIKey,
	}
	keys := []struct {
		key string
		set func(string)
	}{
		{"agent_model_provider", func(v string) { cfg.Provider = v }},
		{"agent_model_name", func(v string) { cfg.Model = v }},
		{"agent_model_url", func(v string) { cfg.URL = v }},
		{"agent_model_api_key", func(v string) { cfg.APIKey = v }},
	}
	for _, item := range keys {
		var raw string
		if err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, item.key).Scan(&raw); err == nil {
			item.set(raw)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return cfg, err
		}
	}
	var providersRaw string
	if err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'agent_model_providers'`).Scan(&providersRaw); err == nil {
		_ = json.Unmarshal([]byte(providersRaw), &cfg.Providers)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return cfg, err
	}
	for index := range cfg.Providers {
		cfg.Providers[index] = normalizeAgentModelProvider(cfg.Providers[index])
	}
	return cfg, nil
}

func SetSystemAgentModelConfig(ctx context.Context, db *sql.DB, cfg AgentModelConfig) error {
	if cfg.Provider == "" {
		cfg.Provider = DefaultAgentModelProvider
	}
	if cfg.Model == "" {
		cfg.Model = DefaultAgentModelName
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO app_settings (key, value) VALUES ('agent_model_provider', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, cfg.Provider); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO app_settings (key, value) VALUES ('agent_model_name', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, cfg.Model); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO app_settings (key, value) VALUES ('agent_model_url', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, cfg.URL); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO app_settings (key, value) VALUES ('agent_model_api_key', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, cfg.APIKey); err != nil {
		return err
	}
	if len(cfg.Providers) > 0 {
		// #nosec G117 -- provider profiles intentionally persist optional per-profile API keys.
		encoded, err := json.Marshal(cfg.Providers)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO app_settings (key, value) VALUES ('agent_model_providers', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, string(encoded)); err != nil {
			return err
		}
	}
	return nil
}

func RegistrationEnabled(ctx context.Context, db *sql.DB) (bool, error) {
	var raw string
	if err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'registration_enabled'`).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true, nil
		}
		return false, err
	}
	return raw == "1" || raw == "true", nil
}

func SetRegistrationEnabled(ctx context.Context, db *sql.DB, enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value) VALUES ('registration_enabled', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, value)
	return err
}

func RegistrationAutoApprove(ctx context.Context, db *sql.DB) (bool, error) {
	var raw string
	if err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'registration_auto_approve'`).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true, nil
		}
		return false, err
	}
	return raw == "1" || raw == "true", nil
}

func SetRegistrationAutoApprove(ctx context.Context, db *sql.DB, enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value) VALUES ('registration_auto_approve', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, value)
	return err
}

func ChatEnabled(ctx context.Context, db *sql.DB) (bool, error) {
	var raw string
	if err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'chat_enabled'`).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DefaultChatEnabled, nil
		}
		return false, err
	}
	return raw == "1" || raw == "true", nil
}

func SetChatEnabled(ctx context.Context, db *sql.DB, enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value) VALUES ('chat_enabled', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, value)
	return err
}

func ChatLimitsConfig(ctx context.Context, db *sql.DB) (ChatLimits, error) {
	limits := ChatLimits{
		MaxConnections: DefaultChatMaxConnections,
		MaxDurationMin: DefaultChatMaxDurationMinutes,
	}
	var rawConnections string
	if err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'chat_max_connections'`).Scan(&rawConnections); err == nil {
		if parsed := parsePositiveInt(rawConnections); parsed > 0 {
			limits.MaxConnections = parsed
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return limits, err
	}
	var rawDuration string
	if err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'chat_max_duration_minutes'`).Scan(&rawDuration); err == nil {
		if parsed := parsePositiveInt(rawDuration); parsed > 0 {
			limits.MaxDurationMin = parsed
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return limits, err
	}
	return limits, nil
}

func SetChatLimitsConfig(ctx context.Context, db *sql.DB, maxConnections, maxDurationMin int) error {
	if maxConnections <= 0 {
		maxConnections = DefaultChatMaxConnections
	}
	if maxDurationMin <= 0 {
		maxDurationMin = DefaultChatMaxDurationMinutes
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value) VALUES ('chat_max_connections', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, maxConnections); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value) VALUES ('chat_max_duration_minutes', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, maxDurationMin); err != nil {
		return err
	}
	return nil
}

func parsePositiveInt(raw string) int {
	n := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0
		}
		n = (n * 10) + int(r-'0')
	}
	return n
}
