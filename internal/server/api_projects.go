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
			if err := json.NewDecoder(r.Body).Decode(&projectPayload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			project, err := store.CreateProjectWithParams(r.Context(), db, store.ProjectCreateParams{
				Prefix:             projectPayload.Prefix,
				Title:              projectPayload.Title,
				Description:        projectPayload.Description,
				AcceptanceCriteria: projectPayload.AcceptanceCriteria,
				GitRepository:      projectPayload.GitRepository,
				GitBranch:          projectPayload.GitBranch,
				Notes:              projectPayload.Notes,
				Visibility:         projectPayload.Visibility,
				CreatedBy:          user.ID,
				SdlcID:         projectPayload.SdlcID,
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
				if _, err := fmt.Sscan(raw, &limit); err != nil {
					writeError(w, http.StatusBadRequest, "limit must be numeric")
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
			stories, err := store.ListStoriesByProject(r.Context(), db, project.ID)
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
				member, err := store.AddProjectMember(r.Context(), db, project.ID, payload.UserID, payload.Role)
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
				member, err := store.AddProjectTeamMember(r.Context(), db, project.ID, payload.TeamID, payload.Role)
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
				labels, err := store.ListLabels(r.Context(), db, projectID)
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
				label, err := store.CreateLabel(r.Context(), db, projectID, req.Name, req.Color)
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
			project, err := store.GetProject(r.Context(), db, parts[0])
			if err != nil {
				if errors.Is(err, store.ErrProjectNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeError(w, http.StatusInternalServerError, err.Error())
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
			currentProject, err := store.GetProject(r.Context(), db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			role, err := projectRoleForUser(r.Context(), db, currentProject.ID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			project, err := store.UpdateProjectWithParams(r.Context(), db, currentProject.ID, store.ProjectUpdateParams{
				Title:              projectPayload.Title,
				Description:        projectPayload.Description,
				AcceptanceCriteria: projectPayload.AcceptanceCriteria,
				GitRepository:      projectPayload.GitRepository,
				GitBranch:          projectPayload.GitBranch,
				Notes:              projectPayload.Notes,
				Visibility:         projectPayload.Visibility,
				SdlcID:         projectPayload.SdlcID,
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
