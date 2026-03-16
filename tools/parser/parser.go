package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type parseError struct {
	Line    int
	Message string
}

func (e parseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Message)
}

type entry struct {
	Kind         string
	Title        string
	ID           string
	Description  string
	AC           []string
	Priority     int
	DependsOn    []string
	EpicID       string
	Line         int
	IDLine       int
	DescLine     int
	ACLine       int
	PriorityLine int
	DependsLine  int
}

var (
	epicIDPattern  = regexp.MustCompile(`^E[0-9]+$`)
	storyIDPattern = regexp.MustCompile(`^E[0-9]+-S[0-9]+$`)
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("parser", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	filePath := fs.String("f", "", "requirements markdown file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*filePath) == "" {
		return errors.New("usage: parser -f REQUIREMENTS.md")
	}
	if fs.NArg() != 0 {
		return errors.New("usage: parser -f REQUIREMENTS.md")
	}

	entries, err := parseRequirements(*filePath)
	if err != nil {
		return err
	}
	commands := buildCommands(entries)
	fmt.Print(strings.Join(commands, "\n\n"))
	if len(commands) > 0 {
		fmt.Println()
	}
	return nil
}

func parseRequirements(path string) ([]entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		entries     []entry
		current     *entry
		currentEpic *entry
		inAC        bool
	)

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(line, "EPIC: ") {
			if current != nil {
				if err := validateEntry(*current); err != nil {
					return nil, err
				}
				entries = append(entries, *current)
			}
			title := strings.TrimSpace(strings.TrimPrefix(line, "EPIC: "))
			current = &entry{Kind: "epic", Title: title, Line: lineNo}
			currentEpic = current
			inAC = false
			continue
		}

		if strings.HasPrefix(line, "    STORY: ") {
			if currentEpic == nil {
				return nil, parseError{Line: lineNo, Message: "story declared before any epic"}
			}
			if current != nil {
				if err := validateEntry(*current); err != nil {
					return nil, err
				}
				entries = append(entries, *current)
			}
			title := strings.TrimSpace(strings.TrimPrefix(line, "    STORY: "))
			current = &entry{Kind: "story", Title: title, EpicID: currentEpic.ID, Line: lineNo}
			inAC = false
			continue
		}

		if current == nil {
			return nil, parseError{Line: lineNo, Message: "content found before first epic"}
		}

		if inAC && strings.HasPrefix(trimmed, "- ") {
			current.AC = append(current.AC, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
			continue
		}
		inAC = false

		switch {
		case strings.HasPrefix(trimmed, "ID: "):
			if current.IDLine != 0 {
				return nil, parseError{Line: lineNo, Message: "duplicate ID field"}
			}
			current.ID = strings.TrimSpace(strings.TrimPrefix(trimmed, "ID: "))
			current.IDLine = lineNo
			if current.Kind == "epic" {
				currentEpic = current
			}
		case strings.HasPrefix(trimmed, "DESCRIPTION: "):
			if current.DescLine != 0 {
				return nil, parseError{Line: lineNo, Message: "duplicate DESCRIPTION field"}
			}
			current.Description = strings.TrimSpace(strings.TrimPrefix(trimmed, "DESCRIPTION: "))
			current.DescLine = lineNo
		case trimmed == "AC:":
			if current.ACLine != 0 {
				return nil, parseError{Line: lineNo, Message: "duplicate AC field"}
			}
			current.ACLine = lineNo
			inAC = true
		case strings.HasPrefix(trimmed, "PRIORITY: "):
			if current.PriorityLine != 0 {
				return nil, parseError{Line: lineNo, Message: "duplicate PRIORITY field"}
			}
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "PRIORITY: "))
			priority, err := strconv.Atoi(value)
			if err != nil {
				return nil, parseError{Line: lineNo, Message: "priority must be numeric"}
			}
			current.Priority = priority
			current.PriorityLine = lineNo
		case strings.HasPrefix(trimmed, "DEPENDS-ON: "):
			if current.DependsLine != 0 {
				return nil, parseError{Line: lineNo, Message: "duplicate DEPENDS-ON field"}
			}
			raw := strings.TrimSpace(strings.TrimPrefix(trimmed, "DEPENDS-ON: "))
			current.DependsLine = lineNo
			if !strings.EqualFold(raw, "NONE") && raw != "" {
				parts := strings.Split(raw, ",")
				for _, part := range parts {
					dep := strings.TrimSpace(part)
					if dep != "" {
						current.DependsOn = append(current.DependsOn, dep)
					}
				}
			}
		default:
			return nil, parseError{Line: lineNo, Message: fmt.Sprintf("unrecognized content %q", trimmed)}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if current != nil {
		if err := validateEntry(*current); err != nil {
			return nil, err
		}
		entries = append(entries, *current)
	}
	if len(entries) == 0 {
		return nil, errors.New("no entries found")
	}
	if err := validateIntegrity(entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func validateEntry(e entry) error {
	if strings.TrimSpace(e.Title) == "" {
		return parseError{Line: e.Line, Message: fmt.Sprintf("%s title is required", e.Kind)}
	}
	if strings.TrimSpace(e.ID) == "" {
		return parseError{Line: e.Line, Message: "ID is required"}
	}
	if strings.TrimSpace(e.Description) == "" {
		return parseError{Line: e.Line, Message: "DESCRIPTION is required"}
	}
	if e.ACLine == 0 || len(e.AC) == 0 {
		return parseError{Line: e.Line, Message: "AC must contain at least one bullet"}
	}
	if e.PriorityLine == 0 {
		return parseError{Line: e.Line, Message: "PRIORITY is required"}
	}
	if e.Priority < 1 {
		return parseError{Line: e.PriorityLine, Message: "priority must be 1 or greater"}
	}
	if e.DependsLine == 0 {
		return parseError{Line: e.Line, Message: "DEPENDS-ON is required"}
	}
	return nil
}

func validateIntegrity(entries []entry) error {
	ids := make(map[string]entry, len(entries))
	for _, e := range entries {
		if _, exists := ids[e.ID]; exists {
			return parseError{Line: e.IDLine, Message: fmt.Sprintf("duplicate ID %s", e.ID)}
		}
		switch e.Kind {
		case "epic":
			if !epicIDPattern.MatchString(e.ID) {
				return parseError{Line: e.IDLine, Message: fmt.Sprintf("invalid epic ID %s", e.ID)}
			}
		case "story":
			if !storyIDPattern.MatchString(e.ID) {
				return parseError{Line: e.IDLine, Message: fmt.Sprintf("invalid story ID %s", e.ID)}
			}
			if e.EpicID == "" {
				return parseError{Line: e.Line, Message: "story is missing parent epic context"}
			}
			if !strings.HasPrefix(e.ID, e.EpicID+"-S") {
				return parseError{Line: e.IDLine, Message: fmt.Sprintf("story ID %s does not belong to epic %s", e.ID, e.EpicID)}
			}
		default:
			return parseError{Line: e.Line, Message: fmt.Sprintf("invalid entry kind %s", e.Kind)}
		}
		ids[e.ID] = e
	}
	for _, e := range entries {
		for _, dep := range e.DependsOn {
			target, ok := ids[dep]
			if !ok {
				return parseError{Line: e.DependsLine, Message: fmt.Sprintf("unknown dependency %s", dep)}
			}
			if dep == e.ID {
				return parseError{Line: e.DependsLine, Message: "entry cannot depend on itself"}
			}
			if e.Kind == "epic" && target.Kind == "story" {
				return parseError{Line: e.DependsLine, Message: fmt.Sprintf("epic %s cannot depend on story %s", e.ID, dep)}
			}
		}
	}
	return nil
}

func buildCommands(entries []entry) []string {
	var commands []string
	for _, e := range entries {
		commands = append(commands, buildCreateCommand(e))
	}
	for _, e := range entries {
		if len(e.DependsOn) > 0 {
			commands = append(commands, buildDependencyCommand(e, e.DependsOn))
		}
	}
	return commands
}

func buildCreateCommand(e entry) string {
	varName := shellVarName(e.ID)
	taskType := "task"
	if e.Kind == "epic" {
		taskType = "epic"
	}
	description := strings.TrimSpace(fmt.Sprintf("Source ID: %s\n\n%s", e.ID, e.Description))
	ac := append([]string{}, e.AC...)
	ac = append(ac, "Additional context: review docs/RULES.md, docs/DESIGN.md, and USER_GUIDE.md.")

	var args []string
	args = append(args, "task", "create", "-t", shellQuote(taskType))
	args = append(args, "-title", shellQuote(e.Title))
	args = append(args, "-d", shellQuote(description))
	args = append(args, "-ac", shellQuote(formatAC(ac)))
	args = append(args, "-p", strconv.Itoa(e.Priority))
	if e.Kind == "story" {
		args = append(args, "-parent", fmt.Sprintf("\"${%s}\"", shellVarName(e.EpicID)))
	}
	return fmt.Sprintf("%s=$(%s)", varName, strings.Join(args, " "))
}

func buildDependencyCommand(e entry, deps []string) string {
	var vars []string
	for _, dep := range deps {
		vars = append(vars, fmt.Sprintf("\"${%s}\"", shellVarName(dep)))
	}
	return fmt.Sprintf("task dependency add \"${%s}\" %s", shellVarName(e.ID), strings.Join(vars, ","))
}

func formatAC(items []string) string {
	var lines []string
	for _, item := range items {
		lines = append(lines, "- "+strings.TrimSpace(item))
	}
	return strings.Join(lines, "\n")
}

func shellVarName(id string) string {
	return strings.ReplaceAll(strings.TrimSpace(id), "-", "_")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func init() {
	if base := filepath.Base(os.Args[0]); base == "parser" {
		flag.CommandLine = flag.NewFlagSet(base, flag.ExitOnError)
	}
}
