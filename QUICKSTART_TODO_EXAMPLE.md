# Quickstart: Todo Example Scenario

This tutorial creates a reproducible **example usage scenario** for `tk` using a
fictional todo application backlog. It does **not** implement a todo app; it
seeds project data only.

## 1. Build `tk`

```bash
make build-dev
```

## 2. Seed the example project

```bash
./scripts/populate_todo_example.sh
```

The script creates:

- project `DEMO` (title: `demo`)
- reference SDLC (`demo-flow-*`) with stage-role assignments
- epic + child tasks + bug
- story, decision, and idea entries
- labels, dependencies, comments, and time entries

It writes a manifest file at `.ticket/demo-example.env` in your active `TICKET_HOME`.

## 3. Inspect the seeded scenario

```bash
source ./.ticket/demo-example.env
./bin/tk status
./bin/tk ls
./bin/tk get -id "$EPIC_ID"
./bin/tk label ls
./bin/tk time total -id "$TASK_API_ID"
./bin/tk story ls
./bin/tk decision ls
```

## 4. Re-run verification

```bash
./scripts/verify_todo_example.sh
```

The scripts operate on your current active ticket database.
