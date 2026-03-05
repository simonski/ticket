package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultReadyLimit     = 100
	defaultLoopSleepSecs  = 1
	defaultPromptTemplate = "Perform the following:\n<BEAD>"
)

type issue struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Acceptance   string    `json:"acceptance_criteria"`
	Status       string    `json:"status"`
	Priority     int       `json:"priority"`
	IssueType    string    `json:"issue_type"`
	ExternalRef  string    `json:"external_ref"`
	Assignee     string    `json:"assignee"`
	Owner        string    `json:"owner"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Parent       string    `json:"parent"`
	Dependencies []dep     `json:"dependencies"`
}

type dep struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	DependencyType string `json:"dependency_type"`
	ExternalRef    string `json:"external_ref"`
}

type workStatus struct {
	StartedAt       string `json:"started_at"`
	CompletedAt     string `json:"completed_at"`
	ExitCode        int    `json:"exit_code"`
	Branch          string `json:"branch"`
	BeadID          string `json:"bead_id"`
	Title           string `json:"title"`
	Instruction     string `json:"instruction"`
	BeadStatusStart string `json:"bead_status_start"`
	BeadStatusEnd   string `json:"bead_status_end"`
}

var logicalIDPattern = regexp.MustCompile(`(?i)\[Logical ID:\s*([^\]]+)\]`)

func main() {
	if len(os.Args) == 1 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "loop":
		if err := runLoop(os.Args[2:]); err != nil {
			exitErr(err)
		}
	case "check":
		if err := runCheck(os.Args[2:]); err != nil {
			exitErr(err)
		}
	case "agent":
		if err := runAgent(os.Args[2:]); err != nil {
			exitErr(err)
		}
	case "-h", "--help", "help":
		printUsage()
	default:
		printUsage()
		exitErr(fmt.Errorf("unknown command %q", os.Args[1]))
	}
}

func runLoop(args []string) error {
	fs := flag.NewFlagSet("loop", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	var (
		name       string
		max        int
		dryRunSecs int
		readyLimit int
		sleepSecs  int
		noBranch   bool
	)

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: wiggum loop -name fred -max 1 [-dryrun N]\n\n")
		fmt.Fprintf(fs.Output(), "Chooses the next best ready bead for the named wiggum, assigns it,\n")
		fmt.Fprintf(fs.Output(), "creates or switches to a branch for the work, performs the work,\n")
		fmt.Fprintf(fs.Output(), "closes the bead, and repeats.\n\n")
		fmt.Fprintf(fs.Output(), "Flags:\n")
		fs.PrintDefaults()
	}

	fs.StringVar(&name, "name", "", "unique wiggum name used for assignment")
	fs.IntVar(&max, "max", 1, "maximum number of issues to process (0 = forever)")
	fs.IntVar(&dryRunSecs, "dryrun", -1, "simulate work and pass this integer to the dry-run invocation")
	fs.IntVar(&readyLimit, "limit", defaultReadyLimit, "maximum ready issues to consider per iteration")
	fs.IntVar(&sleepSecs, "sleep", defaultLoopSleepSecs, "number of seconds to sleep between loop iterations")
	fs.BoolVar(&noBranch, "no-branch", false, "do not create or switch git branches")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if name == "" {
		fs.Usage()
		return errors.New("missing required -name flag")
	}
	if sleepSecs < 0 {
		fs.Usage()
		return errors.New("sleep must be >= 0")
	}
	dryRun := dryRunSecs >= 0

	processed := 0
	for max == 0 || processed < max {
		ready, err := loadReady(readyLimit)
		if err != nil {
			return err
		}

		chosen, ok := chooseIssue(ready, name)
		if !ok {
			if processed == 0 {
				return errors.New("no suitable ready issue found")
			}
			return nil
		}

		if err := assignIssue(chosen.ID, name); err != nil {
			return err
		}

		full, err := showIssue(chosen.ID)
		if err != nil {
			return err
		}

		branchName := branchNameFor(name, full)
		branched := false
		if !noBranch && !dryRun {
			if err := switchBranch(branchName); err != nil {
				return err
			}
			branched = true
		}

		printWorkPacket(full, name, branchName, true, branched, dryRun)

		if dryRun {
			if err := performDryRunWork(full, name, branchName, branched, dryRunSecs); err != nil {
				return err
			}
		} else {
			if err := performWork(full, name, branchName, branched); err != nil {
				return err
			}
		}

		if err := closeIssue(full.ID, name, dryRun); err != nil {
			return err
		}
		if err := updateWorkStatusEnd(full, branchName, "closed"); err != nil {
			return fmt.Errorf("failed to update work status for %s: %w", full.ID, err)
		}

		processed++
		if (max == 0 || processed < max) && sleepSecs > 0 {
			time.Sleep(time.Duration(sleepSecs) * time.Second)
		}
	}

	return nil
}

func runCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	var (
		name       string
		readyLimit int
	)

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: wiggum check -name fred\n\n")
		fmt.Fprintf(fs.Output(), "Shows the next bead wiggum would work on, including the derived branch\n")
		fmt.Fprintf(fs.Output(), "name and work packet, without mutating beads state or git state.\n\n")
		fmt.Fprintf(fs.Output(), "Flags:\n")
		fs.PrintDefaults()
	}

	fs.StringVar(&name, "name", "", "unique wiggum name used for assignment filtering")
	fs.IntVar(&readyLimit, "limit", defaultReadyLimit, "maximum ready issues to consider")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if name == "" {
		fs.Usage()
		return errors.New("missing required -name flag")
	}

	ready, err := loadReady(readyLimit)
	if err != nil {
		return err
	}

	chosen, ok := chooseIssue(ready, name)
	if !ok {
		fmt.Printf("Wiggum: %s\n", name)
		fmt.Println("Next: none")
		fmt.Println("Reason: no suitable ready issue found")
		return nil
	}

	full, err := showIssue(chosen.ID)
	if err != nil {
		return err
	}

	branchName := branchNameFor(name, full)
	fmt.Print(renderWorkPacket(full, name, branchName, false, false, false))
	return nil
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  wiggum agent \"entire command\"")
	fmt.Println("  wiggum agent command [arg...]")
	fmt.Println("  wiggum check -name fred")
	fmt.Println("  wiggum loop -name fred -max 1")
	fmt.Println("  wiggum loop -name fred -max 1 -dryrun 5")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  agent   Run an interactive coding agent command with stdio passed through.")
	fmt.Println("  check   Show what wiggum would do next without changing beads or git.")
	fmt.Println("  loop    Claim the next best ready bead, work it, close it, and repeat.")
	fmt.Println()
	fmt.Println("Run `wiggum agent -h`, `wiggum check -h`, or `wiggum loop -h` for command flags.")
}

func runAgent(args []string) error {
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: wiggum agent \"entire command\"\n")
		fmt.Fprintf(fs.Output(), "       wiggum agent command [arg...]\n\n")
		fmt.Fprintf(fs.Output(), "Starts a child process and passes stdin, stdout, and stderr through directly,\n")
		fmt.Fprintf(fs.Output(), "so interactive coding agents behave as if they were launched without wiggum.\n")
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	cmd, err := buildAgentCommand(fs.Args())
	if err != nil {
		fs.Usage()
		return err
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("agent command failed: %w", err)
	}
	return nil
}

func buildAgentCommand(args []string) (*exec.Cmd, error) {
	switch len(args) {
	case 0:
		return nil, errors.New("missing agent command")
	case 1:
		return exec.Command("sh", "-c", args[0]), nil
	default:
		return exec.Command(args[0], args[1:]...), nil
	}
}

func loadReady(limit int) ([]issue, error) {
	args := []string{"ready", "--json", "--limit", fmt.Sprintf("%d", limit)}
	output, err := runBD(args...)
	if err != nil {
		return nil, err
	}

	var issues []issue
	if err := json.Unmarshal(output, &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

func chooseIssue(issues []issue, name string) (issue, bool) {
	candidates := make([]issue, 0, len(issues))
	for _, item := range issues {
		if item.Assignee != "" && !strings.EqualFold(item.Assignee, name) {
			continue
		}
		candidates = append(candidates, item)
	}

	if len(candidates) == 0 {
		return issue{}, false
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		leftAssigned := strings.EqualFold(candidates[i].Assignee, name)
		rightAssigned := strings.EqualFold(candidates[j].Assignee, name)
		if leftAssigned != rightAssigned {
			return leftAssigned
		}

		leftLeaf := isLeafWork(candidates[i].IssueType)
		rightLeaf := isLeafWork(candidates[j].IssueType)
		if leftLeaf != rightLeaf {
			return leftLeaf
		}

		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority < candidates[j].Priority
		}
		if !candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
		}
		return candidates[i].ID < candidates[j].ID
	})

	return candidates[0], true
}

func isLeafWork(issueType string) bool {
	switch issueType {
	case "task", "bug", "feature", "chore", "merge-request":
		return true
	default:
		return false
	}
}

func assignIssue(id, name string) error {
	_, err := runBD("update", id, "--assignee", name, "--status", "in_progress")
	if err != nil {
		return fmt.Errorf("failed to assign %s to %s: %w", id, name, err)
	}
	return nil
}

func showIssue(id string) (issue, error) {
	output, err := runBD("show", "--json", id)
	if err != nil {
		return issue{}, err
	}

	var issues []issue
	if err := json.Unmarshal(output, &issues); err != nil {
		return issue{}, err
	}
	if len(issues) == 0 {
		return issue{}, fmt.Errorf("bd show returned no issue for %s", id)
	}
	return issues[0], nil
}

func performWork(item issue, name, branch string, branched bool) error {
	err := runWorkCommand(item, name, branch, branched, false, 0)
	if err != nil {
		return fmt.Errorf("worker command failed for %s: %w", item.ID, err)
	}
	return nil
}

func performDryRunWork(item issue, name, branch string, branched bool, dryRunSecs int) error {
	return runWorkCommand(item, name, branch, branched, true, dryRunSecs)
}

func dryRunCommand(dryRunSecs int) string {
	return fmt.Sprintf("echo 'dry-run; sleep %d'", dryRunSecs)
}

func runWorkCommand(item issue, name, branch string, branched, dryRun bool, dryRunSecs int) error {
	startedAt := time.Now()
	input := renderWorkPacket(item, name, branch, true, branched, dryRun)
	prompt := renderPrompt(input)
	transcript := &lockedBuffer{}

	workDir := workDirFor(item)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}

	inputPath := filepath.Join(workDir, "input.md")
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		return err
	}

	promptPath := filepath.Join(workDir, "prompt.md")
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return err
	}

	var (
		cmd         *exec.Cmd
		err         error
		instruction string
	)
	if dryRun {
		instruction = dryRunCommand(dryRunSecs)
		cmd, err = buildAgentCommand([]string{instruction})
		if err != nil {
			return err
		}
		cmd.Stdin = strings.NewReader(prompt)
	} else {
		promptFile, openErr := os.Open(promptPath)
		if openErr != nil {
			return openErr
		}
		defer promptFile.Close()

		cmd = exec.Command("codex", "exec", "-")
		cmd.Stdin = promptFile
		instruction = "codex exec - < " + promptPath
	}
	cmd.Stdout = io.MultiWriter(os.Stdout, transcript)
	cmd.Stderr = io.MultiWriter(os.Stderr, transcript)
	cmd.Env = append(os.Environ(),
		"WIGGUM_NAME="+name,
		"WIGGUM_ISSUE_ID="+item.ID,
		"WIGGUM_LOGICAL_ID="+item.ExternalRef,
		"WIGGUM_BRANCH="+branch,
	)

	err = cmd.Run()
	completedAt := time.Now()
	exitCode := exitCodeFor(err)
	if logErr := writeWorkArtifacts(workDir, item, branch, instruction, transcript.String(), startedAt, completedAt, exitCode, item.Status, item.Status); logErr != nil {
		if err != nil {
			return fmt.Errorf("%w (also failed to write work artifacts: %v)", err, logErr)
		}
		return fmt.Errorf("failed to write work artifacts for %s: %w", item.ID, logErr)
	}
	return err
}

func closeIssue(id, name string, dryRun bool) error {
	reason := "completed by wiggum " + name
	if dryRun {
		reason = "dryrun completed by wiggum " + name
	}
	_, err := runBD("close", id, "--reason", reason)
	if err != nil {
		return fmt.Errorf("failed to close %s: %w", id, err)
	}
	return nil
}

func branchNameFor(name string, item issue) string {
	logicalID := item.ExternalRef
	if logicalID == "" {
		if match := logicalIDPattern.FindStringSubmatch(item.Description); len(match) == 2 {
			logicalID = match[1]
		}
	}

	prefix := slugify(logicalID)
	title := slugify(item.Title)
	switch {
	case prefix != "" && title != "":
		return "feature/" + slugify(name) + "/" + prefix + "-" + title
	case title != "":
		return "feature/" + slugify(name) + "/" + title
	case prefix != "":
		return "feature/" + slugify(name) + "/" + prefix
	default:
		return "feature/" + slugify(name) + "/" + slugify(item.ID)
	}
}

func slugify(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return ""
	}

	var b strings.Builder
	lastDash := false
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(b.String(), "-")
}

func switchBranch(branch string) error {
	if branch == "" {
		return errors.New("empty branch name")
	}

	if err := exec.Command("git", "rev-parse", "--verify", branch).Run(); err == nil {
		cmd := exec.Command("git", "switch", branch)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to switch to branch %s: %s", branch, strings.TrimSpace(string(output)))
		}
		return nil
	}

	cmd := exec.Command("git", "switch", "-c", branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create branch %s: %s", branch, strings.TrimSpace(string(output)))
	}
	return nil
}

func printWorkPacket(item issue, name, branch string, assigned, branched, dryRun bool) {
	fmt.Print(renderPrompt(renderWorkPacket(item, name, branch, assigned, branched, dryRun)))
}

func renderWorkPacket(item issue, name, branch string, assigned, branched, dryRun bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Wiggum: %s\n", name)
	fmt.Fprintf(&b, "ID: %s\n", item.ID)
	if item.ExternalRef != "" {
		fmt.Fprintf(&b, "Logical ID: %s\n", item.ExternalRef)
	}
	fmt.Fprintf(&b, "Type: %s\n", item.IssueType)
	fmt.Fprintf(&b, "Priority: %d\n", item.Priority)
	fmt.Fprintf(&b, "Status: %s\n", item.Status)
	fmt.Fprintf(&b, "Assignee: %s\n", item.Assignee)
	fmt.Fprintf(&b, "Owner: %s\n", item.Owner)
	fmt.Fprintf(&b, "Title: %s\n", item.Title)
	fmt.Fprintf(&b, "Branch: %s\n", branch)
	fmt.Fprintf(&b, "Assigned: %t\n", assigned)
	fmt.Fprintf(&b, "Branch Switched: %t\n", branched)
	fmt.Fprintf(&b, "Dry Run: %t\n\n", dryRun)

	fmt.Fprintln(&b, "Description:")
	fmt.Fprintln(&b, strings.TrimSpace(item.Description))
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "Acceptance Criteria:")
	for _, part := range splitAcceptance(item.Acceptance) {
		fmt.Fprintf(&b, "- %s\n", part)
	}
	fmt.Fprintln(&b)

	if len(item.Dependencies) > 0 {
		fmt.Fprintln(&b, "Dependencies:")
		for _, dep := range item.Dependencies {
			ref := dep.ID
			if dep.ExternalRef != "" {
				ref = dep.ExternalRef + " (" + dep.ID + ")"
			}
			fmt.Fprintf(&b, "- %s: %s [%s]\n", ref, dep.Title, dep.Status)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "Next:")
	fmt.Fprintf(&b, "- Start work on %s\n", item.ID)
	fmt.Fprintln(&b, "- Run tests with make before closing the issue")
	return b.String()
}

func renderPrompt(packet string) string {
	template := strings.TrimSpace(os.Getenv("WIGGUM_PROMPT_TEMPLATE"))
	if template == "" {
		template = defaultPromptTemplate
	}
	if strings.Contains(template, "<BEAD>") {
		return strings.ReplaceAll(template, "<BEAD>", packet)
	}
	if strings.HasSuffix(template, "\n") {
		return template + packet
	}
	return template + "\n" + packet
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func writeWorkArtifacts(workDir string, item issue, branch, instruction, transcript string, startedAt, completedAt time.Time, exitCode int, beadStatusStart, beadStatusEnd string) error {
	outputPath := filepath.Join(workDir, "output.md")
	if err := os.WriteFile(outputPath, []byte(transcript), 0o644); err != nil {
		return err
	}

	status := workStatus{
		StartedAt:       startedAt.Format(time.RFC3339Nano),
		CompletedAt:     completedAt.Format(time.RFC3339Nano),
		ExitCode:        exitCode,
		Branch:          branch,
		BeadID:          item.ID,
		Title:           item.Title,
		Instruction:     instruction,
		BeadStatusStart: beadStatusStart,
		BeadStatusEnd:   beadStatusEnd,
	}

	statusBytes, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	statusBytes = append(statusBytes, '\n')
	return os.WriteFile(filepath.Join(workDir, "status.json"), statusBytes, 0o644)
}

func updateWorkStatusEnd(item issue, branch, endStatus string) error {
	workDir := workDirFor(item)
	statusPath := filepath.Join(workDir, "status.json")

	data, err := os.ReadFile(statusPath)
	if err != nil {
		return err
	}

	var status workStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return err
	}

	status.Branch = branch
	status.BeadStatusEnd = endStatus

	statusBytes, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	statusBytes = append(statusBytes, '\n')
	return os.WriteFile(statusPath, statusBytes, 0o644)
}

func workDirFor(item issue) string {
	return filepath.Join("logs", sanitizeLogComponent(item.ID)+"-"+sanitizeLogComponent(item.Title))
}

func sanitizeLogComponent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}

	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '.' || r == '_' || r == '-':
			if !(r == '-' && lastDash) {
				b.WriteRune(r)
				lastDash = r == '-'
			}
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	return out
}

func exitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func splitAcceptance(input string) []string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "|", "\n")
	parts := strings.Split(input, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func runBD(args ...string) ([]byte, error) {
	cmd := exec.Command("bd", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New(msg)
	}
	return stdout.Bytes(), nil
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
