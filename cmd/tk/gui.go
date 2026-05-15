package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/internal/tui"
	"github.com/simonski/ticket/libticket"
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

	cfg, project, err := resolveGUIProject(context.Background(), cfg, svc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load project %s: %v\n", cfg.ProjectID, err)
	}

	return tui.Run(svc, cfg, project, themeID)
}

func resolveGUIProject(ctx context.Context, cfg config.Config, svc libticket.Service) (config.Config, store.Project, error) {
	project, err := resolveProjectFromFlagOrConfig(ctx, cfg, svc, "")
	if err != nil {
		return cfg, store.Project{}, err
	}
	cfg.ProjectID = strconv.FormatInt(project.ID, 10)
	return cfg, project, nil
}
