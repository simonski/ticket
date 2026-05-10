package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

func runUpgradeDatabase(args []string) error {
	fs := flag.NewFlagSet("upgrade-database", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	outputPath := fs.String("o", filepath.Join("new_database", "ticket.db"), "target database file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("usage: tk upgrade-database [-o <target-db>]")
	}
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	if strings.TrimSpace(resolved.DBPath) == "" {
		return errors.New("ticket upgrade-database is a server-side maintenance command; pass -f <source-db> to select the database file")
	}
	target := strings.TrimSpace(*outputPath)
	if target == "" {
		return errors.New("usage: tk upgrade-database [-o <target-db>]")
	}
	if err := store.UpgradeDatabase(context.Background(), resolved.DBPath, target); err != nil {
		return err
	}
	fmt.Printf("upgraded database from %s to %s\n", resolved.DBPath, target)
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
