package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/simonski/ticket/internal/store"
)

func registerAPI(mux *http.ServeMux, db *sql.DB, version string, live *liveHub, verbose bool, output io.Writer) {
	authLimiter := newRateLimiter(10, 1*time.Minute)
	var chatLog func(string)
	if verbose {
		if output == nil {
			output = io.Discard
		}
		var chatLogMu sync.Mutex
		chatLog = func(line string) {
			chatLogMu.Lock()
			defer chatLogMu.Unlock()
			fmt.Fprintf(output, "CHAT %s\n", strings.TrimRight(line, "\n"))
		}
	}

	notify := func(eventType string, projectID int64, ticketID string) {
		if live == nil {
			return
		}
		live.broadcast(buildLiveChangeEvent(eventType, projectID, ticketID))
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
		chatEnabled, err := store.ChatEnabled(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !chatEnabled {
			writeError(w, http.StatusForbidden, "chat is disabled")
			return
		}
		if err := websocketServeChat(w, r, db, chatLog); err != nil {
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
		if !authLimiter.allow(clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many requests")
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
		if !authLimiter.allow(clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many requests")
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
			Secure:   r.TLS != nil,
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
			Secure:   r.TLS != nil,
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
		chatLimits, chatErr := store.ChatLimitsConfig(db)
		if chatErr != nil {
			writeError(w, http.StatusInternalServerError, chatErr.Error())
			return
		}
		chatEnabled, chatEnabledErr := store.ChatEnabled(db)
		if chatEnabledErr != nil {
			writeError(w, http.StatusInternalServerError, chatEnabledErr.Error())
			return
		}
		runningChats := sharedChatRuntime.runningProcessCount()
		user, err := userFromRequest(db, r)
		if err != nil {
			if errors.Is(err, store.ErrUnauthorized) {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":                    "ok",
					"authenticated":             false,
					"registration_enabled":      registrationEnabled,
					"chat_enabled":              chatEnabled,
					"chat_max_connections":      chatLimits.MaxConnections,
					"chat_max_duration_minutes": chatLimits.MaxDurationMin,
					"chat_running_processes":    runningChats,
					"server_version":            version,
				})
				return
			}
			writeAuthError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                    "ok",
			"authenticated":             true,
			"registration_enabled":      registrationEnabled,
			"chat_enabled":              chatEnabled,
			"chat_max_connections":      chatLimits.MaxConnections,
			"chat_max_duration_minutes": chatLimits.MaxDurationMin,
			"chat_running_processes":    runningChats,
			"server_version":            version,
			"user":                      user,
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
	mux.HandleFunc("/api/config/chat_limits", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		var payload struct {
			MaxConnections int `json:"max_connections"`
			MaxDurationMin int `json:"max_duration_minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if payload.MaxConnections <= 0 {
			payload.MaxConnections = store.DefaultChatMaxConnections
		}
		if payload.MaxDurationMin <= 0 {
			payload.MaxDurationMin = store.DefaultChatMaxDurationMinutes
		}
		if err := store.SetChatLimitsConfig(db, payload.MaxConnections, payload.MaxDurationMin); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"chat_max_connections":      payload.MaxConnections,
			"chat_max_duration_minutes": payload.MaxDurationMin,
		})
	})
	mux.HandleFunc("/api/config/chat_enabled", func(w http.ResponseWriter, r *http.Request) {
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
		if err := store.SetChatEnabled(db, payload.Enabled); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"chat_enabled": payload.Enabled,
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
		case "reset-password":
			var payload struct {
				Password string `json:"password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			user, err := store.ResetUserPassword(db, username, payload.Password)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "user not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, user)
			return
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
			agent, generatedPassword, err := store.CreateAgent(db, payload.Password)
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
		if parts[0] == "statuses" {
			if r.Method != http.MethodGet {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			statuses, err := store.ListAgentStatuses(db)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, statuses)
			return
		}
		if parts[0] == "register" {
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			agentID, agentPass, ok := r.BasicAuth()
			if !ok || agentID == "" || agentPass == "" {
				writeError(w, http.StatusUnauthorized, "basic auth required")
				return
			}
			agent, err := store.AuthenticateAgent(db, agentID, agentPass)
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
		if parts[0] == "heartbeat" {
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			agentID, agentPass, ok := r.BasicAuth()
			if !ok || agentID == "" || agentPass == "" {
				writeError(w, http.StatusUnauthorized, "basic auth required")
				return
			}
			agent, err := store.AuthenticateAgent(db, agentID, agentPass)
			if err != nil {
				if errors.Is(err, store.ErrInvalidCredentials) || errors.Is(err, store.ErrForbidden) {
					writeAuthError(w, err)
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			var payload struct {
				Status string `json:"status"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			status := strings.TrimSpace(payload.Status)
			if status == "" {
				status = agent.Status // keep current status
			}
			agent, err = store.TouchAgent(db, agent.ID, status)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
			return
		}
		if parts[0] == "request" {
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			agentID, agentPass, ok := r.BasicAuth()
			if !ok || agentID == "" || agentPass == "" {
				writeError(w, http.StatusUnauthorized, "basic auth required")
				return
			}
			var payload struct {
				ProjectID       int64   `json:"project_id,omitempty"`
				TicketID        *string `json:"ticket_id,omitempty"`
				DryRun          bool   `json:"dry_run,omitempty"`
				ConfigUpdatedAt string `json:"config_updated_at,omitempty"`
			}
			// Body is optional — may be empty for simple requests.
			_ = json.NewDecoder(r.Body).Decode(&payload)
			vlog := func(format string, args ...any) {
				if verbose && output != nil {
					fmt.Fprintf(output, "AGENT %s\n", fmt.Sprintf(format, args...))
				}
			}
			vlog("request from agent=%q project_id=%d", agentID, payload.ProjectID)
			agent, err := store.AuthenticateAgent(db, agentID, agentPass)
			if err != nil {
				vlog("auth failed for agent=%q: %v", agentID, err)
				if errors.Is(err, store.ErrInvalidCredentials) || errors.Is(err, store.ErrForbidden) {
					writeAuthError(w, err)
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			vlog("agent=%q authenticated (id=%s)", agent.Username, agent.ID)
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
						vlog("auto-selected project=%d %q (first open)", p.ID, p.Title)
						break
					}
				}
				if projectID == 0 {
					vlog("no open projects found")
				}
			}
			currentAssigned, hadCurrent, err := store.CurrentAssignedTicketForUser(db, projectID, agent.Username)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if hadCurrent {
				vlog("agent has current assignment: %s %q (status=%s)", currentAssigned.ID, currentAssigned.Title, currentAssigned.Status)
			} else {
				vlog("agent has no current assignment")
			}
			ticket, status, err := store.RequestTicket(db, store.TicketRequestParams{
				ProjectID: projectID,
				TicketID:  payload.TicketID,
				Username:  agent.Username,
				UserID:    "",
				DryRun:    payload.DryRun,
			})
			if err != nil {
				vlog("RequestTicket error: %v", err)
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			vlog("RequestTicket result: status=%s ticket_id=%s", status, ticket.ID)
			if status == "NO-WORK" {
				// Explain why no work was found.
				vlog("explaining NO-WORK decision for project=%d:", projectID)
				if reasons, err := store.ExplainNoWork(db, projectID, agent.Username); err == nil {
					for _, reason := range reasons {
						vlog("  %s", reason)
					}
				}
			}
			agentStatus := "NONE"
			switch status {
			case "NO-WORK", "REJECTED":
				agentStatus = "NONE"
				if status == "REJECTED" {
					vlog("ticket rejected: not claimable (wrong stage, already assigned, or has children)")
				}
			case "ASSIGNED", "AVAILABLE":
				if hadCurrent && currentAssigned.ID == ticket.ID {
					agentStatus = "CURRENT"
					vlog("returning current assignment %s", ticket.ID)
				} else {
					agentStatus = "NEW"
					vlog("assigned new ticket %s %q to agent", ticket.ID, ticket.Title)
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
			// Check if config should be included in response.
			// Include config if: 1) it has changed since last poll, or 2) a new ticket is assigned
			var configMap map[string]string
			var configUpdatedAt string
			currentConfigUpdatedAt, err := store.GetAgentConfigUpdatedAt(db, agent.ID)
			if err == nil {
				configHasChanged := payload.ConfigUpdatedAt != currentConfigUpdatedAt
				includeConfig := configHasChanged || agentStatus == "NEW"
				if includeConfig {
					configMap, err = store.GetAgentConfigMap(db, agent.ID)
					if err == nil && len(configMap) > 0 {
						configUpdatedAt = currentConfigUpdatedAt
						vlog("including config in response (changed=%v, new_ticket=%v)", configHasChanged, agentStatus == "NEW")
					}
				}
			}
			response := map[string]any{
				"status":            agentStatus,
				"project":           nil,
				"ticket":            nil,
				"parents":           []store.Ticket{},
				"workflow":          nil,
				"role":              nil,
				"config":            configMap,
				"config_updated_at": configUpdatedAt,
			}
			if agentStatus == "NEW" || agentStatus == "CURRENT" {
				response["ticket"] = ticket
				ctx := store.EnrichTicketContext(db, ticket)
				response["project"] = ctx.Project
				response["parents"] = ctx.Parents
				response["workflow"] = ctx.Workflow
				response["role"] = ctx.Role
			}
			vlog("response: agent_status=%s", agentStatus)
			writeJSON(w, http.StatusOK, response)
			return
		}
		if parts[0] == "tickets" && len(parts) == 3 && parts[2] == "update" {
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			agentID, agentPass, ok := r.BasicAuth()
			if !ok || agentID == "" || agentPass == "" {
				writeError(w, http.StatusUnauthorized, "basic auth required")
				return
			}
			ticketID := strings.TrimSpace(parts[1])
			if ticketID == "" {
				writeError(w, http.StatusBadRequest, "invalid ticket id")
				return
			}
			var payload struct {
				Result string `json:"result"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			agent, err := store.AuthenticateAgent(db, agentID, agentPass)
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
				Description:        current.Description,
				AcceptanceCriteria: current.AcceptanceCriteria,
				GitRepository:      current.GitRepository,
				GitBranch:          current.GitBranch,
				ParentID:           current.ParentID,
				Assignee:           agent.Username,
				State:              store.StateSuccess,
				Priority:           current.Priority,
				Order:              current.Order,
				EstimateEffort:     current.EstimateEffort,
				EstimateComplete:   current.EstimateComplete,
				UpdatedBy:          "",
				ActorUsername:      agent.Username,
				ActorRole:          "admin",
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			_ = store.AddHistoryEvent(db, updated.ProjectID, updated.ID, "agent_completed", map[string]any{
				"key":       updated.ID,
				"agent":     agent.Username,
				"result":    payload.Result,
				"new_stage": updated.Stage,
				"new_state": updated.State,
			}, agent.ID)
			_, _ = store.TouchAgent(db, agent.ID, "soliciting")
			notify("ticket_updated", updated.ProjectID, updated.ID)
			writeJSON(w, http.StatusOK, updated)
			return
		}

		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		id := strings.TrimSpace(parts[0])
		if id == "" {
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
					Password: nullableTrimmed(payload.Password),
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
		if (len(parts) == 2 || len(parts) == 3) && parts[1] == "config" {
			switch r.Method {
			case http.MethodGet:
				entries, err := store.ListAgentConfig(db, id)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, entries)
			case http.MethodPost:
				var payload struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				if err := store.SetAgentConfig(db, id, payload.Key, payload.Value); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			case http.MethodDelete:
				if len(parts) != 3 {
					writeError(w, http.StatusBadRequest, "usage: /api/agents/{id}/config/{key}")
					return
				}
				if err := store.DeleteAgentConfig(db, id, parts[2]); err != nil {
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
		wf, err := store.ImportWorkflow(db, export)
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
		if err := store.RemoveWorkflowStage(db, stageID); err != nil {
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
			workflows, err := store.ListWorkflows(db)
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
			wf, err := store.CreateWorkflow(db, payload.Name, payload.Description)
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
				stage, err := store.AddWorkflowStage(db, wfID, payload.StageName, payload.Description, payload.RoleID, payload.SortOrder)
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
				if err := store.ReorderWorkflowStages(db, wfID, payload.StageIDs); err != nil {
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
				export, err := store.ExportWorkflow(db, wfID)
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
			wf, err := store.GetWorkflow(db, wfID)
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
			if err := store.DeleteWorkflow(db, wfID); err != nil {
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

	mux.HandleFunc("/api/teams", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireUser(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			teams, err := store.ListTeams(db)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, teams)
		case http.MethodPost:
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			if user.Role != "admin" {
				writeAuthError(w, store.ErrAdminRequired)
				return
			}
			var payload teamRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			team, err := store.CreateTeam(db, payload.Name, payload.ParentTeamID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			// The creator is always an owner of the new team.
			if _, err := store.AddTeamMember(db, team.ID, user.ID, store.TeamRoleOwner, ""); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, team)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/teams/", func(w http.ResponseWriter, r *http.Request) {
		user, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/teams/")
		parts := strings.Split(trimmed, "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		var teamID int64
		if _, err := fmt.Sscan(parts[0], &teamID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid team id")
			return
		}
		team, err := store.GetTeamByID(db, teamID)
		if err != nil {
			if errors.Is(err, store.ErrTeamNotFound) {
				writeError(w, http.StatusNotFound, "team not found")
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		canManageTeam := user.Role == "admin"
		if !canManageTeam {
			role, ok, err := store.TeamRoleForUser(db, team.ID, user.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			canManageTeam = ok && role == store.TeamRoleOwner
		}

		if len(parts) == 1 {
			switch r.Method {
			case http.MethodPut:
				if !canManageTeam {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload teamRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				updated, err := store.UpdateTeam(db, team.ID, payload.Name, payload.ParentTeamID)
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, updated)
			case http.MethodDelete:
				if !canManageTeam {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				if err := store.DeleteTeam(db, team.ID); err != nil {
					if errors.Is(err, store.ErrTeamNotFound) {
						writeError(w, http.StatusNotFound, "team not found")
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

		if len(parts) == 2 && parts[1] == "users" {
			switch r.Method {
			case http.MethodGet:
				if !canManageTeam {
					_, ok, err := store.TeamRoleForUser(db, team.ID, user.ID)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					if !ok {
						writeAuthError(w, store.ErrForbidden)
						return
					}
				}
				members, err := store.ListTeamMembers(db, team.ID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, members)
			case http.MethodPost:
				if !canManageTeam {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload teamMemberRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				member, err := store.AddTeamMember(db, team.ID, payload.UserID, payload.Role, payload.JobTitle)
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, member)
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}

		if len(parts) == 3 && parts[1] == "users" && r.Method == http.MethodDelete {
			if !canManageTeam {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			userID := strings.TrimSpace(parts[2])
			if userID == "" {
				writeError(w, http.StatusBadRequest, "user_id is required")
				return
			}
			if err := store.RemoveTeamMember(db, team.ID, userID); err != nil {
				if errors.Is(err, store.ErrTeamMemberNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
			return
		}

		if len(parts) == 2 && parts[1] == "agents" {
			switch r.Method {
			case http.MethodGet:
				if !canManageTeam {
					_, ok, err := store.TeamRoleForUser(db, team.ID, user.ID)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					if !ok {
						writeAuthError(w, store.ErrForbidden)
						return
					}
				}
				items, err := store.ListTeamAgents(db, team.ID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, items)
			case http.MethodPost:
				if !canManageTeam {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload struct {
					AgentID string `json:"agent_id"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				item, err := store.AddTeamAgent(db, team.ID, payload.AgentID)
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, item)
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}

		if len(parts) == 3 && parts[1] == "agents" && r.Method == http.MethodDelete {
			if !canManageTeam {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			agentID := strings.TrimSpace(parts[2])
			if agentID == "" {
				writeError(w, http.StatusBadRequest, "agent_id is required")
				return
			}
			if err := store.RemoveTeamAgent(db, team.ID, agentID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "team agent assignment not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
			return
		}

		writeError(w, http.StatusNotFound, "not found")
	})

	mux.HandleFunc("/api/projects", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			projects, err := store.ListProjectsVisibleToUser(db, user)
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
				Visibility:         projectPayload.Visibility,
				CreatedBy:          user.ID,
				WorkflowID:         projectPayload.WorkflowID,
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			notify("project_created", project.ID, "")
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
		if len(parts) == 2 && parts[1] == "history" && r.Method == http.MethodGet {
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
			limit := 10
			if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
				if _, err := fmt.Sscan(raw, &limit); err != nil {
					writeError(w, http.StatusBadRequest, "limit must be numeric")
					return
				}
			}
			var filter store.HistoryFilter
			filter.UserID = r.URL.Query().Get("user_id")
			filter.AgentID = r.URL.Query().Get("agent_id")
			if raw := strings.TrimSpace(r.URL.Query().Get("team_id")); raw != "" {
				if _, err := fmt.Sscan(raw, &filter.TeamID); err != nil {
					writeError(w, http.StatusBadRequest, "team_id must be numeric")
					return
				}
			}
			events, err := store.ListProjectHistoryFiltered(db, project.ID, limit, filter)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, events)
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
				userID := strings.TrimSpace(parts[2])
				if userID == "" {
					writeError(w, http.StatusBadRequest, "user_id is required")
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
				notify("project_users_updated", project.ID, "")
				writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
				return
			case http.MethodPost:
				if len(parts) != 2 {
					writeError(w, http.StatusBadRequest, "usage: /api/projects/{id}/users")
					return
				}
				var payload struct {
					UserID string `json:"user_id"`
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
				notify("project_users_updated", project.ID, "")
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

		if (len(parts) == 2 && parts[1] == "teams") || (len(parts) == 3 && parts[1] == "teams") {
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
					writeError(w, http.StatusBadRequest, "usage: /api/projects/{id}/teams/{team_id}")
					return
				}
				var teamID int64
				if _, err := fmt.Sscan(parts[2], &teamID); err != nil {
					writeError(w, http.StatusBadRequest, "team_id must be numeric")
					return
				}
				if err := store.RemoveProjectTeamMember(db, project.ID, teamID); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeError(w, http.StatusNotFound, "project team membership not found")
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("project_users_updated", project.ID, "")
				writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
				return
			case http.MethodPost:
				if len(parts) != 2 {
					writeError(w, http.StatusBadRequest, "usage: /api/projects/{id}/teams")
					return
				}
				var payload struct {
					TeamID int64  `json:"team_id"`
					Role   string `json:"role"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				member, err := store.AddProjectTeamMember(db, project.ID, payload.TeamID, payload.Role)
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("project_users_updated", project.ID, "")
				writeJSON(w, http.StatusOK, member)
				return
			case http.MethodGet:
				if len(parts) != 2 {
					writeError(w, http.StatusBadRequest, "usage: /api/projects/{id}/teams")
					return
				}
				members, err := store.ListProjectTeamMembers(db, project.ID)
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

		if (len(parts) == 2 && parts[1] == "labels") || (len(parts) == 3 && parts[1] == "labels") {
			if _, err := requireUser(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var projectID int64
			if _, err := fmt.Sscan(parts[0], &projectID); err != nil {
				writeError(w, http.StatusBadRequest, "invalid project id")
				return
			}
			if len(parts) == 3 {
				// /api/projects/<id>/labels/<label_id>
				var labelID int64
				if _, err := fmt.Sscan(parts[2], &labelID); err != nil {
					writeError(w, http.StatusBadRequest, "invalid label id")
					return
				}
				if r.Method == http.MethodDelete {
					if err := store.DeleteLabel(db, labelID); err != nil {
						if errors.Is(err, store.ErrLabelNotFound) {
							writeError(w, http.StatusNotFound, "label not found")
							return
						}
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
					return
				}
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			// /api/projects/<id>/labels
			switch r.Method {
			case http.MethodGet:
				labels, err := store.ListLabels(db, projectID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, labels)
			case http.MethodPost:
				var req struct {
					Name  string `json:"name"`
					Color string `json:"color"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				label, err := store.CreateLabel(db, projectID, req.Name, req.Color)
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusCreated, label)
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}

		if len(parts) == 2 && r.Method == http.MethodPost {
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
			notify("project_updated", project.ID, "")
			writeJSON(w, http.StatusOK, project)
			return
		}

		if len(parts) != 1 {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		switch r.Method {
		case http.MethodGet:
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			project, err := store.GetProject(db, parts[0])
			if err != nil {
				if errors.Is(err, store.ErrProjectNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeError(w, http.StatusInternalServerError, err.Error())
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
			writeJSON(w, http.StatusOK, project)
		case http.MethodPut:
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
			currentProject, err := store.GetProject(db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			role, err := projectRoleForUser(db, currentProject.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			project, err := store.UpdateProjectWithParams(db, currentProject.ID, store.ProjectUpdateParams{
				Title:              projectPayload.Title,
				Description:        projectPayload.Description,
				AcceptanceCriteria: projectPayload.AcceptanceCriteria,
				GitRepository:      projectPayload.GitRepository,
				GitBranch:          projectPayload.GitBranch,
				Notes:              projectPayload.Notes,
				Visibility:         projectPayload.Visibility,
				WorkflowID:         projectPayload.WorkflowID,
			})
			if err != nil {
				if errors.Is(err, store.ErrProjectNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			notify("project_updated", project.ID, "")
			writeJSON(w, http.StatusOK, project)
		case http.MethodDelete:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			project, err := store.GetProject(db, parts[0])
			if err != nil {
				if errors.Is(err, store.ErrProjectNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if err := store.DeleteProject(db, project.ID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
		_, state, _ := resolveLifecycleRequest(ticketPayload.Status, ticketPayload.Stage, ticketPayload.State)
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
			State:              state,
			Author:             user.Username,
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
		if len(parts) == 1 && r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, story)
			return
		}
		if len(parts) == 1 && r.Method == http.MethodDelete {
			if err := store.DeleteStory(db, story.ID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.WriteHeader(http.StatusNoContent)
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

		project, err := store.GetProjectByID(db, story.ProjectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		beforeTickets, err := store.ListTicketsByProject(db, story.ProjectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		beforeIDs := make(map[string]struct{}, len(beforeTickets))
		for _, ticket := range beforeTickets {
			beforeIDs[ticket.ID] = struct{}{}
		}

		if err := runStoryBreakdownViaTicketCLI(db, project, story); err != nil {
			var analysis storyAnalysisResult
			prompt := fmt.Sprintf(
				"Story title: %s\nStory description: %s\nGenerate JSON shape {\"epics\":[{\"title\":\"...\",\"description\":\"...\",\"tasks\":[{\"title\":\"...\",\"description\":\"...\"}]}]} with 1-4 epics and 2-5 tasks per epic.",
				story.Title,
				story.Description,
			)
			if err := runRoleJSONAnalysis(db, "StoryReview", prompt, &analysis); err != nil || len(analysis.Epics) == 0 {
				analysis = fallbackStoryAnalysis(story)
			}
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
					Author:      user.Username,
					CreatedBy:   user.ID,
					State:       store.StateIdle,
				})
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				for _, taskSpec := range epicSpec.Tasks {
					taskTitle := strings.TrimSpace(taskSpec.Title)
					if taskTitle == "" {
						continue
					}
					parentID := epic.ID
					_, err := store.CreateTicket(db, store.TicketCreateParams{
						ProjectID:   story.ProjectID,
						ParentID:    &parentID,
						Type:        "task",
						Title:       taskTitle,
						Description: strings.TrimSpace(taskSpec.Description),
						Author:      user.Username,
						CreatedBy:   user.ID,
						State:       store.StateIdle,
					})
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
				}
			}
		}

		afterTickets, err := store.ListTicketsByProject(db, story.ProjectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		createdEpics := 0
		createdTasks := 0
		for _, ticket := range afterTickets {
			if _, existed := beforeIDs[ticket.ID]; existed {
				continue
			}
			_ = store.LinkStoryToTicket(db, story.ID, ticket.ID)
			notify("ticket_created", ticket.ProjectID, ticket.ID)
			switch strings.ToLower(strings.TrimSpace(ticket.Type)) {
			case "epic":
				createdEpics++
			case "task":
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
			ctx := store.EnrichTicketContext(db, ticket)
			payload["project"] = ctx.Project
			payload["parents"] = ctx.Parents
			payload["workflow"] = ctx.Workflow
			payload["role"] = ctx.Role
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

			if (len(parts) == 2 && parts[1] == "labels") || (len(parts) == 3 && parts[1] == "labels") {
				if len(parts) == 3 {
					// /api/tickets/<ref>/labels/<label_id>
					var labelID int64
					if _, err := fmt.Sscan(parts[2], &labelID); err != nil {
						writeError(w, http.StatusBadRequest, "invalid label id")
						return
					}
					if r.Method == http.MethodDelete {
						if !canWriteProject(role) {
							writeAuthError(w, store.ErrForbidden)
							return
						}
						if err := store.RemoveTicketLabel(db, id, labelID); err != nil {
							writeError(w, http.StatusBadRequest, err.Error())
							return
						}
						writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
						return
					}
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					return
				}
				switch r.Method {
				case http.MethodGet:
					if !canReadProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					labels, err := store.ListTicketLabels(db, id)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, labels)
				case http.MethodPost:
					if !canWriteProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					var req struct {
						LabelID int64 `json:"label_id"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					if err := store.AddTicketLabel(db, id, req.LabelID); err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
				default:
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			}

			if (len(parts) == 2 && parts[1] == "time") || (len(parts) == 3 && parts[1] == "time") {
				if len(parts) == 3 && parts[2] == "total" {
					if !canReadProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					total, err := store.TotalTimeForTicket(db, id)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, map[string]int{"total": total})
					return
				}
				switch r.Method {
				case http.MethodGet:
					if !canReadProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					entries, err := store.ListTimeEntries(db, id)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, entries)
				case http.MethodPost:
					if !canWriteProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					var req struct {
						Minutes int    `json:"minutes"`
						Note    string `json:"note"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					entry, err := store.LogTime(db, id, user.ID, req.Minutes, req.Note)
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					writeJSON(w, http.StatusCreated, entry)
				default:
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
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
							"key":        ticket.ID,
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
				cloned, err := store.CloneTicket(db, id, user.Username, user.ID)
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
			if len(parts) == 2 && parts[1] == "ready" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				ticket, err := store.SetTicketReady(db, id, true, user.Username, user.ID)
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
			if len(parts) == 2 && parts[1] == "notready" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				ticket, err := store.SetTicketReady(db, id, false, user.Username, user.ID)
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
			if len(parts) == 2 && parts[1] == "workflow" {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				switch r.Method {
				case http.MethodPost:
					var payload struct {
						WorkflowID int64 `json:"workflow_id"`
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.WorkflowID == 0 {
						writeError(w, http.StatusBadRequest, "workflow_id is required")
						return
					}
					ticket, err := store.SetTicketWorkflow(db, id, payload.WorkflowID)
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					notify("ticket_updated", ticket.ProjectID, ticket.ID)
					writeJSON(w, http.StatusOK, ticket)
				case http.MethodDelete:
					ticket, err := store.UnsetTicketWorkflow(db, id)
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					notify("ticket_updated", ticket.ProjectID, ticket.ID)
					writeJSON(w, http.StatusOK, ticket)
				default:
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
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
						Author:      user.Username,
						CreatedBy:   user.ID,
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
				stage, state, _ := resolveLifecycleRequest(ticketPayload.Status, ticketPayload.Stage, ticketPayload.State)
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

	mux.HandleFunc("/api/labels/", func(w http.ResponseWriter, r *http.Request) {
		_, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		idStr := strings.TrimPrefix(r.URL.Path, "/api/labels/")
		var id int64
		if _, err := fmt.Sscan(idStr, &id); err != nil {
			writeError(w, http.StatusBadRequest, "invalid label id")
			return
		}
		if err := store.DeleteLabel(db, id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	})

	mux.HandleFunc("/api/time/", func(w http.ResponseWriter, r *http.Request) {
		_, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		idStr := strings.TrimPrefix(r.URL.Path, "/api/time/")
		var id int64
		if _, err := fmt.Sscan(idStr, &id); err != nil {
			writeError(w, http.StatusBadRequest, "invalid time entry id")
			return
		}
		if err := store.DeleteTimeEntry(db, id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	})

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
			var projectID int64
			if _, err := fmt.Sscan(strings.TrimSpace(r.URL.Query().Get("project_id")), &projectID); err != nil {
				writeError(w, http.StatusBadRequest, "project_id must be numeric")
				return
			}
			ticketID := strings.TrimSpace(r.URL.Query().Get("ticket_id"))
			if ticketID == "" {
				writeError(w, http.StatusBadRequest, "ticket_id is required")
				return
			}
			dependsOn := strings.TrimSpace(r.URL.Query().Get("depends_on"))
			if dependsOn == "" {
				writeError(w, http.StatusBadRequest, "depends_on is required")
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

