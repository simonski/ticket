package server

import (
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
	})
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
	})
	if err != nil {
		t.Fatalf("startChatBridge() error = %v", err)
	}
	defer bridge.Close()

	handleChatInput(bridge, "hello from test", func(message chatOutboundMessage) {
		messages <- message
	})

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
