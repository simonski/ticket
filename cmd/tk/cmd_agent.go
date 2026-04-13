package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func runAgent(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(agentUsage)
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

	// resolveAgentID validates and returns the agent UUID string as the ID.
	resolveAgentID := func(uuid string) (string, error) {
		uuid = strings.TrimSpace(uuid)
		if uuid == "" {
			return "", errors.New("agent UUID is required")
		}
		return uuid, nil
	}

	switch args[0] {
	case "create", "new":
		fs := flag.NewFlagSet("agent create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		password := fs.String("password", "", "agent password")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		agent, generatedPassword, err := svc.CreateAgent(context.Background(), libticket.AgentCreateRequest{
			Password: strings.TrimSpace(*password),
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"agent": agent, "password": generatedPassword})
		}
		fmt.Printf("agent_id: %s\n", agent.ID)
		fmt.Printf("password: %s\n", generatedPassword)
		return nil
	case "ls", "list":
		statuses, err := svc.ListAgentStatuses(context.Background())
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(statuses)
		}
		printAgentTable(statuses)
		return nil
	case "udpate", "update":
		fs := flag.NewFlagSet("agent update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "agent UUID")
		password := fs.String("password", "", "agent password")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		dbID, err := resolveAgentID(*id)
		if err != nil {
			return err
		}
		trimmed := strings.TrimSpace(*password)
		if trimmed == "" {
			return errors.New("agent update requires -password")
		}
		agent, err := svc.UpdateAgent(context.Background(), dbID, libticket.AgentUpdateRequest{
			Password: &trimmed,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(agent)
		}
		fmt.Printf("updated agent %s\n", agent.ID)
		return nil
	case "rm", "delete":
		fs := flag.NewFlagSet("agent "+args[0], flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "agent UUID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		dbID, err := resolveAgentID(*id)
		if err != nil {
			return err
		}
		if err := svc.DeleteAgent(context.Background(), dbID); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"status": "deleted", "agent_id": *id})
		}
		fmt.Printf("deleted agent %s\n", *id)
		return nil
	case "enable", "disable":
		fs := flag.NewFlagSet("agent "+args[0], flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "agent UUID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		dbID, err := resolveAgentID(*id)
		if err != nil {
			return err
		}
		agent, err := svc.SetAgentEnabled(context.Background(), dbID, args[0] == "enable")
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(agent)
		}
		fmt.Printf("%sd agent %s\n", args[0], agent.ID)
		return nil
	case "run":
		fs := flag.NewFlagSet("agent run", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		agentID := fs.String("id", "", "agent UUID")
		projectID := fs.Int64("project-id", 0, "project id override")
		pollSeconds := fs.Int("poll-seconds", 5, "idle poll interval seconds")
		llmCommand := fs.String("llm", envValue("TICKET_AGENT_LLM"), "llm command (claude, codex, or path to binary)")
		verbose := fs.Bool("v", false, "verbose logging")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		agentIDVal := strings.TrimSpace(*agentID)
		if agentIDVal == "" {
			agentIDVal = envValue("AGENT_ID")
		}
		agentPassword := envValue("AGENT_PASSWORD")
		if agentPassword == "" {
			fmt.Fprint(os.Stdout, "agent password: ")
			var err error
			agentPassword, err = readPasswordPrompt(bufio.NewReader(os.Stdin), os.Stdin, os.Stdout)
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
			agentPassword = strings.TrimSpace(agentPassword)
		}
		resolved, rErr := config.ResolveURL()
		if rErr != nil || resolved.Mode != config.ModeRemote {
			return errors.New("agent run requires remote mode (run tk init to configure)")
		}
		missing := make([]string, 0, 2)
		if agentIDVal == "" {
			missing = append(missing, "AGENT_ID or -id")
		}
		if agentPassword == "" {
			missing = append(missing, "AGENT_PASSWORD (or prompt)")
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing required values: %s", strings.Join(missing, ", "))
		}
		if *pollSeconds < 1 {
			return errors.New("poll-seconds must be >= 1")
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		registerRequest := libticket.AgentRegisterRequest{
			ID:       agentIDVal,
			Password: agentPassword,
		}
		fmt.Printf("[agent] REGISTER request: ID=%s Password=%s\n", registerRequest.ID, strings.Repeat("*", len(registerRequest.Password)))
		agent, err := svc.RegisterAgent(context.Background(), registerRequest)
		if err != nil {
			return err
		}
		fmt.Printf("[agent] REGISTER response: agent_id=%s username=%s status=%s enabled=%v\n", agent.ID, agent.Username, agent.Status, agent.Enabled)
		if !outputJSON {
			fmt.Printf("agent %s registered\n", agentIDVal)
		}
		modelCommand := strings.TrimSpace(*llmCommand)
		if modelCommand == "" {
			modelCommand = "claude"
		}
		agentVerbose := *verbose
		idleDelay := time.Duration(*pollSeconds) * time.Second
		configUpdatedAt := "" // track last received config timestamp
		for {
			if agentVerbose {
				fmt.Printf("[agent] requesting work (project=%d)\n", *projectID)
			}
			response, err := svc.RequestAgentWork(context.Background(), libticket.AgentRequest{
				ID:              agentIDVal,
				Password:        agentPassword,
				ProjectID:       *projectID,
				ConfigUpdatedAt: configUpdatedAt,
			})
			if err != nil {
				return err
			}
			// Process config updates from server
			if len(response.Config) > 0 {
				if agentVerbose {
					fmt.Printf("[agent] received config update\n")
				}
				configUpdatedAt = response.ConfigUpdatedAt
				// Apply config values (command-line flags take precedence)
				if *llmCommand == "" || *llmCommand == envValue("TICKET_AGENT_LLM") {
					if llmVal, ok := response.Config["llm"]; ok && llmVal != "" {
						modelCommand = llmVal
						if agentVerbose {
							fmt.Printf("[agent] config: llm=%s\n", modelCommand)
						}
					}
				}
				if *projectID == 0 {
					if projVal, ok := response.Config["project_id"]; ok && projVal != "" {
						if parsed, err := strconv.ParseInt(projVal, 10, 64); err == nil {
							*projectID = parsed
							if agentVerbose {
								fmt.Printf("[agent] config: project_id=%d\n", *projectID)
							}
						}
					}
				}
				if *pollSeconds == 5 {
					if pollVal, ok := response.Config["poll_seconds"]; ok && pollVal != "" {
						if parsed, err := strconv.Atoi(pollVal); err == nil && parsed >= 1 {
							*pollSeconds = parsed
							idleDelay = time.Duration(*pollSeconds) * time.Second
							if agentVerbose {
								fmt.Printf("[agent] config: poll_seconds=%d\n", *pollSeconds)
							}
						}
					}
				}
				if !*verbose {
					if verboseVal, ok := response.Config["verbose"]; ok {
						agentVerbose = verboseVal == "true" || verboseVal == "1"
						if agentVerbose {
							fmt.Printf("[agent] config: verbose=%v\n", agentVerbose)
						}
					}
				}
			}
			if agentVerbose {
				fmt.Printf("[agent] response status=%s", response.Status)
				if response.Ticket != nil {
					fmt.Printf(" ticket=%s type=%s title=%q", response.Ticket.ID, response.Ticket.Type, response.Ticket.Title)
				}
				fmt.Println()
			}
			if (response.Status != "NEW" && response.Status != "CURRENT") || response.Ticket == nil {
				if agentVerbose {
					fmt.Printf("[agent] no work, sleeping %s\n", idleDelay)
				}
				time.Sleep(idleDelay)
				continue
			}
			ticket := response.Ticket
			if agentVerbose {
				fmt.Printf("[agent] processing %s %q\n", ticketLabel(*ticket), ticket.Title)
			}
			prompt := buildAgentPrompt(response)

			// Start a background heartbeat while the LLM is working.
			heartbeatStop := make(chan struct{})
			go func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						if err := svc.HeartbeatAgent(context.Background(), agentIDVal, agentPassword, "working"); err != nil {
							if agentVerbose {
								fmt.Printf("[agent] heartbeat error: %v\n", err)
							}
						} else if agentVerbose {
							fmt.Printf("[agent] heartbeat sent (working)\n")
						}
					case <-heartbeatStop:
						return
					}
				}
			}()

			result, err := runAgentCommand(modelCommand, prompt, agentVerbose, ticket.ID)
			close(heartbeatStop)
			if err != nil {
				fmt.Printf("failed %s: %v\n", ticketLabel(*ticket), err)
				return fmt.Errorf("agent llm processing failed for ticket %s: %w", ticketLabel(*ticket), err)
			}
			if agentVerbose {
				fmt.Printf("[agent] submitting result for %s (%d bytes)\n", ticketLabel(*ticket), len(result))
			}
			updated, err := svc.AgentUpdateTicket(context.Background(), ticket.ID, libticket.AgentTicketUpdateRequest{
				ID:       agentIDVal,
				Password: agentPassword,
				Result:   strings.TrimSpace(result),
			})
			if err != nil {
				fmt.Printf("failed %s: could not submit result: %v\n", ticketLabel(*ticket), err)
				return err
			}
			if outputJSON {
				if err := printJSON(updated); err != nil {
					return err
				}
			}
			fmt.Printf("completed %s -> %s\n", ticketLabel(*ticket), updated.Status)
		}
	case "request":
		fs := flag.NewFlagSet("agent request", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		reqAgentID := fs.String("agent-id", "", "agent UUID")
		password := fs.String("password", "", "agent password")
		id := fs.String("id", "", "specific ticket id")
		dryRun := fs.Bool("dryrun", false, "simulate assignment only")
		loop := fs.Int("loop", 1, "number of request loops")
		sleepSeconds := fs.Int("sleep", 1, "sleep seconds between loops")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		reqAgentIDVal := strings.TrimSpace(*reqAgentID)
		if reqAgentIDVal == "" {
			reqAgentIDVal = envValue("AGENT_ID")
		}
		agentPassword := strings.TrimSpace(*password)
		if agentPassword == "" {
			agentPassword = envValue("AGENT_PASSWORD")
		}
		resolved, rErr := config.ResolveURL()
		if rErr != nil || resolved.Mode != config.ModeRemote {
			return errors.New("agent request requires remote mode (run tk init to configure)")
		}
		missing := make([]string, 0, 2)
		if reqAgentIDVal == "" {
			missing = append(missing, "AGENT_ID or -agent-id")
		}
		if agentPassword == "" {
			missing = append(missing, "AGENT_PASSWORD or -password")
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing required values: %s", strings.Join(missing, ", "))
		}
		if *loop < 0 {
			return errors.New("loop must be >= 0")
		}
		if *sleepSeconds < 0 {
			return errors.New("sleep must be >= 0")
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		var requestedID *string
		if *id != "" {
			requestedID = id
		}
		for i := 0; *loop == 0 || i < *loop; i++ {
			response, err := svc.RequestAgentWork(context.Background(), libticket.AgentRequest{
				ID:       reqAgentIDVal,
				Password: agentPassword,
				TicketID: requestedID,
				DryRun:   *dryRun,
			})
			if err != nil {
				return err
			}
			if err := printJSON(response); err != nil {
				return err
			}
			if *loop == 0 || i < *loop-1 {
				time.Sleep(time.Duration(*sleepSeconds) * time.Second)
			}
		}
		return nil
	case "reset-password":
		fs := flag.NewFlagSet("agent reset-password", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "agent UUID")
		newPassword := fs.String("password", "", "new password (generated if omitted)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		dbID, err := resolveAgentID(*id)
		if err != nil {
			return err
		}
		pw := strings.TrimSpace(*newPassword)
		if pw == "" {
			generated, err := generatePassword(24)
			if err != nil {
				return err
			}
			pw = generated
		}
		agent, err := svc.UpdateAgent(context.Background(), dbID, libticket.AgentUpdateRequest{Password: &pw})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"agent_id": agent.ID, "password": pw})
		}
		fmt.Printf("agent    : %s\n", agent.ID)
		fmt.Printf("password : %s\n", pw)
		return nil
	case "config-set":
		if len(args) < 4 {
			return errors.New("usage: tk agent config-set -id <agent-uuid> <key> <value>")
		}
		fs := flag.NewFlagSet("agent config-set", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		agentUUID := fs.String("id", "", "agent UUID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		dbID, err := resolveAgentID(*agentUUID)
		if err != nil {
			return err
		}
		if fs.NArg() < 2 {
			return errors.New("usage: tk agent config-set -id <agent-uuid> <key> <value>")
		}
		if err := svc.SetAgentConfig(context.Background(), dbID, fs.Arg(0), fs.Arg(1)); err != nil {
			return err
		}
		fmt.Printf("%s=%s\n", fs.Arg(0), fs.Arg(1))
		return nil
	case "config-ls":
		fs := flag.NewFlagSet("agent config-ls", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		agentUUID := fs.String("id", "", "agent UUID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		dbID, err := resolveAgentID(*agentUUID)
		if err != nil {
			return err
		}
		entries, err := svc.ListAgentConfig(context.Background(), dbID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(entries)
		}
		if len(entries) == 0 {
			fmt.Println("(no config)")
			return nil
		}
		for _, e := range entries {
			fmt.Printf("%s=%s\n", e.Key, e.Value)
		}
		return nil
	case "config-rm":
		fs := flag.NewFlagSet("agent config-rm", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		agentUUID := fs.String("id", "", "agent UUID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		dbID, err := resolveAgentID(*agentUUID)
		if err != nil {
			return err
		}
		if fs.NArg() < 1 {
			return errors.New("usage: tk agent config-rm -id <agent-uuid> <key>")
		}
		if err := svc.DeleteAgentConfig(context.Background(), dbID, fs.Arg(0)); err != nil {
			return err
		}
		fmt.Printf("deleted %s\n", fs.Arg(0))
		return nil
	default:
		return fmt.Errorf("unknown agent command %q; see: ticket agent help", args[0])
	}
}

func buildAgentPrompt(resp libticket.AgentWorkResponse) string {
	var b strings.Builder
	ticket := resp.Ticket
	if ticket == nil {
		return ""
	}

	if resp.Role != nil {
		b.WriteString(fmt.Sprintf("Role: %s\n", resp.Role.Title))
		if resp.Role.Description != "" {
			b.WriteString(fmt.Sprintf("Description: %s\n", strings.TrimSpace(resp.Role.Description)))
		}
		if resp.Role.AcceptanceCriteria != "" {
			b.WriteString(fmt.Sprintf("AcceptanceCriteria: %s\n", strings.TrimSpace(resp.Role.AcceptanceCriteria)))
		}
		b.WriteString("\n")
	}

	if resp.Project != nil {
		b.WriteString(fmt.Sprintf("Project: %s — %s\n", resp.Project.Prefix, resp.Project.Title))
		if resp.Project.GitRepository != "" {
			b.WriteString(fmt.Sprintf("Repository: %s\n", resp.Project.GitRepository))
			if resp.Project.GitBranch != "" {
				b.WriteString(fmt.Sprintf("Branch: %s\n", resp.Project.GitBranch))
			}
		}
		b.WriteString("\n")
	}

	if resp.Sdlc != nil {
		b.WriteString(fmt.Sprintf("Sdlc: %s\n", resp.Sdlc.Name))
		b.WriteString("Stages:")
		for _, stage := range resp.Sdlc.Stages {
			marker := " "
			if ticket.SdlcStageID != nil && stage.ID == *ticket.SdlcStageID {
				marker = ">"
			}
			b.WriteString(fmt.Sprintf(" %s %s", marker, stage.StageName))
		}
		b.WriteString("\n\n")
	}

	b.WriteString(fmt.Sprintf("Ticket: %s\n", ticketLabel(*ticket)))
	b.WriteString(fmt.Sprintf("Title: %s\n", strings.TrimSpace(ticket.Title)))
	if strings.TrimSpace(ticket.Description) != "" {
		b.WriteString("Description:\n")
		b.WriteString(strings.TrimSpace(ticket.Description))
		b.WriteString("\n")
	}
	if strings.TrimSpace(ticket.AcceptanceCriteria) != "" {
		b.WriteString("Acceptance Criteria:\n")
		b.WriteString(strings.TrimSpace(ticket.AcceptanceCriteria))
		b.WriteString("\n")
	}

	if len(resp.Parents) > 0 {
		b.WriteString("\nParents:\n")
		for _, p := range resp.Parents {
			b.WriteString(fmt.Sprintf("  %s [%s] %s\n", p.ID, p.Type, p.Title))
		}
	}

	return strings.TrimSpace(b.String())
}

func printAgentTable(statuses []store.AgentStatus) {
	if len(statuses) == 0 {
		fmt.Println("no agents")
		return
	}
	sort.SliceStable(statuses, func(i, j int) bool {
		return strings.ToLower(statuses[i].Agent.Username) < strings.ToLower(statuses[j].Agent.Username)
	})
	rows := make([]string, 0, len(statuses))
	for _, s := range statuses {
		lastSeen := strings.TrimSpace(s.Agent.LastSeen)
		if lastSeen == "" {
			lastSeen = "-"
		}
		ticket := "-"
		if s.TicketKey != nil {
			ticket = *s.TicketKey
		}
		proj := "-"
		if s.ProjectName != "" {
			proj = s.ProjectName
		}
		wf := "-"
		if s.SdlcName != "" {
			wf = s.SdlcName
		}
		role := "-"
		if s.RoleTitle != "" {
			role = s.RoleTitle
		}
		rows = append(rows, fmt.Sprintf("%s\t%t\t%s\t%s\t%s\t%s\t%s\t%s", s.Agent.ID, s.Agent.Enabled, s.Agent.Status, ticket, proj, wf, role, lastSeen))
	}
	printBoxTable("UUID\tENABLED\tSTATUS\tTICKET\tPROJECT\tSDLC\tROLE\tLAST_SEEN", rows)
}

func printUserTable(users []store.User) {
	if len(users) == 0 {
		fmt.Println("no users")
		return
	}
	rows := make([]string, 0, len(users))
	for _, user := range users {
		created := user.CreatedAt
		if len(created) > 10 {
			created = created[:10]
		}
		rows = append(rows, fmt.Sprintf("%s\t%s\t%t\t%s", user.Username, user.Role, user.Enabled, created))
	}
	printBoxTable("USERNAME\tROLE\tENABLED\tCREATED", rows)
}

// prefixWriter prepends a prefix to each line written.
type prefixWriter struct {
	w      io.Writer
	prefix string
	bol    bool // at beginning of line
}

func (pw *prefixWriter) Write(p []byte) (int, error) {
	total := len(p)
	for len(p) > 0 {
		if pw.bol || !pw.bol && total == len(p) {
			// First write or start of new line: emit prefix.
			if _, err := fmt.Fprint(pw.w, pw.prefix); err != nil {
				return total - len(p), err
			}
			pw.bol = false
		}
		idx := bytes.IndexByte(p, '\n')
		if idx < 0 {
			_, err := pw.w.Write(p)
			return total, err
		}
		if _, err := pw.w.Write(p[:idx+1]); err != nil {
			return total - len(p), err
		}
		p = p[idx+1:]
		if len(p) > 0 {
			pw.bol = true
		}
	}
	return total, nil
}

// prefixReader wraps a reader and echoes what's read with a prefix.
type prefixReader struct {
	r      io.Reader
	prefix string
	w      io.Writer
}

func (pr *prefixReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		fmt.Fprintf(pr.w, "%s%s\n", pr.prefix, strings.TrimRight(string(p[:n]), "\n"))
	}
	return n, err
}

func defaultRunTicketAgentCommand(agent, prompt string, stream bool, ticketKey string) (string, error) {
	if agent == "" {
		return "", errors.New("agent is required")
	}

	// Write the prompt to a file so large prompts don't hit arg-length
	// limits and to work around CLI escaping issues.
	promptFile := ""
	if ticketKey != "" {
		promptFile = fmt.Sprintf("prompt_%s.md", ticketKey)
	} else {
		promptFile = "prompt_agent.md"
	}
	prompt += "\n"
	if err := os.WriteFile(promptFile, []byte(prompt), 0o600); err != nil {
		return "", fmt.Errorf("write prompt file: %w", err)
	}
	defer os.Remove(promptFile)

	// Build the LLM command. Known agents use sh -c for piping; the default
	// case avoids sh -c to prevent command injection via the --llm flag.
	var cmd *exec.Cmd
	var llmCmd string // for display only
	switch agent {
	case "claude":
		llmCmd = fmt.Sprintf("cat %s | claude -p --model claude-sonnet-4-5", promptFile)
		cmd = exec.Command("sh", "-c", llmCmd) //nolint:gosec // hardcoded trusted command
	case "codex":
		llmCmd = fmt.Sprintf("codex exec < %s", promptFile)
		cmd = exec.Command("sh", "-c", llmCmd) //nolint:gosec // hardcoded trusted command
	default:
		if err := validateLLMBinary(agent); err != nil {
			return "", err
		}
		llmCmd = fmt.Sprintf("%s -p < %s", agent, promptFile)
		f, err := os.Open(promptFile)
		if err != nil {
			return "", fmt.Errorf("open prompt file: %w", err)
		}
		defer f.Close()
		cmd = exec.Command(agent, "-p") // #nosec G204 -- agent comes from --llm flag; no shell interpretation
		cmd.Stdin = f
	}

	if stream {
		fmt.Printf("> %s\n\n", llmCmd)
	}

	// Use StdoutPipe + goroutine to stream output byte-by-byte as it
	// arrives, avoiding any block-buffering in Go's cmd.Run path.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if stream {
		cmd.Stderr = &prefixWriter{w: os.Stderr, prefix: "< "}
	} else {
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	small := make([]byte, 256)
	for {
		n, readErr := stdout.Read(small)
		if n > 0 {
			chunk := small[:n]
			buf.Write(chunk)
			if stream {
				// Write with prefix per line
				for _, line := range bytes.SplitAfter(chunk, []byte("\n")) {
					if len(line) > 0 {
						fmt.Fprintf(os.Stdout, "< %s", string(line))
					}
				}
			} else {
				if _, err := os.Stdout.Write(chunk); err != nil {
					return "", err
				}
			}
		}
		if readErr != nil {
			break
		}
	}
	if stream {
		fmt.Println()
	}
	if err := cmd.Wait(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var llmBinaryNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func validateLLMBinary(agent string) error {
	agent = strings.TrimSpace(agent)
	if agent == "" {
		return errors.New("llm binary is required")
	}
	if !llmBinaryNamePattern.MatchString(agent) || strings.Contains(agent, "..") || strings.Contains(agent, "/") || strings.Contains(agent, "\\") {
		return fmt.Errorf("invalid llm binary %q: only [a-zA-Z0-9._-] names are allowed", agent)
	}

	allowed := allowedLLMBinaries()
	if _, ok := allowed[agent]; ok {
		return nil
	}

	known := make([]string, 0, len(allowed))
	for name := range allowed {
		known = append(known, name)
	}
	sort.Strings(known)
	return fmt.Errorf("llm binary %q is not in the allow-list (%s)", agent, strings.Join(known, ", "))
}

func allowedLLMBinaries() map[string]struct{} {
	allowed := map[string]struct{}{
		"claude": {},
		"codex":  {},
	}
	for _, part := range strings.Split(envValue("TICKET_AGENT_ALLOWED_LLM_BINARIES"), ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		allowed[name] = struct{}{}
	}
	return allowed
}
