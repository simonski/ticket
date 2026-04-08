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

func (r *router) registerWorkflowHandlers() {
	db := r.db
	mux := r.mux

	// Workflow endpoints
	mux.HandleFunc("/api/workflows/import", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var export store.WorkflowExport
		if err := json.NewDecoder(r.Body).Decode(&export); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		wf, err := store.ImportWorkflow(r.Context(), db, export)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, wf)
	})

	mux.HandleFunc("/api/workflows/stages/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/workflows/stages/")
		var stageID int64
		if _, err := fmt.Sscan(strings.TrimSpace(trimmed), &stageID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid stage id")
			return
		}
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := store.RemoveWorkflowStage(r.Context(), db, stageID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "workflow stage not found")
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	})

	mux.HandleFunc("/api/workflows", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireUser(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			workflows, err := store.ListWorkflows(r.Context(), db)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, workflows)
		case http.MethodPost:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var payload workflowRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			wf, err := store.CreateWorkflow(r.Context(), db, payload.Name, payload.Description)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, wf)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/workflows/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireUser(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/workflows/")
		parts := strings.Split(trimmed, "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		var wfID int64
		if _, err := fmt.Sscan(parts[0], &wfID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid workflow id")
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
				var payload workflowStageRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				stage, err := store.AddWorkflowStage(r.Context(), db, wfID, payload.StageName, payload.Description, payload.RoleID, payload.SortOrder)
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
				var payload workflowReorderRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				if err := store.ReorderWorkflowStages(r.Context(), db, wfID, payload.StageIDs); err != nil {
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
				export, err := store.ExportWorkflow(r.Context(), db, wfID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeError(w, http.StatusNotFound, "workflow not found")
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, export)
				return
			}
		}
		// Direct workflow resource
		switch r.Method {
		case http.MethodGet:
			wf, err := store.GetWorkflow(r.Context(), db, wfID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "workflow not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, wf)
		case http.MethodDelete:
			if err := store.DeleteWorkflow(r.Context(), db, wfID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "workflow not found")
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
