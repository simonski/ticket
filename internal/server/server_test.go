package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/simonski/ticket/internal/static"
	"github.com/simonski/ticket/internal/store"
)

type hijackableResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
	conn   net.Conn
	rw     *bufio.ReadWriter
}

func (w *hijackableResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *hijackableResponseWriter) Write(p []byte) (int, error) {
	return w.body.Write(p)
}

func (w *hijackableResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *hijackableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, w.rw, nil
}

type flushTrackingResponseWriter struct {
	header  http.Header
	body    bytes.Buffer
	status  int
	flushed bool
}

type stubAddr string

func (a stubAddr) Network() string { return "test" }
func (a stubAddr) String() string  { return string(a) }

type stubConn struct{}

func (stubConn) Read(_ []byte) (int, error)         { return 0, io.EOF }
func (stubConn) Write(p []byte) (int, error)        { return len(p), nil }
func (stubConn) Close() error                       { return nil }
func (stubConn) LocalAddr() net.Addr                { return stubAddr("local") }
func (stubConn) RemoteAddr() net.Addr               { return stubAddr("remote") }
func (stubConn) SetDeadline(_ time.Time) error      { return nil }
func (stubConn) SetReadDeadline(_ time.Time) error  { return nil }
func (stubConn) SetWriteDeadline(_ time.Time) error { return nil }

func (w *flushTrackingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *flushTrackingResponseWriter) Write(p []byte) (int, error) {
	return w.body.Write(p)
}

func (w *flushTrackingResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *flushTrackingResponseWriter) Flush() {
	w.flushed = true
}

func TestRequestIsSecure(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	if requestIsSecure(req) {
		t.Fatal("requestIsSecure() = true, want false for plain HTTP")
	}

	req.Header.Set("X-Forwarded-Proto", "https")
	if !requestIsSecure(req) {
		t.Fatal("requestIsSecure() = false, want true for X-Forwarded-Proto=https")
	}

	req.Header.Set("X-Forwarded-Proto", "https, http")
	if !requestIsSecure(req) {
		t.Fatal("requestIsSecure() = false, want true for forwarded proto list")
	}
}

func TestRequestTimeoutFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "default when unset", value: "", want: 30 * time.Second},
		{name: "default when invalid", value: "abc", want: 30 * time.Second},
		{name: "default when non positive", value: "0", want: 30 * time.Second},
		{name: "custom seconds", value: "12", want: 12 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TICKET_REQUEST_TIMEOUT_SECONDS", tt.value)
			if got := requestTimeoutFromEnv(); got != tt.want {
				t.Fatalf("requestTimeoutFromEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteThrottleMiddlewareUsesPerUserKeys(t *testing.T) {
	t.Setenv("TICKET_WRITE_RATE_LIMIT", "1")

	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := store.Init(dbPath, "admin", "password", static.SeedDatabase); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()
	if _, err := store.CreateUser(context.Background(), db, "alice", "password", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	srv, err := New(":0", db, "1.2.3", false, nil, "", "")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := srv.httpServer.Handler

	login := func(username string) string {
		payload, _ := json.Marshal(map[string]string{"username": username, "password": "password"})
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("login(%s) status = %d body=%s", username, rec.Code, rec.Body.String())
		}
		var auth struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &auth); err != nil {
			t.Fatalf("json.Unmarshal(login %s) error = %v", username, err)
		}
		return auth.Token
	}

	adminToken := login("admin")
	aliceToken := login("alice")

	projectPayload, _ := json.Marshal(map[string]any{"title": "Admin Project", "prefix": "ADM"})
	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(projectPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin first write status = %d body=%s", rec.Code, rec.Body.String())
	}

	projectPayload2, _ := json.Marshal(map[string]any{"title": "Admin Project 2", "prefix": "AD2"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(projectPayload2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+adminToken)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("admin second write status = %d, want 429; body=%s", rec2.Code, rec2.Body.String())
	}

	projectPayload3, _ := json.Marshal(map[string]any{"title": "Alice Project", "prefix": "ALI"})
	req3 := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(projectPayload3))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer "+aliceToken)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code == http.StatusTooManyRequests {
		t.Fatalf("alice first write unexpectedly throttled: status=%d body=%s", rec3.Code, rec3.Body.String())
	}
}

func TestCSRFCookieSecureWithForwardedProto(t *testing.T) {
	t.Parallel()

	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	setCookie := resp.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "__Host-_csrf=") {
		t.Fatalf("Set-Cookie = %q, want CSRF cookie", setCookie)
	}
	if !strings.Contains(setCookie, "Secure") {
		t.Fatalf("Set-Cookie = %q, want Secure attribute", setCookie)
	}
}

func TestSecurityHeadersAddsHSTSWhenSecure(t *testing.T) {
	t.Parallel()

	handler := securityHeadersHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if got := resp.Header().Get("Strict-Transport-Security"); got == "" {
		t.Fatal("Strict-Transport-Security header = empty, want value for secure request")
	}
}

func TestSecurityHeadersInjectsNonceIntoContext(t *testing.T) {
	t.Parallel()

	var nonceFromHandler string
	handler := securityHeadersHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonceFromHandler = cspNonceFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if nonceFromHandler == "" {
		t.Fatal("cspNonceFromContext() = empty, want nonce from security headers middleware")
	}
	if !strings.Contains(resp.Header().Get("Content-Security-Policy"), nonceFromHandler) {
		t.Fatalf("Content-Security-Policy = %q, want nonce %q", resp.Header().Get("Content-Security-Policy"), nonceFromHandler)
	}
}

func TestCSPNonceFromContextNilAndMissing(t *testing.T) {
	t.Parallel()

	if got := cspNonceFromContext(nil); got != "" {
		t.Fatalf("cspNonceFromContext(nil) = %q, want empty", got)
	}
	if got := cspNonceFromContext(context.Background()); got != "" {
		t.Fatalf("cspNonceFromContext(background) = %q, want empty", got)
	}
}

func TestRequestTimeoutMiddlewareTimesOutNonWebSocket(t *testing.T) {
	t.Parallel()
	handler := requestTimeoutMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(60 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}), 20*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestRequestTimeoutMiddlewareBypassesWebSocketPaths(t *testing.T) {
	t.Parallel()
	handler := requestTimeoutMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(40 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}), 20*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRecoverMiddlewareReturnsInternalServerError(t *testing.T) {
	t.Parallel()

	handler := recoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Fatalf("body = %q, want internal server error", rec.Body.String())
	}
}

func TestServerServesHealthAndFrontend(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := store.Init(dbPath, "admin", "password", static.SeedDatabase); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	srv, err := New(":0", db, "1.2.3", false, nil, "", "")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/healthz")
	if err != nil {
		t.Fatalf("GET /api/healthz error = %v", err)
	}
	defer resp.Body.Close()

	var payload map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("health payload = %#v, want status ok", payload)
	}
	if payload["version"] != "1.2.3" {
		t.Fatalf("health payload version = %#v, want 1.2.3", payload)
	}

	rootResp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / error = %v", err)
	}
	defer rootResp.Body.Close()
	csp := rootResp.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self' 'nonce-") || !strings.Contains(csp, "style-src 'self' 'nonce-") {
		t.Fatalf("root CSP missing nonce directives: %q", csp)
	}

	body, err := io.ReadAll(rootResp.Body)
	if err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	if !strings.Contains(string(body), "<title>ticket board</title>") {
		t.Fatalf("root response missing embedded frontend")
	}
	if !strings.Contains(string(body), "<style nonce=\"") || !strings.Contains(string(body), "<script nonce=\"") {
		t.Fatalf("root response missing CSP nonce injection")
	}
}

func TestServerServesNamedEmbeddedSite(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := store.Init(dbPath, "admin", "password", static.SeedDatabase); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	srv, err := New(":0", db, "1.2.3", false, nil, "", "site2")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	rootResp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / error = %v", err)
	}
	defer rootResp.Body.Close()

	body, err := io.ReadAll(rootResp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !strings.Contains(string(body), "<title>ticket site2</title>") {
		t.Fatalf("root response missing site2 frontend")
	}
}

func TestHandlerRejectsUnknownEmbeddedSite(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := store.Init(dbPath, "admin", "password", static.SeedDatabase); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if _, err := Handler(db, "1.2.3", false, nil, "", "unknown-site"); err == nil || !strings.Contains(err.Error(), "unknown embedded site") {
		t.Fatalf("Handler() error = %v, want unknown embedded site", err)
	}
}

func TestServerVerboseLogging(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := store.Init(dbPath, "admin", "password", static.SeedDatabase); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	var logs strings.Builder
	srv, err := New(":0", db, "1.2.3", true, &logs, "", "")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/healthz")
	if err != nil {
		t.Fatalf("GET /api/healthz error = %v", err)
	}
	defer resp.Body.Close()

	if !strings.Contains(logs.String(), "method=GET") || !strings.Contains(logs.String(), "path=/api/healthz") {
		t.Fatalf("verbose logs missing request:\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), "status=200") {
		t.Fatalf("verbose logs missing response:\n%s", logs.String())
	}
}

func TestLoggingHandlerRedactsSensitiveBodiesAndCapsPayloads(t *testing.T) {
	t.Parallel()
	var logs strings.Builder
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	handler := loggingHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("io.ReadAll() error = %v", err)
		}
		_, _ = w.Write(body)
	}), logger)

	sensitiveReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"alice","password":"super-secret"}`))
	sensitiveReq.Header.Set("Content-Type", "application/json")
	sensitiveResp := httptest.NewRecorder()
	handler.ServeHTTP(sensitiveResp, sensitiveReq)

	if strings.Contains(logs.String(), "super-secret") || strings.Contains(logs.String(), "request_body") {
		t.Fatalf("sensitive request body should not be logged:\n%s", logs.String())
	}

	logs.Reset()
	largeBody := strings.Repeat("x", maxLoggedBodyBytes+128)
	normalReq := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(largeBody))
	normalReq.Header.Set("Content-Type", "application/json")
	normalResp := httptest.NewRecorder()
	handler.ServeHTTP(normalResp, normalReq)

	logOutput := logs.String()
	if !strings.Contains(logOutput, "response_body=") || !strings.Contains(logOutput, "…(truncated)") {
		t.Fatalf("expected truncated response body log:\n%s", logOutput)
	}
	if strings.Contains(logOutput, largeBody) {
		t.Fatalf("full large body should not be logged:\n%s", logOutput)
	}
}

func TestRealtimeHelpersAndHubLifecycle(t *testing.T) {
	t.Parallel()

	if !headerContainsToken(http.Header{"Connection": {"keep-alive, Upgrade"}}, "Connection", "upgrade") {
		t.Fatal("headerContainsToken() = false, want true")
	}
	if headerContainsToken(http.Header{"Connection": {"keep-alive"}}, "Connection", "upgrade") {
		t.Fatal("headerContainsToken() = true, want false")
	}
	if got := websocketAcceptKey("dGhlIHNhbXBsZSBub25jZQ=="); got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatalf("websocketAcceptKey() = %q", got)
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := writeWebSocketFrame(serverConn, 0x1, []byte("hello")); err != nil {
			t.Errorf("writeWebSocketFrame() error = %v", err)
		}
	}()
	opcode, payload, err := readWebSocketFrame(clientConn)
	if err != nil {
		t.Fatalf("readWebSocketFrame() error = %v", err)
	}
	<-done
	if opcode != 0x1 || string(payload) != "hello" {
		t.Fatalf("frame = (%d, %q), want (1, %q)", opcode, string(payload), "hello")
	}

	hubConnA, hubConnB := net.Pipe()
	defer hubConnA.Close()
	defer hubConnB.Close()
	hub := newLiveHub()
	client := hub.add(hubConnA)
	client.projectID = 42
	hub.broadcast(liveEvent{Type: "ticket_updated", ProjectID: 42})
	select {
	case msg := <-client.send:
		if !bytes.Contains(msg, []byte(`"ticket_updated"`)) {
			t.Fatalf("broadcast payload = %s", string(msg))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast")
	}
	hub.remove(client)
	select {
	case <-client.done:
	default:
		t.Fatal("client.done should be closed after remove")
	}
}

func TestIsUpgradeRequest(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	req.Header.Add("Connection", "keep-alive, Upgrade")
	if !isUpgradeRequest(req) {
		t.Fatal("isUpgradeRequest() = false, want true for upgrade token in connection header")
	}

	req.Header.Set("Connection", "keep-alive")
	if isUpgradeRequest(req) {
		t.Fatal("isUpgradeRequest() = true, want false when no upgrade token present")
	}
}

func TestAddVaryHeaderDeduplicates(t *testing.T) {
	t.Parallel()

	header := http.Header{}
	addVaryHeader(header, "Accept-Encoding")
	addVaryHeader(header, "accept-encoding")
	addVaryHeader(header, "Origin")

	got := strings.Join(header.Values("Vary"), ",")
	if strings.Count(strings.ToLower(got), "accept-encoding") != 1 {
		t.Fatalf("Vary header = %q, want Accept-Encoding only once", got)
	}
	if !strings.Contains(strings.ToLower(got), "origin") {
		t.Fatalf("Vary header = %q, want Origin token", got)
	}
}

func TestGzipResponseWriterFlushInitializesWriter(t *testing.T) {
	t.Parallel()

	rec := &flushTrackingResponseWriter{}
	writer := &gzipResponseWriter{ResponseWriter: rec}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	writer.Flush()

	if rec.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.status, http.StatusOK)
	}
	if !rec.flushed {
		t.Fatal("Flush() should forward to underlying flusher")
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
}

func TestGzipResponseWriterHijack(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	base := &hijackableResponseWriter{
		conn: serverConn,
		rw:   bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn)),
	}
	writer := &gzipResponseWriter{ResponseWriter: base}

	gotConn, gotRW, err := writer.Hijack()
	if err != nil {
		t.Fatalf("Hijack() error = %v", err)
	}
	if gotConn != serverConn {
		t.Fatal("Hijack() returned unexpected conn")
	}
	if gotRW == nil {
		t.Fatal("Hijack() returned nil read writer")
	}
}

func TestGzipResponseWriterHijackUnsupported(t *testing.T) {
	t.Parallel()

	writer := &gzipResponseWriter{ResponseWriter: httptest.NewRecorder()}
	conn, rw, err := writer.Hijack()
	if err == nil {
		t.Fatal("Hijack() error = nil, want unsupported error")
	}
	if conn != nil || rw != nil {
		t.Fatalf("Hijack() = (%v, %v, %v), want nils and error", conn, rw, err)
	}
}

func TestUpgradeWebSocketRejectsCrossOrigin(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "http://ticket.test/api/ws", nil)
	req.Host = "ticket.test"
	req.Header.Set("Origin", "https://evil.test")
	rec := httptest.NewRecorder()

	conn, err := upgradeWebSocket(rec, req)
	if err == nil {
		t.Fatal("upgradeWebSocket() error = nil, want cross-origin rejection")
	}
	if conn != nil {
		t.Fatalf("upgradeWebSocket() conn = %#v, want nil on cross-origin rejection", conn)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestUpgradeWebSocketSucceeds(t *testing.T) {
	t.Parallel()

	var written bytes.Buffer

	rec := &hijackableResponseWriter{
		conn: stubConn{},
		rw:   bufio.NewReadWriter(bufio.NewReader(strings.NewReader("")), bufio.NewWriter(&written)),
	}

	req := httptest.NewRequest(http.MethodGet, "http://ticket.test/api/ws", nil)
	req.Host = "ticket.test"
	req.Header.Set("Origin", "http://ticket.test")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	conn, err := upgradeWebSocket(rec, req)
	if err != nil {
		t.Fatalf("upgradeWebSocket() error = %v", err)
	}
	if conn == nil {
		t.Fatal("upgradeWebSocket() conn = nil, want hijacked conn")
	}

	joined := written.String()
	if !strings.Contains(joined, "101 Switching Protocols") {
		t.Fatalf("handshake = %q, want websocket status line", joined)
	}
	if !strings.Contains(joined, "Upgrade: websocket") {
		t.Fatalf("handshake headers = %q, want upgrade header", joined)
	}
	if !strings.Contains(joined, "Sec-WebSocket-Accept:") {
		t.Fatalf("handshake headers = %q, want accept header", joined)
	}
}

func TestReadWebSocketFrameDecodesMaskedExtendedPayload(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	payload := bytes.Repeat([]byte("a"), 130)
	maskKey := [4]byte{1, 2, 3, 4}
	masked := make([]byte, len(payload))
	for i := range payload {
		masked[i] = payload[i] ^ maskKey[i%4]
	}

	go func() {
		frame := []byte{0x81, 0x80 | 126, 0x00, 0x82, maskKey[0], maskKey[1], maskKey[2], maskKey[3]}
		frame = append(frame, masked...)
		_, _ = serverConn.Write(frame)
	}()

	opcode, gotPayload, err := readWebSocketFrame(clientConn)
	if err != nil {
		t.Fatalf("readWebSocketFrame() error = %v", err)
	}
	if opcode != 0x1 {
		t.Fatalf("opcode = %d, want %d", opcode, 0x1)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Fatalf("payload mismatch: got %d bytes want %d bytes", len(gotPayload), len(payload))
	}
}

func TestChatRuntimeAndBridgeStateHelpers(t *testing.T) {
	t.Parallel()

	rt := newChatRuntime()
	bridge := &chatProcessBridge{runtime: rt, startedAt: time.Now().UTC()}
	if id := rt.registerProcess(bridge); id == 0 {
		t.Fatal("registerProcess() = 0, want non-zero")
	} else {
		bridge.processID = id
	}

	rt.connectionOpened()
	rt.connectionClosed()
	rt.connectionClosed()

	bridge.markPrompt()
	bridge.markOutput()
	bridge.markError(" boom ")
	bridge.markCompleted(23, " done ")
	if !bridge.isCompleted() {
		t.Fatal("bridge should be completed")
	}
	if got := bridge.currentError(); got != "done" {
		t.Fatalf("currentError() = %q, want %q", got, "done")
	}
	lines := rt.processStatusLines()
	if len(lines) != 1 || !strings.Contains(lines[0], "completed=true") {
		t.Fatalf("processStatusLines() = %#v", lines)
	}

	var logLines []string
	rt.setLogger(func(line string) { logLines = append(logLines, line) })
	if len(logLines) == 0 || !strings.Contains(logLines[0], "heartbeat") {
		t.Fatalf("setLogger() logs = %#v", logLines)
	}
	rt.stopHeartbeat()
	if rt.heartbeatRunning {
		t.Fatal("heartbeatRunning = true, want false")
	}

	reader, writer := net.Pipe()
	defer reader.Close()
	bridge.stdin = writer
	sendDone := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(reader).ReadString('\n')
		sendDone <- line
	}()
	if err := bridge.Send("hello"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got := <-sendDone; got != "hello\n" {
		t.Fatalf("Send() wrote %q, want %q", got, "hello\n")
	}
	if err := bridge.CloseInput(); err != nil {
		t.Fatalf("CloseInput() error = %v", err)
	}
	rt.unregisterProcess(bridge.processID)
	if count := rt.runningProcessCount(); count != 0 {
		t.Fatalf("runningProcessCount() = %d, want 0", count)
	}
	if !rt.hasCapacity(1) {
		t.Fatal("hasCapacity(1) = false, want true")
	}
}
