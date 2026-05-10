package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
  {"id":"openai","label":"OpenAI","base_url":"https://api.openai.com/v1","default_model":"gpt-5.3-codex"},
  {"id":"anthropic","label":"Anthropic","base_url":"https://api.anthropic.com","default_model":"claude-sonnet-4.5"},
  {"id":"google","label":"Google Gemini","base_url":"https://generativelanguage.googleapis.com","default_model":"gemini-2.5-pro"},
  {"id":"openrouter","label":"OpenRouter","base_url":"https://openrouter.ai/api/v1","default_model":"openai/gpt-5"}
]`
)

type ChatLimits struct {
	MaxConnections int `json:"chat_max_connections"`
	MaxDurationMin int `json:"chat_max_duration_minutes"`
}

type AgentModelProvider struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	BaseURL      string `json:"base_url"`
	DefaultModel string `json:"default_model"`
}

type AgentModelConfig struct {
	Provider  string               `json:"provider"`
	Model     string               `json:"model"`
	URL       string               `json:"url"`
	APIKey    string               `json:"api_key"`
	Providers []AgentModelProvider `json:"providers,omitempty"`
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
