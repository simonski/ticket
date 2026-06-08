package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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
		policy, policyErr := store.GetAutomationPolicy(r.Context(), db)
		if policyErr != nil {
			writeStoreError(w, policyErr)
			return
		}
		var queueReady int
		_ = db.QueryRow(`
			SELECT COUNT(*)
			FROM tickets
			WHERE deleted = 0 AND archived = 0 AND complete = 0 AND draft = 0
			  AND (assignee IS NULL OR TRIM(assignee) = '')
			  AND LOWER(COALESCE(state, '')) <> 'fail'
		`).Scan(&queueReady)
		var interventionOpen int
		_ = db.QueryRow(`
			SELECT COUNT(*)
			FROM tickets
			WHERE deleted = 0 AND archived = 0 AND complete = 0
			  AND LOWER(COALESCE(state, '')) = 'fail'
		`).Scan(&interventionOpen)
		writeJSON(w, http.StatusOK, map[string]string{
			"status":              "ok",
			"version":             version,
			"queue_strategy":      policy.QueueStrategy,
			"queue_ready_total":   fmt.Sprintf("%d", queueReady),
			"interventions_total": fmt.Sprintf("%d", interventionOpen),
		})
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
		var queueReadyCount, interventionOpenCount, forecastSnapshotCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM tickets WHERE open = 1`).Scan(&ticketCount); err != nil {
			slog.Error("load ticket count metric", "error", err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&projectCount); err != nil {
			slog.Error("load project count metric", "error", err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE user_type = 'user' OR user_type = '' OR user_type IS NULL`).Scan(&userCount); err != nil {
			slog.Error("load user count metric", "error", err)
		}
		if err := db.QueryRow(`
			SELECT COUNT(*)
			FROM tickets
			WHERE deleted = 0 AND archived = 0 AND complete = 0 AND draft = 0
			  AND (assignee IS NULL OR TRIM(assignee) = '')
			  AND LOWER(COALESCE(state, '')) <> 'fail'
		`).Scan(&queueReadyCount); err != nil {
			slog.Error("load queue ready metric", "error", err)
		}
		if err := db.QueryRow(`
			SELECT COUNT(*)
			FROM tickets
			WHERE deleted = 0 AND archived = 0 AND complete = 0
			  AND LOWER(COALESCE(state, '')) = 'fail'
		`).Scan(&interventionOpenCount); err != nil {
			slog.Error("load intervention metric", "error", err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM forecast_snapshots`).Scan(&forecastSnapshotCount); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "no such table") {
				slog.Error("load forecast snapshot metric", "error", err)
			}
		}

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

		fmt.Fprintf(w, "# HELP ticket_work_item_queue_ready_total Number of queue-ready work items\n")
		fmt.Fprintf(w, "# TYPE ticket_work_item_queue_ready_total gauge\n")
		fmt.Fprintf(w, "ticket_work_item_queue_ready_total %d\n", queueReadyCount)

		fmt.Fprintf(w, "# HELP ticket_interventions_open_total Number of open interventions\n")
		fmt.Fprintf(w, "# TYPE ticket_interventions_open_total gauge\n")
		fmt.Fprintf(w, "ticket_interventions_open_total %d\n", interventionOpenCount)

		fmt.Fprintf(w, "# HELP ticket_forecast_snapshots_total Number of persisted forecast snapshots\n")
		fmt.Fprintf(w, "# TYPE ticket_forecast_snapshots_total gauge\n")
		fmt.Fprintf(w, "ticket_forecast_snapshots_total %d\n", forecastSnapshotCount)

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
			Enabled     bool  `json:"enabled"`
			AutoApprove *bool `json:"auto_approve,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if err := store.SetRegistrationEnabled(r.Context(), db, payload.Enabled); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var autoApprove bool
		if payload.AutoApprove == nil {
			currentAutoApprove, err := store.RegistrationAutoApprove(r.Context(), db)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			autoApprove = currentAutoApprove
		} else {
			autoApprove = *payload.AutoApprove
			if err := store.SetRegistrationAutoApprove(r.Context(), db, autoApprove); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"registration_enabled":      payload.Enabled,
			"registration_auto_approve": autoApprove,
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
	mux.HandleFunc("/api/config/orchestrator", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireUser(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			interval, err := store.OrchestratorIntervalSeconds(r.Context(), db)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			timeout, err := store.OrchestratorHeartbeatTimeoutSeconds(r.Context(), db)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"interval_seconds":          interval,
				"heartbeat_timeout_seconds": timeout,
			})
		case http.MethodPost:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var payload struct {
				IntervalSeconds         int `json:"interval_seconds"`
				HeartbeatTimeoutSeconds int `json:"heartbeat_timeout_seconds"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			if err := store.SetOrchestratorIntervalSeconds(r.Context(), db, payload.IntervalSeconds); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if err := store.SetOrchestratorHeartbeatTimeoutSeconds(r.Context(), db, payload.HeartbeatTimeoutSeconds); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			interval, _ := store.OrchestratorIntervalSeconds(r.Context(), db)
			timeout, _ := store.OrchestratorHeartbeatTimeoutSeconds(r.Context(), db)
			writeJSON(w, http.StatusOK, map[string]any{
				"interval_seconds":          interval,
				"heartbeat_timeout_seconds": timeout,
			})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
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
	mux.HandleFunc("/api/config/agent-model", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireUser(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			cfg, err := store.SystemAgentModelConfig(r.Context(), db)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, cfg)
		case http.MethodPut:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var payload agentModelConfigRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			if err := store.SetSystemAgentModelConfig(r.Context(), db, store.AgentModelConfig{
				Provider:  payload.Provider,
				Model:     payload.Model,
				URL:       payload.URL,
				APIKey:    payload.APIKey,
				Providers: payload.Providers,
			}); err != nil {
				writeStoreError(w, err)
				return
			}
			cfg, err := store.SystemAgentModelConfig(r.Context(), db)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, cfg)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
	mux.HandleFunc("/api/config/settings", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		switch r.Method {
		case http.MethodGet:
			settings, err := store.ListAppSettings(r.Context(), db)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, settings)
		case http.MethodPost:
			var payload store.AppSetting
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			payload.Key = strings.TrimSpace(payload.Key)
			if payload.Key == "" {
				writeError(w, http.StatusBadRequest, "config key is required")
				return
			}
			if err := store.SetAppSetting(r.Context(), db, payload.Key, payload.Value); err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, payload)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
	mux.HandleFunc("/api/config/settings/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireAdmin(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		rawKey := strings.TrimPrefix(r.URL.Path, "/api/config/settings/")
		key, err := url.PathUnescape(rawKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid config key")
			return
		}
		key = strings.TrimSpace(key)
		if key == "" {
			writeError(w, http.StatusBadRequest, "config key is required")
			return
		}
		switch r.Method {
		case http.MethodPut:
			var payload store.AppSetting
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			nextKey := strings.TrimSpace(payload.Key)
			if nextKey == "" {
				nextKey = key
			}
			if err := store.SetAppSetting(r.Context(), db, nextKey, payload.Value); err != nil {
				writeStoreError(w, err)
				return
			}
			if nextKey != key {
				if err := store.DeleteAppSetting(r.Context(), db, key); err != nil {
					writeStoreError(w, err)
					return
				}
			}
			writeJSON(w, http.StatusOK, store.AppSetting{Key: nextKey, Value: payload.Value})
		case http.MethodDelete:
			if err := store.DeleteAppSetting(r.Context(), db, key); err != nil {
				writeStoreError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
	mux.HandleFunc("/api/config/automation_policy", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireUser(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			policy, err := store.GetAutomationPolicy(r.Context(), db)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, policy)
		case http.MethodPut:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var payload store.AutomationPolicy
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			policy, err := store.SetAutomationPolicy(r.Context(), db, payload)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, policy)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
	mux.HandleFunc("/api/tickets/policy/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireUser(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		ticketID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/tickets/policy/"))
		if ticketID == "" {
			writeError(w, http.StatusBadRequest, "ticket id is required")
			return
		}
		diag, err := store.DiagnoseTicketPolicy(r.Context(), db, ticketID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, diag)
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
