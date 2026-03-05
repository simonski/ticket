EPIC: Core Runtime, Bootstrap, And Delivery
ID: E1
DESCRIPTION: Build the single-binary runtime for `task`, including versioning, onboarding, SQLite bootstrap, server startup, embedded web delivery, and make-based quality gates.
AC:
- `task onboard` appends the embedded onboarding template into `${CWD}/AGENTS.md`.
- `task version` prints the embedded semantic version from the build asset.
- `task initdb` creates the SQLite schema, the `admin` account, and the default project.
- `task server` starts the API and embedded web UI from the same binary and serves on `http://localhost:8080` by default.
- `task server -v` prints verbose request and response logs to stdout.
- `task` with no arguments and `task server` print the ASCII-art `TASK` banner.
- use red/green testing
- use make to verify all tests pass
- work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
PRIORITY: 1
DEPENDS-ON: NONE

    STORY: Implement build versioning and embedded runtime assets
    ID: E1-S1
    DESCRIPTION: Embed static runtime assets needed by the CLI, including `VERSION` and the onboarding template, and wire build-time version incrementing through `make`.
    AC:
    - `VERSION` is incremented during `make build`.
    - The built binary embeds the version value used by `task version`.
    - The built binary embeds `cmd/task/AGENTS.md` for `task onboard`.
    - Automated tests cover version lookup and onboard asset behavior.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: NONE

    STORY: Implement SQLite bootstrap and default workspace initialization
    ID: E1-S2
    DESCRIPTION: Provide CLI-driven database initialization with safe overwrite behavior and generated admin credentials when needed.
    AC:
    - `task initdb` creates a database at `$TICKET_HOME/task.db` when `-f` is omitted.
    - `task initdb -f task.db --force -password secret` overwrites an existing database and uses the supplied password.
    - `task initdb` without `-password` generates a random admin password and prints it to stdout.
    - The initialized database contains the `admin` user and the default project with project id `1`.
    - Passwords are stored as Argon2id hashes in SQLite.
    - Automated tests cover first-run, overwrite, and generated-password flows.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E1-S1

    STORY: Implement server startup, banner output, and embedded web serving
    ID: E1-S3
    DESCRIPTION: Start the HTTP server, serve the embedded frontend, and print the startup banner, version, and database path.
    AC:
    - `task server` opens `$TICKET_HOME/task.db` when `-f` is omitted.
    - `task server -f filename.db` serves the API and embedded frontend against the selected database.
    - `task server` prints the banner, the embedded version, and the resolved task database path before the listen message.
    - `task server -v` logs request and response details to stdout.
    - Automated tests cover startup wiring and verbose logging.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E1-S2

    STORY: Implement root usage and command help output
    ID: E1-S4
    DESCRIPTION: Provide concise top-level usage and per-command help with examples that match the documented command surface.
    AC:
    - `task` prints the main client and admin commands only.
    - `task help <command>` prints command usage, details, and a short example.
    - Root usage and command help stay aligned with the current CLI surface.
    - Automated tests cover root usage and representative command help output.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 2
    DEPENDS-ON: E1-S3

EPIC: Authentication, Sessions, And Admin User Management
ID: E2
DESCRIPTION: Implement registration, login, logout, session storage, status inspection, and admin-only user management across the API, CLI, and web UI.
AC:
- The system supports `admin` and `user` roles.
- Admins can create, list, delete, enable, and disable users.
- `task register`, `task login`, `task logout`, and `task status` behave as documented.
- Client-side state is split between `$TICKET_HOME/config.json` and `$TICKET_HOME/credentials.json`.
- The web app uses the same authentication and session model as the CLI.
- Admin-only calls made by non-admin users return HTTP 403 with `user is not an admin`.
- use red/green testing
- use make to verify all tests pass
- work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
PRIORITY: 1
DEPENDS-ON: E1

    STORY: Implement user model, roles, and admin authorization
    ID: E2-S1
    DESCRIPTION: Build backend user storage and enforce admin-only access for protected user-management endpoints.
    AC:
    - Users persist `user_id`, `username`, `password_hash`, `role`, `display_name`, `enabled`, and `created_at`.
    - Admin-only endpoints reject authenticated non-admin callers with HTTP 403 and `user is not an admin`.
    - Disabled users cannot authenticate or perform protected operations.
    - Automated tests cover allowed and denied access paths.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E1-S2

    STORY: Implement admin user-management commands and API flows
    ID: E2-S2
    DESCRIPTION: Support the documented admin CLI commands for managing users.
    AC:
    - `task user create --username alice --password secret` creates a user.
    - `task user ls` and `task user list` list users.
    - `task user delete --username alice` and `task user rm --username alice` delete a user.
    - `task user enable --username alice` and `task user disable --username alice` update enabled state.
    - Successful commands return human-readable output and support `-json`.
    - Automated tests cover create, list, delete, enable, and disable flows.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E2-S1

    STORY: Implement registration, login, logout, and local session storage
    ID: E2-S3
    DESCRIPTION: Implement the documented credential resolution rules, interactive prompting, and local session persistence.
    AC:
    - `task register --username name --password secret` creates an account and does not log the user in.
    - `task register` resolves missing values from `TICKET_USERNAME`, `TICKET_PASSWORD`, then `whoami` and `password`.
    - `task login` checks `$TICKET_HOME/credentials.json`, then `$TICKET_HOME/config.json`, then flags, then env vars, then prompts.
    - `task login` prompts with editable defaults and masks password input with `*` on interactive terminals.
    - `task login` stores the session token in `$TICKET_HOME/credentials.json` and stores `username` and `server_url` in `$TICKET_HOME/config.json`.
    - `task logout` removes `$TICKET_HOME/credentials.json`.
    - Automated tests cover registration, login success, invalid credentials retry, and logout.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E2-S1

    STORY: Implement status inspection and shared web authentication flows
    ID: E2-S4
    DESCRIPTION: Expose server status and version information and reuse the same authentication model in the browser.
    AC:
    - `task status` prints the effective configuration first, then performs a mode-appropriate connectivity check.
    - In remote mode it prints mode, server, username, authentication state, and remote connection success/failure.
    - In local mode it prints mode, resolved database path, file existence, and local connection success/failure.
    - The web UI supports login, logout, and authenticated session reuse.
    - Automated tests cover status responses and browser auth state transitions.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 2
    DEPENDS-ON: E2-S3, E1-S3

EPIC: Project Management And Local CLI Context
ID: E3
DESCRIPTION: Implement projects as top-level containers, including create/list/get/use workflows, active-project context, and project switching in the web UI.
AC:
- Users can create, list, inspect, and select projects from the CLI.
- The CLI remembers the active project in local config.
- The web UI supports project selection and creation.
- Project APIs are authenticated and shared by CLI and web clients.
- use red/green testing
- use make to verify all tests pass
- work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
PRIORITY: 1
DEPENDS-ON: E2

    STORY: Implement project model, persistence, and API endpoints
    ID: E3-S1
    DESCRIPTION: Add the backend project domain model and authenticated APIs for create, list, and lookup.
    AC:
    - Projects persist `project_id`, `title`, `description`, `created_at`, `created_by`, and `status`.
    - The API supports project creation, listing, and lookup by numeric id.
    - Project records are available to both CLI and web clients through the same API.
    - Automated tests cover create, list, and lookup behavior.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E2-S1

    STORY: Implement project CLI commands and active-project persistence
    ID: E3-S2
    DESCRIPTION: Support the documented project commands and remember the active project for subsequent CLI commands.
    AC:
    - `task project create "Customer Portal"` creates a project and makes it current.
    - `task project list` and `task project ls` list projects.
    - `task project get 2` shows project details.
    - `task project use 2` changes the active project.
    - `task project` shows the current project or `no active project`.
    - Automated tests cover active-project persistence and lookup by id.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E3-S1, E2-S3

    STORY: Implement project switching and creation in the web UI
    ID: E3-S3
    DESCRIPTION: Provide browser controls for listing, creating, and switching the active project.
    AC:
    - The web app displays available projects and the current selection.
    - Users can create a project from the web UI.
    - Switching the project reloads the visible work items without a full manual reload.
    - Browser or integration tests cover project creation and switching.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 2
    DEPENDS-ON: E3-S1, E1-S3

EPIC: Work Item Model, Creation, And Hierarchy
ID: E4
DESCRIPTION: Implement the shared work-item model for epics, tasks, and bugs, including creation defaults, parent-child hierarchy, acceptance criteria, and active-epic context.
AC:
- The system supports `epic`, `task`, and `bug` item types only.
- Users can create work through `task add`, `task create`, `task new`, `task bug`, and `task epic`.
- Work items support title, description, acceptance criteria, priority, assignee, project, and optional parent.
- Parent-child relationships support epics with child tasks and bugs.
- The web UI supports creating and editing items against the same model.
- use red/green testing
- use make to verify all tests pass
- work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
PRIORITY: 1
DEPENDS-ON: E3

    STORY: Implement the shared work-item schema and validation
    ID: E4-S1
    DESCRIPTION: Add the persistence layer and validation rules for `epic`, `task`, and `bug` records.
    AC:
    - Work items persist `task_id`, `project_id`, `parent_id`, `type`, `title`, `description`, `acceptance_criteria`, `status`, `priority`, `assignee`, `created_at`, `created_by`, `updated_at`, and `archived`.
    - Only `epic`, `task`, and `bug` are accepted as valid task types.
    - Parent-child relationships are stored correctly.
    - Automated tests cover CRUD operations and type validation.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E3-S1

    STORY: Implement CLI work-item creation flows
    ID: E4-S2
    DESCRIPTION: Support all documented CLI creation examples and defaults for tasks, bugs, and epics.
    AC:
    - `task add "Customers can reset their password."` creates a task.
    - `task create "I am a new task"` and `task new "I am a new task"` are aliases for task creation.
    - `task add -title "I am a new task"` sets the title without positional words.
    - `task bug "This is a bug"` creates a bug.
    - `task epic "This is an Epic"` creates an epic.
    - `task create -t task -p 1 -a alice -d "This is a Task" -ac "Has a title and description" "This is a Task"` is supported.
    - Successful create commands print the created task id to stdout.
    - Automated tests cover aliases, defaults, and flag parsing.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E4-S1, E3-S2

    STORY: Implement active-epic context and hierarchy behavior
    ID: E4-S3
    DESCRIPTION: Track the active epic in the CLI and support hierarchical organization of tasks and bugs under epics.
    AC:
    - Creating or selecting an epic can set the active epic context for subsequent work.
    - New tasks and bugs can be attached beneath an epic via `parent_id`.
    - The CLI stores useful local context for the active epic.
    - Automated tests cover parent assignment and active-epic behavior.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 2
    DEPENDS-ON: E4-S2

    STORY: Implement browser capture and editing for work items
    ID: E4-S4
    DESCRIPTION: Add web UI support for creating and updating tasks, bugs, and epics.
    AC:
    - The web capture form creates tasks, bugs, and epics.
    - Newly created items appear in the current project view without manual reload.
    - The detail form updates title, description, and status through the shared API.
    - Browser or integration tests cover web creation and update flows.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 2
    DEPENDS-ON: E4-S1, E3-S3

EPIC: Retrieval, Assignment, Dependencies, And Activity
ID: E5
DESCRIPTION: Implement list, search, detail, history, comments, dependencies, assignment, orphan detection, and aggregate count workflows for the current project model.
AC:
- Users can list, search, inspect, and count work items through the CLI and API.
- `task get` prints the flat detail view documented in the design.
- Users can add comments and review append-only history.
- Dependencies can be added and removed between tasks.
- Admin assignment and self-claim workflows are enforced correctly.
- `task count` reports users, projects, and current work-item totals by type and status.
- use red/green testing
- use make to verify all tests pass
- work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
PRIORITY: 1
DEPENDS-ON: E4

    STORY: Implement project-scoped list, search, and orphan workflows
    ID: E5-S1
    DESCRIPTION: Provide list, filter, search, and orphan queries for the active project, including server-side limits.
    AC:
    - `task list`, `task ls`, and `task list -n 20` list work items from the active project.
    - `task list --type task`, `task list --type bug`, and `task list --type epic` filter by type.
    - `task list --status open`, `task list --status in_progress`, and `task list --status done` filter by status.
    - `task list -u alice` and `task ls -u alice` filter by assignee.
    - `task search "password reset"` searches titles and descriptions.
    - `task orphans` lists items whose `parent_id` is null.
    - `task list` prints a readable table with id, type, status, assignee, priority, and title.
    - Automated tests cover filtering, search, assignee filtering, and limit handling.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E4-S1, E3-S2

    STORY: Implement task detail rendering, history, and comments
    ID: E5-S2
    DESCRIPTION: Support detailed task inspection and append-only activity review through the API, CLI, and web UI.
    AC:
    - `task get 42` prints `ID`, `Type`, `Description`, `ParentID`, `ProjectID`, `Title`, `Assignee`, `Order`, `DependsOn`, `Status`, `Priority`, `Created`, `LastModified`, `Closed`, and `Acceptance Criteria`.
    - `task get -json 42` pretty-prints the raw JSON response.
    - `task history 17` prints the history for the item.
    - `task comment add 17 "Waiting on API changes."` creates a comment and corresponding activity entry.
    - The web detail pane shows the current item, dependencies, comments, and revision history.
    - Automated tests cover detail rendering, history retrieval, and comment creation.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E4-S1

    STORY: Implement assignment, claim, and unclaim workflows
    ID: E5-S3
    DESCRIPTION: Enforce task assignment rules for admins and standard users across API and CLI flows.
    AC:
    - `task assign 42 alice` and `task unassign 42 alice` are admin-only.
    - `task assign` and `task unassign` fail if the named user does not exist.
    - `task assign` and `task unassign` fail if the named user is disabled.
    - `task claim 42` assigns the caller unless another user already owns the task.
    - `task unclaim 42` succeeds only when the caller is the current assignee.
    - Automated tests cover admin assignment, non-admin rejection, claim, and unclaim flows.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E2-S2, E4-S1

    STORY: Implement dependency management between work items
    ID: E5-S4
    DESCRIPTION: Support task dependency creation and removal through the API, CLI, and detail views.
    AC:
    - `task dependency add 4 1,2,3` adds comma-separated dependencies.
    - `task dependency remove 4 2` removes one or more dependencies.
    - `task get` renders `DependsOn` from the stored dependency data.
    - The web detail pane exposes dependency information.
    - Automated tests cover add, remove, and detail rendering for dependencies.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E4-S1

    STORY: Implement aggregate count reporting
    ID: E5-S5
    DESCRIPTION: Add server-backed aggregate counting for users, projects, and current work-item types.
    AC:
    - `task count` prints users, projects, and work-item totals by type.
    - `task count -project_id 1` prints project-scoped work-item totals and omits the global project total.
    - Count output groups totals by status for tasks, bugs, and epics where applicable.
    - Automated tests cover global and project-scoped count responses.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 2
    DEPENDS-ON: E3-S1, E4-S1, E2-S1

EPIC: Web Application Views And Collaborative UX
ID: E6
DESCRIPTION: Implement the embedded web experience for authenticated project work, including list, board, hierarchy, detail, comments, and live refresh.
AC:
- The web UI is served from the same binary and uses the same API contracts as the CLI.
- Authenticated users can create, view, and update project work in the browser.
- The web UI supports list, board, and hierarchy views.
- Detail views show history, comments, and dependency context.
- Connected browser sessions refresh changes without manual page reloads.
- use red/green testing
- use make to verify all tests pass
- work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
PRIORITY: 2
DEPENDS-ON: E3, E4, E5

    STORY: Implement browser list, board, and hierarchy views
    ID: E6-S1
    DESCRIPTION: Provide the main browser navigation and task browsing modes for the active project.
    AC:
    - The web app shows a list view for project work items.
    - The web app shows a status-based board with `open`, `in_progress`, `blocked`, and `done` columns.
    - The web app shows a hierarchy view that groups child tasks beneath epics and separates unparented work.
    - Browser or integration tests cover list, board, and hierarchy rendering.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E4-S4, E5-S1

    STORY: Implement browser detail, comments, and activity views
    ID: E6-S2
    DESCRIPTION: Surface task detail editing and activity inspection in the web UI.
    AC:
    - Selecting an item opens a detail pane for that item.
    - The detail pane supports title, description, and status updates through the shared API.
    - The detail pane shows comments and history for the selected item.
    - The detail pane shows dependency context for the selected item.
    - Browser or integration tests cover detail editing and activity display.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 1
    DEPENDS-ON: E5-S2, E5-S4

    STORY: Implement collaborative refresh for active browser sessions
    ID: E6-S3
    DESCRIPTION: Refresh browser state automatically so active users see project updates without manual reloads.
    AC:
    - Changes made by one user become visible to another connected browser session without manual page reload.
    - The refresh behavior reuses the existing API and project resource model.
    - Browser or integration tests cover collaborative refresh behavior.
    - use red/green testing
    - use make to verify all tests pass
    - work in a branch that contains the EPIC and TASK name for example `feature/<epic>-<task>`
    PRIORITY: 2
    DEPENDS-ON: E6-S1, E6-S2
