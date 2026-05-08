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

func (r *router) registerTeamHandlers() {
	db := r.db
	mux := r.mux

	mux.HandleFunc("/api/teams", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireUser(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			teams, err := store.ListTeams(r.Context(), db, 0)
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
			if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			team, err := store.CreateTeamWithParams(r.Context(), db, store.TeamCreateParams{
				ID:           payload.ID,
				Name:         payload.Name,
				ParentTeamID: payload.ParentTeamID,
			})
			if err != nil {
				writeStoreError(w, err)
				return
			}
			// The creator is always an owner of the new team.
			if _, err := store.AddTeamMember(r.Context(), db, team.ID, user.ID, store.TeamRoleOwner, ""); err != nil {
				writeStoreError(w, err)
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
		if _, scanErr := fmt.Sscan(parts[0], &teamID); scanErr != nil {
			writeError(w, http.StatusBadRequest, "invalid team id")
			return
		}
		team, err := store.GetTeamByID(r.Context(), db, teamID)
		if err != nil {
			if errors.Is(err, store.ErrTeamNotFound) {
				writeError(w, http.StatusNotFound, "team not found")
				return
			}
			writeStoreError(w, err)
			return
		}
		canManageTeam := user.Role == "admin"
		if !canManageTeam {
			role, ok, err := store.TeamRoleForUser(r.Context(), db, team.ID, user.ID)
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
				updated, err := store.UpdateTeam(r.Context(), db, team.ID, payload.Name, payload.ParentTeamID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, updated)
			case http.MethodDelete:
				if !canManageTeam {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				if err := store.DeleteTeam(r.Context(), db, team.ID); err != nil {
					if errors.Is(err, store.ErrTeamNotFound) {
						writeError(w, http.StatusNotFound, "team not found")
						return
					}
					writeStoreError(w, err)
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
					_, ok, err := store.TeamRoleForUser(r.Context(), db, team.ID, user.ID)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					if !ok {
						writeAuthError(w, store.ErrForbidden)
						return
					}
				}
				members, err := store.ListTeamMembers(r.Context(), db, team.ID)
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
				member, err := store.AddTeamMember(r.Context(), db, team.ID, payload.UserID, payload.Role, payload.JobTitle)
				if err != nil {
					writeStoreError(w, err)
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
			if err := store.RemoveTeamMember(r.Context(), db, team.ID, userID); err != nil {
				if errors.Is(err, store.ErrTeamMemberNotFound) {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
			return
		}

		if len(parts) == 2 && parts[1] == "agents" {
			switch r.Method {
			case http.MethodGet:
				if !canManageTeam {
					_, ok, err := store.TeamRoleForUser(r.Context(), db, team.ID, user.ID)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					if !ok {
						writeAuthError(w, store.ErrForbidden)
						return
					}
				}
				items, err := store.ListTeamAgents(r.Context(), db, team.ID)
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
				item, err := store.AddTeamAgent(r.Context(), db, team.ID, payload.AgentID)
				if err != nil {
					writeStoreError(w, err)
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
			if err := store.RemoveTeamAgent(r.Context(), db, team.ID, agentID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "team agent assignment not found")
					return
				}
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
			return
		}

		writeError(w, http.StatusNotFound, "not found")
	})
}
