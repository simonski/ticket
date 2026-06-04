package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/client"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func runRegister(args []string) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	usernameFlag := fs.String("username", "", "username")
	passwordFlag := fs.String("password", "", "password")
	emailFlag := fs.String("email", "", "email")
	if err := fs.Parse(args); err != nil {
		return err
	}

	username := strings.TrimSpace(*usernameFlag)
	if username == "" {
		return errors.New("username is required")
	}
	email := strings.TrimSpace(*emailFlag)
	if email == "" {
		return errors.New("email is required")
	}
	password := strings.TrimSpace(*passwordFlag)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURLForAuth(cfg)
	if err != nil {
		return err
	}
	api := client.New(config.Config{Location: serverURL})
	response, err := api.RegisterDetailed(context.Background(), client.RegisterRequest{
		Username: username,
		Password: password,
		Email:    email,
	})
	if err != nil {
		var statusErr *client.HTTPStatusError
		if errors.As(err, &statusErr) && statusErr.StatusCode == 403 && strings.TrimSpace(statusErr.APIError) == "registration is disabled" {
			return errors.New("server is not accepting registrations right now")
		}
		return err
	}
	if outputJSON {
		return printJSON(response)
	}
	fmt.Printf("registered user %s\n", response.Username)
	if response.Password != "" {
		fmt.Printf("password: %s\n", response.Password)
	}
	if !response.Approved {
		fmt.Println("registration submitted; wait for approval or check your email for next steps.")
	}
	return nil
}

func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	usernameFlag := fs.String("username", "", "username")
	passwordFlag := fs.String("password", "", "password")
	tokenFlag := fs.String("token", "", "bearer token")
	passkeyFlag := fs.Bool("passkey", false, "use browser-assisted passkey login")
	if err := fs.Parse(args); err != nil {
		return err
	}

	token := strings.TrimSpace(*tokenFlag)
	if token != "" && strings.TrimSpace(*passwordFlag) != "" {
		return errors.New("use either -password or -token, not both")
	}
	if *passkeyFlag && (token != "" || strings.TrimSpace(*passwordFlag) != "") {
		return errors.New("use either -password, -token, or --passkey")
	}

	resolvedUsername := ""
	resolvedPassword := ""
	var err error
	if token == "" && !*passkeyFlag {
		resolvedUsername, resolvedPassword, err = resolveCredentials(*usernameFlag, *passwordFlag, true)
		if err != nil {
			return err
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURLForAuth(cfg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Token) == "" {
		creds, credsErr := config.LoadCredentials()
		if credsErr != nil {
			return credsErr
		}
		if remoteCreds, ok := creds.Remote(serverURL); ok && strings.TrimSpace(remoteCreds.Token) != "" {
			if strings.TrimSpace(cfg.Username) == "" {
				cfg.Username = strings.TrimSpace(remoteCreds.Username)
			}
			cfg.Token = strings.TrimSpace(remoteCreds.Token)
		}
	}
	if *passkeyFlag {
		return runPasskeyLogin(cfg, serverURL, *usernameFlag)
	}
	svc := libticket.NewHTTP(config.Config{Location: serverURL, Token: cfg.Token})

	if token != "" {
		tokenSvc := libticket.NewHTTP(config.Config{Location: serverURL, Token: token})
		status, statusErr := tokenSvc.Status(context.Background())
		if statusErr != nil {
			return statusErr
		}
		if !status.Authenticated || status.User == nil {
			return errors.New("invalid token")
		}
		return finishLogin(cfg, *status.User, token)
	}

	if cfg.Token != "" {
		status, statusErr := svc.Status(context.Background())
		if statusErr == nil && status.Authenticated && status.User != nil {
			cfg.Username = status.User.Username
			if outputJSON {
				return printJSON(status)
			}
			fmt.Printf("logged in as %s\n", status.User.Username)
			return nil
		}
	}

	username := resolveLoginUsername(cfg.Username, *usernameFlag)
	if username == "" {
		username = strings.TrimSpace(resolvedUsername)
	}
	password := resolveLoginPassword(*passwordFlag)
	if password == "" {
		password = resolvedPassword
	}

	if username != "" && password != "" {
		user, sessionToken, loginErr := svc.Login(context.Background(), username, password)
		if loginErr == nil {
			return finishLogin(cfg, user, sessionToken)
		}
		if loginErr.Error() != "invalid credentials" {
			return loginErr
		}
		fmt.Println("invalid credentials")
	}

	username, password, err = promptForCredentials(loginPromptInput, loginPromptOutput, username, password)
	if err != nil {
		return err
	}
	user, sessionToken, err := svc.Login(context.Background(), username, password)
	if err != nil {
		return err
	}
	return finishLogin(cfg, user, sessionToken)
}

func resolveServerURLForAuth(cfg config.Config) (string, error) {
	location := strings.TrimSpace(os.Getenv("TICKET_URL"))
	if location == "" {
		if isTestBinary() {
			location = strings.TrimSpace(cfg.Location)
		} else {
			location = defaultTicketURL
		}
	}
	resolved, err := config.ResolveLocation(location)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resolved.ServerURL) == "" {
		return "", errors.New("ticket login/register require a running server")
	}
	return resolved.ServerURL, nil
}

func finishLogin(cfg config.Config, user store.User, token string) error {
	serverURL, err := resolveServerURLForAuth(cfg)
	if err != nil {
		return err
	}
	if err := config.SaveRemoteCredentials(serverURL, user.Username, token); err != nil {
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
	location := strings.TrimSpace(os.Getenv("TICKET_URL"))
	if location == "" {
		return errors.New("ticket logout only works in remote mode; set TICKET_URL and try again")
	}
	resolved, err := config.ResolveLocation(location)
	if err != nil {
		return err
	}
	if resolved.Mode != config.ModeRemote || strings.TrimSpace(resolved.ServerURL) == "" {
		return errors.New("ticket logout requires a remote server URL; set TICKET_URL to http:// or https://")
	}
	creds, err := config.LoadCredentials()
	if err != nil {
		return err
	}
	remoteCreds, ok := creds.Remote(resolved.ServerURL)
	if !ok || strings.TrimSpace(remoteCreds.Token) == "" {
		return fmt.Errorf("no stored login session for %s; nothing to log out", resolved.ServerURL)
	}
	svc := libticket.NewHTTP(config.Config{
		Location: resolved.ServerURL,
		Username: remoteCreds.Username,
		Token:    remoteCreds.Token,
	})
	if err := svc.Logout(context.Background()); err != nil {
		if clearErr := config.ClearRemoteCredentials(resolved.ServerURL); clearErr != nil {
			return clearErr
		}
		return err
	}
	if err := config.ClearRemoteCredentials(resolved.ServerURL); err != nil {
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
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return runRemoteStatusWithSummaryStyle(cfg, statusUnicode)
}

func runCount(args []string) error {
	fs := flag.NewFlagSet("count", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	projectRef := fs.String("project_id", "", "limit counts to a project id, title, prefix, or alias")
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
	var (
		projectFilter *int64
		filterProject store.Project
	)
	if strings.TrimSpace(*projectRef) != "" {
		filterProject, err = resolveProjectFromFlagOrConfig(context.Background(), cfg, svc, strings.TrimSpace(*projectRef))
		if err != nil {
			return err
		}
		projectFilter = &filterProject.ID
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
			project = filterProject
		} else {
			_, resolvedSvc, currentProject, resolveErr := resolveCurrentProjectClient()
			if resolveErr != nil {
				return resolveErr
			}
			svc = resolvedSvc
			project = currentProject
		}
		resolvedStage, resolvedState, lifecycleErr := resolveLifecycleInput(*status, *stage, *state)
		if lifecycleErr != nil {
			return lifecycleErr
		}
		tickets, listErr := svc.ListTicketsFiltered(context.Background(), project.ID, *taskType, resolvedStage, resolvedState, "", *search, *assignee, 0, *includeAll)
		if listErr != nil {
			return listErr
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
			expected, parseErr := parseExpectedCount("expect_equals", *expectEquals)
			if parseErr != nil {
				return parseErr
			}
			if count != expected {
				return fmt.Errorf("expected count to equal %d, got %d", expected, count)
			}
		}
		if hasExpectNotEquals {
			expected, parseErr := parseExpectedCount("expect_notequals", *expectNotEquals)
			if parseErr != nil {
				return parseErr
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
	case "passkey":
		return runUserPasskey(args[1:])
	case "create", "new":
		fs := flag.NewFlagSet("user create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		usernameFlag := fs.String("username", "", "username")
		passwordFlag := fs.String("password", "", "password")
		emailFlag := fs.String("email", "", "email")
		printID := fs.Bool("printid", false, "print only the created user id")
		if parseErr := fs.Parse(args[1:]); parseErr != nil {
			return parseErr
		}
		username := strings.TrimSpace(*usernameFlag)
		if username == "" {
			username = currentOSUser()
		}
		if username == "" {
			return errors.New("username is required")
		}
		password := strings.TrimSpace(*passwordFlag)
		var (
			user              store.User
			generatedPassword string
		)
		if svcWithParams, ok := svc.(interface {
			CreateUserWithParams(context.Context, libticket.UserCreateParams) (store.User, string, error)
		}); ok {
			user, generatedPassword, err = svcWithParams.CreateUserWithParams(context.Background(), libticket.UserCreateParams{
				Username: username,
				Password: password,
				Email:    strings.TrimSpace(*emailFlag),
			})
		} else {
			if password == "" {
				password, err = generatePassword(24)
				if err != nil {
					return err
				}
				generatedPassword = password
			}
			user, err = svc.CreateUser(context.Background(), username, password)
		}
		if err != nil {
			return err
		}
		if outputJSON {
			if generatedPassword != "" {
				return printJSON(map[string]any{"user": user, "password": generatedPassword})
			}
			return printJSON(user)
		}
		if printCreatedID(user.ID, *printID) {
			return nil
		}
		fmt.Printf("created user %s\n", user.Username)
		if generatedPassword != "" {
			fmt.Printf("password: %s\n", generatedPassword)
		}
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
	case "notifications":
		fs := flag.NewFlagSet("user notifications", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		status := fs.String("status", "", "filter by status")
		limit := fs.Int("limit", 20, "max notifications to return")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("usage: tk user notifications [-status <unread|read>] [-limit <n>]")
		}
		notifications, err := svc.ListMyNotifications(context.Background(), strings.TrimSpace(*status), *limit)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(notifications)
		}
		printUserNotificationTable(notifications)
		return nil
	case "read-notification":
		fs := flag.NewFlagSet("user read-notification", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		notificationID := fs.Int64("id", 0, "notification id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("usage: tk user read-notification -id <notification-id>")
		}
		if *notificationID <= 0 {
			return errors.New("notification id must be greater than zero")
		}
		notification, err := svc.MarkNotificationRead(context.Background(), *notificationID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(notification)
		}
		fmt.Printf("marked notification as read: notification_id=%d status=%s title=%s\n", notification.ID, notification.Status, notification.Title)
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
