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
	"strings"
	"sync"
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
	cmd   *exec.Cmd
	stdin io.WriteCloser
	done  chan struct{}
	once  sync.Once
}

func websocketServeChat(w http.ResponseWriter, r *http.Request) error {
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

	bridge, err := startChatBridge(sendJSON)
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
			if bridge == nil {
				sendJSON(chatOutboundMessage{Type: "chat_error", Error: "chat backend is unavailable"})
				continue
			}
			if err := bridge.Send(message.Text); err != nil {
				sendJSON(chatOutboundMessage{Type: "chat_error", Error: err.Error()})
			}
		}
	}
}

func startChatBridge(send func(chatOutboundMessage)) (*chatProcessBridge, error) {
	command, args := resolveChatCommand()
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("unable to start chat command %q: %w", command, err)
	}
	bridge := &chatProcessBridge{
		cmd:   cmd,
		stdin: stdin,
		done:  make(chan struct{}),
	}
	bridge.streamOutput(stdout, "stdout", send)
	bridge.streamOutput(stderr, "stderr", send)
	go func() {
		err := cmd.Wait()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				send(chatOutboundMessage{Type: "chat_exit", Code: exitErr.ExitCode()})
			} else {
				send(chatOutboundMessage{Type: "chat_error", Error: err.Error()})
			}
		} else {
			send(chatOutboundMessage{Type: "chat_exit", Code: 0})
		}
		bridge.Close()
	}()
	return bridge, nil
}

func resolveChatCommand() (string, []string) {
	if raw := strings.TrimSpace(os.Getenv("TICKET_CHAT_CMD")); raw != "" {
		parts := strings.Fields(raw)
		if len(parts) == 1 {
			return parts[0], nil
		}
		return parts[0], parts[1:]
	}
	// Default to codex in interactive chat mode.
	return "codex", []string{"chat"}
}

func (b *chatProcessBridge) streamOutput(reader io.Reader, stream string, send func(chatOutboundMessage)) {
	go func() {
		buffered := bufio.NewReader(reader)
		for {
			chunk := make([]byte, 1024)
			n, err := buffered.Read(chunk)
			if n > 0 {
				send(chatOutboundMessage{
					Type:   "chat_output",
					Stream: stream,
					Text:   string(chunk[:n]),
				})
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					send(chatOutboundMessage{Type: "chat_error", Error: err.Error()})
				}
				return
			}
		}
	}()
}

func (b *chatProcessBridge) Send(input string) error {
	if b == nil {
		return errors.New("chat process not started")
	}
	if strings.TrimSpace(input) == "" {
		return nil
	}
	_, err := io.WriteString(b.stdin, input+"\n")
	return err
}

func (b *chatProcessBridge) Close() {
	b.once.Do(func() {
		close(b.done)
		if b.stdin != nil {
			_ = b.stdin.Close()
		}
		if b.cmd != nil && b.cmd.Process != nil {
			_ = b.cmd.Process.Kill()
		}
	})
}
