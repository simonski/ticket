package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

type releaseRequest struct {
	Title      string `json:"title"`
	Purpose    string `json:"purpose"`
	TargetDate string `json:"target_date"`
}

type releaseStatusRequest struct {
	Status string `json:"status"`
}

// handleProjectReleasesRoute handles GET/POST /api/projects/{projectRef}/releases.
func handleProjectReleasesRoute(w http.ResponseWriter, req *http.Request, db *sql.DB, projectRef string) bool {
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
		releases, listErr := store.ListReleases(req.Context(), db, int(project.ID))
		if listErr != nil {
			writeStoreError(w, listErr)
			return true
		}
		writeJSON(w, http.StatusOK, releases)
	case http.MethodPost:
		if !canWriteProject(role) {
			writeAuthError(w, store.ErrForbidden)
			return true
		}
		var payload releaseRequest
		if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return true
		}
		release, createErr := store.CreateRelease(req.Context(), db, int(project.ID),
			strings.TrimSpace(payload.Title), strings.TrimSpace(payload.Purpose), strings.TrimSpace(payload.TargetDate))
		if createErr != nil {
			writeStoreError(w, createErr)
			return true
		}
		writeJSON(w, http.StatusCreated, release)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
	return true
}

func (r *router) registerReleaseHandlers() {
	db := r.db
	mux := r.mux
	notify := r.notify

	// PUT/DELETE /api/releases/{id} and POST /api/releases/{id}/status
	mux.HandleFunc("/api/releases/", func(w http.ResponseWriter, req *http.Request) {
		trimmed := strings.TrimPrefix(req.URL.Path, "/api/releases/")
		parts := strings.Split(strings.Trim(trimmed, "/"), "/")
		releaseID, parseErr := strconv.Atoi(strings.TrimSpace(parts[0]))
		if parseErr != nil || releaseID < 1 {
			writeError(w, http.StatusBadRequest, "invalid release id")
			return
		}
		user, err := requireUser(db, req)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		// Authorise against the release's project.
		existing, relErr := store.GetRelease(req.Context(), db, releaseID)
		if relErr != nil {
			writeStoreError(w, relErr)
			return
		}
		_, projRole, authErr := resolveProjectPathForUser(req.Context(), db, user, strconv.Itoa(existing.ProjectID), false)
		if authErr != nil {
			writeStoreError(w, authErr)
			return
		}
		if !canWriteProject(projRole) {
			writeAuthError(w, store.ErrForbidden)
			return
		}

		// POST /api/releases/{id}/status
		if len(parts) == 2 && parts[1] == "status" && req.Method == http.MethodPost {
			var payload releaseStatusRequest
			if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			updated, statusErr := store.SetReleaseStatus(req.Context(), db, releaseID, strings.TrimSpace(payload.Status))
			if statusErr != nil {
				writeStoreError(w, statusErr)
				return
			}
			if notify != nil {
				notify("release_updated", int64(updated.ProjectID), "")
			}
			writeJSON(w, http.StatusOK, updated)
			return
		}
		if len(parts) != 1 {
			writeError(w, http.StatusNotFound, "not found")
			return
		}

		switch req.Method {
		case http.MethodPut:
			var payload releaseRequest
			if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			updated, updateErr := store.UpdateRelease(req.Context(), db, releaseID,
				strings.TrimSpace(payload.Title), strings.TrimSpace(payload.Purpose), strings.TrimSpace(payload.TargetDate))
			if updateErr != nil {
				writeStoreError(w, updateErr)
				return
			}
			if notify != nil {
				notify("release_updated", int64(updated.ProjectID), "")
			}
			writeJSON(w, http.StatusOK, updated)
		case http.MethodDelete:
			if err := store.DeleteRelease(req.Context(), db, releaseID); err != nil {
				writeStoreError(w, err)
				return
			}
			if notify != nil {
				notify("release_updated", int64(existing.ProjectID), "")
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}
