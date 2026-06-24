package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/libticket"
)

const inviteUsage = "usage: tk invite <email|username> -project <id|title|prefix|alias>\n" +
	"  [-role <observer|commenter|member|admin>]"

// runInvite adds a user — resolved by email or username — to a project in a
// role, joining them to that project's set of users (TK-129). It is a friendly
// front door over `tk project add-user`, which requires an exact user id.
func runInvite(args []string) error {
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		fmt.Println(inviteUsage)
		return nil
	}
	fs := flag.NewFlagSet("invite", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	role := fs.String("role", "member", "project role [observer,commenter,member,admin]")

	// The user identifier is positional and conventionally leads (tk invite
	// <user> -project P). Go's flag parser stops at the first positional, so peel
	// a leading non-flag arg off before parsing; otherwise fall back to a
	// trailing positional. -project / -project_id are global flags, extracted
	// before dispatch into the run's configured project reference.
	positional := ""
	flagArgs := args
	leading := false
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		positional = args[0]
		flagArgs = args[1:]
		leading = true
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	// Count positionals seen so exactly one (the user) is accepted.
	extra := fs.NArg()
	if !leading {
		positional = fs.Arg(0)
		extra = fs.NArg() - 1
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	target := strings.TrimSpace(positional)
	ref := strings.TrimSpace(resolveConfiguredProjectReference(cfg))
	if target == "" || ref == "" || extra > 0 {
		return errors.New(inviteUsage)
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	project, err := svc.GetProject(context.Background(), ref)
	if err != nil {
		return err
	}

	users, err := svc.ListUsers(context.Background())
	if err != nil {
		return err
	}
	needle := strings.ToLower(target)
	matchedID := ""
	matchedName := ""
	for _, u := range users {
		if strings.ToLower(u.Username) == needle || (u.Email != "" && strings.ToLower(u.Email) == needle) {
			matchedID = u.ID
			matchedName = u.Username
			break
		}
	}
	if matchedID == "" {
		return fmt.Errorf("no user found matching %q (by username or email)", target)
	}

	member, err := svc.AddProjectMember(context.Background(), project.ID, libticket.ProjectMemberRequest{
		UserID: matchedID,
		Role:   strings.TrimSpace(*role),
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(member)
	}
	fmt.Printf("invited %s to project %s as %s\n", matchedName, project.Prefix, member.Role)
	return nil
}
