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
		printID := fs.Bool("printid", false, "print only the created agent id")
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
		if printCreatedID(agent.ID, *printID) {
			return nil
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
			return errors.New("agent run requires a configured server remote")
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
		svc, err = resolveService(cfg)
		if err != nil {
			return err
		}
		registerRequest := libticket.AgentRegisterRequest{
			ID:       agentIDVal,
			Password: agentPassword,
		}
		agentVerbose := *verbose
		alog := func(format string, args ...any) {
			if agentVerbose {
				fmt.Printf("[agent] "+format+"\n", args...)
			}
		}
		alog("POST /api/agents/register  id=%s", registerRequest.ID)
		agent, err := svc.RegisterAgent(context.Background(), registerRequest)
		if err != nil {
			return err
		}
		alog("  → 200 OK  username=%s role=%s status=%s enabled=%v", agent.Username, agent.AgentRole, agent.Status, agent.Enabled)
		if !outputJSON {
			fmt.Printf("agent %s (%s) registered\n", agentIDVal, agent.AgentRole)
		}
		agentRole := agent.AgentRole
		modelCommand := strings.TrimSpace(*llmCommand)
		if modelCommand == "" {
			modelCommand = "claude"
		}
		idleDelay := time.Duration(*pollSeconds) * time.Second
		configUpdatedAt := "" // track last received config timestamp
		for {
			alog("POST /api/agents/request  project=%d", *projectID)
			response, err := svc.RequestAgentWork(context.Background(), libticket.AgentRequest{
				ID:              agentIDVal,
				Password:        agentPassword,
				ProjectID:       *projectID,
				ConfigUpdatedAt: configUpdatedAt,
			})
			if err != nil {
				return err
			}
			alog("  → 200 OK  status=%s", response.Status)
			// Process config updates from server
			if len(response.Config) > 0 {
				alog("  config update received")
				configUpdatedAt = response.ConfigUpdatedAt
				// Apply config values (command-line flags take precedence)
				if *llmCommand == "" || *llmCommand == envValue("TICKET_AGENT_LLM") {
					if llmVal, ok := response.Config["llm"]; ok && llmVal != "" {
						modelCommand = llmVal
						alog("  config: llm=%s", modelCommand)
					}
				}
				if *projectID == 0 {
					if projVal, ok := response.Config["project_id"]; ok && projVal != "" {
						if parsed, parseErr := strconv.ParseInt(projVal, 10, 64); parseErr == nil {
							*projectID = parsed
							alog("  config: project_id=%d", *projectID)
						}
					}
				}
				if *pollSeconds == 5 {
					if pollVal, ok := response.Config["poll_seconds"]; ok && pollVal != "" {
						if parsed, parseErr := strconv.Atoi(pollVal); parseErr == nil && parsed >= 1 {
							*pollSeconds = parsed
							idleDelay = time.Duration(*pollSeconds) * time.Second
							alog("  config: poll_seconds=%d", *pollSeconds)
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
			if response.Ticket != nil {
				alog("  ticket=%s type=%s title=%q", response.Ticket.ID, response.Ticket.Type, response.Ticket.Title)
			}
			if len(response.Reasons) > 0 {
				for _, r := range response.Reasons {
					alog("  reason: %s", r)
				}
			}
			if (response.Status != "NEW" && response.Status != "CURRENT") || response.Ticket == nil {
				alog("  no work available — sleeping %s", idleDelay)
				time.Sleep(idleDelay)
				continue
			}
			ticket := response.Ticket
			alog("processing %s %q (role=%s)", ticketLabel(*ticket), ticket.Title, agentRole)

			// A refiner agent takes one turn of the idea→refinement dialogue. The
			// orchestrator only assigns refinement (a draft backlog story) to refiner
			// agents, so the agent's role is the signal — refinement is an in-place
			// activity, not tied to any literal "refine" stage.
			isRefine := false
			for _, role := range store.SplitAgentRoles(agentRole) {
				if strings.EqualFold(role, "refiner") {
					isRefine = true
					break
				}
			}
			var prompt string
			if isRefine {
				comments, _ := svc.ListComments(context.Background(), ticket.ID)
				prompt = buildRefinementPrompt(response, comments)
			} else {
				prompt = buildAgentPrompt(response)
			}

			// Start a background heartbeat while the LLM is working.
			heartbeatStop := make(chan struct{})
			go func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						alog("POST /api/agents/heartbeat  status=working")
						if heartbeatErr := svc.HeartbeatAgent(context.Background(), agentIDVal, agentPassword, "working"); heartbeatErr != nil {
							alog("  heartbeat error: %v", heartbeatErr)
						} else {
							alog("  → 200 OK")
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
			alog("submitting result for %s (%d bytes)", ticketLabel(*ticket), len(result))

			if isRefine {
				// Parse the LLM output into a refinement turn: a chat reply plus an
				// optional proposal (a single ready story, or a breakdown into stories).
				msg, kind, desc, ac, stories := parseRefinementOutput(result)
				refineReq := libticket.AgentRefineRequest{
					ID: agentIDVal, Password: agentPassword,
					Message: msg, ProposalKind: kind, Description: desc, AcceptanceCriteria: ac,
					Stories: stories,
				}
				if _, refErr := svc.AgentRefineTicket(context.Background(), ticket.ID, refineReq); refErr != nil {
					fmt.Printf("dropping refinement of %s: %v\n", ticketLabel(*ticket), refErr)
					alog("  refine turn rejected — dropping and re-polling")
					time.Sleep(idleDelay)
					continue
				}
				fmt.Printf("refinement turn on %s — %s\n", ticketLabel(*ticket), kind)
				continue
			}

			updated, err := svc.AgentUpdateTicket(context.Background(), ticket.ID, libticket.AgentTicketUpdateRequest{
				ID:       agentIDVal,
				Password: agentPassword,
				Result:   strings.TrimSpace(result),
			})
			if err != nil {
				// The orchestrator may have abandoned this ticket (heartbeat timeout)
				// or it was reassigned while the agent worked. The server rejects the
				// stale result; the agent drops the work and keeps polling rather than
				// crashing.
				fmt.Printf("dropping %s: result rejected (likely abandoned or reassigned): %v\n", ticketLabel(*ticket), err)
				alog("  result rejected — dropping work and re-polling")
				time.Sleep(idleDelay)
				continue
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
		projectID := fs.Int64("project-id", 0, "project id override")
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
		svc, err = resolveService(cfg)
		if err != nil {
			return err
		}
		var requestedID *string
		if *id != "" {
			requestedID = id
		}
		for i := 0; *loop == 0 || i < *loop; i++ {
			response, err := svc.RequestAgentWork(context.Background(), libticket.AgentRequest{
				ID:        reqAgentIDVal,
				Password:  agentPassword,
				ProjectID: *projectID,
				TicketID:  requestedID,
				DryRun:    *dryRun,
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
			generated, genErr := generatePassword(24)
			if genErr != nil {
				return genErr
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

// buildRefinementPrompt builds one turn of the idea→refinement dialogue: the idea,
// the conversation so far, and instructions for how to respond (ask questions, or
// propose a single ready story, or propose a breakdown into stories).
func buildRefinementPrompt(resp libticket.AgentWorkResponse, comments []store.Comment) string {
	var b strings.Builder
	b.WriteString("You are a product manager refining a backlog idea with a human, turn by turn.\n")
	b.WriteString("Read the idea and the conversation so far, then take ONE turn.\n\n")
	b.WriteString("Rules for your reply:\n")
	b.WriteString("- If anything is ambiguous or missing, ask concise clarifying questions (plain text).\n")
	b.WriteString("- When the requirement is clear AND small enough for a single story, end your reply with:\n")
	b.WriteString("    PROPOSE_READY\n")
	b.WriteString("    DESCRIPTION: <one-paragraph refined description>\n")
	b.WriteString("    ACCEPTANCE_CRITERIA: <testable criteria, semicolon-separated>\n")
	b.WriteString("- When the idea is too big and should be split, end your reply with:\n")
	b.WriteString("    PROPOSE_BREAKDOWN\n")
	b.WriteString("    STORY: <title> | <one-line description>\n")
	b.WriteString("    STORY: <title> | <one-line description>\n")
	b.WriteString("- Otherwise just ask your questions and stop (no marker).\n\n")
	if resp.Project != nil {
		b.WriteString(fmt.Sprintf("Project: %s — %s\n", resp.Project.Prefix, resp.Project.Title))
	}
	if resp.Ticket != nil {
		t := resp.Ticket
		b.WriteString(fmt.Sprintf("Idea: %s — %s\n", t.ID, strings.TrimSpace(t.Title)))
		if strings.TrimSpace(t.Description) != "" {
			b.WriteString("Description:\n" + strings.TrimSpace(t.Description) + "\n")
		}
		if strings.TrimSpace(t.AcceptanceCriteria) != "" {
			b.WriteString("Acceptance Criteria:\n" + strings.TrimSpace(t.AcceptanceCriteria) + "\n")
		}
	}
	if len(comments) > 0 {
		b.WriteString("\nConversation so far (oldest first):\n")
		for _, c := range comments {
			text := strings.TrimSpace(c.Text)
			if text == "" {
				text = strings.TrimSpace(c.Comment)
			}
			b.WriteString(fmt.Sprintf("[%s] %s\n", c.Author, text))
		}
	}
	b.WriteString("\nYour turn:")
	return strings.TrimSpace(b.String())
}

// parseRefinementOutput interprets the refiner LLM output into a refinement turn:
// the chat message plus a proposal (question / ready / breakdown).
func parseRefinementOutput(out string) (message, kind, description, acceptanceCriteria string, stories []libticket.AgentRefineStory) {
	text := strings.TrimSpace(out)
	switch {
	case strings.Contains(text, "PROPOSE_READY"):
		kind = "ready"
		idx := strings.Index(text, "PROPOSE_READY")
		message = strings.TrimSpace(text[:idx])
		body := text[idx+len("PROPOSE_READY"):]
		description = extractField(body, "DESCRIPTION:")
		acceptanceCriteria = extractField(body, "ACCEPTANCE_CRITERIA:")
		if message == "" {
			message = "Proposed a refined, ready story."
		}
	case strings.Contains(text, "PROPOSE_BREAKDOWN"):
		kind = "breakdown"
		idx := strings.Index(text, "PROPOSE_BREAKDOWN")
		message = strings.TrimSpace(text[:idx])
		for _, line := range strings.Split(text[idx:], "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "STORY:") {
				continue
			}
			rest := strings.TrimSpace(strings.TrimPrefix(line, "STORY:"))
			title, desc := rest, ""
			if pipe := strings.Index(rest, "|"); pipe >= 0 {
				title = strings.TrimSpace(rest[:pipe])
				desc = strings.TrimSpace(rest[pipe+1:])
			}
			if title != "" {
				stories = append(stories, libticket.AgentRefineStory{Title: title, Description: desc})
			}
		}
		if message == "" {
			message = "Proposed breaking this idea into stories."
		}
		if len(stories) == 0 {
			// No parseable stories — fall back to a question turn.
			kind = "question"
			message = text
		}
	default:
		kind = "question"
		message = text
	}
	return message, kind, description, acceptanceCriteria, stories
}

// extractField pulls the text following a "LABEL:" marker up to the next blank line
// or recognised marker.
func extractField(body, label string) string {
	idx := strings.Index(body, label)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(label):]
	var out []string
	for _, line := range strings.Split(rest, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(out) > 0 && (trimmed == "" || strings.HasPrefix(trimmed, "DESCRIPTION:") ||
			strings.HasPrefix(trimmed, "ACCEPTANCE_CRITERIA:") || strings.HasPrefix(trimmed, "STORY:") ||
			strings.HasPrefix(trimmed, "PROPOSE_")) {
			break
		}
		out = append(out, trimmed)
	}
	return strings.TrimSpace(strings.Join(out, " "))
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
		}
		b.WriteString("\n")
	}

	if resp.Workflow != nil {
		b.WriteString(fmt.Sprintf("Workflow: %s\n", resp.Workflow.Name))
		b.WriteString("Stages:")
		for _, stage := range resp.Workflow.Stages {
			marker := " "
			if ticket.WorkflowStageID != nil && stage.ID == *ticket.WorkflowStageID {
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
		if s.WorkflowName != "" {
			wf = s.WorkflowName
		}
		role := "-"
		if s.RoleTitle != "" {
			role = s.RoleTitle
		}
		rows = append(rows, fmt.Sprintf("%s\t%t\t%s\t%s\t%s\t%s\t%s\t%s", s.Agent.ID, s.Agent.Enabled, s.Agent.Status, ticket, proj, wf, role, lastSeen))
	}
	printBoxTable("UUID\tENABLED\tSTATUS\tTICKET\tPROJECT\tWorkflow\tROLE\tLAST_SEEN", rows)
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
		cmd = exec.Command("sh", "-c", llmCmd) // #nosec G204 -- hardcoded trusted command for known agent preset
	case "codex":
		llmCmd = fmt.Sprintf("codex exec < %s", promptFile)
		cmd = exec.Command("sh", "-c", llmCmd) // #nosec G204 -- hardcoded trusted command for known agent preset
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
