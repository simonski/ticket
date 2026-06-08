package orchestrator

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/simonski/ticket/internal/store"
)

// ── Pure decision-matrix tests (no DB) ────────────────────────────────────────

func sealedSprintID() *int64 { id := int64(7); return &id }

func engineerAgent() store.OrchestratorAgent {
	return store.OrchestratorAgent{UserID: "u1", Username: "eng-agent", Roles: []string{"Engineer"}, ActiveLoad: 0}
}

func TestDecideAssignsIdleSealedSprintTicket(t *testing.T) {
	pool := newAgentPool([]store.OrchestratorAgent{engineerAgent()})
	tk := store.OrchestratorTicket{
		TicketID: "DEV-1", ProjectID: 1, Stage: "develop", State: store.StateIdle,
		RoleTitle: "Engineer", WorkflowStageID: 5, SprintID: sealedSprintID(),
		SprintStage: store.SprintSealedStage,
	}
	d := decide(tk, pool, time.Now().UTC(), time.Minute, 0, true)
	if d.Kind != ActionAssign {
		t.Fatalf("Kind = %q, want assign (%s)", d.Kind, d.Detail)
	}
	if d.Agent != "eng-agent" {
		t.Fatalf("Agent = %q, want eng-agent", d.Agent)
	}
}

func TestDecideSkipsIdleUnsealedSprint(t *testing.T) {
	pool := newAgentPool([]store.OrchestratorAgent{engineerAgent()})
	tk := store.OrchestratorTicket{
		TicketID: "DEV-2", ProjectID: 1, Stage: "develop", State: store.StateIdle,
		RoleTitle: "Engineer", WorkflowStageID: 5, SprintID: sealedSprintID(),
		SprintStage: "closed", // not sealed
	}
	d := decide(tk, pool, time.Now().UTC(), time.Minute, 0, true)
	if d.Kind != ActionSkip {
		t.Fatalf("Kind = %q, want skip", d.Kind)
	}
}

func TestDecideSkipsWhenNoMatchingAgent(t *testing.T) {
	pool := newAgentPool([]store.OrchestratorAgent{{Username: "qa-agent", Roles: []string{"QA Engineer"}}})
	tk := store.OrchestratorTicket{
		TicketID: "DEV-3", ProjectID: 1, Stage: "develop", State: store.StateIdle,
		RoleTitle: "Engineer", WorkflowStageID: 5, SprintID: sealedSprintID(),
		SprintStage: store.SprintSealedStage,
	}
	d := decide(tk, pool, time.Now().UTC(), time.Minute, 0, true)
	if d.Kind != ActionSkip {
		t.Fatalf("Kind = %q, want skip (no matching agent)", d.Kind)
	}
}

func TestDecideAssignsRefinerOnAgentTurn(t *testing.T) {
	pool := newAgentPool([]store.OrchestratorAgent{{Username: "refiner-bot", Roles: []string{"refiner"}}})
	tk := store.OrchestratorTicket{
		TicketID: "IDEA-1", ProjectID: 1, Stage: "design", State: store.StateIdle, StageIsBacklog: true,
		Draft: true, RefinementAgentTurn: true,
	}
	d := decide(tk, pool, time.Now().UTC(), time.Minute, 0, true)
	if d.Kind != ActionAssign || d.Agent != "refiner-bot" {
		t.Fatalf("decision = %+v, want assign to refiner-bot", d)
	}
}

func TestDecideClosesIdleRefineSession(t *testing.T) {
	pool := newAgentPool([]store.OrchestratorAgent{{Username: "refiner-bot", Roles: []string{"refiner"}}})
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	tk := store.OrchestratorTicket{
		TicketID: "IDEA-3", ProjectID: 1, Stage: "design", State: store.StateIdle, StageIsBacklog: true,
		Draft: true, RefinementAgentTurn: true,
		RefinementLastActivity: "2026-06-08 11:30:00", // 30 min ago
	}
	// Idle window of 15 minutes → the session is dormant → skip (don't tie up a refiner).
	d := decide(tk, pool, now, time.Minute, 15*time.Minute, true)
	if d.Kind != ActionSkip {
		t.Fatalf("Kind = %q, want skip (idle session closed)", d.Kind)
	}
	// Fresh activity → assign.
	tk.RefinementLastActivity = "2026-06-08 11:59:00" // 1 min ago
	if d := decide(tk, pool, now, time.Minute, 15*time.Minute, true); d.Kind != ActionAssign {
		t.Fatalf("Kind = %q, want assign (active session)", d.Kind)
	}
}

func TestDecideSkipsRefineAwaitingHuman(t *testing.T) {
	pool := newAgentPool([]store.OrchestratorAgent{{Username: "refiner-bot", Roles: []string{"refiner"}}})
	tk := store.OrchestratorTicket{
		TicketID: "IDEA-2", ProjectID: 1, Stage: "design", State: store.StateIdle, StageIsBacklog: true,
		Draft: true, RefinementAgentTurn: false, // latest message is the agent's
	}
	d := decide(tk, pool, time.Now().UTC(), time.Minute, 0, true)
	if d.Kind != ActionSkip {
		t.Fatalf("Kind = %q, want skip (awaiting human)", d.Kind)
	}
}

func TestDecideAdvancesSuccess(t *testing.T) {
	pool := newAgentPool(nil)
	tk := store.OrchestratorTicket{
		TicketID: "DEV-4", ProjectID: 1, Stage: "develop", State: store.StateSuccess,
		WorkflowStageID: 5, SprintID: sealedSprintID(), SprintStage: store.SprintSealedStage,
	}
	d := decide(tk, pool, time.Now().UTC(), time.Minute, 0, true)
	if d.Kind != ActionAdvance {
		t.Fatalf("Kind = %q, want advance", d.Kind)
	}
}

func TestDecideRecoversFail(t *testing.T) {
	pool := newAgentPool(nil)
	tk := store.OrchestratorTicket{
		TicketID: "DEV-5", ProjectID: 1, Stage: "develop", State: store.StateFail,
		WorkflowStageID: 5,
	}
	d := decide(tk, pool, time.Now().UTC(), time.Minute, 0, true)
	if d.Kind != ActionRecover {
		t.Fatalf("Kind = %q, want recover", d.Kind)
	}
}

func TestDecideAbandonsStaleActive(t *testing.T) {
	pool := newAgentPool(nil)
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	tk := store.OrchestratorTicket{
		TicketID: "DEV-6", ProjectID: 1, Stage: "develop", State: store.StateActive,
		Assignee: "eng-agent", AssigneeIsAgent: true, AssigneeLastSeen: "2026-06-08 11:50:00", // 10 min ago
	}
	d := decide(tk, pool, now, 2*time.Minute, 0, true)
	if d.Kind != ActionAbandon {
		t.Fatalf("Kind = %q, want abandon", d.Kind)
	}
}

func TestDecideDoesNotAbandonHumanAssigned(t *testing.T) {
	pool := newAgentPool(nil)
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	tk := store.OrchestratorTicket{
		TicketID: "DEV-6h", ProjectID: 1, Stage: "develop", State: store.StateActive,
		Assignee: "alice", AssigneeIsAgent: false, AssigneeLastSeen: "", // a human, no heartbeat
	}
	d := decide(tk, pool, now, 2*time.Minute, 0, true)
	if d.Kind != ActionSkip {
		t.Fatalf("Kind = %q, want skip (human-assigned active is left alone)", d.Kind)
	}
}

func TestDecideLeavesFreshActiveAlone(t *testing.T) {
	pool := newAgentPool(nil)
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	tk := store.OrchestratorTicket{
		TicketID: "DEV-7", ProjectID: 1, Stage: "develop", State: store.StateActive,
		Assignee: "eng-agent", AssigneeIsAgent: true, AssigneeLastSeen: "2026-06-08 11:59:30", // 30s ago
	}
	d := decide(tk, pool, now, 2*time.Minute, 0, true)
	if d.Kind != ActionSkip {
		t.Fatalf("Kind = %q, want skip (fresh heartbeat)", d.Kind)
	}
}

func TestDecideSkipsDisabledProject(t *testing.T) {
	pool := newAgentPool([]store.OrchestratorAgent{engineerAgent()})
	tk := store.OrchestratorTicket{
		TicketID: "DEV-8", ProjectID: 1, Stage: "develop", State: store.StateIdle,
		RoleTitle: "Engineer", WorkflowStageID: 5, SprintID: sealedSprintID(),
		SprintStage: store.SprintSealedStage,
	}
	d := decide(tk, pool, time.Now().UTC(), time.Minute, 0, false /* project disabled */)
	if d.Kind != ActionSkip {
		t.Fatalf("Kind = %q, want skip (disabled project)", d.Kind)
	}
}

func TestDecideSkipsDraftAndNonLeaf(t *testing.T) {
	pool := newAgentPool([]store.OrchestratorAgent{engineerAgent()})
	base := store.OrchestratorTicket{
		TicketID: "DEV-9", ProjectID: 1, Stage: "develop", State: store.StateIdle,
		RoleTitle: "Engineer", WorkflowStageID: 5, SprintID: sealedSprintID(),
		SprintStage: store.SprintSealedStage,
	}
	draft := base
	draft.Draft = true
	if d := decide(draft, pool, time.Now().UTC(), time.Minute, 0, true); d.Kind != ActionSkip {
		t.Fatalf("draft Kind = %q, want skip", d.Kind)
	}
	parent := base
	parent.HasChildren = true
	if d := decide(parent, pool, time.Now().UTC(), time.Minute, 0, true); d.Kind != ActionSkip {
		t.Fatalf("non-leaf Kind = %q, want skip", d.Kind)
	}
}

func TestAgentPoolPicksLeastBusyAndBalances(t *testing.T) {
	pool := newAgentPool([]store.OrchestratorAgent{
		{Username: "eng-a", Roles: []string{"Engineer"}, ActiveLoad: 2},
		{Username: "eng-b", Roles: []string{"Engineer"}, ActiveLoad: 0},
	})
	first := pool.pick("Engineer")
	if first == nil || first.Username != "eng-b" {
		t.Fatalf("first pick = %v, want eng-b (least busy)", first)
	}
	pool.charge("eng-b") // eng-b now at 1
	second := pool.pick("Engineer")
	if second == nil || second.Username != "eng-b" {
		t.Fatalf("second pick = %v, want eng-b (still 1 < 2)", second)
	}
	pool.charge("eng-b") // eng-b now at 2, tie with eng-a -> name order picks eng-a
	third := pool.pick("Engineer")
	if third == nil || third.Username != "eng-a" {
		t.Fatalf("third pick = %v, want eng-a (tie broken by name)", third)
	}
}

// ── DB-backed apply test ──────────────────────────────────────────────────────

func TestPassAssignsInSealedSprint(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "orch.db")
	if err := store.Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Workflow with a single "develop" stage owned by role "Engineer".
	wf, err := store.CreateWorkflow(ctx, db, "Exec Flow", "")
	if err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}
	stage, err := store.AddWorkflowStage(ctx, db, wf.ID, "develop", "", "", 1)
	if err != nil {
		t.Fatalf("AddWorkflowStage: %v", err)
	}
	role, err := store.CreateRoleWithParams(ctx, db, store.RoleCreateParams{WorkflowID: &wf.ID, Title: "Engineer"})
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if err := store.AddWorkflowStageRole(ctx, db, wf.ID, stage.ID, role.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole: %v", err)
	}

	proj, err := store.CreateProject(ctx, db, "Exec", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// An enabled agent that performs Engineer.
	agent, _, err := store.CreateAgent(ctx, db, "password")
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET username = 'eng-agent', agent_role = 'Engineer', last_seen = CURRENT_TIMESTAMP, status = 'soliciting' WHERE user_id = ?`, agent.ID); err != nil {
		t.Fatalf("configure agent: %v", err)
	}

	// A ticket, put it on the workflow (sets role to first stage role), make it a
	// leaf, non-draft, develop/idle, and place it in a sealed (active) sprint.
	tk, err := store.CreateTicket(ctx, db, store.TicketCreateParams{ProjectID: proj.ID, Type: "story", Title: "Build it"})
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if _, err := store.SetTicketWorkflow(ctx, db, tk.ID, wf.ID); err != nil {
		t.Fatalf("SetTicketWorkflow: %v", err)
	}
	adminID := adminUserID(ctx, t, db)
	if _, err := store.SetTicketDraft(ctx, db, tk.ID, false, "admin", adminID); err != nil {
		t.Fatalf("SetTicketDraft: %v", err)
	}
	sprint, err := store.CreateSprint(ctx, db, int(proj.ID), "S1")
	if err != nil {
		t.Fatalf("CreateSprint: %v", err)
	}
	// Place the leaf at develop/idle inside the sprint (direct SQL, seeding-style,
	// bypassing the ready-stage gate the same way tk demo does).
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET stage = 'develop', state = 'idle', status = 'develop/idle', sprint_id = ? WHERE ticket_id = ?`, sprint.ID, tk.ID); err != nil {
		t.Fatalf("set develop/idle in sprint: %v", err)
	}
	if _, err := store.SealSprint(ctx, db, sprint.ID); err != nil {
		t.Fatalf("SealSprint: %v", err)
	}

	// Run a real pass.
	decisions, err := Pass(ctx, db, Options{HeartbeatTimeout: 2 * time.Minute})
	if err != nil {
		t.Fatalf("Pass: %v", err)
	}
	var assigned *Decision
	for i := range decisions {
		if decisions[i].TicketID == tk.ID {
			assigned = &decisions[i]
		}
	}
	if assigned == nil {
		t.Fatalf("no decision for ticket %s", tk.ID)
	}
	if assigned.Kind != ActionAssign || !assigned.Applied || assigned.Agent != "eng-agent" {
		t.Fatalf("decision = %+v, want applied assign to eng-agent", *assigned)
	}

	// Verify the ticket is now active and assigned in the DB.
	got, err := store.GetTicket(ctx, db, tk.ID)
	if err != nil {
		t.Fatalf("GetTicket: %v", err)
	}
	if got.Assignee != "eng-agent" || got.State != store.StateActive {
		t.Fatalf("ticket = assignee %q state %q, want eng-agent/active", got.Assignee, got.State)
	}
}

func TestPassAbandonsStaleAgentWork(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "orch_abandon.db")
	if err := store.Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	proj, err := store.CreateProject(ctx, db, "Abandon", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	agent, _, err := store.CreateAgent(ctx, db, "password")
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	// Agent last seen 10 minutes ago (stale).
	if _, err := db.ExecContext(ctx, `UPDATE users SET username = 'eng-agent', agent_role = 'Engineer', last_seen = datetime('now','-10 minutes'), status = 'working' WHERE user_id = ?`, agent.ID); err != nil {
		t.Fatalf("configure stale agent: %v", err)
	}
	tk, err := store.CreateTicket(ctx, db, store.TicketCreateParams{ProjectID: proj.ID, Type: "story", Title: "In flight"})
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	// Active, assigned to the (now silent) agent.
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET stage='develop', state='active', draft=0, assignee='eng-agent', status='develop/active' WHERE ticket_id=?`, tk.ID); err != nil {
		t.Fatalf("set active assigned: %v", err)
	}

	decisions, err := Pass(ctx, db, Options{HeartbeatTimeout: 2 * time.Minute})
	if err != nil {
		t.Fatalf("Pass: %v", err)
	}
	var dec *Decision
	for i := range decisions {
		if decisions[i].TicketID == tk.ID {
			dec = &decisions[i]
		}
	}
	if dec == nil || dec.Kind != ActionAbandon || !dec.Applied {
		t.Fatalf("decision = %+v, want applied abandon", dec)
	}
	got, err := store.GetTicket(ctx, db, tk.ID)
	if err != nil {
		t.Fatalf("GetTicket: %v", err)
	}
	if got.Assignee != "" || got.State != store.StateIdle {
		t.Fatalf("after abandon: assignee %q state %q, want empty/idle", got.Assignee, got.State)
	}
}

func adminUserID(ctx context.Context, t *testing.T, db *sql.DB) string {
	t.Helper()
	u, err := store.GetUserByUsername(ctx, db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin): %v", err)
	}
	return u.ID
}
