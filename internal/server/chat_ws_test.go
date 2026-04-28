package server

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestSanitizeTerminalOutputRemovesAnsiSequences(t *testing.T) {
	t.Parallel()
	raw := "\x1b[?2026h\x1b[39mhello\x1b[0m\x1b[?25l world\x1b[?2026l"
	got := sanitizeTerminalOutput(raw)
	want := "hello world"
	if got != want {
		t.Fatalf("sanitizeTerminalOutput() = %q, want %q", got, want)
	}
}

func TestStartChatBridgeStreamsOutputForInput(t *testing.T) {
	t.Setenv("TICKET_CHAT_CMD", "cat")

	messages := make(chan chatOutboundMessage, 16)
	bridge, err := startChatBridge(func(message chatOutboundMessage) {
		messages <- message
	}, nil, 3*time.Second)
	if err != nil {
		t.Fatalf("startChatBridge() error = %v", err)
	}
	defer bridge.Close()

	if err := bridge.Send("hello from test"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if err := bridge.CloseInput(); err != nil {
		t.Fatalf("CloseInput() error = %v", err)
	}

	deadline := time.After(2 * time.Second)
	seenOutput := false
	for !seenOutput {
		select {
		case msg := <-messages:
			if msg.Type == "chat_output" && strings.Contains(msg.Text, "hello from test") {
				seenOutput = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for chat_output (output=%v)", seenOutput)
		}
	}
}

func TestChatProcessBridgeStatusLineIncludesStatusAndActivity(t *testing.T) {
	t.Parallel()
	bridge := &chatProcessBridge{
		processID:    3,
		cmd:          &exec.Cmd{},
		startedAt:    time.Now().UTC().Add(-2 * time.Second),
		lastPromptAt: time.Now().UTC().Add(-4 * time.Second),
		lastOutputAt: time.Now().UTC().Add(-3 * time.Second),
		lastActivity: time.Now().UTC().Add(-2 * time.Second),
		exitCode:     0,
	}
	line := bridge.statusLine(time.Now().UTC())
	for _, want := range []string{
		"process_status",
		"id=3",
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

func TestChatProcessBridgeStatusLineShowsCompletedAndError(t *testing.T) {
	t.Parallel()
	bridge := &chatProcessBridge{
		processID: 9,
		completed: true,
		exitCode:  7,
		lastError: "boom",
	}
	line := bridge.statusLine(time.Now().UTC())
	for _, want := range []string{
		"id=9",
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

func TestChatRuntimeHeartbeatLineIncludesConnectionAndRunningCounts(t *testing.T) {
	t.Parallel()
	runtime := newChatRuntime()
	runtime.activeConnections = 4
	runtime.processes[1] = &chatProcessBridge{}
	runtime.processes[2] = &chatProcessBridge{completed: true}
	line := runtime.heartbeatLine()
	for _, want := range []string{
		"connections=4",
		"processes_running=1",
		"processes_total=2",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("runtime heartbeat line missing %q: %s", want, line)
		}
	}
}

func TestChatRuntimeRunningProcessCount(t *testing.T) {
	t.Parallel()
	runtime := newChatRuntime()
	runtime.processes[1] = &chatProcessBridge{}
	runtime.processes[2] = &chatProcessBridge{completed: true}
	runtime.processes[3] = &chatProcessBridge{}
	if got := runtime.runningProcessCount(); got != 2 {
		t.Fatalf("runningProcessCount() = %d, want 2", got)
	}
}

func TestChatRuntimeHasCapacity(t *testing.T) {
	t.Parallel()
	runtime := newChatRuntime()
	runtime.processes[1] = &chatProcessBridge{}
	runtime.processes[2] = &chatProcessBridge{completed: true}
	if !runtime.hasCapacity(2) {
		t.Fatalf("hasCapacity(2) = false, want true")
	}
	if runtime.hasCapacity(1) {
		t.Fatalf("hasCapacity(1) = true, want false")
	}
}

func TestChatRuntimeNilAndBridgeEdgeCases(t *testing.T) {
	t.Parallel()

	var runtime *chatRuntime
	runtime.setLogger(func(string) {})
	runtime.stopHeartbeat()
	runtime.connectionOpened()
	runtime.connectionClosed()
	if id := runtime.registerProcess(&chatProcessBridge{}); id != 0 {
		t.Fatalf("nil runtime registerProcess() = %d, want 0", id)
	}
	runtime.unregisterProcess(1)
	if got := runtime.heartbeatLine(); got != "heartbeat connections=0 processes_running=0 processes_total=0" {
		t.Fatalf("nil runtime heartbeatLine() = %q", got)
	}
	if got := runtime.runningProcessCount(); got != 0 {
		t.Fatalf("nil runtime runningProcessCount() = %d, want 0", got)
	}

	live := newChatRuntime()
	if id := live.registerProcess(nil); id != 0 {
		t.Fatalf("registerProcess(nil) = %d, want 0", id)
	}
	live.unregisterProcess(0)
	live.processes[1] = nil
	if got := live.runningProcessCount(); got != 0 {
		t.Fatalf("runningProcessCount() with nil bridge = %d, want 0", got)
	}
	if lines := live.processStatusLines(); len(lines) != 0 {
		t.Fatalf("processStatusLines() with nil bridge = %#v, want empty", lines)
	}
	if !live.hasCapacity(0) {
		t.Fatal("hasCapacity(0) = false, want unlimited capacity")
	}

	var bridge *chatProcessBridge
	if err := bridge.Send("hello"); err == nil {
		t.Fatal("nil bridge Send() error = nil, want error")
	}
	if err := bridge.CloseInput(); err != nil {
		t.Fatalf("nil bridge CloseInput() error = %v, want nil", err)
	}
	emptyBridge := &chatProcessBridge{}
	if err := emptyBridge.Send("   "); err != nil {
		t.Fatalf("empty input Send() error = %v, want nil", err)
	}
	if err := emptyBridge.Send("hello"); err == nil {
		t.Fatal("bridge without stdin Send() error = nil, want error")
	}
	if err := emptyBridge.CloseInput(); err != nil {
		t.Fatalf("bridge without stdin CloseInput() error = %v, want nil", err)
	}
	emptyBridge.Close()
}

func TestStartChatBridgeWithDurationEnforcesTimeout(t *testing.T) {
	t.Setenv("TICKET_CHAT_CMD", "sleep 5")
	messages := make(chan chatOutboundMessage, 32)
	bridge, err := startChatBridgeWithDuration(func(message chatOutboundMessage) {
		messages <- message
	}, nil, 120*time.Millisecond)
	if err != nil {
		t.Fatalf("startChatBridgeWithDuration() error = %v", err)
	}
	defer bridge.Close()

	deadline := time.After(3 * time.Second)
	seenTimeoutError := false
	seenExit124 := false
	for !(seenTimeoutError && seenExit124) {
		select {
		case msg := <-messages:
			if msg.Type == "chat_error" && strings.Contains(strings.ToLower(msg.Error), "conversation limit reached") {
				seenTimeoutError = true
			}
			if msg.Type == "chat_exit" && msg.Code == 124 {
				seenExit124 = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for timeout signal and exit (timeoutError=%v exit124=%v)", seenTimeoutError, seenExit124)
		}
	}
}

func TestResolveChatCommandArgsDefaultsToCodexExec(t *testing.T) {
	t.Setenv("TICKET_CHAT_CMD", "")
	got := resolveChatCommandArgs()
	want := []string{"codex", "exec"}
	if len(got) != len(want) {
		t.Fatalf("resolveChatCommandArgs() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("resolveChatCommandArgs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolveChatCommandArgsInjectsExecForCodexFlags(t *testing.T) {
	t.Setenv("TICKET_CHAT_CMD", "codex --model gpt-5.3-codex")
	got := resolveChatCommandArgs()
	want := []string{"codex", "exec", "--model", "gpt-5.3-codex"}
	if len(got) != len(want) {
		t.Fatalf("resolveChatCommandArgs() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("resolveChatCommandArgs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
