package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/simonski/ticket/internal/static"
	"github.com/simonski/ticket/internal/store"
)

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// --- Content pools ---

var frontendTitles = []string{
	"Implement responsive task list with virtual scrolling",
	"Add keyboard shortcut system for power users",
	"Fix task card hover state regression on Safari 17",
	"Build drag-and-drop task reordering with animation",
	"Implement optimistic UI updates for task completion",
	"Design and build empty state illustrations",
	"Add task priority colour indicators to board view",
	"Implement infinite scroll for projects with large task lists",
	"Fix focus management in task creation modal",
	"Add markdown rendering in task description fields",
	"Build real-time task search with debounced filtering",
	"Implement task due date picker with timezone support",
	"Fix animation jank when toggling task status",
	"Add unread notification badge to sidebar",
	"Build bulk action toolbar for multi-selected tasks",
	"Implement user avatar stack for task assignees",
	"Fix CSS grid layout regression introduced in last sprint",
	"Add loading skeleton screens for initial page render",
	"Build notification centre with read/unread management",
	"Implement task archive workflow with undo action",
	"Add project colour picker to project settings",
	"Fix memory leak in WebSocket reconnection handler",
	"Build onboarding tour for new user registration",
	"Implement task template library for common workflows",
	"Add export to CSV and JSON from task list view",
	"Fix contrast ratio failures flagged in accessibility audit",
	"Build task comment threading with inline replies",
	"Implement mention autocomplete in comment editor",
	"Add task watcher subscription system",
	"Fix tooltip positioning in compact board view",
}

var backendTitles = []string{
	"Implement paginated task list endpoint with cursor support",
	"Add JWT refresh token rotation with sliding expiry",
	"Build webhook delivery system with retry and dead-letter queue",
	"Implement task assignment notification emails via SendGrid",
	"Add PostgreSQL full-text search index for tasks",
	"Fix N+1 query in project members list endpoint",
	"Implement per-user API rate limiting with token bucket",
	"Build task export endpoint supporting CSV and JSON",
	"Add soft delete and restore for tasks and projects",
	"Implement task activity audit log with pagination",
	"Fix race condition in concurrent task status updates",
	"Add input sanitisation middleware for all write endpoints",
	"Build batch task update endpoint for bulk status changes",
	"Implement GraphQL schema alongside existing REST API",
	"Add GDPR data export endpoint for user account data",
	"Build project statistics aggregation with caching",
	"Fix memory leak in long-running WebSocket goroutines",
	"Implement task comment system with threading support",
	"Add API versioning with header-based negotiation",
	"Build task recurrence engine for repeating tasks",
	"Implement OAuth2 provider integration (Google, GitHub)",
	"Add multi-tenancy isolation at the database query layer",
	"Fix timezone handling in due date serialisation",
	"Build event sourcing log for task state transitions",
	"Implement task dependency graph with cycle detection",
	"Add structured logging with trace ID propagation",
	"Build idempotency key support for POST endpoints",
	"Implement server-sent events for lightweight real-time updates",
	"Fix connection pool exhaustion under high concurrency load",
	"Add request signing for outbound webhook calls",
}

var infraTitles = []string{
	"Set up GitHub Actions CI pipeline with matrix build",
	"Configure pgBouncer connection pooling in front of PostgreSQL",
	"Add Redis cluster for session cache and rate limit counters",
	"Write Kubernetes deployment manifests for all services",
	"Configure automated database backup to S3 with point-in-time restore",
	"Add Prometheus metrics endpoint and scrape config",
	"Build Grafana dashboard for API latency percentiles",
	"Configure automated SSL certificate renewal via cert-manager",
	"Optimise Docker multi-stage build to reduce image size by 60%",
	"Set up staging environment with anonymised production dataset",
	"Configure Loki log aggregation with Grafana dashboard",
	"Implement blue-green deployment strategy with traffic shifting",
	"Document database schema migration rollback procedures",
	"Configure CloudFront CDN for static asset delivery",
	"Set up PagerDuty alerting for error rate threshold breaches",
	"Add k6 load test suite to CI pipeline",
	"Configure HashiCorp Vault for secrets management",
	"Set up Renovate for automated dependency updates",
	"Write Terraform modules for VPC, RDS, and ECS infrastructure",
	"Add OpenTelemetry distributed tracing across all services",
	"Configure horizontal pod autoscaling based on request rate",
	"Write runbook for on-call incident response procedures",
	"Set up chaos engineering tests with Chaos Monkey",
	"Add SBOM generation to release pipeline",
	"Configure network policies to enforce service mesh isolation",
	"Implement GitOps workflow with ArgoCD",
	"Add database query analyser job for slow query detection",
	"Configure multi-region failover with Route 53 health checks",
	"Set up security scanning with Trivy in CI",
	"Document disaster recovery procedure with RTO/RPO targets",
}

var bugTitles = []string{
	"Tasks marked complete reappear after browser refresh",
	"API returns 500 on task creation when title contains emoji",
	"Drag and drop breaks when more than 50 tasks are visible",
	"Notification emails sent with wrong timezone offset",
	"Search returns stale results after task is deleted",
	"Rate limit headers missing from OPTIONS preflight responses",
	"Mobile keyboard pushes task modal off screen on iOS 17",
	"Database connection leaks after failed authentication attempt",
	"Task sort order resets to zero after project rename",
	"Webhook delivery silently drops payloads over 64KB",
	"Dark mode colours inverted on task priority chips",
	"Sprint dates not preserved after daylight saving change",
	"Bulk delete removes tasks in wrong project when filter active",
	"WebSocket disconnects after exactly 30 minutes idle",
	"Avatar images not loading behind corporate proxy",
	"Team member removal does not revoke active API tokens",
	"Export CSV contains duplicate rows for tasks with multiple labels",
	"Password reset link expires too quickly on slow email clients",
	"Board view performance degrades with more than 200 open tasks",
	"Comment timestamps show UTC instead of user local time",
}

var choreTitles = []string{
	"Upgrade React to v19 and fix breaking API changes",
	"Replace deprecated crypto.createHash calls in token generator",
	"Migrate ESLint config from .eslintrc to flat config format",
	"Remove unused feature flags from configuration system",
	"Update Go dependencies to latest stable versions",
	"Refactor task store to use repository pattern",
	"Add missing OpenAPI documentation for webhook endpoints",
	"Replace polling with WebSocket in dashboard stats widget",
	"Clean up dead code paths in task state machine",
	"Standardise error response format across all API endpoints",
}

var descTemplates = []string{
	"This %s needs to be addressed as part of the ongoing %s work. The current implementation has several shortcomings that have been flagged by the team during the last sprint review. We should prioritise this to unblock downstream work.",
	"Following the technical design session on %s, we agreed to implement this as a standalone %s. Acceptance criteria have been defined in collaboration with the product team and QA. See linked Figma designs for visual reference.",
	"Reported by multiple users during beta testing of the %s feature. Reproducible on %s consistently. Root cause has been narrowed down but full fix requires refactoring the affected module.",
	"Part of the %s epic. This subtask covers the %s layer implementation. Depends on the authentication work landing first — coordinate with the relevant assignee before starting.",
	"Technical debt item identified during the %s architecture review. Leaving this unresolved increases the risk of production incidents. Estimated effort is low but the impact on system stability is high.",
	"Spike to investigate the best approach for %s. We need to evaluate at least two options and document the trade-offs before committing to an implementation. Output should be a short design doc.",
	"Customer-reported issue affecting all users on the %s plan. Priority raised after three separate support tickets this week. Needs a hotfix candidate before the next release window.",
	"Performance regression introduced in the %s refactor last sprint. The %s endpoint is now 3× slower under load. Profiling data attached in the linked document.",
}

var commentTemplates = []string{
	"Took a look at this — the issue is in the %s layer. Will have a fix up for review by end of day.",
	"Left a few comments on the draft PR. Main concern is around error handling for the edge case where the %s is nil.",
	"QA has verified the fix on staging. No regressions found in the related test suite. Good to go for the next release.",
	"Blocked on the %s dependency — reaching out to that team now. Should have an update by tomorrow.",
	"Design review complete. The approach looks solid. One suggestion: consider extracting the %s logic into a separate helper to make testing easier.",
	"This is now merged and deployed to staging. Monitoring dashboards look clean. Will promote to production during Thursday's release window.",
	"Reproduced locally. The failure only happens when %s is enabled. Adding a regression test now.",
	"Updated the PR based on review feedback. Changed the approach to use %s instead — this avoids the locking issues we saw in the previous attempt.",
	"Sync'd with the product team — they're okay with the simplified scope for now. We can revisit the full feature set in the next sprint.",
	"Added comprehensive unit tests covering the happy path and three edge cases. Coverage for this module is now at 87%%.",
	"Performance numbers after the fix: p95 latency down from 340ms to 42ms. The bottleneck was exactly where we suspected.",
	"Flagging a scope creep risk here — the original estimate did not account for the %s requirement. May need to split into a follow-up ticket.",
}

var commentFillins = []string{
	"authentication", "data access", "validation", "caching", "rendering",
	"API", "database", "frontend", "backend", "storage",
	"session handling", "rate limiting", "WebSocket", "notification",
	"permission", "migration", "deployment", "monitoring", "logging",
}

// persona pool
type persona struct {
	username    string
	email       string
	displayName string
}

var demoDomains = []string{"blowpipe.xyz", "brzzt.com"}

var personaPool = []persona{
	{"schan", "sarah.chen@blowpipe.xyz", "Sarah Chen"},
	{"mwright", "marcus.wright@brzzt.com", "Marcus Wright"},
	{"ppatel", "priya.patel@blowpipe.xyz", "Priya Patel"},
	{"jokoye", "james.okoye@brzzt.com", "James Okoye"},
	{"lnakamura", "lisa.nakamura@blowpipe.xyz", "Lisa Nakamura"},
	{"tharrison", "tom.harrison@brzzt.com", "Tom Harrison"},
	{"evasquez", "elena.vasquez@blowpipe.xyz", "Elena Vasquez"},
	{"rcooper", "ryan.cooper@brzzt.com", "Ryan Cooper"},
}

type projectDef struct {
	prefix      string
	title       string
	description string
	pool        string // "frontend", "backend", "infra", "mixed"
}

var projectPool = []projectDef{
	{
		prefix:      "TDA",
		title:       "TodoApp Web",
		description: "React/TypeScript frontend application providing the user interface for the TodoApp platform. Includes task management, project views, real-time updates, and responsive design for mobile and desktop.",
		pool:        "frontend",
	},
	{
		prefix:      "API",
		title:       "TodoApp API",
		description: "Go-based REST API server powering the TodoApp backend. Handles authentication, task CRUD operations, team management, webhook delivery, and real-time notifications via WebSockets.",
		pool:        "backend",
	},
	{
		prefix:      "INF",
		title:       "TodoApp Infrastructure",
		description: "Kubernetes, CI/CD pipelines, PostgreSQL, Redis, and observability stack. Manages deployment, scaling, secrets, backups, and monitoring for the entire TodoApp platform.",
		pool:        "infra",
	},
	{
		prefix:      "MOB",
		title:       "TodoApp Mobile",
		description: "React Native mobile application for iOS and Android. Shares business logic with the web frontend while providing native navigation, push notifications, and offline support.",
		pool:        "frontend",
	},
	{
		prefix:      "SDK",
		title:       "TodoApp SDK",
		description: "TypeScript client SDK for third-party integrations. Provides typed wrappers around the REST API, webhook handling utilities, and example integrations.",
		pool:        "frontend",
	},
}

func runDemo(args []string) error {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	dbPath := fs.String("db", "./demo.db", "database file path")
	n := fs.Int("n", 500, "approximate total item count")
	force := fs.Bool("force", false, "delete existing database before seeding")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Scale model
	numTickets := int(float64(*n) * 0.65)
	numComments := int(float64(*n) * 0.25)
	numTimeEntries := int(float64(*n) * 0.10)
	numUsers := clamp(*n/50, 5, 100)
	numTeams := clamp(*n/200, 2, 30)
	numProjects := clamp(*n/200, 2, 20)

	fmt.Printf("Creating demo database at %s (~%d items)...\n", *dbPath, *n)

	// Handle existing DB
	if *dbPath != ":memory:" {
		if _, err := os.Stat(*dbPath); err == nil {
			if !*force {
				return fmt.Errorf("database already exists at %s (use --force to overwrite)", *dbPath)
			}
			if err := os.Remove(*dbPath); err != nil {
				return fmt.Errorf("removing existing database: %w", err)
			}
		}
	}

	// Initialize DB
	if err := store.Init(*dbPath, "admin", "password"); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	ctx := context.Background()

	fmt.Println("  ✓ Initialized database")

	// Remove auto-generated private and public placeholder projects from store.Init
	// (admin private workspace, bootstrap ticket tracker project, public placeholder)
	if _, execErr := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); execErr != nil {
		return fmt.Errorf("disabling fk for cleanup: %w", execErr)
	}
	if _, execErr := db.ExecContext(ctx, `DELETE FROM projects WHERE visibility = 'private' OR (prefix = 'PUB' AND title = 'Public')`); execErr != nil {
		return fmt.Errorf("removing placeholder projects: %w", execErr)
	}
	if _, execErr := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); execErr != nil {
		return fmt.Errorf("re-enabling fk after cleanup: %w", execErr)
	}

	// Get admin user
	adminUser, err := store.GetUserByUsername(ctx, db, "admin")
	if err != nil {
		return fmt.Errorf("getting admin user: %w", err)
	}

	// Set organisation name
	if _, orgErr := store.UpdateOrg(ctx, db, "Acme Corp", "acme.example.com", "Acme Corporation — demo organisation", ""); orgErr != nil {
		return fmt.Errorf("setting org: %w", orgErr)
	}
	fmt.Printf("  ✓ Organisation set to %q\n", "Acme Corp")

	// Seed built-in Agile workflow and roles
	err = static.SeedDatabase(ctx, db)
	if err != nil {
		return fmt.Errorf("seeding database with Agile workflow: %w", err)
	}

	// Look up the Agile workflow by name
	var agileWorkflowID int64
	err = db.QueryRowContext(ctx, `SELECT workflow_id FROM workflows WHERE name = 'Agile' LIMIT 1`).Scan(&agileWorkflowID)
	if err != nil {
		return fmt.Errorf("looking up Agile workflow: %w", err)
	}

	// Mark the design stage as the backlog stage
	var designStageID int64
	err = db.QueryRowContext(ctx, `SELECT workflow_stage_id FROM workflow_stages WHERE workflow_id = ? AND stage_name = 'design' LIMIT 1`, agileWorkflowID).Scan(&designStageID)
	if err != nil {
		return fmt.Errorf("looking up design stage: %w", err)
	}
	err = store.SetWorkflowStageBacklog(ctx, db, designStageID, true)
	if err != nil {
		return fmt.Errorf("setting backlog flag for design stage: %w", err)
	}

	fmt.Printf("  ✓ Seeded Agile workflow (design, develop, test, done)\n")

	// Create users
	users := make([]store.User, 0, numUsers)

	// Create persona users (up to 8 from the pool, then synthetic)
	for i := 0; i < numUsers; i++ {
		var u store.User
		var createErr error
		if i < len(personaPool) {
			p := personaPool[i]
			u, createErr = store.CreateUserWithParams(ctx, db, store.UserCreateParams{
				Username:               p.username,
				Email:                  p.email,
				PlainPassword:          "password",
				Role:                   "user",
				Enabled:                true,
				SkipPasswordValidation: true,
				SkipProvisioning:       true,
			})
			if createErr != nil {
				return fmt.Errorf("creating user %s: %w", p.username, createErr)
			}
			// Update display name via direct SQL
			if _, sqlErr := db.ExecContext(ctx, `UPDATE users SET display_name = ? WHERE user_id = ?`, p.displayName, u.ID); sqlErr != nil {
				return fmt.Errorf("updating display name for %s: %w", p.username, sqlErr)
			}
			u.DisplayName = p.displayName
		} else {
			n := i - len(personaPool) + 1
			username := fmt.Sprintf("ddev%02d", n)
			domain := demoDomains[i%len(demoDomains)]
			email := fmt.Sprintf("dev%02d@%s", n, domain)
			u, createErr = store.CreateUserWithParams(ctx, db, store.UserCreateParams{
				Username:               username,
				Email:                  email,
				PlainPassword:          "password",
				Role:                   "user",
				Enabled:                true,
				SkipPasswordValidation: true,
				SkipProvisioning:       true,
			})
			if createErr != nil {
				return fmt.Errorf("creating user %s: %w", username, createErr)
			}
		}
		users = append(users, u)
	}

	fmt.Printf("  ✓ Created %d users\n", len(users))

	// Create 4 global demo agents — each with password "password".
	type demoAgent struct {
		username string
		role     string
		id       string
	}
	// Role values are the workflow role TITLES (matched case-insensitively by
	// name against each ticket's current role), not slugs.
	demoAgents := []demoAgent{
		{username: "po-agent", role: "Product Owner"},
		{username: "ba-agent", role: "Business Analyst"},
		{username: "eng-agent", role: "Engineer"},
		{username: "qa-agent", role: "QA Engineer"},
		{username: "refiner-agent", role: "refiner"},
	}
	for i, da := range demoAgents {
		agent, _, agentErr := store.CreateAgent(ctx, db, "password")
		if agentErr != nil {
			return fmt.Errorf("creating agent %s: %w", da.username, agentErr)
		}
		demoAgents[i].id = agent.ID
		displayName := da.username + " agent"
		// Rename to friendly username and set agent_role.
		if _, sqlErr := db.ExecContext(ctx, `UPDATE users SET username = ?, display_name = ?, agent_role = ? WHERE user_id = ?`,
			da.username, displayName, da.role, agent.ID); sqlErr != nil {
			return fmt.Errorf("updating agent username for %s: %w", da.username, sqlErr)
		}
	}
	fmt.Printf("  ✓ Created global demo agents (po-agent, ba-agent, eng-agent, qa-agent) — password: password\n")

	// Create teams
	engTeam, err := store.CreateTeam(ctx, db, "Engineering", nil)
	if err != nil {
		return fmt.Errorf("creating Engineering team: %w", err)
	}
	// Add engineering members: sarah, marcus, priya, elena, ryan (indices 0,1,2,6,7)
	engIndices := []int{0, 1, 2, 6, 7}
	for _, idx := range engIndices {
		if idx < len(users) {
			// ignore duplicate errors
			_, _ = store.AddTeamMember(ctx, db, engTeam.ID, users[idx].ID, "member", "")
		}
	}
	// Add synthetic devs to engineering
	for i := len(personaPool); i < len(users); i++ {
		_, _ = store.AddTeamMember(ctx, db, engTeam.ID, users[i].ID, "member", "")
	}

	pqTeam, err := store.CreateTeam(ctx, db, "Product & QA", nil)
	if err != nil {
		return fmt.Errorf("creating Product & QA team: %w", err)
	}
	// tom=5, lisa=4
	for _, idx := range []int{4, 5} {
		if idx < len(users) {
			_, _ = store.AddTeamMember(ctx, db, pqTeam.ID, users[idx].ID, "member", "")
		}
	}

	teamsCreated := 2
	var devopsTeam store.Team
	if numTeams > 2 {
		devopsTeam, err = store.CreateTeam(ctx, db, "DevOps & Infrastructure", nil)
		if err != nil {
			return fmt.Errorf("creating DevOps team: %w", err)
		}
		// james=3
		if 3 < len(users) {
			_, _ = store.AddTeamMember(ctx, db, devopsTeam.ID, users[3].ID, "member", "")
		}
		teamsCreated = 3
	}
	_ = devopsTeam

	fmt.Printf("  ✓ Created %d teams\n", teamsCreated)

	// Create projects
	projects := make([]store.Project, 0, numProjects)
	for i := 0; i < numProjects; i++ {
		var def projectDef
		if i < len(projectPool) {
			def = projectPool[i]
		} else {
			def = projectDef{
				prefix:      fmt.Sprintf("M%02d", i+1),
				title:       fmt.Sprintf("Module %d", i+1),
				description: fmt.Sprintf("Additional module %d for the TodoApp platform.", i+1),
				pool:        "mixed",
			}
		}
		proj, projErr := store.CreateProjectWithParams(ctx, db, store.ProjectCreateParams{
			Prefix:      def.prefix,
			Title:       def.title,
			Description: def.description,
			CreatedBy:   adminUser.ID,
			WorkflowID:  &agileWorkflowID,
		})
		if projErr != nil {
			return fmt.Errorf("creating project %s: %w", def.title, projErr)
		}
		projects = append(projects, proj)
	}

	fmt.Printf("  ✓ Created %d projects\n", len(projects))

	// Create programme and assign all projects to it.
	programme, err := store.CreateProgramme(ctx, db, "TODO Application", "Core delivery programme for the TODO Application platform")
	if err != nil {
		return fmt.Errorf("creating programme: %w", err)
	}
	for _, proj := range projects {
		if err := store.SetProjectProgramme(ctx, db, proj.ID, &programme.ID); err != nil {
			return fmt.Errorf("assigning project %s to programme: %w", proj.Title, err)
		}
	}
	fmt.Printf("  ✓ Created programme %q with %d projects\n", programme.Name, len(projects))

	// ── Releases → Features → Epics → Stories ────────────────────────────────
	// Sprints are gone. Work is organised as Releases (delivery containers) that
	// hold Features (the "grand plan" / requirement), which break down into Epics,
	// which break down into Stories/Bugs.
	now := time.Now().UTC()
	sqlTS := func(t time.Time) string { return t.Format("2006-01-02 15:04:05") }

	type projectMeta struct {
		project store.Project
		pool    string
	}
	projectData := make([]projectMeta, len(projects))
	for pi, proj := range projects {
		pool := "mixed"
		if pi < len(projectPool) {
			pool = projectPool[pi].pool
		}
		projectData[pi] = projectMeta{project: proj, pool: pool}
	}

	// setTicketFields applies seed-only stage/state/draft/release/timestamps via
	// direct SQL, bypassing the lifecycle advance guards.
	setTicketFields := func(id, stage, state string, draft bool, created time.Time) error {
		d := 0
		if draft {
			d = 1
		}
		ts := sqlTS(created)
		_, err := db.ExecContext(ctx, `UPDATE tickets SET stage = ?, state = ?, status = ?, draft = ?, created_at = ?, updated_at = ? WHERE ticket_id = ?`,
			stage, state, stage+"/"+state, d, ts, ts, id)
		return err
	}

	titlesFor := func(pool string) []string {
		switch pool {
		case "frontend":
			return frontendTitles
		case "backend":
			return backendTitles
		case "infra":
			return infraTitles
		default:
			out := append([]string{}, frontendTitles...)
			out = append(out, backendTitles...)
			return out
		}
	}

	// Release blueprints applied to every project.
	type relSpec struct {
		title   string
		purpose string
		status  string
		target  time.Time
		// storyStage returns (stage, state) for a story by index.
		storyStage func(i int) (string, string)
		draftFeat  bool // features still being refined?
	}
	relSpecs := []relSpec{
		{
			title: "1.0 — Foundation", purpose: "Establish the core product: the must-have capabilities for launch.",
			status: store.ReleaseComplete, target: now.AddDate(0, 0, -20),
			storyStage: func(i int) (string, string) { return "done", store.StateSuccess },
		},
		{
			title: "1.1 — Growth", purpose: "Round out the experience and harden what shipped in 1.0.",
			status: store.ReleaseInProgress, target: now.AddDate(0, 0, 14),
			storyStage: func(i int) (string, string) {
				switch i % 5 {
				case 0, 1:
					return "done", store.StateSuccess
				case 2:
					return "test", store.StateActive
				case 3:
					return "develop", store.StateActive
				default:
					return "develop", store.StateIdle
				}
			},
		},
		{
			title: "2.0 — Horizon", purpose: "The next big bet — still being shaped with the Product Owner.",
			status: store.ReleaseInDesign, target: now.AddDate(0, 1, 0),
			storyStage: func(i int) (string, string) { return "design", store.StateIdle },
			draftFeat:  true,
		},
	}

	featCount, epicCount, storyCount, relCount := 0, 0, 0, 0
	ti := 0

	// makeFeature builds one feature + its epics + stories, returns the feature ticket ID.
	makeFeature := func(pd projectMeta, featTitle, featDesc string, spec *relSpec, created time.Time) (string, error) {
		pool := titlesFor(pd.pool)
		feat, err := store.CreateTicket(ctx, db, store.TicketCreateParams{
			ProjectID: pd.project.ID, Type: "feature", Title: featTitle, Description: featDesc,
			Priority: (ti % 5) + 1, Author: adminUser.Username, CreatedBy: adminUser.ID, State: "idle",
		})
		if err != nil {
			return "", err
		}
		ti++
		featCount++
		featDraft := spec != nil && spec.draftFeat
		featStage, featState := "design", store.StateIdle
		if spec != nil && spec.status == store.ReleaseComplete {
			featStage, featState = "done", store.StateSuccess
		}
		if err := setTicketFields(feat.ID, featStage, featState, featDraft, created); err != nil {
			return "", err
		}
		for e := 0; e < 2; e++ {
			epicTitle := "Epic: " + pool[ti%len(pool)]
			fid := feat.ID
			epic, eErr := store.CreateTicket(ctx, db, store.TicketCreateParams{
				ProjectID: pd.project.ID, ParentID: &fid, Type: "epic",
				Title: epicTitle, Description: descTemplates[ti%len(descTemplates)],
				Priority: (ti % 5) + 1, Author: adminUser.Username, CreatedBy: adminUser.ID, State: "idle",
			})
			if eErr != nil {
				return "", eErr
			}
			ti++
			epicCount++
			_ = setTicketFields(epic.ID, "design", store.StateIdle, false, created)
			for s := 0; s < 3; s++ {
				stType := "story"
				stTitle := pool[(ti+1)%len(pool)]
				switch ti % 4 {
				case 3:
					stType = "bug"
					stTitle = bugTitles[ti%len(bugTitles)]
				case 2:
					stType = "chore"
					stTitle = choreTitles[ti%len(choreTitles)]
				}
				eid := epic.ID
				st, sErr := store.CreateTicket(ctx, db, store.TicketCreateParams{
					ProjectID: pd.project.ID, ParentID: &eid, Type: stType,
					Title: stTitle, Description: descTemplates[ti%len(descTemplates)],
					Priority: (ti % 5) + 1, Author: adminUser.Username, CreatedBy: adminUser.ID, State: "idle",
				})
				if sErr != nil {
					return "", sErr
				}
				stStage, stState := "design", store.StateIdle
				stDraft := false
				if spec != nil {
					stStage, stState = spec.storyStage(ti)
					stDraft = spec.draftFeat
				} else {
					stDraft = true // backlog features are still being refined
				}
				if err := setTicketFields(st.ID, stStage, stState, stDraft, created); err != nil {
					return "", err
				}
				ti++
				storyCount++
			}
		}
		return feat.ID, nil
	}

	fmt.Printf("  Building release/feature/epic/story hierarchy...")
	for _, pd := range projectData {
		// Releases with their features.
		for ri := range relSpecs {
			spec := relSpecs[ri]
			rel, err := store.CreateRelease(ctx, db, int(pd.project.ID), spec.title, spec.purpose, spec.target.Format("2006-01-02"))
			if err != nil {
				return fmt.Errorf("creating release: %w", err)
			}
			relCount++
			created := now.AddDate(0, 0, -10)
			if spec.status == store.ReleaseComplete {
				created = now.AddDate(0, 0, -40)
			}
			featureIDs := make([]string, 0, 2)
			for f := 0; f < 2; f++ {
				ftitle := "Feature: " + titlesFor(pd.pool)[ti%len(titlesFor(pd.pool))]
				fdesc := "Grand plan — " + descTemplates[ti%len(descTemplates)]
				fid, fErr := makeFeature(pd, ftitle, fdesc, &spec, created)
				if fErr != nil {
					return fErr
				}
				featureIDs = append(featureIDs, fid)
			}
			// Attach features to the release while it is in_design, then move the
			// release to its target status.
			for _, fid := range featureIDs {
				if err := store.AssignFeatureToRelease(ctx, db, fid, rel.ID); err != nil {
					return fmt.Errorf("assigning feature to release: %w", err)
				}
			}
			if spec.status != store.ReleaseInDesign {
				if _, err := store.SetReleaseStatus(ctx, db, rel.ID, spec.status); err != nil {
					return fmt.Errorf("setting release status: %w", err)
				}
			}
		}
		// Backlog features (not in any release) — still being refined.
		for b := 0; b < 2; b++ {
			btitle := "Feature: " + titlesFor(pd.pool)[(ti+3)%len(titlesFor(pd.pool))]
			if _, err := makeFeature(pd, btitle, "Backlog idea awaiting refinement.", nil, now.AddDate(0, 0, -2)); err != nil {
				return err
			}
		}
	}
	fmt.Println(" done")
	fmt.Printf("  ✓ Seeded %d releases, %d features, %d epics, %d stories/bugs\n", relCount, featCount, epicCount, storyCount)

	// A couple of standalone refine ideas so the orchestrator's preparation loop
	// has draft work to chew on.
	if len(projects) > 0 {
		refineIdeas := []struct{ title, desc string }{
			{"Let users export their data as a portable archive", "A user asked to download everything they've created. Scope is unclear: which entities, which formats, how big."},
			{"Support single sign-on for enterprise customers", "Large prospect needs SSO. Which protocols and providers are in scope is undecided."},
		}
		for _, idea := range refineIdeas {
			if _, err := store.CreateTicket(ctx, db, store.TicketCreateParams{
				ProjectID: projects[0].ID, Type: "idea", Title: idea.title, Description: idea.desc,
				Author: adminUser.Username, CreatedBy: adminUser.ID, State: "idle",
			}); err != nil {
				return fmt.Errorf("creating refine idea: %w", err)
			}
		}
		fmt.Printf("  ✓ Seeded %d backlog ideas in refinement\n", len(refineIdeas))
	}

	// Create comments
	fmt.Printf("  Creating %d comments...", numComments)

	// Collect ticket IDs that can receive comments (not complete/archived)
	// We'll just attempt to add comments and skip failures (ticket closed)
	commentIdx := 0
	for commentIdx < numComments {
		// Pick a random ticket by index
		ticketLocalIdx := rand.Intn(numTickets) // #nosec G404 -- demo data seeding, not security-sensitive
		// Determine project and local ticket index
		pi := ticketLocalIdx % len(projects)
		pd := projectData[pi]

		// We'll pick a comment template
		tmpl := commentTemplates[commentIdx%len(commentTemplates)]
		fillIn := commentFillins[commentIdx%len(commentFillins)]
		var commentText string
		verbCount := strings.Count(tmpl, "%")
		switch {
		case verbCount > 0 && strings.Contains(tmpl, "%s"):
			commentText = fmt.Sprintf(tmpl, fillIn)
		case strings.Contains(tmpl, "%%"):
			commentText = fmt.Sprintf(tmpl, fillIn)
		default:
			commentText = tmpl
		}
		_ = pd

		commenter := users[commentIdx%len(users)]

		// Build a synthetic ticket ID to attempt — we'll iterate through what we have
		// Since we can't easily get all ticket IDs, we use the project + offset approach.
		// Try to get tickets for a project
		listParams := store.TicketListParams{
			ProjectID: projectData[pi].project.ID,
			Limit:     1,
			Offset:    ticketLocalIdx / len(projects),
		}
		tickets, _ := store.ListTickets(ctx, db, listParams)
		if len(tickets) > 0 {
			_, _ = store.AddComment(ctx, db, tickets[0].ID, commenter.ID, commentText)
		}
		commentIdx++

		if commentIdx%1000 == 0 {
			fmt.Printf(" [%d/%d]", commentIdx, numComments)
		}
	}
	fmt.Println(" done")

	// Create time entries
	fmt.Printf("  Creating %d time entries...", numTimeEntries)

	minuteOptions := []int{30, 60, 90, 120, 240}
	noteOptions := []string{"Implementation work", "Code review", "Testing", "Bug investigation", "Deployment"}

	for teIdx := 0; teIdx < numTimeEntries; teIdx++ {
		pi := teIdx % len(projects)
		listParams := store.TicketListParams{
			ProjectID: projectData[pi].project.ID,
			Limit:     1,
			Offset:    teIdx / len(projects),
		}
		tickets, _ := store.ListTickets(ctx, db, listParams)
		if len(tickets) > 0 {
			user := users[teIdx%len(users)]
			minutes := minuteOptions[teIdx%len(minuteOptions)]
			note := noteOptions[teIdx%len(noteOptions)]
			_, _ = store.LogTime(ctx, db, tickets[0].ID, user.ID, minutes, note)
		}

		if teIdx%1000 == 0 && teIdx > 0 {
			fmt.Printf(" [%d/%d]", teIdx, numTimeEntries)
		}
	}
	fmt.Println(" done")

	// Print summary
	fmt.Printf("\nDemo database ready: %s\n", *dbPath)
	fmt.Printf("  Login:   admin / password\n")
	fmt.Printf("  Server:  tk server -f %s (then open http://localhost:8080)\n", *dbPath)

	// Build user list for display
	usernames := make([]string, 0, len(users))
	for i, u := range users {
		if i >= 5 {
			break
		}
		usernames = append(usernames, u.Username)
	}
	suffix := ""
	if len(users) > 5 {
		suffix = ", ..."
	}
	fmt.Printf("  Users:   %s%s (password: password)\n", strings.Join(usernames, ", "), suffix)
	fmt.Printf("  Agents:  po-agent, ba-agent, eng-agent, qa-agent (password: password)\n")
	fmt.Printf("           Run: TICKET_URL=http://localhost:8080 AGENT_ID=<uuid> AGENT_PASSWORD=password tk agent run\n")

	return nil
}
