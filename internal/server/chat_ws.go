package server

import (
	"bufio"
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

	"github.com/creack/pty"
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
	tty          *os.File
	mu           sync.Mutex
	once         sync.Once
	stateMu      sync.Mutex
	startedAt    time.Time
	lastPromptAt time.Time
	lastOutputAt time.Time
	lastActivity time.Time
	completed    bool
	exitCode     int
	lastError    string
	heartbeatCh  chan struct{}
}

var ansiControlRE = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\a]*(?:\a|\x1b\\)|[@-Z\\-_])`)

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

func websocketServeChat(w http.ResponseWriter, r *http.Request, logf func(string)) error {
	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		return err
	}
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

	bridge, err := startChatBridge(sendJSON, logf)
	if err != nil {
		sendJSON(chatOutboundMessage{
			Type:  "chat_error",
			Error: err.Error(),
		})
	} else {
		sendJSON(chatOutboundMessage{Type: "chat_ready"})
	}

	defer func() {
		if bridge != nil {
			bridge.Close()
		}
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
			handleChatInput(bridge, message.Text, sendJSON, logf)
		}
	}
}

func handleChatInput(bridge *chatProcessBridge, text string, send func(chatOutboundMessage), logf func(string)) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if logf != nil {
		logf(fmt.Sprintf("prompt: %s", text))
	}
	if bridge == nil {
		send(chatOutboundMessage{Type: "chat_error", Error: "chat backend is unavailable"})
		return
	}
	bridge.markPrompt()
	send(chatOutboundMessage{Type: "chat_processing"})
	if err := bridge.Send(text); err != nil {
		bridge.markError(err.Error())
		send(chatOutboundMessage{Type: "chat_error", Error: err.Error()})
	}
}

func startChatBridge(send func(chatOutboundMessage), logf func(string)) (*chatProcessBridge, error) {
	commandLine := resolveChatCommandLine()
	shellPath := resolveShellPath()
	cmd := exec.Command(shellPath, "-lc", commandLine)
	cmd.Env = append(os.Environ(),
		"TERM=dumb",
		"NO_COLOR=1",
		"CLICOLOR=0",
	)
	tty, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("unable to start chat shell %q with command %q: %w", shellPath, commandLine, err)
	}
	bridge := &chatProcessBridge{
		cmd:          cmd,
		tty:          tty,
		startedAt:    time.Now().UTC(),
		lastActivity: time.Now().UTC(),
		heartbeatCh:  make(chan struct{}),
	}
	if logf != nil {
		pid := int64(0)
		if cmd.Process != nil {
			pid = int64(cmd.Process.Pid)
		}
		logf(fmt.Sprintf("process spawned pid=%d command=%q shell=%q", pid, commandLine, shellPath))
	}
	bridge.startHeartbeat(logf)
	bridge.streamOutput(tty, "pty", send, logf)
	go func() {
		err := cmd.Wait()
		if err != nil {
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
	return bridge, nil
}

func resolveChatCommandLine() string {
	if raw := strings.TrimSpace(os.Getenv("TICKET_CHAT_CMD")); raw != "" {
		return raw
	}
	return "codex"
}

func resolveShellPath() string {
	if shell := strings.TrimSpace(os.Getenv("SHELL")); shell != "" {
		return shell
	}
	return "/bin/sh"
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

func (b *chatProcessBridge) startHeartbeat(logf func(string)) {
	if b == nil || logf == nil {
		return
	}
	logf(b.heartbeatLine())
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				logf(b.heartbeatLine())
			case <-b.heartbeatCh:
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

func (b *chatProcessBridge) heartbeatLine() string {
	if b == nil {
		return "heartbeat running=false completed=true error=\"chat bridge unavailable\""
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	now := time.Now().UTC()
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
	return fmt.Sprintf("heartbeat pid=%d running=%t completed=%t exit_code=%d error=%q last_prompt=%s last_output=%s last_activity=%s", pid, running, b.completed, b.exitCode, errorLabel, lastPrompt, lastOutput, lastActivity)
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
	if b.tty == nil {
		return errors.New("chat terminal is not available")
	}
	_, err := io.WriteString(b.tty, input+"\n")
	return err
}

func (b *chatProcessBridge) Close() {
	b.once.Do(func() {
		if b.heartbeatCh != nil {
			close(b.heartbeatCh)
		}
		b.mu.Lock()
		if b.tty != nil {
			_ = b.tty.Close()
			b.tty = nil
		}
		b.mu.Unlock()
		if b.cmd != nil && b.cmd.Process != nil {
			_ = b.cmd.Process.Kill()
		}
	})
}
