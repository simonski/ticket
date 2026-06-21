package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/store"
	"modernc.org/sqlite"
)

const (
	legacySessionCookieName = "ticket_token"
	hostSessionCookieName   = "__Host-session"
	legacyCSRFCookieName    = "_csrf"
	hostCSRFCookieName      = "__Host-_csrf"
	gitRepositoryHeader     = "X-Ticket-Git-Repository"
)

func resolveLifecycleRequest(status, stage, state string) (resolvedStage, resolvedState string, err error) {
	stage = strings.TrimSpace(strings.ToLower(stage))
	state = strings.TrimSpace(strings.ToLower(state))
	if stage != "" || state != "" {
		if stage != "" && !store.ValidStage(stage) {
			return "", "", fmt.Errorf("invalid stage %q", stage)
		}
		if state != "" && !store.ValidState(state) {
			return "", "", fmt.Errorf("invalid state %q", state)
		}
		if stage != "" && state != "" && !store.ValidLifecycle(stage, state) {
			return "", "", fmt.Errorf("invalid status %q", store.RenderLifecycleStatus(stage, state))
		}
		return stage, state, nil
	}
	if strings.TrimSpace(status) == "" {
		return stage, state, nil
	}
	return store.ParseLifecycleStatus(status)
}

func resolveCreateLifecycleRequest(status, stage, state string) (string, error) {
	stage, state, err := resolveLifecycleRequest(status, stage, state)
	if err != nil {
		return "", err
	}
	if stage == "" {
		stage = store.StageDesign
	}
	if stage != store.StageDesign {
		return "", fmt.Errorf("new tickets must start in %s stage", store.StageDesign)
	}
	if state != "" && !store.ValidLifecycle(stage, state) {
		return "", fmt.Errorf("invalid status %q", store.RenderLifecycleStatus(stage, state))
	}
	return state, nil
}

func nullableTrimmed(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func userFromRequest(db *sql.DB, r *http.Request) (store.User, error) {
	if username, password, ok := r.BasicAuth(); ok {
		return store.AuthenticateUser(r.Context(), db, username, password)
	}
	return store.GetUserByToken(r.Context(), db, bearerToken(r))
}

func requireUser(db *sql.DB, r *http.Request) (store.User, error) {
	return userFromRequest(db, r)
}

func requireAdmin(db *sql.DB, r *http.Request) (store.User, error) {
	user, err := requireUser(db, r)
	if err != nil {
		return store.User{}, err
	}
	if user.Role != "admin" {
		return store.User{}, store.ErrAdminRequired
	}
	return user, nil
}

func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	for _, name := range []string{hostSessionCookieName, legacySessionCookieName} {
		cookie, err := r.Cookie(name)
		if err == nil && strings.TrimSpace(cookie.Value) != "" {
			return strings.TrimSpace(cookie.Value)
		}
	}
	return ""
}

func sessionCookieName(r *http.Request) string {
	if requestIsSecure(r) {
		return hostSessionCookieName
	}
	return legacySessionCookieName
}

func csrfCookieName(r *http.Request) string {
	if requestIsSecure(r) {
		return hostCSRFCookieName
	}
	return legacyCSRFCookieName
}

func intParam(r *http.Request, key string, defaultValue int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return value
}

func queryInt(r *http.Request, key string, defaultValue int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be numeric", key)
	}
	return value, nil
}

func projectRoleForUser(ctx context.Context, db *sql.DB, projectID int64, user store.User) (string, error) {
	if user.Role == "admin" {
		return store.ProjectRoleAdmin, nil
	}
	project, err := store.GetProjectByID(ctx, db, projectID)
	if err != nil {
		return "", err
	}
	role, ok, err := store.ProjectRoleForUser(ctx, db, projectID, user.ID)
	if err != nil {
		return "", err
	}
	if ok {
		return role, nil
	}
	teamIDs, err := store.TeamIDsForUserWithAncestors(ctx, db, user.ID)
	if err != nil {
		return "", err
	}
	teamRole, teamOK, err := store.HighestProjectRoleForTeams(ctx, db, projectID, teamIDs)
	if err != nil {
		return "", err
	}
	if teamOK {
		return teamRole, nil
	}
	if project.Visibility == store.ProjectVisibilityPublic {
		return store.ProjectRoleObserver, nil
	}
	return "", nil
}

func resolveProjectRefForUser(ctx context.Context, db *sql.DB, ref string, user store.User) (store.Project, string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return store.Project{}, "", store.ErrProjectNotFound
	}
	var (
		project store.Project
		err     error
	)
	switch strings.ToLower(ref) {
	case "public":
		project, err = store.GetProjectByAlias(ctx, db, "public", "")
	case "private":
		project, err = store.GetProjectByAlias(ctx, db, "private", user.ID)
	default:
		project, err = store.GetProject(ctx, db, ref)
	}
	if err != nil {
		return store.Project{}, "", err
	}
	role, err := projectRoleForUser(ctx, db, project.ID, user)
	if err != nil {
		return store.Project{}, "", err
	}
	if !canReadProject(role) {
		// #nosec G706 -- log fields are stripped of control characters by sanitizeLogField.
		log.Printf("security: project access denied user=%s ref=%s project_id=%d", sanitizeLogField(user.Username), sanitizeLogField(ref), project.ID)
		if project.AcceptsNewMembers {
			return store.Project{}, "", fmt.Errorf("%w: access denied for project %s; request access via POST /api/projects/%s/access-requests", store.ErrUnauthorized, project.Prefix, sanitizeLogField(ref))
		}
		return store.Project{}, "", store.ErrUnauthorized
	}
	return project, role, nil
}

func sanitizeLogField(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, value)
}

func resolveProjectForWriteRequest(ctx context.Context, db *sql.DB, r *http.Request, user store.User, explicitProjectID int64) (store.Project, string, error) {
	if explicitProjectID > 0 {
		project, role, err := resolveProjectRefForUser(ctx, db, strconv.FormatInt(explicitProjectID, 10), user)
		if err != nil {
			return store.Project{}, "", err
		}
		if !canWriteProject(role) {
			return store.Project{}, "", store.ErrForbidden
		}
		return project, role, nil
	}
	repo := strings.TrimSpace(r.Header.Get(gitRepositoryHeader))
	if repo != "" {
		project, err := store.GetProjectByGitRepository(ctx, db, repo)
		switch {
		case err == nil:
			role, roleErr := projectRoleForUser(ctx, db, project.ID, user)
			if roleErr != nil {
				return store.Project{}, "", roleErr
			}
			if role == "" {
				// #nosec G706 -- log fields are stripped of control characters by sanitizeLogField.
				log.Printf("security: project write denied user=%s ref=%s project_id=%d", sanitizeLogField(user.Username), sanitizeLogField(repo), project.ID)
				if project.AcceptsNewMembers {
					return store.Project{}, "", fmt.Errorf("%w: access denied for project %s; request access via POST /api/projects/%s/access-requests", store.ErrUnauthorized, project.Prefix, sanitizeLogField(project.Prefix))
				}
				return store.Project{}, "", store.ErrUnauthorized
			}
			if !canWriteProject(role) {
				// #nosec G706 -- log fields are stripped of control characters by sanitizeLogField.
				log.Printf("security: project write forbidden user=%s ref=%s project_id=%d role=%s", sanitizeLogField(user.Username), sanitizeLogField(repo), project.ID, sanitizeLogField(role))
				return store.Project{}, "", store.ErrForbidden
			}
			return project, role, nil
		case !errors.Is(err, store.ErrProjectNotFound):
			return store.Project{}, "", err
		}
	}
	project, role, err := resolveUserDefaultProjectForWrite(ctx, db, user)
	switch {
	case err == nil:
		return project, role, nil
	case !errors.Is(err, store.ErrProjectNotFound):
		return store.Project{}, "", err
	}
	project, err = store.GetProjectByAlias(ctx, db, "private", user.ID)
	if err != nil {
		return store.Project{}, "", err
	}
	role, err = projectRoleForUser(ctx, db, project.ID, user)
	if err != nil {
		return store.Project{}, "", err
	}
	if !canWriteProject(role) {
		return store.Project{}, "", store.ErrUnauthorized
	}
	return project, role, nil
}

func resolveUserDefaultProjectForWrite(ctx context.Context, db *sql.DB, user store.User) (store.Project, string, error) {
	project, err := store.GetUserDefaultProject(ctx, db, user.ID)
	if err != nil {
		return store.Project{}, "", err
	}
	role, err := projectRoleForUser(ctx, db, project.ID, user)
	if err != nil {
		return store.Project{}, "", err
	}
	if !canReadProject(role) {
		return store.Project{}, "", store.ErrProjectNotFound
	}
	if !canWriteProject(role) {
		return store.Project{}, "", store.ErrForbidden
	}
	return project, role, nil
}

func resolveProjectPathForUser(ctx context.Context, db *sql.DB, user store.User, ref string, requireWrite bool) (store.Project, string, error) {
	project, role, err := resolveProjectRefForUser(ctx, db, ref, user)
	if err != nil {
		return store.Project{}, "", err
	}
	if requireWrite && !canWriteProject(role) {
		return store.Project{}, "", store.ErrForbidden
	}
	return project, role, nil
}

func resolveProjectForDependencyRequest(ctx context.Context, db *sql.DB, r *http.Request, user store.User, explicitProjectID int64, ticketID, dependsOn string) (store.Project, string, error) {
	if explicitProjectID > 0 {
		return resolveProjectForWriteRequest(ctx, db, r, user, explicitProjectID)
	}
	ticketID = strings.TrimSpace(ticketID)
	dependsOn = strings.TrimSpace(dependsOn)
	if ticketID != "" {
		ticket, err := store.GetTicketByRef(ctx, db, ticketID)
		if err != nil {
			return store.Project{}, "", err
		}
		if dependsOn != "" {
			blocker, blockerErr := store.GetTicketByRef(ctx, db, dependsOn)
			if blockerErr != nil {
				return store.Project{}, "", blockerErr
			}
			if blocker.ProjectID != ticket.ProjectID {
				return store.Project{}, "", errors.New("ticket_id and depends_on must belong to the same project")
			}
		}
		project, err := store.GetProjectByID(ctx, db, ticket.ProjectID)
		if err != nil {
			return store.Project{}, "", err
		}
		role, err := projectRoleForUser(ctx, db, project.ID, user)
		if err != nil {
			return store.Project{}, "", err
		}
		if !canWriteProject(role) {
			return store.Project{}, "", store.ErrForbidden
		}
		return project, role, nil
	}
	return resolveProjectForWriteRequest(ctx, db, r, user, 0)
}

func canReadProject(role string) bool {
	switch role {
	case store.ProjectRoleObserver, store.ProjectRoleCommenter, store.ProjectRoleMember, store.ProjectRoleAdmin:
		return true
	default:
		return false
	}
}

func canWriteProject(role string) bool {
	switch role {
	case store.ProjectRoleMember, store.ProjectRoleAdmin:
		return true
	default:
		return false
	}
}

func canAdminProject(role string) bool {
	return role == store.ProjectRoleAdmin
}

func canCommentProject(role string) bool {
	switch role {
	case store.ProjectRoleCommenter, store.ProjectRoleMember, store.ProjectRoleAdmin:
		return true
	default:
		return false
	}
}

func writeStoreError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeError(w, http.StatusNotFound, err.Error())
	case isDatabaseError(err):
		writeError(w, http.StatusInternalServerError, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func isDatabaseError(err error) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) ||
		errors.Is(err, sql.ErrTxDone) ||
		errors.Is(err, sql.ErrConnDone)
}

func canManageProjectUsers(role string) bool {
	return role == store.ProjectRoleAdmin
}

func canViewInterventions(role string) bool {
	switch role {
	case store.ProjectRoleMember, store.ProjectRoleAdmin:
		return true
	default:
		return false
	}
}

func canViewWorkItems(role string) bool {
	switch role {
	case store.ProjectRoleMember, store.ProjectRoleAdmin:
		return true
	default:
		return false
	}
}

func canManageInterventions(role string) bool {
	return role == store.ProjectRoleAdmin
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("server: writeJSON encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, store.ErrProjectAmbiguous):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, store.ErrAdminRequired):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
