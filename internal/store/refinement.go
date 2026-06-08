package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Refinement (Phase 6 — backlog preparation loop).
//
// The idea→refinement→breakdown dialogue reuses the ticket comment thread as its
// persisted, ticket-scoped medium: the refiner agent and the human exchange
// comments while the ticket is in the refine stage. The orchestrator drives the
// loop by assigning the ticket to a refiner agent when it is the agent's turn,
// and the human ends it with explicit approval. See docs/DESIGN_ORCHESTRATOR.md.

// RefinementTurn describes whose turn it is in a ticket's refinement dialogue.
type RefinementTurn string

const (
	// RefinementTurnAgent means the latest message is from the human (or there are
	// no messages yet) — the refiner agent should respond.
	RefinementTurnAgent RefinementTurn = "agent"
	// RefinementTurnHuman means the latest message is from the agent — the dialogue
	// is waiting for the human to reply or approve.
	RefinementTurnHuman RefinementTurn = "human"
)

// RefinementDialogueTurn reports whose turn it is, based on the author of the most
// recent comment on the ticket. No comments yet → the agent should open the
// dialogue (respond to the requirement in the ticket description).
func RefinementDialogueTurn(ctx context.Context, db *sql.DB, ticketID string) (RefinementTurn, error) {
	var userType string
	err := db.QueryRowContext(ctx, `
		SELECT COALESCE(u.user_type, '')
		FROM comments c
		JOIN users u ON u.user_id = c.user_id
		WHERE c.item_id = ?
		ORDER BY c.id DESC
		LIMIT 1
	`, ticketID).Scan(&userType)
	if errors.Is(err, sql.ErrNoRows) {
		return RefinementTurnAgent, nil
	}
	if err != nil {
		return "", err
	}
	if userType == "agent" {
		return RefinementTurnHuman, nil
	}
	return RefinementTurnAgent, nil
}

// ReleaseRefinementTurn returns a ticket to idle and clears its assignee after the
// refiner has taken its turn, so the dialogue waits for the human. The orchestrator
// will not re-assign it until the human replies (the latest comment becomes theirs).
func ReleaseRefinementTurn(ctx context.Context, db *sql.DB, ticketID string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE tickets
		SET assignee = '', state = 'idle', status = stage || '/idle', updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ? AND state = 'active'
	`, ticketID)
	return err
}

// ErrNotRefinable is returned when a ticket cannot be approved out of refinement.
var ErrNotRefinable = errors.New("ticket is not in a refinable state")

// ErrRefinementNotAssigned is returned when an agent tries to take a refinement
// turn on a ticket it does not currently hold (released or reassigned).
var ErrRefinementNotAssigned = errors.New("ticket is no longer assigned to you (abandoned or reassigned)")

// RefinementStory is a proposed child story produced during breakdown.
type RefinementStory struct {
	Title              string
	Description        string
	AcceptanceCriteria string
}

// RefinementTurnParams describes one turn the refiner agent takes: an optional
// chat message plus an optional proposal (a single ready story or a breakdown).
type RefinementTurnParams struct {
	Message            string
	ProposalKind       string // "" / "question" | "ready" | "breakdown"
	Description        string // refined description (ready)
	AcceptanceCriteria string // refined acceptance criteria (ready)
	Stories            []RefinementStory
}

// ApplyRefinementTurn records one refiner turn against a ticket: it posts the
// agent's message to the comment thread, applies any proposal, and releases the
// ticket back to idle (awaiting the human). It enforces the same ownership guard as
// the execution path — an agent may only act on a ticket still assigned to it and
// active. Shared by the HTTP handler, the HTTP client, and the local service.
func ApplyRefinementTurn(ctx context.Context, db *sql.DB, ticketID, agentUsername, agentID string, p RefinementTurnParams) (Ticket, error) {
	current, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return Ticket{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(current.Assignee), agentUsername) || current.State != StateActive {
		return Ticket{}, ErrRefinementNotAssigned
	}
	if msg := strings.TrimSpace(p.Message); msg != "" {
		if _, cErr := AddComment(ctx, db, ticketID, agentID, msg); cErr != nil {
			return Ticket{}, cErr
		}
	}
	switch p.ProposalKind {
	case "ready":
		desc := current.Description
		if strings.TrimSpace(p.Description) != "" {
			desc = p.Description
		}
		ac := current.AcceptanceCriteria
		if strings.TrimSpace(p.AcceptanceCriteria) != "" {
			ac = p.AcceptanceCriteria
		}
		if _, uErr := UpdateTicket(ctx, db, ticketID, TicketUpdateParams{
			Title: current.Title, Description: desc, AcceptanceCriteria: ac,
			ParentID: current.ParentID, Assignee: "", Stage: current.Stage, State: StateIdle,
			Priority: current.Priority, Order: current.Order,
			ActorUsername: agentUsername, ActorRole: "admin",
		}); uErr != nil {
			return Ticket{}, uErr
		}
		if _, rErr := SetRecommendedReady(ctx, db, ticketID, true, agentUsername, agentID); rErr != nil {
			return Ticket{}, rErr
		}
	case "breakdown":
		for _, st := range p.Stories {
			if strings.TrimSpace(st.Title) == "" {
				continue
			}
			if _, cErr := AddRefinementProposalChild(ctx, db, ticketID, st.Title, st.Description, st.AcceptanceCriteria, agentID); cErr != nil {
				return Ticket{}, cErr
			}
		}
		if _, rErr := SetRecommendedReady(ctx, db, ticketID, true, agentUsername, agentID); rErr != nil {
			return Ticket{}, rErr
		}
		if relErr := ReleaseRefinementTurn(ctx, db, ticketID); relErr != nil {
			return Ticket{}, relErr
		}
	default: // question / continue dialogue
		if relErr := ReleaseRefinementTurn(ctx, db, ticketID); relErr != nil {
			return Ticket{}, relErr
		}
	}
	return GetTicket(ctx, db, ticketID)
}

// ApproveRefinement is the human's explicit "this requirement is ready" action.
//   - If the refiner proposed a breakdown (the ticket has live draft children),
//     the ticket is re-typed to an epic and each child story is marked ready; the
//     epic then derives its lifecycle from its children.
//   - Otherwise the single ticket is marked ready (draft cleared, stage = ready).
//
// Returns the updated parent/epic ticket.
func ApproveRefinement(ctx context.Context, db *sql.DB, ticketID, actorUsername, actorID string) (Ticket, error) {
	ticket, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return Ticket{}, err
	}
	if ticket.Complete || ticket.Archived {
		return Ticket{}, ErrNotRefinable
	}

	children, err := listStoredChildTickets(ctx, db, ticketID)
	if err != nil {
		return Ticket{}, err
	}
	liveChildren := make([]Ticket, 0, len(children))
	for _, c := range children {
		if !c.Deleted {
			liveChildren = append(liveChildren, c)
		}
	}

	if len(liveChildren) == 0 {
		// Single story: just promote it to ready.
		return MarkTicketReady(ctx, db, ticketID, actorUsername, actorID)
	}

	// Breakdown: the idea becomes the epic; its children become ready stories.
	if _, err := db.ExecContext(ctx, `
		UPDATE tickets SET type = 'epic', draft = 0, recommended_ready = 0, assignee = '', updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, ticketID); err != nil {
		return Ticket{}, err
	}
	for _, child := range liveChildren {
		if child.Complete || child.Archived {
			continue
		}
		if _, err := MarkTicketReady(ctx, db, child.ID, actorUsername, actorID); err != nil {
			return Ticket{}, err
		}
	}
	_ = AddHistoryEvent(ctx, db, ticket.ProjectID, ticketID, "refinement_approved_breakdown", map[string]any{
		"children": len(liveChildren),
		"actor":    actorUsername,
	}, actorID)
	return GetTicket(ctx, db, ticketID)
}

// EnsureRefinerUser returns the user_id of an agent that performs the refiner
// role, creating a dedicated system refiner agent if none exists. Used to attribute
// streamed live-refinement replies to a refiner identity.
func EnsureRefinerUser(ctx context.Context, db *sql.DB) (string, error) {
	var id string
	err := db.QueryRowContext(ctx, `
		SELECT user_id FROM users
		WHERE user_type = 'agent' AND enabled = 1 AND LOWER(COALESCE(agent_role, '')) LIKE '%refiner%'
		ORDER BY user_id LIMIT 1
	`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	// Create a system refiner agent.
	agent, _, createErr := CreateAgent(ctx, db, "")
	if createErr != nil {
		return "", createErr
	}
	username := "refiner"
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username = ?`, username).Scan(&exists); err == nil && exists > 0 {
		username = "refiner-" + agent.ID[:8]
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE users SET username = ?, display_name = 'Refiner', agent_role = 'refiner' WHERE user_id = ?
	`, username, agent.ID); err != nil {
		return "", err
	}
	return agent.ID, nil
}

// ParseRefinementProposal interprets refiner LLM output into a refinement turn: a
// chat message plus a proposal (question / ready / breakdown). The markers are
// PROPOSE_READY (with DESCRIPTION:/ACCEPTANCE_CRITERIA:) and PROPOSE_BREAKDOWN
// (with STORY: <title> | <description> lines).
func ParseRefinementProposal(out string) RefinementTurnParams {
	text := strings.TrimSpace(out)
	switch {
	case strings.Contains(text, "PROPOSE_READY"):
		idx := strings.Index(text, "PROPOSE_READY")
		p := RefinementTurnParams{ProposalKind: "ready", Message: strings.TrimSpace(text[:idx])}
		body := text[idx+len("PROPOSE_READY"):]
		p.Description = refinementField(body, "DESCRIPTION:")
		p.AcceptanceCriteria = refinementField(body, "ACCEPTANCE_CRITERIA:")
		if p.Message == "" {
			p.Message = "Proposed a refined, ready story."
		}
		return p
	case strings.Contains(text, "PROPOSE_BREAKDOWN"):
		idx := strings.Index(text, "PROPOSE_BREAKDOWN")
		p := RefinementTurnParams{ProposalKind: "breakdown", Message: strings.TrimSpace(text[:idx])}
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
				p.Stories = append(p.Stories, RefinementStory{Title: title, Description: desc})
			}
		}
		if p.Message == "" {
			p.Message = "Proposed breaking this idea into stories."
		}
		if len(p.Stories) == 0 {
			return RefinementTurnParams{ProposalKind: "question", Message: text}
		}
		return p
	default:
		return RefinementTurnParams{ProposalKind: "question", Message: text}
	}
}

func refinementField(body, label string) string {
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

// ApplyLiveRefinerReply records a streamed live-refinement reply: it posts the
// refiner's message as a comment authored by refinerUserID and applies the proposal
// (single ready story, or breakdown into draft children). Unlike ApplyRefinementTurn
// it has no assignee guard — live refinement is driven by the server, not an
// orchestrator-assigned agent.
func ApplyLiveRefinerReply(ctx context.Context, db *sql.DB, ticketID, refinerUsername, refinerUserID string, p RefinementTurnParams) error {
	current, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return err
	}
	if msg := strings.TrimSpace(p.Message); msg != "" {
		if _, cErr := AddComment(ctx, db, ticketID, refinerUserID, msg); cErr != nil {
			return cErr
		}
	}
	switch p.ProposalKind {
	case "ready":
		desc := current.Description
		if strings.TrimSpace(p.Description) != "" {
			desc = p.Description
		}
		ac := current.AcceptanceCriteria
		if strings.TrimSpace(p.AcceptanceCriteria) != "" {
			ac = p.AcceptanceCriteria
		}
		if _, uErr := UpdateTicket(ctx, db, ticketID, TicketUpdateParams{
			Title: current.Title, Description: desc, AcceptanceCriteria: ac,
			ParentID: current.ParentID, Assignee: "", Stage: current.Stage, State: StateIdle,
			Priority: current.Priority, Order: current.Order,
			ActorUsername: refinerUsername, ActorRole: "admin",
		}); uErr != nil {
			return uErr
		}
		if _, rErr := SetRecommendedReady(ctx, db, ticketID, true, refinerUsername, refinerUserID); rErr != nil {
			return rErr
		}
	case "breakdown":
		for _, st := range p.Stories {
			if strings.TrimSpace(st.Title) == "" {
				continue
			}
			if _, cErr := AddRefinementProposalChild(ctx, db, ticketID, st.Title, st.Description, st.AcceptanceCriteria, refinerUserID); cErr != nil {
				return cErr
			}
		}
		if _, rErr := SetRecommendedReady(ctx, db, ticketID, true, refinerUsername, refinerUserID); rErr != nil {
			return rErr
		}
	}
	return nil
}

// AddRefinementProposalChild creates a proposed child story under an idea during
// breakdown. The child is created as a draft in the same project so the human can
// review it before approving. Returns the new child ticket.
func AddRefinementProposalChild(ctx context.Context, db *sql.DB, parentID, title, description, acceptanceCriteria, createdBy string) (Ticket, error) {
	parent, err := GetTicket(ctx, db, parentID)
	if err != nil {
		return Ticket{}, err
	}
	pid := parentID
	return CreateTicket(ctx, db, TicketCreateParams{
		ProjectID:          parent.ProjectID,
		ParentID:           &pid,
		Type:               "story",
		Title:              strings.TrimSpace(title),
		Description:        strings.TrimSpace(description),
		AcceptanceCriteria: strings.TrimSpace(acceptanceCriteria),
		CreatedBy:          createdBy,
	})
}
