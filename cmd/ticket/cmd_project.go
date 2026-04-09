package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/libticket"
)

func runProject(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(projectUsage)
		return nil
	}

	if projectID, ok := parseProjectCommandID(args[0]); ok {
		return runProjectByID(svc, projectID, args[1:])
	}

	switch args[0] {
	case "add-user":
		return runProjectAddUser(svc, args[1:])
	case "remove-user":
		return runProjectRemoveUser(svc, args[1:])
	case "add-team":
		return runProjectAddTeam(svc, args[1:])
	case "remove-team":
		return runProjectRemoveTeam(svc, args[1:])
	case "create", "add", "new":
		fs := flag.NewFlagSet("project create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		prefix := fs.String("prefix", "", "project prefix")
		title := fs.String("title", "", "project title")
		description := fs.String("description", "", "project description")
		acceptanceCriteria := fs.String("ac", "", "project acceptance criteria")
		gitRepository := fs.String("git-repository", "", "project git repository")
		gitBranch := fs.String("git-branch", "", "project git branch")
		workflowID := fs.Int64("workflow", 0, "workflow id to associate")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("usage: ticket project create -title <title> -prefix <prefix> [-description text] [-ac text] [-workflow id]")
		}
		if strings.TrimSpace(*prefix) == "" {
			return errors.New("project prefix is required")
		}
		if strings.TrimSpace(*title) == "" {
			return errors.New("project title is required")
		}
		var wfID *int64
		if *workflowID > 0 {
			wfID = workflowID
		}
		project, err := svc.CreateProject(libticket.ProjectCreateRequest{
			Prefix:             *prefix,
			Title:              *title,
			Description:        *description,
			AcceptanceCriteria: *acceptanceCriteria,
			GitRepository:      strings.TrimSpace(*gitRepository),
			GitBranch:          strings.TrimSpace(*gitBranch),
			WorkflowID:         wfID,
		})
		if err != nil {
			return err
		}
		cfg.ProjectID = project.Prefix
		cfg.CurrentEpicID = ""
		if err := config.Save(cfg); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	case "list", "ls":
		projects, err := svc.ListProjects()
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(projects)
		}
		workflowNames := map[int64]string{}
		if wfs, err := svc.ListWorkflows(); err == nil {
			for _, wf := range wfs {
				workflowNames[wf.ID] = wf.Name
			}
		}
		printProjectTable(projects, cfg.ProjectID, workflowNames)
		return nil
	case "get":
		if len(args) != 2 {
			return errors.New("usage: ticket project get <id>")
		}
		project, err := svc.GetProject(args[1])
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	case "use", "default":
		if len(args) < 2 {
			// No ID: print the current project
			if cfg.ProjectID == "" {
				fmt.Println("no project set")
				return nil
			}
			project, err := svc.GetProject(cfg.ProjectID)
			if err != nil {
				fmt.Println(cfg.ProjectID)
				return nil
			}
			fmt.Printf("%s — %s\n", project.Prefix, project.Title)
			return nil
		}
		project, err := svc.GetProject(args[1])
		if err != nil {
			return err
		}
		cfg.ProjectID = project.Prefix
		cfg.CurrentEpicID = ""
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("using project %s\n", project.Prefix)
		return nil
	case "update":
		if containsFlag(args[1:], "-id") {
			// Parse -id from args so we don't require a current project
			fs := flag.NewFlagSet("project update id", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			idFlag := fs.Int64("id", 0, "")
			// Absorb all other flags so Parse doesn't fail on them
			fs.String("title", "", "")
			fs.String("description", "", "")
			fs.String("ac", "", "")
			fs.String("git-repository", "", "")
			fs.String("git", "", "")
			fs.String("git-branch", "", "")
			fs.String("status", "", "")
			fs.Int64("workflow", 0, "")
			_ = fs.Parse(args[1:])
			if *idFlag > 0 {
				return runProjectByID(svc, *idFlag, args)
			}
		}
		if cfg.ProjectID == "" {
			return errors.New("no current project set; use: tk project use <id>")
		}
		project, err := svc.GetProject(cfg.ProjectID)
		if err != nil {
			return err
		}
		return runProjectByID(svc, project.ID, args)
	case "init":
		return runProjectInit(cfg, svc, args[1:])
	case "workflow":
		return runProjectWorkflow(cfg, svc, args[1:])
	case "rename-prefix":
		if len(args) < 2 {
			return errors.New("usage: ticket project rename-prefix <new-prefix>")
		}
		newPrefix := strings.ToUpper(strings.TrimSpace(args[1]))
		if newPrefix == "" {
			return errors.New("new prefix is required")
		}
		if cfg.ProjectID == "" {
			return errors.New("no current project set; use: tk project use <id>")
		}
		project, err := svc.GetProject(cfg.ProjectID)
		if err != nil {
			return err
		}
		oldPrefix := project.Prefix
		count, err := svc.RenameProjectPrefix(project.ID, newPrefix)
		if err != nil {
			return err
		}
		// Update config to point to the new prefix.
		cfg.ProjectID = newPrefix
		// Update current_epic_id if it references the old prefix.
		if strings.HasPrefix(cfg.CurrentEpicID, oldPrefix+"-") {
			cfg.CurrentEpicID = newPrefix + cfg.CurrentEpicID[len(oldPrefix):]
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("renamed %s → %s (%d tickets updated)\n", oldPrefix, newPrefix, count)
		return nil
	case "rm", "delete":
		fs := flag.NewFlagSet("project rm", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "project id or prefix")
		confirm := fs.String("confirm", "", "confirmation token from first run")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*id) == "" && fs.NArg() == 1 {
			v := fs.Arg(0)
			id = &v
		}
		if strings.TrimSpace(*id) == "" {
			return errors.New("usage: ticket project rm [-id] <id> [--confirm <token>]")
		}
		project, err := svc.GetProject(strings.TrimSpace(*id))
		if err != nil {
			return err
		}
		if strings.TrimSpace(*confirm) == "" {
			// Phase 1: generate confirmation token
			token, err := generateConfirmToken()
			if err != nil {
				return err
			}
			tickets, _ := svc.ListTicketsFiltered(project.ID, "", "", "", "", "", "", 0, true)
			fmt.Printf("project  : %s — %s\n", project.Prefix, project.Title)
			fmt.Printf("tickets  : %d\n", len(tickets))
			fmt.Printf("\nThis will permanently delete the project and all associated data.\n")
			fmt.Printf("To confirm, run:\n\n")
			fmt.Printf("  ticket project rm -id %s --confirm %s\n\n", *id, token)
			// Store token temporarily in config
			cfg.DeleteConfirmToken = token
			cfg.DeleteConfirmProject = fmt.Sprintf("%d", project.ID)
			return config.Save(cfg)
		}
		// Phase 2: verify token and delete
		if *confirm != cfg.DeleteConfirmToken || fmt.Sprintf("%d", project.ID) != cfg.DeleteConfirmProject {
			return errors.New("invalid confirmation token")
		}
		if err := svc.DeleteProject(project.ID); err != nil {
			return err
		}
		// Clear stored token and switch project if needed
		cfg.DeleteConfirmToken = ""
		cfg.DeleteConfirmProject = ""
		if cfg.ProjectID == project.Prefix || cfg.ProjectID == fmt.Sprintf("%d", project.ID) {
			cfg.ProjectID = ""
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("deleted project %s — %s\n", project.Prefix, project.Title)
		return nil
	default:
		return fmt.Errorf("unknown project command %q; see: ticket project help", args[0])
	}
}

func runProjectInit(cfg config.Config, svc libticket.Service, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	dirName := filepath.Base(cwd)

	fs := flag.NewFlagSet("project init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	prefix := fs.String("prefix", strings.ToUpper(dirName[:min(3, len(dirName))]), "project prefix (default: first 3 chars of dir name)")
	title := fs.String("title", dirName, "project title (default: directory name)")
	description := fs.String("description", dirName, "project description (default: directory name)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if a project is already initialised
	if cfg.ProjectID != "" {
		cfgPath, _ := config.Path()
		return fmt.Errorf("project already initialised: %s (in %s)", cfg.ProjectID, cfgPath)
	}

	// Try to find existing project by prefix
	project, err := svc.GetProject(*prefix)
	if err != nil {
		// Project doesn't exist — create it
		project, err = svc.CreateProject(libticket.ProjectCreateRequest{
			Prefix:      *prefix,
			Title:       *title,
			Description: *description,
		})
		if err != nil {
			return err
		}
		fmt.Printf("created project %s (%s)\n", project.Prefix, project.Title)
	} else {
		fmt.Printf("found existing project %s (%s)\n", project.Prefix, project.Title)
	}

	cfg.ProjectID = project.Prefix
	cfg.CurrentEpicID = ""
	if err := config.Save(cfg); err != nil {
		return err
	}
	return nil
}

func runProjectWorkflow(cfg config.Config, svc libticket.Service, args []string) error {
	usage := "ticket project workflow <workflow-id>   (use 0 to clear)"
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(usage)
		return nil
	}
	if cfg.ProjectID == "" {
		return errors.New("no current project set; use: tk project use <id>")
	}
	current, err := svc.GetProject(cfg.ProjectID)
	if err != nil {
		return err
	}
	wfIDRaw, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
	if err != nil {
		return fmt.Errorf("usage: %s", usage)
	}
	var nextWorkflowID *int64
	if wfIDRaw > 0 {
		nextWorkflowID = &wfIDRaw
	}
	project, err := svc.UpdateProject(current.ID, libticket.ProjectUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		GitRepository:      current.GitRepository,
		GitBranch:          current.GitBranch,
		Status:             current.Status,
		WorkflowID:         nextWorkflowID,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(project)
	}
	if nextWorkflowID == nil {
		fmt.Printf("cleared workflow from project %s\n", project.Prefix)
	} else {
		fmt.Printf("set workflow %d on project %s\n", wfIDRaw, project.Prefix)
	}
	printProject(project)
	return nil
}

func runProjectAddUser(svc libticket.Service, args []string) error {
	fs := flag.NewFlagSet("project add-user", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	userID := fs.String("user_id", "", "user id")
	projectID := fs.Int64("project_id", 0, "project id")
	role := fs.String("role", "", "project role [viewer,editor,owner]")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *userID == "" || *projectID == 0 || strings.TrimSpace(*role) == "" || fs.NArg() != 0 {
		return errors.New("usage: ticket project add-user -user_id <id> -project_id <id> -role <viewer|editor|owner>")
	}
	member, err := svc.AddProjectMember(*projectID, libticket.ProjectMemberRequest{
		UserID: *userID,
		Role:   strings.TrimSpace(*role),
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(member)
	}
	fmt.Printf("added project user: project_id=%d user_id=%s role=%s\n", member.ProjectID, member.UserID, member.Role)
	return nil
}

func runProjectRemoveUser(svc libticket.Service, args []string) error {
	fs := flag.NewFlagSet("project remove-user", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	userID := fs.String("user_id", "", "user id")
	projectID := fs.Int64("project_id", 0, "project id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *userID == "" || *projectID == 0 || fs.NArg() != 0 {
		return errors.New("usage: ticket project remove-user -user_id <id> -project_id <id>")
	}
	if err := svc.RemoveProjectMember(*projectID, *userID); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]any{"status": "deleted", "project_id": *projectID, "user_id": *userID})
	}
	fmt.Printf("removed project user: project_id=%d user_id=%s\n", *projectID, *userID)
	return nil
}

func runProjectAddTeam(svc libticket.Service, args []string) error {
	fs := flag.NewFlagSet("project add-team", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	teamID := fs.Int64("team_id", 0, "team id")
	projectID := fs.Int64("project_id", 0, "project id")
	role := fs.String("role", "", "project role [viewer,editor,owner]")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *teamID == 0 || *projectID == 0 || strings.TrimSpace(*role) == "" || fs.NArg() != 0 {
		return errors.New("usage: ticket project add-team -team_id <id> -project_id <id> -role <viewer|editor|owner>")
	}
	member, err := svc.AddProjectTeamMember(*projectID, libticket.ProjectTeamMemberRequest{
		TeamID: *teamID,
		Role:   strings.TrimSpace(*role),
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(member)
	}
	fmt.Printf("added project team: project_id=%d team_id=%d role=%s\n", member.ProjectID, member.TeamID, member.Role)
	return nil
}

func runProjectRemoveTeam(svc libticket.Service, args []string) error {
	fs := flag.NewFlagSet("project remove-team", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	teamID := fs.Int64("team_id", 0, "team id")
	projectID := fs.Int64("project_id", 0, "project id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *teamID == 0 || *projectID == 0 || fs.NArg() != 0 {
		return errors.New("usage: ticket project remove-team -team_id <id> -project_id <id>")
	}
	if err := svc.RemoveProjectTeamMember(*projectID, *teamID); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]any{"status": "deleted", "project_id": *projectID, "team_id": *teamID})
	}
	fmt.Printf("removed project team: project_id=%d team_id=%d\n", *projectID, *teamID)
	return nil
}

func parseProjectCommandID(raw string) (int64, bool) {
	var id int64
	if _, err := fmt.Sscan(raw, &id); err != nil {
		return 0, false
	}
	return id, true
}

func runProjectByID(svc libticket.Service, projectID int64, args []string) error {
	if len(args) == 0 {
		project, err := svc.GetProject(strconv.FormatInt(projectID, 10))
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	}
	switch args[0] {
	case "update":
		fs := flag.NewFlagSet("project update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.Int64("id", 0, "project ID (overrides positional ID)")
		title := fs.String("title", "", "project title")
		description := fs.String("description", "", "project description")
		acceptanceCriteria := fs.String("ac", "", "project acceptance criteria")
		gitRepository := fs.String("git-repository", "", "project git repository")
		gitShort := fs.String("git", "", "project git repository (shorthand for -git-repository)")
		gitBranch := fs.String("git-branch", "", "project git branch")
		status := fs.String("status", "", "project status (open|closed)")
		workflowID := fs.Int64("workflow", 0, "workflow ID to associate with project")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *idFlag > 0 {
			projectID = *idFlag
		}
		if containsFlag(args[1:], "-git") && !containsFlag(args[1:], "-git-repository") {
			gitRepository = gitShort
		}
		current, err := svc.GetProject(strconv.FormatInt(projectID, 10))
		if err != nil {
			return err
		}
		nextDescription := current.Description
		nextAC := current.AcceptanceCriteria
		nextRepo := current.GitRepository
		nextBranch := current.GitBranch
		nextStatus := current.Status
		if fs.Lookup("description") != nil && strings.TrimSpace(*description) != "" || containsFlag(args[1:], "-description") {
			nextDescription = *description
		}
		if containsFlag(args[1:], "-ac") {
			nextAC = *acceptanceCriteria
		}
		if containsFlag(args[1:], "-git-repository") || containsFlag(args[1:], "-git") {
			nextRepo = strings.TrimSpace(*gitRepository)
		}
		if containsFlag(args[1:], "-git-branch") {
			nextBranch = strings.TrimSpace(*gitBranch)
		}
		if containsFlag(args[1:], "-status") && strings.TrimSpace(*status) != "" {
			nextStatus = strings.TrimSpace(*status)
		}
		if nextStatus == "closed" {
			if err := guardProjectClose(svc, projectID); err != nil {
				return err
			}
		}
		var nextWorkflowID *int64
		if containsFlag(args[1:], "-workflow") {
			if *workflowID > 0 {
				nextWorkflowID = workflowID
			}
		} else {
			nextWorkflowID = current.WorkflowID
		}
		project, err := svc.UpdateProject(projectID, libticket.ProjectUpdateRequest{
			Title:              *title,
			Description:        nextDescription,
			AcceptanceCriteria: nextAC,
			GitRepository:      nextRepo,
			GitBranch:          nextBranch,
			Status:             nextStatus,
			WorkflowID:         nextWorkflowID,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	case "enable":
		project, err := svc.SetProjectEnabled(projectID, true)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	case "disable":
		if err := guardProjectClose(svc, projectID); err != nil {
			return err
		}
		project, err := svc.SetProjectEnabled(projectID, false)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	default:
		return fmt.Errorf("unknown project command %q; see: ticket project help", args[0])
	}
}

// guardProjectClose returns an error if closing the given project is not allowed.
// A project may not be closed if it is the current project and there are no other
// open projects to switch to.
func guardProjectClose(svc libticket.Service, projectID int64) error {
	cfg, _ := config.Load()
	projects, err := svc.ListProjects()
	if err != nil {
		return err
	}
	// Count open projects and check whether this project is the current one.
	isCurrent := false
	openCount := 0
	for _, p := range projects {
		if p.Status == "open" {
			openCount++
		}
		if p.ID == projectID {
			if strings.EqualFold(p.Prefix, cfg.ProjectID) || strconv.FormatInt(p.ID, 10) == cfg.ProjectID {
				isCurrent = true
			}
		}
	}
	if isCurrent && openCount <= 1 {
		return errors.New("cannot close the current project when it is the only open project; create another project or switch to one first")
	}
	if isCurrent {
		return errors.New("cannot close the current project; switch to another project first (tk project use <id>)")
	}
	return nil
}
