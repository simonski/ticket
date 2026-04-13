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

func resolveLifecycleRequest(status, stage, state string) (string, string, error) {
	if strings.TrimSpace(stage) != "" || strings.TrimSpace(state) != "" {
		return stage, state, nil
	}
	if strings.TrimSpace(status) == "" {
		return stage, state, nil
	}
	return store.ParseLifecycleStatus(status)
}

func autoProgressTicketLifecycle(payload ticketRequest, current store.Ticket, actorUsername string) ticketRequest {
	if hasExplicitLifecycleChange(payload, current) {
		return payload
	}
	if !hasMeaningfulTicketContentChange(payload, current) {
		return payload
	}
	nextAssignee := strings.TrimSpace(payload.Assignee)
	if nextAssignee == "" {
		nextAssignee = strings.TrimSpace(current.Assignee)
	}
	switch current.Stage {
	case store.StageDesign:
		payload.Stage = store.StageDevelop
		payload.State = store.StateActive
		if nextAssignee == "" {
			payload.Assignee = strings.TrimSpace(actorUsername)
		}
	case store.StageDevelop:
		payload.Stage = store.StageDevelop
		payload.State = store.StateActive
		if nextAssignee == "" {
			payload.Assignee = strings.TrimSpace(actorUsername)
		}
		if strings.TrimSpace(payload.EstimateComplete) != "" && strings.TrimSpace(payload.EstimateComplete) != strings.TrimSpace(current.EstimateComplete) {
			payload.Stage = store.StageTest
			payload.State = store.StateActive
		}
	case store.StageTest:
		payload.Stage = store.StageTest
		payload.State = store.StateActive
		if nextAssignee == "" {
			payload.Assignee = strings.TrimSpace(actorUsername)
		}
	}
	return payload
}

func hasExplicitLifecycleChange(payload ticketRequest, current store.Ticket) bool {
	if strings.TrimSpace(payload.Status) != "" {
		return true
	}
	stage := strings.TrimSpace(strings.ToLower(payload.Stage))
	state := strings.TrimSpace(strings.ToLower(payload.State))
	if stage == "" && state == "" {
		return false
	}
	return stage != current.Stage || state != current.State
}

func hasMeaningfulTicketContentChange(payload ticketRequest, current store.Ticket) bool {
	if payload.Title != current.Title {
		return true
	}
	if payload.Description != current.Description {
		return true
	}
	if payload.AcceptanceCriteria != current.AcceptanceCriteria {
		return true
	}
	if payload.Priority != current.Priority {
		return true
	}
	if payload.Order != current.Order {
		return true
	}
	if payload.EstimateEffort != current.EstimateEffort {
		return true
	}
	if strings.TrimSpace(payload.EstimateComplete) != strings.TrimSpace(current.EstimateComplete) {
		return true
	}
	if strings.TrimSpace(payload.Assignee) != strings.TrimSpace(current.Assignee) {
		return true
	}
	if (payload.ParentID == nil) != (current.ParentID == nil) {
		return true
	}
	if payload.ParentID != nil && current.ParentID != nil && *payload.ParentID != *current.ParentID {
		return true
	}
	return false
}

func nullableTrimmed(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func userFromRequest(db *sql.DB, r *http.Request) (store.User, error) {
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
	cookie, err := r.Cookie("ticket_token")
	if err == nil && strings.TrimSpace(cookie.Value) != "" {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
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
		return store.ProjectRoleOwner, nil
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
		return store.ProjectRoleViewer, nil
	}
	return "", nil
}

func canReadProject(role string) bool {
	switch role {
	case store.ProjectRoleViewer, store.ProjectRoleEditor, store.ProjectRoleOwner:
		return true
	default:
		return false
	}
}

func canWriteProject(role string) bool {
	switch role {
	case store.ProjectRoleEditor, store.ProjectRoleOwner:
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
	return role == store.ProjectRoleOwner
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
	case errors.Is(err, store.ErrAdminRequired):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
