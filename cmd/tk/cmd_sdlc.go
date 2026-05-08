package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func runWorkflow(args []string) error {
	if len(args) == 0 {
		fmt.Println(workflowUsage)
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Println(workflowUsage)
		return nil
	case "list", "ls":
		workflows, err := svc.ListWorkflows(context.Background())
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(workflows)
		}
		printWorkflowTable(workflows)
		return nil
	case "create", "add", "new":
		fs := flag.NewFlagSet("workflow create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "force workflow id")
		printID := fs.Bool("printid", false, "print only the created workflow id")
		name := fs.String("name", "", "workflow name")
		desc := fs.String("d", "", "workflow description")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" {
			return errors.New("usage: tk workflow create -name <name> [-id <id>] [-d <description>]")
		}
		wf, err := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{ID: optionalInt64Flag(*id), Name: *name, Description: *desc})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(wf)
		}
		if printCreatedID(wf.ID, *printID) {
			return nil
		}
		fmt.Printf("workflow: %s\nworkflow_id: %d\n", wf.Name, wf.ID)
		return nil
	case "get":
		fs := flag.NewFlagSet("workflow get", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "workflow id")
		tree := fs.Bool("tree", false, "render workflow as tree (workflow -> phase -> role)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: tk workflow get -id <id> [-tree]")
		}
		wf, err := svc.GetWorkflow(context.Background(), *id)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(wf)
		}
		if *tree {
			printWorkflowTree(wf)
			return nil
		}
		printWorkflowDetail(wf)
		return nil
	case "delete", "rm":
		fs := flag.NewFlagSet("workflow delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "workflow id")
		check := fs.Bool("check", false, "check for references without deleting")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: tk workflow delete -id <id> [-check]")
		}
		if *check {
			return runWorkflowDeleteCheck(context.Background(), svc, *id)
		}
		if err := svc.DeleteWorkflow(context.Background(), *id); err != nil {
			return err
		}
		fmt.Printf("deleted workflow %d\n", *id)
		return nil
	case "add-stage", "stage-add":
		fs := flag.NewFlagSet("workflow add-stage", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		wfID := fs.Int64("id", 0, "workflow id")
		name := fs.String("name", "", "stage name")
		desc := fs.String("d", "", "stage description")
		wow := fs.String("wow", "", "ways of working")
		dor := fs.String("dor", "", "definition of ready")
		dod := fs.String("dod", "", "definition of done")
		order := fs.Int("order", 0, "sort order")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *wfID == 0 || *name == "" {
			return errors.New("usage: tk workflow add-stage -id <workflow_id> -name <stage> [-d <desc>] [-wow <wow>] [-dor <ready>] [-dod <done>] [-order <n>]")
		}
		stageWoW := strings.TrimSpace(*wow)
		if stageWoW == "" {
			stageWoW = *desc
		}
		stage, err := svc.AddWorkflowStage(context.Background(), *wfID, libticket.WorkflowStageRequest{
			StageName:         *name,
			Description:       stageWoW,
			WaysOfWorking:     stageWoW,
			DefinitionOfReady: *dor,
			DefinitionOfDone:  *dod,
			SortOrder:         *order,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(stage)
		}
		fmt.Printf("added stage: %s (id %d)\n", stage.StageName, stage.ID)
		return nil
	case "stage-update":
		fs := flag.NewFlagSet("workflow stage-update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		stageID := fs.Int64("stage-id", 0, "workflow stage id")
		name := fs.String("name", "", "stage name")
		desc := fs.String("d", "", "stage description")
		ac := fs.String("ac", "", "acceptance criteria")
		wow := fs.String("wow", "", "ways of working")
		dor := fs.String("dor", "", "definition of ready")
		dod := fs.String("dod", "", "definition of done")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *stageID == 0 || *name == "" {
			return errors.New("usage: tk workflow stage-update -stage-id <id> -name <name> [-d <desc>] [-ac <criteria>] [-wow <wow>] [-dor <ready>] [-dod <done>]")
		}
		stageWoW := strings.TrimSpace(*wow)
		if stageWoW == "" {
			stageWoW = *desc
		}
		stageDoR := strings.TrimSpace(*dor)
		if stageDoR == "" {
			stageDoR = *ac
		}
		stage, err := svc.UpdateWorkflowStage(context.Background(), *stageID, libticket.WorkflowStageRequest{
			StageName:          *name,
			Description:        stageWoW,
			AcceptanceCriteria: stageDoR,
			WaysOfWorking:      stageWoW,
			DefinitionOfReady:  stageDoR,
			DefinitionOfDone:   *dod,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(stage)
		}
		fmt.Printf("updated stage: %s (id %d)\n", stage.StageName, stage.ID)
		return nil
	case "stage-list":
		fs := flag.NewFlagSet("workflow stage-list", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		workflowID := fs.Int64("id", 0, "workflow id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *workflowID == 0 {
			return errors.New("usage: tk workflow stage-list -id <workflow_id>")
		}
		stages, err := svc.ListWorkflowStages(context.Background(), *workflowID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(stages)
		}
		printStageTable(stages)
		return nil
	case "stage-get":
		fs := flag.NewFlagSet("workflow stage-get", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		stageID := fs.Int64("stage-id", 0, "workflow stage id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *stageID == 0 {
			return errors.New("usage: tk workflow stage-get -stage-id <id>")
		}
		stage, err := svc.GetWorkflowStage(context.Background(), *stageID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(stage)
		}
		printStageDetail(stage)
		return nil
	case "remove-stage", "stage-rm":
		fs := flag.NewFlagSet("workflow remove-stage", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		stageID := fs.Int64("stage-id", 0, "workflow stage id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *stageID == 0 {
			return errors.New("usage: tk workflow remove-stage -stage-id <id>")
		}
		if err := svc.RemoveWorkflowStage(context.Background(), *stageID); err != nil {
			return err
		}
		fmt.Printf("removed stage %d\n", *stageID)
		return nil
	case "reorder-stages", "stage-order":
		fs := flag.NewFlagSet("workflow reorder-stages", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		wfID := fs.Int64("id", 0, "workflow id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *wfID == 0 || fs.NArg() < 1 {
			return errors.New("usage: tk workflow reorder-stages -id <workflow_id> <stage_id,stage_id,...>")
		}
		parts := strings.Split(fs.Arg(0), ",")
		ids := make([]int64, 0, len(parts))
		for _, p := range parts {
			v, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid stage id %q", p)
			}
			ids = append(ids, v)
		}
		if err := svc.ReorderWorkflowStages(context.Background(), *wfID, ids); err != nil {
			return err
		}
		fmt.Println("stages reordered")
		return nil
	case "export":
		fs := flag.NewFlagSet("workflow export", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "workflow id")
		outFile := fs.String("o", "", "output file (default stdout)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: tk workflow export -id <id> [-o file]")
		}
		export, err := svc.ExportWorkflow(context.Background(), *id)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(export, "", "  ")
		if err != nil {
			return err
		}
		if *outFile != "" {
			return os.WriteFile(*outFile, append(data, '\n'), 0o644) // #nosec G306 -- output file is user-specified, 0644 is intentional
		}
		fmt.Println(string(data))
		return nil
	case "import":
		fs := flag.NewFlagSet("workflow import", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		inFile := fs.String("file", "", "input file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *inFile == "" {
			return errors.New("usage: tk workflow import -file <file>")
		}
		data, err := os.ReadFile(*inFile)
		if err != nil {
			return err
		}
		var export store.WorkflowExport
		if unmarshalErr := json.Unmarshal(data, &export); unmarshalErr != nil {
			return unmarshalErr
		}
		wf, err := svc.ImportWorkflow(context.Background(), export)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(wf)
		}
		fmt.Printf("imported workflow: %s (id %d)\n", wf.Name, wf.ID)
		return nil
	case "set":
		fs := flag.NewFlagSet("workflow set", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		ticketID := fs.String("ticket", "", "ticket id")
		workflowID := fs.Int64("workflow", 0, "workflow id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *ticketID == "" || *workflowID == 0 {
			return errors.New("usage: tk workflow set -ticket <ticket-id> -workflow <workflow-id>")
		}
		ticket, err := svc.SetTicketWorkflow(context.Background(), *ticketID, *workflowID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(ticket)
		}
		fmt.Printf("set workflow %d on ticket %s\n", *workflowID, ticket.ID)
		return nil
	case "role-list", "role-ls":
		fs := flag.NewFlagSet("workflow role-list", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		workflowID := fs.Int64("id", 0, "workflow id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *workflowID == 0 {
			return errors.New("usage: tk workflow role-list -id <workflow_id>")
		}
		roles, err := svc.ListRoles(context.Background())
		if err != nil {
			return err
		}
		filtered := make([]store.Role, 0)
		for _, r := range roles {
			if r.WorkflowID != nil && *r.WorkflowID == *workflowID {
				filtered = append(filtered, r)
			}
		}
		if outputJSON {
			return printJSON(filtered)
		}
		printRoleTable(filtered)
		return nil
	case "role-add":
		fs := flag.NewFlagSet("workflow role-add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		workflowID := fs.Int64("workflow_id", 0, "workflow id")
		title := fs.String("title", "", "role title")
		description := fs.String("description", "", "role description")
		ac := fs.String("ac", "", "role acceptance criteria")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *workflowID == 0 || strings.TrimSpace(*title) == "" || fs.NArg() != 0 {
			return errors.New("usage: tk workflow role-add -workflow_id <id> -title <title> [-description <text>] [-ac <text>]")
		}
		role, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
			WorkflowID:         workflowID,
			Title:              strings.TrimSpace(*title),
			Description:        strings.TrimSpace(*description),
			AcceptanceCriteria: strings.TrimSpace(*ac),
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(role)
		}
		fmt.Printf("created workflow role #%d %s\n", role.ID, role.Title)
		return nil
	case "role-get":
		fs := flag.NewFlagSet("workflow role-get", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		workflowID := fs.Int64("workflow_id", 0, "workflow id")
		roleID := fs.Int64("role_id", 0, "role id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *workflowID == 0 || *roleID == 0 || fs.NArg() != 0 {
			return errors.New("usage: tk workflow role-get -workflow_id <id> -role_id <id>")
		}
		role, err := workflowScopedRole(svc, *workflowID, *roleID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(role)
		}
		printRoleDetail(role)
		return nil
	case "role-update":
		fs := flag.NewFlagSet("workflow role-update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		workflowID := fs.Int64("workflow_id", 0, "workflow id")
		roleID := fs.Int64("role_id", 0, "role id")
		title := fs.String("title", "", "role title")
		description := fs.String("description", "", "role description")
		ac := fs.String("ac", "", "role acceptance criteria")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *workflowID == 0 || *roleID == 0 || strings.TrimSpace(*title) == "" || fs.NArg() != 0 {
			return errors.New("usage: tk workflow role-update -workflow_id <id> -role_id <id> -title <title> [-description <text>] [-ac <text>]")
		}
		if _, err := workflowScopedRole(svc, *workflowID, *roleID); err != nil {
			return err
		}
		role, err := svc.UpdateRole(context.Background(), *roleID, libticket.RoleRequest{
			WorkflowID:         workflowID,
			Title:              strings.TrimSpace(*title),
			Description:        strings.TrimSpace(*description),
			AcceptanceCriteria: strings.TrimSpace(*ac),
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(role)
		}
		fmt.Printf("updated workflow role #%d %s\n", role.ID, role.Title)
		return nil
	case "role-rm":
		fs := flag.NewFlagSet("workflow role-rm", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		workflowID := fs.Int64("workflow_id", 0, "workflow id")
		roleID := fs.Int64("role_id", 0, "role id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *workflowID == 0 || *roleID == 0 || fs.NArg() != 0 {
			return errors.New("usage: tk workflow role-rm -workflow_id <id> -role_id <id>")
		}
		if _, err := workflowScopedRole(svc, *workflowID, *roleID); err != nil {
			return err
		}
		if err := svc.DeleteRole(context.Background(), *roleID); err != nil {
			return err
		}
		fmt.Printf("deleted workflow role #%d\n", *roleID)
		return nil
	case "stage-role-add":
		fs := flag.NewFlagSet("workflow stage-role-add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		workflowID := fs.Int64("workflow_id", 0, "workflow id")
		stageID := fs.Int64("stage_id", 0, "stage id")
		roleID := fs.Int64("role_id", 0, "role id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *workflowID == 0 || *stageID == 0 || *roleID == 0 {
			return errors.New("usage: tk workflow stage-role-add -workflow_id <id> -stage_id <id> -role_id <id>")
		}
		if err := svc.AddWorkflowStageRole(context.Background(), *workflowID, *stageID, *roleID); err != nil {
			return err
		}
		fmt.Printf("assigned role #%d to stage #%d\n", *roleID, *stageID)
		return nil
	case "stage-role-rm":
		fs := flag.NewFlagSet("workflow stage-role-rm", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		workflowID := fs.Int64("workflow_id", 0, "workflow id")
		stageID := fs.Int64("stage_id", 0, "stage id")
		roleID := fs.Int64("role_id", 0, "role id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *workflowID == 0 || *stageID == 0 || *roleID == 0 {
			return errors.New("usage: tk workflow stage-role-rm -workflow_id <id> -stage_id <id> -role_id <id>")
		}
		if err := svc.RemoveWorkflowStageRole(context.Background(), *workflowID, *stageID, *roleID); err != nil {
			return err
		}
		fmt.Printf("removed role #%d from stage #%d\n", *roleID, *stageID)
		return nil
	case "stage-role-order":
		fs := flag.NewFlagSet("workflow stage-role-order", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		workflowID := fs.Int64("workflow_id", 0, "workflow id")
		stageID := fs.Int64("stage_id", 0, "stage id")
		roles := fs.String("roles", "", "comma-separated role ids")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *workflowID == 0 || *stageID == 0 || *roles == "" {
			return errors.New("usage: tk workflow stage-role-order -workflow_id <id> -stage_id <id> -roles <id,id,...>")
		}
		parts := strings.Split(*roles, ",")
		roleIDs := make([]int64, 0, len(parts))
		for _, p := range parts {
			v, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid role id %q", p)
			}
			roleIDs = append(roleIDs, v)
		}
		if err := svc.ReorderWorkflowStageRoles(context.Background(), *workflowID, *stageID, roleIDs); err != nil {
			return err
		}
		fmt.Printf("reordered roles in stage #%d\n", *stageID)
		return nil
	case "unset":
		fs := flag.NewFlagSet("workflow unset", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		ticketID := fs.String("ticket", "", "ticket id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *ticketID == "" {
			return errors.New("usage: tk workflow unset -ticket <ticket-id>")
		}
		ticket, err := svc.UnsetTicketWorkflow(context.Background(), *ticketID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(ticket)
		}
		fmt.Printf("cleared workflow on ticket %s (now inherits from parent/project)\n", ticket.ID)
		return nil
	default:
		return fmt.Errorf("unknown workflow command %q; see: ticket workflow help", args[0])
	}
}

func runWorkflowDeleteCheck(ctx context.Context, svc libticket.Service, workflowID int64) error {
	projects, err := svc.ListProjects(ctx)
	if err != nil {
		return err
	}

	projectRefs := make([]string, 0)
	explicitTicketRefs := make([]string, 0)

	for _, p := range projects {
		if p.WorkflowID != nil && *p.WorkflowID == workflowID {
			projectRefs = append(projectRefs, fmt.Sprintf("%s (%s)", p.Prefix, p.Title))
		}
		tickets, err := svc.ListTickets(ctx, p.ID)
		if err != nil {
			return err
		}
		for _, t := range tickets {
			if t.WorkflowID != nil && *t.WorkflowID == workflowID {
				explicitTicketRefs = append(explicitTicketRefs, ticketLabel(t))
			}
		}
	}

	if outputJSON {
		return printJSON(map[string]any{
			"workflow_id":                workflowID,
			"project_references":         projectRefs,
			"explicit_ticket_references": explicitTicketRefs,
			"safe_to_delete":             len(projectRefs) == 0 && len(explicitTicketRefs) == 0,
		})
	}

	fmt.Printf("workflow %d reference check\n", workflowID)
	fmt.Printf("projects using workflow: %d\n", len(projectRefs))
	for _, ref := range projectRefs {
		fmt.Printf("  - %s\n", ref)
	}
	fmt.Printf("tickets with explicit workflow: %d\n", len(explicitTicketRefs))
	for _, ref := range explicitTicketRefs {
		fmt.Printf("  - %s\n", ref)
	}
	if len(projectRefs) == 0 && len(explicitTicketRefs) == 0 {
		fmt.Println("safe to delete")
		return nil
	}
	return fmt.Errorf("workflow %d still has references", workflowID)
}

func printWorkflowTable(workflows []store.Workflow) {
	rows := make([]string, 0, len(workflows))
	for _, wf := range workflows {
		rows = append(rows, fmt.Sprintf("%d\t%s\t%s", wf.ID, wf.Name, wf.Description))
	}
	printBoxTable("ID\tNAME\tDESCRIPTION", rows)
}

func printStageTable(stages []store.WorkflowStage) {
	rows := make([]string, 0, len(stages))
	for _, s := range stages {
		var roleNames []string
		for _, r := range s.Roles {
			roleNames = append(roleNames, r.Title)
		}
		rows = append(rows, fmt.Sprintf("%d\t%d\t%s\t%s\t%s\t%s\t%s", s.SortOrder, s.ID, s.StageName, strings.Join(roleNames, ", "), s.Description, stageDoRValue(s), s.DefinitionOfDone))
	}
	printBoxTable("ORDER\tID\tSTAGE\tROLES\tWOW\tDOR\tDOD", rows)
}

func printStageDetail(s store.WorkflowStage) {
	fmt.Printf("ID                  : %d\n", s.ID)
	fmt.Printf("Workflow ID             : %d\n", s.WorkflowID)
	fmt.Printf("Stage Name          : %s\n", s.StageName)
	fmt.Printf("WoW                 : %s\n", s.Description)
	fmt.Printf("DoR                 : %s\n", stageDoRValue(s))
	fmt.Printf("DoD                 : %s\n", s.DefinitionOfDone)
	fmt.Printf("Description         : %s\n", s.Description)
	fmt.Printf("Acceptance Criteria : %s\n", s.AcceptanceCriteria)
	fmt.Printf("Sort Order          : %d\n", s.SortOrder)
	if len(s.Roles) > 0 {
		var roleNames []string
		for _, r := range s.Roles {
			roleNames = append(roleNames, r.Title)
		}
		fmt.Printf("Roles               : %s\n", strings.Join(roleNames, ", "))
	}
}

func printWorkflowDetail(wf store.WorkflowWithStages) {
	fmt.Printf("ID          : %d\n", wf.ID)
	fmt.Printf("Name        : %s\n", wf.Name)
	fmt.Printf("Description : %s\n", wf.Description)
	fmt.Printf("Stages      :\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  ORDER\tID\tSTAGE\tROLES\tWOW\tDOR\tDOD\tDESCRIPTION\tACCEPTANCE CRITERIA")
	for _, s := range wf.Stages {
		var roleNames []string
		for _, r := range s.Roles {
			roleNames = append(roleNames, r.Title)
		}
		fmt.Fprintf(w, "  %d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", s.SortOrder, s.ID, s.StageName, strings.Join(roleNames, ", "), s.Description, stageDoRValue(s), s.DefinitionOfDone, s.Description, s.AcceptanceCriteria)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not flush Workflow stage table: %v\n", err)
	}
}

func printWorkflowTree(wf store.WorkflowWithStages) {
	fmt.Printf("workflow: %s (%d)\n", wf.Name, wf.ID)
	for stageIndex, stage := range wf.Stages {
		stagePrefix := "├─"
		roleIndent := "│ "
		if stageIndex == len(wf.Stages)-1 {
			stagePrefix = "└─"
			roleIndent = "  "
		}
		fmt.Printf("%s phase: %s (%d)\n", stagePrefix, stage.StageName, stage.ID)
		if len(stage.Roles) == 0 {
			fmt.Printf("%s└─ role: (none)\n", roleIndent)
			continue
		}
		for roleIndex, role := range stage.Roles {
			rolePrefix := "├─"
			if roleIndex == len(stage.Roles)-1 {
				rolePrefix = "└─"
			}
			fmt.Printf("%s%s role: %s (%d)\n", roleIndent, rolePrefix, role.Title, role.ID)
		}
	}
}

func stageDoRValue(s store.WorkflowStage) string {
	if strings.TrimSpace(s.DefinitionOfReady) != "" {
		return s.DefinitionOfReady
	}
	return s.AcceptanceCriteria
}

func workflowScopedRole(svc libticket.Service, workflowID, roleID int64) (store.Role, error) {
	roles, err := svc.ListRoles(context.Background())
	if err != nil {
		return store.Role{}, err
	}
	for _, role := range roles {
		if role.ID != roleID {
			continue
		}
		if role.WorkflowID != nil && *role.WorkflowID == workflowID {
			return role, nil
		}
		break
	}
	return store.Role{}, fmt.Errorf("role %d not found in workflow %d", roleID, workflowID)
}

func printRoleDetail(role store.Role) {
	fmt.Printf("ID:                  %d\n", role.ID)
	if role.WorkflowID != nil {
		fmt.Printf("Workflow ID:             %d\n", *role.WorkflowID)
	}
	fmt.Printf("Title:               %s\n", role.Title)
	fmt.Printf("Description:         %s\n", role.Description)
	fmt.Printf("Acceptance Criteria: %s\n", role.AcceptanceCriteria)
	fmt.Printf("Created:             %s\n", role.CreatedAt)
	fmt.Printf("Updated:             %s\n", role.UpdatedAt)
}
