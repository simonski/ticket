package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/libticket"
)

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
	out, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").Output() // #nosec G204 -- command and arguments are fixed; root is a trusted local path
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func defaultProjectTitle(root string) string {
	return filepath.Base(root)
}

func defaultProjectPrefix(root string) string {
	title := strings.ToUpper(strings.TrimSpace(filepath.Base(root)))
	letters := make([]rune, 0, len(title))
	for _, r := range title {
		if r >= 'A' && r <= 'Z' {
			letters = append(letters, r)
		}
	}
	if len(letters) == 0 {
		return "TK"
	}
	if len(letters) == 1 {
		return string(letters[0]) + "X"
	}
	return string(letters[:2])
}

func ensureLocalDatabase() (config.Config, error) {
	dbPath, err := config.LocalDBPath()
	if err != nil {
		return config.Config{}, err
	}
	if _, statErr := os.Stat(dbPath); statErr != nil {
		if initErr := runInitDB([]string{"-f", dbPath}); initErr != nil {
			return config.Config{}, initErr
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, err
	}
	if strings.TrimSpace(cfg.ProjectID) == "" {
		cfg.ProjectID = "1"
		if saveErr := config.Save(cfg); saveErr != nil {
			return config.Config{}, saveErr
		}
	}
	return cfg, nil
}

func bindRootToLocalProject(root, titleOverride, prefixOverride, gitOverride string) error {
	dbPath, err := config.LocalDBPath()
	if err != nil {
		return err
	}
	svc, err := resolveService(config.Config{Location: dbPath, ProjectID: "1"})
	if err != nil {
		return err
	}
	project, err := svc.GetProject(context.Background(), "1")
	if err != nil {
		return err
	}
	if title := strings.TrimSpace(titleOverride); title != "" || strings.TrimSpace(gitOverride) != "" {
		update := libticket.ProjectUpdateRequest{}
		if title != "" {
			update.Title = title
		}
		if git := strings.TrimSpace(gitOverride); git != "" {
			update.GitRepository = git
		}
		if _, updateErr := svc.UpdateProject(context.Background(), project.ID, update); updateErr != nil {
			return updateErr
		}
	}
	if prefix := strings.ToUpper(strings.TrimSpace(prefixOverride)); prefix != "" && prefix != project.Prefix {
		if _, renameErr := svc.RenameProjectPrefix(context.Background(), project.ID, prefix); renameErr != nil {
			return renameErr
		}
	}
	if mkdirErr := os.MkdirAll(filepath.Join(root, ".ticket"), 0o750); mkdirErr != nil {
		return mkdirErr
	}
	if saveProjectErr := config.SaveProjectConfigAt(root, config.Config{ProjectID: "1"}); saveProjectErr != nil {
		return saveProjectErr
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.ProjectID = "1"
	return config.Save(cfg)
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

func maybeBootstrapMutableCommand(args []string) error {
	if len(args) == 0 || !isMutableCommand(args) {
		return nil
	}
	return nil
}

func isMutableCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "add", "create", "new", "edit", "update", "set-parent", "attach", "unset-parent", "detach",
		"stage", "state", "idle", "active", "complete", "reopen", "success", "fail", "next", "previous", "prev",
		"draft", "undraft", "reject", "assign", "unassign", "claim", "unclaim", "add-dependency", "remove-dependency",
		"comment", "clone", "close", "open", "archive", "unarchive", "ready", "notready", "rm", "delete",
		"note", "question", "bug", "epic":
		return true
	case "project", "story", "workflow", "team", "user", "label", "role", "req", "requirement", "agent", "time":
		if len(args) < 2 {
			return false
		}
		switch args[1] {
		case "create", "add", "new", "update", "use", "default", "init", "set-draft", "rename-prefix", "rm", "delete",
			"attach", "detach", "link", "unlink", "log":
			return true
		}
	}
	return false
}

func advisoryNotManagedProject() error {
	root, _, err := currentOrAncestorProjectRoot()
	if err != nil {
		return err
	}
	if gitRoot, ok := config.FindGitRoot(root); ok {
		return fmt.Errorf("not a ticket project — run `tk init` in %s or use a mutable command like `tk new` to bootstrap it", gitRoot)
	}
	return fmt.Errorf("not a ticket project — run `tk init` here or use a mutable command like `tk new` to bootstrap it")
}
