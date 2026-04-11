package main

import (
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/internal/tui"
)

// runGUI launches the full-screen TUI. themeID may be "" or "gui-requested"
// to use the default theme.
func runGUI(themeID string) error {
	if themeID == "gui-requested" {
		themeID = ""
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	var project store.Project
	if cfg.ProjectID != "" {
		project, _ = svc.GetProject(cfg.ProjectID)
	}

	return tui.Run(svc, cfg, project, themeID)
}
