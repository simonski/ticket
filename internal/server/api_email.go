package server

import (
	"net/http"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

// registerEmailHandlers wires the SMTP email-sender configuration API (TK-132).
// All endpoints are admin-only. The password is never returned to clients —
// only a has_password flag — and a save that omits the password preserves the
// stored secret.
func (r *router) registerEmailHandlers() {
	db := r.db
	mux := r.mux

	mux.HandleFunc("/api/email/settings", func(w http.ResponseWriter, req *http.Request) {
		if _, err := requireAdmin(db, req); err != nil {
			writeAuthError(w, err)
			return
		}
		switch req.Method {
		case http.MethodGet:
			cfg, gerr := store.GetEmailConfig(req.Context(), db)
			if gerr != nil {
				writeStoreError(w, gerr)
				return
			}
			writeJSON(w, http.StatusOK, emailConfigPayload(cfg))
		case http.MethodPut:
			var body struct {
				Enabled     bool   `json:"enabled"`
				Host        string `json:"host"`
				Port        int    `json:"port"`
				Username    string `json:"username"`
				Password    string `json:"password"`
				FromAddress string `json:"from_address"`
				FromName    string `json:"from_name"`
				Security    string `json:"security"`
			}
			if derr := decodeJSONBody(req, &body); derr != nil {
				writeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			cfg := store.EmailConfig{
				Enabled:     body.Enabled,
				Host:        body.Host,
				Port:        body.Port,
				Username:    body.Username,
				Password:    body.Password,
				FromAddress: body.FromAddress,
				FromName:    body.FromName,
				Security:    body.Security,
			}
			updatePassword := strings.TrimSpace(body.Password) != ""
			if serr := store.SetEmailConfig(req.Context(), db, cfg, updatePassword); serr != nil {
				writeStoreError(w, serr)
				return
			}
			saved, _ := store.GetEmailConfig(req.Context(), db)
			writeJSON(w, http.StatusOK, emailConfigPayload(saved))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/email/enabled", func(w http.ResponseWriter, req *http.Request) {
		if _, err := requireAdmin(db, req); err != nil {
			writeAuthError(w, err)
			return
		}
		if req.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if derr := decodeJSONBody(req, &body); derr != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if serr := store.SetEmailEnabled(req.Context(), db, body.Enabled); serr != nil {
			writeStoreError(w, serr)
			return
		}
		saved, _ := store.GetEmailConfig(req.Context(), db)
		writeJSON(w, http.StatusOK, emailConfigPayload(saved))
	})
}

// emailConfigPayload renders the config for clients with the password masked.
func emailConfigPayload(cfg store.EmailConfig) map[string]any {
	return map[string]any{
		"enabled":      cfg.Enabled,
		"host":         cfg.Host,
		"port":         cfg.Port,
		"username":     cfg.Username,
		"from_address": cfg.FromAddress,
		"from_name":    cfg.FromName,
		"security":     cfg.Security,
		"has_password": cfg.HasPassword(),
	}
}
