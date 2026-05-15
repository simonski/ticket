package main

import (
	"context"
	"testing"

	"github.com/simonski/ticket/internal/config"
)

func TestResolveWorkItemProjectID(t *testing.T) {
	setupLocalCLI(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	svc, err := resolveService(cfg)
	if err != nil {
		t.Fatalf("resolveService() error = %v", err)
	}
	projectID, err := resolveWorkItemProjectID(context.Background(), cfg, svc, "public")
	if err != nil {
		t.Fatalf("resolveWorkItemProjectID(public) error = %v", err)
	}
	if projectID == 0 {
		t.Fatalf("project id = %d, want non-zero", projectID)
	}
	projectID, err = resolveWorkItemProjectID(context.Background(), cfg, svc, "private")
	if err != nil {
		t.Fatalf("resolveWorkItemProjectID(private) error = %v", err)
	}
	if projectID == 0 {
		t.Fatalf("private project id = %d, want non-zero", projectID)
	}
	projectID, err = resolveWorkItemProjectID(context.Background(), config.Config{ProjectID: ""}, svc, "")
	if err != nil {
		t.Fatalf("resolveWorkItemProjectID(fallback) error = %v", err)
	}
	if projectID == 0 {
		t.Fatal("fallback project id = 0, want non-zero")
	}
}
