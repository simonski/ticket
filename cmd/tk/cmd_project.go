package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func runProject(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(projectUsage)
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if args[0] == "remote" {
		return runProjectRemote(cfg, args[1:])
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	if projectID, ok := parseProjectCommandID(args[0]); ok {
		return runProjectByID(svc, projectID, args[1:])
	}

	switch args[0] {
	case "request-access":
		fs := flag.NewFlagSet("project request-access", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRef := fs.String("project_id", strings.TrimSpace(cfg.ProjectID), "project id, prefix, or alias")
		message := fs.String("message", "", "request message")
		if parseErr := fs.Parse(args[1:]); parseErr != nil {
			return parseErr
		}
		if fs.NArg() > 0 {
			joined := strings.TrimSpace(strings.Join(fs.Args(), " "))
			if joined != "" {
				if strings.TrimSpace(*message) != "" {
					return errors.New("usage: tk project request-access [-project_id <id|prefix|alias>] [-message <text>] [message words]")
				}
				*message = joined
			}
		}
		if strings.TrimSpace(*projectRef) == "" {
			return errors.New("project_id is required (set an active project or pass -project_id)")
		}
		request, requestErr := svc.CreateProjectAccessRequest(context.Background(), strings.TrimSpace(*projectRef), strings.TrimSpace(*message))
		if requestErr != nil {
			return requestErr
		}
		if outputJSON {
			return printJSON(request)
		}
		fmt.Printf("requested access: request_id=%d project_id=%d status=%s\n", request.ID, request.ProjectID, request.Status)
		return nil
	case "access-requests":
		fs := flag.NewFlagSet("project access-requests", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRef := fs.String("project_id", strings.TrimSpace(cfg.ProjectID), "project id, prefix, or alias")
		status := fs.String("status", "", "filter by status")
		if parseErr := fs.Parse(args[1:]); parseErr != nil {
			return parseErr
		}
		if fs.NArg() != 0 {
			return errors.New("usage: tk project access-requests [-project_id <id|prefix|alias>] [-status <pending|approved|rejected>]")
		}
		if strings.TrimSpace(*projectRef) == "" {
			return errors.New("project_id is required (set an active project or pass -project_id)")
		}
		requests, listErr := svc.ListProjectAccessRequests(context.Background(), strings.TrimSpace(*projectRef), strings.TrimSpace(*status))
		if listErr != nil {
			return listErr
		}
		if outputJSON {
			return printJSON(requests)
		}
		printProjectAccessRequestTable(requests)
		return nil
	case "my-access-requests":
		fs := flag.NewFlagSet("project my-access-requests", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		status := fs.String("status", "", "filter by status")
		if parseErr := fs.Parse(args[1:]); parseErr != nil {
			return parseErr
		}
		if fs.NArg() != 0 {
			return errors.New("usage: tk project my-access-requests [-status <pending|approved|rejected>]")
		}
		requests, listErr := svc.ListMyProjectAccessRequests(context.Background(), strings.TrimSpace(*status))
		if listErr != nil {
			return listErr
		}
		if outputJSON {
			return printJSON(requests)
		}
		printProjectAccessRequestTable(requests)
		return nil
	case "approve-access-request", "reject-access-request":
		action := args[0]
		fs := flag.NewFlagSet("project "+action, flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRef := fs.String("project_id", strings.TrimSpace(cfg.ProjectID), "project id, prefix, or alias")
		requestID := fs.Int64("request_id", 0, "project access request id")
		message := fs.String("message", "", "optional decision message")
		if parseErr := fs.Parse(args[1:]); parseErr != nil {
			return parseErr
		}
		if fs.NArg() > 0 {
			joined := strings.TrimSpace(strings.Join(fs.Args(), " "))
			if joined != "" {
				if strings.TrimSpace(*message) != "" {
					return fmt.Errorf("usage: tk project %s [-project_id <id|prefix|alias>] -request_id <id> [-message <text>] [message words]", action)
				}
				*message = joined
			}
		}
		if strings.TrimSpace(*projectRef) == "" {
			return errors.New("project_id is required (set an active project or pass -project_id)")
		}
		if *requestID <= 0 {
			return errors.New("request_id must be greater than zero")
		}
		status := "approved"
		verb := "approved"
		if action == "reject-access-request" {
			status = "rejected"
			verb = "rejected"
		}
		request, statusErr := svc.SetProjectAccessRequestStatus(context.Background(), strings.TrimSpace(*projectRef), *requestID, status, strings.TrimSpace(*message))
		if statusErr != nil {
			return statusErr
		}
		if outputJSON {
			return printJSON(request)
		}
		fmt.Printf("%s access request: request_id=%d project_id=%d status=%s user=%s", verb, request.ID, request.ProjectID, request.Status, request.Username)
		if strings.TrimSpace(request.DecisionMessage) != "" {
			fmt.Printf(" message=%s", request.DecisionMessage)
		}
		fmt.Println()
		return nil
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
		wow := fs.String("wow", "", "ways of working")
		dor := fs.String("dor", "", "definition of ready")
		dod := fs.String("dod", "", "definition of done")
		dorMapRaw := fs.String("dor-map", "", "stage-specific DoR entries (stage=value,...)")
		dodMapRaw := fs.String("dod-map", "", "stage-specific DoD entries (stage=value,...)")
		acMapRaw := fs.String("ac-map", "", "stage-specific acceptance criteria entries (stage=value,...)")
		gitRepository := fs.String("git-repository", "", "project git repository")
		id := fs.Int64("id", 0, "force project id")
		printID := fs.Bool("printid", false, "print only the created project id")
		workflowID := fs.Int64("workflow", 0, "workflow id to associate")
		if parseErr := fs.Parse(args[1:]); parseErr != nil {
			return parseErr
		}
		if strings.TrimSpace(*title) == "" && fs.NArg() > 0 {
			*title = strings.Join(fs.Args(), " ")
		} else if fs.NArg() != 0 {
			return errors.New("usage: tk project create -title <title> -prefix <prefix> [-id <id>] [-wow text] [-dor text] [-dod text] [-ac text] [-dor-map stage=value,...] [-dod-map stage=value,...] [-ac-map stage=value,...] [-description text] [-workflow id]")
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
		projectWoW := strings.TrimSpace(*wow)
		if projectWoW == "" {
			projectWoW = *description
		}
		projectDORMap, mergeErr := mergeGuidanceMap(nil, *dor, *dorMapRaw, containsFlag(args[1:], "-dor"), containsFlag(args[1:], "-dor-map"))
		if mergeErr != nil {
			return mergeErr
		}
		projectDODMap, mergeErr := mergeGuidanceMap(nil, *dod, *dodMapRaw, containsFlag(args[1:], "-dod"), containsFlag(args[1:], "-dod-map"))
		if mergeErr != nil {
			return mergeErr
		}
		projectACMap, mergeErr := mergeGuidanceMap(nil, *acceptanceCriteria, *acMapRaw, containsFlag(args[1:], "-ac"), containsFlag(args[1:], "-ac-map"))
		if mergeErr != nil {
			return mergeErr
		}
		project, createErr := svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
			ID:                 optionalInt64Flag(*id),
			Prefix:             *prefix,
			Title:              *title,
			Description:        projectWoW,
			AcceptanceCriteria: strings.TrimSpace(*acceptanceCriteria),
			DORMap:             projectDORMap,
			DODMap:             projectDODMap,
			ACMap:              projectACMap,
			Notes:              strings.TrimSpace(*dod),
			GitRepository:      strings.TrimSpace(*gitRepository),
			WorkflowID:         wfID,
		})
		if createErr != nil {
			return createErr
		}
		cfg.ProjectID = project.Prefix
		cfg.CurrentEpicID = ""
		if saveErr := config.Save(cfg); saveErr != nil {
			return saveErr
		}
		if outputJSON {
			return printJSON(project)
		}
		if printCreatedID(project.ID, *printID) {
			return nil
		}
		printProject(project)
		return nil
	case "list", "ls":
		projects, listErr := svc.ListProjects(context.Background())
		if listErr != nil {
			return listErr
		}
		if outputJSON {
			return printJSON(projects)
		}
		workflowNames := map[int64]string{}
		if wfs, listWorkflowsErr := svc.ListWorkflows(context.Background()); listWorkflowsErr == nil {
			for _, wf := range wfs {
				workflowNames[wf.ID] = wf.Name
			}
		}
		printProjectTable(projects, cfg.ProjectID, workflowNames)
		return nil
	case "get":
		if len(args) > 2 {
			return errors.New("usage: tk project get <id>")
		}
		projectRef := strings.TrimSpace(cfg.ProjectID)
		if len(args) == 2 {
			projectRef = strings.TrimSpace(args[1])
		}
		var project store.Project
		if projectRef == "" {
			project, err = mostRecentProject(svc)
			if err != nil {
				return err
			}
		} else {
			project, err = svc.GetProject(context.Background(), projectRef)
			if err != nil {
				return err
			}
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
			project, err := svc.GetProject(context.Background(), cfg.ProjectID)
			if err != nil {
				fmt.Println(cfg.ProjectID)
				return nil
			}
			fmt.Printf("%s — %s\n", project.Prefix, project.Title)
			return nil
		}
		project, _, err := resolveProjectContext(context.Background(), cfg, svc, args[1])
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
			fs.String("status", "", "")
			fs.Int64("workflow", 0, "")
			if err := fs.Parse(args[1:]); err != nil {
				return err
			}
			if *idFlag > 0 {
				return runProjectByID(svc, *idFlag, args)
			}
		}
		if cfg.ProjectID == "" {
			return errors.New("no current project set; use: tk project use <id>")
		}
		project, err := svc.GetProject(context.Background(), cfg.ProjectID)
		if err != nil {
			return err
		}
		return runProjectByID(svc, project.ID, args)
	case "init":
		return runProjectInit(cfg, svc, args[1:])
	case "remote":
		return runProjectRemote(cfg, args[1:])
	case "set-draft":
		fs := flag.NewFlagSet("project set-draft", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectID := fs.String("project_id", "", "project id, prefix, or alias (default: current project)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return errors.New("usage: tk project set-draft [-project_id <id>] <true|false>")
		}
		val := strings.TrimSpace(fs.Arg(0))
		if val != "true" && val != "false" {
			return fmt.Errorf("expected true or false, got %q", val)
		}
		draft := val == "true"
		project, err := resolveProjectFromFlagOrConfig(context.Background(), cfg, svc, *projectID)
		if err != nil {
			return err
		}
		if err := svc.SetProjectDefaultDraft(context.Background(), project.ID, draft); err != nil {
			return err
		}
		fmt.Printf("default_draft set to %v\n", draft)
		return nil
	case "workflow":
		return runProjectWorkflow(cfg, svc, args[1:])
	case "rename-prefix":
		if len(args) < 2 {
			return errors.New("usage: tk project rename-prefix <new-prefix>")
		}
		newPrefix := strings.ToUpper(strings.TrimSpace(args[1]))
		if newPrefix == "" {
			return errors.New("new prefix is required")
		}
		if cfg.ProjectID == "" {
			return errors.New("no current project set; use: tk project use <id>")
		}
		project, err := svc.GetProject(context.Background(), cfg.ProjectID)
		if err != nil {
			return err
		}
		oldPrefix := project.Prefix
		count, err := svc.RenameProjectPrefix(context.Background(), project.ID, newPrefix)
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
			return errors.New("usage: tk project rm [-id] <id> [--confirm <token>]")
		}
		project, err := svc.GetProject(context.Background(), strings.TrimSpace(*id))
		if err != nil {
			return err
		}
		if strings.TrimSpace(*confirm) == "" {
			// Phase 1: generate confirmation token
			token, err := generateConfirmToken()
			if err != nil {
				return err
			}
			tickets, _ := svc.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, true)
			fmt.Printf("project  : %s — %s\n", project.Prefix, project.Title)
			fmt.Printf("tickets  : %d\n", len(tickets))
			fmt.Printf("\nThis will permanently delete the project and all associated data.\n")
			fmt.Printf("To confirm, run:\n\n")
			fmt.Printf("  tk project rm -id %s --confirm %s\n\n", *id, token)
			// Store token temporarily in config
			cfg.DeleteConfirmToken = token
			cfg.DeleteConfirmProject = fmt.Sprintf("%d", project.ID)
			return config.Save(cfg)
		}
		// Phase 2: verify token and delete
		if *confirm != cfg.DeleteConfirmToken || fmt.Sprintf("%d", project.ID) != cfg.DeleteConfirmProject {
			return errors.New("invalid confirmation token")
		}
		if err := svc.DeleteProject(context.Background(), project.ID); err != nil {
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
	prefix := fs.String("prefix", defaultProjectPrefix(cwd), "project prefix (default: derived from directory name)")
	title := fs.String("title", dirName, "project title (default: directory name)")
	description := fs.String("description", dirName, "project description (default: directory name)")
	dor := fs.String("dor", "", "definition of ready")
	dod := fs.String("dod", "", "definition of done")
	ac := fs.String("ac", "", "acceptance criteria")
	dorMapRaw := fs.String("dor-map", "", "stage-specific DoR entries (stage=value,...)")
	dodMapRaw := fs.String("dod-map", "", "stage-specific DoD entries (stage=value,...)")
	acMapRaw := fs.String("ac-map", "", "stage-specific acceptance criteria entries (stage=value,...)")
	if parseErr := fs.Parse(args); parseErr != nil {
		return parseErr
	}

	// Check if a project is already initialised
	if cfg.ProjectID != "" {
		cfgPath, _, _ := config.ProjectPath()
		return fmt.Errorf("project already initialised: %s (in %s)", cfg.ProjectID, cfgPath)
	}

	// Try to find existing project by prefix
	project, err := svc.GetProject(context.Background(), *prefix)
	if err != nil {
		dorMap, mergeErr := mergeGuidanceMap(nil, *dor, *dorMapRaw, containsFlag(args, "-dor"), containsFlag(args, "-dor-map"))
		if mergeErr != nil {
			return mergeErr
		}
		dodMap, mergeErr := mergeGuidanceMap(nil, *dod, *dodMapRaw, containsFlag(args, "-dod"), containsFlag(args, "-dod-map"))
		if mergeErr != nil {
			return mergeErr
		}
		acMap, mergeErr := mergeGuidanceMap(nil, *ac, *acMapRaw, containsFlag(args, "-ac"), containsFlag(args, "-ac-map"))
		if mergeErr != nil {
			return mergeErr
		}
		// Project doesn't exist — create it
		project, err = svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
			Prefix:             *prefix,
			Title:              *title,
			Description:        *description,
			AcceptanceCriteria: strings.TrimSpace(*ac),
			DORMap:             dorMap,
			DODMap:             dodMap,
			ACMap:              acMap,
			Notes:              strings.TrimSpace(*dod),
		})
		if err != nil {
			return err
		}
		fmt.Printf("created project %s (%s)\n", project.Prefix, project.Title)
	} else {
		fmt.Printf("found existing project %s (%s)\n", project.Prefix, project.Title)
	}

	remoteName := strings.TrimSpace(cfg.Remote)
	if remoteName == "" {
		remoteName = strings.TrimSpace(cfg.DefaultRemote)
	}
	return bindRootToRemoteProject(cwd, remoteName, project.Prefix)
}

func runProjectRemote(cfg config.Config, args []string) error {
	if len(args) == 0 {
		if strings.TrimSpace(cfg.Remote) == "" {
			fmt.Println("(none)")
			return nil
		}
		fmt.Println(cfg.Remote)
		return nil
	}
	if len(args) != 1 {
		return errors.New("usage: tk project remote <name>")
	}
	name := strings.TrimSpace(args[0])
	globalCfg, err := config.Load()
	if err != nil {
		return err
	}
	if _, ok := globalCfg.RemoteByName(name); !ok {
		return fmt.Errorf("remote %q not found", name)
	}
	root, _, err := currentOrAncestorProjectRoot()
	if err != nil {
		return err
	}
	if err := config.SaveProjectConfigAt(root, config.Config{Remote: name}); err != nil {
		return err
	}
	fmt.Printf("using remote %s for %s\n", name, root)
	return nil
}

func runProjectWorkflow(cfg config.Config, svc libticket.Service, args []string) error {
	usage := "tk project workflow <workflow-id>   (use 0 to clear)"
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(usage)
		return nil
	}
	if cfg.ProjectID == "" {
		return errors.New("no current project set; use: tk project use <id>")
	}
	current, err := svc.GetProject(context.Background(), cfg.ProjectID)
	if err != nil {
		return err
	}
	wfIDRaw, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
	if err != nil {
		return fmt.Errorf("usage: %s", usage)
	}
	nextWorkflowID := &wfIDRaw
	project, err := svc.UpdateProject(context.Background(), current.ID, libticket.ProjectUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		GitRepository:      current.GitRepository,
		Status:             current.Status,
		WorkflowID:         nextWorkflowID,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(project)
	}
	if wfIDRaw == 0 {
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
	projectID := fs.String("project_id", "", "project id, prefix, or alias")
	role := fs.String("role", "", "project role [observer,commenter,member,admin]")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *userID == "" || strings.TrimSpace(*projectID) == "" || strings.TrimSpace(*role) == "" || fs.NArg() != 0 {
		return errors.New("usage: tk project add-user -user_id <id> -project_id <id|prefix|public|private> -role <observer|commenter|member|admin>")
	}
	project, err := svc.GetProject(context.Background(), strings.TrimSpace(*projectID))
	if err != nil {
		return err
	}
	member, err := svc.AddProjectMember(context.Background(), project.ID, libticket.ProjectMemberRequest{
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
	projectID := fs.String("project_id", "", "project id, prefix, or alias")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *userID == "" || strings.TrimSpace(*projectID) == "" || fs.NArg() != 0 {
		return errors.New("usage: tk project remove-user -user_id <id> -project_id <id|prefix|public|private>")
	}
	project, err := svc.GetProject(context.Background(), strings.TrimSpace(*projectID))
	if err != nil {
		return err
	}
	if err := svc.RemoveProjectMember(context.Background(), project.ID, *userID); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]any{"status": "deleted", "project_id": project.ID, "user_id": *userID})
	}
	fmt.Printf("removed project user: project_id=%d user_id=%s\n", project.ID, *userID)
	return nil
}

func runProjectAddTeam(svc libticket.Service, args []string) error {
	fs := flag.NewFlagSet("project add-team", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	teamID := fs.Int64("team_id", 0, "team id")
	projectID := fs.String("project_id", "", "project id, prefix, or alias")
	role := fs.String("role", "", "project role [observer,commenter,member,admin]")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *teamID == 0 || strings.TrimSpace(*projectID) == "" || strings.TrimSpace(*role) == "" || fs.NArg() != 0 {
		return errors.New("usage: tk project add-team -team_id <id> -project_id <id|prefix|public|private> -role <observer|commenter|member|admin>")
	}
	project, err := svc.GetProject(context.Background(), strings.TrimSpace(*projectID))
	if err != nil {
		return err
	}
	member, err := svc.AddProjectTeamMember(context.Background(), project.ID, libticket.ProjectTeamMemberRequest{
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
	projectID := fs.String("project_id", "", "project id, prefix, or alias")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *teamID == 0 || strings.TrimSpace(*projectID) == "" || fs.NArg() != 0 {
		return errors.New("usage: tk project remove-team -team_id <id> -project_id <id|prefix|public|private>")
	}
	project, err := svc.GetProject(context.Background(), strings.TrimSpace(*projectID))
	if err != nil {
		return err
	}
	if err := svc.RemoveProjectTeamMember(context.Background(), project.ID, *teamID); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]any{"status": "deleted", "project_id": project.ID, "team_id": *teamID})
	}
	fmt.Printf("removed project team: project_id=%d team_id=%d\n", project.ID, *teamID)
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
		project, err := svc.GetProject(context.Background(), strconv.FormatInt(projectID, 10))
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
		wow := fs.String("wow", "", "ways of working")
		dor := fs.String("dor", "", "definition of ready")
		dod := fs.String("dod", "", "definition of done")
		dorMapRaw := fs.String("dor-map", "", "stage-specific DoR entries (stage=value,...)")
		dodMapRaw := fs.String("dod-map", "", "stage-specific DoD entries (stage=value,...)")
		acMapRaw := fs.String("ac-map", "", "stage-specific acceptance criteria entries (stage=value,...)")
		gitRepository := fs.String("git-repository", "", "project git repository")
		gitShort := fs.String("git", "", "project git repository (shorthand for -git-repository)")
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
		current, err := svc.GetProject(context.Background(), strconv.FormatInt(projectID, 10))
		if err != nil {
			return err
		}
		nextDescription := current.Description
		nextAC := current.AcceptanceCriteria
		nextNotes := current.Notes
		nextRepo := current.GitRepository
		nextStatus := current.Status
		if fs.Lookup("description") != nil && strings.TrimSpace(*description) != "" || containsFlag(args[1:], "-description") {
			nextDescription = *description
		}
		if containsFlag(args[1:], "-wow") {
			nextDescription = strings.TrimSpace(*wow)
		}
		if containsFlag(args[1:], "-dod") {
			nextNotes = strings.TrimSpace(*dod)
		}
		if containsFlag(args[1:], "-ac") {
			nextAC = strings.TrimSpace(*acceptanceCriteria)
		}
		if containsFlag(args[1:], "-git-repository") || containsFlag(args[1:], "-git") {
			nextRepo = strings.TrimSpace(*gitRepository)
		}
		if containsFlag(args[1:], "-status") && strings.TrimSpace(*status) != "" {
			nextStatus = strings.TrimSpace(*status)
		}
		if nextStatus == "closed" {
			if guardErr := guardProjectClose(svc, projectID); guardErr != nil {
				return guardErr
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
		nextDORMap, err := mergeGuidanceMap(current.DORMap, *dor, *dorMapRaw, containsFlag(args[1:], "-dor"), containsFlag(args[1:], "-dor-map"))
		if err != nil {
			return err
		}
		nextDODMap, err := mergeGuidanceMap(current.DODMap, *dod, *dodMapRaw, containsFlag(args[1:], "-dod"), containsFlag(args[1:], "-dod-map"))
		if err != nil {
			return err
		}
		nextACMap, err := mergeGuidanceMap(current.ACMap, *acceptanceCriteria, *acMapRaw, containsFlag(args[1:], "-ac"), containsFlag(args[1:], "-ac-map"))
		if err != nil {
			return err
		}
		project, err := svc.UpdateProject(context.Background(), projectID, libticket.ProjectUpdateRequest{
			Title:              *title,
			Description:        nextDescription,
			AcceptanceCriteria: nextAC,
			DORMap:             nextDORMap,
			DODMap:             nextDODMap,
			ACMap:              nextACMap,
			Notes:              nextNotes,
			GitRepository:      nextRepo,
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
		project, err := svc.SetProjectEnabled(context.Background(), projectID, true)
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
		project, err := svc.SetProjectEnabled(context.Background(), projectID, false)
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
	projects, err := svc.ListProjects(context.Background())
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
