package store

import "database/sql"

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
