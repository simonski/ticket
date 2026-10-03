package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/simonski/ticket/internal/config"
)

var skillUsage = renderNamespaceUsage("SKILL", "tk skill <command>", [][2]string{
	{"print", "Print the embedded SKILL.md to stdout"},
	{"install", "Install or update the tk skill in this repository"},
	{"uninstall", "Remove the tk skill from this repository"},
})

func repositorySkillPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, ok := config.FindGitRoot(cwd)
	if !ok {
		return "", errors.New("not inside a git repository")
	}
	return filepath.Join(root, ".claude", "skills", "tk", "SKILL.md"), nil
}

func embeddedSkillVersion() string { return skillVersion(tkSkillContent) }

func skillVersion(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "version:"); ok {
			return strings.Trim(strings.TrimSpace(value), "\"'")
		}
	}
	return ""
}

func compareSkillVersions(local, embedded string) (int, bool) {
	parse := func(version string) ([]int, bool) {
		parts := strings.Split(version, ".")
		if len(parts) != 3 {
			return nil, false
		}
		values := make([]int, 3)
		for i, part := range parts {
			value, err := strconv.Atoi(part)
			if err != nil || value < 0 {
				return nil, false
			}
			values[i] = value
		}
		return values, true
	}
	a, aOK := parse(local)
	b, bOK := parse(embedded)
	if !aOK || !bOK {
		return 0, false
	}
	for i := range a {
		if a[i] < b[i] {
			return -1, true
		}
		if a[i] > b[i] {
			return 1, true
		}
	}
	return 0, true
}

// skillState returns the repository's directly installed skill status.
func skillState(path string) (state, version string, err error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "missing", "", nil
	}
	if err != nil {
		return "", "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "linked", "", nil
	}
	content, err := os.ReadFile(path) // #nosec G304 -- path is the fixed tk skill location under the discovered git root
	if err != nil {
		return "", "", err
	}
	localVersion := skillVersion(string(content))
	if string(content) == tkSkillContent {
		return "current", localVersion, nil
	}
	comparison, ok := compareSkillVersions(localVersion, embeddedSkillVersion())
	if !ok {
		return "unknown", localVersion, nil
	}
	if comparison < 0 {
		return "outdated", localVersion, nil
	}
	if comparison > 0 {
		return "newer", localVersion, nil
	}
	return "modified", localVersion, nil
}

func installRepositorySkill() error {
	path, err := repositorySkillPath()
	if err != nil {
		return err
	}
	return installSkillAt(path)
}

func installSkillAt(path string) error {
	state, version, err := skillState(path)
	if err != nil {
		return err
	}
	switch state {
	case "current":
		fmt.Printf("tk skill is current (version %s): %s\n", version, path)
		return nil
	case "newer", "modified", "unknown", "linked":
		return fmt.Errorf("existing tk skill at %s has %s content (version %q); review it before replacing", path, state, version)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(tkSkillContent), 0o600); err != nil {
		return err
	}
	verb := "installed"
	if state == "outdated" {
		verb = "updated"
	}
	fmt.Printf("%s tk skill (version %s): %s\n", verb, embeddedSkillVersion(), path)
	return nil
}

func uninstallRepositorySkill() error {
	path, err := repositorySkillPath()
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		fmt.Printf("tk skill is not installed: %s\n", path)
		return nil
	} else if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	// Leave a shared skills directory and any other files intact.
	_ = os.Remove(filepath.Dir(path))
	fmt.Printf("uninstalled tk skill: %s\n", path)
	return nil
}

func reviewRepositorySkill() error {
	path, err := repositorySkillPath()
	if err != nil {
		return err
	}
	state, version, err := skillState(path)
	if err != nil {
		return err
	}
	switch state {
	case "current":
		fmt.Printf("tk skill: current (version %s)\n", version)
		return nil
	case "newer", "modified", "unknown", "linked":
		fmt.Printf("tk skill: %s at %s (version %q); review it manually\n", state, path, version)
		return nil
	}
	action := "install"
	if state == "outdated" {
		action = "update"
	}
	if state == "missing" {
		fmt.Printf("tk skill: not installed in this repository (%s)\n", path)
	} else {
		fmt.Printf("tk skill: version %s installed; embedded version %s is available\n", version, embeddedSkillVersion())
	}
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) { // #nosec G115
		if promptYN(bufio.NewReader(os.Stdin), "Would you like to "+action+" the tk skill?", false) {
			return installSkillAt(path)
		}
	}
	fmt.Printf("Run `tk skill install` to %s it.\n", action)
	return nil
}
