package main

import (
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
		workflows, err := svc.ListWorkflows()
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
		name := fs.String("name", "", "workflow name")
		desc := fs.String("d", "", "workflow description")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" {
			return errors.New("usage: ticket workflow create -name <name> [-d <description>]")
		}
		wf, err := svc.CreateWorkflow(libticket.WorkflowRequest{Name: *name, Description: *desc})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(wf)
		}
		fmt.Printf("workflow: %s\nworkflow_id: %d\n", wf.Name, wf.ID)
		return nil
	case "get":
		fs := flag.NewFlagSet("workflow get", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "workflow id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: ticket workflow get -id <id>")
		}
		wf, err := svc.GetWorkflow(*id)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(wf)
		}
		printWorkflowDetail(wf)
		return nil
	case "delete", "rm":
		fs := flag.NewFlagSet("workflow delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "workflow id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: ticket workflow delete -id <id>")
		}
		if err := svc.DeleteWorkflow(*id); err != nil {
			return err
		}
		fmt.Printf("deleted workflow %d\n", *id)
		return nil
	case "add-stage":
		fs := flag.NewFlagSet("workflow add-stage", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		wfID := fs.Int64("id", 0, "workflow id")
		name := fs.String("name", "", "stage name")
		desc := fs.String("d", "", "stage description")
		roleID := fs.Int64("role", 0, "role id")
		order := fs.Int("order", 0, "sort order")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *wfID == 0 || *name == "" {
			return errors.New("usage: ticket workflow add-stage -id <workflow_id> -name <stage> [-role <role_id>] [-d <desc>] [-order <n>]")
		}
		var rID *int64
		if *roleID > 0 {
			rID = roleID
		}
		stage, err := svc.AddWorkflowStage(*wfID, libticket.WorkflowStageRequest{
			StageName:   *name,
			Description: *desc,
			RoleID:      rID,
			SortOrder:   *order,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(stage)
		}
		fmt.Printf("added stage: %s (id %d)\n", stage.StageName, stage.ID)
		return nil
	case "remove-stage":
		fs := flag.NewFlagSet("workflow remove-stage", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		stageID := fs.Int64("stage-id", 0, "workflow stage id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *stageID == 0 {
			return errors.New("usage: ticket workflow remove-stage -stage-id <id>")
		}
		if err := svc.RemoveWorkflowStage(*stageID); err != nil {
			return err
		}
		fmt.Printf("removed stage %d\n", *stageID)
		return nil
	case "reorder-stages":
		fs := flag.NewFlagSet("workflow reorder-stages", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		wfID := fs.Int64("id", 0, "workflow id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *wfID == 0 || fs.NArg() < 1 {
			return errors.New("usage: ticket workflow reorder-stages -id <workflow_id> <stage_id,stage_id,...>")
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
		if err := svc.ReorderWorkflowStages(*wfID, ids); err != nil {
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
			return errors.New("usage: ticket workflow export -id <id> [-o file]")
		}
		export, err := svc.ExportWorkflow(*id)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(export, "", "  ")
		if err != nil {
			return err
		}
		if *outFile != "" {
			return os.WriteFile(*outFile, append(data, '\n'), 0o644)
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
			return errors.New("usage: ticket workflow import -file <file>")
		}
		data, err := os.ReadFile(*inFile)
		if err != nil {
			return err
		}
		var export store.WorkflowExport
		if err := json.Unmarshal(data, &export); err != nil {
			return err
		}
		wf, err := svc.ImportWorkflow(export)
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
			return errors.New("usage: ticket workflow set -ticket <ticket-id> -workflow <workflow-id>")
		}
		ticket, err := svc.SetTicketWorkflow(*ticketID, *workflowID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(ticket)
		}
		fmt.Printf("set workflow %d on ticket %s\n", *workflowID, ticket.ID)
		return nil
	case "unset":
		fs := flag.NewFlagSet("workflow unset", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		ticketID := fs.String("ticket", "", "ticket id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *ticketID == "" {
			return errors.New("usage: ticket workflow unset -ticket <ticket-id>")
		}
		ticket, err := svc.UnsetTicketWorkflow(*ticketID)
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

func printWorkflowTable(workflows []store.Workflow) {
	rows := make([]string, 0, len(workflows))
	for _, wf := range workflows {
		rows = append(rows, fmt.Sprintf("%d\t%s\t%s", wf.ID, wf.Name, wf.Description))
	}
	printBoxTable("ID\tNAME\tDESCRIPTION", rows)
}

func printWorkflowDetail(wf store.WorkflowWithStages) {
	fmt.Printf("ID          : %d\n", wf.ID)
	fmt.Printf("Name        : %s\n", wf.Name)
	fmt.Printf("Description : %s\n", wf.Description)
	fmt.Printf("Stages      :\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  ORDER\tID\tSTAGE\tROLE\tDESCRIPTION")
	for _, s := range wf.Stages {
		fmt.Fprintf(w, "  %d\t%d\t%s\t%s\t%s\n", s.SortOrder, s.ID, s.StageName, s.RoleTitle, s.Description)
	}
	_ = w.Flush()
}
