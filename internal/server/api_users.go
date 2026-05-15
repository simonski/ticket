package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func (r *router) registerUserHandlers() {
	db := r.db
	mux := r.mux

	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			users, err := store.ListUsers(r.Context(), db, 0)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, users)
		case http.MethodPost:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var payload userCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			enabled := true
			if payload.Enabled != nil {
				enabled = *payload.Enabled
			}
			password := strings.TrimSpace(payload.Password)
			generatedPassword := ""
			var err error
			if password == "" {
				generatedPassword, err = store.GeneratePassword(24)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				password = generatedPassword
			}
			user, err := store.CreateUserWithParams(r.Context(), db, store.UserCreateParams{
				Username:      payload.Username,
				PlainPassword: password,
				Email:         payload.Email,
				Role:          payload.Role,
				Enabled:       enabled,
				PlanSlug:      payload.PlanSlug,
			})
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, registrationResponse{User: user, Password: generatedPassword})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}

		trimmed := strings.TrimPrefix(r.URL.Path, "/api/users/")
		parts := strings.Split(trimmed, "/")
		if r.Method == http.MethodDelete {
			if len(parts) != 1 || strings.TrimSpace(parts[0]) == "" {
				writeError(w, http.StatusNotFound, "not found")
				return
			}
			if err := store.DeleteUser(r.Context(), db, parts[0]); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "user not found")
					return
				}
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
			return
		}

		if len(parts) != 2 {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		username, action := parts[0], parts[1]
		var enabled bool
		switch action {
		case "enable":
			enabled = true
		case "disable":
			enabled = false
		case "plan":
			var payload userPlanAssignmentRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			user, err := store.GetUserByUsername(r.Context(), db, username)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "user not found")
					return
				}
				writeStoreError(w, err)
				return
			}
			var plan store.Plan
			switch {
			case payload.PlanID > 0:
				plan, err = store.GetPlanByID(r.Context(), db, payload.PlanID)
			case strings.TrimSpace(payload.PlanSlug) != "":
				plan, err = store.GetPlanBySlug(r.Context(), db, payload.PlanSlug)
			default:
				writeError(w, http.StatusBadRequest, "plan_id or plan_slug is required")
				return
			}
			if err != nil {
				writeStoreError(w, err)
				return
			}
			if err := store.AssignUserPlan(r.Context(), db, user.ID, plan.ID); err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, plan)
			return
		case "reset-password":
			var payload struct {
				Password string `json:"password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			user, err := store.ResetUserPassword(r.Context(), db, username, payload.Password)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "user not found")
					return
				}
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, user)
			return
		default:
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if err := store.SetUserEnabled(r.Context(), db, username, enabled); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": action + "d"})
	})
}
