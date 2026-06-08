package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/simonski/ticket/internal/store"
)

// Streaming refinement (Phase 6 — near-real-time). A human opens a WebSocket on a
// refine-stage ticket and chats with a refiner LLM whose output is streamed token
// by token. Messages persist to the ticket comment thread (the same transcript the
// orchestrator-driven batch refiner uses), so the two paths interoperate. Idle
// sessions are reaped. While a live session is active for a ticket the orchestrator
// skips it (see runOrchestrator), so the two refiners never both reply.

type refinementClient struct {
	conn net.Conn
	send chan []byte
	done chan struct{}
	once sync.Once
}

func (c *refinementClient) close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

type refinementSession struct {
	mu           sync.Mutex
	clients      map[*refinementClient]struct{}
	lastActivity time.Time
	busy         bool
}

type refinementHub struct {
	mu       sync.Mutex
	sessions map[string]*refinementSession
}

var sharedRefinementHub = &refinementHub{sessions: map[string]*refinementSession{}}

func (h *refinementHub) get(ticketID string, create bool) *refinementSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[ticketID]
	if s == nil && create {
		s = &refinementSession{clients: map[*refinementClient]struct{}{}, lastActivity: time.Now().UTC()}
		h.sessions[ticketID] = s
	}
	return s
}

func (h *refinementHub) addClient(ticketID string, c *refinementClient) {
	s := h.get(ticketID, true)
	s.mu.Lock()
	s.clients[c] = struct{}{}
	s.lastActivity = time.Now().UTC()
	s.mu.Unlock()
}

func (h *refinementHub) removeClient(ticketID string, c *refinementClient) {
	s := h.get(ticketID, false)
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.clients, c)
	empty := len(s.clients) == 0
	s.mu.Unlock()
	c.close()
	if empty {
		h.mu.Lock()
		if cur := h.sessions[ticketID]; cur == s {
			cur.mu.Lock()
			stillEmpty := len(cur.clients) == 0
			cur.mu.Unlock()
			if stillEmpty {
				delete(h.sessions, ticketID)
			}
		}
		h.mu.Unlock()
	}
}

func (h *refinementHub) broadcast(ticketID string, payload []byte) {
	s := h.get(ticketID, false)
	if s == nil {
		return
	}
	s.mu.Lock()
	for c := range s.clients {
		select {
		case c.send <- payload:
		default:
		}
	}
	s.lastActivity = time.Now().UTC()
	s.mu.Unlock()
}

// activeTicketIDs returns the set of tickets with at least one connected live
// session, so the orchestrator can skip them.
func (h *refinementHub) activeTicketIDs() map[string]bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]bool, len(h.sessions))
	for id := range h.sessions {
		out[id] = true
	}
	return out
}

// reapIdle closes sessions whose last activity is older than the idle window.
func (h *refinementHub) reapIdle(idle time.Duration) {
	if idle <= 0 {
		return
	}
	now := time.Now().UTC()
	h.mu.Lock()
	stale := make([]*refinementSession, 0)
	for id, s := range h.sessions {
		s.mu.Lock()
		old := now.Sub(s.lastActivity) > idle
		s.mu.Unlock()
		if old {
			stale = append(stale, s)
			delete(h.sessions, id)
		}
	}
	h.mu.Unlock()
	closeMsg, _ := json.Marshal(map[string]any{"type": "refinement_idle_closed"})
	for _, s := range stale {
		s.mu.Lock()
		for c := range s.clients {
			select {
			case c.send <- closeMsg:
			default:
			}
			c.close()
		}
		s.clients = map[*refinementClient]struct{}{}
		s.mu.Unlock()
	}
}

func sendRefinementJSON(c *refinementClient, v any) {
	payload, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- payload:
	case <-c.done:
	}
}

// websocketServeRefinement upgrades the connection and runs the streaming
// refinement loop for one ticket and one connected human.
func websocketServeRefinement(w http.ResponseWriter, r *http.Request, db *sql.DB, ticketID, userID string, notify func(string, int64, string)) error {
	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		return err
	}
	client := &refinementClient{conn: conn, send: make(chan []byte, 64), done: make(chan struct{})}
	sharedRefinementHub.addClient(ticketID, client)

	go func() {
		defer sharedRefinementHub.removeClient(ticketID, client)
		for {
			select {
			case <-client.done:
				return
			case payload := <-client.send:
				if err := writeWebSocketFrame(client.conn, 0x1, payload); err != nil {
					return
				}
			}
		}
	}()

	connAvail, connCmd, connAdvice := refinerLLMAvailable()
	sendRefinementJSON(client, map[string]any{
		"type":          "refinement_connected",
		"llm_available": connAvail,
		"llm_command":   connCmd,
		"llm_advice":    connAdvice,
	})

	for {
		opcode, payload, err := readWebSocketFrame(client.conn)
		if err != nil {
			sharedRefinementHub.removeClient(ticketID, client)
			return nil
		}
		if opcode == 0x8 { // close
			sharedRefinementHub.removeClient(ticketID, client)
			return nil
		}
		if opcode != 0x1 {
			continue
		}
		var inbound struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(payload, &inbound); err != nil {
			continue
		}
		if inbound.Type != "message" || strings.TrimSpace(inbound.Text) == "" {
			continue
		}
		handleRefinementMessage(db, ticketID, userID, strings.TrimSpace(inbound.Text), notify)
	}
}

// handleRefinementMessage persists the human message, then runs the refiner LLM and
// streams its reply, persisting the result and applying any proposal.
func handleRefinementMessage(db *sql.DB, ticketID, userID, text string, notify func(string, int64, string)) {
	session := sharedRefinementHub.get(ticketID, true)
	session.mu.Lock()
	if session.busy {
		session.mu.Unlock()
		sharedRefinementHub.broadcast(ticketID, mustJSON(map[string]any{"type": "refinement_busy"}))
		return
	}
	session.busy = true
	session.mu.Unlock()
	defer func() {
		session.mu.Lock()
		session.busy = false
		session.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ticket, err := store.GetTicket(ctx, db, ticketID)
	if err != nil {
		sharedRefinementHub.broadcast(ticketID, mustJSON(map[string]any{"type": "refinement_error", "error": err.Error()}))
		return
	}

	// Persist + broadcast the human message.
	if _, err := store.AddComment(ctx, db, ticketID, userID, text); err != nil {
		sharedRefinementHub.broadcast(ticketID, mustJSON(map[string]any{"type": "refinement_error", "error": err.Error()}))
		return
	}
	humanName := userID
	if u, uErr := store.GetUserByID(ctx, db, userID); uErr == nil {
		humanName = u.Username
	}
	sharedRefinementHub.broadcast(ticketID, mustJSON(map[string]any{"type": "message", "author": humanName, "side": "human", "text": text}))
	if notify != nil {
		notify("ticket_updated", ticket.ProjectID, ticketID)
	}

	// Build the prompt from the idea + full thread.
	comments, _ := store.ListComments(ctx, db, ticketID)
	prompt := buildServerRefinementPrompt(ticket, comments)

	// If no refiner LLM is actually wired up, don't pretend one is thinking —
	// tell the human the message was saved but won't get an automated reply, and
	// how to enable it.
	if avail, cmdStr, advice := refinerLLMAvailable(); !avail {
		sharedRefinementHub.broadcast(ticketID, mustJSON(map[string]any{
			"type": "refinement_no_llm", "command": cmdStr, "advice": advice,
		}))
		return
	}

	sharedRefinementHub.broadcast(ticketID, mustJSON(map[string]any{"type": "refinement_thinking"}))
	full, llmErr := streamRefinerLLM(ctx, prompt, func(chunk string) {
		sharedRefinementHub.broadcast(ticketID, mustJSON(map[string]any{"type": "chunk", "text": chunk}))
	})
	if llmErr != nil {
		sharedRefinementHub.broadcast(ticketID, mustJSON(map[string]any{"type": "refinement_error", "error": llmErr.Error()}))
		return
	}

	refinerID, rErr := store.EnsureRefinerUser(ctx, db)
	if rErr != nil {
		sharedRefinementHub.broadcast(ticketID, mustJSON(map[string]any{"type": "refinement_error", "error": rErr.Error()}))
		return
	}
	refinerName := "refiner"
	if u, uErr := store.GetUserByID(ctx, db, refinerID); uErr == nil {
		refinerName = u.Username
	}
	proposal := store.ParseRefinementProposal(full)
	if err := store.ApplyLiveRefinerReply(ctx, db, ticketID, refinerName, refinerID, proposal); err != nil {
		sharedRefinementHub.broadcast(ticketID, mustJSON(map[string]any{"type": "refinement_error", "error": err.Error()}))
		return
	}
	sharedRefinementHub.broadcast(ticketID, mustJSON(map[string]any{
		"type": "message_done", "proposal_kind": proposal.ProposalKind, "author": refinerName,
	}))
	if notify != nil {
		notify("ticket_updated", ticket.ProjectID, ticketID)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// refinerLLMAvailable reports whether a refiner LLM command is configured and its
// executable can be resolved on the server's PATH. When it returns false, the
// caller surfaces the advisory (third return value) to the human so they know the
// refinement panel needs fixing rather than silently waiting on nothing. The
// second return value is the command that was attempted.
func refinerLLMAvailable() (available bool, command, advice string) {
	args := resolveChatCommandArgs()
	cmdStr := strings.TrimSpace(strings.Join(args, " "))
	if len(args) == 0 {
		return false, cmdStr, "No refiner LLM command is configured on the server. Set the TICKET_CHAT_CMD environment variable to an LLM CLI and restart the server to enable AI refinement."
	}
	if _, err := exec.LookPath(args[0]); err != nil {
		return false, cmdStr, fmt.Sprintf("The refiner command %q is not installed on the server (not found on PATH), so your messages are saved but get no AI reply. Install it, or set TICKET_CHAT_CMD to an available LLM CLI, then restart the server.", args[0])
	}
	return true, cmdStr, ""
}

// streamRefinerLLM runs the configured LLM command with the prompt on stdin and
// streams stdout chunks via onChunk, returning the full (sanitised) output.
func streamRefinerLLM(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := resolveChatCommandArgs()
	if len(args) == 0 {
		return "", fmt.Errorf("refiner command is empty")
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) // #nosec G204 -- args from trusted server config
	cmd.Env = append(os.Environ(), "TERM=dumb", "NO_COLOR=1", "CLICOLOR=0")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start refiner command: %w", err)
	}
	go func() {
		_, _ = io.WriteString(stdin, prompt)
		_ = stdin.Close()
	}()
	var b strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := stdout.Read(buf)
		if n > 0 {
			chunk := sanitizeTerminalOutput(string(buf[:n]))
			b.WriteString(chunk)
			if onChunk != nil && chunk != "" {
				onChunk(chunk)
			}
		}
		if readErr != nil {
			break
		}
	}
	waitErr := cmd.Wait()
	out := strings.TrimSpace(b.String())
	if out == "" && waitErr != nil {
		return "", waitErr
	}
	return out, nil
}

func buildServerRefinementPrompt(ticket store.Ticket, comments []store.Comment) string {
	var b strings.Builder
	b.WriteString("You are a product manager refining a backlog idea with a human, turn by turn.\n")
	b.WriteString("Read the idea and the conversation so far, then take ONE turn.\n\n")
	b.WriteString("Rules for your reply:\n")
	b.WriteString("- If anything is ambiguous or missing, ask concise clarifying questions (plain text).\n")
	b.WriteString("- When the requirement is clear AND small enough for a single story, end with:\n")
	b.WriteString("    PROPOSE_READY\n    DESCRIPTION: <refined description>\n    ACCEPTANCE_CRITERIA: <testable criteria>\n")
	b.WriteString("- When the idea is too big and should be split, end with:\n")
	b.WriteString("    PROPOSE_BREAKDOWN\n    STORY: <title> | <one-line description>\n    STORY: <title> | <one-line description>\n")
	b.WriteString("- Otherwise just ask your questions and stop (no marker).\n\n")
	b.WriteString(fmt.Sprintf("Idea: %s — %s\n", ticket.ID, strings.TrimSpace(ticket.Title)))
	if strings.TrimSpace(ticket.Description) != "" {
		b.WriteString("Description:\n" + strings.TrimSpace(ticket.Description) + "\n")
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
