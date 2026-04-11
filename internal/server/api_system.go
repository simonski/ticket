package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func (r *router) registerSystemHandlers() {
	db := r.db
	mux := r.mux
	version := r.version

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

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireUser(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)

		var ticketCount, projectCount, userCount int
		_ = db.QueryRow(`SELECT COUNT(*) FROM tickets WHERE open = 1`).Scan(&ticketCount)
		_ = db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&projectCount)
		_ = db.QueryRow(`SELECT COUNT(*) FROM users WHERE user_type = 'user' OR user_type = '' OR user_type IS NULL`).Scan(&userCount)

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprintf(w, "# HELP ticket_up Is the ticket server running\n")
		fmt.Fprintf(w, "# TYPE ticket_up gauge\n")
		fmt.Fprintf(w, "ticket_up 1\n")

		fmt.Fprintf(w, "# HELP ticket_open_tickets_total Number of open tickets\n")
		fmt.Fprintf(w, "# TYPE ticket_open_tickets_total gauge\n")
		fmt.Fprintf(w, "ticket_open_tickets_total %d\n", ticketCount)

		fmt.Fprintf(w, "# HELP ticket_projects_total Number of projects\n")
		fmt.Fprintf(w, "# TYPE ticket_projects_total gauge\n")
		fmt.Fprintf(w, "ticket_projects_total %d\n", projectCount)

		fmt.Fprintf(w, "# HELP ticket_users_total Number of users\n")
		fmt.Fprintf(w, "# TYPE ticket_users_total gauge\n")
		fmt.Fprintf(w, "ticket_users_total %d\n", userCount)

		fmt.Fprintf(w, "# HELP go_goroutines Number of goroutines\n")
		fmt.Fprintf(w, "# TYPE go_goroutines gauge\n")
		fmt.Fprintf(w, "go_goroutines %d\n", runtime.NumGoroutine())

		fmt.Fprintf(w, "# HELP go_memstats_alloc_bytes Bytes currently allocated\n")
		fmt.Fprintf(w, "# TYPE go_memstats_alloc_bytes gauge\n")
		fmt.Fprintf(w, "go_memstats_alloc_bytes %d\n", ms.Alloc)

		fmt.Fprintf(w, "# HELP go_memstats_sys_bytes Bytes obtained from system\n")
		fmt.Fprintf(w, "# TYPE go_memstats_sys_bytes gauge\n")
		fmt.Fprintf(w, "go_memstats_sys_bytes %d\n", ms.Sys)
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
		if err := store.SetRegistrationEnabled(r.Context(), db, payload.Enabled); err != nil {
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
		if err := store.SetChatLimitsConfig(r.Context(), db, payload.MaxConnections, payload.MaxDurationMin); err != nil {
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
		if err := store.SetChatEnabled(r.Context(), db, payload.Enabled); err != nil {
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
		summary, err := store.CountEverything(r.Context(), db, projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, summary)
	})
}
