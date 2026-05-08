# Tutorial

This tutorial walks through a simple end-to-end setup:

1. start a local Ticket server
2. create a git repository for your project
3. connect that repo with `tk init`
4. add a project, stories, epics, and tickets

It uses the default local server at `http://localhost:8080`.

If you want the same path validated automatically, run:

```bash
make build-dev && ./tests/quickstart_test.sh
```

---

## 1. Start the server

In your first terminal, initialise the shared database and start the server:

```bash
tk initdb
tk server
```

That creates `$TICKET_HOME/ticket.db` (default `~/.ticket/ticket.db`), bootstraps
the initial admin account, and starts the web/API server on
`http://localhost:8080`.

Leave this terminal running.

---

## 2. Create a git repository

In a second terminal, create a new repo for your project:

```bash
mkdir customer-portal
cd customer-portal
git init
```

`tk init` now requires a git repository, so this step is mandatory.

---

## 3. Connect the repo with `tk init`

Run:

```bash
tk init
```

When prompted:

1. choose **Remote server**
2. enter `http://localhost:8080`
3. say you already have an account
4. log in with your admin credentials
5. create a new project when asked

Use values like:

- project prefix: `CUS`
- project name: `Customer Portal`

Ticket will then write `.ticket/config.json` in the repo root so this git repo
is bound to that remote project.

Check the result:

```bash
tk whoami
tk status
```

---

## 4. Add a story

Create a story to group related work:

```bash
tk story create -title "Customer authentication"
tk story ls
```

You can inspect it with:

```bash
tk story get 1
```

---

## 5. Add an epic

Create an epic for a larger area of work:

```bash
tk epic "Sign-in and account recovery"
tk epic ls
```

If you want new tickets to sit under that epic by default:

```bash
tk epic use CUS-1
```

If your epic key is different, use the key shown by `tk epic ls`.

---

## 6. Add some tickets

Now create a few tickets:

```bash
tk new "Users can sign in with email and password"
tk bug "Password reset email link is invalid"
tk new "Add rate limiting to login attempts"
tk ls
```

Inspect one ticket:

```bash
tk get -id CUS-2
```

---

## 7. Optional: create another story and more backlog structure

```bash
tk story create -title "Profile management"
tk epic "Account settings"
tk new "Users can update their display name"
tk new "Users can upload an avatar"
tk ls
```

---

## 8. Open the web UI

With the server still running, open:

```text
http://localhost:8080
```

You can log in with the same account and browse the project in the web UI while
continuing to use the CLI in the repo.

---

## 9. Useful follow-up commands

```bash
tk summary
tk board
tk story ls
tk epic ls
tk history
```

At this point you have:

- a running Ticket server
- a git repo connected with `tk init`
- a remote project bound to that repo
- stories, an epic, and tickets in the backlog
