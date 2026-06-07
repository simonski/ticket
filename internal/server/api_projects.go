package server

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func (r *router) registerProjectHandlers() {
	db := r.db
	mux := r.mux
	notify := r.notify

	mux.HandleFunc("/api/projects", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			projects, err := store.ListProjectsVisibleToUser(r.Context(), db, user)
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
			if decodeErr := json.NewDecoder(r.Body).Decode(&projectPayload); decodeErr != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			project, err := store.CreateProjectWithParams(r.Context(), db, store.ProjectCreateParams{
				ID:                 projectPayload.ID,
				Prefix:             projectPayload.Prefix,
				Title:              projectPayload.Title,
				Description:        projectPayload.Description,
				AcceptanceCriteria: projectPayload.AcceptanceCriteria,
				DORMap:             projectPayload.DORMap,
				DODMap:             projectPayload.DODMap,
				ACMap:              projectPayload.ACMap,
				GitRepository:      projectPayload.GitRepository,
				Notes:              projectPayload.Notes,
				Visibility:         projectPayload.Visibility,
				AcceptsNewMembers:  projectPayload.AcceptsNewMembers,
				CreatedBy:          user.ID,
				WorkflowID:         projectPayload.WorkflowID,
				AgentModelProvider: projectPayload.AgentModelProvider,
				AgentModelName:     projectPayload.AgentModelName,
				AgentModelURL:      projectPayload.AgentModelURL,
				AgentModelAPIKey:   projectPayload.AgentModelAPIKey,
			})
			if err != nil {
				writeStoreError(w, err)
				return
			}
			notify("project_created", project.ID, "")
			writeJSON(w, http.StatusCreated, project)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/projects/by-repository", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		user, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		repository := strings.TrimSpace(r.URL.Query().Get("repository"))
		if repository == "" {
			writeError(w, http.StatusBadRequest, "repository is required")
			return
		}
		project, err := store.GetProjectByGitRepository(r.Context(), db, repository)
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
			writeError(w, http.StatusNotFound, store.ErrProjectNotFound.Error())
			return
		}
		writeJSON(w, http.StatusOK, project)
	})

	mux.HandleFunc("/api/projects/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireUser(db, r); err != nil {
			writeAuthError(w, err)
			return
		}

		trimmed := strings.TrimPrefix(r.URL.Path, "/api/projects/")
		parts := strings.Split(trimmed, "/")
		if len(parts) == 2 && parts[1] == "programme" {
			if handled := handleProjectProgrammeRoute(w, r, db, parts[0]); handled {
				return
			}
		}
		if len(parts) == 2 && parts[1] == "sprints" {
			if handled := handleProjectSprintsRoute(w, r, db, parts[0]); handled {
				return
			}
		}
		if len(parts) == 2 && parts[1] == "goals" {
			if handled := handleProjectGoals(w, r, db, parts[0]); handled {
				return
			}
		}
		if len(parts) == 2 && parts[1] == "goal-inbox" {
			if handled := handleProjectGoalInbox(w, r, db, parts[0]); handled {
				return
			}
		}
		if len(parts) == 2 && parts[1] == "documents" {
			if handled := handleProjectDocuments(w, r, db, parts[0]); handled {
				return
			}
		}
		if len(parts) == 2 && parts[1] == "access-requests" {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			project, err := store.GetProject(r.Context(), db, parts[0])
			if err != nil {
				writeStoreError(w, err)
				return
			}
			switch r.Method {
			case http.MethodPost:
				if !project.AcceptsNewMembers {
					writeAuthError(w, fmt.Errorf("%w: project is not accepting new members", store.ErrUnauthorized))
					return
				}
				var payload struct {
					Message string `json:"message"`
				}
				if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				request, err := store.CreateProjectAccessRequest(r.Context(), db, project.ID, user.ID, payload.Message)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				if err := store.AddHistoryEvent(r.Context(), db, project.ID, "", "project_access_request_created", map[string]any{
					"request_id":     request.ID,
					"user_id":        request.UserID,
					"username":       request.Username,
					"project_id":     request.ProjectID,
					"project_prefix": request.ProjectPrefix,
					"project_title":  request.ProjectTitle,
					"status":         request.Status,
					"message":        request.Message,
					"requested_by":   user.Username,
				}, user.ID); err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusCreated, request)
				return
			case http.MethodGet:
				role, err := projectRoleForUser(r.Context(), db, project.ID, user)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				if !canAdminProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				requests, err := store.ListProjectAccessRequests(r.Context(), db, project.ID, strings.TrimSpace(r.URL.Query().Get("status")))
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, requests)
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}
		if len(parts) == 4 && parts[1] == "access-requests" {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			project, err := store.GetProject(r.Context(), db, parts[0])
			if err != nil {
				writeStoreError(w, err)
				return
			}
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			if !canAdminProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			var requestID int64
			if _, scanErr := fmt.Sscan(parts[2], &requestID); scanErr != nil {
				writeError(w, http.StatusBadRequest, "request id must be numeric")
				return
			}
			if r.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			var payload struct {
				Message string `json:"message"`
			}
			if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			status := ""
			switch parts[3] {
			case "approve":
				status = "approved"
			case "reject":
				status = "rejected"
			default:
				writeError(w, http.StatusNotFound, "not found")
				return
			}
			request, err := store.SetProjectAccessRequestStatus(r.Context(), db, requestID, status, payload.Message, user.Username)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			if err := store.AddHistoryEvent(r.Context(), db, project.ID, "", "project_access_request_"+status, map[string]any{
				"request_id":       request.ID,
				"user_id":          request.UserID,
				"username":         request.Username,
				"project_id":       request.ProjectID,
				"project_prefix":   request.ProjectPrefix,
				"project_title":    request.ProjectTitle,
				"status":           request.Status,
				"message":          request.Message,
				"decision_message": request.DecisionMessage,
				"decided_by":       user.Username,
			}, user.ID); err != nil {
				writeStoreError(w, err)
				return
			}
			if _, err := store.CreateUserNotification(r.Context(), db, store.BuildProjectAccessDecisionNotification(request, user.Username)); err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, request)
			return
		}
		if len(parts) == 2 && parts[1] == "tickets" && r.Method == http.MethodGet {
			project, err := store.GetProject(r.Context(), db, parts[0])
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
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
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
				if _, scanErr := fmt.Sscan(raw, &limit); scanErr != nil {
					writeError(w, http.StatusBadRequest, "limit must be numeric")
					return
				}
			}
			offset := 0
			if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
				if _, scanErr := fmt.Sscan(raw, &offset); scanErr != nil {
					writeError(w, http.StatusBadRequest, "offset must be numeric")
					return
				}
			}
			includeArchived := false
			if raw := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("include_archived"))); raw == "1" || raw == "true" || raw == "yes" {
				includeArchived = true
			}
			tasks, err := store.ListTickets(r.Context(), db, store.TicketListParams{
				ProjectID:       project.ID,
				Type:            r.URL.Query().Get("type"),
				Stage:           r.URL.Query().Get("stage"),
				State:           r.URL.Query().Get("state"),
				Status:          r.URL.Query().Get("status"),
				Search:          r.URL.Query().Get("q"),
				Assignee:        r.URL.Query().Get("assignee"),
				Limit:           limit,
				Offset:          offset,
				IncludeArchived: includeArchived,
			})
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, tasks)
			return
		}
		if len(parts) == 2 && parts[1] == "interventions" && r.Method == http.MethodGet {
			project, err := store.GetProject(r.Context(), db, parts[0])
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
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canViewInterventions(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			limit := 0
			if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
				if _, scanErr := fmt.Sscan(raw, &limit); scanErr != nil {
					writeError(w, http.StatusBadRequest, "limit must be numeric")
					return
				}
			}
			offset := 0
			if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
				if _, scanErr := fmt.Sscan(raw, &offset); scanErr != nil {
					writeError(w, http.StatusBadRequest, "offset must be numeric")
					return
				}
			}
			includeArchived := false
			if raw := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("include_archived"))); raw == "1" || raw == "true" || raw == "yes" {
				includeArchived = true
			}
			interventions, err := store.ListTickets(r.Context(), db, store.TicketListParams{
				ProjectID:       project.ID,
				State:           "fail",
				Limit:           limit,
				Offset:          offset,
				IncludeArchived: includeArchived,
			})
			if err != nil {
				writeStoreError(w, err)
				return
			}
			payload := make([]map[string]any, 0, len(interventions))
			for _, ticket := range interventions {
				state, stateErr := store.GetInterventionState(r.Context(), db, ticket.ID)
				if stateErr != nil {
					writeStoreError(w, stateErr)
					return
				}
				payload = append(payload, map[string]any{
					"ticket_id":                  ticket.ID,
					"project_id":                 ticket.ProjectID,
					"parent_id":                  ticket.ParentID,
					"clone_of":                   ticket.CloneOf,
					"type":                       ticket.Type,
					"title":                      ticket.Title,
					"description":                ticket.Description,
					"acceptance_criteria":        ticket.AcceptanceCriteria,
					"dor_map":                    ticket.DORMap,
					"dod_map":                    ticket.DODMap,
					"ac_map":                     ticket.ACMap,
					"git_repository":             ticket.GitRepository,
					"git_branch":                 ticket.GitBranch,
					"workflow_id":                ticket.WorkflowID,
					"workflow_stage_id":          ticket.WorkflowStageID,
					"role_id":                    ticket.RoleID,
					"stage":                      ticket.Stage,
					"state":                      ticket.State,
					"status":                     ticket.Status,
					"priority":                   ticket.Priority,
					"order":                      ticket.Order,
					"estimate_effort":            ticket.EstimateEffort,
					"estimate_complete":          ticket.EstimateComplete,
					"health_score":               ticket.HealthScore,
					"assignee":                   ticket.Assignee,
					"author":                     ticket.Author,
					"draft":                      ticket.Draft,
					"complete":                   ticket.Complete,
					"archived":                   ticket.Archived,
					"deleted":                    ticket.Deleted,
					"previous_workflow_stage_id": ticket.PreviousWorkflowStageID,
					"previous_role_id":           ticket.PreviousRoleID,
					"created_by":                 ticket.CreatedBy,
					"created_at":                 ticket.CreatedAt,
					"updated_at":                 ticket.UpdatedAt,
					"intervention_state":         state,
				})
			}
			writeJSON(w, http.StatusOK, payload)
			return
		}
		if len(parts) == 3 && parts[1] == "interventions" && parts[2] == "report" && r.Method == http.MethodGet {
			project, err := store.GetProject(r.Context(), db, parts[0])
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
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canViewInterventions(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			escalationHours := 24
			if raw := strings.TrimSpace(r.URL.Query().Get("escalation_hours")); raw != "" {
				if _, scanErr := fmt.Sscan(raw, &escalationHours); scanErr != nil {
					writeError(w, http.StatusBadRequest, "escalation_hours must be numeric")
					return
				}
			}
			report, err := store.BuildInterventionReport(r.Context(), db, project.ID, escalationHours)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("format")), "csv") {
				w.Header().Set("Content-Type", "text/csv; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				writer := csv.NewWriter(w)
				_ = writer.Write([]string{"ticket_id", "title", "state", "owner", "age_hours", "escalated", "updated_at"})
				for _, item := range report.Items {
					_ = writer.Write([]string{
						item.TicketID,
						item.Title,
						item.State,
						item.OwnerName,
						fmt.Sprintf("%d", item.AgeHours),
						fmt.Sprintf("%t", item.Escalated),
						item.UpdatedAt,
					})
				}
				writer.Flush()
				if err := writer.Error(); err != nil {
					writeStoreError(w, err)
				}
				return
			}
			writeJSON(w, http.StatusOK, report)
			return
		}
		if len(parts) == 3 && parts[1] == "interventions" && parts[2] == "trends" && r.Method == http.MethodGet {
			project, err := store.GetProject(r.Context(), db, parts[0])
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
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canViewInterventions(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			days := 7
			if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
				if _, scanErr := fmt.Sscan(raw, &days); scanErr != nil {
					writeError(w, http.StatusBadRequest, "days must be numeric")
					return
				}
			}
			trends, err := store.BuildInterventionTrends(r.Context(), db, project.ID, days)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, trends)
			return
		}
		if len(parts) == 3 && parts[1] == "interventions" && parts[2] == "drilldown" && r.Method == http.MethodGet {
			project, err := store.GetProject(r.Context(), db, parts[0])
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
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canViewInterventions(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			escalationHours := 24
			if raw := strings.TrimSpace(r.URL.Query().Get("escalation_hours")); raw != "" {
				if _, scanErr := fmt.Sscan(raw, &escalationHours); scanErr != nil {
					writeError(w, http.StatusBadRequest, "escalation_hours must be numeric")
					return
				}
			}
			drilldown, err := store.BuildInterventionDrilldown(r.Context(), db, project.ID, escalationHours)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, drilldown)
			return
		}
		if len(parts) == 3 && parts[1] == "work-items" && parts[2] == "queue" && r.Method == http.MethodGet {
			project, err := store.GetProject(r.Context(), db, parts[0])
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
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canReadProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			limit := 20
			if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
				if _, scanErr := fmt.Sscan(raw, &limit); scanErr != nil {
					writeError(w, http.StatusBadRequest, "limit must be numeric")
					return
				}
			}
			strategy := strings.TrimSpace(r.URL.Query().Get("strategy"))
			queue, err := store.ListProjectWorkItemQueue(r.Context(), db, project.ID, strategy, limit)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, queue)
			return
		}
		if len(parts) == 2 && parts[1] == "forecast" && r.Method == http.MethodGet {
			project, err := store.GetProject(r.Context(), db, parts[0])
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
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canReadProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			limit := 50
			if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
				if _, scanErr := fmt.Sscan(raw, &limit); scanErr != nil {
					writeError(w, http.StatusBadRequest, "limit must be numeric")
					return
				}
			}
			forecast, err := store.BuildProjectForecast(r.Context(), db, project.ID, limit)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, forecast)
			return
		}
		if len(parts) == 3 && parts[1] == "forecast" && parts[2] == "calibration" && r.Method == http.MethodGet {
			project, err := store.GetProject(r.Context(), db, parts[0])
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
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canReadProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			lookbackHours := 1
			if raw := strings.TrimSpace(r.URL.Query().Get("lookback_hours")); raw != "" {
				if _, scanErr := fmt.Sscan(raw, &lookbackHours); scanErr != nil {
					writeError(w, http.StatusBadRequest, "lookback_hours must be numeric")
					return
				}
			}
			report, err := store.BuildProjectForecastCalibration(r.Context(), db, project.ID, lookbackHours)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, report)
			return
		}
		if len(parts) == 3 && parts[1] == "forecast" && parts[2] == "backtest" && r.Method == http.MethodGet {
			project, err := store.GetProject(r.Context(), db, parts[0])
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
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canReadProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			windowHours := 24
			if raw := strings.TrimSpace(r.URL.Query().Get("window_hours")); raw != "" {
				if _, scanErr := fmt.Sscan(raw, &windowHours); scanErr != nil {
					writeError(w, http.StatusBadRequest, "window_hours must be numeric")
					return
				}
			}
			report, err := store.BuildProjectForecastBacktest(r.Context(), db, project.ID, windowHours)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, report)
			return
		}
		if len(parts) == 2 && parts[1] == "history" && r.Method == http.MethodGet {
			project, err := store.GetProject(r.Context(), db, parts[0])
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
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
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
				if _, scanErr := fmt.Sscan(raw, &limit); scanErr != nil {
					writeError(w, http.StatusBadRequest, "limit must be numeric")
					return
				}
			}
			var filter store.HistoryFilter
			filter.UserID = r.URL.Query().Get("user_id")
			filter.AgentID = r.URL.Query().Get("agent_id")
			if raw := strings.TrimSpace(r.URL.Query().Get("team_id")); raw != "" {
				if _, scanErr := fmt.Sscan(raw, &filter.TeamID); scanErr != nil {
					writeError(w, http.StatusBadRequest, "team_id must be numeric")
					return
				}
			}
			events, err := store.ListProjectHistoryFiltered(r.Context(), db, project.ID, limit, filter)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, events)
			return
		}
		if len(parts) == 2 && parts[1] == "stories" && r.Method == http.MethodGet {
			project, err := store.GetProject(r.Context(), db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canReadProject(role) {
				writeAuthError(w, store.ErrForbidden)
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
			stories, err := store.ListStoriesByProject(r.Context(), db, project.ID, limit, offset)
			if err != nil {
				writeStoreError(w, err)
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
			project, err := store.GetProject(r.Context(), db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
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
				if err := store.RemoveProjectMember(r.Context(), db, project.ID, userID); err != nil {
					if errors.Is(err, store.ErrProjectMembershipNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeStoreError(w, err)
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
				member, err := store.AddProjectMember(r.Context(), db, project.ID, payload.UserID, payload.Role)
				if err != nil {
					writeStoreError(w, err)
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
				members, err := store.ListProjectMembers(r.Context(), db, project.ID)
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

		if (len(parts) == 2 && parts[1] == "repositories") || (len(parts) >= 3 && parts[1] == "repositories") {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			project, err := store.GetProject(r.Context(), db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			switch r.Method {
			case http.MethodGet:
				if len(parts) != 2 {
					writeError(w, http.StatusBadRequest, "usage: /api/projects/{id}/repositories")
					return
				}
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				repositories, err := store.ListProjectGitRepositories(r.Context(), db, project.ID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, repositories)
				return
			case http.MethodPost:
				if len(parts) != 2 {
					writeError(w, http.StatusBadRequest, "usage: /api/projects/{id}/repositories")
					return
				}
				if !canManageProjectUsers(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload struct {
					Repository string `json:"repository"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				if err := store.AddProjectGitRepository(r.Context(), db, project.ID, payload.Repository); err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
				return
			case http.MethodDelete:
				if len(parts) < 3 {
					writeError(w, http.StatusBadRequest, "usage: /api/projects/{id}/repositories/{repository}")
					return
				}
				if !canManageProjectUsers(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				repository, err := url.PathUnescape(strings.Join(parts[2:], "/"))
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid repository path")
					return
				}
				if err := store.RemoveProjectGitRepository(r.Context(), db, project.ID, repository); err != nil {
					if errors.Is(err, store.ErrProjectGitRepositoryNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
			project, err := store.GetProject(r.Context(), db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
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
				if err := store.RemoveProjectTeamMember(r.Context(), db, project.ID, teamID); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeError(w, http.StatusNotFound, "project team membership not found")
						return
					}
					writeStoreError(w, err)
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
				member, err := store.AddProjectTeamMember(r.Context(), db, project.ID, payload.TeamID, payload.Role)
				if err != nil {
					writeStoreError(w, err)
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
				members, err := store.ListProjectTeamMembers(r.Context(), db, project.ID)
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
					if err := store.DeleteLabel(r.Context(), db, labelID); err != nil {
						if errors.Is(err, store.ErrLabelNotFound) {
							writeError(w, http.StatusNotFound, "label not found")
							return
						}
						writeStoreError(w, err)
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
				labels, err := store.ListLabels(r.Context(), db, projectID, limit, offset)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, labels)
			case http.MethodPost:
				var req struct {
					ID    *int64 `json:"id,omitempty"`
					Name  string `json:"name"`
					Color string `json:"color"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					writeStoreError(w, err)
					return
				}
				label, err := store.CreateLabelWithParams(r.Context(), db, store.LabelCreateParams{
					ID:        req.ID,
					ProjectID: projectID,
					Name:      req.Name,
					Color:     req.Color,
				})
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusCreated, label)
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "set-draft" && r.Method == http.MethodPut {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			project, err := store.GetProject(r.Context(), db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canManageProjectUsers(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			var payload struct {
				Draft bool `json:"draft"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			if err := store.SetProjectDefaultDraft(r.Context(), db, project.ID, payload.Draft); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			notify("project_updated", project.ID, "")
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		if len(parts) == 2 && parts[1] == "agent-model" {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			project, err := store.GetProject(r.Context(), db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			switch r.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				writeJSON(w, http.StatusOK, store.AgentModelConfig{
					Provider: project.AgentModelProvider,
					Model:    project.AgentModelName,
					URL:      project.AgentModelURL,
					APIKey:   project.AgentModelAPIKey,
				})
				return
			case http.MethodPut:
				if !canManageProjectUsers(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload agentModelConfigRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				updated, err := store.SetProjectAgentModelConfig(r.Context(), db, project.ID, store.AgentModelConfig{
					Provider: payload.Provider,
					Model:    payload.Model,
					URL:      payload.URL,
					APIKey:   payload.APIKey,
				})
				if err != nil {
					writeStoreError(w, err)
					return
				}
				notify("project_updated", updated.ID, "")
				writeJSON(w, http.StatusOK, updated)
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}

		if len(parts) == 2 && r.Method == http.MethodPost {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			project, err := store.GetProject(r.Context(), db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			role, err := projectRoleForUser(r.Context(), db, project.ID, user)
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
			project, err = store.SetProjectStatus(r.Context(), db, project.ID, enabled)
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
			project, _, err := resolveProjectRefForUser(r.Context(), db, parts[0], user)
			if err != nil {
				if errors.Is(err, store.ErrProjectNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeAuthError(w, err)
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
			if decodeErr := json.NewDecoder(r.Body).Decode(&projectPayload); decodeErr != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			currentProject, role, err := resolveProjectRefForUser(r.Context(), db, parts[0], user)
			if err != nil {
				if errors.Is(err, store.ErrProjectNotFound) {
					writeError(w, http.StatusNotFound, "project not found")
					return
				}
				writeAuthError(w, err)
				return
			}
			if !canManageProjectUsers(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			project, err := store.UpdateProjectWithParams(r.Context(), db, currentProject.ID, store.ProjectUpdateParams{
				Title:              projectPayload.Title,
				Description:        projectPayload.Description,
				AcceptanceCriteria: projectPayload.AcceptanceCriteria,
				DORMap:             projectPayload.DORMap,
				DODMap:             projectPayload.DODMap,
				ACMap:              projectPayload.ACMap,
				GitRepository:      projectPayload.GitRepository,
				Notes:              projectPayload.Notes,
				Visibility:         projectPayload.Visibility,
				AcceptsNewMembers:  projectPayload.AcceptsNewMembers,
				WorkflowID:         projectPayload.WorkflowID,
				AgentModelProvider: projectPayload.AgentModelProvider,
				AgentModelName:     projectPayload.AgentModelName,
				AgentModelURL:      projectPayload.AgentModelURL,
				AgentModelAPIKey:   projectPayload.AgentModelAPIKey,
			})
			if err != nil {
				if errors.Is(err, store.ErrProjectNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeStoreError(w, err)
				return
			}
			notify("project_updated", project.ID, "")
			writeJSON(w, http.StatusOK, project)
		case http.MethodDelete:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			project, err := store.GetProject(r.Context(), db, parts[0])
			if err != nil {
				if errors.Is(err, store.ErrProjectNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if err := store.DeleteProject(r.Context(), db, project.ID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}
