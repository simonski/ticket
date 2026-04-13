package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func (r *router) registerAuthHandlers() {
	db := r.db
	mux := r.mux
	live := r.live
	version := r.version
	authLimiter := r.authLimiter
	chatLog := r.chatLog

	mux.HandleFunc("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			token = bearerToken(r)
		}
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if _, err := store.GetUserByToken(r.Context(), db, token); err != nil {
			writeAuthError(w, err)
			return
		}
		if err := websocketServe(live, w, r); err != nil {
			if strings.Contains(err.Error(), "websocket") || strings.Contains(err.Error(), "upgrade") {
				writeStoreError(w, err)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	})
	mux.HandleFunc("/api/chat/ws", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			token = bearerToken(r)
		}
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if _, err := store.GetUserByToken(r.Context(), db, token); err != nil {
			writeAuthError(w, err)
			return
		}
		chatEnabled, err := store.ChatEnabled(r.Context(), db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !chatEnabled {
			writeError(w, http.StatusForbidden, "chat is disabled")
			return
		}
		if err := websocketServeChat(w, r, db, chatLog); err != nil {
			if strings.Contains(err.Error(), "websocket") || strings.Contains(err.Error(), "upgrade") {
				writeStoreError(w, err)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	})

	mux.HandleFunc("/api/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !authLimiter.allow(clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many requests")
			return
		}
		enabled, err := store.RegistrationEnabled(r.Context(), db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !enabled {
			writeError(w, http.StatusForbidden, "registration is disabled")
			return
		}
		var credentials credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		user, err := store.RegisterUser(r.Context(), db, credentials.Username, credentials.Password)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, user)
	})

	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !authLimiter.allow(clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many requests")
			return
		}
		var credentials credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		user, err := store.AuthenticateUser(r.Context(), db, credentials.Username, credentials.Password)
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
		token, err := store.CreateSession(r.Context(), db, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		cookieName := sessionCookieName(r)
		http.SetCookie(w, &http.Cookie{ // #nosec G124 -- Secure is set for TLS or trusted TLS-terminating proxies
			Name:     cookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   requestIsSecure(r),
			SameSite: http.SameSiteLaxMode,
			MaxAge:   60 * 60 * 24 * 30,
		})
		writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
	})

	mux.HandleFunc("/api/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		token := bearerToken(r)
		if err := store.DeleteSession(r.Context(), db, token); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		secure := requestIsSecure(r)
		for _, cookieName := range []string{hostSessionCookieName, legacySessionCookieName} {
			http.SetCookie(w, &http.Cookie{ // #nosec G124 -- Secure is set for TLS or trusted TLS-terminating proxies
				Name:     cookieName,
				Value:    "",
				Path:     "/",
				HttpOnly: true,
				Secure:   secure || cookieName == hostSessionCookieName,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   -1,
			})
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		registrationEnabled, regErr := store.RegistrationEnabled(r.Context(), db)
		if regErr != nil {
			writeError(w, http.StatusInternalServerError, regErr.Error())
			return
		}
		chatLimits, chatErr := store.ChatLimitsConfig(r.Context(), db)
		if chatErr != nil {
			writeError(w, http.StatusInternalServerError, chatErr.Error())
			return
		}
		chatEnabled, chatEnabledErr := store.ChatEnabled(r.Context(), db)
		if chatEnabledErr != nil {
			writeError(w, http.StatusInternalServerError, chatEnabledErr.Error())
			return
		}
		runningChats := sharedChatRuntime.runningProcessCount()
		user, err := userFromRequest(db, r)
		if err != nil {
			if errors.Is(err, store.ErrUnauthorized) {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":                    "ok",
					"authenticated":             false,
					"registration_enabled":      registrationEnabled,
					"chat_enabled":              chatEnabled,
					"chat_max_connections":      chatLimits.MaxConnections,
					"chat_max_duration_minutes": chatLimits.MaxDurationMin,
					"chat_running_processes":    runningChats,
					"server_version":            version,
				})
				return
			}
			writeAuthError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                    "ok",
			"authenticated":             true,
			"registration_enabled":      registrationEnabled,
			"chat_enabled":              chatEnabled,
			"chat_max_connections":      chatLimits.MaxConnections,
			"chat_max_duration_minutes": chatLimits.MaxDurationMin,
			"chat_running_processes":    runningChats,
			"server_version":            version,
			"user":                      user,
		})
	})
}
