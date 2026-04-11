package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/simonski/ticket/internal/store"
	web "github.com/simonski/ticket/web"
)

type Server struct {
	httpServer *http.Server
	stopReaper chan struct{}
}

func New(addr string, db *sql.DB, version string, verbose bool, output io.Writer, staticPath string) (*Server, error) {
	handler, err := Handler(db, version, verbose, output, staticPath)
	if err != nil {
		return nil, err
	}
	s := &Server{
		httpServer: &http.Server{ //nolint:gosec
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 30 * time.Second,
			ReadTimeout:       60 * time.Second,
			// WriteTimeout is intentionally omitted: WebSocket connections are
			// long-lived and a write timeout would kill them mid-stream.
			IdleTimeout: 120 * time.Second,
		},
		stopReaper: make(chan struct{}),
	}
	go s.runAgentReaper(db, verbose, output)
	go s.runRetentionPurge(db, verbose)
	return s, nil
}

// runAgentReaper periodically marks agents as idle if they haven't sent a
// heartbeat (TouchAgent) within the threshold.
func (s *Server) runAgentReaper(db *sql.DB, verbose bool, output io.Writer) {
	const thresholdMinutes = 10

	reap := func() {
		n, err := store.ReapStaleAgents(context.Background(), db, thresholdMinutes)
		if err != nil && verbose {
			slog.Error("agent reaper error", "error", err)
		}
		if n > 0 && verbose {
			slog.Info("agent reaper reaped stale agents", "count", n)
		}
	}

	// Run immediately on startup.
	reap()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			reap()
		case <-s.stopReaper:
			return
		}
	}
}

// runRetentionPurge periodically deletes expired sessions and old history events.
// Retention period for history is controlled by the TICKET_HISTORY_RETENTION_DAYS
// environment variable (default: 0 = keep forever).
func (s *Server) runRetentionPurge(db *sql.DB, verbose bool) {
	purge := func() {
		if n, err := store.PurgeExpiredSessions(context.Background(), db); err != nil {
			slog.Error("session purge error", "error", err)
		} else if n > 0 && verbose {
			slog.Info("purged expired sessions", "count", n)
		}

		retentionDays := 0
		if v := os.Getenv("TICKET_HISTORY_RETENTION_DAYS"); v != "" {
			if _, err := fmt.Sscan(v, &retentionDays); err != nil {
				slog.Error("invalid TICKET_HISTORY_RETENTION_DAYS", "value", v) // #nosec G706 -- env var is numeric config, not user-controlled log data
			}
		}
		if n, err := store.PurgeOldHistory(context.Background(), db, retentionDays); err != nil {
			slog.Error("history purge error", "error", err)
		} else if n > 0 && verbose {
			slog.Info("purged old history events", "count", n, "retention_days", retentionDays)
		}
	}

	// Run once at startup, then daily.
	purge()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			purge()
		case <-s.stopReaper:
			return
		}
	}
}

func Handler(db *sql.DB, version string, verbose bool, output io.Writer, staticPath string) (http.Handler, error) {
	var staticFS fs.FS
	if staticPath != "" {
		staticFS = os.DirFS(staticPath)
	} else {
		var err error
		staticFS, err = fs.Sub(web.Static, "static")
		if err != nil {
			return nil, err
		}
	}

	mux := http.NewServeMux()
	live := newLiveHub()
	registerAPI(mux, db, version, live, verbose, output)

	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/", spaHandler(fileServer, staticFS))

	var handler http.Handler = mux
	handler = csrfMiddleware(handler)
	handler = recoverMiddleware(handler)
	handler = bodySizeLimitHandler(handler)
	handler = securityHeadersHandler(handler)
	if verbose {
		logOutput := output
		if logOutput == nil {
			logOutput = os.Stderr
		}
		logger := slog.New(slog.NewTextHandler(logOutput, nil))
		handler = loggingHandler(handler, logger)
	}
	return handler, nil
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "error", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func securityHeadersHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

func bodySizeLimitHandler(next http.Handler) http.Handler {
	const maxBodySize = 1 << 20 // 1 MB
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server and stops background goroutines.
func (s *Server) Shutdown(ctx context.Context) error {
	close(s.stopReaper)
	sharedChatRuntime.stopHeartbeat()
	return s.httpServer.Shutdown(ctx)
}

func spaHandler(next http.Handler, staticFS fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(staticFS, r.URL.Path[1:]); err == nil {
			next.ServeHTTP(w, r)
			return
		}

		r.URL.Path = "/"
		next.ServeHTTP(w, r)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	_, _ = w.body.Write(p)
	return w.ResponseWriter.Write(p)
}

func (w *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("response writer does not support hijacking")
}

func loggingHandler(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Generate a request correlation ID.
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", requestID)

		// Skip logging request body for sensitive endpoints to avoid leaking credentials.
		sensitiveEndpoint := strings.Contains(r.URL.Path, "/login") ||
			strings.Contains(r.URL.Path, "/register") ||
			strings.Contains(r.URL.Path, "/reset-password")

		var requestBody []byte
		if r.Body != nil && !sensitiveEndpoint {
			requestBody, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(requestBody))
		}

		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(lw, r)
		if lw.status == 0 {
			lw.status = http.StatusOK
		}

		attrs := []any{
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", lw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		}
		if q := r.URL.RawQuery; q != "" {
			attrs = append(attrs, "query", q)
		}
		if len(requestBody) > 0 {
			attrs = append(attrs, "request_body", string(requestBody))
		}
		if lw.body.Len() > 0 {
			attrs = append(attrs, "response_body", lw.body.String())
		}
		if lw.status >= 500 {
			logger.Error("api request", attrs...)
		} else {
			logger.Info("api request", attrs...)
		}
	})
}

// csrfMiddleware implements a double-submit cookie pattern for CSRF protection.
// Safe methods (GET/HEAD/OPTIONS) set a CSRF token cookie if absent.
// Mutating methods (POST/PUT/DELETE) require the X-CSRF-Token header to match
// the _csrf cookie value. Requests with a Bearer token (API auth) and the
// login/register endpoints are exempt.
func csrfMiddleware(next http.Handler) http.Handler {
	const cookieName = "_csrf"
	const headerName = "X-CSRF-Token"

	csrfExemptPaths := map[string]bool{
		"/api/login":           true,
		"/api/register":        true,
		"/api/agents/register": true,
	}

	generateToken := func() string {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		return hex.EncodeToString(b)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply to /api/ paths.
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			// Set CSRF cookie if not present.
			if _, err := r.Cookie(cookieName); err != nil {
				http.SetCookie(w, &http.Cookie{
					Name:     cookieName,
					Value:    generateToken(),
					Path:     "/",
					HttpOnly: false, // JS must read it
					SameSite: http.SameSiteStrictMode,
					Secure:   r.TLS != nil,
				})
			}
			next.ServeHTTP(w, r)
			return
		}

		// Mutating method — check CSRF unless exempt.

		// Exempt paths (login/register — no cookie exists yet).
		if csrfExemptPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		// Exempt requests using token-based auth (Bearer or Basic).
		// These are programmatic API calls, not browser-initiated — CSRF
		// is a browser-only attack vector.
		if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") || strings.HasPrefix(authHeader, "Basic ") {
			next.ServeHTTP(w, r)
			return
		}

		// Exempt requests with no session cookie — they're API calls or
		// unauthenticated requests, not browser form submissions.
		if _, err := r.Cookie("session"); err != nil {
			next.ServeHTTP(w, r)
			return
		}

		// Validate double-submit: cookie must match header.
		cookie, err := r.Cookie(cookieName)
		if err != nil || cookie.Value == "" {
			http.Error(w, `{"error":"missing CSRF token"}`, http.StatusForbidden)
			return
		}
		headerVal := r.Header.Get(headerName)
		if headerVal == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(headerVal)) != 1 {
			http.Error(w, `{"error":"CSRF token mismatch"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
