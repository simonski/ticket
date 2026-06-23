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
| `/new <name>` | Create a room |
| `/join` · `/leave` | Join or leave the current room |
| `/invite <username>` | Add a user to the room (and its project) |
| `/kick <username>` | Remove a user from the room |
| `/list` | List rooms alphabetically with member counts |
| `/msg @username <message>` | Send a direct message |
| `/task [@agent] <description>` | Create a tracked ticket (see below) |

### Breakout rooms from a ticket

Open any ticket and click **💬 Breakout** to open (or create) a chat room scoped
to that ticket — handy for discussing an epic or story in context.

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

## More

- Tickets, the board, projects, workflows, roles, and the mailbox are reachable
  from the sidebar or the command palette.
- For setup and the agent/CLI workflow, see
  [DEVELOPER_GUIDE](./DEVELOPER_GUIDE.md) and the docs [index](./INDEX.md).
