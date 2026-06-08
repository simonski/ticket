package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// writeRefinerScript writes an executable shell script and returns its path.
func writeRefinerScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "refiner.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("write script: %v", err)
	}
	return path
}

func TestStreamRefinerLLMLogsStdoutWhenVerbose(t *testing.T) {
	script := writeRefinerScript(t, `cat >/dev/null; echo "hello from refiner"`)
	t.Setenv("TICKET_CHAT_CMD", script)

	var mu sync.Mutex
	var logs []string
	logf := func(line string) { mu.Lock(); logs = append(logs, line); mu.Unlock() }

	out, err := streamRefinerLLM(context.Background(), "a prompt", nil, logf)
	if err != nil {
		t.Fatalf("streamRefinerLLM error = %v", err)
	}
	if !strings.Contains(out, "hello from refiner") {
		t.Errorf("expected stdout captured, got %q", out)
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "exec:") {
		t.Errorf("expected exec log line, got:\n%s", joined)
	}
	if !strings.Contains(joined, "stdout: hello from refiner") {
		t.Errorf("expected stdout log line, got:\n%s", joined)
	}
}

func TestStreamRefinerLLMSurfacesStderrOnFailure(t *testing.T) {
	script := writeRefinerScript(t, `cat >/dev/null; echo "boom: bad api key" >&2; exit 1`)
	t.Setenv("TICKET_CHAT_CMD", script)

	_, err := streamRefinerLLM(context.Background(), "a prompt", nil, nil)
	if err == nil {
		t.Fatal("expected error from failing refiner command")
	}
	if !strings.Contains(err.Error(), "boom: bad api key") {
		t.Errorf("expected stderr in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("expected exit status in error, got %q", err.Error())
	}
}

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
