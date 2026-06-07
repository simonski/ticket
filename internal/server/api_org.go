package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/simonski/ticket/internal/store"
)

func (r *router) registerOrgHandlers() {
	mux := r.mux
	db := r.db

	mux.HandleFunc("/api/org", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			_, err := requireUser(db, req)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			org, err := store.GetOrg(req.Context(), db)
			if err != nil {
				if errors.Is(err, store.ErrOrgNotFound) {
					writeJSON(w, http.StatusOK, store.Org{})
					return
				}
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, org)
		case http.MethodPut:
			user, err := requireUser(db, req)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			if user.Role != "admin" {
				writeError(w, http.StatusForbidden, "admin required")
				return
			}
			var payload struct {
				Name        string `json:"name"`
				Domain      string `json:"domain"`
				Description string `json:"description"`
				LogoURL     string `json:"logo_url"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			org, err := store.UpdateOrg(req.Context(), db, payload.Name, payload.Domain, payload.Description, payload.LogoURL)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, org)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}
