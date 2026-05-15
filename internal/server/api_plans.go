package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func (r *router) registerPlanHandlers() {
	db := r.db
	mux := r.mux

	mux.HandleFunc("/api/plans", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		switch r.Method {
		case http.MethodGet:
			plans, err := store.ListPlans(r.Context(), db)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, plans)
		case http.MethodPost:
			var payload planRequest
			if err := decodeJSONBody(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			plan, err := store.CreatePlan(r.Context(), db, store.PlanCreateParams{
				Slug:                 payload.Slug,
				Name:                 payload.Name,
				Description:          payload.Description,
				MaxProjects:          payload.MaxProjects,
				MaxPrivateProjects:   payload.MaxPrivateProjects,
				MaxTickets:           payload.MaxTickets,
				MaxTicketsPerProject: payload.MaxTicketsPerProject,
				MaxTeamMemberships:   payload.MaxTeamMemberships,
				MaxAPICallsPerDay:    payload.MaxAPICallsPerDay,
				DefaultProjectAlias:  payload.DefaultProjectAlias,
				RegistrationActions:  payload.RegistrationActions,
			})
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, plan)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/plans/default", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		switch r.Method {
		case http.MethodGet:
			plan, err := store.DefaultPlan(r.Context(), db)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, plan)
		case http.MethodPost:
			var payload struct {
				Slug string `json:"slug"`
			}
			if err := decodeJSONBody(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			if err := store.SetDefaultPlanSlug(r.Context(), db, payload.Slug); err != nil {
				writeStoreError(w, err)
				return
			}
			plan, err := store.DefaultPlan(r.Context(), db)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, plan)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/plans/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		ref := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/plans/"))
		if ref == "" || strings.Contains(ref, "/") {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		plan, err := planByRef(r, db, ref)
		if err != nil {
			if errors.Is(err, store.ErrPlanNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeStoreError(w, err)
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, plan)
		case http.MethodPut:
			var payload planUpdateRequest
			if err := decodeJSONBody(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			params := store.PlanUpdateParams{
				Slug:                 payload.Slug,
				Name:                 payload.Name,
				Description:          payload.Description,
				MaxProjects:          payload.MaxProjects,
				MaxPrivateProjects:   payload.MaxPrivateProjects,
				MaxTickets:           payload.MaxTickets,
				MaxTicketsPerProject: payload.MaxTicketsPerProject,
				MaxTeamMemberships:   payload.MaxTeamMemberships,
				MaxAPICallsPerDay:    payload.MaxAPICallsPerDay,
				DefaultProjectAlias:  payload.DefaultProjectAlias,
			}
			if payload.RegistrationActions != nil {
				params.RegistrationActions = *payload.RegistrationActions
				params.ReplaceRegistrationAction = true
			}
			updated, err := store.UpdatePlan(r.Context(), db, plan.ID, params)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}

func planByRef(r *http.Request, db *sql.DB, ref string) (store.Plan, error) {
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		return store.GetPlanByID(r.Context(), db, id)
	}
	return store.GetPlanBySlug(r.Context(), db, ref)
}

func decodeJSONBody(r *http.Request, target any) error {
	return json.NewDecoder(r.Body).Decode(target)
}
