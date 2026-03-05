EPIC: Core Platform And Runtime
ID: E1
DESCRIPTION: Build the single-binary application foundation, including SQLite initialization, configuration, shared runtime wiring, HTTP server startup, embedded frontend serving, and developer quality gates.
AC:
- `task initdb -f <db> -password <password>` creates a new SQLite database and bootstraps the initial administrator account.
- `task server -f <db>` starts the API server and serves the embedded frontend from the same binary on `http://localhost:8000` by default.
- The application supports configuration for remote CLI usage, including `TICKET_SERVER`.
- Passwords are stored securely using Argon2id hashes in SQLite.
- The frontend assets are embedded in the Go binary and served by the backend.
- `make build`, `make test`, `make test-go`, and `make test-playwright` exist and run successfully.
- All tests pass.
- Use `make` to verify tests.
- Work in a branch that contains the EPIC and FEATURE name.
PRIORITY: 1
DEPENDS-ON: NONE

    STORY: Initialize SQLite workspace and bootstrap admin user
    ID: E1-S1
    DESCRIPTION: Implement database initialization, schema creation, and first-run administrator bootstrap through the CLI.
    AC:
    - Running `task initdb -f filename.db -password password` creates the SQLite file, schema, and initial admin account.
    - Re-running init against an existing database fails safely or requires an explicit overwrite flag.
    - The bootstrap password is stored as an Argon2id hash rather than plaintext.
    - The initialization flow is covered by automated Go tests.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: NONE

    STORY: Start HTTP server and serve embedded frontend
    ID: E1-S2
    DESCRIPTION: Implement the runtime entrypoint that starts the API server and serves embedded SPA assets from the same process.
    AC:
    - Running `task server -f filename.db` starts the application successfully.
    - The server listens on `http://localhost:8000` by default.
    - Embedded frontend assets are served from the backend process.
    - Startup and shutdown behavior are covered by automated tests.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E1-S1

    STORY: Add runtime configuration for CLI and server
    ID: E1-S3
    DESCRIPTION: Implement configuration loading for database path, server URL, and other shared runtime settings used by CLI and server commands.
    AC:
    - The CLI can target a remote server using `TICKET_SERVER`.
    - Local defaults are stored and reused between commands where appropriate.
    - Configuration behavior is documented through tests.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 2
    DEPENDS-ON: E1-S2

    STORY: Establish build and test automation
    ID: E1-S4
    DESCRIPTION: Create and wire quality gates for build, unit, integration, and browser tests.
    AC:
    - `make build` builds the application.
    - `make test` runs the full automated test suite.
    - `make test-go` runs backend and CLI tests.
    - `make test-playwright` runs frontend/browser tests.
    - CI-ready scripts exist for local verification.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E1-S2

EPIC: Authentication And User Administration
ID: E2
DESCRIPTION: Implement account management, login/logout flows, session handling, CLI identity commands, and administrator user management across API, CLI, and web interfaces.
AC:
- The system supports administrator and standard user roles.
- Administrators can create, enable, disable, and list users.
- Users can register, log in, log out, and inspect their current identity.
- The CLI supports `TICKET_USERNAME` and `TICKET_PASSWORD` environment variables.
- The web UI uses the same authentication system as the CLI.
- Session and permission checks are enforced on protected operations.
- All tests pass.
- Use `make` to verify tests.
- Work in a branch that contains the EPIC and FEATURE name.
PRIORITY: 1
DEPENDS-ON: E1

    STORY: Implement user model, storage, and role enforcement
    ID: E2-S1
    DESCRIPTION: Add the backend user domain model, repository layer, and authorization checks for admin and normal-user actions.
    AC:
    - Users have `user_id`, `username`, `password_hash`, `role`, `display_name`, `enabled`, and `created_at`.
    - Protected endpoints reject unauthorized or disabled users.
    - Admin-only operations are enforced server-side.
    - Automated tests cover allowed and forbidden access paths.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E1-S1

    STORY: Implement admin user management commands and API
    ID: E2-S2
    DESCRIPTION: Provide administrator features for creating, enabling, disabling, and listing users from the CLI and API.
    AC:
    - `task user create -username XXXX -password YYYY` creates a user.
    - `task user enable -username XXXX` enables a disabled user.
    - `task user disable -username XXXX` disables a user.
    - `task user list` and `task user ls` list existing users.
    - Admin user management is covered by automated tests.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E2-S1

    STORY: Implement registration, login, logout, whoami, and status
    ID: E2-S3
    DESCRIPTION: Provide the core CLI and API authentication flows used by end users.
    AC:
    - `task register` supports interactive account creation.
    - `task login` supports interactive login or credential lookup from environment variables.
    - `task whoami` shows the current authenticated user.
    - `task status` shows authentication and connectivity status.
    - `task logout` clears the local authenticated session.
    - Automated tests cover successful and failed login scenarios.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E2-S1

    STORY: Implement web authentication flows
    ID: E2-S4
    DESCRIPTION: Add browser login/logout/session behavior against the same backend authentication APIs used by the CLI.
    AC:
    - The web app supports login and logout.
    - Authenticated browser sessions persist across normal page use.
    - Unauthenticated users are redirected or blocked from protected project pages.
    - Browser tests cover login and logout behavior.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 2
    DEPENDS-ON: E2-S3, E1-S2

EPIC: Project Management And CLI Context
ID: E3
DESCRIPTION: Implement projects as top-level containers, including create/list/get/use flows, active-project defaults, and project switching in the web UI.
AC:
- Users can create, list, inspect, and select projects from the CLI.
- The CLI remembers the active project for subsequent commands.
- The web UI exposes project switching.
- Project permissions are enforced through authenticated APIs.
- All tests pass.
- Use `make` to verify tests.
- Work in a branch that contains the EPIC and FEATURE name.
PRIORITY: 1
DEPENDS-ON: E2

    STORY: Implement project model, storage, and API
    ID: E3-S1
    DESCRIPTION: Add the backend project domain model, persistence, and CRUD API endpoints required by CLI and web clients.
    AC:
    - Projects store `project_id`, `slug`, `title`, `description`, `created_at`, `created_by`, and `status`.
    - Project create, list, and get APIs exist and are authenticated.
    - Automated tests cover project creation and lookup.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E2-S1

    STORY: Implement project CLI commands and active-project state
    ID: E3-S2
    DESCRIPTION: Add the CLI commands for project creation, listing, inspection, and active-project selection.
    AC:
    - `task project create "Customer Portal"` creates a project.
    - `task project list` lists projects.
    - `task project get <id>` shows project details.
    - `task project use <id>` sets the active project.
    - `task project` shows the current project context.
    - Active-project behavior is covered by automated tests.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E3-S1, E1-S3

    STORY: Implement project switching in the web UI
    ID: E3-S3
    DESCRIPTION: Add project navigation and switching controls in the browser experience.
    AC:
    - The web app displays a project switcher.
    - Switching the active project updates visible work items without a manual reload.
    - Browser tests cover project switching behavior.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 2
    DEPENDS-ON: E3-S1, E1-S2

EPIC: Task, Epic, Bug, Note, And Question Management
ID: E4
DESCRIPTION: Implement the shared task model, item creation flows, epic hierarchy, active-epic defaults, and item editing capabilities across CLI, API, and web UI.
AC:
- The system supports `epic`, `task`, `bug`, `note`, and `question` item types.
- Item creation captures creator, timestamps, project, status, and parent relationship.
- Users can create items using all examples in the user guide, including `task add`, `task bug`, `task note`, `task question`, `task epic`, and `task create -type task`.
- Active-epic behavior attaches new child tasks to the selected epic when appropriate.
- The web UI supports creating and editing items.
- All tests pass.
- Use `make` to verify tests.
- Work in a branch that contains the EPIC and FEATURE name.
PRIORITY: 1
DEPENDS-ON: E3

    STORY: Implement shared task domain model and persistence
    ID: E4-S1
    DESCRIPTION: Build the core work-item model and database schema used for all item types.
    AC:
    - Tasks store `task_id`, `project_id`, `parent_id`, `type`, `title`, `description`, `acceptance_criteria`, `status`, `priority`, `assignee`, `created_at`, `created_by`, `updated_at`, and `archived`.
    - Supported types include `epic`, `task`, `bug`, `note`, and `question`.
    - Parent-child relationships are persisted correctly.
    - Automated tests cover schema behavior and CRUD operations.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E3-S1

    STORY: Implement CLI item creation commands
    ID: E4-S2
    DESCRIPTION: Add terminal commands for all supported item creation flows described in the guide.
    AC:
    - `task add "..."` creates a standard task.
    - `task note "..."` creates a note.
    - `task question "..."` creates a question.
    - `task bug "..."` creates a bug.
    - `task epic "..."` creates an epic.
    - `task create -type task "..."` creates a task with explicit type selection.
    - CLI tests cover all supported creation commands.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E4-S1, E3-S2

    STORY: Implement active epic context and parent-child defaults
    ID: E4-S3
    DESCRIPTION: Track the active epic in CLI context so new items can automatically attach beneath it.
    AC:
    - Creating an epic can mark it as the current epic context.
    - `task create -type task "..."` attaches the new task to the current epic when an active epic exists.
    - Users can inspect or clear the active epic context.
    - Automated tests cover parent assignment behavior.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 2
    DEPENDS-ON: E4-S2

    STORY: Implement item create and edit flows in the web UI
    ID: E4-S4
    DESCRIPTION: Add browser interfaces for creating, editing, and organizing tasks and other work-item types.
    AC:
    - The web app provides item creation controls for task, epic, bug, note, and question types.
    - Users can edit item fields from the web app.
    - Parent-child relationships are visible and editable in the web UI.
    - Browser tests cover create and edit workflows.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 2
    DEPENDS-ON: E4-S1, E3-S3

EPIC: Listing, Search, Detail Views, And Status Workflow
ID: E5
DESCRIPTION: Implement item retrieval, filtering, search, status updates, detail views, and review-oriented workflows across the backend, CLI, and frontend.
AC:
- Users can list items in the active project.
- Users can filter items by type and status.
- Users can search item titles and descriptions.
- Users can inspect item detail by ID.
- The application supports a default status workflow suitable for CLI lists and board views.
- All tests pass.
- Use `make` to verify tests.
- Work in a branch that contains the EPIC and FEATURE name.
PRIORITY: 1
DEPENDS-ON: E4

    STORY: Implement list, get, and search APIs
    ID: E5-S1
    DESCRIPTION: Add backend read APIs for listing project items, fetching a single item, and searching by title and description.
    AC:
    - The API supports list by project, get by ID, and full-text or equivalent search.
    - List results can be filtered by item type and status.
    - Automated tests cover list, get, and search behavior.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E4-S1

    STORY: Implement CLI list, get, show, and search commands
    ID: E5-S2
    DESCRIPTION: Add CLI commands for the documented read flows over work items.
    AC:
    - `task list` lists all items in the active project.
    - `task list --type <type>` filters by item type.
    - `task list --status <status>` filters by status.
    - `task search "password reset"` searches item titles and descriptions.
    - `task get 42` returns item details.
    - `task get 42` shows the full item detail view.
    - CLI tests cover all documented read commands.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E5-S1, E3-S2

    STORY: Implement item status workflow and update operations
    ID: E5-S3
    DESCRIPTION: Add backend and CLI support for item status transitions such as open, in progress, blocked, and done.
    AC:
    - Items can transition between the default statuses `open`, `in_progress`, `blocked`, and `done`.
    - Status changes are persisted and validated.
    - Status changes produce history events.
    - Automated tests cover valid and invalid transitions.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 2
    DEPENDS-ON: E5-S1

    STORY: Implement item detail views and filters in the web UI
    ID: E5-S4
    DESCRIPTION: Add browser pages and controls for list views, filtering, search, and single-item inspection.
    AC:
    - The web app supports project-scoped item lists.
    - Users can filter by type and status in the UI.
    - Users can search items in the UI.
    - Clicking an item opens a detail view with fields and metadata.
    - Browser tests cover list, filter, search, and detail workflows.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 2
    DEPENDS-ON: E5-S1, E3-S3

EPIC: History, Comments, And Activity Tracking
ID: E6
DESCRIPTION: Implement append-only history events and item comments so users can understand how work changed over time and discuss individual tasks.
AC:
- Important item changes create history events.
- Users can inspect item history from CLI and web interfaces.
- Users can add and view comments on work items.
- Comment creation is reflected in activity history.
- All tests pass.
- Use `make` to verify tests.
- Work in a branch that contains the EPIC and FEATURE name.
PRIORITY: 2
DEPENDS-ON: E4, E5

    STORY: Implement history event storage and generation
    ID: E6-S1
    DESCRIPTION: Add append-only history storage and generate events for create, update, status change, parent change, and comment operations.
    AC:
    - History records include `project_id`, `task_id`, `event_type`, `payload`, `created_at`, and `created_by`.
    - Creating and updating items generates history events.
    - Status changes and parent changes generate history events.
    - Automated tests validate history event generation.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E4-S1, E5-S3

    STORY: Implement comment model, API, and CLI command
    ID: E6-S2
    DESCRIPTION: Add comments as item-scoped discussion records available via backend and CLI.
    AC:
    - Comments store `item_id`, `user_id`, `comment`, and `created_at`.
    - `task comment add 17 "Waiting on API changes."` creates a comment on an item.
    - Adding a comment creates a related history event.
    - Automated tests cover comment creation and retrieval.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E6-S1

    STORY: Implement CLI and web history/comment views
    ID: E6-S3
    DESCRIPTION: Surface history and comments in item detail experiences across both interfaces.
    AC:
    - `task history 17` returns ordered history for the target item.
    - Item detail pages in the web app display history and comments.
    - Browser tests cover history and comment visibility.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 2
    DEPENDS-ON: E6-S2, E5-S4

EPIC: Web Board, Hierarchy, And Collaborative UX
ID: E7
DESCRIPTION: Implement the primary browser experience, including project switching, status-based board views, hierarchy browsing, collaborative refresh behavior, and core task management interactions.
AC:
- The web app provides a status-based board view for project items.
- Users can browse work hierarchies, including epics and child items.
- Changes made by one user appear for other connected users without manual refresh under normal operation.
- The frontend remains operationally lightweight and is covered by browser tests.
- All tests pass.
- Use `make` to verify tests.
- Work in a branch that contains the EPIC and FEATURE name.
PRIORITY: 2
DEPENDS-ON: E3, E4, E5, E6

    STORY: Implement status-based board view
    ID: E7-S1
    DESCRIPTION: Build the browser board UI grouped by item status for active-project work management.
    AC:
    - The web app renders project items grouped by status.
    - Status columns reflect the configured default workflow.
    - Board interactions update item status correctly.
    - Browser tests cover board rendering and status movement.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 1
    DEPENDS-ON: E5-S3, E5-S4

    STORY: Implement hierarchy browsing for epics and children
    ID: E7-S2
    DESCRIPTION: Add browser support for viewing parent-child task relationships and epic contents.
    AC:
    - Users can view epics and their child tasks in the web UI.
    - Item detail views show parent and child relationships.
    - Hierarchy browsing works alongside board and list views.
    - Browser tests cover hierarchy rendering.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 2
    DEPENDS-ON: E4-S4, E5-S4

    STORY: Implement near-real-time collaborative refresh
    ID: E7-S3
    DESCRIPTION: Add lightweight live-update behavior so connected clients see new and changed data without manual reloads.
    AC:
    - Changes made by one authenticated user become visible to another active browser session without manual refresh.
    - The live update mechanism builds on the server resource model rather than bypassing it.
    - Browser or integration tests cover collaborative refresh behavior.
    - All tests pass.
    - Use `make` to verify tests.
    - Work in a branch that contains the EPIC and FEATURE name.
    PRIORITY: 3
    DEPENDS-ON: E7-S1, E7-S2
