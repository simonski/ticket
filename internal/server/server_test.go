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

	srv, err := New(":0", db, "1.2.3", false, nil, "")
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

	srv, err := New(":0", db, "1.2.3", false, nil, "")
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
	srv, err := New(":0", db, "1.2.3", true, &logs, "")
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
