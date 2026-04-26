package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/libticket"
)

func currentOrAncestorProjectRoot() (string, bool, error) {
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
	out, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").Output()
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

func uniqueProjectPrefix(svc libticket.Service, root string) (string, error) {
	base := defaultProjectPrefix(root)
	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		return "", err
	}
	used := map[string]bool{}
	for _, project := range projects {
		used[strings.ToUpper(strings.TrimSpace(project.Prefix))] = true
	}
	if !used[base] {
		return base, nil
	}
	title := strings.ToUpper(strings.TrimSpace(filepath.Base(root)))
	letters := make([]rune, 0, len(title))
	for _, r := range title {
		if r >= 'A' && r <= 'Z' {
			letters = append(letters, r)
		}
	}
	for l := 3; l <= len(letters) && l <= 5; l++ {
		candidate := string(letters[:l])
		if !used[candidate] {
			return candidate, nil
		}
	}
	stem := base
	if len(stem) > 4 {
		stem = stem[:4]
	}
	for suffix := 'A'; suffix <= 'Z'; suffix++ {
		candidate := stem + string(suffix)
		if !used[candidate] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find unique prefix for %s", root)
}

func ensureLocalDatabase() (config.Config, error) {
	dbPath, err := defaultDatabasePath()
	if err != nil {
		return config.Config{}, err
	}
	if _, err := os.Stat(dbPath); err == nil {
		return ensureDefaultLocalRemote(dbPath)
	} else if !os.IsNotExist(err) {
		return config.Config{}, err
	}
	if err := runInitDB(nil); err != nil {
		return config.Config{}, err
	}
	return ensureDefaultLocalRemote(dbPath)
}

func bindRootToLocalProject(root string, titleOverride, prefixOverride, gitOverride string) error {
	cfg, err := ensureLocalDatabase()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	gitRepo := strings.TrimSpace(gitOverride)
	if gitRepo == "" {
		gitRepo = detectGitOriginAt(root)
	}
	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		return err
	}
	var projectID string
	if gitRepo != "" {
		if match := matchProjectByGitOrigin(projects, gitRepo); match != nil {
			projectID = match.Prefix
		}
	}
	if projectID == "" {
		prefix := strings.ToUpper(strings.TrimSpace(prefixOverride))
		if prefix != "" {
			for _, project := range projects {
				if strings.EqualFold(project.Prefix, prefix) {
					projectID = project.Prefix
					break
				}
			}
		}
	}
	if projectID == "" {
		title := strings.TrimSpace(titleOverride)
		if title == "" {
			title = defaultProjectTitle(root)
		}
		prefix := strings.ToUpper(strings.TrimSpace(prefixOverride))
		if prefix == "" {
			prefix, err = uniqueProjectPrefix(svc, root)
			if err != nil {
				return err
			}
		}
		project, err := svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
			Prefix:        prefix,
			Title:         title,
			GitRepository: gitRepo,
		})
		if err != nil {
			return err
		}
		projectID = project.Prefix
	}
	if err := os.MkdirAll(filepath.Join(root, ".ticket"), 0o750); err != nil {
		return err
	}
	remoteName := strings.TrimSpace(cfg.Remote)
	if remoteName == "" {
		remoteName = strings.TrimSpace(cfg.DefaultRemote)
	}
	projectCfg := config.Config{
		Remote:    remoteName,
		ProjectID: projectID,
	}
	if err := config.SaveProjectConfigAt(root, projectCfg); err != nil {
		return err
	}
	cfg.ProjectID = projectID
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
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	if resolved.Mode == config.ModeRemote {
		return nil
	}
	root, hasProject, err := currentOrAncestorProjectRoot()
	if err != nil {
		return err
	}
	if hasProject {
		return nil
	}
	if outputJSON {
		return nil
	}
	fmt.Println("ticket is not set up yet — creating the local database and binding this repo/directory")
	if err := bindRootToLocalProject(root, "", "", ""); err != nil {
		return err
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
	case "project", "story", "sdlc", "team", "user", "label", "role", "req", "requirement", "agent", "time":
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

func formatProjectID(projectID int64) string {
	return strconv.FormatInt(projectID, 10)
}
