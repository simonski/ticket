package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrPlanNotFound = errors.New("plan not found")

const (
	DefaultPlanSlug     = "free"
	EnterprisePlanSlug  = "enterprise"
	publicTeamName      = "public"
	publicProjectPrefix = "PUB"
	publicProjectTitle  = "Public"
	publicProjectDesc   = "A project visible to everyone."
	ticketProjectPrefix = "TK"
	ticketProjectTitle  = "ticket"
	ticketProjectRepo   = "github.com/simonski/ticket.git"
	privateProjectDesc  = "Your private project."
)

type RegistrationTeamAssignment struct {
	TeamID int64  `json:"team_id"`
	Role   string `json:"role"`
}

type RegistrationProjectAssignment struct {
	ProjectID int64  `json:"project_id"`
	Role      string `json:"role"`
}

type RegistrationActions struct {
	AutoAssignPublicTeam     bool                            `json:"auto_assign_public_team"`
	AutoCreatePrivateTeam    bool                            `json:"auto_create_private_team"`
	AutoCreatePrivateProject bool                            `json:"auto_create_private_project"`
	Teams                    []RegistrationTeamAssignment    `json:"teams,omitempty"`
	Projects                 []RegistrationProjectAssignment `json:"projects,omitempty"`
}

type Plan struct {
	ID                   int64               `json:"plan_id"`
	Slug                 string              `json:"slug"`
	Name                 string              `json:"name"`
	Description          string              `json:"description"`
	MaxProjects          int                 `json:"max_projects"`
	MaxPrivateProjects   int                 `json:"max_private_projects"`
	MaxTickets           int                 `json:"max_tickets"`
	MaxTicketsPerProject int                 `json:"max_tickets_per_project"`
	MaxTeamMemberships   int                 `json:"max_team_memberships"`
	MaxAPICallsPerDay    int                 `json:"max_api_calls_per_day"`
	DefaultProjectAlias  string              `json:"default_project_alias"`
	RegistrationActions  RegistrationActions `json:"registration_actions"`
	CreatedAt            string              `json:"created_at"`
	UpdatedAt            string              `json:"updated_at"`
}

type PlanCreateParams struct {
	Slug                 string
	Name                 string
	Description          string
	MaxProjects          int
	MaxPrivateProjects   int
	MaxTickets           int
	MaxTicketsPerProject int
	MaxTeamMemberships   int
	MaxAPICallsPerDay    int
	DefaultProjectAlias  string
	RegistrationActions  RegistrationActions
}

type PlanUpdateParams struct {
	Slug                      string
	Name                      string
	Description               string
	MaxProjects               int
	MaxPrivateProjects        int
	MaxTickets                int
	MaxTicketsPerProject      int
	MaxTeamMemberships        int
	MaxAPICallsPerDay         int
	DefaultProjectAlias       string
	RegistrationActions       RegistrationActions
	ReplaceRegistrationAction bool
}

func normalizePlanSlug(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeRegistrationActions(actions RegistrationActions) RegistrationActions {
	next := actions
	teamSeen := map[int64]bool{}
	teams := make([]RegistrationTeamAssignment, 0, len(actions.Teams))
	for _, item := range actions.Teams {
		if item.TeamID <= 0 || teamSeen[item.TeamID] {
			continue
		}
		role := normalizeTeamRole(item.Role)
		if !validTeamRole(role) {
			role = TeamRoleMember
		}
		teamSeen[item.TeamID] = true
		teams = append(teams, RegistrationTeamAssignment{TeamID: item.TeamID, Role: role})
	}
	projectSeen := map[int64]bool{}
	projects := make([]RegistrationProjectAssignment, 0, len(actions.Projects))
	for _, item := range actions.Projects {
		if item.ProjectID <= 0 || projectSeen[item.ProjectID] {
			continue
		}
		role := normalizeProjectRole(item.Role)
		if !validProjectRole(role) {
			role = ProjectRoleObserver
		}
		projectSeen[item.ProjectID] = true
		projects = append(projects, RegistrationProjectAssignment{ProjectID: item.ProjectID, Role: role})
	}
	next.Teams = teams
	next.Projects = projects
	return next
}

func registrationActionsJSON(actions RegistrationActions) (string, error) {
	normalized := normalizeRegistrationActions(actions)
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parseRegistrationActions(raw string) (RegistrationActions, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return RegistrationActions{}, nil
	}
	var actions RegistrationActions
	if err := json.Unmarshal([]byte(raw), &actions); err != nil {
		return RegistrationActions{}, err
	}
	return normalizeRegistrationActions(actions), nil
}

func scanPlan(scan func(dest ...any) error) (Plan, error) {
	var plan Plan
	var actionsRaw string
	if err := scan(
		&plan.ID,
		&plan.Slug,
		&plan.Name,
		&plan.Description,
		&plan.MaxProjects,
		&plan.MaxPrivateProjects,
		&plan.MaxTickets,
		&plan.MaxTicketsPerProject,
		&plan.MaxTeamMemberships,
		&plan.MaxAPICallsPerDay,
		&plan.DefaultProjectAlias,
		&actionsRaw,
		&plan.CreatedAt,
		&plan.UpdatedAt,
	); err != nil {
		return Plan{}, err
	}
	actions, err := parseRegistrationActions(actionsRaw)
	if err != nil {
		return Plan{}, err
	}
	plan.RegistrationActions = actions
	return plan, nil
}

func CreatePlan(ctx context.Context, db *sql.DB, params PlanCreateParams) (Plan, error) {
	slug := normalizePlanSlug(params.Slug)
	if slug == "" {
		return Plan{}, errors.New("plan slug is required")
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return Plan{}, errors.New("plan name is required")
	}
	actionsJSON, err := registrationActionsJSON(params.RegistrationActions)
	if err != nil {
		return Plan{}, err
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO plans (
			slug, name, description, max_projects, max_private_projects, max_tickets, max_tickets_per_project, max_team_memberships, max_api_calls_per_day, default_project_alias, registration_actions, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, slug, name, strings.TrimSpace(params.Description), params.MaxProjects, params.MaxPrivateProjects, params.MaxTickets, params.MaxTicketsPerProject, params.MaxTeamMemberships, params.MaxAPICallsPerDay, strings.TrimSpace(params.DefaultProjectAlias), actionsJSON)
	if err != nil {
		return Plan{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Plan{}, err
	}
	return GetPlanByID(ctx, db, id)
}

func GetPlanByID(ctx context.Context, db *sql.DB, id int64) (Plan, error) {
	row := db.QueryRowContext(ctx, `
		SELECT plan_id, slug, name, description, max_projects, max_private_projects, max_tickets, max_tickets_per_project, max_team_memberships, max_api_calls_per_day, default_project_alias, registration_actions, created_at, updated_at
		FROM plans
		WHERE plan_id = ?
	`, id)
	plan, err := scanPlan(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Plan{}, ErrPlanNotFound
		}
		return Plan{}, err
	}
	return plan, nil
}

func GetPlanBySlug(ctx context.Context, db *sql.DB, slug string) (Plan, error) {
	row := db.QueryRowContext(ctx, `
		SELECT plan_id, slug, name, description, max_projects, max_private_projects, max_tickets, max_tickets_per_project, max_team_memberships, max_api_calls_per_day, default_project_alias, registration_actions, created_at, updated_at
		FROM plans
		WHERE slug = ?
	`, normalizePlanSlug(slug))
	plan, err := scanPlan(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Plan{}, ErrPlanNotFound
		}
		return Plan{}, err
	}
	return plan, nil
}

func ListPlans(ctx context.Context, db *sql.DB) ([]Plan, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT plan_id, slug, name, description, max_projects, max_private_projects, max_tickets, max_tickets_per_project, max_team_memberships, max_api_calls_per_day, default_project_alias, registration_actions, created_at, updated_at
		FROM plans
		ORDER BY plan_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans := make([]Plan, 0)
	for rows.Next() {
		plan, err := scanPlan(rows.Scan)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, rows.Err()
}

func UpdatePlan(ctx context.Context, db *sql.DB, id int64, params PlanUpdateParams) (Plan, error) {
	current, err := GetPlanByID(ctx, db, id)
	if err != nil {
		return Plan{}, err
	}
	slug := normalizePlanSlug(params.Slug)
	if slug == "" {
		slug = current.Slug
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		name = current.Name
	}
	description := params.Description
	if strings.TrimSpace(description) == "" {
		description = current.Description
	}
	actions := current.RegistrationActions
	if params.ReplaceRegistrationAction {
		actions = params.RegistrationActions
	}
	actionsJSON, err := registrationActionsJSON(actions)
	if err != nil {
		return Plan{}, err
	}
	maxProjects := params.MaxProjects
	if maxProjects == 0 {
		maxProjects = current.MaxProjects
	}
	maxPrivateProjects := params.MaxPrivateProjects
	if maxPrivateProjects == 0 {
		maxPrivateProjects = current.MaxPrivateProjects
	}
	maxTickets := params.MaxTickets
	if maxTickets == 0 {
		maxTickets = current.MaxTickets
	}
	maxTicketsPerProject := params.MaxTicketsPerProject
	if maxTicketsPerProject == 0 {
		maxTicketsPerProject = current.MaxTicketsPerProject
	}
	maxTeamMemberships := params.MaxTeamMemberships
	if maxTeamMemberships == 0 {
		maxTeamMemberships = current.MaxTeamMemberships
	}
	maxAPICallsPerDay := params.MaxAPICallsPerDay
	if maxAPICallsPerDay == 0 {
		maxAPICallsPerDay = current.MaxAPICallsPerDay
	}
	defaultProjectAlias := strings.TrimSpace(params.DefaultProjectAlias)
	if defaultProjectAlias == "" {
		defaultProjectAlias = current.DefaultProjectAlias
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE plans
		SET slug = ?, name = ?, description = ?, max_projects = ?, max_private_projects = ?, max_tickets = ?, max_tickets_per_project = ?, max_team_memberships = ?, max_api_calls_per_day = ?, default_project_alias = ?, registration_actions = ?, updated_at = CURRENT_TIMESTAMP
		WHERE plan_id = ?
	`, slug, name, strings.TrimSpace(description), maxProjects, maxPrivateProjects, maxTickets, maxTicketsPerProject, maxTeamMemberships, maxAPICallsPerDay, defaultProjectAlias, actionsJSON, id); err != nil {
		return Plan{}, err
	}
	return GetPlanByID(ctx, db, id)
}

func DeletePlan(ctx context.Context, db *sql.DB, id int64) error {
	current, err := GetPlanByID(ctx, db, id)
	if err != nil {
		return err
	}
	defaultPlan, err := DefaultPlan(ctx, db)
	if err == nil && defaultPlan.ID == current.ID {
		return errors.New("default plan cannot be deleted")
	}
	var assignedUsers int
	if queryErr := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM users WHERE plan_id = ?`, id).Scan(&assignedUsers); queryErr != nil {
		return queryErr
	}
	if assignedUsers > 0 {
		return fmt.Errorf("plan is assigned to %d user(s)", assignedUsers)
	}
	result, err := db.ExecContext(ctx, `DELETE FROM plans WHERE plan_id = ?`, id)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrPlanNotFound
	}
	return nil
}

func AssignUserPlan(ctx context.Context, db *sql.DB, userID string, planID int64) error {
	if _, err := GetPlanByID(ctx, db, planID); err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `UPDATE users SET plan_id = ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?`, planID, userID)
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

func DefaultPlan(ctx context.Context, db *sql.DB) (Plan, error) {
	var slug string
	if err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'default_plan_slug'`).Scan(&slug); err == nil {
		plan, getErr := GetPlanBySlug(ctx, db, slug)
		if getErr == nil {
			return plan, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Plan{}, err
	}
	return GetPlanBySlug(ctx, db, DefaultPlanSlug)
}

func SetDefaultPlanSlug(ctx context.Context, db *sql.DB, slug string) error {
	plan, err := GetPlanBySlug(ctx, db, slug)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value) VALUES ('default_plan_slug', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, plan.Slug)
	return err
}

func ensureDefaultPlans(ctx context.Context, db *sql.DB) error {
	definitions := []PlanCreateParams{
		{
			Slug:                 DefaultPlanSlug,
			Name:                 "Free",
			Description:          "Default plan with public collaboration and one private workspace.",
			MaxProjects:          2,
			MaxPrivateProjects:   1,
			MaxTickets:           100,
			MaxTicketsPerProject: 100,
			MaxTeamMemberships:   3,
			MaxAPICallsPerDay:    100,
			DefaultProjectAlias:  "public",
			RegistrationActions: RegistrationActions{
				AutoAssignPublicTeam:     true,
				AutoCreatePrivateProject: true,
			},
		},
		{
			Slug:                 "pro",
			Name:                 "Pro",
			Description:          "Expanded individual and team limits.",
			MaxProjects:          10,
			MaxPrivateProjects:   3,
			MaxTickets:           2000,
			MaxTicketsPerProject: 1000,
			MaxTeamMemberships:   20,
			MaxAPICallsPerDay:    5000,
			DefaultProjectAlias:  "public",
			RegistrationActions: RegistrationActions{
				AutoAssignPublicTeam:     true,
				AutoCreatePrivateProject: true,
			},
		},
		{
			Slug:                 "pro+",
			Name:                 "Pro+",
			Description:          "Adds a dedicated private team alongside the private project.",
			MaxProjects:          25,
			MaxPrivateProjects:   10,
			MaxTickets:           10000,
			MaxTicketsPerProject: 5000,
			MaxTeamMemberships:   50,
			MaxAPICallsPerDay:    20000,
			DefaultProjectAlias:  "public",
			RegistrationActions: RegistrationActions{
				AutoAssignPublicTeam:     true,
				AutoCreatePrivateTeam:    true,
				AutoCreatePrivateProject: true,
			},
		},
		{
			Slug:                 EnterprisePlanSlug,
			Name:                 "Enterprise",
			Description:          "Administrative plan with the highest limits and private team provisioning.",
			MaxProjects:          100,
			MaxPrivateProjects:   100,
			MaxTickets:           100000,
			MaxTicketsPerProject: 100000,
			MaxTeamMemberships:   1000,
			MaxAPICallsPerDay:    0,
			DefaultProjectAlias:  "public",
			RegistrationActions: RegistrationActions{
				AutoAssignPublicTeam:     true,
				AutoCreatePrivateTeam:    true,
				AutoCreatePrivateProject: true,
			},
		},
	}
	for _, def := range definitions {
		current, err := GetPlanBySlug(ctx, db, def.Slug)
		switch {
		case err == nil:
			if _, updateErr := UpdatePlan(ctx, db, current.ID, PlanUpdateParams{
				Slug:                      def.Slug,
				Name:                      def.Name,
				Description:               def.Description,
				MaxProjects:               def.MaxProjects,
				MaxPrivateProjects:        def.MaxPrivateProjects,
				MaxTickets:                def.MaxTickets,
				MaxTicketsPerProject:      def.MaxTicketsPerProject,
				MaxTeamMemberships:        def.MaxTeamMemberships,
				MaxAPICallsPerDay:         def.MaxAPICallsPerDay,
				DefaultProjectAlias:       def.DefaultProjectAlias,
				RegistrationActions:       def.RegistrationActions,
				ReplaceRegistrationAction: true,
			}); updateErr != nil {
				return updateErr
			}
		case errors.Is(err, ErrPlanNotFound):
			if _, createErr := CreatePlan(ctx, db, def); createErr != nil {
				return createErr
			}
		default:
			return err
		}
	}
	if err := SetDefaultPlanSlug(ctx, db, DefaultPlanSlug); err != nil {
		return err
	}
	return nil
}

func ensurePublicResources(ctx context.Context, db *sql.DB, adminUserID string) (Team, Project, error) {
	team, err := getTeamByName(ctx, db, publicTeamName)
	switch {
	case err == nil:
	case errors.Is(err, ErrTeamNotFound):
		team, err = CreateTeam(ctx, db, publicTeamName, nil)
		if err != nil {
			return Team{}, Project{}, err
		}
	default:
		return Team{}, Project{}, err
	}
	project, err := GetProjectByAlias(ctx, db, "public", "")
	switch {
	case err == nil:
	case errors.Is(err, ErrProjectNotFound):
		project, err = CreateProjectWithParams(ctx, db, ProjectCreateParams{
			Prefix:        publicProjectPrefix,
			Title:         publicProjectTitle,
			Description:   publicProjectDesc,
			Visibility:    ProjectVisibilityPublic,
			CreatedBy:     adminUserID,
			GitRepository: "",
		})
		if err != nil {
			return Team{}, Project{}, err
		}
	default:
		return Team{}, Project{}, err
	}
	if _, addErr := AddProjectTeamMember(ctx, db, project.ID, team.ID, ProjectRoleMember); addErr != nil {
		return Team{}, Project{}, addErr
	}
	if aliasErr := SetProjectAlias(ctx, db, project.ID, "public", ""); aliasErr != nil {
		return Team{}, Project{}, aliasErr
	}
	plans, err := ListPlans(ctx, db)
	if err != nil {
		return Team{}, Project{}, err
	}
	for _, plan := range plans {
		actions := plan.RegistrationActions
		if actions.AutoAssignPublicTeam {
			found := false
			for _, item := range actions.Teams {
				if item.TeamID == team.ID {
					found = true
					break
				}
			}
			if !found {
				actions.Teams = append(actions.Teams, RegistrationTeamAssignment{TeamID: team.ID, Role: TeamRoleMember})
				if _, err := UpdatePlan(ctx, db, plan.ID, PlanUpdateParams{
					Slug:                      plan.Slug,
					Name:                      plan.Name,
					Description:               plan.Description,
					MaxProjects:               plan.MaxProjects,
					MaxPrivateProjects:        plan.MaxPrivateProjects,
					MaxTickets:                plan.MaxTickets,
					MaxTicketsPerProject:      plan.MaxTicketsPerProject,
					MaxTeamMemberships:        plan.MaxTeamMemberships,
					MaxAPICallsPerDay:         plan.MaxAPICallsPerDay,
					DefaultProjectAlias:       plan.DefaultProjectAlias,
					RegistrationActions:       actions,
					ReplaceRegistrationAction: true,
				}); err != nil {
					return Team{}, Project{}, err
				}
			}
		}
	}
	return team, project, nil
}

func getTeamByName(ctx context.Context, db *sql.DB, name string) (Team, error) {
	row := db.QueryRowContext(ctx, `
		SELECT team_id, name, parent_team_id, created_at, updated_at
		FROM teams
		WHERE lower(name) = lower(?)
	`, strings.TrimSpace(name))
	var team Team
	if err := row.Scan(&team.ID, &team.Name, &team.ParentTeamID, &team.CreatedAt, &team.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Team{}, ErrTeamNotFound
		}
		return Team{}, err
	}
	return team, nil
}

func ensurePersonalResources(ctx context.Context, db *sql.DB, user User, plan Plan) error {
	actions := plan.RegistrationActions
	for _, teamAssignment := range actions.Teams {
		if _, err := AddTeamMember(ctx, db, teamAssignment.TeamID, user.ID, teamAssignment.Role, ""); err != nil {
			return err
		}
	}
	for _, projectAssignment := range actions.Projects {
		if _, err := AddProjectMember(ctx, db, projectAssignment.ProjectID, user.ID, projectAssignment.Role); err != nil {
			return err
		}
	}
	privateTeamID := int64(0)
	if actions.AutoCreatePrivateTeam {
		name := fmt.Sprintf("%s-private", user.Username)
		team, err := getTeamByName(ctx, db, name)
		if err != nil {
			if !errors.Is(err, ErrTeamNotFound) {
				return err
			}
			team, err = CreateTeam(ctx, db, name, nil)
			if err != nil {
				return err
			}
		}
		privateTeamID = team.ID
		if _, err := AddTeamMember(ctx, db, team.ID, user.ID, TeamRoleOwner, ""); err != nil {
			return err
		}
	}
	if actions.AutoCreatePrivateProject {
		if _, err := GetProjectByAlias(ctx, db, "private", user.ID); err == nil {
			return nil
		} else if !errors.Is(err, ErrProjectNotFound) {
			return err
		}
		project, err := CreateProjectWithParams(ctx, db, ProjectCreateParams{
			Prefix:      "PRIV",
			Title:       "Private",
			Description: privateProjectDesc,
			Visibility:  ProjectVisibilityPrivate,
			CreatedBy:   user.ID,
		})
		if err != nil {
			return err
		}
		if privateTeamID != 0 {
			if _, err := AddProjectTeamMember(ctx, db, project.ID, privateTeamID, ProjectRoleAdmin); err != nil {
				return err
			}
		}
		if err := SetProjectAlias(ctx, db, project.ID, "private", user.ID); err != nil {
			return err
		}
	}
	return nil
}

func ensureBootstrapTicketProject(ctx context.Context, db *sql.DB, adminUserID string) (Project, error) {
	project, err := GetProjectByGitRepository(ctx, db, ticketProjectRepo)
	switch {
	case err == nil:
	case errors.Is(err, ErrProjectNotFound):
		project, err = CreateProjectWithParams(ctx, db, ProjectCreateParams{
			Prefix:        ticketProjectPrefix,
			Title:         ticketProjectTitle,
			Visibility:    ProjectVisibilityPrivate,
			CreatedBy:     adminUserID,
			GitRepository: ticketProjectRepo,
		})
		if err != nil {
			return Project{}, err
		}
	default:
		return Project{}, err
	}
	if _, err := AddProjectMember(ctx, db, project.ID, adminUserID, ProjectRoleAdmin); err != nil {
		return Project{}, err
	}
	return project, nil
}
