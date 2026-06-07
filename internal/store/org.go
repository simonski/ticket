package store

import (
	"context"
	"database/sql"
	"errors"
)

var ErrOrgNotFound = errors.New("organisation not found")

type Org struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	Description string `json:"description"`
	LogoURL     string `json:"logo_url"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func GetOrg(ctx context.Context, db *sql.DB) (Org, error) {
	row := db.QueryRowContext(ctx, `SELECT id, name, domain, description, logo_url, created_at, updated_at FROM org LIMIT 1`)
	var o Org
	if err := row.Scan(&o.ID, &o.Name, &o.Domain, &o.Description, &o.LogoURL, &o.CreatedAt, &o.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Org{}, ErrOrgNotFound
		}
		return Org{}, err
	}
	return o, nil
}

func UpdateOrg(ctx context.Context, db *sql.DB, name, domain, description, logoURL string) (Org, error) {
	_, err := db.ExecContext(ctx, `
		INSERT INTO org (id, name, domain, description, logo_url)
		VALUES (1, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			domain = excluded.domain,
			description = excluded.description,
			logo_url = excluded.logo_url,
			updated_at = CURRENT_TIMESTAMP
	`, name, domain, description, logoURL)
	if err != nil {
		return Org{}, err
	}
	return GetOrg(ctx, db)
}
