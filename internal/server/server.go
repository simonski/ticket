package server

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"net/http"

	web "github.com/simonski/ticket/web"
)

type Server struct {
	httpServer *http.Server
}

func New(addr string, db *sql.DB, version string, verbose bool, output io.Writer) (*Server, error) {
	handler, err := Handler(db, version, verbose, output)
	if err != nil {
		return nil, err
	}
	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}, nil
}

func Handler(db *sql.DB, version string, verbose bool, output io.Writer) (http.Handler, error) {
	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	registerAPI(mux, db, version)

	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/", spaHandler(fileServer, staticFS))

	var handler http.Handler = mux
	if verbose {
		handler = loggingHandler(handler, output)
	}
	return handler, nil
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

func loggingHandler(next http.Handler, output io.Writer) http.Handler {
	if output == nil {
		output = io.Discard
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestBody []byte
		if r.Body != nil {
			requestBody, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(requestBody))
		}

		lw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(lw, r)
		if lw.status == 0 {
			lw.status = http.StatusOK
		}

		fmt.Fprintf(output, "REQUEST %s %s\n", r.Method, r.URL.String())
		if len(requestBody) > 0 {
			fmt.Fprintf(output, "request body: %s\n", string(requestBody))
		}
		fmt.Fprintf(output, "RESPONSE %d\n", lw.status)
		if lw.body.Len() > 0 {
			fmt.Fprintf(output, "response body: %s\n", lw.body.String())
		}
	})
}
