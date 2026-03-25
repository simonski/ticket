package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const (
	TeamRoleMember = "member"
	TeamRoleOwner  = "owner"
)

var (
	ErrTeamNotFound       = errors.New("team not found")
	ErrTeamMemberNotFound = errors.New("team member not found")
)

type Team struct {
	ID           int64  `json:"team_id"`
	Name         string `json:"name"`
	ParentTeamID *int64 `json:"parent_team_id"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type TeamMember struct {
	TeamID   int64  `json:"team_id"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	JobTitle string `json:"job_title"`
}

type TeamAgent struct {
	TeamID    int64  `json:"team_id"`
	AgentID   string `json:"agent_id"`
	AgentUUID string `json:"agent_uuid"`
	Enabled   bool   `json:"enabled"`
	Status    string `json:"status"`
}

type ProjectTeamMember struct {
	ProjectID int64  `json:"project_id"`
	TeamID    int64  `json:"team_id"`
	TeamName  string `json:"team_name"`
	Role      string `json:"role"`
}

func normalizeTeamRole(role string) string {
	return strings.TrimSpace(strings.ToLower(role))
}

func validTeamRole(role string) bool {
	switch normalizeTeamRole(role) {
	case TeamRoleMember, TeamRoleOwner:
		return true
	default:
		return false
	}
}

func CreateTeam(db *sql.DB, name string, parentTeamID *int64) (Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Team{}, errors.New("team name is required")
	}
	if parentTeamID != nil {
		if _, err := GetTeamByID(db, *parentTeamID); err != nil {
			return Team{}, err
		}
	}
	result, err := db.Exec(`
		INSERT INTO teams (name, parent_team_id, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, name, parentTeamID)
	if err != nil {
		return Team{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Team{}, err
	}
	return GetTeamByID(db, id)
}

func GetTeamByID(db *sql.DB, id int64) (Team, error) {
	row := db.QueryRow(`
		SELECT team_id, name, parent_team_id, created_at, updated_at
		FROM teams
		WHERE team_id = ?
	`, id)
	var team Team
	if err := row.Scan(&team.ID, &team.Name, &team.ParentTeamID, &team.CreatedAt, &team.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Team{}, ErrTeamNotFound
		}
		return Team{}, err
	}
	return team, nil
}

func ListTeams(db *sql.DB) ([]Team, error) {
	rows, err := db.Query(`
		SELECT team_id, name, parent_team_id, created_at, updated_at
		FROM teams
		ORDER BY name, team_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	teams := make([]Team, 0)
	for rows.Next() {
		var team Team
		if err := rows.Scan(&team.ID, &team.Name, &team.ParentTeamID, &team.CreatedAt, &team.UpdatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

func UpdateTeam(db *sql.DB, id int64, name string, parentTeamID *int64) (Team, error) {
	current, err := GetTeamByID(db, id)
	if err != nil {
		return Team{}, err
	}
	nextName := strings.TrimSpace(name)
	if nextName == "" {
		nextName = current.Name
	}
	if parentTeamID != nil {
		if *parentTeamID == id {
			return Team{}, errors.New("team cannot be its own parent")
		}
		if _, err := GetTeamByID(db, *parentTeamID); err != nil {
			return Team{}, err
		}
		// Prevent cycles by ensuring the candidate parent does not descend from this team.
		descendantIDs, err := TeamDescendantIDs(db, id)
		if err != nil {
			return Team{}, err
		}
		for _, descendantID := range descendantIDs {
			if descendantID == *parentTeamID {
				return Team{}, errors.New("team hierarchy cycle is not allowed")
			}
		}
	}
	if _, err := db.Exec(`
		UPDATE teams
		SET name = ?, parent_team_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE team_id = ?
	`, nextName, parentTeamID, id); err != nil {
		return Team{}, err
	}
	return GetTeamByID(db, id)
}

func DeleteTeam(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM teams WHERE team_id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTeamNotFound
	}
	return nil
}

func AddTeamMember(db *sql.DB, teamID int64, userID string, role, jobTitle string) (TeamMember, error) {
	role = normalizeTeamRole(role)
	if !validTeamRole(role) {
		return TeamMember{}, fmt.Errorf("invalid team role %q", role)
	}
	if _, err := GetTeamByID(db, teamID); err != nil {
		return TeamMember{}, err
	}
	if _, err := GetUserByID(db, userID); err != nil {
		return TeamMember{}, err
	}
	if _, err := db.Exec(`
		INSERT INTO team_members (team_id, user_id, role, job_title, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(team_id, user_id) DO UPDATE SET role = excluded.role, job_title = excluded.job_title, updated_at = CURRENT_TIMESTAMP
	`, teamID, userID, role, strings.TrimSpace(jobTitle)); err != nil {
		return TeamMember{}, err
	}
	return GetTeamMember(db, teamID, userID)
}

func GetTeamMember(db *sql.DB, teamID int64, userID string) (TeamMember, error) {
	row := db.QueryRow(`
		SELECT tm.team_id, tm.user_id, u.username, tm.role, tm.job_title
		FROM team_members tm
		JOIN users u ON u.user_id = tm.user_id
		WHERE tm.team_id = ? AND tm.user_id = ?
	`, teamID, userID)
	var member TeamMember
	if err := row.Scan(&member.TeamID, &member.UserID, &member.Username, &member.Role, &member.JobTitle); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TeamMember{}, ErrTeamMemberNotFound
		}
		return TeamMember{}, err
	}
	return member, nil
}

func RemoveTeamMember(db *sql.DB, teamID int64, userID string) error {
	result, err := db.Exec(`DELETE FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTeamMemberNotFound
	}
	return nil
}

func ListTeamMembers(db *sql.DB, teamID int64) ([]TeamMember, error) {
	rows, err := db.Query(`
		SELECT tm.team_id, tm.user_id, u.username, tm.role, tm.job_title
		FROM team_members tm
		JOIN users u ON u.user_id = tm.user_id
		WHERE tm.team_id = ?
		ORDER BY u.username
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := make([]TeamMember, 0)
	for rows.Next() {
		var member TeamMember
		if err := rows.Scan(&member.TeamID, &member.UserID, &member.Username, &member.Role, &member.JobTitle); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func TeamRoleForUser(db *sql.DB, teamID int64, userID string) (string, bool, error) {
	row := db.QueryRow(`SELECT role FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID)
	var role string
	if err := row.Scan(&role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return normalizeTeamRole(role), true, nil
}

func AddTeamAgent(db *sql.DB, teamID int64, agentID string) (TeamAgent, error) {
	if _, err := GetTeamByID(db, teamID); err != nil {
		return TeamAgent{}, err
	}
	if _, err := GetAgentByID(db, agentID); err != nil {
		return TeamAgent{}, err
	}
	if _, err := db.Exec(`
		INSERT INTO team_agents (team_id, user_id)
		VALUES (?, ?)
		ON CONFLICT(team_id, user_id) DO NOTHING
	`, teamID, agentID); err != nil {
		return TeamAgent{}, err
	}
	row := db.QueryRow(`
		SELECT ta.team_id, u.user_id, u.username, u.enabled, COALESCE(u.status, '')
		FROM team_agents ta
		JOIN users u ON u.user_id = ta.user_id
		WHERE ta.team_id = ? AND ta.user_id = ?
	`, teamID, agentID)
	var item TeamAgent
	var enabled int
	if err := row.Scan(&item.TeamID, &item.AgentID, &item.AgentUUID, &enabled, &item.Status); err != nil {
		return TeamAgent{}, err
	}
	item.Enabled = enabled == 1
	return item, nil
}

func RemoveTeamAgent(db *sql.DB, teamID int64, agentID string) error {
	result, err := db.Exec(`DELETE FROM team_agents WHERE team_id = ? AND user_id = ?`, teamID, agentID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func ListTeamAgents(db *sql.DB, teamID int64) ([]TeamAgent, error) {
	rows, err := db.Query(`
		SELECT ta.team_id, u.user_id, u.username, u.enabled, COALESCE(u.status, '')
		FROM team_agents ta
		JOIN users u ON u.user_id = ta.user_id
		WHERE ta.team_id = ?
		ORDER BY u.username
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]TeamAgent, 0)
	for rows.Next() {
		var item TeamAgent
		var enabled int
		if err := rows.Scan(&item.TeamID, &item.AgentID, &item.AgentUUID, &enabled, &item.Status); err != nil {
			return nil, err
		}
		item.Enabled = enabled == 1
		items = append(items, item)
	}
	return items, rows.Err()
}

func AddProjectTeamMember(db *sql.DB, projectID, teamID int64, role string) (ProjectTeamMember, error) {
	role = normalizeProjectRole(role)
	if !validProjectRole(role) {
		return ProjectTeamMember{}, fmt.Errorf("invalid role %q", role)
	}
	if _, err := GetProjectByID(db, projectID); err != nil {
		return ProjectTeamMember{}, err
	}
	team, err := GetTeamByID(db, teamID)
	if err != nil {
		return ProjectTeamMember{}, err
	}
	if _, err := db.Exec(`
		INSERT INTO project_teams (project_id, team_id, role)
		VALUES (?, ?, ?)
		ON CONFLICT(project_id, team_id) DO UPDATE SET role = excluded.role
	`, projectID, teamID, role); err != nil {
		return ProjectTeamMember{}, err
	}
	return ProjectTeamMember{
		ProjectID: projectID,
		TeamID:    teamID,
		TeamName:  team.Name,
		Role:      role,
	}, nil
}

func RemoveProjectTeamMember(db *sql.DB, projectID, teamID int64) error {
	result, err := db.Exec(`DELETE FROM project_teams WHERE project_id = ? AND team_id = ?`, projectID, teamID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func ListProjectTeamMembers(db *sql.DB, projectID int64) ([]ProjectTeamMember, error) {
	rows, err := db.Query(`
		SELECT pt.project_id, t.team_id, t.name, pt.role
		FROM project_teams pt
		JOIN teams t ON t.team_id = pt.team_id
		WHERE pt.project_id = ?
		ORDER BY t.name
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ProjectTeamMember, 0)
	for rows.Next() {
		var item ProjectTeamMember
		if err := rows.Scan(&item.ProjectID, &item.TeamID, &item.TeamName, &item.Role); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func TeamIDsForUserWithAncestors(db *sql.DB, userID string) ([]int64, error) {
	rows, err := db.Query(`
		WITH RECURSIVE walk(team_id, parent_team_id) AS (
			SELECT t.team_id, t.parent_team_id
			FROM teams t
			JOIN team_members tm ON tm.team_id = t.team_id
			WHERE tm.user_id = ?
			UNION
			SELECT parent.team_id, parent.parent_team_id
			FROM teams parent
			JOIN walk w ON w.parent_team_id = parent.team_id
		)
		SELECT DISTINCT team_id
		FROM walk
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func TeamDescendantIDs(db *sql.DB, teamID int64) ([]int64, error) {
	rows, err := db.Query(`
		WITH RECURSIVE children(team_id) AS (
			SELECT team_id FROM teams WHERE parent_team_id = ?
			UNION
			SELECT t.team_id FROM teams t JOIN children c ON t.parent_team_id = c.team_id
		)
		SELECT team_id FROM children
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func HighestProjectRoleForTeams(db *sql.DB, projectID int64, teamIDs []int64) (string, bool, error) {
	if len(teamIDs) == 0 {
		return "", false, nil
	}
	placeholders := make([]string, 0, len(teamIDs))
	args := make([]any, 0, len(teamIDs)+1)
	args = append(args, projectID)
	for _, id := range teamIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	query := `
		SELECT role
		FROM project_teams
		WHERE project_id = ? AND team_id IN (` + strings.Join(placeholders, ",") + `)
	`
	rows, err := db.Query(query, args...)
	if err != nil {
		return "", false, err
	}
	defer rows.Close()
	best := ""
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return "", false, err
		}
		role = normalizeProjectRole(role)
		if role == ProjectRoleOwner {
			return ProjectRoleOwner, true, nil
		}
		if role == ProjectRoleEditor {
			best = ProjectRoleEditor
		}
		if role == ProjectRoleViewer && best == "" {
			best = ProjectRoleViewer
		}
	}
	if err := rows.Err(); err != nil {
		return "", false, err
	}
	if best == "" {
		return "", false, nil
	}
	return best, true, nil
}
