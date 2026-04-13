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
		sdlcs, err := svc.ListSdlcs(context.Background())
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
			return errors.New("usage: tk sdlc create -name <name> [-d <description>]")
		}
		wf, err := svc.CreateSdlc(context.Background(), libticket.SdlcRequest{Name: *name, Description: *desc})
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
			return errors.New("usage: tk sdlc get -id <id>")
		}
		wf, err := svc.GetSdlc(context.Background(), *id)
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
			return errors.New("usage: tk sdlc delete -id <id>")
		}
		if err := svc.DeleteSdlc(context.Background(), *id); err != nil {
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
		wow := fs.String("wow", "", "ways of working")
		dor := fs.String("dor", "", "definition of ready")
		dod := fs.String("dod", "", "definition of done")
		order := fs.Int("order", 0, "sort order")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *wfID == 0 || *name == "" {
			return errors.New("usage: tk sdlc add-stage -id <sdlc_id> -name <stage> [-d <desc>] [-wow <wow>] [-dor <ready>] [-dod <done>] [-order <n>]")
		}
		stageWoW := strings.TrimSpace(*wow)
		if stageWoW == "" {
			stageWoW = *desc
		}
		stage, err := svc.AddSdlcStage(context.Background(), *wfID, libticket.SdlcStageRequest{
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
		fs := flag.NewFlagSet("sdlc stage-update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		stageID := fs.Int64("stage-id", 0, "sdlc stage id")
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
			return errors.New("usage: tk sdlc stage-update -stage-id <id> -name <name> [-d <desc>] [-ac <criteria>] [-wow <wow>] [-dor <ready>] [-dod <done>]")
		}
		stageWoW := strings.TrimSpace(*wow)
		if stageWoW == "" {
			stageWoW = *desc
		}
		stageDoR := strings.TrimSpace(*dor)
		if stageDoR == "" {
			stageDoR = *ac
		}
		stage, err := svc.UpdateSdlcStage(context.Background(), *stageID, libticket.SdlcStageRequest{
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
		fs := flag.NewFlagSet("sdlc stage-list", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sdlcID := fs.Int64("id", 0, "sdlc id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *sdlcID == 0 {
			return errors.New("usage: tk sdlc stage-list -id <sdlc_id>")
		}
		stages, err := svc.ListSdlcStages(context.Background(), *sdlcID)
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
			return errors.New("usage: tk sdlc stage-get -stage-id <id>")
		}
		stage, err := svc.GetSdlcStage(context.Background(), *stageID)
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
			return errors.New("usage: tk sdlc remove-stage -stage-id <id>")
		}
		if err := svc.RemoveSdlcStage(context.Background(), *stageID); err != nil {
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
			return errors.New("usage: tk sdlc reorder-stages -id <sdlc_id> <stage_id,stage_id,...>")
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
		if err := svc.ReorderSdlcStages(context.Background(), *wfID, ids); err != nil {
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
			return errors.New("usage: tk sdlc export -id <id> [-o file]")
		}
		export, err := svc.ExportSdlc(context.Background(), *id)
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
			return errors.New("usage: tk sdlc import -file <file>")
		}
		data, err := os.ReadFile(*inFile)
		if err != nil {
			return err
		}
		var export store.SdlcExport
		if err := json.Unmarshal(data, &export); err != nil {
			return err
		}
		wf, err := svc.ImportSdlc(context.Background(), export)
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
			return errors.New("usage: tk sdlc set -ticket <ticket-id> -sdlc <sdlc-id>")
		}
		ticket, err := svc.SetTicketSdlc(context.Background(), *ticketID, *sdlcID)
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
			return errors.New("usage: tk sdlc role-list -id <sdlc_id>")
		}
		roles, err := svc.ListRoles(context.Background())
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
	case "role-add":
		fs := flag.NewFlagSet("sdlc role-add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sdlcID := fs.Int64("sdlc_id", 0, "sdlc id")
		title := fs.String("title", "", "role title")
		description := fs.String("description", "", "role description")
		ac := fs.String("ac", "", "role acceptance criteria")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *sdlcID == 0 || strings.TrimSpace(*title) == "" || fs.NArg() != 0 {
			return errors.New("usage: tk sdlc role-add -sdlc_id <id> -title <title> [-description <text>] [-ac <text>]")
		}
		role, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
			SdlcID:             sdlcID,
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
		fmt.Printf("created sdlc role #%d %s\n", role.ID, role.Title)
		return nil
	case "role-get":
		fs := flag.NewFlagSet("sdlc role-get", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sdlcID := fs.Int64("sdlc_id", 0, "sdlc id")
		roleID := fs.Int64("role_id", 0, "role id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *sdlcID == 0 || *roleID == 0 || fs.NArg() != 0 {
			return errors.New("usage: tk sdlc role-get -sdlc_id <id> -role_id <id>")
		}
		role, err := sdlcScopedRole(svc, *sdlcID, *roleID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(role)
		}
		printRoleDetail(role)
		return nil
	case "role-update":
		fs := flag.NewFlagSet("sdlc role-update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sdlcID := fs.Int64("sdlc_id", 0, "sdlc id")
		roleID := fs.Int64("role_id", 0, "role id")
		title := fs.String("title", "", "role title")
		description := fs.String("description", "", "role description")
		ac := fs.String("ac", "", "role acceptance criteria")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *sdlcID == 0 || *roleID == 0 || strings.TrimSpace(*title) == "" || fs.NArg() != 0 {
			return errors.New("usage: tk sdlc role-update -sdlc_id <id> -role_id <id> -title <title> [-description <text>] [-ac <text>]")
		}
		if _, err := sdlcScopedRole(svc, *sdlcID, *roleID); err != nil {
			return err
		}
		role, err := svc.UpdateRole(context.Background(), *roleID, libticket.RoleRequest{
			SdlcID:             sdlcID,
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
		fmt.Printf("updated sdlc role #%d %s\n", role.ID, role.Title)
		return nil
	case "role-rm":
		fs := flag.NewFlagSet("sdlc role-rm", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sdlcID := fs.Int64("sdlc_id", 0, "sdlc id")
		roleID := fs.Int64("role_id", 0, "role id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *sdlcID == 0 || *roleID == 0 || fs.NArg() != 0 {
			return errors.New("usage: tk sdlc role-rm -sdlc_id <id> -role_id <id>")
		}
		if _, err := sdlcScopedRole(svc, *sdlcID, *roleID); err != nil {
			return err
		}
		if err := svc.DeleteRole(context.Background(), *roleID); err != nil {
			return err
		}
		fmt.Printf("deleted sdlc role #%d\n", *roleID)
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
			return errors.New("usage: tk sdlc stage-role-add -sdlc_id <id> -stage_id <id> -role_id <id>")
		}
		if err := svc.AddSdlcStageRole(context.Background(), *sdlcID, *stageID, *roleID); err != nil {
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
			return errors.New("usage: tk sdlc stage-role-rm -sdlc_id <id> -stage_id <id> -role_id <id>")
		}
		if err := svc.RemoveSdlcStageRole(context.Background(), *sdlcID, *stageID, *roleID); err != nil {
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
			return errors.New("usage: tk sdlc stage-role-order -sdlc_id <id> -stage_id <id> -roles <id,id,...>")
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
		if err := svc.ReorderSdlcStageRoles(context.Background(), *sdlcID, *stageID, roleIDs); err != nil {
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
			return errors.New("usage: tk sdlc unset -ticket <ticket-id>")
		}
		ticket, err := svc.UnsetTicketSdlc(context.Background(), *ticketID)
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
		rows = append(rows, fmt.Sprintf("%d\t%d\t%s\t%s\t%s\t%s\t%s", s.SortOrder, s.ID, s.StageName, strings.Join(roleNames, ", "), s.Description, stageDoRValue(s), s.DefinitionOfDone))
	}
	printBoxTable("ORDER\tID\tSTAGE\tROLES\tWOW\tDOR\tDOD", rows)
}

func printStageDetail(s store.SdlcStage) {
	fmt.Printf("ID                  : %d\n", s.ID)
	fmt.Printf("SDLC ID             : %d\n", s.SdlcID)
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

func printSdlcDetail(wf store.SdlcWithStages) {
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
		fmt.Fprintf(os.Stderr, "warning: could not flush SDLC stage table: %v\n", err)
	}
}

func stageDoRValue(s store.SdlcStage) string {
	if strings.TrimSpace(s.DefinitionOfReady) != "" {
		return s.DefinitionOfReady
	}
	return s.AcceptanceCriteria
}

func sdlcScopedRole(svc libticket.Service, sdlcID, roleID int64) (store.Role, error) {
	roles, err := svc.ListRoles(context.Background())
	if err != nil {
		return store.Role{}, err
	}
	for _, role := range roles {
		if role.ID != roleID {
			continue
		}
		if role.SdlcID != nil && *role.SdlcID == sdlcID {
			return role, nil
		}
		break
	}
	return store.Role{}, fmt.Errorf("role %d not found in sdlc %d", roleID, sdlcID)
}

func printRoleDetail(role store.Role) {
	fmt.Printf("ID:                  %d\n", role.ID)
	if role.SdlcID != nil {
		fmt.Printf("SDLC ID:             %d\n", *role.SdlcID)
	}
	fmt.Printf("Title:               %s\n", role.Title)
	fmt.Printf("Description:         %s\n", role.Description)
	fmt.Printf("Acceptance Criteria: %s\n", role.AcceptanceCriteria)
	fmt.Printf("Created:             %s\n", role.CreatedAt)
	fmt.Printf("Updated:             %s\n", role.UpdatedAt)
}
