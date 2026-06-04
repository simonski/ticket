package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
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
		return errors.New("tk project remote has been removed; set TICKET_URL instead")
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
		projectRef := fs.String("project_id", resolveConfiguredProjectReference(cfg), "project id, title, prefix, or alias")
		message := fs.String("message", "", "request message")
		if parseErr := fs.Parse(args[1:]); parseErr != nil {
			return parseErr
		}
		if fs.NArg() > 0 {
			joined := strings.TrimSpace(strings.Join(fs.Args(), " "))
			if joined != "" {
				if strings.TrimSpace(*message) != "" {
					return errors.New("usage: tk project request-access [-project_id <id|title|prefix|alias>] [-message <text>] [message words]")
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
		projectRef := fs.String("project_id", resolveConfiguredProjectReference(cfg), "project id, title, prefix, or alias")
		status := fs.String("status", "", "filter by status")
		if parseErr := fs.Parse(args[1:]); parseErr != nil {
			return parseErr
		}
		if fs.NArg() != 0 {
			return errors.New("usage: tk project access-requests [-project_id <id|title|prefix|alias>] [-status <pending|approved|rejected>]")
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
		projectRef := fs.String("project_id", resolveConfiguredProjectReference(cfg), "project id, title, prefix, or alias")
		requestID := fs.Int64("request_id", 0, "project access request id")
		message := fs.String("message", "", "optional decision message")
		if parseErr := fs.Parse(args[1:]); parseErr != nil {
			return parseErr
		}
		if fs.NArg() > 0 {
			joined := strings.TrimSpace(strings.Join(fs.Args(), " "))
			if joined != "" {
				if strings.TrimSpace(*message) != "" {
					return fmt.Errorf("usage: tk project %s [-project_id <id|title|prefix|alias>] -request_id <id> [-message <text>] [message words]", action)
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
		currentProjectMarker, markerErr := resolveProjectListMarker(context.Background(), cfg, svc)
		if markerErr != nil {
			return markerErr
		}
		printProjectTable(projects, currentProjectMarker, workflowNames)
		return nil
	case "get":
		if len(args) > 2 {
			return errors.New("usage: tk project get <id>")
		}
		projectRef := resolveConfiguredProjectReference(cfg)
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
		repositories, workflowName, err := loadProjectSummaryDetails(svc, project)
		if err != nil {
			return err
		}
		printProjectSummary(project, repositories, workflowName)
		return nil
	case "use", "default":
		if len(args) < 2 {
			project, _, err := resolveProjectContext(context.Background(), cfg, svc, resolveConfiguredProjectReference(cfg))
			if err != nil {
				fmt.Println("no project set")
				return nil
			}
			fmt.Printf("%s — %s\n", project.Prefix, project.Title)
			return nil
		}
		return errors.New("tk project use has been removed; pass -project_id or set TICKET_PROJECT instead")
	case "set-default":
		fs := flag.NewFlagSet("project set-default", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRef := fs.String("project_id", resolveConfiguredProjectReference(cfg), "project id, title, prefix, or alias")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() > 0 {
			if strings.TrimSpace(*projectRef) != "" {
				return errors.New("usage: tk project set-default [-project_id <id|title|prefix|alias>] [project-ref]")
			}
			*projectRef = strings.TrimSpace(fs.Arg(0))
		}
		if strings.TrimSpace(*projectRef) == "" {
			return errors.New("project reference is required (pass -project_id or a positional project ref)")
		}
		project, err := svc.SetMyDefaultProject(context.Background(), strings.TrimSpace(*projectRef))
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		fmt.Printf("default project set: %s — %s\n", project.Prefix, project.Title)
		return nil
	case "clear-default":
		if len(args) != 1 {
			return errors.New("usage: tk project clear-default")
		}
		if err := svc.ClearMyDefaultProject(context.Background()); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]string{"status": "cleared"})
		}
		fmt.Println("default project cleared")
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
		project, err := requireCurrentProject(cfg, svc)
		if err != nil {
			return err
		}
		return runProjectByID(svc, project.ID, args)
	case "repo", "repos", "repository", "repositories":
		return runProjectRepository(cfg, svc, args[1:])
	case "set-draft":
		fs := flag.NewFlagSet("project set-draft", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectID := fs.String("project_id", "", "project id, title, prefix, or alias (default: current project)")
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
		project, err := requireCurrentProject(cfg, svc)
		if err != nil {
			return err
		}
		oldPrefix := project.Prefix
		count, err := svc.RenameProjectPrefix(context.Background(), project.ID, newPrefix)
		if err != nil {
			return err
		}
		fmt.Printf("renamed %s → %s (%d tickets updated)\n", oldPrefix, newPrefix, count)
		return nil
	case "rm", "delete":
		fs := flag.NewFlagSet("project rm", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "project id or prefix")
		confirm := fs.String("confirm", "", "repeat the project prefix shown by the first run")
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
			tickets, _ := svc.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, true)
			fmt.Printf("project  : %s — %s\n", project.Prefix, project.Title)
			fmt.Printf("tickets  : %d\n", len(tickets))
			fmt.Printf("\nThis will permanently delete the project and all associated data.\n")
			fmt.Printf("To confirm, run:\n\n")
			fmt.Printf("  tk project rm -id %s --confirm %s\n\n", *id, project.Prefix)
			return nil
		}
		if strings.TrimSpace(*confirm) != project.Prefix {
			return fmt.Errorf("invalid confirmation value: expected %s", project.Prefix)
		}
		if err := svc.DeleteProject(context.Background(), project.ID); err != nil {
			return err
		}
		fmt.Printf("deleted project %s — %s\n", project.Prefix, project.Title)
		return nil
	default:
		return fmt.Errorf("unknown project command %q; see: ticket project help", args[0])
	}
}

func runProjectRepository(cfg config.Config, svc libticket.Service, args []string) error {
	usage := "tk project repo <ls|add|rm> [-project_id <id|prefix|public|private>] [repository]"
	if len(args) == 0 {
		return errors.New(usage)
	}
	fs := flag.NewFlagSet("project repo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	projectID := fs.String("project_id", "", "project id, title, prefix, or alias")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	projectRef := strings.TrimSpace(*projectID)
	if projectRef == "" {
		project, err := requireCurrentProject(cfg, svc)
		if err != nil {
			return err
		}
		projectRef = project.Prefix
	}
	switch strings.TrimSpace(args[0]) {
	case "ls", "list":
		repositories, err := svc.ListProjectGitRepositories(context.Background(), projectRef)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(repositories)
		}
		for _, repository := range repositories {
			fmt.Println(repository)
		}
		return nil
	case "add":
		if fs.NArg() != 1 {
			return errors.New(usage)
		}
		repository := fs.Arg(0)
		if err := svc.AddProjectGitRepository(context.Background(), projectRef, repository); err != nil {
			return err
		}
		fmt.Printf("added repository %s to project %s\n", repository, projectRef)
		return nil
	case "rm", "remove", "delete":
		if fs.NArg() != 1 {
			return errors.New(usage)
		}
		repository := fs.Arg(0)
		if err := svc.RemoveProjectGitRepository(context.Background(), projectRef, repository); err != nil {
			return err
		}
		fmt.Printf("removed repository %s from project %s\n", repository, projectRef)
		return nil
	default:
		return errors.New(usage)
	}
}

func runProjectWorkflow(cfg config.Config, svc libticket.Service, args []string) error {
	usage := "tk project workflow <workflow-id>   (use 0 to clear)"
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(usage)
		return nil
	}
	current, err := requireCurrentProject(cfg, svc)
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
	projectID := fs.String("project_id", "", "project id, title, prefix, or alias")
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
	projectID := fs.String("project_id", "", "project id, title, prefix, or alias")
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
	projectID := fs.String("project_id", "", "project id, title, prefix, or alias")
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
	projectID := fs.String("project_id", "", "project id, title, prefix, or alias")
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
		repositories, workflowName, err := loadProjectSummaryDetails(svc, project)
		if err != nil {
			return err
		}
		printProjectSummary(project, repositories, workflowName)
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

func resolveProjectListMarker(ctx context.Context, cfg config.Config, svc libticket.Service) (string, error) {
	projectRef := strings.TrimSpace(resolveConfiguredProjectReference(cfg))
	if projectRef != "" {
		project, err := svc.GetProject(ctx, projectRef)
		switch {
		case err == nil:
			return strconv.FormatInt(project.ID, 10), nil
		case errors.Is(err, store.ErrProjectNotFound):
			return "", nil
		default:
			return "", err
		}
	}

	repo := strings.TrimSpace(nearestGitRemoteFromCLI())
	if repo != "" {
		project, err := svc.FindProjectByGitRepository(ctx, repo)
		switch {
		case err == nil:
			return strconv.FormatInt(project.ID, 10), nil
		case !errors.Is(err, store.ErrProjectNotFound):
			return "", err
		}
	}
	project, err := svc.GetMyDefaultProject(ctx)
	switch {
	case err == nil:
		return strconv.FormatInt(project.ID, 10), nil
	case errors.Is(err, store.ErrProjectNotFound):
		return "", nil
	default:
		return "", err
	}
}

func loadProjectSummaryDetails(svc libticket.Service, project store.Project) (repositories []string, workflowName string, err error) {
	repositories, err = svc.ListProjectGitRepositories(context.Background(), project.Prefix)
	if err != nil {
		return nil, "", err
	}
	if project.WorkflowID != nil {
		workflow, workflowErr := svc.GetWorkflow(context.Background(), *project.WorkflowID)
		if workflowErr != nil {
			return nil, "", workflowErr
		}
		workflowName = workflow.Name
	}
	return repositories, workflowName, nil
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
		return errors.New("cannot close the current project; switch to another project first with TICKET_PROJECT or -project_id")
	}
	return nil
}
