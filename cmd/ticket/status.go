package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

func runRemoteStatus(cfg config.Config) error {
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	serverURL := strings.TrimSpace(resolved.ServerURL)
	if serverURL == "" {
		return errors.New("TICKET_URL is required for remote mode")
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	status, err := svc.Status()
	authenticated := err == nil && status.Authenticated
	username := strings.TrimSpace(cfg.Username)
	if status.User != nil {
		username = status.User.Username
	}
	if outputJSON {
		return printJSON(map[string]any{
			"TICKET_URL":    serverURL,
			"username":      username,
			"authenticated": authenticated,
			"connection":    map[bool]string{true: "success", false: "failure"}[err == nil],
		})
	}
	fmt.Printf("TICKET_URL: %s\n", serverURL)
	fmt.Printf("username: %s\n", username)
	fmt.Printf("authenticated: %t\n", authenticated)
	printConnectionLine(err == nil)
	return err
}

func runLocalStatus() error {
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	dbPath := resolved.DBPath
	_, statErr := os.Stat(dbPath)
	dbExists := statErr == nil
	if outputJSON {
		return printJSON(map[string]any{
			"TICKET_URL": "file://" + dbPath,
			"db_exists":  dbExists,
			"connection": map[bool]string{true: "success", false: "failure"}[localStatusCheck(dbPath) == nil],
		})
	}
	fmt.Printf("TICKET_URL: file://%s\n", dbPath)
	fmt.Printf("db_exists: %t\n", dbExists)
	err = localStatusCheck(dbPath)
	printConnectionLine(err == nil)
	if !dbExists {
		fmt.Println("hint: run ticket init")
	}
	return err
}

func localStatusCheck(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil {
		return err
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&count); err != nil {
		return err
	}
	return nil
}

func printConnectionLine(ok bool) {
	status := "failure"
	color := "\x1b[31m"
	if ok {
		status = "success"
		color = "\x1b[32m"
	}
	if noColorOutput {
		fmt.Printf("connection: %s\n", status)
		return
	}
	fmt.Printf("connection: %s%s\x1b[0m\n", color, status)
}
