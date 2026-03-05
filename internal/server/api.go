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

func registerAPI(mux *http.ServeMux, db *sql.DB, version string) {
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
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		user, err := userFromRequest(db, r)
		if err != nil {
			if errors.Is(err, store.ErrUnauthorized) {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":         "ok",
					"authenticated":  false,
					"server_version": version,
				})
				return
			}
			writeAuthError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":         "ok",
			"authenticated":  true,
			"server_version": version,
			"user":           user,
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
				Notes:              projectPayload.Notes,
				CreatedBy:          user.ID,
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
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
			limit := 0
			if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
				if _, err := fmt.Sscan(raw, &limit); err != nil {
					writeError(w, http.StatusBadRequest, "limit must be numeric")
					return
				}
			}
			tasks, err := store.ListTickets(db, store.TicketListParams{
				ProjectID: project.ID,
				Type:      r.URL.Query().Get("type"),
				Stage:     r.URL.Query().Get("stage"),
				State:     r.URL.Query().Get("state"),
				Status:    r.URL.Query().Get("status"),
				Search:    r.URL.Query().Get("q"),
				Assignee:  r.URL.Query().Get("assignee"),
				Limit:     limit,
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, tasks)
			return
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
		ticket, err := store.CreateTicket(db, store.TicketCreateParams{
			ProjectID:          ticketPayload.ProjectID,
			ParentID:           ticketPayload.ParentID,
			Type:               ticketPayload.Type,
			Title:              ticketPayload.Title,
			Description:        ticketPayload.Description,
			AcceptanceCriteria: ticketPayload.AcceptanceCriteria,
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
		writeJSON(w, http.StatusCreated, ticket)
	}
	mux.HandleFunc("/api/tickets", handleTicketsCollection)

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

			if len(parts) == 2 && parts[1] == "history" && r.Method == http.MethodGet {
				events, err := store.ListHistoryEvents(db, id)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, events)
				return
			}

			if len(parts) == 2 && parts[1] == "health" && r.Method == http.MethodPost {
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
				writeJSON(w, http.StatusOK, ticket)
				return
			}

			if len(parts) == 2 && parts[1] == "comments" {
				switch r.Method {
				case http.MethodGet:
					comments, err := store.ListComments(db, id)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, comments)
				case http.MethodPost:
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
					}
					writeJSON(w, http.StatusCreated, comment)
				default:
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			}

			if len(parts) == 2 && parts[1] == "dependencies" && r.Method == http.MethodGet {
				dependencies, err := store.ListDependencies(db, id)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, dependencies)
				return
			}

			if len(parts) == 2 && parts[1] == "clone" && r.Method == http.MethodPost {
				cloned, err := store.CloneTicket(db, id, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusCreated, cloned)
				return
			}

			switch r.Method {
			case http.MethodGet:
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
				ticket, err := store.UpdateTicket(db, id, store.TicketUpdateParams{
					Title:              ticketPayload.Title,
					Description:        ticketPayload.Description,
					AcceptanceCriteria: ticketPayload.AcceptanceCriteria,
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
				writeJSON(w, http.StatusOK, ticket)
			case http.MethodDelete:
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
			dependency, err := store.AddDependency(db, dependencyPayload.ProjectID, dependencyPayload.TicketID, dependencyPayload.DependsOn, user.ID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
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
			if err := store.DeleteDependency(db, projectID, ticketID, dependsOn); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "dependency not found")
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

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type projectRequest struct {
	Prefix             string `json:"prefix"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	Notes              string `json:"notes"`
}

type ticketRequest struct {
	ProjectID          int64  `json:"project_id"`
	ParentID           *int64 `json:"parent_id"`
	Type               string `json:"type"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
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
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
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
