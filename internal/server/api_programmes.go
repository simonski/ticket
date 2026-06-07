package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func (r *router) registerProgrammeHandlers() {
	mux := r.mux
	db := r.db

	mux.HandleFunc("/api/programmes", func(w http.ResponseWriter, req *http.Request) {
		user, err := requireUser(db, req)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		switch req.Method {
		case http.MethodGet:
			programmes, listErr := store.ListProgrammes(req.Context(), db)
			if listErr != nil {
				writeError(w, http.StatusInternalServerError, listErr.Error())
				return
			}
			if programmes == nil {
				programmes = []store.Programme{}
			}
			writeJSON(w, http.StatusOK, programmes)
		case http.MethodPost:
			if user.Role != "admin" {
				writeError(w, http.StatusForbidden, "admin required")
				return
			}
			var payload struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			programme, createErr := store.CreateProgramme(req.Context(), db, payload.Name, payload.Description)
			if createErr != nil {
				writeError(w, http.StatusInternalServerError, createErr.Error())
				return
			}
			writeJSON(w, http.StatusCreated, programme)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/programmes/", func(w http.ResponseWriter, req *http.Request) {
		trimmed := strings.TrimPrefix(req.URL.Path, "/api/programmes/")
		if strings.Contains(trimmed, "/") {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		id, parseErr := strconv.ParseInt(strings.TrimSpace(trimmed), 10, 64)
		if parseErr != nil || id < 1 {
			writeError(w, http.StatusBadRequest, "invalid programme id")
			return
		}
		user, err := requireUser(db, req)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		switch req.Method {
		case http.MethodGet:
			programme, getErr := store.GetProgramme(req.Context(), db, id)
			if getErr != nil {
				writeStoreError(w, getErr)
				return
			}
			writeJSON(w, http.StatusOK, programme)
		case http.MethodPut:
			if user.Role != "admin" {
				writeError(w, http.StatusForbidden, "admin required")
				return
			}
			var payload struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			programme, updateErr := store.UpdateProgramme(req.Context(), db, id, payload.Name, payload.Description)
			if updateErr != nil {
				writeStoreError(w, updateErr)
				return
			}
			writeJSON(w, http.StatusOK, programme)
		case http.MethodDelete:
			if user.Role != "admin" {
				writeError(w, http.StatusForbidden, "admin required")
				return
			}
			if deleteErr := store.DeleteProgramme(req.Context(), db, id); deleteErr != nil {
				writeStoreError(w, deleteErr)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}

func handleProjectProgrammeRoute(w http.ResponseWriter, req *http.Request, db *sql.DB, projectRef string) bool {
	if req.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return true
	}
	user, err := requireUser(db, req)
	if err != nil {
		writeAuthError(w, err)
		return true
	}
	if user.Role != "admin" {
		writeError(w, http.StatusForbidden, "admin required")
		return true
	}
	projectID, parseErr := strconv.ParseInt(strings.TrimSpace(projectRef), 10, 64)
	if parseErr != nil || projectID < 1 {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return true
	}
	var payload struct {
		ProgrammeID *int64 `json:"programme_id"`
	}
	if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return true
	}
	if setErr := store.SetProjectProgramme(req.Context(), db, projectID, payload.ProgrammeID); setErr != nil {
		writeError(w, http.StatusInternalServerError, setErr.Error())
		return true
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return true
}
