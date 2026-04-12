package server

import (
	"bufio"
	"bytes"
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

	body, err := io.ReadAll(rootResp.Body)
	if err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	if !strings.Contains(string(body), "<title>ticket board</title>") {
		t.Fatalf("root response missing embedded frontend")
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
