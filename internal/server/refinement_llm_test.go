package server

import (
	"strings"
	"testing"
)

func TestRefinerLLMAvailableMissingCommand(t *testing.T) {
	t.Setenv("TICKET_CHAT_CMD", "definitely-not-a-real-binary-xyz")

	avail, cmd, advice := refinerLLMAvailable()
	if avail {
		t.Fatalf("expected refiner LLM to be unavailable for a missing binary")
	}
	if !strings.Contains(cmd, "definitely-not-a-real-binary-xyz") {
		t.Errorf("expected command in result, got %q", cmd)
	}
	if !strings.Contains(advice, "TICKET_CHAT_CMD") {
		t.Errorf("expected advice to mention TICKET_CHAT_CMD, got %q", advice)
	}
}

func TestRefinerLLMAvailableResolvableCommand(t *testing.T) {
	// "echo" resolves on PATH on the test platforms we run on.
	t.Setenv("TICKET_CHAT_CMD", "echo")

	avail, _, advice := refinerLLMAvailable()
	if !avail {
		t.Fatalf("expected refiner LLM to be available for a resolvable binary; advice=%q", advice)
	}
	if advice != "" {
		t.Errorf("expected no advice when available, got %q", advice)
	}
}
