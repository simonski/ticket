package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/config"
)

func runRemote(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(remoteUsage)
		return nil
	}

	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("remote add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		username := fs.String("username", "", "username to store for this remote")
		password := fs.String("password", "", "password to exchange for and store a session token")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 2 {
			return errors.New("usage: tk remote add NAME URL [-username <name>] [-password <password>]")
		}
		name := strings.TrimSpace(fs.Arg(0))
		rawURL := strings.TrimSpace(fs.Arg(1))
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		cfg, err = config.AddRemote(cfg, config.Remote{Name: name, URL: rawURL})
		if err != nil {
			return err
		}
		if cfg.DefaultRemote == "" {
			cfg.DefaultRemote = name
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		remote, _ := cfg.RemoteByName(name)
		if strings.TrimSpace(*username) != "" || strings.TrimSpace(*password) != "" {
			if strings.TrimSpace(*username) == "" || strings.TrimSpace(*password) == "" {
				return errors.New("username and password must be supplied together")
			}
			loginCfg := cfg
			loginCfg.Location = remote.URL
			svc, err := resolveService(loginCfg)
			if err != nil {
				return err
			}
			user, token, err := svc.Login(context.Background(), *username, *password)
			if err != nil {
				return err
			}
			if err := config.SaveRemoteCredentials(remote.URL, user.Username, token); err != nil {
				return err
			}
		}
		if outputJSON {
			return printJSON(map[string]any{"name": remote.Name, "url": remote.URL, "default": cfg.DefaultRemote == remote.Name})
		}
		fmt.Printf("added remote %s -> %s\n", remote.Name, remote.URL)
		return nil
	case "ls", "list":
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(cfg.Remotes)
		}
		if len(cfg.Remotes) == 0 {
			fmt.Println("(no remotes)")
			return nil
		}
		rows := []string{"NAME\tURL\tDEFAULT"}
		for _, remote := range cfg.Remotes {
			defaultMarker := ""
			if remote.Name == cfg.DefaultRemote {
				defaultMarker = "*"
			}
			rows = append(rows, fmt.Sprintf("%s\t%s\t%s", remote.Name, remote.URL, defaultMarker))
		}
		printBoxTable(rows[0], rows[1:])
		return nil
	case "remove", "rm", "delete":
		if len(args) != 2 {
			return errors.New("usage: tk remote remove NAME")
		}
		name := strings.TrimSpace(args[1])
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.DefaultRemote == name {
			return fmt.Errorf("remote %q is the default remote and cannot be removed", name)
		}
		if projectPath, ok, _ := config.ProjectPath(); ok {
			projectCfg, _ := config.Load()
			if strings.TrimSpace(projectCfg.Remote) == name {
				return fmt.Errorf("remote %q is currently selected by %s", name, projectPath)
			}
		}
		remote, ok := cfg.RemoteByName(name)
		if !ok {
			return fmt.Errorf("remote %q not found", name)
		}
		cfg, removed := config.RemoveRemote(cfg, name)
		if !removed {
			return fmt.Errorf("remote %q not found", name)
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		if err := config.ClearRemoteCredentials(remote.URL); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"status": "deleted", "name": name})
		}
		fmt.Printf("removed remote %s\n", name)
		return nil
	default:
		return fmt.Errorf("unknown remote command %q", args[0])
	}
}

const remoteUsage = `Usage: tk remote <command> [args]

Commands:
  add NAME URL [-username <name>] [-password <password>]   Add a named remote
  ls                                                        List configured remotes
  remove NAME                                               Remove a named remote`
