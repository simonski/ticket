// Package orchestrator coordinates the movement of tickets through their
// workflows. It is the single component permitted to assign work to agents.
//
// The orchestrator is purely deterministic — it applies workflow rules and never
// reasons with an LLM (all intelligence lives in the agents, which execute work
// and self-assess success/fail). On each pass it sweeps every eligible ticket and,
// per ticket, decides one of:
//
//   - abandon  — an active job whose agent has gone silent (heartbeat stale);
//     released back to idle so it can be re-assigned.
//   - advance  — a story the agent marked success; moved to the next role/stage.
//   - recover  — a story the agent marked fail; moved back a step.
//   - assign   — an idle, unassigned, leaf story in a sealed sprint whose current
//     role matches an available agent; pushed to the least-busy one.
//   - skip     — nothing to do (with a reason, surfaced by the dry-run CLI).
//
// See docs/DESIGN_ORCHESTRATOR.md for the full design and the decisions behind it.
package orchestrator

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/simonski/ticket/internal/store"
)

// ActionKind enumerates the decisions the orchestrator can make about a ticket.
type ActionKind string

const (
	ActionSkip    ActionKind = "skip"
	ActionAssign  ActionKind = "assign"
	ActionAdvance ActionKind = "advance"
	ActionRecover ActionKind = "recover"
	ActionAbandon ActionKind = "abandon"
)

// Decision records what the orchestrator chose (or would choose, in dry-run) for a
// single ticket.
type Decision struct {
	TicketID  string     `json:"ticket_id"`
	ProjectID int64      `json:"project_id"`
	Kind      ActionKind `json:"kind"`
	From      string     `json:"from"`            // e.g. "develop/idle"
	Agent     string     `json:"agent,omitempty"` // assignee, for assign/abandon
	Detail    string     `json:"detail"`          // human-readable explanation
	Applied   bool       `json:"applied"`         // whether the action was executed
	Err       string     `json:"error,omitempty"` // execution error, if any
}

// Options controls a single orchestrator pass.
type Options struct {
	DryRun           bool          // when true, decisions are computed but not applied
	ProjectID        int64         // 0 = all projects
	TicketID         string        // "" = all tickets
	HeartbeatTimeout time.Duration // 0 = look up from settings
	RefinementIdle   time.Duration // 0 = look up from settings; refinement session idle window
	Now              time.Time     // injectable clock for tests; zero = time.Now()
	// SkipRefineTickets are tickets with an active live (streaming) refinement
	// session; the orchestrator leaves their refinement to the live session so the
	// two refiners never both reply.
	SkipRefineTickets map[string]bool
}

// Pass performs one orchestration sweep and returns the decision for every ticket
// it considered (including skips, so dry-run can explain inaction). When
// opts.DryRun is false, the assign/advance/recover/abandon actions are applied.
func Pass(ctx context.Context, db *sql.DB, opts Options) ([]Decision, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	timeout := opts.HeartbeatTimeout
	if timeout <= 0 {
		secs, err := store.OrchestratorHeartbeatTimeoutSeconds(ctx, db)
		if err != nil {
			return nil, fmt.Errorf("read heartbeat timeout: %w", err)
		}
		timeout = time.Duration(secs) * time.Second
	}

	refinementIdle := opts.RefinementIdle
	if refinementIdle <= 0 {
		mins, err := store.RefinementIdleMinutes(ctx, db)
		if err != nil {
			return nil, fmt.Errorf("read refinement idle minutes: %w", err)
		}
		refinementIdle = time.Duration(mins) * time.Minute
	}

	candidates, err := store.ListOrchestratorCandidates(ctx, db, opts.ProjectID, opts.TicketID)
	if err != nil {
		return nil, fmt.Errorf("list candidates: %w", err)
	}

	agents, err := store.ListOrchestratorAgents(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	pool := newAgentPool(agents)

	// Cache per-project enabled lookups within a pass.
	enabledCache := map[int64]bool{}
	projectEnabled := func(pid int64) (bool, error) {
		if v, ok := enabledCache[pid]; ok {
			return v, nil
		}
		v, err := store.OrchestratorEnabledForProject(ctx, db, pid)
		if err != nil {
			return false, err
		}
		enabledCache[pid] = v
		return v, nil
	}

	decisions := make([]Decision, 0, len(candidates))
	for _, t := range candidates {
		enabled, err := projectEnabled(t.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("project %d enabled: %w", t.ProjectID, err)
		}
		if opts.SkipRefineTickets[t.TicketID] && t.InRefinement() {
			decisions = append(decisions, Decision{
				TicketID: t.TicketID, ProjectID: t.ProjectID, Kind: ActionSkip,
				From: t.Stage + "/" + t.State, Detail: "refinement — live streaming session active",
			})
			continue
		}
		d := decide(t, pool, now, timeout, refinementIdle, enabled)
		if !opts.DryRun && d.Kind != ActionSkip {
			apply(ctx, db, t, &d, pool)
		}
		decisions = append(decisions, d)
	}
	return decisions, nil
}

// decide computes (but does not apply) the action for one ticket.
func decide(t store.OrchestratorTicket, pool *agentPool, now time.Time, timeout, refinementIdle time.Duration, projectEnabled bool) Decision {
	d := Decision{
		TicketID:  t.TicketID,
		ProjectID: t.ProjectID,
		Kind:      ActionSkip,
		From:      t.Stage + "/" + t.State,
		Agent:     t.Assignee,
	}

	if !projectEnabled {
		d.Detail = "project is opted out of orchestration"
		return d
	}

	switch t.State {
	case store.StateActive:
		// In-flight work. Only agent-assigned work is the orchestrator's to manage;
		// human-assigned active tickets are left alone. Abandon only if the assigned
		// agent has gone silent past the heartbeat timeout.
		if t.Assignee != "" && t.AssigneeIsAgent && store.AssigneeHeartbeatStale(t.AssigneeLastSeen, now, timeout) {
			d.Kind = ActionAbandon
			d.Detail = fmt.Sprintf("agent %q heartbeat stale (> %s) — release to idle", t.Assignee, timeout)
			return d
		}
		d.Detail = "in progress"
		return d

	case store.StateSuccess:
		// Agent reported success. Advance to the next role/stage.
		if t.Stage == store.StageReady && t.SprintID == nil {
			d.Detail = "ready but not in a sprint — cannot advance until sealed into a sprint"
			return d
		}
		d.Kind = ActionAdvance
		d.Detail = "success — advance to next step"
		return d

	case store.StateFail:
		// Agent reported failure. Move back a step for a retry.
		d.Kind = ActionRecover
		d.Detail = "fail — move back a step"
		return d

	case store.StateIdle:
		return decideIdle(t, pool, now, refinementIdle, d)
	}

	d.Detail = "unknown state"
	return d
}

// decideIdle handles the assignment decision for an idle ticket.
func decideIdle(t store.OrchestratorTicket, pool *agentPool, now time.Time, refinementIdle time.Duration, d Decision) Decision {
	if t.Assignee != "" {
		d.Detail = "idle but already has an assignee"
		return d
	}

	// Preparation phase: a draft ticket sitting in a backlog stage is refined in
	// place by a refiner agent, in a turn-based dialogue with the human — regardless
	// of what that backlog stage is named (design, idea, refine, …). No literal
	// "refine" stage is required.
	if t.InRefinement() {
		if !t.RefinementAgentTurn {
			d.Detail = "refinement — awaiting human reply or approval"
			return d
		}
		// Idle-session cleanup: a dormant conversation closes so it doesn't tie up a
		// refiner. The human resumes it by replying (which refreshes the timestamp).
		if store.RefinementSessionIdle(t.RefinementLastActivity, now, refinementIdle) {
			d.Detail = "refinement — session idle, closed (reply to resume)"
			return d
		}
		if !t.IsLeaf() {
			d.Detail = "refinement — broken down into stories, awaiting human approval"
			return d
		}
		agent := pool.pick("refiner")
		if agent == nil {
			d.Detail = "refinement — no available refiner agent"
			return d
		}
		d.Kind = ActionAssign
		d.Agent = agent.Username
		d.Detail = fmt.Sprintf("assign refinement to refiner agent %q", agent.Username)
		return d
	}

	// A still-draft ticket that is NOT in a backlog stage is not yet ready for work.
	if t.Draft {
		d.Detail = "draft — not yet ready for work"
		return d
	}
	if !t.IsLeaf() {
		d.Detail = "has children — only leaf stories are worked"
		return d
	}
	// Ready (non-draft) work is executed once it is in a sealed sprint; backlog work
	// that isn't draft (e.g. a literal "ready" stage) waits for a sprint.
	if !t.SprintSealed() {
		d.Detail = "not in a sealed (active) sprint"
		return d
	}
	if t.WorkflowStageID == 0 || t.RoleTitle == "" {
		d.Detail = "no current role to match an agent against"
		return d
	}
	agent := pool.pick(t.RoleTitle)
	if agent == nil {
		d.Detail = fmt.Sprintf("no available agent performs role %q", t.RoleTitle)
		return d
	}
	d.Kind = ActionAssign
	d.Agent = agent.Username
	d.Detail = fmt.Sprintf("assign role %q to least-busy agent %q", t.RoleTitle, agent.Username)
	return d
}

// apply executes a non-skip decision against the database, recording success on
// the decision.
func apply(ctx context.Context, db *sql.DB, t store.OrchestratorTicket, d *Decision, pool *agentPool) {
	switch d.Kind {
	case ActionAssign:
		ok, err := store.AssignTicketToAgent(ctx, db, t.TicketID, d.Agent, t.ProjectID)
		if err != nil {
			d.Err = err.Error()
			return
		}
		if !ok {
			d.Detail = "assignment lost a race (ticket no longer idle)"
			return
		}
		pool.charge(d.Agent) // count the new load so balancing holds within the pass
		d.Applied = true

	case ActionAdvance:
		if _, err := store.NextTicket(ctx, db, t.TicketID, "orchestrator", "orchestrator"); err != nil {
			d.Err = err.Error()
			return
		}
		// Clear any lingering assignee so the next role starts unassigned.
		_ = store.ClearTicketAssignee(ctx, db, t.TicketID)
		_ = store.AddHistoryEvent(ctx, db, t.ProjectID, t.TicketID, "orchestrator_advanced",
			map[string]any{"from": d.From}, "orchestrator")
		d.Applied = true

	case ActionRecover:
		if _, err := store.PreviousTicket(ctx, db, t.TicketID, "orchestrator", "orchestrator"); err != nil {
			d.Err = err.Error()
			return
		}
		_ = store.ClearTicketAssignee(ctx, db, t.TicketID)
		_ = store.AddHistoryEvent(ctx, db, t.ProjectID, t.TicketID, "orchestrator_recovered",
			map[string]any{"from": d.From}, "orchestrator")
		d.Applied = true

	case ActionAbandon:
		ok, err := store.AbandonTicket(ctx, db, t.TicketID, t.ProjectID, d.Agent, "heartbeat timeout")
		if err != nil {
			d.Err = err.Error()
			return
		}
		d.Applied = ok
	}
}

// ── Agent pool: load-balanced selection ───────────────────────────────────────

// agentPool tracks agents and their live workload so the orchestrator can spread
// work across the least-busy agents for a role (Q2: order unimportant, balance is).
type agentPool struct {
	agents []store.OrchestratorAgent
	load   map[string]int // username -> active load (mutated as we assign in a pass)
}

func newAgentPool(agents []store.OrchestratorAgent) *agentPool {
	load := make(map[string]int, len(agents))
	for _, a := range agents {
		load[a.Username] = a.ActiveLoad
	}
	return &agentPool{agents: agents, load: load}
}

// pick returns the least-busy enabled agent able to perform the role, or nil.
func (p *agentPool) pick(roleName string) *store.OrchestratorAgent {
	matching := make([]*store.OrchestratorAgent, 0)
	for i := range p.agents {
		if p.agents[i].PerformsRole(roleName) {
			matching = append(matching, &p.agents[i])
		}
	}
	if len(matching) == 0 {
		return nil
	}
	sort.SliceStable(matching, func(i, j int) bool {
		li, lj := p.load[matching[i].Username], p.load[matching[j].Username]
		if li != lj {
			return li < lj
		}
		return matching[i].Username < matching[j].Username
	})
	return matching[0]
}

// charge increments an agent's tracked load after an assignment so subsequent
// picks in the same pass keep the distribution balanced.
func (p *agentPool) charge(username string) {
	p.load[username]++
}
