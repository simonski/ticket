package server

import (
	"bufio"
	"bytes"
	"database/sql"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

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
		httpServer: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
		stopReaper: make(chan struct{}),
	}
	go s.runAgentReaper(db, verbose, output)
	return s, nil
}

// runAgentReaper periodically marks agents as idle if they haven't sent a
// heartbeat (TouchAgent) within the threshold.
func (s *Server) runAgentReaper(db *sql.DB, verbose bool, output io.Writer) {
	const thresholdMinutes = 10

	reap := func() {
		n, err := store.ReapStaleAgents(db, thresholdMinutes)
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

func securityHeadersHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
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
		var requestBody []byte
		if r.Body != nil {
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
