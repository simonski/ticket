package main

import (
	"testing"

	"github.com/simonski/ticket/internal/config"
)

func TestResolveWorkItemProjectID(t *testing.T) {
	t.Parallel()
	projectID, err := resolveWorkItemProjectID(config.Config{ProjectID: "7"}, 0)
	if err != nil {
		t.Fatalf("resolveWorkItemProjectID() error = %v", err)
	}
	if projectID != 7 {
		t.Fatalf("project id = %d, want 7", projectID)
	}
	projectID, err = resolveWorkItemProjectID(config.Config{ProjectID: ""}, 9)
	if err != nil {
		t.Fatalf("resolveWorkItemProjectID(provided) error = %v", err)
	}
	if projectID != 9 {
		t.Fatalf("project id = %d, want 9", projectID)
	}
	if _, err := resolveWorkItemProjectID(config.Config{ProjectID: ""}, 0); err == nil {
		t.Fatal("expected error when no project id is available")
	}
}
