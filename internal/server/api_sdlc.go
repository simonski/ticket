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
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, wf)
	})

	mux.HandleFunc("/api/sdlcs/stages/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/sdlcs/stages/")
		var stageID int64
		if _, err := fmt.Sscan(strings.TrimSpace(trimmed), &stageID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid stage id")
			return
		}
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := store.RemoveSdlcStage(r.Context(), db, stageID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "sdlc stage not found")
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	})

	mux.HandleFunc("/api/sdlcs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireUser(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			sdlcs, err := store.ListSdlcs(r.Context(), db)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
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
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, wf)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/sdlcs/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireUser(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
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
				if r.Method != http.MethodPost {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					return
				}
				var payload sdlcStageRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				stage, err := store.AddSdlcStage(r.Context(), db, wfID, payload.StageName, payload.Description, payload.SortOrder)
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusCreated, stage)
				return
			case "reorder":
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
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"status": "reordered"})
				return
			case "export":
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
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, export)
				return
			}
		}
		// Direct sdlc resource
		switch r.Method {
		case http.MethodGet:
			wf, err := store.GetSdlc(r.Context(), db, wfID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "sdlc not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, wf)
		case http.MethodDelete:
			if err := store.DeleteSdlc(r.Context(), db, wfID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "sdlc not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}
