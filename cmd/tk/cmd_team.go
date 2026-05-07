package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func runTeam(args []string) error {
	if len(args) == 0 {
		fmt.Println(teamUsage)
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
		fmt.Println(teamUsage)
		return nil
	case "list", "ls":
		teams, err := svc.ListTeams(context.Background())
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(teams)
		}
		printTeamTable(teams)
		return nil
	case "create", "add", "new":
		fs := flag.NewFlagSet("team create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "force team id")
		printID := fs.Bool("printid", false, "print only the created team id")
		name := fs.String("name", "", "team name")
		parentID := fs.Int64("parent_id", 0, "optional parent team id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" || fs.NArg() != 0 {
			return errors.New("usage: tk team create -name <name> [-id <id>] [-parent_id <id>]")
		}
		var parent *int64
		if *parentID > 0 {
			parent = parentID
		}
		team, err := svc.CreateTeam(context.Background(), libticket.TeamRequest{
			ID:           optionalInt64Flag(*id),
			Name:         strings.TrimSpace(*name),
			ParentTeamID: parent,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(team)
		}
		if printCreatedID(team.ID, *printID) {
			return nil
		}
		fmt.Printf("created team #%d %s\n", team.ID, team.Name)
		return nil
	case "update":
		fs := flag.NewFlagSet("team update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "team id")
		name := fs.String("name", "", "team name")
		parentID := fs.Int64("parent_id", -1, "parent team id (0 clears)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 || fs.NArg() != 0 {
			return errors.New("usage: tk team update -id <id> [-name <name>] [-parent_id <id|0>]")
		}
		var parent *int64
		if *parentID > 0 {
			parent = parentID
		}
		team, err := svc.UpdateTeam(context.Background(), *id, libticket.TeamRequest{
			Name:         strings.TrimSpace(*name),
			ParentTeamID: parent,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(team)
		}
		fmt.Printf("updated team #%d %s\n", team.ID, team.Name)
		return nil
	case "delete", "rm":
		fs := flag.NewFlagSet("team delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "team id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 || fs.NArg() != 0 {
			return errors.New("usage: tk team delete -id <id>")
		}
		if err := svc.DeleteTeam(context.Background(), *id); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"status": "deleted", "team_id": *id})
		}
		fmt.Printf("deleted team #%d\n", *id)
		return nil
	case "add-user":
		fs := flag.NewFlagSet("team add-user", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		teamID := fs.Int64("team_id", 0, "team id")
		userID := fs.String("user_id", "", "user id")
		role := fs.String("role", "", "team role [member,owner]")
		jobTitle := fs.String("job_title", "", "job title")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *teamID == 0 || *userID == "" || strings.TrimSpace(*role) == "" || fs.NArg() != 0 {
			return errors.New("usage: tk team add-user -team_id <id> -user_id <id> -role <member|owner> [-job_title <title>]")
		}
		member, err := svc.AddTeamMember(context.Background(), *teamID, libticket.TeamMemberRequest{
			UserID:   *userID,
			Role:     strings.TrimSpace(*role),
			JobTitle: strings.TrimSpace(*jobTitle),
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(member)
		}
		fmt.Printf("added team user: team_id=%d user_id=%s role=%s job_title=%s\n", member.TeamID, member.UserID, member.Role, member.JobTitle)
		return nil
	case "remove-user":
		fs := flag.NewFlagSet("team remove-user", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		teamID := fs.Int64("team_id", 0, "team id")
		userID := fs.String("user_id", "", "user id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *teamID == 0 || *userID == "" || fs.NArg() != 0 {
			return errors.New("usage: tk team remove-user -team_id <id> -user_id <id>")
		}
		if err := svc.RemoveTeamMember(context.Background(), *teamID, *userID); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"status": "deleted", "team_id": *teamID, "user_id": *userID})
		}
		fmt.Printf("removed team user: team_id=%d user_id=%s\n", *teamID, *userID)
		return nil
	case "users":
		fs := flag.NewFlagSet("team users", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		teamID := fs.Int64("team_id", 0, "team id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *teamID == 0 || fs.NArg() != 0 {
			return errors.New("usage: tk team users -team_id <id>")
		}
		members, err := svc.ListTeamMembers(context.Background(), *teamID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(members)
		}
		printTeamMemberTable(members)
		return nil
	case "add-agent":
		fs := flag.NewFlagSet("team add-agent", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		teamID := fs.Int64("team_id", 0, "team id")
		agentID := fs.String("agent_id", "", "agent id (UUID)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *teamID == 0 || *agentID == "" || fs.NArg() != 0 {
			return errors.New("usage: tk team add-agent -team_id <id> -agent_id <uuid>")
		}
		item, err := svc.AddTeamAgent(context.Background(), *teamID, *agentID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(item)
		}
		fmt.Printf("added team agent: team_id=%d agent_id=%s\n", item.TeamID, item.AgentID)
		return nil
	case "remove-agent":
		fs := flag.NewFlagSet("team remove-agent", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		teamID := fs.Int64("team_id", 0, "team id")
		agentID := fs.String("agent_id", "", "agent id (UUID)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *teamID == 0 || *agentID == "" || fs.NArg() != 0 {
			return errors.New("usage: tk team remove-agent -team_id <id> -agent_id <uuid>")
		}
		if err := svc.RemoveTeamAgent(context.Background(), *teamID, *agentID); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"status": "deleted", "team_id": *teamID, "agent_id": *agentID})
		}
		fmt.Printf("removed team agent: team_id=%d agent_id=%s\n", *teamID, *agentID)
		return nil
	case "agents":
		fs := flag.NewFlagSet("team agents", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		teamID := fs.Int64("team_id", 0, "team id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *teamID == 0 || fs.NArg() != 0 {
			return errors.New("usage: tk team agents -team_id <id>")
		}
		items, err := svc.ListTeamAgents(context.Background(), *teamID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(items)
		}
		printTeamAgentTable(items)
		return nil
	default:
		return fmt.Errorf("unknown team command %q; see: ticket team help", args[0])
	}
}

func printTeamTable(teams []store.Team) {
	if len(teams) == 0 {
		fmt.Println("no teams")
		return
	}
	rows := make([]string, 0, len(teams))
	for _, team := range teams {
		parent := "-"
		if team.ParentTeamID != nil {
			parent = fmt.Sprintf("%d", *team.ParentTeamID)
		}
		rows = append(rows, fmt.Sprintf("%d\t%s\t%s", team.ID, team.Name, parent))
	}
	printBoxTable("ID\tNAME\tPARENT_TEAM_ID", rows)
}

func printTeamMemberTable(members []store.TeamMember) {
	if len(members) == 0 {
		fmt.Println("no team members")
		return
	}
	rows := make([]string, 0, len(members))
	for _, m := range members {
		rows = append(rows, fmt.Sprintf("%d\t%s\t%s\t%s\t%s", m.TeamID, m.UserID, m.Username, m.Role, m.JobTitle))
	}
	printBoxTable("TEAM_ID\tUSER_ID\tUSERNAME\tROLE\tJOB_TITLE", rows)
}

func printTeamAgentTable(items []store.TeamAgent) {
	if len(items) == 0 {
		fmt.Println("no team agents")
		return
	}
	rows := make([]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, fmt.Sprintf("%d\t%s\t%s\t%t\t%s", item.TeamID, item.AgentID, item.AgentUUID, item.Enabled, item.Status))
	}
	printBoxTable("TEAM_ID\tAGENT_ID\tUUID\tENABLED\tSTATUS", rows)
}

func runRole(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		fmt.Println(roleUsage)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Println(roleUsage)
		return nil
	case "list", "ls":
		roles, err := svc.ListRoles(context.Background())
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(roles)
		}
		printRoleTable(roles)
		return nil
	case "get", "show":
		fs := flag.NewFlagSet("role get", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "role id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			if fs.NArg() > 0 {
				if v, err := strconv.ParseInt(fs.Arg(0), 10, 64); err == nil {
					*id = v
				}
			}
		}
		if *id == 0 {
			return errors.New("usage: tk role get -id <id>")
		}
		roles, err := svc.ListRoles(context.Background())
		if err != nil {
			return err
		}
		for _, role := range roles {
			if role.ID != *id {
				continue
			}
			if outputJSON {
				return printJSON(role)
			}
			fmt.Printf("ID:         %d\n", role.ID)
			fmt.Printf("Title:      %s\n", role.Title)
			fmt.Printf("Description: %s\n", role.Description)
			fmt.Printf("AcceptanceCriteria:      %s\n", role.AcceptanceCriteria)
			printGuidanceMap("dor_map", role.DORMap)
			printGuidanceMap("dod_map", role.DODMap)
			printGuidanceMap("ac_map", role.ACMap)
			fmt.Printf("Created:    %s\n", role.CreatedAt)
			fmt.Printf("Updated:    %s\n", role.UpdatedAt)
			return nil
		}
		return fmt.Errorf("role %d not found", *id)
	case "create", "add", "new":
		fs := flag.NewFlagSet("role create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "force role id")
		printID := fs.Bool("printid", false, "print only the created role id")
		title := fs.String("title", "", "role title")
		description := fs.String("description", "", "role description")
		ac := fs.String("ac", "", "role acceptance criteria")
		dor := fs.String("dor", "", "role default definition of ready")
		dod := fs.String("dod", "", "role default definition of done")
		dorMapRaw := fs.String("dor-map", "", "stage-specific DoR entries (stage=value,...)")
		dodMapRaw := fs.String("dod-map", "", "stage-specific DoD entries (stage=value,...)")
		acMapRaw := fs.String("ac-map", "", "stage-specific acceptance criteria entries (stage=value,...)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*title) == "" || fs.NArg() != 0 {
			return errors.New("usage: tk role create -title <title> [-id <id>] [-description <text>] [-dor <text>] [-dod <text>] [-ac <text>] [-dor-map stage=value,...] [-dod-map stage=value,...] [-ac-map stage=value,...]")
		}
		dorMap, err := mergeGuidanceMap(nil, *dor, *dorMapRaw, containsFlag(args[1:], "-dor"), containsFlag(args[1:], "-dor-map"))
		if err != nil {
			return err
		}
		dodMap, err := mergeGuidanceMap(nil, *dod, *dodMapRaw, containsFlag(args[1:], "-dod"), containsFlag(args[1:], "-dod-map"))
		if err != nil {
			return err
		}
		acMap, err := mergeGuidanceMap(nil, *ac, *acMapRaw, containsFlag(args[1:], "-ac"), containsFlag(args[1:], "-ac-map"))
		if err != nil {
			return err
		}
		role, err := svc.CreateRole(context.Background(), libticket.RoleRequest{
			ID:                 optionalInt64Flag(*id),
			Title:              strings.TrimSpace(*title),
			Description:        strings.TrimSpace(*description),
			AcceptanceCriteria: strings.TrimSpace(*ac),
			DORMap:             dorMap,
			DODMap:             dodMap,
			ACMap:              acMap,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(role)
		}
		if printCreatedID(role.ID, *printID) {
			return nil
		}
		fmt.Printf("created role #%d %s\n", role.ID, role.Title)
		return nil
	case "update":
		fs := flag.NewFlagSet("role update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "role id")
		title := fs.String("title", "", "role title")
		description := fs.String("description", "", "role description")
		ac := fs.String("ac", "", "role acceptance criteria")
		dor := fs.String("dor", "", "role default definition of ready")
		dod := fs.String("dod", "", "role default definition of done")
		dorMapRaw := fs.String("dor-map", "", "stage-specific DoR entries (stage=value,...)")
		dodMapRaw := fs.String("dod-map", "", "stage-specific DoD entries (stage=value,...)")
		acMapRaw := fs.String("ac-map", "", "stage-specific acceptance criteria entries (stage=value,...)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 || fs.NArg() != 0 {
			return errors.New("usage: tk role update -id <id> [-title <title>] [-description <text>] [-dor <text>] [-dod <text>] [-ac <text>] [-dor-map stage=value,...] [-dod-map stage=value,...] [-ac-map stage=value,...]")
		}
		roles, err := svc.ListRoles(context.Background())
		if err != nil {
			return err
		}
		var current *store.Role
		for i := range roles {
			if roles[i].ID == *id {
				current = &roles[i]
				break
			}
		}
		if current == nil {
			return fmt.Errorf("role %d not found", *id)
		}
		nextTitle := current.Title
		if containsFlag(args[1:], "-title") {
			nextTitle = strings.TrimSpace(*title)
		}
		nextDescription := current.Description
		if containsFlag(args[1:], "-description") {
			nextDescription = strings.TrimSpace(*description)
		}
		nextAC := current.AcceptanceCriteria
		if containsFlag(args[1:], "-ac") {
			nextAC = strings.TrimSpace(*ac)
		}
		dorMap, err := mergeGuidanceMap(current.DORMap, *dor, *dorMapRaw, containsFlag(args[1:], "-dor"), containsFlag(args[1:], "-dor-map"))
		if err != nil {
			return err
		}
		dodMap, err := mergeGuidanceMap(current.DODMap, *dod, *dodMapRaw, containsFlag(args[1:], "-dod"), containsFlag(args[1:], "-dod-map"))
		if err != nil {
			return err
		}
		acMap, err := mergeGuidanceMap(current.ACMap, *ac, *acMapRaw, containsFlag(args[1:], "-ac"), containsFlag(args[1:], "-ac-map"))
		if err != nil {
			return err
		}
		role, err := svc.UpdateRole(context.Background(), *id, libticket.RoleRequest{
			Title:              nextTitle,
			Description:        nextDescription,
			AcceptanceCriteria: nextAC,
			DORMap:             dorMap,
			DODMap:             dodMap,
			ACMap:              acMap,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(role)
		}
		fmt.Printf("updated role #%d %s\n", role.ID, role.Title)
		return nil
	case "delete", "rm":
		fs := flag.NewFlagSet("role delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "role id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 || fs.NArg() != 0 {
			return errors.New("usage: tk role delete -id <id>")
		}
		if err := svc.DeleteRole(context.Background(), *id); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"status": "deleted", "role_id": *id})
		}
		fmt.Printf("deleted role #%d\n", *id)
		return nil
	default:
		return fmt.Errorf("unknown role command %q; see: ticket role help", args[0])
	}
}
