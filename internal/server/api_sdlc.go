package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func (r *router) registerSdlcHandlers() {
	db := r.db
	mux := r.mux

	// Sdlc endpoints
	mux.HandleFunc("/api/sdlcs/import", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var export store.SdlcExport
		if err := json.NewDecoder(r.Body).Decode(&export); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		wf, err := store.ImportSdlc(r.Context(), db, export)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, wf)
	})

	mux.HandleFunc("/api/sdlcs/stages/", func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/sdlcs/stages/")
		// Skip if this is a roles sub-path (handled by a different handler)
		if strings.HasPrefix(trimmed, "roles/") {
			return
		}
		var stageID int64
		if _, err := fmt.Sscan(strings.TrimSpace(trimmed), &stageID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid stage id")
			return
		}
		switch r.Method {
		case http.MethodGet:
			if _, err := requireUser(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			stage, err := store.GetSdlcStage(r.Context(), db, stageID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "sdlc stage not found")
					return
				}
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, stage)
		case http.MethodPut:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var payload sdlcStageRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			stage, err := store.UpdateSdlcStage(r.Context(), db, stageID, payload.StageName, payload.Description, payload.AcceptanceCriteria)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "sdlc stage not found")
					return
				}
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, stage)
		case http.MethodDelete:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			if err := store.RemoveSdlcStage(r.Context(), db, stageID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "sdlc stage not found")
					return
				}
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	// Stage-role management: /api/sdlcs/{id}/stages/{stageId}/roles[/{roleId}]
	mux.HandleFunc("/api/sdlcs/stages/roles/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		// Parse: /api/sdlcs/stages/roles/{sdlcId}/{stageId}[/{roleId}]
		// This is a simplified routing — we use sdlcId/stageId/roleId in the path
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/sdlcs/stages/roles/")
		pathParts := strings.Split(trimmed, "/")
		if len(pathParts) < 2 {
			writeError(w, http.StatusBadRequest, "invalid path")
			return
		}
		var sdlcID, stageID int64
		if _, err := fmt.Sscan(pathParts[0], &sdlcID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid sdlc id")
			return
		}
		if _, err := fmt.Sscan(pathParts[1], &stageID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid stage id")
			return
		}
		switch r.Method {
		case http.MethodPost:
			var payload struct {
				RoleID int64 `json:"role_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.RoleID == 0 {
				writeError(w, http.StatusBadRequest, "role_id is required")
				return
			}
			if err := store.AddSdlcStageRole(r.Context(), db, sdlcID, stageID, payload.RoleID); err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]string{"status": "assigned"})
		case http.MethodDelete:
			if len(pathParts) < 3 {
				writeError(w, http.StatusBadRequest, "role id required")
				return
			}
			var roleID int64
			if _, err := fmt.Sscan(pathParts[2], &roleID); err != nil {
				writeError(w, http.StatusBadRequest, "invalid role id")
				return
			}
			if err := store.RemoveSdlcStageRole(r.Context(), db, sdlcID, stageID, roleID); err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
		case http.MethodPut:
			var payload struct {
				RoleIDs []int64 `json:"role_ids"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || len(payload.RoleIDs) == 0 {
				writeError(w, http.StatusBadRequest, "role_ids array is required")
				return
			}
			if err := store.ReorderSdlcStageRoles(r.Context(), db, sdlcID, stageID, payload.RoleIDs); err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "reordered"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/sdlcs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireUser(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			limit, err := queryInt(r, "limit", 0)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			offset, err := queryInt(r, "offset", 0)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			sdlcs, err := store.ListSdlcs(r.Context(), db, limit, offset)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, sdlcs)
		case http.MethodPost:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var payload sdlcRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			wf, err := store.CreateSdlc(r.Context(), db, payload.Name, payload.Description)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, wf)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/sdlcs/", func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/sdlcs/")
		parts := strings.Split(trimmed, "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		var wfID int64
		if _, err := fmt.Sscan(parts[0], &wfID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid sdlc id")
			return
		}
		// Sub-resource routing
		if len(parts) >= 2 {
			switch parts[1] {
			case "stages":
				if _, err := requireAdmin(db, r); err != nil {
					writeAuthError(w, err)
					return
				}
				if r.Method != http.MethodPost {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					return
				}
				var payload sdlcStageRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				stage, err := store.AddSdlcStage(r.Context(), db, wfID, payload.StageName, payload.Description, payload.AcceptanceCriteria, payload.SortOrder)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusCreated, stage)
				return
			case "reorder":
				if _, err := requireAdmin(db, r); err != nil {
					writeAuthError(w, err)
					return
				}
				if r.Method != http.MethodPut {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					return
				}
				var payload sdlcReorderRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				if err := store.ReorderSdlcStages(r.Context(), db, wfID, payload.StageIDs); err != nil {
					if errors.Is(err, store.ErrSdlcStageNotFound) {
						writeStoreError(w, err)
					} else {
						writeError(w, http.StatusInternalServerError, err.Error())
					}
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"status": "reordered"})
				return
			case "export":
				if _, err := requireUser(db, r); err != nil {
					writeAuthError(w, err)
					return
				}
				if r.Method != http.MethodGet {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					return
				}
				export, err := store.ExportSdlc(r.Context(), db, wfID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeError(w, http.StatusNotFound, "sdlc not found")
						return
					}
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, export)
				return
			}
		}
		// Direct sdlc resource — auth check moved here for non-sub-resource paths
		if _, err := requireUser(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		switch r.Method {
		case http.MethodGet:
			wf, err := store.GetSdlc(r.Context(), db, wfID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "sdlc not found")
					return
				}
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, wf)
		case http.MethodDelete:
			if err := store.DeleteSdlc(r.Context(), db, wfID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "sdlc not found")
					return
				}
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}
