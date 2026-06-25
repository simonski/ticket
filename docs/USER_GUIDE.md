# User Guide

How to use `ticket` day to day. New here? Start with
[QUICKSTART](./QUICKSTART.md) and the [TUTORIAL](./TUTORIAL.md); this guide covers
the web app's everyday features.

## Driving the app by keyboard

The web UI is keyboard-driveable through a **command palette**.

- Press **Shift twice** (Shift-Shift) anywhere to open it.
- Type a **/slash command** (or fuzzy text) and press **Enter** to jump to a
  window. Single-letter aliases work too: `/c` chat, `/b` board, `/p` projects,
  `/w` workflows, `/d` documents, `/m` mailbox, `/r` roles, `/t` teams, `/g`
  context, `/a` agents, `/u` users, `/s` settings.
- Type a **ticket key** (e.g. `/tk-23`) to open that ticket's **action menu** —
  pick an action by number or with the arrow keys + Enter.
- **Esc** pops one level back (action menu → command list → closed); **↑/↓** move,
  **Enter** selects, click also works.

On the **board**, the **arrow keys** or **w/a/s/d** move focus between ticket
cards (left/right across lanes, up/down within a lane) and **Enter** opens the
focused ticket — instead of scrolling the page.

## Chat & rooms

Open **Chat** from the sidebar (or `/chat`, which also focuses the message box).

### Rooms

Rooms come in three scopes, grouped in the sidebar:

- **Global** — standalone rooms like `#general`, independent of any project.
- **Project** — a channel that belongs to a project.
- **Breakouts** — a room scoped to a specific ticket (epic/story).

Create a room with **New room**. Rooms are **public** (anyone can find and join)
or **private** (invite-only). Use **Join** / **Leave** on a room you have open.

### Messaging

Type in the composer and press Enter (or **Send**). In messages:

- **`@name`** mentions a person or agent (highlighted).
- **`#label`** references a label (highlighted).
- **`@username <message>`** (at the start) is a shortcut to direct-message that
  person (an `@agent` at the start instead pings the agent in the room).

Rooms with unread messages are marked with a `*` in the sidebar, and each room
shows its member count.

### Chat commands

Type these in the composer:

| Command | Does |
|---------|------|
| `/new <name>` | Create a room and switch into it |
| `/join <name>` | Join a room by name and switch into it (just switches if you're already a member) |
| `/leave` | Leave the current room and return to the previous room |
| `/invite <username>` | Add a user to the room (and its project) |
| `/kick <username>` | Remove a user from the room |
| `/list` | List rooms alphabetically with member counts |
| `/rename <name>` | Rename the current room (owner/admin; no-op if already named that) |
| `/status [words]` | Set your status, or print it with no argument |
| `/ping <username>` | Send a ping notification to a user |
| `/msg <username> <message>` | DM a user **without** switching to that chat |
| `/task [@agent] <description>` | Create a tracked ticket (see below) |

The **public room** and a **project's room** are permanent — you can't leave them
(no Leave button, and `/leave` is rejected). When messages arrive in rooms you're
not viewing, a **red counter** appears next to that room in the list.

### Breakout rooms from a ticket

Open any ticket and click **💬 Breakout** to open (or create) a chat room scoped
to that ticket — handy for discussing an epic or story in context.

## Your personal agent

Every user gets their own agent, provisioned automatically the first time you open
chat. It appears as a DM under **People & Agents** (e.g. `you's agent`). Talk to it
there — each message is a **query** it answers in the thread (the DM is your
ongoing session), and `/task <description>` delegates a tracked **ticket**. Only
you see your personal-agent DM.

## Working with agents in chat

Agents are first-class room members. There are two ways to involve them:

- **Converse** — `@mention` an agent that is a member of the room and it replies
  in the room (requires the agent runtime to be configured on the server).
- **Task** — type **`/task @agent <description>`** to create a tracked **ticket**
  assigned to that agent. The message posts a link to the new ticket, and the
  orchestrator/agent picks the work up from there. In a project or breakout room
  the ticket is created in that project (and parented to the breakout's ticket);
  tasking is not available in a global room.

  Examples:

  ```
  /task @builder add a health-check endpoint
  /task investigate the flaky deploy
  ```

## Email / SMTP (admin)

Configure the outbound email sender under **Settings → Email** (or via the CLI):

- Set the SMTP **host, port, security** (none/STARTTLS/TLS), **username/password**,
  and **from address/name**, then toggle **Enable email sending**.
- The password is **never shown back** — the form/`tk email show` only indicate
  whether one is stored; saving without re-entering it keeps the stored secret.
- CLI: `tk email show`, `tk email set -host … -port … -password …`,
  `tk email enable`, `tk email disable`.
- This only *configures* the sender; actually sending mail is a separate
  capability.

## Access roles (admin)

Admins can gate which panels each user sees from the **Access** panel (admin
sidebar section):

- **Access roles** are named sets of panels (Board, Projects, Chat, Mailbox,
  Workflows, Roles, Documents, Context, Teams). A user sees the **union** of
  their roles' panels.
- A user with **no** assigned roles sees all standard panels (so existing users
  are never locked out); **admins always see everything**, including the
  admin-only panels (Users, Settings, Agents, Programmes, Summary), which cannot
  be granted through an access role.
- Create/edit roles (tick the panels they grant), then **assign roles to a
  user** in the same panel. The builtin **Member** role grants every standard
  panel and cannot be deleted.
- Hiding a panel also blocks its self-contained API (Chat, Documents); panels
  whose data backs the board (Workflows, Roles, Teams) are hidden in the nav but
  their shared data stays reachable so ticket flows keep working.

## Database upgrades, backups & restore (admin)

`tk` stores everything in a single SQLite database (`$TICKET_HOME/ticket.db` by
default). When you upgrade `tk` to a build with a newer schema, the database is
migrated — **always behind a verified, automatic backup**.

**How upgrades happen**

- **Server startup** auto-upgrades the database when the schema is out of date.
- **`tk admin upgrade-database [-f <db>]`** upgrades a database on demand.
- The **local/CLI path refuses to migrate silently**: if you point the CLI at an
  older database, it stops with a clear error telling you to run
  `tk admin upgrade-database -f <db>` rather than touching your data unexpectedly.

In every case that *does* migrate, the upgrade first takes a backup, verifies it,
and **rolls the database back automatically if the migration fails** — so a
failed upgrade leaves your original database intact.

**What the backup contains and where it lives**

- Before migrating, the WAL is checkpointed into the main file and the database is
  copied to a timestamped file **next to your database**:
  `ticket.db.bak-YYYYMMDD-HHMMSS.<nanos>`. The backup is a self-contained,
  consistent copy (any `-wal`/`-shm` sidecars present are copied too).
- The backup is verified (`PRAGMA integrity_check` + a readable schema version)
  *before* the live database is touched; if verification fails, the upgrade
  aborts without modifying your data.
- The **5 most recent** backups are retained; older ones are pruned only after a
  successful upgrade, so the last known-good backup is never removed.

**Restoring manually**

If you ever need to roll back yourself (the database is corrupt, or you want the
pre-upgrade state):

1. **Stop the `tk` server** (no process may be using the database).
2. Find the newest backup beside your database:
   `ls -t ticket.db.bak-*` — the first entry is the most recent.
3. Remove any stale sidecars and restore the backup over the live file:
   ```bash
   rm -f ticket.db-wal ticket.db-shm
   cp "ticket.db.bak-YYYYMMDD-HHMMSS.<nanos>" ticket.db
   ```
4. Restart the server. (If the backup is older than your current binary's schema,
   run `tk admin upgrade-database -f ticket.db` — which will itself take a fresh
   backup first.)

## More

- Tickets, the board, projects, workflows, roles, and the mailbox are reachable
  from the sidebar or the command palette.
- For setup and the agent/CLI workflow, see
  [DEVELOPER_GUIDE](./DEVELOPER_GUIDE.md) and the docs [index](./INDEX.md).
