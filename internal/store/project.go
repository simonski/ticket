package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/config"
)

var ErrProjectNotFound = errors.New("project not found")
var ErrProjectAmbiguous = errors.New("project reference is ambiguous")

const (
	ProjectVisibilityPrivate = "private"
	ProjectVisibilityTeam    = "team"
	ProjectVisibilityPublic  = "public"
)

type Project struct {
	ID                 int64       `json:"project_id"`
	Prefix             string      `json:"prefix"`
	Title              string      `json:"title"`
	Description        string      `json:"description"`
	AcceptanceCriteria string      `json:"acceptance_criteria"`
	DORMap             GuidanceMap `json:"dor_map,omitempty"`
	DODMap             GuidanceMap `json:"dod_map,omitempty"`
	ACMap              GuidanceMap `json:"ac_map,omitempty"`
	GitRepository      string      `json:"git_repository"`
	Notes              string      `json:"notes"`
	Status             string      `json:"status"`
	Visibility         string      `json:"visibility"`
	AcceptsNewMembers  bool        `json:"accepts_new_members"`
	DefaultDraft       bool        `json:"default_draft"`
	CreatedBy          string      `json:"created_by"`
	CreatedAt          string      `json:"created_at"`
	UpdatedAt          string      `json:"updated_at"`
	WorkflowID         *int64      `json:"workflow_id,omitempty"`
	AgentModelProvider string      `json:"agent_model_provider"`
	AgentModelName     string      `json:"agent_model_name"`
	AgentModelURL      string      `json:"agent_model_url"`
	AgentModelAPIKey   string      `json:"agent_model_api_key"`
	ProgrammeID        *int64      `json:"programme_id"`
	Attrs              Attrs       `json:"attrs,omitempty"`
}

func (p Project) ResolveGuidance(stage string) ResolvedGuidance {
	return resolveGuidance(stage, p.DORMap, p.DODMap, p.ACMap)
}

type ProjectCreateParams struct {
	ID                 *int64
	Prefix             string
	Title              string
	Description        string
	AcceptanceCriteria string
	DORMap             GuidanceMap
	DODMap             GuidanceMap
	ACMap              GuidanceMap
	GitRepository      string
	Notes              string
	Visibility         string
	AcceptsNewMembers  bool
	CreatedBy          string
	WorkflowID         *int64
	AgentModelProvider string
	AgentModelName     string
	AgentModelURL      string
	AgentModelAPIKey   string
	Attrs              Attrs
}

type ProjectUpdateParams struct {
	Title              string
	Description        string
	AcceptanceCriteria string
	DORMap             GuidanceMap
	DODMap             GuidanceMap
	ACMap              GuidanceMap
	GitRepository      string
	Notes              string
	Status             string
	Visibility         string
	AcceptsNewMembers  bool
	WorkflowID         *int64
	AgentModelProvider string
	AgentModelName     string
	AgentModelURL      string
	AgentModelAPIKey   string
	Attrs              Attrs // nil = preserve existing attrs; non-nil = replace the bag
}

func CreateProject(ctx context.Context, db *sql.DB, title, description, acceptanceCriteria, createdBy string) (Project, error) {
	return CreateProjectWithParams(ctx, db, ProjectCreateParams{
		Prefix:             deriveProjectPrefix(title),
		Title:              title,
		Description:        description,
		AcceptanceCriteria: acceptanceCriteria,
		CreatedBy:          createdBy,
	})
}

func CreateProjectWithParams(ctx context.Context, db *sql.DB, params ProjectCreateParams) (Project, error) {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return Project{}, errors.New("project title is required")
	}
	prefix := normalizeProjectPrefix(params.Prefix)
	if prefix == "" {
		prefix = deriveProjectPrefix(title)
	}
	if err := validateProjectPrefix(prefix); err != nil {
		return Project{}, err
	}
	visibility := normalizeProjectVisibility(params.Visibility)
	if visibility == "" {
		visibility = ProjectVisibilityPublic
	}
	if !validProjectVisibility(visibility) {
		return Project{}, fmt.Errorf("invalid project visibility %q", params.Visibility)
	}
	uniquePrefix, err := nextUniqueProjectPrefix(ctx, db, prefix)
	if err != nil {
		return Project{}, err
	}
	// Default to the first available workflow if none specified
	workflowID := params.WorkflowID
	if workflowID == nil {
		var wfID int64
		if queryErr := db.QueryRowContext(ctx, `SELECT workflow_id FROM workflows ORDER BY workflow_id LIMIT 1`).Scan(&wfID); queryErr == nil {
			workflowID = &wfID
		}
	}
	explicitID, hasExplicitID, err := normalizeExplicitID(params.ID)
	if err != nil {
		return Project{}, err
	}
	acMap := withLegacyAcceptanceCriteria(params.AcceptanceCriteria, params.ACMap)
	acceptanceCriteria := strings.TrimSpace(params.AcceptanceCriteria)
	if acceptanceCriteria == "" && acMap != nil {
		acceptanceCriteria = acMap[DefaultGuidanceStageKey]
	}
	// agent_model_* (TK-112) and dor/dod/ac guidance maps (TK-115) live in attrs.
	attrsJSON, err := projectAttrsForWrite(params.Attrs, params.AgentModelProvider, params.AgentModelName, params.AgentModelURL, params.AgentModelAPIKey, params.DORMap, params.DODMap, acMap)
	if err != nil {
		return Project{}, err
	}
	query := `
		INSERT INTO projects (prefix, title, description, acceptance_criteria, git_repository, notes, status, visibility, created_by, workflow_id, attrs)
		VALUES (?, ?, ?, ?, ?, ?, 'open', ?, ?, ?, ?)
	`
	args := []any{
		uniquePrefix, title, strings.TrimSpace(params.Description), acceptanceCriteria,
		strings.TrimSpace(params.GitRepository), strings.TrimSpace(params.Notes), visibility, nullableUserID(params.CreatedBy), workflowID,
		attrsJSON,
	}
	if hasExplicitID {
		query = `
			INSERT INTO projects (project_id, prefix, title, description, acceptance_criteria, git_repository, notes, status, visibility, created_by, workflow_id, attrs)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'open', ?, ?, ?, ?)
		`
		args = append([]any{explicitID}, args...)
	}
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return Project{}, err
	}
	id := explicitID
	if !hasExplicitID {
		id, err = result.LastInsertId()
		if err != nil {
			return Project{}, err
		}
	}
	if params.CreatedBy != "" {
		if _, err := AddProjectMember(ctx, db, id, params.CreatedBy, ProjectRoleOwner); err != nil {
			return Project{}, err
		}
	}
	if repo := strings.TrimSpace(params.GitRepository); repo != "" {
		if err := AddProjectGitRepository(ctx, db, id, repo); err != nil {
			return Project{}, err
		}
	}
	if err := SetProjectAcceptsNewMembers(ctx, db, id, params.AcceptsNewMembers); err != nil {
		return Project{}, err
	}
	return GetProjectByID(ctx, db, id)
}

// projectAttrAgentModelKeys are the agent-model config fields that now live in the
// projects attrs bag (TK-112), surfaced as typed Project fields for back-compat.
var projectAttrAgentModelKeys = []string{"agent_model_provider", "agent_model_name", "agent_model_url", "agent_model_api_key"}

// hydrateProjectAttrs copies the bag-backed agent-model fields into typed Project
// fields and strips them from the visible bag.
func hydrateProjectAttrs(p *Project) {
	p.AgentModelProvider = p.Attrs.GetString("agent_model_provider")
	p.AgentModelName = p.Attrs.GetString("agent_model_name")
	p.AgentModelURL = p.Attrs.GetString("agent_model_url")
	p.AgentModelAPIKey = p.Attrs.GetString("agent_model_api_key")
	// Guidance maps folded into the bag (TK-115).
	p.DORMap = guidanceMapFromAttr(p.Attrs["dor_map"])
	p.DODMap = guidanceMapFromAttr(p.Attrs["dod_map"])
	p.ACMap = withLegacyAcceptanceCriteria(p.AcceptanceCriteria, guidanceMapFromAttr(p.Attrs["ac_map"]))
	for _, k := range projectAttrAgentModelKeys {
		delete(p.Attrs, k)
	}
	for _, k := range []string{"dor_map", "dod_map", "ac_map"} {
		delete(p.Attrs, k)
	}
	if len(p.Attrs) == 0 {
		p.Attrs = nil
	}
}

// projectAttrsForWrite merges the bag-backed agent-model fields and guidance maps
// into a base bag (TK-112, TK-115).
func projectAttrsForWrite(base Attrs, provider, name, url, apiKey string, dor, dod, ac GuidanceMap) (string, error) {
	merged := Attrs{}
	for k, v := range base {
		merged[k] = v
	}
	merged.SetString("agent_model_provider", strings.TrimSpace(provider))
	merged.SetString("agent_model_name", strings.TrimSpace(name))
	merged.SetString("agent_model_url", strings.TrimSpace(url))
	merged.SetString("agent_model_api_key", strings.TrimSpace(apiKey))
	setGuidanceAttr(merged, "dor_map", dor)
	setGuidanceAttr(merged, "dod_map", dod)
	setGuidanceAttr(merged, "ac_map", ac)
	return marshalAttrs(merged)
}

func scanProject(s scanner) (Project, error) {
	var project Project
	var workflowID sql.NullInt64
	var programmeID sql.NullInt64
	var attrsJSON sql.NullString
	if err := s.Scan(
		&project.ID, &project.Prefix, &project.Title, &project.Description, &project.AcceptanceCriteria,
		&project.GitRepository, &project.Notes, &project.Status, &project.Visibility,
		&project.DefaultDraft, &project.CreatedBy, &project.CreatedAt, &project.UpdatedAt, &workflowID,
		&programmeID, &attrsJSON,
	); err != nil {
		return Project{}, err
	}
	var err error
	project.Attrs, err = parseAttrs(attrsJSON.String)
	if err != nil {
		return Project{}, err
	}
	hydrateProjectAttrs(&project)
	if workflowID.Valid {
		project.WorkflowID = &workflowID.Int64
	}
	if programmeID.Valid {
		project.ProgrammeID = &programmeID.Int64
	}
	return project, nil
}

func ListProjects(ctx context.Context, db *sql.DB, limit int) ([]Project, error) {
	if err := ensureProjectAccessTables(ctx, db); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 1000
	}
	rows, err := db.QueryContext(ctx, `
		SELECT project_id, prefix, title, description, acceptance_criteria, git_repository, notes, status, visibility, default_draft, COALESCE(created_by, ''), created_at, updated_at, workflow_id, programme_id, attrs
		FROM projects
		ORDER BY created_at, project_id
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range projects {
		enabled, err := AcceptsNewMembers(ctx, db, projects[i].ID)
		if err != nil {
			return nil, err
		}
		projects[i].AcceptsNewMembers = enabled
	}
	return projects, nil
}

func ListProjectsVisibleToUser(ctx context.Context, db *sql.DB, user User) ([]Project, error) {
	if err := ensureProjectAccessTables(ctx, db); err != nil {
		return nil, err
	}
	if user.Role == "admin" {
		return ListProjects(ctx, db, 0)
	}
	rows, err := db.QueryContext(ctx, `
		WITH RECURSIVE team_scope(team_id, parent_team_id) AS (
			SELECT t.team_id, t.parent_team_id
			FROM teams t
			JOIN team_members tm ON tm.team_id = t.team_id
			WHERE tm.user_id = ?
			UNION
			SELECT parent.team_id, parent.parent_team_id
			FROM teams parent
			JOIN team_scope ts ON ts.parent_team_id = parent.team_id
		)
		SELECT DISTINCT p.project_id, p.prefix, p.title, p.description, p.acceptance_criteria, p.git_repository, p.notes, p.status, p.visibility, p.default_draft, COALESCE(p.created_by, ''), p.created_at, p.updated_at, p.workflow_id, p.programme_id, p.attrs
		FROM projects p
		LEFT JOIN project_members pm ON pm.project_id = p.project_id AND pm.user_id = ?
		LEFT JOIN project_teams pt ON pt.project_id = p.project_id
		LEFT JOIN team_scope ts ON ts.team_id = pt.team_id
		WHERE p.visibility = ? OR pm.user_id IS NOT NULL OR ts.team_id IS NOT NULL
		ORDER BY p.created_at, p.project_id
	`, user.ID, user.ID, ProjectVisibilityPublic)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range projects {
		enabled, err := AcceptsNewMembers(ctx, db, projects[i].ID)
		if err != nil {
			return nil, err
		}
		projects[i].AcceptsNewMembers = enabled
	}
	return projects, nil
}

func GetProject(ctx context.Context, db *sql.DB, rawID string) (Project, error) {
	if err := ensureProjectAccessTables(ctx, db); err != nil {
		return Project{}, err
	}
	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return Project{}, ErrProjectNotFound
	}
	var id int64
	if _, err := fmt.Sscan(rawID, &id); err == nil {
		return GetProjectByID(ctx, db, id)
	}
	row := db.QueryRowContext(ctx, `
		SELECT project_id, prefix, title, description, acceptance_criteria, git_repository, notes, status, visibility, default_draft, COALESCE(created_by, ''), created_at, updated_at, workflow_id, programme_id, attrs
		FROM projects
		WHERE prefix = ?
	`, strings.ToUpper(rawID))
	project, err := scanProject(row)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return Project{}, err
		}
		return getProjectByTitle(ctx, db, rawID)
	}
	project.AcceptsNewMembers, err = AcceptsNewMembers(ctx, db, project.ID)
	if err != nil {
		return Project{}, err
	}
	return project, nil
}

func GetProjectByID(ctx context.Context, db *sql.DB, id int64) (Project, error) {
	if err := ensureProjectAccessTables(ctx, db); err != nil {
		return Project{}, err
	}
	row := db.QueryRowContext(ctx, `
		SELECT project_id, prefix, title, description, acceptance_criteria, git_repository, notes, status, visibility, default_draft, COALESCE(created_by, ''), created_at, updated_at, workflow_id, programme_id, attrs
		FROM projects
		WHERE project_id = ?
	`, id)
	project, err := scanProject(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrProjectNotFound
		}
		return Project{}, err
	}
	project.AcceptsNewMembers, err = AcceptsNewMembers(ctx, db, project.ID)
	if err != nil {
		return Project{}, err
	}
	return project, nil
}

func GetProjectByGitRepository(ctx context.Context, db *sql.DB, gitRepository string) (Project, error) {
	if err := ensureProjectAccessTables(ctx, db); err != nil {
		return Project{}, err
	}
	if err := ensureProjectRepositoryTable(ctx, db); err != nil {
		return Project{}, err
	}
	canonicalRepo, err := config.CanonicalizeGitRepository(gitRepository)
	if err != nil {
		return Project{}, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT p.project_id, p.prefix, p.title, p.description, p.acceptance_criteria, p.git_repository, p.notes, p.status, p.visibility, p.default_draft, COALESCE(p.created_by, ''), p.created_at, p.updated_at, p.workflow_id, p.programme_id, p.attrs,
		       r.repository
		FROM projects p
		JOIN project_git_repositories r ON r.project_id = p.project_id
		ORDER BY p.project_id, r.repository
	`)
	if err != nil {
		return Project{}, err
	}
	var matched *Project
	for rows.Next() {
		project, repository, scanErr := scanProjectWithRepository(rows)
		if scanErr != nil {
			return Project{}, scanErr
		}
		canonicalCandidate, candidateErr := config.CanonicalizeGitRepository(repository)
		if candidateErr != nil {
			return Project{}, candidateErr
		}
		if canonicalCandidate != canonicalRepo {
			continue
		}
		projectCopy := project
		matched = &projectCopy
		break
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		_ = rows.Close()
		return Project{}, rowsErr
	}
	if closeErr := rows.Close(); closeErr != nil {
		return Project{}, closeErr
	}
	if matched == nil {
		return Project{}, ErrProjectNotFound
	}
	matched.AcceptsNewMembers, err = AcceptsNewMembers(ctx, db, matched.ID)
	if err != nil {
		return Project{}, err
	}
	return *matched, nil
}

func scanProjectWithRepository(s scanner) (Project, string, error) {
	project, err := scanProjectWithWorkflowIDAndRepository(s)
	if err != nil {
		return Project{}, "", err
	}
	return project.project, project.repository, nil
}

type projectWithRepository struct {
	project    Project
	repository string
}

func scanProjectWithWorkflowIDAndRepository(s scanner) (projectWithRepository, error) {
	var result projectWithRepository
	var workflowID sql.NullInt64
	var programmeID sql.NullInt64
	var attrsJSON sql.NullString
	if err := s.Scan(
		&result.project.ID, &result.project.Prefix, &result.project.Title, &result.project.Description, &result.project.AcceptanceCriteria,
		&result.project.GitRepository, &result.project.Notes, &result.project.Status, &result.project.Visibility,
		&result.project.DefaultDraft, &result.project.CreatedBy, &result.project.CreatedAt, &result.project.UpdatedAt, &workflowID,
		&programmeID, &attrsJSON,
		&result.repository,
	); err != nil {
		return projectWithRepository{}, err
	}
	var err error
	result.project.Attrs, err = parseAttrs(attrsJSON.String)
	if err != nil {
		return projectWithRepository{}, err
	}
	hydrateProjectAttrs(&result.project)
	if workflowID.Valid {
		id := workflowID.Int64
		result.project.WorkflowID = &id
	}
	if programmeID.Valid {
		id := programmeID.Int64
		result.project.ProgrammeID = &id
	}
	return result, nil
}

func UpdateProject(ctx context.Context, db *sql.DB, id int64, title, description, acceptanceCriteria string) (Project, error) {
	return UpdateProjectWithParams(ctx, db, id, ProjectUpdateParams{
		Title:              title,
		Description:        description,
		AcceptanceCriteria: acceptanceCriteria,
	})
}

func UpdateProjectWithParams(ctx context.Context, db *sql.DB, id int64, params ProjectUpdateParams) (Project, error) {
	current, err := GetProjectByID(ctx, db, id)
	if err != nil {
		return Project{}, err
	}
	nextTitle := strings.TrimSpace(params.Title)
	if nextTitle == "" {
		nextTitle = current.Title
	}
	nextDescription := params.Description
	if strings.TrimSpace(nextDescription) == "" {
		nextDescription = current.Description
	}
	nextAC := params.AcceptanceCriteria
	if strings.TrimSpace(nextAC) == "" {
		nextAC = current.AcceptanceCriteria
	}
	nextDORMap := current.DORMap
	if params.DORMap != nil {
		nextDORMap = normalizeGuidanceMap(params.DORMap)
	}
	nextDODMap := current.DODMap
	if params.DODMap != nil {
		nextDODMap = normalizeGuidanceMap(params.DODMap)
	}
	nextACMap := current.ACMap
	if params.ACMap != nil {
		nextACMap = params.ACMap
	}
	if strings.TrimSpace(params.AcceptanceCriteria) != "" && params.ACMap == nil {
		nextACMap = withLegacyAcceptanceCriteria(params.AcceptanceCriteria, current.ACMap)
	}
	nextACMap = withLegacyAcceptanceCriteria(nextAC, nextACMap)
	nextRepo := strings.TrimSpace(params.GitRepository)
	if nextRepo == "" {
		nextRepo = current.GitRepository
	}
	nextNotes := strings.TrimSpace(params.Notes)
	if nextNotes == "" {
		nextNotes = current.Notes
	}
	nextVisibility := normalizeProjectVisibility(params.Visibility)
	if nextVisibility == "" {
		nextVisibility = current.Visibility
	}
	if !validProjectVisibility(nextVisibility) {
		return Project{}, fmt.Errorf("invalid project visibility %q", params.Visibility)
	}
	nextStatus := strings.TrimSpace(params.Status)
	if nextStatus == "" {
		nextStatus = current.Status
	}
	nextWorkflowID := params.WorkflowID
	if nextWorkflowID == nil {
		nextWorkflowID = current.WorkflowID
	} else if *nextWorkflowID == 0 {
		nextWorkflowID = nil
	}
	nextAgentModelProvider := strings.TrimSpace(params.AgentModelProvider)
	if nextAgentModelProvider == "" {
		nextAgentModelProvider = current.AgentModelProvider
	}
	nextAgentModelName := strings.TrimSpace(params.AgentModelName)
	if nextAgentModelName == "" {
		nextAgentModelName = current.AgentModelName
	}
	nextAgentModelURL := strings.TrimSpace(params.AgentModelURL)
	if nextAgentModelURL == "" {
		nextAgentModelURL = current.AgentModelURL
	}
	nextAgentModelAPIKey := strings.TrimSpace(params.AgentModelAPIKey)
	if nextAgentModelAPIKey == "" {
		nextAgentModelAPIKey = current.AgentModelAPIKey
	}
	baseAttrs := current.Attrs
	if params.Attrs != nil {
		baseAttrs = params.Attrs
	}
	attrsJSON, err := projectAttrsForWrite(baseAttrs, nextAgentModelProvider, nextAgentModelName, nextAgentModelURL, nextAgentModelAPIKey, nextDORMap, nextDODMap, nextACMap)
	if err != nil {
		return Project{}, err
	}
	_, err = db.ExecContext(ctx, `
		UPDATE projects
		SET title = ?, description = ?, acceptance_criteria = ?, git_repository = ?, notes = ?, status = ?, visibility = ?, workflow_id = ?, attrs = ?, updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ?
	`, nextTitle, nextDescription, nextAC, nextRepo, nextNotes, nextStatus, nextVisibility, nextWorkflowID, attrsJSON, id)
	if err != nil {
		return Project{}, err
	}
	if nextRepo != "" {
		if err := AddProjectGitRepository(ctx, db, id, nextRepo); err != nil {
			return Project{}, err
		}
	}
	if err := SetProjectAcceptsNewMembers(ctx, db, id, params.AcceptsNewMembers); err != nil {
		return Project{}, err
	}
	return GetProjectByID(ctx, db, id)
}

func normalizeProjectVisibility(visibility string) string {
	return strings.TrimSpace(strings.ToLower(visibility))
}

func validProjectVisibility(visibility string) bool {
	switch normalizeProjectVisibility(visibility) {
	case ProjectVisibilityPrivate, ProjectVisibilityTeam, ProjectVisibilityPublic:
		return true
	default:
		return false
	}
}

func getProjectByTitle(ctx context.Context, db *sql.DB, title string) (Project, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT project_id, prefix, title, description, acceptance_criteria, git_repository, notes, status, visibility, default_draft, COALESCE(created_by, ''), created_at, updated_at, workflow_id, programme_id, attrs
		FROM projects
		WHERE LOWER(title) = LOWER(?)
		ORDER BY project_id
		LIMIT 2
	`, strings.TrimSpace(title))
	if err != nil {
		return Project{}, err
	}
	defer rows.Close()

	projects := make([]Project, 0, 2)
	for rows.Next() {
		project, scanErr := scanProject(rows)
		if scanErr != nil {
			return Project{}, scanErr
		}
		projects = append(projects, project)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return Project{}, rowsErr
	}
	switch len(projects) {
	case 0:
		return Project{}, ErrProjectNotFound
	case 1:
		projects[0].AcceptsNewMembers, err = AcceptsNewMembers(ctx, db, projects[0].ID)
		if err != nil {
			return Project{}, err
		}
		return projects[0], nil
	default:
		return Project{}, fmt.Errorf("%w: multiple projects share title %q; use the numeric id or prefix", ErrProjectAmbiguous, strings.TrimSpace(title))
	}
}

func SetProjectStatus(ctx context.Context, db *sql.DB, id int64, enabled bool) (Project, error) {
	status := "closed"
	if enabled {
		status = "open"
	}
	result, err := db.ExecContext(ctx, `UPDATE projects SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, status, id)
	if err != nil {
		return Project{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Project{}, err
	}
	if affected == 0 {
		return Project{}, ErrProjectNotFound
	}
	return GetProjectByID(ctx, db, id)
}

// SetProjectDefaultDraft sets the default_draft flag on a project.
func SetProjectDefaultDraft(ctx context.Context, db *sql.DB, projectID int64, draft bool) error {
	val := 0
	if draft {
		val = 1
	}
	result, err := db.ExecContext(ctx, `UPDATE projects SET default_draft = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, val, projectID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrProjectNotFound
	}
	return nil
}

func SetProjectAgentModelConfig(ctx context.Context, db *sql.DB, projectID int64, cfg AgentModelConfig) (Project, error) {
	result, err := db.ExecContext(ctx, `
		UPDATE projects
		SET attrs = json_set(
			json_set(
				json_set(
					json_set(attrs, '$.agent_model_provider', ?),
					'$.agent_model_name', ?),
				'$.agent_model_url', ?),
			'$.agent_model_api_key', ?),
			updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ?
	`, strings.TrimSpace(cfg.Provider), strings.TrimSpace(cfg.Model), strings.TrimSpace(cfg.URL), strings.TrimSpace(cfg.APIKey), projectID)
	if err != nil {
		return Project{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Project{}, err
	}
	if affected == 0 {
		return Project{}, ErrProjectNotFound
	}
	return GetProjectByID(ctx, db, projectID)
}

// DeleteProject removes a project and all associated data.
func DeleteProject(ctx context.Context, db *sql.DB, id int64) error {
	if _, err := GetProjectByID(ctx, db, id); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Delete child data that references tickets in this project
	if _, err := tx.ExecContext(ctx, `DELETE FROM comments WHERE item_id IN (SELECT ticket_id FROM tickets WHERE project_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM time_entries WHERE ticket_id IN (SELECT ticket_id FROM tickets WHERE project_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM ticket_labels WHERE ticket_id IN (SELECT ticket_id FROM tickets WHERE project_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dependencies WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM history_events WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM ticket_history WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM story_ticket_links WHERE story_id IN (SELECT story_id FROM stories WHERE project_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM stories WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tickets WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_members WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_teams WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET default_project_id = NULL, updated_at = CURRENT_TIMESTAMP WHERE default_project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_aliases WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE project_id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// RenameProjectPrefix changes a project's prefix and re-keys every ticket
// in that project. All foreign-key references (parent_id, clone_of,
// dependencies, comments, history, labels, time entries, story links) are
// updated in a single transaction.
func RenameProjectPrefix(ctx context.Context, db *sql.DB, projectID int64, newPrefix string) (int, error) {
	newPrefix = normalizeProjectPrefix(newPrefix)
	if err := validateProjectPrefix(newPrefix); err != nil {
		return 0, err
	}

	// Check the new prefix isn't already used by another project.
	var existingID int64
	err := db.QueryRowContext(ctx, `SELECT project_id FROM projects WHERE prefix = ?`, newPrefix).Scan(&existingID)
	if err == nil && existingID != projectID {
		return 0, fmt.Errorf("prefix %q is already used by another project", newPrefix)
	}

	// Load project to get current prefix.
	project, err := GetProjectByID(ctx, db, projectID)
	if err != nil {
		return 0, err
	}
	if project.Prefix == newPrefix {
		return 0, nil // nothing to do
	}

	// Load all tickets for this project and compute new keys.
	rows, err := db.QueryContext(ctx, `SELECT ticket_id, type FROM tickets WHERE project_id = ?`, projectID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type keyMapping struct {
		oldKey     string
		newKey     string
		ticketType string
	}
	var mappings []keyMapping
	for rows.Next() {
		var oldKey, ticketType string
		if scanErr := rows.Scan(&oldKey, &ticketType); scanErr != nil {
			return 0, scanErr
		}
		// Extract the sequence number from the old key.
		seq := extractSequence(oldKey)
		if seq <= 0 {
			return 0, fmt.Errorf("could not extract sequence from key %q", oldKey)
		}
		newKey, keyErr := generateTicketKey(newPrefix, ticketType, seq)
		if keyErr != nil {
			return 0, fmt.Errorf("generating new key for %q: %w", oldKey, keyErr)
		}
		mappings = append(mappings, keyMapping{oldKey: oldKey, newKey: newKey, ticketType: ticketType})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return 0, rowsErr
	}

	// PRAGMA foreign_keys must be set outside a transaction in SQLite.
	if _, pragmaErr := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); pragmaErr != nil {
		return 0, pragmaErr
	}
	defer db.ExecContext(ctx, `PRAGMA foreign_keys = ON`) //nolint:errcheck // best-effort restore of FK enforcement

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// Update each ticket key and all references.
	for _, m := range mappings {
		// Primary key
		if _, err := tx.ExecContext(ctx, `UPDATE tickets SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, fmt.Errorf("renaming %s → %s: %w", m.oldKey, m.newKey, err)
		}
		// Parent references
		if _, err := tx.ExecContext(ctx, `UPDATE tickets SET parent_id = ? WHERE parent_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// Clone references
		if _, err := tx.ExecContext(ctx, `UPDATE tickets SET clone_of = ? WHERE clone_of = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// Dependencies
		if _, err := tx.ExecContext(ctx, `UPDATE dependencies SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE dependencies SET depends_on = ? WHERE depends_on = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// Story links
		if _, err := tx.ExecContext(ctx, `UPDATE story_ticket_links SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// History
		if _, err := tx.ExecContext(ctx, `UPDATE history_events SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE ticket_history SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// Comments
		if _, err := tx.ExecContext(ctx, `UPDATE comments SET item_id = ? WHERE item_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// Labels
		if _, err := tx.ExecContext(ctx, `UPDATE ticket_labels SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// Time entries
		if _, err := tx.ExecContext(ctx, `UPDATE time_entries SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
	}

	// Update the project prefix.
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET prefix = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, newPrefix, projectID); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(mappings), nil
}

// extractSequence pulls the numeric suffix from a ticket key.
// E.g. "CUS-T-42" → 42, "TK-7" → 7.
func extractSequence(key string) int64 {
	idx := strings.LastIndex(key, "-")
	if idx < 0 {
		return 0
	}
	n, err := strconv.ParseInt(key[idx+1:], 10, 64)
	if err != nil {
		return 0
	}
	return n
}
