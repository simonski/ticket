package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/simonski/ticket/internal/config"
)

var gitOriginByRoot sync.Map

func currentOrAncestorProjectRoot() (root string, hasProject bool, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false, err
	}
	if root, ok := config.FindTicketRoot(cwd); ok {
		return root, true, nil
	}
	if gitRoot, ok := config.FindGitRoot(cwd); ok {
		return gitRoot, false, nil
	}
	return cwd, false, nil
}

func detectGitOriginAt(root string) string {
	if cached, ok := gitOriginByRoot.Load(root); ok {
		return cached.(string)
	}
	out, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").Output() // #nosec G204 -- command and arguments are fixed; root is a trusted local path
	if err != nil {
		return ""
	}
	remote := strings.TrimSpace(string(out))
	if remote != "" {
		gitOriginByRoot.Store(root, remote)
	}
	return remote
}

func bindRootToRemoteProject(root, remoteName, projectID string) error {
	if strings.TrimSpace(remoteName) == "" {
		return fmt.Errorf("remote name is required")
	}
	if strings.TrimSpace(projectID) == "" {
		return fmt.Errorf("remote project id is required")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if _, ok := cfg.RemoteByName(remoteName); !ok {
		return fmt.Errorf("remote %q not found", remoteName)
	}
	if err := os.MkdirAll(filepath.Join(root, ".ticket"), 0o750); err != nil {
		return err
	}
	if err := config.SaveProjectConfigAt(root, config.Config{
		Remote:    strings.TrimSpace(remoteName),
		ProjectID: strings.TrimSpace(projectID),
	}); err != nil {
		return err
	}
	cfg.ProjectID = strings.TrimSpace(projectID)
	return config.Save(cfg)
}
