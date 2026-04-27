TK_INIT

tk initdb
  initialise a database file in ~/.ticket/ticket.db
  or initialise a database file the locaiton at -f (tk initdb -f filename.db)

tk server
  run the server using the db at ~/.ticket/ticket.db

tk server -f filename.db
  run the server using the db at filename.db

tk init
  intent is to make this location map to a project
  using ${CWD} create a new project  (or explain it is already setup)
  use the local .git as the git location
  use the local dirname as the project name
  use the prefix as the first 3 letter of the project name
  create the project
  update the ${CWD}/.ticket/config.json to contain the project id
  prompt the user for all the above

tk init -url http://ticket.exe.xyz
  the intent is to make this a project (or be part of the project) at the endpoint
  the endpoint is just the server, now we figure out the project id
  if it is a git repo, then the origin must be the id
  if it is not, then what to do? go into a nice tui mode to look at the various and do that
  including what... login via the tui and hold credentials in the ~/.ticket/credentials?



GOAL

Reimagine the design and implementation of:

1. `tk init` / `tk initdb`
2. running the server
3. where the database lives, how it is accessed, and how `tk` decides which project is currently in scope

This file is an ideas document. The next step is to turn this into a coherent design and then an implementation plan against the current codebase and documentation.

The end result should be:

- functioning code with a working test harness
- work done using red/green
- complete, correct documentation across README, QUICKSTART, and developer-facing docs

OBJECTIVES

- setup and day-to-day use should be extremely low friction
- `tk` should try to infer user intent instead of forcing repeated manual setup
- project detection should prefer Git context when available

PROPOSED DIRECTION

## 1. Central database by default

One strong option is for `tk initdb` or first-time setup to create a single central database at:

`~/.ticket/ticket.db`

This would give the user one database rather than one database per repository.

If we do this, the design should lean heavily into projects as the way work is separated and scoped.

This should also be the default first-run storage location when the user has just installed `tk` and has not yet set anything up.

Example first-run flow:

```text
brew install ticket
cd ~/code/project-1
tk new "foo"
```

In that situation, if all of the following are true:

- there is no `~/.ticket/ticket.db`
- there is no `.ticket/config.json` in the current directory or any parent
- the user is not already bound to a remote-backed project

then the expected UX should be:

1. explain briefly that Ticket is not set up yet
2. create `~/.ticket/ticket.db`
3. create a project in that central database for the current repo or directory
4. write `.ticket/config.json` in the repo or directory root
5. create the requested ticket

The important rule is:

- ticket data is stored in the central local database by default
- the repo or directory stores only the routing marker and project mapping metadata

## 2. Project inference from the current directory

For any given directory or Git repository, `tk` should infer the active project automatically.

Preferred approach:

- first, detect whether the user is inside a Git repository
- if so, walk up to the repository root
- inspect the Git remote and use that repository identity as the primary project marker

This should let the user work naturally:

```text
tk init
  -> initializes the central ~/.ticket/ticket.db

cd ~/code/project-1
tk new "foo"
  -> tk detects project-1 and uses that project automatically

cd ~/code/project-x
tk new "bar"
  -> tk detects project-x and uses that project automatically
```

## 3. Behaviour outside Git repositories

If the user is not in a Git repository, `tk` should still try to help rather than just fail.

Example:

```text
cd ~
tk ls
```

In that case `tk` could:

- explain that the current directory is not mapped to a Ticket project
- list existing projects
- offer a path to "ticketify" the current directory

That mapping could be based on either:

- the directory location itself
- a local `.ticket/` directory containing `.ticket/config.json`

The important idea is that plain (that is, non-git) directories should have a workable fallback when Git is not available.

For first-run use outside Git, the default should still be the same central local database at `~/.ticket/ticket.db`.

The only difference is that `tk` should probably ask for a quick confirmation before writing `.ticket/config.json` into a plain directory.

## 4. Favour helpful auto-creation

In all cases, `tk` should try to do what the user is obviously trying to do.

If the user runs something like:

```text
tk new "foo"
```

and there is no project available for the current location, `tk` should prefer to guide the user into creating a project and then performing the add instead of making them restart the workflow manually.

That could mean a short wizard such as:

- "This directory is not yet managed by Ticket."
- "Let us create a project for it now."
- prefill sensible defaults derived from directory name, Git remote, and location

For the common first-call case, the wizard or prompt should steer toward:

- creating `~/.ticket/ticket.db` if it does not already exist
- creating the project in that central database
- writing `.ticket/config.json` locally
- then continuing with the original command

## 5. Meaning of `tk init`

`tk init` should be treated primarily as a signal that the user wants to set up Ticket for the current location.

That likely means:

- ensure the main database exists
- resolve the current location
- create or connect the appropriate project for that location
- store enough metadata that future `tk` commands can infer project scope automatically

## 6. Directory walk-up matters

Commands will often be run from nested subdirectories, not only from a repository root.

Because of that, `tk` should first walk up from the current working directory to find the nearest enclosing mapped project or Git repository.

Example:

- user is three directories deep inside a repository
- user runs `tk new "foo"`
- `tk` should infer the repository-level project, not create a new nested project for the subdirectory

Generally the walk-up should work like this:

- if `.ticket/config.json` is found, stop there and use it as the source of truth
- otherwise, if a `.git` directory is found, stop there and use Git context to derive defaults or offer `tk init`

## 7. Explicit user overrides still win

Autodetection should not override an explicit user choice.

If the user forcibly provides a location or database path during initialization, that explicit input should take precedence over inferred defaults.

## 8. Hard rule

One Git repository maps to one Ticket project.

A single repository must not resolve to multiple Ticket projects.

NEXT STEP

Review the above, clarify any remaining design choices, and then turn it into a concrete design and implementation plan against the current codebase.

Keep the write-up plain and direct. Avoid emojis and avoid decorative or cosmetic noise.

## 9. Local mode and remote mode must feel similar

The ideas above mainly describe local mode:

- direct access to a SQLite database
- usually one person working in one shell at a time

Remote mode is different:

- SQLite is hidden behind the server
- access happens over HTTP or HTTPS
- multiple users may be active at the same time

Concurrency and SQLite tuning are not the main concern for this design pass.

The main concern is preserving the same user workflow as much as possible, while adding the extra information remote mode requires.

That extra information is:

- server URL
- username
- password or stored credential/token

## 10. Remote login should grant capability, not change project routing globally

The user should be able to do something like:

```text
tk login -url https://ticket.exe.dev
username: my-username
password: ********
```

That should authenticate the user against the remote server and store credentials centrally, probably under `~/.ticket/credentials.json` or an equivalent location.

The key design point is this:

Logging in to a remote server should mean that the `tk` binary is now capable of talking to that remote.

It should not mean that every directory the user enters automatically becomes remote.

The user may have a mixed world:

- some local Ticket-managed projects
- some remote Ticket-managed projects

That must be supported cleanly.

## 11. Project-local metadata should decide the backend

If the user does this:

```text
cd ~/code/project-1
tk ls
tk new "zxed"
```

and `project-1` is a local project, it should remain local even if the user has previously logged in to a remote server.

That implies that each Ticket-managed area needs local metadata that says what is actually controlling tickets for that area.

A likely direction is a project-local `.ticket/` directory at the repository or mapped-directory root, with `.ticket/config.json` as the main config file.

`.ticket/config.json` should record at least:

- whether the project is local or remote
- the controlling location:
  - local database path, or
  - remote server URL
- the resolved Ticket project identity on that backend

This would let `tk` behave consistently:

- enter a local repo -> use the local database and local project
- enter a remote-backed repo -> use the remote server and remote project
- be logged in to a remote server without affecting unrelated local repos

## 12. Suggested split of responsibilities

The current direction suggests three layers of state:

1. global machine state
   - main local database location
   - stored remote credentials

2. project-local state
   - whether this area is local or remote
   - where its tickets live
   - which project it maps to

3. runtime resolution
   - walk up from the current directory
   - find the nearest enclosing Ticket-managed root
   - load its backend mapping
   - then execute the command against that backend

## 13. Likely design conclusion

The simplest coherent model may be:

- one central local database by default
- one project per Git repository
- a local `.ticket/` directory at the project root with `.ticket/config.json` to bind that area to either:
  - the local database, or
  - a remote server
- remote credentials stored centrally, not in the project file

Under this model, the first ticket created by a brand-new user should land in `~/.ticket/ticket.db`, not in a repo-local SQLite database.

If this holds, then `tk init` becomes mostly about setting up the current directory or repository so future commands can resolve the correct backend and project automatically.

NEXT DESIGN QUESTION

Define exactly what `.ticket/config.json` should contain, how the `.ticket/` marker is discovered, and how it interacts with existing global config and credentials.

## 14. `.ticket/` as the local project marker

The current preferred direction is:

- `.ticket/` is the repo-local or directory-local marker
- `.ticket/config.json` is the non-secret project routing config
- `~/.ticket/credentials.json` is the user-level credential store for remote access

This keeps responsibilities clean:

- repo-local config explains what backend controls this project
- user-level credentials explain who the client is when talking to a remote server

That means credentials must never be written into `.ticket/config.json`.

The server remains the source of truth for actual user records, password hashes, roles, and permissions.

The client machine only needs enough information to:

- discover whether this project is local or remote
- find the local database path or remote server URL
- identify the project on that backend

For purely local mode, no remote credentials are needed at all.
