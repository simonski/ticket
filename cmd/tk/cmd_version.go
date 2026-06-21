package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

func runVersion(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: tk version")
	}
	fmt.Println(strings.TrimSpace(embeddedVersion))
	return nil
}

func runUpgrade(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: tk upgrade")
	}

	localVersion := strings.TrimSpace(embeddedVersion)
	repoVersion, err := fetchRepoVersion()
	if err != nil {
		return errors.New("Unable to check for updates right now. Check your network connection and try again.")
	}

	fmt.Printf("Local version: %s\n", localVersion)
	fmt.Printf("Repo version:  %s\n", repoVersion)

	switch compareVersions(localVersion, repoVersion) {
	case 0:
		fmt.Printf("You are on the latest version (%s)\n", localVersion)
	case -1:
		fmt.Println("A newer version of tk is available, upgrade using `go install github.com/simonski/ticket@latest`")
	default:
		fmt.Println("Your local copy is newer than the repo")
	}
	return nil
}

// runUpgradeDatabase upgrades a database IN PLACE, applying any pending additive
// schema migrations (missing tables/columns) and stamping the current schema
// version. It takes a timestamped .bak copy first unless -no-backup is given.
// The tk server also performs this upgrade automatically on startup.
func runUpgradeDatabase(args []string) error {
	fs := flag.NewFlagSet("upgrade-database", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dbPath := fs.String("f", "", "SQLite database file (default: resolved/local database)")
	noBackup := fs.Bool("no-backup", false, "skip the safety backup taken before upgrading")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("usage: tk admin upgrade-database [-f <db>] [-no-backup]")
	}

	path := strings.TrimSpace(*dbPath)
	if path == "" {
		if resolved, err := config.ResolveURL(); err == nil {
			path = strings.TrimSpace(resolved.DBPath)
		}
	}
	if path == "" {
		def, err := defaultDatabasePath()
		if err != nil {
			return err
		}
		path = def
	}
	if path == "" {
		return errors.New("could not resolve a local database; pass -f <db>")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("database not found at %s: %w", path, err)
	}

	if !*noBackup {
		backup := fmt.Sprintf("%s.bak-%s", path, time.Now().Format("20060102-150405"))
		if err := store.BackupDatabase(path, backup); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
		fmt.Printf("backup written: %s\n", backup)
	}

	from, err := store.UpgradeInPlace(context.Background(), path)
	if err != nil {
		return err
	}
	if from == store.CurrentSchemaVersion {
		fmt.Printf("upgraded %s in place (schema version %d; re-applied pending column migrations)\n", path, store.CurrentSchemaVersion)
	} else {
		fmt.Printf("upgraded %s in place (schema version %d -> %d)\n", path, from, store.CurrentSchemaVersion)
	}
	fmt.Println("restart the tk server to pick up the upgraded database")
	return nil
}

func defaultFetchRepoVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repoVersionURL, http.NoBody)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("version lookup failed with status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(body))
	if version == "" {
		return "", errors.New("empty repo version")
	}
	return version, nil
}

func compareVersions(left, right string) int {
	leftParts := parseVersionParts(left)
	rightParts := parseVersionParts(right)
	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}
	for i := 0; i < maxLen; i++ {
		var leftPart, rightPart int
		if i < len(leftParts) {
			leftPart = leftParts[i]
		}
		if i < len(rightParts) {
			rightPart = rightParts[i]
		}
		switch {
		case leftPart < rightPart:
			return -1
		case leftPart > rightPart:
			return 1
		}
	}
	return 0
}

func parseVersionParts(raw string) []int {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(raw), "v"))
	if trimmed == "" {
		return []int{0}
	}
	parts := strings.Split(trimmed, ".")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil {
			values = append(values, 0)
			continue
		}
		values = append(values, n)
	}
	return values
}
