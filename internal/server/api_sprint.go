package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

type sprintRequest struct {
	Title string `json:"title"`
	Stage string `json:"stage"`
}

// handleProjectSprintsRoute handles GET/POST /api/projects/{projectRef}/sprints.
// Returns true if the request was handled.
func handleProjectSprintsRoute(w http.ResponseWriter, req *http.Request, db *sql.DB, projectRef string) bool {
	user, err := requireUser(db, req)
	if err != nil {
		writeAuthError(w, err)
		return true
	}
	project, role, err := resolveProjectPathForUser(req.Context(), db, user, projectRef, req.Method == http.MethodPost)
	if err != nil {
		writeStoreError(w, err)
		return true
	}
	switch req.Method {
	case http.MethodGet:
		if !canReadProject(role) {
			writeAuthError(w, store.ErrForbidden)
			return true
		}
		sprints, listErr := store.ListSprints(req.Context(), db, int(project.ID))
		if listErr != nil {
			writeStoreError(w, listErr)
			return true
		}
		writeJSON(w, http.StatusOK, sprints)
	case http.MethodPost:
		if !canWriteProject(role) {
			writeAuthError(w, store.ErrForbidden)
			return true
		}
		var payload sprintRequest
		if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return true
		}
		sprint, createErr := store.CreateSprint(req.Context(), db, int(project.ID), payload.Title)
		if createErr != nil {
			writeStoreError(w, createErr)
			return true
		}
		writeJSON(w, http.StatusCreated, sprint)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
	return true
}

func (r *router) registerSprintHandlers() {
	db := r.db
	mux := r.mux

	// PUT/DELETE /api/sprints/{id}
	mux.HandleFunc("/api/sprints/", func(w http.ResponseWriter, req *http.Request) {
		trimmed := strings.TrimPrefix(req.URL.Path, "/api/sprints/")
		if strings.Contains(trimmed, "/") {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		sprintID, parseErr := strconv.Atoi(strings.TrimSpace(trimmed))
		if parseErr != nil || sprintID < 1 {
			writeError(w, http.StatusBadRequest, "invalid sprint id")
			return
		}
		user, err := requireUser(db, req)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		switch req.Method {
		case http.MethodPut:
			var payload sprintRequest
			if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			validStages := map[string]bool{"design": true, "active": true, "closed": true}
			stage := payload.Stage
			if stage == "" {
				stage = "design"
			}
			if !validStages[stage] {
				writeError(w, http.StatusBadRequest, "invalid sprint stage: must be design, active, or closed")
				return
			}
			updated, updateErr := store.UpdateSprint(req.Context(), db, sprintID, payload.Title, stage)
			if updateErr != nil {
				writeStoreError(w, updateErr)
				return
			}
			// Auth check: user must be able to write to the project
			_, role, authErr := resolveProjectPathForUser(req.Context(), db, user, strconv.Itoa(updated.ProjectID), false)
			if authErr != nil {
				writeStoreError(w, authErr)
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		case http.MethodDelete:
			_ = user
			if err := store.DeleteSprint(req.Context(), db, sprintID); err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}
