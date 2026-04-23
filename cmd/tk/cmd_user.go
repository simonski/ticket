package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

func runRegister(args []string) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	usernameFlag := fs.String("username", "", "username")
	passwordFlag := fs.String("password", "", "password")
	if err := fs.Parse(args); err != nil {
		return err
	}

	username, password, err := resolveCredentials(*usernameFlag, *passwordFlag, true)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	user, err := svc.Register(context.Background(), username, password)
	if err != nil {
		return err
	}
	cfg.Username = user.Username
	if err := config.Save(cfg); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(user)
	}
	fmt.Printf("registered user %s\n", user.Username)
	return nil
}

func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	usernameFlag := fs.String("username", "", "username")
	passwordFlag := fs.String("password", "", "password")
	if err := fs.Parse(args); err != nil {
		return err
	}

	username, password, err := resolveCredentials(*usernameFlag, *passwordFlag, true)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	if cfg.Token != "" {
		status, err := svc.Status(context.Background())
		if err == nil && status.Authenticated && status.User != nil {
			cfg.Username = status.User.Username
			if err := config.Save(cfg); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(status)
			}
			fmt.Printf("logged in as %s\n", status.User.Username)
			return nil
		}
	}

	username = resolveLoginUsername(cfg.Username, *usernameFlag)
	password = resolveLoginPassword(*passwordFlag)

	if username != "" && password != "" {
		user, token, err := svc.Login(context.Background(), username, password)
		if err == nil {
			return finishLogin(cfg, user, token)
		}
		if err.Error() != "invalid credentials" {
			return err
		}
		fmt.Println("invalid credentials")
	}

	username, password, err = promptForCredentials(loginPromptInput, loginPromptOutput, username, password)
	if err != nil {
		return err
	}
	user, token, err := svc.Login(context.Background(), username, password)
	if err != nil {
		return err
	}
	return finishLogin(cfg, user, token)
}

func finishLogin(cfg config.Config, user store.User, token string) error {
	cfg.Username = user.Username
	if err := config.Save(cfg); err != nil {
		return err
	}
	if err := config.SaveRemoteCredentials(cfg.Location, user.Username, token); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]any{"token": token, "user": user})
	}
	fmt.Printf("logged in as %s\n", user.Username)
	return nil
}

func runLogout(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: tk logout")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	if err := svc.Logout(context.Background()); err != nil {
		if clearErr := config.ClearRemoteCredentials(cfg.Location); clearErr != nil {
			return clearErr
		}
		cfg.Token = ""
		return err
	}
	if err := config.ClearRemoteCredentials(cfg.Location); err != nil {
		return err
	}
	cfg.Token = ""
	if err := config.Save(cfg); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]string{"status": "logged_out"})
	}
	return nil
}

func runStatus(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: tk status")
	}
	return runStatusWithSummaryStyle(true)
}

func runStatusWithSummaryStyle(statusUnicode bool) error {
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	switch resolved.Mode {
	case config.ModeRemote:
		return runRemoteStatusWithSummaryStyle(cfg, statusUnicode)
	case config.ModeLocal:
		return runLocalStatusWithSummaryStyle(statusUnicode)
	default:
		return fmt.Errorf("unsupported mode %q", resolved.Mode)
	}
}

func runCount(args []string) error {
	fs := flag.NewFlagSet("count", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	projectID := fs.Int64("project_id", 0, "limit counts to a project id")
	taskType := fs.String("type", "", "filter ticket count by ticket type")
	stage := fs.String("stage", "", "filter ticket count by stage")
	state := fs.String("state", "", "filter ticket count by state")
	status := fs.String("status", "", "filter ticket count by rendered status")
	assignee := fs.String("user", "", "filter ticket count by assignee")
	fs.StringVar(assignee, "u", "", "filter ticket count by assignee")
	search := fs.String("search", "", "filter ticket count by search text")
	includeAll := fs.Bool("a", false, "include closed and archived tickets")
	includeDeleted := fs.Bool("d", false, "include archived tickets")
	expectEquals := fs.String("expect_equals", "", "expect the resulting count to equal this number")
	expectNotEquals := fs.String("expect_notequals", "", "expect the resulting count to not equal this number")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: tk count [-project_id <id>] [-type <type>] [-stage <stage>] [-state <state>] [-status <status>] [-user <user>] [-search <text>] [-a] [-d] [-expect_equals <n>] [-expect_notequals <n>]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	var projectFilter *int64
	if *projectID != 0 {
		projectFilter = projectID
		if _, err := svc.GetProject(context.Background(), fmt.Sprintf("%d", *projectID)); err != nil {
			return err
		}
	}
	hasTicketFilters := strings.TrimSpace(*taskType) != "" ||
		strings.TrimSpace(*stage) != "" ||
		strings.TrimSpace(*state) != "" ||
		strings.TrimSpace(*status) != "" ||
		strings.TrimSpace(*assignee) != "" ||
		strings.TrimSpace(*search) != ""
	hasExpectEquals := strings.TrimSpace(*expectEquals) != ""
	hasExpectNotEquals := strings.TrimSpace(*expectNotEquals) != ""
	if hasExpectEquals && hasExpectNotEquals {
		return errors.New("count expects only one of -expect_equals or -expect_notequals")
	}
	if *includeDeleted {
		*includeAll = true
	}
	if hasTicketFilters || hasExpectEquals || hasExpectNotEquals {
		var project store.Project
		if projectFilter != nil {
			project, err = svc.GetProject(context.Background(), fmt.Sprintf("%d", *projectFilter))
			if err != nil {
				return err
			}
		} else {
			_, resolvedSvc, currentProject, err := resolveCurrentProjectClient()
			if err != nil {
				return err
			}
			svc = resolvedSvc
			project = currentProject
		}
		resolvedStage, resolvedState, err := resolveLifecycleInput(*status, *stage, *state)
		if err != nil {
			return err
		}
		tickets, err := svc.ListTicketsFiltered(context.Background(), project.ID, *taskType, resolvedStage, resolvedState, "", *search, *assignee, 0, *includeAll)
		if err != nil {
			return err
		}
		if !*includeAll {
			open := tickets[:0]
			for _, ticket := range tickets {
				if ticketIsOpenForList(ticket) {
					open = append(open, ticket)
				}
			}
			tickets = open
		} else if !*includeDeleted {
			nonArchived := tickets[:0]
			for _, ticket := range tickets {
				if !ticket.Archived {
					nonArchived = append(nonArchived, ticket)
				}
			}
			tickets = nonArchived
		}
		count := len(tickets)
		if hasExpectEquals {
			expected, err := parseExpectedCount("expect_equals", *expectEquals)
			if err != nil {
				return err
			}
			if count != expected {
				return fmt.Errorf("expected count to equal %d, got %d", expected, count)
			}
		}
		if hasExpectNotEquals {
			expected, err := parseExpectedCount("expect_notequals", *expectNotEquals)
			if err != nil {
				return err
			}
			if count == expected {
				return fmt.Errorf("expected count to not equal %d, got %d", expected, count)
			}
		}
		if outputJSON {
			return printJSON(map[string]any{"count": count})
		}
		fmt.Println(count)
		return nil
	}
	summary, err := svc.Count(context.Background(), projectFilter)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(summary)
	}
	printCountSummary(summary, projectFilter != nil)
	return nil
}

func runWhoami(args []string) error {
	_ = args
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	// User info
	username := cfg.Username
	if username == "" {
		username = "admin"
	}
	users, _ := svc.ListUsers(context.Background())
	var currentUser *store.User
	for _, u := range users {
		if u.Username == username {
			currentUser = &u
			break
		}
	}

	fmt.Println("USER")
	if currentUser != nil {
		fmt.Printf("  username : %s\n", currentUser.Username)
		fmt.Printf("  role     : %s\n", currentUser.Role)
		fmt.Printf("  user_id  : %s\n", currentUser.ID)
	} else {
		fmt.Printf("  username : %s\n", username)
	}

	// Connection info
	fmt.Println()
	fmt.Println("CONNECTION")
	fmt.Printf("  mode     : %s\n", resolved.Mode)
	if resolved.Mode == config.ModeRemote {
		fmt.Printf("  server   : %s\n", resolved.ServerURL)
	} else {
		fmt.Printf("  database : %s\n", resolved.DBPath)
	}

	// Projects with user role
	fmt.Println()
	fmt.Println("PROJECTS")
	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		fmt.Println("  (unable to list projects)")
		return nil
	}
	if len(projects) == 0 {
		fmt.Println("  (none)")
		return nil
	}
	for _, p := range projects {
		marker := "  "
		if p.Prefix == cfg.ProjectID || fmt.Sprintf("%d", p.ID) == cfg.ProjectID {
			marker = "* "
		}
		role := ""
		if currentUser != nil {
			members, _ := svc.ListProjectMembers(context.Background(), p.ID)
			for _, m := range members {
				if m.UserID == currentUser.ID {
					role = m.Role
					break
				}
			}
		}
		if role != "" {
			fmt.Printf("  %s%-6s  %-20s  (%s)\n", marker, p.Prefix, p.Title, role)
		} else {
			fmt.Printf("  %s%-6s  %s\n", marker, p.Prefix, p.Title)
		}
	}

	return nil
}

func runUser(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(userUsage)
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
	case "create", "new":
		fs := flag.NewFlagSet("user create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		usernameFlag := fs.String("username", "", "username")
		passwordFlag := fs.String("password", "", "password")
		printID := fs.Bool("printid", false, "print only the created user id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		username, password, err := resolveCredentials(*usernameFlag, *passwordFlag, true)
		if err != nil {
			return err
		}
		user, err := svc.CreateUser(context.Background(), username, password)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(user)
		}
		if printCreatedID(user.ID, *printID) {
			return nil
		}
		fmt.Printf("created user %s\n", user.Username)
		return nil
	case "rm", "delete", "del":
		fs := flag.NewFlagSet("user "+args[0], flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		username := fs.String("username", "", "username")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *username == "" {
			return errors.New("user rm/delete/del requires -username")
		}
		if err := svc.DeleteUser(context.Background(), *username); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]string{"status": "deleted", "username": *username})
		}
		fmt.Printf("deleted user %s\n", *username)
		return nil
	case "enable", "disable":
		fs := flag.NewFlagSet("user "+args[0], flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		username := fs.String("username", "", "username")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *username == "" {
			return errors.New("user enable/disable requires -username")
		}
		if err := svc.SetUserEnabled(context.Background(), *username, args[0] == "enable"); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]string{"status": args[0] + "d", "username": *username})
		}
		fmt.Printf("%sd user %s\n", args[0], *username)
		return nil
	case "list", "ls":
		users, err := svc.ListUsers(context.Background())
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(users)
		}
		printUserTable(users)
		return nil
	case "reset-password":
		fs := flag.NewFlagSet("user reset-password", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		username := fs.String("username", "", "username")
		newPassword := fs.String("password", "", "new password (generated if omitted)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*username) == "" {
			return errors.New("usage: tk user reset-password -username <name> [-password <new-password>]")
		}
		pw := strings.TrimSpace(*newPassword)
		if pw == "" {
			generated, err := generatePassword(24)
			if err != nil {
				return err
			}
			pw = generated
		}
		user, err := svc.ResetUserPassword(context.Background(), strings.TrimSpace(*username), pw)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"user_id": user.ID, "username": user.Username, "password": pw})
		}
		fmt.Printf("username : %s\n", user.Username)
		fmt.Printf("password : %s\n", pw)
		fmt.Println("all sessions invalidated")
		return nil
	default:
		return fmt.Errorf("unknown user command %q; see: ticket user help", args[0])
	}
}
