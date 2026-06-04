package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
		if r.Method != http.MethodPost && r.Method != http.MethodDelete && r.Method != http.MethodGet && r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		trimmed := strings.TrimPrefix(r.URL.Path, "/api/users/")
		parts := strings.Split(trimmed, "/")
		if r.Method == http.MethodGet {
			if len(parts) == 3 && parts[0] == "me" && parts[1] == "default-project" {
				user, err := requireUser(db, r)
				if err != nil {
					writeAuthError(w, err)
					return
				}
				project, role, err := resolveProjectPathForUser(r.Context(), db, user, parts[2], false)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				if !canReadProject(role) {
					writeStoreError(w, store.ErrUnauthorized)
					return
				}
				writeJSON(w, http.StatusOK, project)
				return
			}
			if len(parts) == 2 && parts[0] == "me" && parts[1] == "default-project" {
				user, err := requireUser(db, r)
				if err != nil {
					writeAuthError(w, err)
					return
				}
				project, err := store.GetUserDefaultProject(r.Context(), db, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrProjectNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeStoreError(w, err)
					return
				}
				role, err := projectRoleForUser(r.Context(), db, project.ID, user)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				if !canReadProject(role) {
					writeStoreError(w, store.ErrProjectNotFound)
					return
				}
				writeJSON(w, http.StatusOK, project)
				return
			}
			if len(parts) == 2 && parts[0] == "me" && parts[1] == "access-requests" {
				user, err := requireUser(db, r)
				if err != nil {
					writeAuthError(w, err)
					return
				}
				requests, err := store.ListUserProjectAccessRequests(r.Context(), db, user.ID, strings.TrimSpace(r.URL.Query().Get("status")))
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, requests)
				return
			}
			if len(parts) == 2 && parts[0] == "me" && parts[1] == "notifications" {
				user, err := requireUser(db, r)
				if err != nil {
					writeAuthError(w, err)
					return
				}
				limit := 20
				if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
					if _, scanErr := fmt.Sscan(raw, &limit); scanErr != nil {
						writeError(w, http.StatusBadRequest, "limit must be numeric")
						return
					}
				}
				notifications, err := store.ListUserNotifications(r.Context(), db, user.ID, strings.TrimSpace(r.URL.Query().Get("status")), limit)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, notifications)
				return
			}
			if len(parts) == 2 && parts[0] == "me" && parts[1] == "passkeys" {
				user, err := requireUser(db, r)
				if err != nil {
					writeAuthError(w, err)
					return
				}
				credentials, err := store.ListPasskeyCredentials(r.Context(), db, user.ID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				response := make([]passkeyCredentialResponse, 0, len(credentials))
				for _, credential := range credentials {
					response = append(response, newPasskeyCredentialResponse(credential))
				}
				writeJSON(w, http.StatusOK, response)
				return
			}
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		if r.Method == http.MethodPut && len(parts) == 2 && parts[0] == "me" && parts[1] == "default-project" {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			var payload struct {
				ProjectRef string `json:"project_ref"`
			}
			decodeErr := json.NewDecoder(r.Body).Decode(&payload)
			if decodeErr != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			project, role, err := resolveProjectPathForUser(r.Context(), db, user, payload.ProjectRef, false)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			if !canReadProject(role) {
				writeStoreError(w, store.ErrUnauthorized)
				return
			}
			setErr := store.SetUserDefaultProject(r.Context(), db, user.ID, project.ID)
			if setErr != nil {
				writeStoreError(w, setErr)
				return
			}
			refreshed, err := store.GetUserDefaultProject(r.Context(), db, user.ID)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, refreshed)
			return
		}

		if r.Method == http.MethodPut && len(parts) == 3 && parts[0] == "me" && parts[1] == "passkeys" {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			credentialID, err := url.PathUnescape(strings.TrimSpace(parts[2]))
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid credential id")
				return
			}
			var payload struct {
				Name string `json:"name"`
			}
			if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			name := strings.TrimSpace(payload.Name)
			if name == "" {
				writeError(w, http.StatusBadRequest, "passkey name is required")
				return
			}
			if renameErr := store.RenamePasskeyCredential(r.Context(), db, user.ID, credentialID, name); renameErr != nil {
				if errors.Is(renameErr, store.ErrPasskeyNotFound) {
					writeError(w, http.StatusNotFound, renameErr.Error())
					return
				}
				writeStoreError(w, renameErr)
				return
			}
			credentials, err := store.ListPasskeyCredentials(r.Context(), db, user.ID)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			for _, credential := range credentials {
				if credential.CredentialID == credentialID {
					writeJSON(w, http.StatusOK, newPasskeyCredentialResponse(credential))
					return
				}
			}
			writeError(w, http.StatusNotFound, store.ErrPasskeyNotFound.Error())
			return
		}

		if r.Method == http.MethodPost && len(parts) == 4 && parts[0] == "me" && parts[1] == "notifications" && parts[3] == "read" {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			var notificationID int64
			if _, scanErr := fmt.Sscan(strings.TrimSpace(parts[2]), &notificationID); scanErr != nil {
				writeError(w, http.StatusBadRequest, "notification id must be numeric")
				return
			}
			notification, err := store.MarkUserNotificationRead(r.Context(), db, notificationID, user.ID)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, notification)
			return
		}

		if r.Method == http.MethodDelete && len(parts) == 2 && parts[0] == "me" && parts[1] == "default-project" {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			if err := store.ClearUserDefaultProject(r.Context(), db, user.ID); err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
			return
		}

		if r.Method == http.MethodDelete && len(parts) == 3 && parts[0] == "me" && parts[1] == "passkeys" {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			credentialID, err := url.PathUnescape(strings.TrimSpace(parts[2]))
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid credential id")
				return
			}
			if err := store.DeletePasskeyCredential(r.Context(), db, user.ID, credentialID); err != nil {
				if errors.Is(err, store.ErrPasskeyNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
			return
		}

		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
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
