package server

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
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

func New(addr string, db *sql.DB, version string, verbose bool, output io.Writer, staticPath, siteName string) (*Server, error) {
	handler, err := Handler(db, version, verbose, output, staticPath, siteName)
	if err != nil {
		return nil, err
	}
	s := &Server{
		httpServer: &http.Server{ //nolint:gosec // bind address is caller-controlled configuration, not untrusted input
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
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		n, err := store.ReapStaleAgents(ctx, db, thresholdMinutes)
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
		sessionCtx, cancelSessions := context.WithTimeout(context.Background(), 30*time.Second)
		if n, err := store.PurgeExpiredSessions(sessionCtx, db); err != nil {
			slog.Error("session purge error", "error", err)
		} else if n > 0 && verbose {
			slog.Info("purged expired sessions", "count", n)
		}
		cancelSessions()

		retentionDays := 0
		if v := os.Getenv("TICKET_HISTORY_RETENTION_DAYS"); v != "" {
			if _, err := fmt.Sscan(v, &retentionDays); err != nil {
				slog.Error("invalid TICKET_HISTORY_RETENTION_DAYS", "value", v) // #nosec G706 -- env var is numeric config, not user-controlled log data
			}
		}
		historyCtx, cancelHistory := context.WithTimeout(context.Background(), 30*time.Second)
		if n, err := store.PurgeOldHistory(historyCtx, db, retentionDays); err != nil {
			slog.Error("history purge error", "error", err)
		} else if n > 0 && verbose {
			slog.Info("purged old history events", "count", n, "retention_days", retentionDays)
		}
		cancelHistory()
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

func Handler(db *sql.DB, version string, verbose bool, output io.Writer, staticPath, siteName string) (http.Handler, error) {
	return handlerWithPasskeyFactory(db, version, verbose, output, staticPath, siteName, defaultPasskeyServiceFactory)
}

func handlerWithPasskeyFactory(db *sql.DB, version string, verbose bool, output io.Writer, staticPath, siteName string, passkeys passkeyServiceFactory) (http.Handler, error) {
	var staticFS fs.FS
	if staticPath != "" {
		staticFS = os.DirFS(staticPath)
	} else {
		var err error
		staticFS, err = web.SiteFS(siteName)
		if err != nil {
			return nil, err
		}
	}

	mux := http.NewServeMux()
	live := newLiveHub()
	registerAPI(mux, db, version, live, verbose, output, passkeys)

	fileServer := staticCacheHeadersMiddleware(http.FileServer(http.FS(staticFS)))
	mux.Handle("/", spaHandler(fileServer, staticFS))

	var handler http.Handler = mux
	handler = csrfMiddleware(handler)
	handler = writeThrottleMiddleware(handler, db, newRateLimiter(writeRateLimitFromEnv(), time.Minute))
	handler = requestTimeoutMiddleware(handler, requestTimeoutFromEnv())
	handler = recoverMiddleware(handler)
	handler = bodySizeLimitHandler(handler)
	handler = securityHeadersHandler(handler)
	if verbose {
		logOutput := output
		if logOutput == nil {
			logOutput = os.Stderr
		}
		handler = loggingHandler(handler, logOutput)
	}
	handler = compressionMiddleware(handler)
	return handler, nil
}

func writeRateLimitFromEnv() int {
	const defaultLimit = 120
	raw := strings.TrimSpace(os.Getenv("TICKET_WRITE_RATE_LIMIT"))
	if raw == "" {
		return defaultLimit
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultLimit
	}
	return value
}

func writeThrottleMiddleware(next http.Handler, db *sql.DB, limiter *rateLimiter) http.Handler {
	writeExemptPaths := map[string]bool{
		"/api/login":           true,
		"/api/register":        true,
		"/api/agents/register": true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limiter == nil || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if writeExemptPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			// continue
		default:
			next.ServeHTTP(w, r)
			return
		}

		key := "ip:" + clientIP(r)
		if token := bearerToken(r); token != "" {
			if user, err := store.GetUserByToken(r.Context(), db, token); err == nil && strings.TrimSpace(user.ID) != "" {
				key = "user:" + strings.TrimSpace(user.ID)
			}
		}

		if !limiter.allow(key) {
			writeError(w, http.StatusTooManyRequests, "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestTimeoutFromEnv() time.Duration {
	const defaultTimeout = 30 * time.Second
	raw := strings.TrimSpace(os.Getenv("TICKET_REQUEST_TIMEOUT_SECONDS"))
	if raw == "" {
		return defaultTimeout
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return defaultTimeout
	}
	return time.Duration(seconds) * time.Second
}

func requestTimeoutMiddleware(next http.Handler, timeout time.Duration) http.Handler {
	if timeout <= 0 {
		return next
	}
	timeoutHandler := http.TimeoutHandler(next, timeout, `{"error":"request timeout"}`)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ws", "/api/chat/ws":
			next.ServeHTTP(w, r)
			return
		default:
			timeoutHandler.ServeHTTP(w, r)
		}
	})
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
		nonce := cspNonce()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		if requestIsSecure(r) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		w.Header().Set("Content-Security-Policy",
			fmt.Sprintf("default-src 'self'; script-src 'self' 'nonce-%s'; style-src 'self' 'nonce-%s'", nonce, nonce))
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), cspNonceKey{}, nonce)))
	})
}

type cspNonceKey struct{}

func cspNonceFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if nonce, ok := ctx.Value(cspNonceKey{}).(string); ok {
		return nonce
	}
	return ""
}

func cspNonce() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(buf)
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

func compressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead || !requestAcceptsGzip(r) || isUpgradeRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		addVaryHeader(w.Header(), "Accept-Encoding")
		gzw := &gzipResponseWriter{ResponseWriter: w}
		defer gzw.Close()
		next.ServeHTTP(gzw, r)
	})
}

func requestAcceptsGzip(r *http.Request) bool {
	for _, value := range r.Header.Values("Accept-Encoding") {
		for _, encoding := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(encoding), "gzip") {
				return true
			}
		}
	}
	return false
}

func isUpgradeRequest(r *http.Request) bool {
	for _, value := range r.Header.Values("Connection") {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), "upgrade") {
				return true
			}
		}
	}
	return false
}

func addVaryHeader(header http.Header, value string) {
	for _, existing := range header.Values("Vary") {
		for _, part := range strings.Split(existing, ",") {
			if strings.EqualFold(strings.TrimSpace(part), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer     *gzip.Writer
	statusCode int
	decided    bool
	compress   bool
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	w.ensureWriter(statusCode)
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	if !w.decided {
		w.ensureWriter(http.StatusOK)
		w.ResponseWriter.WriteHeader(http.StatusOK)
	}
	if !w.compress {
		return w.ResponseWriter.Write(p)
	}
	return w.writer.Write(p)
}

func (w *gzipResponseWriter) Flush() {
	if !w.decided {
		w.ensureWriter(http.StatusOK)
		w.ResponseWriter.WriteHeader(http.StatusOK)
	}
	if w.writer != nil {
		_ = w.writer.Flush()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, errors.New("response writer does not support hijacking")
}

func (w *gzipResponseWriter) Close() error {
	if w.writer != nil {
		return w.writer.Close()
	}
	return nil
}

func (w *gzipResponseWriter) ensureWriter(statusCode int) {
	if w.decided {
		return
	}
	w.decided = true
	w.statusCode = statusCode
	if statusCode == http.StatusNoContent || statusCode == http.StatusNotModified || statusCode == http.StatusSwitchingProtocols {
		return
	}
	if w.Header().Get("Content-Encoding") != "" {
		return
	}
	w.compress = true
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Del("Content-Length")
	w.writer = gzip.NewWriter(w.ResponseWriter)
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
			data, err := fs.ReadFile(staticFS, "index.html")
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			nonce := cspNonceFromContext(r.Context())
			indexHTML := string(data)
			if nonce != "" {
				indexHTML = strings.Replace(indexHTML, "<style>", fmt.Sprintf(`<style nonce=%q>`, nonce), 1)
				indexHTML = strings.Replace(indexHTML, "<script>", fmt.Sprintf(`<script nonce=%q>`, nonce), 1)
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			if _, err := io.WriteString(w, indexHTML); err != nil {
				slog.Error("write spa index response", "error", err)
			}
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

func staticCacheHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := strings.ToLower(path.Ext(r.URL.Path))
		switch ext {
		case ".js", ".css", ".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".woff", ".woff2", ".ttf":
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		next.ServeHTTP(w, r)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status        int
	body          bytes.Buffer
	bodyTruncated bool
}

const maxLoggedBodyBytes = 4096

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	appendLoggedBody(&w.body, p, &w.bodyTruncated)
	return w.ResponseWriter.Write(p)
}

func (w *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("response writer does not support hijacking")
}

func appendLoggedBody(buf *bytes.Buffer, payload []byte, truncated *bool) {
	if len(payload) == 0 || buf.Len() >= maxLoggedBodyBytes {
		if len(payload) > 0 && truncated != nil {
			*truncated = true
		}
		return
	}
	remaining := maxLoggedBodyBytes - buf.Len()
	if len(payload) > remaining {
		buf.Write(payload[:remaining])
		if truncated != nil {
			*truncated = true
		}
		return
	}
	buf.Write(payload)
}

func loggedBodyString(buf *bytes.Buffer, truncated bool) string {
	if buf == nil || buf.Len() == 0 {
		return ""
	}
	if !truncated {
		return buf.String()
	}
	return buf.String() + "…(truncated)"
}

func compactLogLine(ts time.Time, level, method, path string, status int, durationMS int64, requestID, query, requestBody, responseBody string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s path=%s status=%d duration_ms=%d request_id=%s", ts.Format(time.RFC3339Nano), level, method, path, status, durationMS, requestID)
	if query != "" {
		fmt.Fprintf(&b, " query=%q", query)
	}
	if requestBody != "" {
		fmt.Fprintf(&b, " request_body=%q", requestBody)
	}
	if responseBody != "" {
		fmt.Fprintf(&b, " response_body=%q", responseBody)
	}
	b.WriteByte('\n')
	return b.String()
}

func loggingHandler(next http.Handler, output io.Writer) http.Handler {
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
			strings.Contains(r.URL.Path, "/reset-password") ||
			strings.HasPrefix(r.URL.Path, "/api/agents")

		var requestBody bytes.Buffer
		var requestBodyTruncated bool
		if r.Body != nil && !sensitiveEndpoint {
			rawRequestBody, _ := io.ReadAll(r.Body)
			appendLoggedBody(&requestBody, rawRequestBody, &requestBodyTruncated)
			r.Body = io.NopCloser(bytes.NewReader(rawRequestBody))
		}

		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(lw, r)
		if lw.status == 0 {
			lw.status = http.StatusOK
		}

		level := "INFO"
		if lw.status >= 500 {
			level = "ERROR"
		}
		query := r.URL.RawQuery
		requestBodyString := ""
		if requestBody.Len() > 0 {
			requestBodyString = loggedBodyString(&requestBody, requestBodyTruncated)
		}
		responseBodyString := ""
		if lw.body.Len() > 0 && !sensitiveEndpoint {
			responseBodyString = loggedBodyString(&lw.body, lw.bodyTruncated)
		}
		if output != nil {
			_, _ = io.WriteString(output, compactLogLine(
				time.Now(),
				level,
				r.Method,
				r.URL.Path,
				lw.status,
				time.Since(start).Milliseconds(),
				requestID,
				query,
				requestBodyString,
				responseBodyString,
			))
		}
	})
}

// csrfMiddleware implements a double-submit cookie pattern for CSRF protection.
// Safe methods (GET/HEAD/OPTIONS) set a CSRF token cookie if absent.
// Mutating methods (POST/PUT/DELETE) require the X-CSRF-Token header to match
// the _csrf cookie value. Requests with a Bearer token (API auth) and the
// login/register endpoints are exempt.
func csrfMiddleware(next http.Handler) http.Handler {
	const headerName = "X-CSRF-Token"

	csrfExemptPaths := map[string]bool{
		"/api/login":           true,
		"/api/register":        true,
		"/api/agents/register": true,
	}

	generateToken := func() string {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			slog.Error("generate csrf token", "error", err)
			return strings.ReplaceAll(uuid.NewString(), "-", "")
		}
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
			cookieName := csrfCookieName(r)
			// Set CSRF cookie if not present.
			if _, err := r.Cookie(cookieName); err != nil {
				http.SetCookie(w, &http.Cookie{ // #nosec G124 -- CSRF double-submit requires JS-readable cookie (HttpOnly=false)
					Name:     cookieName,
					Value:    generateToken(),
					Path:     "/",
					HttpOnly: false, // JS must read it
					SameSite: http.SameSiteStrictMode,
					Secure:   requestIsSecure(r),
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
		if _, err := r.Cookie(hostSessionCookieName); err != nil {
			if _, legacyErr := r.Cookie(legacySessionCookieName); legacyErr != nil {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Validate double-submit: cookie must match header.
		cookie, err := r.Cookie(csrfCookieName(r))
		if err != nil || cookie.Value == "" {
			// Fallback to legacy cookie name for clients that still carry it.
			cookie, err = r.Cookie(legacyCSRFCookieName)
		}
		if err != nil || cookie.Value == "" {
			// Fallback to host-prefixed cookie when request arrived insecurely via proxy.
			cookie, err = r.Cookie(hostCSRFCookieName)
		}
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

// requestIsSecure reports whether the inbound request should be treated as HTTPS.
// It supports TLS-terminating proxies by honoring X-Forwarded-Proto=https.
func requestIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		return false
	}
	if comma := strings.Index(proto, ","); comma >= 0 {
		proto = strings.TrimSpace(proto[:comma])
	}
	return strings.EqualFold(proto, "https")
}
