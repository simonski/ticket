package server

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/simonski/ticket/internal/store"
)

// roomAgentResponder generates an agent's reply to a room prompt. It is a
// package var so tests can inject a deterministic stub; the default runs the
// configured chat/agent command one-shot (TK-131).
type roomAgentResponder func(ctx context.Context, cfg store.AgentModelConfig, cmdArgs []string, agentName, prompt string, history []store.RoomMessage) (string, error)

// resolveAgentChatCommand resolves the CLI chat command: TICKET_CHAT_CMD env
// overrides the persisted chat_command setting, which overrides the default
// (codex exec). The args may contain a {prompt} placeholder (TK-157).
func resolveAgentChatCommand(ctx context.Context, db *sql.DB) []string {
	if env := strings.TrimSpace(os.Getenv("TICKET_CHAT_CMD")); env != "" {
		return chatCommandArgsFrom(env)
	}
	return chatCommandArgsFrom(store.GetChatCommand(ctx, db))
}

var roomAgentReply roomAgentResponder = defaultRoomAgentReply

// replyAsAgents posts a reply for every agent member that was @mentioned in msg.
// It is synchronous (callers run it in a goroutine) and returns the number of
// replies posted, which keeps it unit-testable with a stubbed responder.
func replyAsAgents(ctx context.Context, db *sql.DB, room store.Room, msg store.RoomMessage, hub *liveHub) int {
	posted := 0
	for _, name := range parseMentions(msg.Body) {
		agent, err := store.GetUserByUsername(ctx, db, name)
		if err != nil || agent.UserType != "agent" {
			continue
		}
		member, merr := store.IsRoomMember(ctx, db, room.ID, agent.ID)
		if merr != nil || !member {
			continue
		}
		if postAgentReply(ctx, db, room, agent, msg.SenderID, msg.Body, hub) {
			posted++
		}
	}
	// In a DM (e.g. a user's personal agent), the agent answers every message from
	// a human — no @mention required (TK-142).
	if posted == 0 && strings.HasPrefix(room.Slug, "dm-") {
		if sender, serr := store.GetUserByID(ctx, db, msg.SenderID); serr == nil && sender.UserType != "agent" {
			members, _ := store.ListRoomMembers(ctx, db, room.ID)
			for _, m := range members {
				if m.MemberID == msg.SenderID {
					continue
				}
				agent, gerr := store.GetUserByID(ctx, db, m.MemberID)
				if gerr != nil || agent.UserType != "agent" {
					continue
				}
				if postAgentReply(ctx, db, room, agent, msg.SenderID, msg.Body, hub) {
					posted++
				}
			}
		}
	}
	if posted == 0 {
		log.Printf("server: no agent reply in room %d (slug=%q): no agent @mentioned and not a personal-agent DM with an agent member", room.ID, room.Slug)
	}
	return posted
}

// postAgentReply generates and posts a single agent reply into the room. The
// model is resolved for the requesting user in the room's project (TK-149).
func postAgentReply(ctx context.Context, db *sql.DB, room store.Room, agent store.User, requesterID, trigger string, hub *liveHub) bool {
	cfg, _ := store.ResolveAgentModelConfig(ctx, db, requesterID, room.ProjectID)
	cmdArgs := resolveAgentChatCommand(ctx, db)
	history, _ := store.ListRoomMessages(ctx, db, room.ID, 20, 0)
	reply, rerr := roomAgentReply(ctx, cfg, cmdArgs, agent.Username, trigger, history)
	if rerr != nil {
		log.Printf("server: room agent %s reply failed: %v", agent.Username, rerr)
		// Surface the failure in the room so the user isn't left wondering why
		// nothing happened; the detailed error stays in the server log.
		postAgentNotice(ctx, db, room, agent,
			"couldn't reply — the agent command failed. Check the server's chat command / model (TICKET_CHAT_CMD) and the server logs.", hub)
		return false
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		log.Printf("server: room agent %s returned an empty reply", agent.Username)
		postAgentNotice(ctx, db, room, agent, "returned an empty reply (the model produced no output).", hub)
		return false
	}
	agentAttrs := store.Attrs{"agent": true}
	if strings.TrimSpace(cfg.Model) != "" {
		agentAttrs["model"] = cfg.Model
	}
	out, perr := store.PostRoomMessage(ctx, db, store.RoomMessage{
		RoomID:   room.ID,
		SenderID: agent.ID,
		Kind:     "text",
		Body:     reply,
		Attrs:    agentAttrs,
	})
	if perr != nil {
		return false
	}
	if hub != nil {
		hub.broadcastRoomMessage(room.ID, out)
	}
	return true
}

// postAgentNotice posts a muted agent_event line (e.g. a reply failure) into the
// room from the agent, so failures are visible rather than silent.
func postAgentNotice(ctx context.Context, db *sql.DB, room store.Room, agent store.User, text string, hub *liveHub) {
	out, err := store.PostRoomMessage(ctx, db, store.RoomMessage{
		RoomID:   room.ID,
		SenderID: agent.ID,
		Kind:     "agent_event",
		Body:     "⚠️ " + text,
		Attrs:    store.Attrs{"agent": true},
	})
	if err != nil {
		return
	}
	if hub != nil {
		hub.broadcastRoomMessage(room.ID, out)
	}
}

// defaultRoomAgentReply runs the configured chat/agent command one-shot, feeding
// it a prompt built from the room transcript, and returns its stdout. Requires
// the agent runtime to be configured; otherwise it returns an error and no reply
// is posted.
func defaultRoomAgentReply(ctx context.Context, cfg store.AgentModelConfig, cmdArgs []string, agentName, prompt string, history []store.RoomMessage) (string, error) {
	full := buildRoomAgentPrompt(agentName, prompt, history)
	// When a provider API is configured (resolved per user/project/system), call
	// it directly; otherwise fall back to the configured CLI command (TK-149).
	if agentModelCanCallAPI(cfg) {
		log.Printf("server: agent %s replying via %s model %q (api)", agentName, cfg.Provider, cfg.Model)
		return callAgentModelAPI(ctx, cfg, full)
	}
	if len(cmdArgs) == 0 {
		cmdArgs = resolveChatCommandArgs()
	}
	if len(cmdArgs) == 0 {
		return "", errors.New("chat agent command is empty")
	}
	// {prompt} placeholder: substitute the prompt as an argument (e.g. claude -p
	// {prompt}); without it, pipe the prompt to stdin (e.g. codex exec).
	args := make([]string, len(cmdArgs))
	usePlaceholder := false
	for i, a := range cmdArgs {
		if strings.Contains(a, "{prompt}") {
			usePlaceholder = true
			args[i] = strings.ReplaceAll(a, "{prompt}", full)
		} else {
			args[i] = a
		}
	}
	if strings.TrimSpace(cfg.Provider) != "" {
		log.Printf("server: agent %s — no API key/URL resolved for provider %q, falling back to CLI %v (set the provider/system API key to use the API)", agentName, cfg.Provider, cmdArgs)
	} else {
		log.Printf("server: agent %s replying via CLI %v", agentName, cmdArgs)
	}
	cctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, args[0], args[1:]...) // #nosec G204 -- args resolved from trusted server configuration
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if !usePlaceholder {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return "", err
		}
		if serr := cmd.Start(); serr != nil {
			return "", serr
		}
		if _, werr := stdin.Write([]byte(full + "\n")); werr != nil {
			_ = stdin.Close()
			return "", werr
		}
		if cerr := stdin.Close(); cerr != nil {
			return "", cerr
		}
	} else if serr := cmd.Start(); serr != nil {
		return "", serr
	}
	if werr := cmd.Wait(); werr != nil {
		return "", errors.Join(werr, errors.New(strings.TrimSpace(stderr.String())))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func buildRoomAgentPrompt(agentName, latest string, history []store.RoomMessage) string {
	var b strings.Builder
	b.WriteString("You are the agent @" + agentName + " participating in a team chat room.\n")
	if len(history) > 0 {
		b.WriteString("Recent conversation:\n")
		for _, m := range history {
			b.WriteString(m.SenderID + ": " + m.Body + "\n")
		}
	}
	b.WriteString("\nRespond concisely to the latest message addressed to you: " + latest)
	return b.String()
}
