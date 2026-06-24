package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Access roles gate which UI panels a user can see/use (TK-135). They are
// distinct from workflow roles (architect/engineer/QA) and from the admin/member
// flag. A user is a member of zero or more access roles; their effective panel
// set is the union of those roles' granted panels. Admins implicitly have every
// panel. A user with no assigned access roles is grandfathered to the default
// member panel set so upgrades never lock anyone out.

// Panel keys mirror the web UI nav view identifiers.
const (
	PanelTickets       = "tickets"
	PanelProjects      = "projects"
	PanelChat          = "chat"
	PanelInterventions = "interventions"
	PanelWorkflows     = "workflows"
	PanelRoles         = "roles"
	PanelDocuments     = "documents"
	PanelContext       = "context"
	PanelTeams         = "teams"
	PanelProgrammes    = "programmes"
	PanelSettings      = "settings"
	PanelAgents        = "agents"
	PanelUsers         = "users"
	PanelAdminSummary  = "admin-summary"

	builtinMemberRoleName = "Member"
)

// adminPanels always require the admin flag and cannot be granted via an access
// role. The remaining panels are grantable.
var adminPanels = []string{PanelProgrammes, PanelSettings, PanelAgents, PanelUsers, PanelAdminSummary}

// grantablePanels are the panels an access role may grant (everything that is
// not admin-only). This is also the default member panel set, preserving the
// pre-TK-135 behaviour where every signed-in user saw the general+process nav.
var grantablePanels = []string{
	PanelTickets, PanelProjects, PanelChat, PanelInterventions,
	PanelWorkflows, PanelRoles, PanelDocuments, PanelContext, PanelTeams,
}

// AllPanels is the full ordered panel list (grantable first, then admin).
func AllPanels() []string {
	out := make([]string, 0, len(grantablePanels)+len(adminPanels))
	out = append(out, grantablePanels...)
	out = append(out, adminPanels...)
	return out
}

// GrantablePanels returns the panels an access role may grant (a copy).
func GrantablePanels() []string {
	return append([]string(nil), grantablePanels...)
}

func isGrantablePanel(key string) bool {
	return slices.Contains(grantablePanels, key)
}

func isAdminPanel(key string) bool {
	return slices.Contains(adminPanels, key)
}

// AccessRole is a named set of panel grants.
type AccessRole struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Builtin     bool     `json:"builtin"`
	Panels      []string `json:"panels"`
	CreatedAt   string   `json:"created_at,omitempty"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
}

// ErrAccessRoleNotFound is returned when an access role does not exist.
var ErrAccessRoleNotFound = errors.New("access role not found")

// normalizePanels validates, de-duplicates, and orders a panel set, rejecting
// admin-only or unknown panel keys.
func normalizePanels(panels []string) ([]string, error) {
	seen := map[string]bool{}
	for _, p := range panels {
		key := strings.TrimSpace(p)
		if key == "" {
			continue
		}
		if isAdminPanel(key) {
			return nil, fmt.Errorf("panel %q is admin-only and cannot be granted via an access role", key)
		}
		if !isGrantablePanel(key) {
			return nil, fmt.Errorf("unknown panel %q", key)
		}
		seen[key] = true
	}
	out := make([]string, 0, len(seen))
	for _, p := range grantablePanels {
		if seen[p] {
			out = append(out, p)
		}
	}
	return out, nil
}

// EnsureDefaultAccessRoles creates the builtin "Member" role (granting every
// non-admin panel) if it does not already exist. Idempotent.
func EnsureDefaultAccessRoles(ctx context.Context, db *sql.DB) error {
	var id int64
	err := db.QueryRowContext(ctx, `SELECT access_role_id FROM access_roles WHERE name = ?`, builtinMemberRoleName).Scan(&id)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, cerr := createAccessRole(ctx, db, builtinMemberRoleName,
		"Standard access to the general and process panels.", grantablePanels, true)
	return cerr
}

func createAccessRole(ctx context.Context, db *sql.DB, name, description string, panels []string, builtin bool) (AccessRole, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return AccessRole{}, errors.New("access role name is required")
	}
	norm, err := normalizePanels(panels)
	if err != nil {
		return AccessRole{}, err
	}
	res, err := db.ExecContext(ctx, `INSERT INTO access_roles (name, description, builtin) VALUES (?, ?, ?)`,
		name, strings.TrimSpace(description), boolToInt(builtin))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return AccessRole{}, fmt.Errorf("an access role named %q already exists", name)
		}
		return AccessRole{}, err
	}
	id, _ := res.LastInsertId()
	if perr := replaceAccessRolePanels(ctx, db, id, norm); perr != nil {
		return AccessRole{}, perr
	}
	return GetAccessRole(ctx, db, id)
}

// CreateAccessRole creates a new (non-builtin) access role.
func CreateAccessRole(ctx context.Context, db *sql.DB, name, description string, panels []string) (AccessRole, error) {
	return createAccessRole(ctx, db, name, description, panels, false)
}

func replaceAccessRolePanels(ctx context.Context, db *sql.DB, roleID int64, panels []string) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM access_role_panels WHERE access_role_id = ?`, roleID); err != nil {
		return err
	}
	for _, p := range panels {
		if _, err := db.ExecContext(ctx, `INSERT INTO access_role_panels (access_role_id, panel_key) VALUES (?, ?)`, roleID, p); err != nil {
			return err
		}
	}
	return nil
}

// UpdateAccessRole updates an access role's name/description/panels.
func UpdateAccessRole(ctx context.Context, db *sql.DB, id int64, name, description string, panels []string) (AccessRole, error) {
	existing, err := GetAccessRole(ctx, db, id)
	if err != nil {
		return AccessRole{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = existing.Name
	}
	norm, err := normalizePanels(panels)
	if err != nil {
		return AccessRole{}, err
	}
	if _, err := db.ExecContext(ctx, `UPDATE access_roles SET name = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE access_role_id = ?`,
		name, strings.TrimSpace(description), id); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return AccessRole{}, fmt.Errorf("an access role named %q already exists", name)
		}
		return AccessRole{}, err
	}
	if perr := replaceAccessRolePanels(ctx, db, id, norm); perr != nil {
		return AccessRole{}, perr
	}
	return GetAccessRole(ctx, db, id)
}

// DeleteAccessRole removes an access role and its memberships. Builtin roles
// cannot be deleted.
func DeleteAccessRole(ctx context.Context, db *sql.DB, id int64) error {
	role, err := GetAccessRole(ctx, db, id)
	if err != nil {
		return err
	}
	if role.Builtin {
		return errors.New("the builtin access role cannot be deleted")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM access_role_panels WHERE access_role_id = ?`, id); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM user_access_roles WHERE access_role_id = ?`, id); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `DELETE FROM access_roles WHERE access_role_id = ?`, id)
	return err
}

// GetAccessRole loads one access role with its panels.
func GetAccessRole(ctx context.Context, db *sql.DB, id int64) (AccessRole, error) {
	var r AccessRole
	var builtin int
	err := db.QueryRowContext(ctx, `SELECT access_role_id, name, description, builtin, created_at, updated_at FROM access_roles WHERE access_role_id = ?`, id).
		Scan(&r.ID, &r.Name, &r.Description, &builtin, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AccessRole{}, ErrAccessRoleNotFound
	}
	if err != nil {
		return AccessRole{}, err
	}
	r.Builtin = builtin != 0
	panels, perr := accessRolePanels(ctx, db, id)
	if perr != nil {
		return AccessRole{}, perr
	}
	r.Panels = panels
	return r, nil
}

func accessRolePanels(ctx context.Context, db *sql.DB, roleID int64) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT panel_key FROM access_role_panels WHERE access_role_id = ?`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := map[string]bool{}
	for rows.Next() {
		var k string
		if serr := rows.Scan(&k); serr != nil {
			return nil, serr
		}
		set[k] = true
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, rerr
	}
	out := make([]string, 0, len(set))
	for _, p := range grantablePanels {
		if set[p] {
			out = append(out, p)
		}
	}
	return out, nil
}

// ListAccessRoles returns all access roles (with panels), seeding the builtin
// role first so freshly-upgraded databases always expose the preset.
func ListAccessRoles(ctx context.Context, db *sql.DB) ([]AccessRole, error) {
	if err := EnsureDefaultAccessRoles(ctx, db); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT access_role_id FROM access_roles ORDER BY builtin DESC, name COLLATE NOCASE ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if serr := rows.Scan(&id); serr != nil {
			return nil, serr
		}
		ids = append(ids, id)
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, rerr
	}
	out := make([]AccessRole, 0, len(ids))
	for _, id := range ids {
		r, gerr := GetAccessRole(ctx, db, id)
		if gerr != nil {
			return nil, gerr
		}
		out = append(out, r)
	}
	return out, nil
}

// SetUserAccessRoles replaces a user's access-role membership.
func SetUserAccessRoles(ctx context.Context, db *sql.DB, userID string, roleIDs []int64) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("user id is required")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM user_access_roles WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, rid := range roleIDs {
		if _, err := GetAccessRole(ctx, db, rid); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO user_access_roles (user_id, access_role_id) VALUES (?, ?)`, userID, rid); err != nil {
			return err
		}
	}
	return nil
}

// UserAccessRoleIDs returns the access-role ids a user belongs to.
func UserAccessRoleIDs(ctx context.Context, db *sql.DB, userID string) ([]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT access_role_id FROM user_access_roles WHERE user_id = ? ORDER BY access_role_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if serr := rows.Scan(&id); serr != nil {
			return nil, serr
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// EffectivePanels resolves the panel set a user can access. Admins get every
// panel; otherwise the union of their access roles' panels, or the default
// member set when the user has no assigned roles (grandfathered).
func EffectivePanels(ctx context.Context, db *sql.DB, userID string, isAdmin bool) ([]string, error) {
	if isAdmin {
		return AllPanels(), nil
	}
	ids, err := UserAccessRoleIDs(ctx, db, userID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return GrantablePanels(), nil
	}
	set := map[string]bool{}
	for _, id := range ids {
		panels, perr := accessRolePanels(ctx, db, id)
		if perr != nil {
			return nil, perr
		}
		for _, p := range panels {
			set[p] = true
		}
	}
	out := make([]string, 0, len(set))
	for _, p := range grantablePanels {
		if set[p] {
			out = append(out, p)
		}
	}
	return out, nil
}

// UserCanAccessPanel reports whether a user may access a given panel.
func UserCanAccessPanel(ctx context.Context, db *sql.DB, userID string, isAdmin bool, panel string) (bool, error) {
	if isAdmin {
		return true, nil
	}
	if isAdminPanel(panel) {
		return false, nil
	}
	panels, err := EffectivePanels(ctx, db, userID, isAdmin)
	if err != nil {
		return false, err
	}
	return slices.Contains(panels, panel), nil
}
