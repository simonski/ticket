package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Email (SMTP sender) configuration is persisted in the app_settings KV store
// under the keys below (TK-132). This layer only stores and validates the
// configuration; actually sending mail is a separate concern (TK-138).
const (
	emailKeyEnabled     = "email.enabled"
	emailKeyHost        = "email.smtp_host"
	emailKeyPort        = "email.smtp_port"
	emailKeyUsername    = "email.smtp_username"
	emailKeyPassword    = "email.smtp_password"
	emailKeyFromAddress = "email.from_address"
	emailKeyFromName    = "email.from_name"
	emailKeySecurity    = "email.security"

	EmailSecurityNone     = "none"
	EmailSecuritySTARTTLS = "starttls"
	EmailSecurityTLS      = "tls"
)

// EmailConfig describes the SMTP sender. Password is write-through; callers that
// expose it (the API) are responsible for masking it.
type EmailConfig struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
	Security    string `json:"security"`
}

// HasPassword reports whether a password is configured (used for masked display).
func (c EmailConfig) HasPassword() bool { return strings.TrimSpace(c.Password) != "" }

func getAppSettingValue(ctx context.Context, db *sql.DB, key string) (string, error) {
	var v string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// GetEmailConfig loads the SMTP configuration, applying sensible defaults.
func GetEmailConfig(ctx context.Context, db *sql.DB) (EmailConfig, error) {
	cfg := EmailConfig{Port: 587, Security: EmailSecuritySTARTTLS}
	pairs := []struct {
		key string
		set func(string)
	}{
		{emailKeyEnabled, func(v string) { cfg.Enabled = v == "1" || strings.EqualFold(v, "true") }},
		{emailKeyHost, func(v string) { cfg.Host = v }},
		{emailKeyPort, func(v string) {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
				cfg.Port = n
			}
		}},
		{emailKeyUsername, func(v string) { cfg.Username = v }},
		{emailKeyPassword, func(v string) { cfg.Password = v }},
		{emailKeyFromAddress, func(v string) { cfg.FromAddress = v }},
		{emailKeyFromName, func(v string) { cfg.FromName = v }},
		{emailKeySecurity, func(v string) {
			if s := normalizeEmailSecurity(v); s != "" {
				cfg.Security = s
			}
		}},
	}
	for _, p := range pairs {
		v, err := getAppSettingValue(ctx, db, p.key)
		if err != nil {
			return EmailConfig{}, err
		}
		if v != "" {
			p.set(v)
		}
	}
	return cfg, nil
}

func normalizeEmailSecurity(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case EmailSecurityNone:
		return EmailSecurityNone
	case EmailSecuritySTARTTLS, "tls/starttls":
		return EmailSecuritySTARTTLS
	case EmailSecurityTLS, "ssl":
		return EmailSecurityTLS
	default:
		return ""
	}
}

// SetEmailConfig validates and persists the SMTP configuration. When
// updatePassword is false the stored password is left untouched (so callers can
// save the form without re-entering the secret).
func SetEmailConfig(ctx context.Context, db *sql.DB, cfg EmailConfig, updatePassword bool) error {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("smtp port must be between 1 and 65535")
	}
	sec := normalizeEmailSecurity(cfg.Security)
	if sec == "" {
		return fmt.Errorf("security must be one of none, starttls, tls")
	}
	if cfg.Enabled && strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("smtp host is required when email is enabled")
	}
	writes := map[string]string{
		emailKeyEnabled:     boolToSetting(cfg.Enabled),
		emailKeyHost:        strings.TrimSpace(cfg.Host),
		emailKeyPort:        strconv.Itoa(cfg.Port),
		emailKeyUsername:    cfg.Username,
		emailKeyFromAddress: strings.TrimSpace(cfg.FromAddress),
		emailKeyFromName:    strings.TrimSpace(cfg.FromName),
		emailKeySecurity:    sec,
	}
	if updatePassword {
		writes[emailKeyPassword] = cfg.Password
	}
	for k, v := range writes {
		if err := SetAppSetting(ctx, db, k, v); err != nil {
			return err
		}
	}
	return nil
}

// SetEmailEnabled flips just the enable flag.
func SetEmailEnabled(ctx context.Context, db *sql.DB, enabled bool) error {
	if enabled {
		cfg, err := GetEmailConfig(ctx, db)
		if err != nil {
			return err
		}
		if strings.TrimSpace(cfg.Host) == "" {
			return fmt.Errorf("configure an SMTP host before enabling email")
		}
	}
	return SetAppSetting(ctx, db, emailKeyEnabled, boolToSetting(enabled))
}

func boolToSetting(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
