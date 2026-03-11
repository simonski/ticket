package server

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestSanitizeTerminalOutputRemovesAnsiSequences(t *testing.T) {
	raw := "\x1b[?2026h\x1b[39mhello\x1b[0m\x1b[?25l world\x1b[?2026l"
	got := sanitizeTerminalOutput(raw)
	want := "hello world"
	if got != want {
		t.Fatalf("sanitizeTerminalOutput() = %q, want %q", got, want)
	}
}

func TestHandleChatInputEmitsErrorWhenBridgeMissing(t *testing.T) {
	out := make([]chatOutboundMessage, 0, 2)
	handleChatInput(nil, "hello", func(message chatOutboundMessage) {
		out = append(out, message)
	}, nil)
	if len(out) != 1 {
		t.Fatalf("message count = %d, want 1", len(out))
	}
	if out[0].Type != "chat_error" {
		t.Fatalf("first type = %q, want chat_error", out[0].Type)
	}
}

func TestStartChatBridgeStreamsOutputForInput(t *testing.T) {
	t.Setenv("TICKET_CHAT_CMD", "cat")

	messages := make(chan chatOutboundMessage, 16)
	bridge, err := startChatBridge(func(message chatOutboundMessage) {
		messages <- message
	}, nil)
	if err != nil {
		t.Fatalf("startChatBridge() error = %v", err)
	}
	defer bridge.Close()

	handleChatInput(bridge, "hello from test", func(message chatOutboundMessage) {
		messages <- message
	}, nil)

	deadline := time.After(2 * time.Second)
	seenProcessing := false
	seenOutput := false
	for !(seenProcessing && seenOutput) {
		select {
		case msg := <-messages:
			if msg.Type == "chat_processing" {
				seenProcessing = true
			}
			if msg.Type == "chat_output" && strings.Contains(msg.Text, "hello from test") {
				seenOutput = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for chat_processing and chat_output (processing=%v output=%v)", seenProcessing, seenOutput)
		}
	}
}

func TestHandleChatInputLogsPrompt(t *testing.T) {
	lines := make([]string, 0, 1)
	handleChatInput(nil, "hello world", func(chatOutboundMessage) {}, func(line string) {
		lines = append(lines, line)
	})
	if len(lines) != 1 {
		t.Fatalf("log line count = %d, want 1", len(lines))
	}
	if !strings.Contains(lines[0], "prompt: hello world") {
		t.Fatalf("log line = %q, want prompt content", lines[0])
	}
}

func TestChatProcessBridgeHeartbeatLineIncludesStatusAndActivity(t *testing.T) {
	bridge := &chatProcessBridge{
		cmd:          &exec.Cmd{},
		startedAt:    time.Now().UTC().Add(-2 * time.Second),
		lastPromptAt: time.Now().UTC().Add(-4 * time.Second),
		lastOutputAt: time.Now().UTC().Add(-3 * time.Second),
		lastActivity: time.Now().UTC().Add(-2 * time.Second),
		exitCode:     0,
	}
	line := bridge.heartbeatLine()
	for _, want := range []string{
		"heartbeat",
		"running=true",
		"completed=false",
		"error=\"none\"",
		"last_prompt=",
		"last_output=",
		"last_activity=",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("heartbeat line missing %q: %s", want, line)
		}
	}
}

func TestChatProcessBridgeHeartbeatLineShowsCompletedAndError(t *testing.T) {
	bridge := &chatProcessBridge{
		completed: true,
		exitCode:  7,
		lastError: "boom",
	}
	line := bridge.heartbeatLine()
	for _, want := range []string{
		"running=false",
		"completed=true",
		"exit_code=7",
		"error=\"boom\"",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("completed heartbeat line missing %q: %s", want, line)
		}
	}
}
