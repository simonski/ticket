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

func (r *router) registerRoleHandlers() {
	db := r.db
	mux := r.mux

	mux.HandleFunc("/api/roles", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			roles, err := store.ListRoles(r.Context(), db, 0)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, roles)
		case http.MethodPost:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var payload roleRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			role, err := store.CreateRoleWithParams(r.Context(), db, store.RoleCreateParams{
				ID:                 payload.ID,
				WorkflowID:         payload.WorkflowID,
				Title:              payload.Title,
				Description:        payload.Description,
				AcceptanceCriteria: payload.AcceptanceCriteria,
				DORMap:             payload.DORMap,
				DODMap:             payload.DODMap,
				ACMap:              payload.ACMap,
			})
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, role)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/roles/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/roles/")
		var id int64
		if _, err := fmt.Sscan(strings.TrimSpace(trimmed), &id); err != nil {
			writeError(w, http.StatusBadRequest, "invalid role id")
			return
		}
		switch r.Method {
		case http.MethodPut:
			var payload roleRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			role, err := store.UpdateRoleWithParams(r.Context(), db, id, store.RoleUpdateParams{
				Title:              payload.Title,
				Description:        payload.Description,
				AcceptanceCriteria: payload.AcceptanceCriteria,
				DORMap:             payload.DORMap,
				DODMap:             payload.DODMap,
				ACMap:              payload.ACMap,
			})
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "role not found")
					return
				}
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, role)
		case http.MethodDelete:
			if err := store.DeleteRole(r.Context(), db, id); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "role not found")
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
