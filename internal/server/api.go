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

func registerAPI(mux *http.ServeMux, db *sql.DB, version string, live *liveHub) {
	notify := func(eventType string, projectID, ticketID int64) {
		if live == nil {
			return
		}
		live.broadcast(liveEvent{
			Type:      eventType,
			ProjectID: projectID,
			TicketID:  ticketID,
		})
	}

	mux.HandleFunc("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			token = bearerToken(r)
		}
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if _, err := store.GetUserByToken(db, token); err != nil {
			writeAuthError(w, err)
			return
		}
		if err := websocketServe(live, w, r); err != nil {
			if strings.Contains(err.Error(), "websocket") || strings.Contains(err.Error(), "upgrade") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	})
	mux.HandleFunc("/api/chat/ws", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			token = bearerToken(r)
		}
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if _, err := store.GetUserByToken(db, token); err != nil {
			writeAuthError(w, err)
			return
		}
		if err := websocketServeChat(w, r); err != nil {
			if strings.Contains(err.Error(), "websocket") || strings.Contains(err.Error(), "upgrade") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	})
	mux.HandleFunc("/api/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var ping int
		if err := db.QueryRow("SELECT 1").Scan(&ping); err != nil {
			writeError(w, http.StatusInternalServerError, "database unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version})
	})

	mux.HandleFunc("/api/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		enabled, err := store.RegistrationEnabled(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !enabled {
			writeError(w, http.StatusForbidden, "registration is disabled")
			return
		}
		var credentials credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		user, err := store.RegisterUser(db, credentials.Username, credentials.Password)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, user)
	})

	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var credentials credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		user, err := store.AuthenticateUser(db, credentials.Username, credentials.Password)
		if err != nil {
			switch {
			case errors.Is(err, store.ErrInvalidCredentials):
				writeError(w, http.StatusUnauthorized, err.Error())
			case errors.Is(err, store.ErrForbidden):
				writeError(w, http.StatusForbidden, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		token, err := store.CreateSession(db, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "ticket_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   60 * 60 * 24 * 30,
		})
		writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
	})

	mux.HandleFunc("/api/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		token := bearerToken(r)
		if err := store.DeleteSession(db, token); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "ticket_token",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		registrationEnabled, regErr := store.RegistrationEnabled(db)
		if regErr != nil {
			writeError(w, http.StatusInternalServerError, regErr.Error())
			return
		}
		user, err := userFromRequest(db, r)
		if err != nil {
			if errors.Is(err, store.ErrUnauthorized) {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":               "ok",
					"authenticated":        false,
					"registration_enabled": registrationEnabled,
					"server_version":       version,
				})
				return
			}
			writeAuthError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":               "ok",
			"authenticated":        true,
			"registration_enabled": registrationEnabled,
			"server_version":       version,
			"user":                 user,
		})
	})

	mux.HandleFunc("/api/config/registration", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		var payload struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if err := store.SetRegistrationEnabled(db, payload.Enabled); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"registration_enabled": payload.Enabled,
		})
	})

	mux.HandleFunc("/api/count", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if _, err := requireUser(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		var projectID *int64
		if raw := strings.TrimSpace(r.URL.Query().Get("project_id")); raw != "" {
			var parsed int64
			if _, err := fmt.Sscan(raw, &parsed); err != nil {
				writeError(w, http.StatusBadRequest, "project_id must be numeric")
				return
			}
			projectID = &parsed
		}
		summary, err := store.CountEverything(db, projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, summary)
	})

	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			users, err := store.ListUsers(db)
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
			var credentials credentialsRequest
			if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			user, err := store.CreateUser(db, credentials.Username, credentials.Password, "user")
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, user)
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
			if err := store.DeleteUser(db, parts[0]); err != nil {
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
		default:
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if err := store.SetUserEnabled(db, username, enabled); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": action + "d"})
	})

	mux.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			agents, err := store.ListAgents(db)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, agents)
		case http.MethodPost:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var payload agentRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			agent, generatedPassword, err := store.CreateAgent(db, payload.Name, payload.Description, payload.Password)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{
				"agent":    agent,
				"password": generatedPassword,
			})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/agents/", func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/agents/")
		parts := strings.Split(trimmed, "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if parts[0] == "register" {
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			var payload agentAuthRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			agent, err := store.AuthenticateAgent(db, payload.Name, payload.Password)
			if err != nil {
				if errors.Is(err, store.ErrInvalidCredentials) || errors.Is(err, store.ErrForbidden) {
					writeAuthError(w, err)
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			agent, err = store.TouchAgent(db, agent.ID, "online")
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "agent": agent})
			return
		}
		if parts[0] == "request" {
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			var payload agentRequestWork
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			agent, err := store.AuthenticateAgent(db, payload.Name, payload.Password)
			if err != nil {
				if errors.Is(err, store.ErrInvalidCredentials) || errors.Is(err, store.ErrForbidden) {
					writeAuthError(w, err)
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			projectID := payload.ProjectID
			if payload.TicketID == nil && projectID == 0 {
				projects, err := store.ListProjects(db)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				for _, p := range projects {
					if p.Status == "open" {
						projectID = p.ID
						break
					}
				}
			}
			currentAssigned, hadCurrent, err := store.CurrentAssignedTicketForUser(db, projectID, agent.Name)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			ticket, status, err := store.RequestTicket(db, store.TicketRequestParams{
				ProjectID: projectID,
				TicketID:  payload.TicketID,
				Username:  agent.Name,
				UserID:    0,
				DryRun:    payload.DryRun,
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			agentStatus := "NONE"
			switch status {
			case "NO-WORK", "REJECTED":
				agentStatus = "NONE"
			case "ASSIGNED", "AVAILABLE":
				if hadCurrent && currentAssigned.ID == ticket.ID {
					agentStatus = "CURRENT"
				} else {
					agentStatus = "NEW"
				}
			default:
				agentStatus = status
			}
			if status == "ASSIGNED" && agentStatus == "NEW" {
				_, _ = store.TouchAgent(db, agent.ID, "working")
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
			} else {
				_, _ = store.TouchAgent(db, agent.ID, "soliciting")
			}
			response := map[string]any{
				"status":  agentStatus,
				"project": nil,
				"ticket":  nil,
				"parents": []store.Ticket{},
			}
			if agentStatus == "NEW" || agentStatus == "CURRENT" {
				response["ticket"] = ticket
				if project, err := store.GetProjectByID(db, ticket.ProjectID); err == nil {
					response["project"] = project
				}
				if parents, err := store.ListTicketParents(db, ticket.ID); err == nil {
					response["parents"] = parents
				}
			}
			writeJSON(w, http.StatusOK, response)
			return
		}
		if parts[0] == "tickets" && len(parts) == 3 && parts[2] == "update" {
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			var ticketID int64
			if _, err := fmt.Sscan(parts[1], &ticketID); err != nil {
				writeError(w, http.StatusBadRequest, "invalid ticket id")
				return
			}
			var payload agentTicketUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			agent, err := store.AuthenticateAgent(db, payload.Name, payload.Password)
			if err != nil {
				if errors.Is(err, store.ErrInvalidCredentials) || errors.Is(err, store.ErrForbidden) {
					writeAuthError(w, err)
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			current, err := store.GetTicket(db, ticketID)
			if err != nil {
				if errors.Is(err, store.ErrTicketNotFound) {
					writeError(w, http.StatusNotFound, "ticket not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			updated, err := store.UpdateTicket(db, ticketID, store.TicketUpdateParams{
				Title:              current.Title,
				Description:        payload.Result,
				AcceptanceCriteria: current.AcceptanceCriteria,
				GitRepository:      current.GitRepository,
				GitBranch:          current.GitBranch,
				ParentID:           current.ParentID,
				Assignee:           agent.Name,
				Stage:              store.StageDone,
				State:              store.StateSuccess,
				Priority:           current.Priority,
				Order:              current.Order,
				EstimateEffort:     current.EstimateEffort,
				EstimateComplete:   current.EstimateComplete,
				UpdatedBy:          0,
				ActorUsername:      agent.Name,
				ActorRole:          "admin",
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			_, _ = store.TouchAgent(db, agent.ID, "soliciting")
			notify("ticket_updated", updated.ProjectID, updated.ID)
			writeJSON(w, http.StatusOK, updated)
			return
		}

		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		var id int64
		if _, err := fmt.Sscan(parts[0], &id); err != nil {
			writeError(w, http.StatusBadRequest, "invalid agent id")
			return
		}
		if len(parts) == 1 {
			switch r.Method {
			case http.MethodPut:
				var payload agentRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				updated, err := store.UpdateAgent(db, id, store.AgentUpdateParams{
					Name:        nullableTrimmed(payload.Name),
					Description: nullableTrimmed(payload.Description),
					Password:    nullableTrimmed(payload.Password),
				})
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeError(w, http.StatusNotFound, "agent not found")
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, updated)
			case http.MethodDelete:
				if err := store.DeleteAgent(db, id); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeError(w, http.StatusNotFound, "agent not found")
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}
		if len(parts) == 2 && r.Method == http.MethodPost {
			var enabled bool
			switch parts[1] {
			case "enable":
				enabled = true
			case "disable":
				enabled = false
			default:
				writeError(w, http.StatusNotFound, "not found")
				return
			}
			updated, err := store.SetAgentEnabled(db, id, enabled)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "agent not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, updated)
			return
		}
		writeError(w, http.StatusNotFound, "not found")
	})

	mux.HandleFunc("/api/roles", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			roles, err := store.ListRoles(db)
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
			role, err := store.CreateRole(db, payload.Title, payload.Motivation, payload.Goals)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
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
			role, err := store.UpdateRole(db, id, payload.Title, payload.Motivation, payload.Goals)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "role not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, role)
		case http.MethodDelete:
			if err := store.DeleteRole(db, id); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "role not found")
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

	mux.HandleFunc("/api/projects", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireUser(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			projects, err := store.ListProjects(db)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, projects)
		case http.MethodPost:
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			var projectPayload projectRequest
			if err := json.NewDecoder(r.Body).Decode(&projectPayload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			project, err := store.CreateProjectWithParams(db, store.ProjectCreateParams{
				Prefix:             projectPayload.Prefix,
				Title:              projectPayload.Title,
				Description:        projectPayload.Description,
				AcceptanceCriteria: projectPayload.AcceptanceCriteria,
				GitRepository:      projectPayload.GitRepository,
				GitBranch:          projectPayload.GitBranch,
				Notes:              projectPayload.Notes,
				CreatedBy:          user.ID,
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			notify("project_created", project.ID, 0)
			writeJSON(w, http.StatusCreated, project)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/projects/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireUser(db, r); err != nil {
			writeAuthError(w, err)
			return
		}

		trimmed := strings.TrimPrefix(r.URL.Path, "/api/projects/")
		parts := strings.Split(trimmed, "/")
		if len(parts) == 2 && parts[1] == "tickets" && r.Method == http.MethodGet {
			project, err := store.GetProject(db, parts[0])
			if err != nil {
				if errors.Is(err, store.ErrProjectNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			role, err := projectRoleForUser(db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canReadProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			limit := 0
			if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
				if _, err := fmt.Sscan(raw, &limit); err != nil {
					writeError(w, http.StatusBadRequest, "limit must be numeric")
					return
				}
			}
			includeArchived := false
			if raw := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("include_archived"))); raw == "1" || raw == "true" || raw == "yes" {
				includeArchived = true
			}
			tasks, err := store.ListTickets(db, store.TicketListParams{
				ProjectID:       project.ID,
				Type:            r.URL.Query().Get("type"),
				Stage:           r.URL.Query().Get("stage"),
				State:           r.URL.Query().Get("state"),
				Status:          r.URL.Query().Get("status"),
				Search:          r.URL.Query().Get("q"),
				Assignee:        r.URL.Query().Get("assignee"),
				Limit:           limit,
				IncludeArchived: includeArchived,
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, tasks)
			return
		}
		if len(parts) == 2 && parts[1] == "stories" && r.Method == http.MethodGet {
			project, err := store.GetProject(db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			role, err := projectRoleForUser(db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canReadProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			stories, err := store.ListStoriesByProject(db, project.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, stories)
			return
		}

		if (len(parts) == 2 && parts[1] == "users") || (len(parts) == 3 && parts[1] == "users") {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			project, err := store.GetProject(db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			role, err := projectRoleForUser(db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canManageProjectUsers(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			switch r.Method {
			case http.MethodDelete:
				if len(parts) != 3 {
					writeError(w, http.StatusBadRequest, "usage: /api/projects/{id}/users/{user_id}")
					return
				}
				var userID int64
				if _, err := fmt.Sscan(parts[2], &userID); err != nil {
					writeError(w, http.StatusBadRequest, "user_id must be numeric")
					return
				}
				if err := store.RemoveProjectMember(db, project.ID, userID); err != nil {
					if errors.Is(err, store.ErrProjectMembershipNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("project_users_updated", project.ID, 0)
				writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
				return
			case http.MethodPost:
				if len(parts) != 2 {
					writeError(w, http.StatusBadRequest, "usage: /api/projects/{id}/users")
					return
				}
				var payload struct {
					UserID int64  `json:"user_id"`
					Role   string `json:"role"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				member, err := store.AddProjectMember(db, project.ID, payload.UserID, payload.Role)
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("project_users_updated", project.ID, 0)
				writeJSON(w, http.StatusOK, member)
				return
			case http.MethodGet:
				if len(parts) != 2 {
					writeError(w, http.StatusBadRequest, "usage: /api/projects/{id}/users")
					return
				}
				members, err := store.ListProjectMembers(db, project.ID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, members)
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}

		if len(parts) == 2 && r.Method == http.MethodPost {
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			project, err := store.GetProject(db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			var enabled bool
			switch parts[1] {
			case "enable":
				enabled = true
			case "disable":
				enabled = false
			default:
				writeError(w, http.StatusNotFound, "not found")
				return
			}
			project, err = store.SetProjectStatus(db, project.ID, enabled)
			if err != nil {
				if errors.Is(err, store.ErrProjectNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			notify("project_updated", project.ID, 0)
			writeJSON(w, http.StatusOK, project)
			return
		}

		if len(parts) != 1 {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		switch r.Method {
		case http.MethodGet:
			project, err := store.GetProject(db, parts[0])
			if err != nil {
				if errors.Is(err, store.ErrProjectNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, project)
		case http.MethodPut:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var projectPayload projectRequest
			if err := json.NewDecoder(r.Body).Decode(&projectPayload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			currentProject, err := store.GetProject(db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			project, err := store.UpdateProjectWithParams(db, currentProject.ID, store.ProjectUpdateParams{
				Title:              projectPayload.Title,
				Description:        projectPayload.Description,
				AcceptanceCriteria: projectPayload.AcceptanceCriteria,
				GitRepository:      projectPayload.GitRepository,
				GitBranch:          projectPayload.GitBranch,
				Notes:              projectPayload.Notes,
			})
			if err != nil {
				if errors.Is(err, store.ErrProjectNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			notify("project_updated", project.ID, 0)
			writeJSON(w, http.StatusOK, project)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	handleTicketsCollection := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		user, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		var ticketPayload ticketRequest
		if err := json.NewDecoder(r.Body).Decode(&ticketPayload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		stage, state, err := resolveLifecycleRequest(ticketPayload.Status, ticketPayload.Stage, ticketPayload.State)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		role, err := projectRoleForUser(db, ticketPayload.ProjectID, user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !canWriteProject(role) {
			writeAuthError(w, store.ErrForbidden)
			return
		}
		ticket, err := store.CreateTicket(db, store.TicketCreateParams{
			ProjectID:          ticketPayload.ProjectID,
			ParentID:           ticketPayload.ParentID,
			Type:               ticketPayload.Type,
			Title:              ticketPayload.Title,
			Description:        ticketPayload.Description,
			AcceptanceCriteria: ticketPayload.AcceptanceCriteria,
			GitRepository:      ticketPayload.GitRepository,
			GitBranch:          ticketPayload.GitBranch,
			Priority:           ticketPayload.Priority,
			EstimateEffort:     ticketPayload.EstimateEffort,
			EstimateComplete:   ticketPayload.EstimateComplete,
			Assignee:           ticketPayload.Assignee,
			Stage:              stage,
			State:              state,
			CreatedBy:          user.ID,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		notify("ticket_created", ticket.ProjectID, ticket.ID)
		writeJSON(w, http.StatusCreated, ticket)
	}
	mux.HandleFunc("/api/tickets", handleTicketsCollection)

	mux.HandleFunc("/api/stories", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			var payload storyRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			role, err := projectRoleForUser(db, payload.ProjectID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			story, err := store.CreateStory(db, payload.ProjectID, payload.Title, payload.Description, user.ID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, story)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/stories/", func(w http.ResponseWriter, r *http.Request) {
		user, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/stories/")
		parts := strings.Split(trimmed, "/")
		var storyID int64
		if _, err := fmt.Sscan(parts[0], &storyID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid story id")
			return
		}
		story, err := store.GetStory(db, storyID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "story not found")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		role, err := projectRoleForUser(db, story.ProjectID, user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !canWriteProject(role) {
			writeAuthError(w, store.ErrForbidden)
			return
		}
		if len(parts) == 1 && r.Method == http.MethodPut {
			var payload storyRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			updated, err := store.UpdateStory(db, story.ID, payload.Title, payload.Description)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "story not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, updated)
			return
		}
		if len(parts) != 2 || parts[1] != "analyse" || r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var analysis storyAnalysisResult
		prompt := fmt.Sprintf(
			"Story title: %s\nStory description: %s\nGenerate JSON shape {\"epics\":[{\"title\":\"...\",\"description\":\"...\",\"tasks\":[{\"title\":\"...\",\"description\":\"...\"}]}]} with 1-4 epics and 2-5 tasks per epic.",
			story.Title,
			story.Description,
		)
		if err := runRoleJSONAnalysis(db, "StoryReview", prompt, &analysis); err != nil || len(analysis.Epics) == 0 {
			analysis = fallbackStoryAnalysis(story)
		}

		createdEpics := 0
		createdTasks := 0
		for _, epicSpec := range analysis.Epics {
			epicTitle := strings.TrimSpace(epicSpec.Title)
			if epicTitle == "" {
				continue
			}
			epic, err := store.CreateTicket(db, store.TicketCreateParams{
				ProjectID:   story.ProjectID,
				Type:        "epic",
				Title:       epicTitle,
				Description: strings.TrimSpace(epicSpec.Description),
				CreatedBy:   user.ID,
				Stage:       store.StageDesign,
				State:       store.StateIdle,
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			_ = store.LinkStoryToTicket(db, story.ID, epic.ID)
			notify("ticket_created", epic.ProjectID, epic.ID)
			createdEpics++
			for _, taskSpec := range epicSpec.Tasks {
				taskTitle := strings.TrimSpace(taskSpec.Title)
				if taskTitle == "" {
					continue
				}
				parentID := epic.ID
				task, err := store.CreateTicket(db, store.TicketCreateParams{
					ProjectID:   story.ProjectID,
					ParentID:    &parentID,
					Type:        "task",
					Title:       taskTitle,
					Description: strings.TrimSpace(taskSpec.Description),
					CreatedBy:   user.ID,
					Stage:       store.StageDesign,
					State:       store.StateIdle,
				})
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				_ = store.LinkStoryToTicket(db, story.ID, task.ID)
				notify("ticket_created", task.ProjectID, task.ID)
				createdTasks++
			}
		}
		updatedStory, err := store.UpdateStoryStatus(db, story.ID, "ready_for_review")
		if err == nil {
			story = updatedStory
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"story":         story,
			"created_epics": createdEpics,
			"created_tasks": createdTasks,
		})
	})

	handleTicketClaim := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		user, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		var claimRequest ticketClaimRequest
		if err := json.NewDecoder(r.Body).Decode(&claimRequest); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		ticketID := claimRequest.TicketID
		ticketRef := strings.TrimSpace(claimRequest.TicketRef)
		if claimRequest.ProjectID != 0 {
			role, err := projectRoleForUser(db, claimRequest.ProjectID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
		}
		ticket, status, err := store.RequestTicket(db, store.TicketRequestParams{
			ProjectID: claimRequest.ProjectID,
			TicketID:  ticketID,
			TicketRef: ticketRef,
			Username:  user.Username,
			UserID:    user.ID,
			DryRun:    claimRequest.DryRun,
		})
		if err != nil {
			if errors.Is(err, store.ErrTicketNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		payload := map[string]any{"status": status}
		if status == "ASSIGNED" || status == "AVAILABLE" {
			payload["ticket"] = ticket
			if status == "ASSIGNED" {
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
			}
		}
		writeJSON(w, http.StatusOK, payload)
	}
	mux.HandleFunc("/api/tickets/claim", handleTicketClaim)

	handleTicketByRef := func(pathPrefix string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}

			trimmed := strings.TrimPrefix(r.URL.Path, pathPrefix)
			parts := strings.Split(trimmed, "/")
			ticketRef, err := store.GetTicketByRef(db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "ticket not found")
				return
			}
			id := ticketRef.ID
			role, err := projectRoleForUser(db, ticketRef.ProjectID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}

			if len(parts) == 2 && parts[1] == "history" && r.Method == http.MethodGet {
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				events, err := store.ListHistoryEvents(db, id)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, events)
				return
			}

			if len(parts) == 2 && parts[1] == "health" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var healthPayload ticketHealthRequest
				if err := json.NewDecoder(r.Body).Decode(&healthPayload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				ticket, err := store.SetTicketHealth(db, id, healthPayload.Score)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) || errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, "ticket not found")
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}

			if len(parts) == 2 && parts[1] == "comments" {
				switch r.Method {
				case http.MethodGet:
					if !canReadProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					comments, err := store.ListComments(db, id)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, comments)
				case http.MethodPost:
					if !canWriteProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					var commentPayload commentRequest
					if err := json.NewDecoder(r.Body).Decode(&commentPayload); err != nil {
						writeError(w, http.StatusBadRequest, "invalid json body")
						return
					}
					comment, err := store.AddComment(db, id, user.ID, commentPayload.Comment)
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					ticket, err := store.GetTicket(db, id)
					if err == nil {
						_ = store.AddHistoryEvent(db, ticket.ProjectID, id, "comment_added", map[string]any{
							"key":        ticket.Key,
							"comment_id": comment.ID,
						}, user.ID)
						notify("ticket_updated", ticket.ProjectID, ticket.ID)
					}
					writeJSON(w, http.StatusCreated, comment)
				default:
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			}

			if len(parts) == 2 && parts[1] == "dependencies" && r.Method == http.MethodGet {
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				dependencies, err := store.ListDependencies(db, id)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, dependencies)
				return
			}

			if len(parts) == 2 && parts[1] == "clone" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				cloned, err := store.CloneTicket(db, id, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("ticket_created", cloned.ProjectID, cloned.ID)
				writeJSON(w, http.StatusCreated, cloned)
				return
			}
			if len(parts) == 2 && parts[1] == "close" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				ticket, err := store.SetTicketOpen(db, id, false, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "open" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				ticket, err := store.SetTicketOpen(db, id, true, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "archive" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				ticket, err := store.SetTicketArchived(db, id, true, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "unarchive" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				ticket, err := store.SetTicketArchived(db, id, false, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "analyse" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				epic, err := store.GetTicket(db, id)
				if err != nil {
					writeError(w, http.StatusNotFound, "ticket not found")
					return
				}
				if epic.Type != "epic" {
					writeError(w, http.StatusBadRequest, "analyse is only supported for epics")
					return
				}
				storyID, ok, err := store.StoryIDForTicket(db, epic.ID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				if !ok {
					writeError(w, http.StatusBadRequest, "epic is not linked to a story")
					return
				}
				story, err := store.GetStory(db, storyID)
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}

				var analysis epicAnalysisResult
				prompt := fmt.Sprintf(
					"Story title: %s\nStory description: %s\nEpic title: %s\nEpic description: %s\nGenerate JSON shape {\"tickets\":[{\"title\":\"...\",\"description\":\"...\"}]} with 2-6 implementation tickets.",
					story.Title, story.Description, epic.Title, epic.Description,
				)
				if err := runRoleJSONAnalysis(db, "EpicReview", prompt, &analysis); err != nil || len(analysis.Tickets) == 0 {
					analysis = fallbackEpicAnalysis(epic)
				}

				created := 0
				for _, taskSpec := range analysis.Tickets {
					taskTitle := strings.TrimSpace(taskSpec.Title)
					if taskTitle == "" {
						continue
					}
					parentID := epic.ID
					task, err := store.CreateTicket(db, store.TicketCreateParams{
						ProjectID:   epic.ProjectID,
						ParentID:    &parentID,
						Type:        "task",
						Title:       taskTitle,
						Description: strings.TrimSpace(taskSpec.Description),
						CreatedBy:   user.ID,
						Stage:       store.StageDesign,
						State:       store.StateIdle,
					})
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					_ = store.LinkStoryToTicket(db, story.ID, task.ID)
					notify("ticket_created", task.ProjectID, task.ID)
					created++
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"epic_id":         epic.ID,
					"story_id":        story.ID,
					"created_tickets": created,
				})
				return
			}

			switch r.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				ticket, err := store.GetTicket(db, id)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, ticket)
			case http.MethodPut:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var ticketPayload ticketRequest
				if err := json.NewDecoder(r.Body).Decode(&ticketPayload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				currentTicket, err := store.GetTicket(db, id)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				ticketPayload = autoProgressTicketLifecycle(ticketPayload, currentTicket, user.Username)
				stage, state, err := resolveLifecycleRequest(ticketPayload.Status, ticketPayload.Stage, ticketPayload.State)
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				ticket, err := store.UpdateTicket(db, id, store.TicketUpdateParams{
					Title:              ticketPayload.Title,
					Description:        ticketPayload.Description,
					AcceptanceCriteria: ticketPayload.AcceptanceCriteria,
					GitRepository:      ticketPayload.GitRepository,
					GitBranch:          ticketPayload.GitBranch,
					ParentID:           ticketPayload.ParentID,
					Assignee:           ticketPayload.Assignee,
					Stage:              stage,
					State:              state,
					Priority:           ticketPayload.Priority,
					Order:              ticketPayload.Order,
					EstimateEffort:     ticketPayload.EstimateEffort,
					EstimateComplete:   ticketPayload.EstimateComplete,
					UpdatedBy:          user.ID,
					ActorUsername:      user.Username,
					ActorRole:          user.Role,
				})
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					if errors.Is(err, store.ErrAdminRequired) || errors.Is(err, store.ErrForbidden) {
						writeAuthError(w, err)
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
			case http.MethodDelete:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				if err := store.DeleteTicket(db, id); err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					if errors.Is(err, store.ErrTicketHasChildren) {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				notify("ticket_deleted", ticketRef.ProjectID, id)
				writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		}
	}
	mux.HandleFunc("/api/tickets/", handleTicketByRef("/api/tickets/"))

	mux.HandleFunc("/api/dependencies", func(w http.ResponseWriter, r *http.Request) {
		user, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var dependencyPayload dependencyRequest
			if err := json.NewDecoder(r.Body).Decode(&dependencyPayload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			role, err := projectRoleForUser(db, dependencyPayload.ProjectID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			dependency, err := store.AddDependency(db, dependencyPayload.ProjectID, dependencyPayload.TicketID, dependencyPayload.DependsOn, user.ID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			notify("ticket_updated", dependencyPayload.ProjectID, dependencyPayload.TicketID)
			writeJSON(w, http.StatusCreated, dependency)
		case http.MethodDelete:
			var projectID, ticketID, dependsOn int64
			if _, err := fmt.Sscan(strings.TrimSpace(r.URL.Query().Get("project_id")), &projectID); err != nil {
				writeError(w, http.StatusBadRequest, "project_id must be numeric")
				return
			}
			if _, err := fmt.Sscan(strings.TrimSpace(r.URL.Query().Get("ticket_id")), &ticketID); err != nil {
				writeError(w, http.StatusBadRequest, "ticket_id must be numeric")
				return
			}
			if _, err := fmt.Sscan(strings.TrimSpace(r.URL.Query().Get("depends_on")), &dependsOn); err != nil {
				writeError(w, http.StatusBadRequest, "depends_on must be numeric")
				return
			}
			role, err := projectRoleForUser(db, projectID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			if err := store.DeleteDependency(db, projectID, ticketID, dependsOn); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "dependency not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			notify("ticket_updated", projectID, ticketID)
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type agentRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Password    string `json:"password,omitempty"`
}

type agentAuthRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type agentRequestWork struct {
	Name      string `json:"name"`
	Password  string `json:"password"`
	ProjectID int64  `json:"project_id,omitempty"`
	TicketID  *int64 `json:"ticket_id,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

type agentTicketUpdateRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Result   string `json:"result"`
}

type projectRequest struct {
	Prefix             string `json:"prefix"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	GitRepository      string `json:"git_repository"`
	GitBranch          string `json:"git_branch"`
	Notes              string `json:"notes"`
}

type roleRequest struct {
	Title      string `json:"title"`
	Motivation string `json:"motivation"`
	Goals      string `json:"goals"`
}

type storyRequest struct {
	ProjectID   int64  `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ticketRequest struct {
	ProjectID          int64  `json:"project_id"`
	ParentID           *int64 `json:"parent_id"`
	Type               string `json:"type"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	GitRepository      string `json:"git_repository"`
	GitBranch          string `json:"git_branch"`
	Status             string `json:"status"`
	Stage              string `json:"stage"`
	State              string `json:"state"`
	Priority           int    `json:"priority"`
	Order              int    `json:"order"`
	EstimateEffort     int    `json:"estimate_effort"`
	EstimateComplete   string `json:"estimate_complete,omitempty"`
	Assignee           string `json:"assignee"`
}

type ticketHealthRequest struct {
	Score int `json:"score"`
}

type commentRequest struct {
	Comment string `json:"comment"`
}

type dependencyRequest struct {
	ProjectID int64 `json:"project_id"`
	TicketID  int64 `json:"ticket_id"`
	DependsOn int64 `json:"depends_on"`
}

type ticketClaimRequest struct {
	ProjectID int64  `json:"project_id"`
	TicketID  *int64 `json:"ticket_id,omitempty"`
	TicketRef string `json:"ticket_ref,omitempty"`
	DryRun    bool   `json:"dry_run"`
}

type authResponse struct {
	Token string     `json:"token"`
	User  store.User `json:"user"`
}

func resolveLifecycleRequest(status, stage, state string) (string, string, error) {
	if strings.TrimSpace(stage) != "" || strings.TrimSpace(state) != "" {
		return stage, state, nil
	}
	if strings.TrimSpace(status) == "" {
		return stage, state, nil
	}
	return store.ParseLifecycleStatus(status)
}

func autoProgressTicketLifecycle(payload ticketRequest, current store.Ticket, actorUsername string) ticketRequest {
	if hasExplicitLifecycleChange(payload, current) {
		return payload
	}
	if !hasMeaningfulTicketContentChange(payload, current) {
		return payload
	}
	nextAssignee := strings.TrimSpace(payload.Assignee)
	if nextAssignee == "" {
		nextAssignee = strings.TrimSpace(current.Assignee)
	}
	switch current.Stage {
	case store.StageDesign:
		payload.Stage = store.StageDevelop
		payload.State = store.StateActive
		if nextAssignee == "" {
			payload.Assignee = strings.TrimSpace(actorUsername)
		}
	case store.StageDevelop:
		payload.Stage = store.StageDevelop
		payload.State = store.StateActive
		if nextAssignee == "" {
			payload.Assignee = strings.TrimSpace(actorUsername)
		}
		if strings.TrimSpace(payload.EstimateComplete) != "" && strings.TrimSpace(payload.EstimateComplete) != strings.TrimSpace(current.EstimateComplete) {
			payload.Stage = store.StageTest
			payload.State = store.StateActive
		}
	case store.StageTest:
		payload.Stage = store.StageTest
		payload.State = store.StateActive
		if nextAssignee == "" {
			payload.Assignee = strings.TrimSpace(actorUsername)
		}
	}
	return payload
}

func hasExplicitLifecycleChange(payload ticketRequest, current store.Ticket) bool {
	if strings.TrimSpace(payload.Status) != "" {
		return true
	}
	stage := strings.TrimSpace(strings.ToLower(payload.Stage))
	state := strings.TrimSpace(strings.ToLower(payload.State))
	if stage == "" && state == "" {
		return false
	}
	return stage != current.Stage || state != current.State
}

func hasMeaningfulTicketContentChange(payload ticketRequest, current store.Ticket) bool {
	if payload.Title != current.Title {
		return true
	}
	if payload.Description != current.Description {
		return true
	}
	if payload.AcceptanceCriteria != current.AcceptanceCriteria {
		return true
	}
	if payload.Priority != current.Priority {
		return true
	}
	if payload.Order != current.Order {
		return true
	}
	if payload.EstimateEffort != current.EstimateEffort {
		return true
	}
	if strings.TrimSpace(payload.EstimateComplete) != strings.TrimSpace(current.EstimateComplete) {
		return true
	}
	if strings.TrimSpace(payload.Assignee) != strings.TrimSpace(current.Assignee) {
		return true
	}
	if (payload.ParentID == nil) != (current.ParentID == nil) {
		return true
	}
	if payload.ParentID != nil && current.ParentID != nil && *payload.ParentID != *current.ParentID {
		return true
	}
	return false
}

func nullableTrimmed(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func userFromRequest(db *sql.DB, r *http.Request) (store.User, error) {
	return store.GetUserByToken(db, bearerToken(r))
}

func requireUser(db *sql.DB, r *http.Request) (store.User, error) {
	return userFromRequest(db, r)
}

func requireAdmin(db *sql.DB, r *http.Request) (store.User, error) {
	user, err := requireUser(db, r)
	if err != nil {
		return store.User{}, err
	}
	if user.Role != "admin" {
		return store.User{}, store.ErrAdminRequired
	}
	return user, nil
}

func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	cookie, err := r.Cookie("ticket_token")
	if err == nil && strings.TrimSpace(cookie.Value) != "" {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
}

func projectRoleForUser(db *sql.DB, projectID int64, user store.User) (string, error) {
	if user.Role == "admin" {
		return store.ProjectRoleOwner, nil
	}
	role, ok, err := store.ProjectRoleForUser(db, projectID, user.ID)
	if err != nil {
		return "", err
	}
	if ok {
		return role, nil
	}
	// Legacy behavior: authenticated users can edit if no explicit project role exists.
	return store.ProjectRoleEditor, nil
}

func canReadProject(role string) bool {
	switch role {
	case store.ProjectRoleViewer, store.ProjectRoleEditor, store.ProjectRoleOwner:
		return true
	default:
		return false
	}
}

func canWriteProject(role string) bool {
	switch role {
	case store.ProjectRoleEditor, store.ProjectRoleOwner:
		return true
	default:
		return false
	}
}

func canManageProjectUsers(role string) bool {
	return role == store.ProjectRoleOwner
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, store.ErrAdminRequired):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
