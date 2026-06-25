package server

import (
	"os"
	"testing"
)

// TestMain disables the ephemeral agent-task dispatcher for the server test
// suite. Tests drive the queue directly (enqueue/claim/execute), so a live
// background worker would race with them; production keeps it enabled (TK-168).
func TestMain(m *testing.M) {
	roomWorkerEnabled = false
	os.Exit(m.Run())
}
