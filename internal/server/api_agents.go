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

func (r *router) registerAgentHandlers() {
	db := r.db
	mux := r.mux
	notify := r.notify
	verbose := r.verbose
	output := r.output

	mux.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			agents, err := store.ListAgents(r.Context(), db)
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
			agent, generatedPassword, err := store.CreateAgent(r.Context(), db, payload.Password)
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
			statuses, err := store.ListAgentStatuses(r.Context(), db)
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
			agent, err := store.AuthenticateAgent(r.Context(), db, agentID, agentPass)
			if err != nil {
				if errors.Is(err, store.ErrInvalidCredentials) || errors.Is(err, store.ErrForbidden) {
					writeAuthError(w, err)
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			agent, err = store.TouchAgent(r.Context(), db, agent.ID, "online")
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
			agent, err := store.AuthenticateAgent(r.Context(), db, agentID, agentPass)
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
			agent, err = store.TouchAgent(r.Context(), db, agent.ID, status)
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
				DryRun          bool    `json:"dry_run,omitempty"`
				ConfigUpdatedAt string  `json:"config_updated_at,omitempty"`
			}
			// Body is optional — may be empty for simple requests.
			_ = json.NewDecoder(r.Body).Decode(&payload)
			vlog := func(format string, args ...any) {
				if verbose && output != nil {
					fmt.Fprintf(output, "AGENT %s\n", fmt.Sprintf(format, args...))
				}
			}
			vlog("request from agent=%q project_id=%d", agentID, payload.ProjectID)
			agent, err := store.AuthenticateAgent(r.Context(), db, agentID, agentPass)
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
				projects, err := store.ListProjects(r.Context(), db)
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
			currentAssigned, hadCurrent, err := store.CurrentAssignedTicketForUser(r.Context(), db, projectID, agent.Username)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if hadCurrent {
				vlog("agent has current assignment: %s %q (status=%s)", currentAssigned.ID, currentAssigned.Title, currentAssigned.Status)
			} else {
				vlog("agent has no current assignment")
			}
			ticket, status, err := store.RequestTicket(r.Context(), db, store.TicketRequestParams{
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
				if reasons, err := store.ExplainNoWork(r.Context(), db, projectID, agent.Username); err == nil {
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
				_, _ = store.TouchAgent(r.Context(), db, agent.ID, "working")
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
			} else {
				_, _ = store.TouchAgent(r.Context(), db, agent.ID, "soliciting")
			}
			// Check if config should be included in response.
			// Include config if: 1) it has changed since last poll, or 2) a new ticket is assigned
			var configMap map[string]string
			var configUpdatedAt string
			currentConfigUpdatedAt, err := store.GetAgentConfigUpdatedAt(r.Context(), db, agent.ID)
			if err == nil {
				configHasChanged := payload.ConfigUpdatedAt != currentConfigUpdatedAt
				includeConfig := configHasChanged || agentStatus == "NEW"
				if includeConfig {
					configMap, err = store.GetAgentConfigMap(r.Context(), db, agent.ID)
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
				"sdlc":          nil,
				"role":              nil,
				"config":            configMap,
				"config_updated_at": configUpdatedAt,
			}
			if agentStatus == "NEW" || agentStatus == "CURRENT" {
				response["ticket"] = ticket
				ctx := store.EnrichTicketContext(r.Context(), db, ticket)
				response["project"] = ctx.Project
				response["parents"] = ctx.Parents
				response["sdlc"] = ctx.Sdlc
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
			agent, err := store.AuthenticateAgent(r.Context(), db, agentID, agentPass)
			if err != nil {
				if errors.Is(err, store.ErrInvalidCredentials) || errors.Is(err, store.ErrForbidden) {
					writeAuthError(w, err)
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			current, err := store.GetTicket(r.Context(), db, ticketID)
			if err != nil {
				if errors.Is(err, store.ErrTicketNotFound) {
					writeError(w, http.StatusNotFound, "ticket not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			updated, err := store.UpdateTicket(r.Context(), db, ticketID, store.TicketUpdateParams{
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
			_ = store.AddHistoryEvent(r.Context(), db, updated.ProjectID, updated.ID, "agent_completed", map[string]any{
				"key":       updated.ID,
				"agent":     agent.Username,
				"result":    payload.Result,
				"new_stage": updated.Stage,
				"new_state": updated.State,
			}, agent.ID)
			_, _ = store.TouchAgent(r.Context(), db, agent.ID, "soliciting")
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
				updated, err := store.UpdateAgent(r.Context(), db, id, store.AgentUpdateParams{
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
				if err := store.DeleteAgent(r.Context(), db, id); err != nil {
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
				entries, err := store.ListAgentConfig(r.Context(), db, id)
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
				if err := store.SetAgentConfig(r.Context(), db, id, payload.Key, payload.Value); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			case http.MethodDelete:
				if len(parts) != 3 {
					writeError(w, http.StatusBadRequest, "usage: /api/agents/{id}/config/{key}")
					return
				}
				if err := store.DeleteAgentConfig(r.Context(), db, id, parts[2]); err != nil {
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
			updated, err := store.SetAgentEnabled(r.Context(), db, id, enabled)
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
}
