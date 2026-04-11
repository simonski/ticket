package main

import (
	"errors"
	"fmt"

	"github.com/simonski/ticket/internal/config"
)

func runConfig(args []string) error {
	if len(args) < 1 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(configUsage)
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	switch args[0] {
	case "registration-enable":
		if len(args) != 1 {
			return errors.New("usage: ticket config registration-enable")
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		if err := svc.SetRegistrationEnabled(true); err != nil {
			return err
		}
		fmt.Println("registration_enabled=true")
		return nil
	case "registration-disable":
		if len(args) != 1 {
			return errors.New("usage: ticket config registration-disable")
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		if err := svc.SetRegistrationEnabled(false); err != nil {
			return err
		}
		fmt.Println("registration_enabled=false")
		return nil
	case "set":
		if len(args) != 3 {
			return errors.New("usage: ticket config set <key> <value>")
		}
		switch args[1] {
		case "location":
			cfg.Location = args[2]
		default:
			return fmt.Errorf("unknown config key %q", args[1])
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("%s=%s\n", args[1], args[2])
		return nil
	case "get":
		if len(args) != 2 {
			return errors.New("usage: ticket config get <key>")
		}
		switch args[1] {
		case "location":
			if cfg.Location != "" {
				fmt.Println(cfg.Location)
			}
			return nil
		case "registration_enabled":
			svc, err := resolveService(cfg)
			if err != nil {
				return err
			}
			status, err := svc.Status()
			if err != nil {
				return err
			}
			fmt.Println(status.RegistrationEnabled)
			return nil
		default:
			return fmt.Errorf("unknown config key %q", args[1])
		}
	case "ls", "list":
		if len(args) != 1 {
			return errors.New("usage: ticket config ls")
		}
		r, _ := config.ResolveURL()
		printBoxTable("KEY\tVALUE", []string{
			fmt.Sprintf("location\t%s", cfg.Location),
			fmt.Sprintf("mode\t%s", r.Mode),
			fmt.Sprintf("username\t%s", cfg.Username),
			fmt.Sprintf("project_id\t%s", cfg.ProjectID),
			fmt.Sprintf("current_epic_id\t%s", cfg.CurrentEpicID),
		})
		return nil
	case "rm", "delete":
		if len(args) != 2 {
			return errors.New("usage: ticket config rm|delete <key>")
		}
		switch args[1] {
		case "location":
			cfg.Location = ""
		case "username":
			cfg.Username = ""
		case "project_id":
			cfg.ProjectID = ""
		case "current_epic_id":
			cfg.CurrentEpicID = ""
		default:
			return fmt.Errorf("unknown config key %q", args[1])
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("deleted %s\n", args[1])
		return nil
	default:
		fmt.Println(configUsage)
		return fmt.Errorf("unknown config action %q", args[0])
	}
}
