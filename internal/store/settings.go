package store

import "database/sql"

const (
	DefaultChatMaxConnections     = 2
	DefaultChatMaxDurationMinutes = 3
	DefaultChatEnabled            = true
)

type ChatLimits struct {
	MaxConnections int `json:"chat_max_connections"`
	MaxDurationMin int `json:"chat_max_duration_minutes"`
}

func RegistrationEnabled(db *sql.DB) (bool, error) {
	var raw string
	if err := db.QueryRow(`SELECT value FROM app_settings WHERE key = 'registration_enabled'`).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return true, nil
		}
		return false, err
	}
	return raw == "1" || raw == "true", nil
}

func SetRegistrationEnabled(db *sql.DB, enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	_, err := db.Exec(`
		INSERT INTO app_settings (key, value) VALUES ('registration_enabled', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, value)
	return err
}

func ChatEnabled(db *sql.DB) (bool, error) {
	var raw string
	if err := db.QueryRow(`SELECT value FROM app_settings WHERE key = 'chat_enabled'`).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return DefaultChatEnabled, nil
		}
		return false, err
	}
	return raw == "1" || raw == "true", nil
}

func SetChatEnabled(db *sql.DB, enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	_, err := db.Exec(`
		INSERT INTO app_settings (key, value) VALUES ('chat_enabled', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, value)
	return err
}

func ChatLimitsConfig(db *sql.DB) (ChatLimits, error) {
	limits := ChatLimits{
		MaxConnections: DefaultChatMaxConnections,
		MaxDurationMin: DefaultChatMaxDurationMinutes,
	}
	var rawConnections string
	if err := db.QueryRow(`SELECT value FROM app_settings WHERE key = 'chat_max_connections'`).Scan(&rawConnections); err == nil {
		if parsed := parsePositiveInt(rawConnections); parsed > 0 {
			limits.MaxConnections = parsed
		}
	} else if err != sql.ErrNoRows {
		return limits, err
	}
	var rawDuration string
	if err := db.QueryRow(`SELECT value FROM app_settings WHERE key = 'chat_max_duration_minutes'`).Scan(&rawDuration); err == nil {
		if parsed := parsePositiveInt(rawDuration); parsed > 0 {
			limits.MaxDurationMin = parsed
		}
	} else if err != sql.ErrNoRows {
		return limits, err
	}
	return limits, nil
}

func SetChatLimitsConfig(db *sql.DB, maxConnections, maxDurationMin int) error {
	if maxConnections <= 0 {
		maxConnections = DefaultChatMaxConnections
	}
	if maxDurationMin <= 0 {
		maxDurationMin = DefaultChatMaxDurationMinutes
	}
	if _, err := db.Exec(`
		INSERT INTO app_settings (key, value) VALUES ('chat_max_connections', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, maxConnections); err != nil {
		return err
	}
	if _, err := db.Exec(`
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
