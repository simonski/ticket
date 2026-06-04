package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("TICKET_FAST_HASH") == "" {
		if err := os.Setenv("TICKET_FAST_HASH", "1"); err != nil {
			panic(err)
		}
	}
	for _, key := range []string{"TICKET_URL", "TICKET_USERNAME", "TICKET_PASSWORD", "TICKET_TOKEN", "TICKET_PROJECT"} {
		if err := os.Unsetenv(key); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}
