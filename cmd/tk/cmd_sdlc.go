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

func runSdlc(args []string) error {
	if len(args) == 0 {
		fmt.Println(sdlcUsage)
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
		fmt.Println(sdlcUsage)
		return nil
	case "list", "ls":
		sdlcs, err := svc.ListSdlcs()
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(sdlcs)
		}
		printSdlcTable(sdlcs)
		return nil
	case "create", "add", "new":
		fs := flag.NewFlagSet("sdlc create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "sdlc name")
		desc := fs.String("d", "", "sdlc description")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" {
			return errors.New("usage: ticket sdlc create -name <name> [-d <description>]")
		}
		wf, err := svc.CreateSdlc(libticket.SdlcRequest{Name: *name, Description: *desc})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(wf)
		}
		fmt.Printf("sdlc: %s\nsdlc_id: %d\n", wf.Name, wf.ID)
		return nil
	case "get":
		fs := flag.NewFlagSet("sdlc get", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "sdlc id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: ticket sdlc get -id <id>")
		}
		wf, err := svc.GetSdlc(*id)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(wf)
		}
		printSdlcDetail(wf)
		return nil
	case "delete", "rm":
		fs := flag.NewFlagSet("sdlc delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "sdlc id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: ticket sdlc delete -id <id>")
		}
		if err := svc.DeleteSdlc(*id); err != nil {
			return err
		}
		fmt.Printf("deleted sdlc %d\n", *id)
		return nil
	case "add-stage", "stage-add":
		fs := flag.NewFlagSet("sdlc add-stage", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		wfID := fs.Int64("id", 0, "sdlc id")
		name := fs.String("name", "", "stage name")
		desc := fs.String("d", "", "stage description")
		order := fs.Int("order", 0, "sort order")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *wfID == 0 || *name == "" {
			return errors.New("usage: ticket sdlc add-stage -id <sdlc_id> -name <stage> [-d <desc>] [-order <n>]")
		}
		stage, err := svc.AddSdlcStage(*wfID, libticket.SdlcStageRequest{
			StageName:   *name,
			Description: *desc,
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
	case "stage-update":
		fs := flag.NewFlagSet("sdlc stage-update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		stageID := fs.Int64("stage-id", 0, "sdlc stage id")
		name := fs.String("name", "", "stage name")
		desc := fs.String("d", "", "stage description")
		ac := fs.String("ac", "", "acceptance criteria")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *stageID == 0 || *name == "" {
			return errors.New("usage: ticket sdlc stage-update -stage-id <id> -name <name> [-d <desc>] [-ac <criteria>]")
		}
		stage, err := svc.UpdateSdlcStage(*stageID, libticket.SdlcStageRequest{
			StageName:          *name,
			Description:        *desc,
			AcceptanceCriteria: *ac,
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
		fs := flag.NewFlagSet("sdlc stage-list", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sdlcID := fs.Int64("id", 0, "sdlc id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *sdlcID == 0 {
			return errors.New("usage: ticket sdlc stage-list -id <sdlc_id>")
		}
		stages, err := svc.ListSdlcStages(*sdlcID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(stages)
		}
		printStageTable(stages)
		return nil
	case "stage-get":
		fs := flag.NewFlagSet("sdlc stage-get", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		stageID := fs.Int64("stage-id", 0, "sdlc stage id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *stageID == 0 {
			return errors.New("usage: ticket sdlc stage-get -stage-id <id>")
		}
		stage, err := svc.GetSdlcStage(*stageID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(stage)
		}
		printStageDetail(stage)
		return nil
	case "remove-stage", "stage-rm":
		fs := flag.NewFlagSet("sdlc remove-stage", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		stageID := fs.Int64("stage-id", 0, "sdlc stage id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *stageID == 0 {
			return errors.New("usage: ticket sdlc remove-stage -stage-id <id>")
		}
		if err := svc.RemoveSdlcStage(*stageID); err != nil {
			return err
		}
		fmt.Printf("removed stage %d\n", *stageID)
		return nil
	case "reorder-stages", "stage-order":
		fs := flag.NewFlagSet("sdlc reorder-stages", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		wfID := fs.Int64("id", 0, "sdlc id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *wfID == 0 || fs.NArg() < 1 {
			return errors.New("usage: ticket sdlc reorder-stages -id <sdlc_id> <stage_id,stage_id,...>")
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
		if err := svc.ReorderSdlcStages(*wfID, ids); err != nil {
			return err
		}
		fmt.Println("stages reordered")
		return nil
	case "export":
		fs := flag.NewFlagSet("sdlc export", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "sdlc id")
		outFile := fs.String("o", "", "output file (default stdout)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: ticket sdlc export -id <id> [-o file]")
		}
		export, err := svc.ExportSdlc(*id)
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
		fs := flag.NewFlagSet("sdlc import", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		inFile := fs.String("file", "", "input file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *inFile == "" {
			return errors.New("usage: ticket sdlc import -file <file>")
		}
		data, err := os.ReadFile(*inFile)
		if err != nil {
			return err
		}
		var export store.SdlcExport
		if err := json.Unmarshal(data, &export); err != nil {
			return err
		}
		wf, err := svc.ImportSdlc(export)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(wf)
		}
		fmt.Printf("imported sdlc: %s (id %d)\n", wf.Name, wf.ID)
		return nil
	case "set":
		fs := flag.NewFlagSet("sdlc set", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		ticketID := fs.String("ticket", "", "ticket id")
		sdlcID := fs.Int64("sdlc", 0, "sdlc id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *ticketID == "" || *sdlcID == 0 {
			return errors.New("usage: ticket sdlc set -ticket <ticket-id> -sdlc <sdlc-id>")
		}
		ticket, err := svc.SetTicketSdlc(*ticketID, *sdlcID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(ticket)
		}
		fmt.Printf("set sdlc %d on ticket %s\n", *sdlcID, ticket.ID)
		return nil
	case "role-list", "role-ls":
		fs := flag.NewFlagSet("sdlc role-list", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sdlcID := fs.Int64("id", 0, "sdlc id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *sdlcID == 0 {
			return errors.New("usage: ticket sdlc role-list -id <sdlc_id>")
		}
		roles, err := svc.ListRoles()
		if err != nil {
			return err
		}
		filtered := make([]store.Role, 0)
		for _, r := range roles {
			if r.SdlcID != nil && *r.SdlcID == *sdlcID {
				filtered = append(filtered, r)
			}
		}
		if outputJSON {
			return printJSON(filtered)
		}
		printRoleTable(filtered)
		return nil
	case "stage-role-add":
		fs := flag.NewFlagSet("sdlc stage-role-add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sdlcID := fs.Int64("sdlc_id", 0, "sdlc id")
		stageID := fs.Int64("stage_id", 0, "stage id")
		roleID := fs.Int64("role_id", 0, "role id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *sdlcID == 0 || *stageID == 0 || *roleID == 0 {
			return errors.New("usage: ticket sdlc stage-role-add -sdlc_id <id> -stage_id <id> -role_id <id>")
		}
		if err := svc.AddSdlcStageRole(*sdlcID, *stageID, *roleID); err != nil {
			return err
		}
		fmt.Printf("assigned role #%d to stage #%d\n", *roleID, *stageID)
		return nil
	case "stage-role-rm":
		fs := flag.NewFlagSet("sdlc stage-role-rm", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sdlcID := fs.Int64("sdlc_id", 0, "sdlc id")
		stageID := fs.Int64("stage_id", 0, "stage id")
		roleID := fs.Int64("role_id", 0, "role id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *sdlcID == 0 || *stageID == 0 || *roleID == 0 {
			return errors.New("usage: ticket sdlc stage-role-rm -sdlc_id <id> -stage_id <id> -role_id <id>")
		}
		if err := svc.RemoveSdlcStageRole(*sdlcID, *stageID, *roleID); err != nil {
			return err
		}
		fmt.Printf("removed role #%d from stage #%d\n", *roleID, *stageID)
		return nil
	case "stage-role-order":
		fs := flag.NewFlagSet("sdlc stage-role-order", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sdlcID := fs.Int64("sdlc_id", 0, "sdlc id")
		stageID := fs.Int64("stage_id", 0, "stage id")
		roles := fs.String("roles", "", "comma-separated role ids")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *sdlcID == 0 || *stageID == 0 || *roles == "" {
			return errors.New("usage: ticket sdlc stage-role-order -sdlc_id <id> -stage_id <id> -roles <id,id,...>")
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
		if err := svc.ReorderSdlcStageRoles(*sdlcID, *stageID, roleIDs); err != nil {
			return err
		}
		fmt.Printf("reordered roles in stage #%d\n", *stageID)
		return nil
	case "unset":
		fs := flag.NewFlagSet("sdlc unset", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		ticketID := fs.String("ticket", "", "ticket id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *ticketID == "" {
			return errors.New("usage: ticket sdlc unset -ticket <ticket-id>")
		}
		ticket, err := svc.UnsetTicketSdlc(*ticketID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(ticket)
		}
		fmt.Printf("cleared sdlc on ticket %s (now inherits from parent/project)\n", ticket.ID)
		return nil
	default:
		return fmt.Errorf("unknown sdlc command %q; see: ticket sdlc help", args[0])
	}
}

func printSdlcTable(sdlcs []store.Sdlc) {
	rows := make([]string, 0, len(sdlcs))
	for _, wf := range sdlcs {
		rows = append(rows, fmt.Sprintf("%d\t%s\t%s", wf.ID, wf.Name, wf.Description))
	}
	printBoxTable("ID\tNAME\tDESCRIPTION", rows)
}

func printStageTable(stages []store.SdlcStage) {
	rows := make([]string, 0, len(stages))
	for _, s := range stages {
		var roleNames []string
		for _, r := range s.Roles {
			roleNames = append(roleNames, r.Title)
		}
		rows = append(rows, fmt.Sprintf("%d\t%d\t%s\t%s\t%s", s.SortOrder, s.ID, s.StageName, strings.Join(roleNames, ", "), s.Description))
	}
	printBoxTable("ORDER\tID\tSTAGE\tROLES\tDESCRIPTION", rows)
}

func printStageDetail(s store.SdlcStage) {
	fmt.Printf("ID                  : %d\n", s.ID)
	fmt.Printf("SDLC ID             : %d\n", s.SdlcID)
	fmt.Printf("Stage Name          : %s\n", s.StageName)
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

func printSdlcDetail(wf store.SdlcWithStages) {
	fmt.Printf("ID          : %d\n", wf.ID)
	fmt.Printf("Name        : %s\n", wf.Name)
	fmt.Printf("Description : %s\n", wf.Description)
	fmt.Printf("Stages      :\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  ORDER\tID\tSTAGE\tROLES\tDESCRIPTION")
	for _, s := range wf.Stages {
		var roleNames []string
		for _, r := range s.Roles {
			roleNames = append(roleNames, r.Title)
		}
		fmt.Fprintf(w, "  %d\t%d\t%s\t%s\t%s\n", s.SortOrder, s.ID, s.StageName, strings.Join(roleNames, ", "), s.Description)
	}
	_ = w.Flush()
}
