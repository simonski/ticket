package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

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
	numSprintsPerProj := 6 // always: sprints 1-4 closed, 5 active (current), 6 design

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
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disabling fk for cleanup: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM projects WHERE visibility = 'private' OR (prefix = 'PUB' AND title = 'Public')`); err != nil {
		return fmt.Errorf("removing placeholder projects: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("re-enabling fk after cleanup: %w", err)
	}

	// Get admin user
	adminUser, err := store.GetUserByUsername(ctx, db, "admin")
	if err != nil {
		return fmt.Errorf("getting admin user: %w", err)
	}

	// Create workflow
	wf, err := store.CreateWorkflow(ctx, db, "Product Development",
		"Standard software development lifecycle from discovery to delivery")
	if err != nil {
		return fmt.Errorf("creating workflow: %w", err)
	}

	stages := []struct {
		name string
		desc string
	}{
		{"discovery", "Explore the problem, write user stories, define acceptance criteria"},
		{"design", "Architect the solution, create technical design documents"},
		{"develop", "Implement the feature or fix"},
		{"review", "Code review, QA testing, and stakeholder sign-off"},
		{"done", "Shipped to production"},
	}

	createdStages := make([]store.WorkflowStage, 0, len(stages))
	for i, s := range stages {
		ws, err := store.AddWorkflowStage(ctx, db, wf.ID, s.name, s.desc, "", i)
		if err != nil {
			return fmt.Errorf("adding workflow stage %s: %w", s.name, err)
		}
		createdStages = append(createdStages, ws)
	}

	// Add linear transitions: discovery→design, design→develop, develop→review, review→done
	for i := 0; i < len(createdStages)-1; i++ {
		if err := store.SetWorkflowStageTransitions(ctx, db, wf.ID, createdStages[i].ID, []int64{createdStages[i+1].ID}); err != nil {
			return fmt.Errorf("setting workflow transitions: %w", err)
		}
	}

	wfFull, err := store.GetWorkflow(ctx, db, wf.ID)
	if err != nil {
		return fmt.Errorf("getting workflow: %w", err)
	}
	_ = wfFull

	fmt.Printf("  ✓ Created workflow %q (%d stages)\n", wf.Name, len(stages))

	// Create users
	users := make([]store.User, 0, numUsers)

	// Create persona users (up to 8 from the pool, then synthetic)
	for i := 0; i < numUsers; i++ {
		var u store.User
		var err error
		if i < len(personaPool) {
			p := personaPool[i]
			u, err = store.CreateUserWithParams(ctx, db, store.UserCreateParams{
				Username:               p.username,
				Email:                  p.email,
				PlainPassword:          "password",
				Role:                   "user",
				Enabled:                true,
				SkipPasswordValidation: true,
				SkipProvisioning:       true,
			})
			if err != nil {
				return fmt.Errorf("creating user %s: %w", p.username, err)
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
			u, err = store.CreateUserWithParams(ctx, db, store.UserCreateParams{
				Username:               username,
				Email:                  email,
				PlainPassword:          "password",
				Role:                   "user",
				Enabled:                true,
				SkipPasswordValidation: true,
				SkipProvisioning:       true,
			})
			if err != nil {
				return fmt.Errorf("creating user %s: %w", username, err)
			}
		}
		users = append(users, u)
	}

	fmt.Printf("  ✓ Created %d users\n", len(users))

	// Create teams
	engTeam, err := store.CreateTeam(ctx, db, "Engineering", nil)
	if err != nil {
		return fmt.Errorf("creating Engineering team: %w", err)
	}
	// Add engineering members: sarah, marcus, priya, elena, ryan (indices 0,1,2,6,7)
	engIndices := []int{0, 1, 2, 6, 7}
	for _, idx := range engIndices {
		if idx < len(users) {
			if _, err := store.AddTeamMember(ctx, db, engTeam.ID, users[idx].ID, "member", ""); err != nil {
				// ignore duplicate errors
				_ = err
			}
		}
	}
	// Add synthetic devs to engineering
	for i := len(personaPool); i < len(users); i++ {
		if _, err := store.AddTeamMember(ctx, db, engTeam.ID, users[i].ID, "member", ""); err != nil {
			_ = err
		}
	}

	pqTeam, err := store.CreateTeam(ctx, db, "Product & QA", nil)
	if err != nil {
		return fmt.Errorf("creating Product & QA team: %w", err)
	}
	// tom=5, lisa=4
	for _, idx := range []int{4, 5} {
		if idx < len(users) {
			if _, err := store.AddTeamMember(ctx, db, pqTeam.ID, users[idx].ID, "member", ""); err != nil {
				_ = err
			}
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
			if _, err := store.AddTeamMember(ctx, db, devopsTeam.ID, users[3].ID, "member", ""); err != nil {
				_ = err
			}
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
		proj, err := store.CreateProjectWithParams(ctx, db, store.ProjectCreateParams{
			Prefix:      def.prefix,
			Title:       def.title,
			Description: def.description,
			CreatedBy:   adminUser.ID,
			WorkflowID:  &wf.ID,
		})
		if err != nil {
			return fmt.Errorf("creating project %s: %w", def.title, err)
		}
		projects = append(projects, proj)
	}

	fmt.Printf("  ✓ Created %d projects\n", len(projects))

	// Sprint timeline: 6 sprints, 2 weeks each, today = mid-sprint 5.
	// Sprint 5 (index 4) started 7 days ago; each prior sprint is 14 days earlier.
	now := time.Now().UTC()
	sprintLen := 14 * 24 * time.Hour
	activeSprintIdx := numSprintsPerProj - 2 // index 4 for 6 sprints
	midSprintOffset := sprintLen / 2          // 7 days

	sqlTS := func(t time.Time) string { return t.Format("2006-01-02 15:04:05") }

	// sprintStart(si) returns the start of sprint at 0-based index si.
	sprintStart := func(si int) time.Time {
		return now.Add(-midSprintOffset - time.Duration(activeSprintIdx-si)*sprintLen)
	}
	sprintEnd := func(si int) time.Time { return sprintStart(si).Add(sprintLen) }

	// Create sprints per project
	type projectSprints struct {
		project    store.Project
		sprints    []store.Sprint
		pool       string
		sprintIdxs []int // 0-based sprint index for each sprint
	}
	projectData := make([]projectSprints, len(projects))
	totalSprints := 0

	for pi, proj := range projects {
		pool := "mixed"
		if pi < len(projectPool) {
			pool = projectPool[pi].pool
		}
		sprints := make([]store.Sprint, 0, numSprintsPerProj)
		sprintIdxs := make([]int, 0, numSprintsPerProj)
		for si := 0; si < numSprintsPerProj; si++ {
			sp, err := store.CreateSprint(ctx, db, int(proj.ID), "")
			if err != nil {
				return fmt.Errorf("creating sprint %d for project %s: %w", si+1, proj.Title, err)
			}
			var stage string
			switch {
			case si < activeSprintIdx:
				stage = "closed"
			case si == activeSprintIdx:
				stage = "active"
			default:
				stage = "design"
			}
			sp, err = store.UpdateSprint(ctx, db, sp.ID, sp.Title, stage)
			if err != nil {
				return fmt.Errorf("updating sprint stage: %w", err)
			}
			// Backdate sprint created_at to sprint start
			ts := sqlTS(sprintStart(si))
			if _, err := db.ExecContext(ctx, `UPDATE sprints SET created_at = ?, updated_at = ? WHERE id = ?`, ts, ts, sp.ID); err != nil {
				return fmt.Errorf("backdating sprint: %w", err)
			}
			sprints = append(sprints, sp)
			sprintIdxs = append(sprintIdxs, si)
		}
		projectData[pi] = projectSprints{project: proj, sprints: sprints, pool: pool, sprintIdxs: sprintIdxs}
		totalSprints += len(sprints)
	}

	fmt.Printf("  ✓ Created %d sprints\n", totalSprints)

	// Create tickets
	fmt.Printf("  Creating %d tickets...", numTickets)

	// Distribute tickets across projects
	ticketsPerProject := numTickets / len(projects)
	extraTickets := numTickets % len(projects)

	// Track sprint assignment for backdating: map ticketID → sprint 0-based index (-1 = backlog)
	type ticketMeta struct {
		id        string
		sprintIdx int // -1 = backlog
		stage     string
	}
	allTicketMeta := make([]ticketMeta, 0, numTickets)

	ticketIndex := 0

	// sprint5AssignCount tracks how many tickets have been routed to sprint 5 so we can
	// send every other one to the backlog instead.
	sprint5AssignCount := 0

	for pi, pd := range projectData {
		count := ticketsPerProject
		if pi < extraTickets {
			count++
		}

		// Pick title pool for this project
		var titlePool []string
		switch pd.pool {
		case "frontend":
			titlePool = frontendTitles
		case "backend":
			titlePool = backendTitles
		case "infra":
			titlePool = infraTitles
		default:
			titlePool = append(titlePool, frontendTitles...)
			titlePool = append(titlePool, backendTitles...)
			titlePool = append(titlePool, bugTitles...)
		}

		// Each project round-robins through all its sprints.
		sprintCursor := 0

		for ti := 0; ti < count; ti++ {
			localIdx := ticketIndex

			// Pick type
			mod20 := localIdx % 20
			var ticketType string
			switch {
			case mod20 < 10:
				ticketType = "task"
			case mod20 < 15:
				ticketType = "bug"
			case mod20 < 18:
				ticketType = "chore"
			default:
				ticketType = "epic"
			}

			// Pick title
			var title string
			switch ticketType {
			case "bug":
				title = bugTitles[localIdx%len(bugTitles)]
			case "chore":
				title = choreTitles[localIdx%len(choreTitles)]
			default:
				title = titlePool[localIdx%len(titlePool)]
			}

			// Build description
			descTmpl := descTemplates[localIdx%len(descTemplates)]
			words := strings.Fields(title)
			word1 := "feature"
			if len(words) > 0 {
				word1 = words[0]
			}
			word2 := pd.project.Title
			var description string
			verbCount := strings.Count(descTmpl, "%s")
			switch verbCount {
			case 0:
				description = descTmpl
			case 1:
				description = fmt.Sprintf(descTmpl, word1)
			default:
				description = fmt.Sprintf(descTmpl, word1, word2)
			}

			assignee := users[localIdx%len(users)].Username
			author := users[(localIdx+1)%len(users)].Username
			priority := (localIdx%5) + 1

			// Determine which sprint this ticket belongs to (round-robin across all sprints).
			targetSprintLocalIdx := sprintCursor % len(pd.sprints)
			targetSprintGlobalIdx := pd.sprintIdxs[targetSprintLocalIdx]
			sprintCursor++

			// Determine stage/state based on which sprint the ticket is in.
			var stage, state string
			isBacklog := false
			if targetSprintGlobalIdx == activeSprintIdx {
				// Sprint 5 (active): send every other ticket to the backlog instead.
				if sprint5AssignCount%2 == 1 {
					isBacklog = true
					stage, state = "design", "idle" // not yet started but designed
				} else {
					// In-sprint: tickets are past discovery (ready for dev)
					switch localIdx % 10 {
					case 0, 1:
						stage, state = "design", "idle"
					case 2, 3, 4:
						stage, state = "develop", "idle"
					case 5, 6, 7:
						stage, state = "develop", "active"
					case 8:
						stage, state = "review", "idle"
					default:
						stage, state = "review", "active"
					}
				}
				sprint5AssignCount++
			} else if targetSprintGlobalIdx > activeSprintIdx {
				// Future design sprint: tickets are in design stage with various states
				stage = "design"
				switch localIdx % 3 {
				case 0:
					state = "idle"
				case 1:
					state = "active"
				default:
					state = "idle"
				}
				// ~30% have no assignee yet
				if localIdx%3 == 2 {
					assignee = ""
				}
			} else {
				// Past (closed) sprint: most tickets done, some in various stages
				switch localIdx % 20 {
				case 0, 1, 2:
					stage, state = "discovery", "idle"
				case 3, 4, 5:
					stage, state = "design", "idle"
				case 6, 7, 8, 9:
					stage, state = "develop", "idle"
				case 10, 11, 12, 13:
					stage, state = "develop", "active"
				case 14, 15, 16:
					stage, state = "review", "idle"
				case 17:
					stage, state = "review", "active"
				default: // 18, 19 — 10% done
					stage, state = "done", "success"
				}
			}

			t, err := store.CreateTicket(ctx, db, store.TicketCreateParams{
				ProjectID:   pd.project.ID,
				Type:        ticketType,
				Title:       title,
				Description: description,
				Priority:    priority,
				Assignee:    assignee,
				Author:      author,
				CreatedBy:   adminUser.ID,
				State:       "idle",
			})
			if err != nil {
				return fmt.Errorf("creating ticket %d: %w", localIdx, err)
			}

			if stage != "discovery" || state != "idle" {
				_, _ = store.UpdateTicket(ctx, db, t.ID, store.TicketUpdateParams{
					Title:       t.Title,
					Description: t.Description,
					Stage:       stage,
					State:       state,
					Priority:    t.Priority,
					Assignee:    t.Assignee,
					UpdatedBy:   adminUser.Username,
				})
			}

			if !isBacklog {
				// Direct SQL to bypass the closed-sprint guard (valid for seeding only).
				spID := pd.sprints[targetSprintLocalIdx].ID
				if _, err := db.ExecContext(ctx, `UPDATE tickets SET sprint_id = ? WHERE ticket_id = ?`, spID, t.ID); err != nil {
					return fmt.Errorf("assigning sprint to ticket %s: %w", t.ID, err)
				}
			}

			allTicketMeta = append(allTicketMeta, ticketMeta{
				id:        t.ID,
				sprintIdx: func() int { if isBacklog { return -1 }; return targetSprintGlobalIdx }(),
				stage:     stage,
			})

			ticketIndex++
			if ticketIndex%1000 == 0 {
				fmt.Printf(" [%d/%d]", ticketIndex, numTickets)
			}
		}
	}
	fmt.Println(" done")

	// Backdate ticket timestamps based on their sprint assignment.
	fmt.Printf("  Backdating %d tickets...", len(allTicketMeta))
	for _, tm := range allTicketMeta {
		var createdAt, updatedAt string
		if tm.sprintIdx == -1 {
			// Backlog: created in the last 3 days
			offset := time.Duration(rand.Intn(3*24*60)) * time.Minute
			createdAt = sqlTS(now.Add(-offset))
			updatedAt = createdAt
		} else {
			si := tm.sprintIdx
			start := sprintStart(si)
			end := sprintEnd(si)
			if si == activeSprintIdx {
				end = now // cap active sprint end at now
			}
			dur := end.Sub(start)
			if dur <= 0 {
				dur = time.Hour
			}
			// Created randomly in the first half of the sprint
			halfDur := dur / 2
			createOffset := time.Duration(rand.Int63n(int64(halfDur)))
			createdAt = sqlTS(start.Add(createOffset))
			if tm.stage == "done" && si < activeSprintIdx {
				// Completed in the second half of the sprint
				doneOffset := halfDur + time.Duration(rand.Int63n(int64(halfDur)))
				updatedAt = sqlTS(start.Add(doneOffset))
			} else if si == activeSprintIdx {
				// Updated at some point since sprint start
				updateOffset := time.Duration(rand.Int63n(int64(end.Sub(start))))
				updatedAt = sqlTS(start.Add(updateOffset))
			} else {
				// Past closed sprint, not done: updated mid-sprint
				updateOffset := halfDur + time.Duration(rand.Int63n(int64(halfDur)/2+1))
				updatedAt = sqlTS(start.Add(updateOffset))
			}
		}
		if _, err := db.ExecContext(ctx, `UPDATE tickets SET created_at = ?, updated_at = ? WHERE ticket_id = ?`,
			createdAt, updatedAt, tm.id); err != nil {
			return fmt.Errorf("backdating ticket %s: %w", tm.id, err)
		}
	}
	fmt.Println(" done")

	// Create comments
	fmt.Printf("  Creating %d comments...", numComments)

	// Collect ticket IDs that can receive comments (not complete/archived)
	// We'll just attempt to add comments and skip failures (ticket closed)
	commentIdx := 0
	for commentIdx < numComments {
		// Pick a random ticket by index
		ticketLocalIdx := rand.Intn(numTickets)
		// Determine project and local ticket index
		pi := ticketLocalIdx % len(projects)
		pd := projectData[pi]

		// We'll pick a comment template
		tmpl := commentTemplates[commentIdx%len(commentTemplates)]
		fillIn := commentFillins[commentIdx%len(commentFillins)]
		var commentText string
		verbCount := strings.Count(tmpl, "%")
		if verbCount > 0 && strings.Contains(tmpl, "%s") {
			commentText = fmt.Sprintf(tmpl, fillIn)
		} else if strings.Contains(tmpl, "%%") {
			commentText = fmt.Sprintf(tmpl, fillIn)
		} else {
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
	fmt.Printf("  Server:  tk serve -f %s (then open http://localhost:8080)\n", *dbPath)

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

	return nil
}
