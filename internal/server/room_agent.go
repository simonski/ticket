package server

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/simonski/ticket/internal/store"
)

// roomAgentResponder generates an agent's reply to a room prompt. It is a
// package var so tests can inject a deterministic stub; the default runs the
// configured chat/agent command one-shot (TK-131).
type roomAgentResponder func(ctx context.Context, agentName, prompt string, history []store.RoomMessage) (string, error)

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
		if postAgentReply(ctx, db, room, agent, msg.Body, hub) {
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
				if postAgentReply(ctx, db, room, agent, msg.Body, hub) {
					posted++
				}
			}
		}
	}
	return posted
}

// postAgentReply generates and posts a single agent reply into the room.
func postAgentReply(ctx context.Context, db *sql.DB, room store.Room, agent store.User, trigger string, hub *liveHub) bool {
	history, _ := store.ListRoomMessages(ctx, db, room.ID, 20, 0)
	reply, rerr := roomAgentReply(ctx, agent.Username, trigger, history)
	if rerr != nil {
		log.Printf("server: room agent %s reply failed: %v", agent.Username, rerr)
		return false
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return false
	}
	out, perr := store.PostRoomMessage(ctx, db, store.RoomMessage{
		RoomID:   room.ID,
		SenderID: agent.ID,
		Kind:     "text",
		Body:     reply,
		Attrs:    store.Attrs{"agent": true},
	})
	if perr != nil {
		return false
	}
	if hub != nil {
		hub.broadcastRoomMessage(room.ID, out)
	}
	return true
}

// defaultRoomAgentReply runs the configured chat/agent command one-shot, feeding
// it a prompt built from the room transcript, and returns its stdout. Requires
// the agent runtime to be configured; otherwise it returns an error and no reply
// is posted.
func defaultRoomAgentReply(_ context.Context, agentName, prompt string, history []store.RoomMessage) (string, error) {
	commandArgs := resolveChatCommandArgs()
	if len(commandArgs) == 0 {
		return "", errors.New("chat agent command is empty")
	}
	full := buildRoomAgentPrompt(agentName, prompt, history)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, commandArgs[0], commandArgs[1:]...) // #nosec G204 -- commandArgs resolved from trusted server configuration
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	if _, werr := stdin.Write([]byte(full + "\n")); werr != nil {
		_ = stdin.Close()
		return "", werr
	}
	if cerr := stdin.Close(); cerr != nil {
		return "", cerr
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
