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
	"github.com/simonski/ticket/libticket"
)

const emailUsage = "usage: tk email <show|set|enable|disable>\n" +
	"  set [-host H] [-port N] [-username U] [-password P] [-from ADDR] [-from-name NAME] [-security none|starttls|tls]"

// runEmail manages the SMTP email-sender configuration (TK-132). It configures
// and enables/disables the sender; actually sending mail is separate (TK-138).
func runEmail(args []string) error {
	sub := "show"
	if len(args) > 0 {
		sub = args[0]
	}
	if sub == "help" || sub == "-h" || sub == "--help" {
		fmt.Println(emailUsage)
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
	ctx := context.Background()
	switch sub {
	case "show", "get", "status":
		ec, gerr := svc.GetEmailConfig(ctx)
		if gerr != nil {
			return gerr
		}
		return printEmailConfig(ec)
	case "enable":
		if eerr := svc.SetEmailEnabled(ctx, true); eerr != nil {
			return eerr
		}
		fmt.Println("email sending enabled")
		return nil
	case "disable":
		if eerr := svc.SetEmailEnabled(ctx, false); eerr != nil {
			return eerr
		}
		fmt.Println("email sending disabled")
		return nil
	case "set", "config":
		return runEmailSet(svc, args[1:])
	default:
		return errors.New(emailUsage)
	}
}

func runEmailSet(svc libticket.Service, args []string) error {
	fs := flag.NewFlagSet("email set", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	host := fs.String("host", "", "SMTP host")
	port := fs.Int("port", 0, "SMTP port")
	username := fs.String("username", "", "SMTP username")
	password := fs.String("password", "", "SMTP password")
	from := fs.String("from", "", "from address")
	fromName := fs.String("from-name", "", "from display name")
	security := fs.String("security", "", "none|starttls|tls")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	cur, err := svc.GetEmailConfig(ctx)
	if err != nil {
		return err
	}
	// A masked sentinel password must never be written back.
	if cur.Password == "********" {
		cur.Password = ""
	}
	updatePassword := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "host":
			cur.Host = *host
		case "port":
			cur.Port = *port
		case "username":
			cur.Username = *username
		case "password":
			cur.Password = *password
			updatePassword = true
		case "from":
			cur.FromAddress = *from
		case "from-name":
			cur.FromName = *fromName
		case "security":
			cur.Security = *security
		}
	})
	if serr := svc.SetEmailConfig(ctx, cur, updatePassword); serr != nil {
		return serr
	}
	saved, err := svc.GetEmailConfig(ctx)
	if err != nil {
		return err
	}
	fmt.Println("email configuration saved")
	return printEmailConfig(saved)
}

func printEmailConfig(ec store.EmailConfig) error {
	passwordState := "not set"
	if strings.TrimSpace(ec.Password) != "" {
		passwordState = "set"
	}
	if outputJSON {
		return printJSON(map[string]any{
			"enabled":      ec.Enabled,
			"host":         ec.Host,
			"port":         ec.Port,
			"username":     ec.Username,
			"from_address": ec.FromAddress,
			"from_name":    ec.FromName,
			"security":     ec.Security,
			"has_password": strings.TrimSpace(ec.Password) != "",
		})
	}
	enabled := "disabled"
	if ec.Enabled {
		enabled = "enabled"
	}
	fmt.Printf("email sending : %s\n", enabled)
	fmt.Printf("smtp host     : %s\n", emptyDash(ec.Host))
	fmt.Printf("smtp port     : %d\n", ec.Port)
	fmt.Printf("username      : %s\n", emptyDash(ec.Username))
	fmt.Printf("password      : %s\n", passwordState)
	fmt.Printf("from address  : %s\n", emptyDash(ec.FromAddress))
	fmt.Printf("from name     : %s\n", emptyDash(ec.FromName))
	fmt.Printf("security      : %s\n", ec.Security)
	return nil
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
