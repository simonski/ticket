package server

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/simonski/ticket/internal/store"
)

type chatInboundMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type chatOutboundMessage struct {
	Type   string `json:"type"`
	Stream string `json:"stream,omitempty"`
	Text   string `json:"text,omitempty"`
	Error  string `json:"error,omitempty"`
	Code   int    `json:"code,omitempty"`
}

type chatProcessBridge struct {
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	mu           sync.Mutex
	once         sync.Once
	stateMu      sync.Mutex
	processID    int64
	runtime      *chatRuntime
	startedAt    time.Time
	lastPromptAt time.Time
	lastOutputAt time.Time
	lastActivity time.Time
	maxDuration  time.Duration
	timedOut     bool
	completed    bool
	exitCode     int
	lastError    string
}

var ansiControlRE = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\a]*(?:\a|\x1b\\)|[@-Z\\-_])`)
var sharedChatRuntime = newChatRuntime()

type chatRuntime struct {
	mu                sync.Mutex
	activeConnections int
	processes         map[int64]*chatProcessBridge
	nextProcessID     int64
	logger            func(string)
	heartbeatRunning  bool
	heartbeatStop     chan struct{}
}

func newChatRuntime() *chatRuntime {
	return &chatRuntime{
		processes: make(map[int64]*chatProcessBridge),
	}
}

func sanitizeTerminalOutput(raw string) string {
	if raw == "" {
		return ""
	}
	sanitized := ansiControlRE.ReplaceAllString(raw, "")
	var b strings.Builder
	for _, r := range sanitized {
		if r == '\n' || r == '\r' || r == '\t' || (r >= 32 && r != 127) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func websocketServeChat(w http.ResponseWriter, r *http.Request, db *sql.DB, logf func(string)) error {
	sharedChatRuntime.setLogger(logf)
	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		return err
	}
	sharedChatRuntime.connectionOpened()
	client := &liveClient{
		conn: conn,
		send: make(chan []byte, 64),
		done: make(chan struct{}),
	}
	sendJSON := func(message chatOutboundMessage) {
		payload, err := json.Marshal(message)
		if err != nil {
			return
		}
		select {
		case client.send <- payload:
		default:
		}
	}

	go func() {
		defer client.close()
		for {
			select {
			case <-client.done:
				return
			case payload := <-client.send:
				if err := writeWebSocketFrame(client.conn, 0x1, payload); err != nil {
					return
				}
			}
		}
	}()

	sendJSON(chatOutboundMessage{Type: "chat_connected"})
	sendJSON(chatOutboundMessage{Type: "chat_ready"})
	var bridge *chatProcessBridge
	sessionStartedAt := time.Now().UTC()

	defer func() {
		if bridge != nil {
			bridge.Close()
		}
		sharedChatRuntime.connectionClosed()
		client.close()
	}()

	for {
		opcode, payload, err := readWebSocketFrame(client.conn)
		if err != nil {
			return nil
		}
		switch opcode {
		case 0x8:
			_ = writeWebSocketFrame(client.conn, 0x8, nil)
			return nil
		case 0x9:
			_ = writeWebSocketFrame(client.conn, 0xA, payload)
		case 0x1:
			var message chatInboundMessage
			if err := json.Unmarshal(payload, &message); err != nil {
				sendJSON(chatOutboundMessage{Type: "chat_error", Error: "invalid chat payload"})
				continue
			}
			if message.Type != "chat_input" {
				continue
			}
			text := strings.TrimSpace(message.Text)
			if text == "" {
				continue
			}
			if logf != nil {
				logf(fmt.Sprintf("prompt: %s", text))
			}
			if bridge != nil && !bridge.isCompleted() {
				sendJSON(chatOutboundMessage{Type: "chat_error", Error: "chat request already running"})
				continue
			}
			limits, err := store.ChatLimitsConfig(db)
			if err != nil {
				sendJSON(chatOutboundMessage{Type: "chat_error", Error: "failed to load chat settings"})
				continue
			}
			maxSessionDuration := time.Duration(limits.MaxDurationMin) * time.Minute
			remaining := maxSessionDuration - time.Since(sessionStartedAt)
			if maxSessionDuration > 0 && remaining <= 0 {
				sendJSON(chatOutboundMessage{Type: "chat_error", Error: fmt.Sprintf("conversation limit reached (%d minutes)", limits.MaxDurationMin)})
				sendJSON(chatOutboundMessage{Type: "chat_exit", Code: 124})
				continue
			}
			running := sharedChatRuntime.runningProcessCount()
			if !sharedChatRuntime.hasCapacity(limits.MaxConnections) {
				sendJSON(chatOutboundMessage{
					Type:  "chat_error",
					Error: fmt.Sprintf("chat capacity reached (%d/%d). wait for an active chat to finish", running, limits.MaxConnections),
				})
				continue
			}
			sendJSON(chatOutboundMessage{Type: "chat_processing"})
			newBridge, err := startChatBridge(sendJSON, logf, remaining)
			if err != nil {
				sendJSON(chatOutboundMessage{Type: "chat_error", Error: err.Error()})
				continue
			}
			bridge = newBridge
			bridge.markPrompt()
			if err := bridge.Send(text); err != nil {
				bridge.markError(err.Error())
				sendJSON(chatOutboundMessage{Type: "chat_error", Error: err.Error()})
				bridge.Close()
				continue
			}
			if err := bridge.CloseInput(); err != nil && logf != nil {
				logf(fmt.Sprintf("stdin close error: %s", err.Error()))
			}
		}
	}
}

func startChatBridge(send func(chatOutboundMessage), logf func(string), maxDuration time.Duration) (*chatProcessBridge, error) {
	return startChatBridgeWithDuration(send, logf, maxDuration)
}

func startChatBridgeWithDuration(send func(chatOutboundMessage), logf func(string), maxDuration time.Duration) (*chatProcessBridge, error) {
	commandArgs := resolveChatCommandArgs()
	if len(commandArgs) == 0 {
		return nil, errors.New("chat command is empty")
	}
	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	cmd.Env = append(os.Environ(),
		"TERM=dumb",
		"NO_COLOR=1",
		"CLICOLOR=0",
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("unable to create chat stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("unable to create chat stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("unable to create chat stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("unable to start chat command %q: %w", strings.Join(commandArgs, " "), err)
	}
	bridge := &chatProcessBridge{
		cmd:          cmd,
		stdin:        stdin,
		runtime:      sharedChatRuntime,
		startedAt:    time.Now().UTC(),
		lastActivity: time.Now().UTC(),
		maxDuration:  maxDuration,
	}
	bridge.processID = sharedChatRuntime.registerProcess(bridge)
	if logf != nil {
		pid := int64(0)
		if cmd.Process != nil {
			pid = int64(cmd.Process.Pid)
		}
		logf(fmt.Sprintf("process spawned id=%d pid=%d command=%q", bridge.processID, pid, strings.Join(commandArgs, " ")))
	}
	bridge.streamOutput(stdout, "stdout", send, logf)
	bridge.streamOutput(stderr, "stderr", send, logf)
	go func() {
		err := cmd.Wait()
		if err != nil {
			if bridge.isTimedOut() {
				bridge.markCompleted(124, bridge.currentError())
				if logf != nil {
					logf(fmt.Sprintf("process completed running=false completed=true error=%q exit_code=%d", bridge.currentError(), 124))
				}
				send(chatOutboundMessage{Type: "chat_exit", Code: 124})
				bridge.Close()
				return
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				bridge.markCompleted(exitErr.ExitCode(), err.Error())
				if logf != nil {
					logf(fmt.Sprintf("process completed running=false completed=true error=%q exit_code=%d", err.Error(), exitErr.ExitCode()))
				}
				send(chatOutboundMessage{Type: "chat_exit", Code: exitErr.ExitCode()})
			} else {
				bridge.markCompleted(-1, err.Error())
				if logf != nil {
					logf(fmt.Sprintf("process completed running=false completed=true error=%q exit_code=%d", err.Error(), -1))
				}
				send(chatOutboundMessage{Type: "chat_error", Error: err.Error()})
			}
		} else {
			bridge.markCompleted(0, "")
			if logf != nil {
				logf("process completed running=false completed=true error=none exit_code=0")
			}
			send(chatOutboundMessage{Type: "chat_exit", Code: 0})
		}
		bridge.Close()
	}()
	if bridge.maxDuration > 0 {
		go func(maxDuration time.Duration) {
			timer := time.NewTimer(maxDuration)
			defer timer.Stop()
			<-timer.C
			if bridge.isCompleted() {
				return
			}
			label := maxDuration.Round(time.Second).String()
			errText := fmt.Sprintf("conversation limit reached (%s)", label)
			bridge.markTimedOut(errText)
			send(chatOutboundMessage{Type: "chat_error", Error: errText})
			bridge.Close()
		}(bridge.maxDuration)
	}
	return bridge, nil
}

func resolveChatCommandArgs() []string {
	if raw := strings.TrimSpace(os.Getenv("TICKET_CHAT_CMD")); raw != "" {
		parts := strings.Fields(raw)
		if len(parts) == 0 {
			return []string{"codex", "exec"}
		}
		if strings.EqualFold(parts[0], "codex") && (len(parts) == 1 || strings.HasPrefix(parts[1], "-")) {
			return append([]string{parts[0], "exec"}, parts[1:]...)
		}
		return parts
	}
	return []string{"codex", "exec"}
}

func (b *chatProcessBridge) streamOutput(reader io.Reader, stream string, send func(chatOutboundMessage), logf func(string)) {
	go func() {
		buffered := bufio.NewReader(reader)
		for {
			chunk := make([]byte, 1024)
			n, err := buffered.Read(chunk)
			if n > 0 {
				clean := sanitizeTerminalOutput(string(chunk[:n]))
				if clean == "" {
					continue
				}
				b.markOutput()
				if logf != nil {
					logf(fmt.Sprintf("output[%s]: %s", stream, clean))
				}
				send(chatOutboundMessage{
					Type:   "chat_output",
					Stream: stream,
					Text:   clean,
				})
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					b.markError(err.Error())
					send(chatOutboundMessage{Type: "chat_error", Error: err.Error()})
				}
				return
			}
		}
	}()
}

func (b *chatProcessBridge) markPrompt() {
	if b == nil {
		return
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	now := time.Now().UTC()
	b.lastPromptAt = now
	b.lastActivity = now
}

func (b *chatProcessBridge) markOutput() {
	if b == nil {
		return
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	now := time.Now().UTC()
	b.lastOutputAt = now
	b.lastActivity = now
}

func (b *chatProcessBridge) markError(err string) {
	if b == nil {
		return
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.lastError = strings.TrimSpace(err)
}

func (b *chatProcessBridge) markCompleted(code int, err string) {
	if b == nil {
		return
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.completed = true
	b.exitCode = code
	b.lastError = strings.TrimSpace(err)
}

func (b *chatProcessBridge) markTimedOut(err string) {
	if b == nil {
		return
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.timedOut = true
	b.lastError = strings.TrimSpace(err)
}

func (b *chatProcessBridge) isTimedOut() bool {
	if b == nil {
		return false
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	return b.timedOut
}

func (b *chatProcessBridge) isCompleted() bool {
	if b == nil {
		return true
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	return b.completed
}

func (b *chatProcessBridge) currentError() string {
	if b == nil {
		return ""
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	return b.lastError
}

func (b *chatProcessBridge) statusLine(now time.Time) string {
	if b == nil {
		return "process_status running=false completed=true error=\"chat bridge unavailable\""
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	running := !b.completed
	errorLabel := "none"
	if strings.TrimSpace(b.lastError) != "" {
		errorLabel = b.lastError
	}
	lastPrompt := "never"
	if !b.lastPromptAt.IsZero() {
		lastPrompt = now.Sub(b.lastPromptAt).Round(time.Second).String() + " ago"
	}
	lastOutput := "never"
	if !b.lastOutputAt.IsZero() {
		lastOutput = now.Sub(b.lastOutputAt).Round(time.Second).String() + " ago"
	}
	lastActivity := "never"
	if !b.lastActivity.IsZero() {
		lastActivity = now.Sub(b.lastActivity).Round(time.Second).String() + " ago"
	}
	pid := 0
	if b.cmd != nil && b.cmd.Process != nil {
		pid = b.cmd.Process.Pid
	}
	return fmt.Sprintf("process_status id=%d pid=%d running=%t completed=%t exit_code=%d error=%q last_prompt=%s last_output=%s last_activity=%s", b.processID, pid, running, b.completed, b.exitCode, errorLabel, lastPrompt, lastOutput, lastActivity)
}

func (rt *chatRuntime) setLogger(logf func(string)) {
	if rt == nil || logf == nil {
		return
	}
	rt.mu.Lock()
	rt.logger = logf
	if rt.heartbeatRunning {
		rt.mu.Unlock()
		return
	}
	rt.heartbeatRunning = true
	rt.heartbeatStop = make(chan struct{})
	stop := rt.heartbeatStop
	rt.mu.Unlock()

	logf(rt.heartbeatLine())
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				logf(rt.heartbeatLine())
				for _, line := range rt.processStatusLines() {
					logf(line)
				}
			case <-stop:
				return
			}
		}
	}()
}

func (rt *chatRuntime) connectionOpened() {
	if rt == nil {
		return
	}
	rt.mu.Lock()
	rt.activeConnections++
	rt.mu.Unlock()
}

func (rt *chatRuntime) connectionClosed() {
	if rt == nil {
		return
	}
	rt.mu.Lock()
	if rt.activeConnections > 0 {
		rt.activeConnections--
	}
	rt.mu.Unlock()
}

func (rt *chatRuntime) registerProcess(bridge *chatProcessBridge) int64 {
	if rt == nil || bridge == nil {
		return 0
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.nextProcessID++
	id := rt.nextProcessID
	rt.processes[id] = bridge
	return id
}

func (rt *chatRuntime) unregisterProcess(id int64) {
	if rt == nil || id == 0 {
		return
	}
	rt.mu.Lock()
	delete(rt.processes, id)
	rt.mu.Unlock()
}

func (rt *chatRuntime) heartbeatLine() string {
	if rt == nil {
		return "heartbeat connections=0 processes_running=0 processes_total=0"
	}
	rt.mu.Lock()
	connections := rt.activeConnections
	total := len(rt.processes)
	running := 0
	for _, bridge := range rt.processes {
		if bridge == nil {
			continue
		}
		bridge.stateMu.Lock()
		if !bridge.completed {
			running++
		}
		bridge.stateMu.Unlock()
	}
	rt.mu.Unlock()
	return fmt.Sprintf("heartbeat connections=%d processes_running=%d processes_total=%d", connections, running, total)
}

func (rt *chatRuntime) runningProcessCount() int {
	if rt == nil {
		return 0
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	running := 0
	for _, bridge := range rt.processes {
		if bridge == nil {
			continue
		}
		bridge.stateMu.Lock()
		if !bridge.completed {
			running++
		}
		bridge.stateMu.Unlock()
	}
	return running
}

func (rt *chatRuntime) hasCapacity(maxConnections int) bool {
	if maxConnections <= 0 {
		return true
	}
	return rt.runningProcessCount() < maxConnections
}

func (rt *chatRuntime) processStatusLines() []string {
	if rt == nil {
		return nil
	}
	rt.mu.Lock()
	snapshot := make([]*chatProcessBridge, 0, len(rt.processes))
	for _, bridge := range rt.processes {
		snapshot = append(snapshot, bridge)
	}
	rt.mu.Unlock()
	now := time.Now().UTC()
	lines := make([]string, 0, len(snapshot))
	for _, bridge := range snapshot {
		if bridge == nil {
			continue
		}
		lines = append(lines, bridge.statusLine(now))
	}
	return lines
}

func (b *chatProcessBridge) Send(input string) error {
	if b == nil {
		return errors.New("chat process not started")
	}
	if strings.TrimSpace(input) == "" {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stdin == nil {
		return errors.New("chat stdin is not available")
	}
	_, err := io.WriteString(b.stdin, input+"\n")
	return err
}

func (b *chatProcessBridge) CloseInput() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stdin == nil {
		return nil
	}
	err := b.stdin.Close()
	b.stdin = nil
	return err
}

func (b *chatProcessBridge) Close() {
	b.once.Do(func() {
		if b.runtime != nil && b.processID != 0 {
			b.runtime.unregisterProcess(b.processID)
		}
		b.mu.Lock()
		if b.stdin != nil {
			_ = b.stdin.Close()
			b.stdin = nil
		}
		b.mu.Unlock()
		if b.cmd != nil && b.cmd.Process != nil {
			_ = b.cmd.Process.Kill()
		}
	})
}
