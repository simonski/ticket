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
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, wf)
	})

	mux.HandleFunc("/api/workflows/stages/", func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/workflows/stages/")
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
			stage, err := store.GetWorkflowStage(r.Context(), db, stageID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "workflow stage not found")
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
			var payload workflowStageRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			wow := strings.TrimSpace(payload.WaysOfWorking)
			if wow == "" {
				wow = payload.Description
			}
			dor := strings.TrimSpace(payload.DefinitionOfReady)
			if dor == "" {
				dor = payload.AcceptanceCriteria
			}
			stage, err := store.UpdateWorkflowStageWithDefinitions(r.Context(), db, stageID, payload.StageName, wow, dor, payload.DefinitionOfDone)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "workflow stage not found")
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
			if err := store.RemoveWorkflowStage(r.Context(), db, stageID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "workflow stage not found")
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

	// Stage-role management: /api/workflows/{id}/stages/{stageId}/roles[/{roleId}]
	mux.HandleFunc("/api/workflows/stages/roles/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		// Parse: /api/workflows/stages/roles/{workflowId}/{stageId}[/{roleId}]
		// This is a simplified routing — we use workflowId/stageId/roleId in the path
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/workflows/stages/roles/")
		pathParts := strings.Split(trimmed, "/")
		if len(pathParts) < 2 {
			writeError(w, http.StatusBadRequest, "invalid path")
			return
		}
		var workflowID, stageID int64
		if _, err := fmt.Sscan(pathParts[0], &workflowID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid workflow id")
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
			if err := store.AddWorkflowStageRole(r.Context(), db, workflowID, stageID, payload.RoleID); err != nil {
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
			if err := store.RemoveWorkflowStageRole(r.Context(), db, workflowID, stageID, roleID); err != nil {
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
			if err := store.ReorderWorkflowStageRoles(r.Context(), db, workflowID, stageID, payload.RoleIDs); err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "reordered"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/workflows", func(w http.ResponseWriter, r *http.Request) {
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
			workflows, err := store.ListWorkflows(r.Context(), db, limit, offset)
			if err != nil {
				writeStoreError(w, err)
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
			wf, err := store.CreateWorkflowWithOptions(
				r.Context(),
				db,
				payload.ID,
				payload.Name,
				payload.Description,
				payload.ApprovalPolicy,
				payload.ProgressionMode,
			)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, wf)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/workflows/", func(w http.ResponseWriter, r *http.Request) {
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
				if _, err := requireAdmin(db, r); err != nil {
					writeAuthError(w, err)
					return
				}
				if r.Method != http.MethodPost {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					return
				}
				var payload workflowStageRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				wow := strings.TrimSpace(payload.WaysOfWorking)
				if wow == "" {
					wow = payload.Description
				}
				dor := strings.TrimSpace(payload.DefinitionOfReady)
				if dor == "" {
					dor = payload.AcceptanceCriteria
				}
				stage, err := store.AddWorkflowStageWithDefinitions(r.Context(), db, wfID, payload.StageName, wow, dor, payload.DefinitionOfDone, payload.SortOrder)
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
				var payload workflowReorderRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				if err := store.ReorderWorkflowStages(r.Context(), db, wfID, payload.StageIDs); err != nil {
					if errors.Is(err, store.ErrWorkflowStageNotFound) {
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
				export, err := store.ExportWorkflow(r.Context(), db, wfID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeError(w, http.StatusNotFound, "workflow not found")
						return
					}
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, export)
				return
			}
		}
		// Direct workflow resource — auth check moved here for non-sub-resource paths
		if _, err := requireUser(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		switch r.Method {
		case http.MethodGet:
			wf, err := store.GetWorkflow(r.Context(), db, wfID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "workflow not found")
					return
				}
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, wf)
		case http.MethodPut:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var payload workflowRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			wf, err := store.UpdateWorkflow(
				r.Context(),
				db,
				wfID,
				payload.Name,
				payload.Description,
				payload.ApprovalPolicy,
				payload.ProgressionMode,
			)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "workflow not found")
					return
				}
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, wf)
		case http.MethodDelete:
			if err := store.DeleteWorkflow(r.Context(), db, wfID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "workflow not found")
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
