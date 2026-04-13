package main

import "testing"

func TestValidateLLMBinaryKnownDefaults(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"claude", "codex"} {
		if err := validateLLMBinary(name); err != nil {
			t.Fatalf("validateLLMBinary(%q) error = %v", name, err)
		}
	}
}

func TestValidateLLMBinaryRejectsInvalidCharacters(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"claude;rm", "../claude", "claude/alt", `claude\alt`} {
		if err := validateLLMBinary(name); err == nil {
			t.Fatalf("validateLLMBinary(%q) error = nil, want rejection", name)
		}
	}
}

func TestValidateLLMBinarySupportsConfiguredAllowList(t *testing.T) {
	t.Setenv("TICKET_AGENT_ALLOWED_LLM_BINARIES", "my-llm")
	if err := validateLLMBinary("my-llm"); err != nil {
		t.Fatalf("validateLLMBinary(custom) error = %v", err)
	}
}
