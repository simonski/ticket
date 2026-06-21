package main

import (
	"context"
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
			return errors.New("usage: tk admin config registration-enable")
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		if err := svc.SetRegistrationEnabled(context.Background(), true); err != nil {
			return err
		}
		fmt.Println("registration_enabled=true")
		return nil
	case "registration-disable":
		if len(args) != 1 {
			return errors.New("usage: tk admin config registration-disable")
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		if err := svc.SetRegistrationEnabled(context.Background(), false); err != nil {
			return err
		}
		fmt.Println("registration_enabled=false")
		return nil
	case "registration-autoapprove-enable":
		if len(args) != 1 {
			return errors.New("usage: tk admin config registration-autoapprove-enable")
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		if err := svc.SetRegistrationAutoApprove(context.Background(), true); err != nil {
			return err
		}
		fmt.Println("registration_auto_approve=true")
		return nil
	case "registration-autoapprove-disable":
		if len(args) != 1 {
			return errors.New("usage: tk admin config registration-autoapprove-disable")
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		if err := svc.SetRegistrationAutoApprove(context.Background(), false); err != nil {
			return err
		}
		fmt.Println("registration_auto_approve=false")
		return nil
	case "set":
		return errors.New("tk admin config set has been removed; use TICKET_URL, TICKET_PROJECT, and tk login instead")
	case "get":
		if len(args) != 2 {
			return errors.New("usage: tk admin config get <key>")
		}
		switch args[1] {
		case "registration_enabled":
			svc, err := resolveService(cfg)
			if err != nil {
				return err
			}
			status, err := svc.Status(context.Background())
			if err != nil {
				return err
			}
			fmt.Println(status.RegistrationEnabled)
			return nil
		case "registration_auto_approve":
			svc, err := resolveService(cfg)
			if err != nil {
				return err
			}
			status, err := svc.Status(context.Background())
			if err != nil {
				return err
			}
			fmt.Println(status.RegistrationAutoApprove)
			return nil
		default:
			return fmt.Errorf("unknown config key %q", args[1])
		}
	case "ls", "list":
		if len(args) != 1 {
			return errors.New("usage: tk admin config ls")
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		status, err := svc.Status(context.Background())
		if err != nil {
			return err
		}
		printBoxTable("KEY\tVALUE", []string{
			fmt.Sprintf("registration_enabled\t%t", status.RegistrationEnabled),
			fmt.Sprintf("registration_auto_approve\t%t", status.RegistrationAutoApprove),
		})
		return nil
	case "rm", "delete":
		return errors.New("tk admin config rm has been removed; unset the relevant environment variable instead")
	default:
		fmt.Println(configUsage)
		return fmt.Errorf("unknown config action %q", args[0])
	}
}

func runAdmin(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(adminUsage)
		return nil
	}
	switch args[0] {
	case "config":
		return runConfig(args[1:])
	case "export":
		return runExportSnapshot(args[1:])
	case "import":
		return runImportSnapshot(args[1:])
	case "role":
		return runRole(args[1:])
	case "workflow":
		return runWorkflow(args[1:])
	case "team":
		return runTeam(args[1:])
	case "agent":
		return runAgent(args[1:])
	case "user":
		return runUser(args[1:])
	case "upgrade-database":
		return runUpgradeDatabase(args[1:])
	default:
		fmt.Println(adminUsage)
		return fmt.Errorf("unknown admin command %q", args[0])
	}
}
