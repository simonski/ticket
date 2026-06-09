# Releases

A **Release** is the top-level delivery container for a project. It groups the
work designed for a delivery, then seals it and hands it to the orchestrator for
execution. Releases replace the old "sprint" model.

## The delivery hierarchy

```
Release  →  Feature  →  Epic  →  Story / Bug
```

- A **Release** holds one or more features.
- A **Feature** (ticket type `feature`) is the "grand plan" / requirement. It is
  refined together with a human and an agent, then broken down.
- A **Feature** contains **Epics** (ticket type `epic`); an **Epic** contains
  **Stories** / **Bugs** (`story` / `bug`).
- Hierarchy is expressed through `parent_id`: `story.parent = epic`,
  `epic.parent = feature`.
- Every ticket carries a `release_id`. Adding a feature to a release propagates
  that `release_id` across the feature's whole subtree.

## Refining and breaking down a feature

A feature starts as a requirement. Through refinement (a human + agent dialogue
on the ticket — see [`DESIGN_ORCHESTRATOR.md`](./DESIGN_ORCHESTRATOR.md)) the
feature is clarified until it is unambiguous, then **broken down** into epics and
their stories/bugs. The leaf stories become `ready` and the feature is ready to be
added to a release.

## Cloning a feature

A feature can be **cloned** — a deep copy of its entire epic/story subtree — to
extend or vary functionality without disturbing the original.

```bash
tk feature clone <feature-id>      # POST /api/tickets/{id}/clone
```

## Release lifecycle

A release moves through three states:

```
in_design  →  in_progress  →  complete
```

| Status | Meaning | Timestamp |
|--------|---------|-----------|
| `in_design` | Being assembled. Features may be added to or removed from it. | `designed_at` |
| `in_progress` | **Sealed**. The feature set is frozen; the orchestrator executes its ready stories. | `started_at` |
| `complete` | Terminal. The release has shipped. | `completed_at` |

A release also exposes a `title`, a `purpose`, an aspirational `target_date`, and
derived `feature_count` / `story_count`.

### Adding features (only while `in_design`)

A feature can be added to a release **only while the release is `in_design`**, and
only once the feature is at least `ready` (non-draft). Adding it propagates the
`release_id` to the feature's epics and stories. While the release is `in_design`,
features may also be removed (clearing their `release_id`).

```bash
tk release add-feature <release-id> <feature-id>   # PUT /api/tickets/{id}/release {release_id}
tk release remove <release-id> <feature-id>        # PUT /api/tickets/{id}/release {release_id:null}
```

### Sealing (`in_progress`)

Moving a release to `in_progress` **seals** it: the feature set is frozen and the
ready stories across all its features become available for execution. This is the
release-model equivalent of the old "sealed sprint".

```bash
tk release status <release-id> in_progress
```

### Execution by the orchestrator

The deterministic orchestrator keys its execution loop on releases that are
`in_progress`. Once a release is sealed it sweeps the release's leaf stories: any
that are idle, unassigned, and have a role-matching free agent are pushed to that
agent; on `success` the story advances through its workflow stages/roles, on `fail`
it bounces back. The orchestrator never works the stories of a release that is
still `in_design`. (A ticket at `ready` cannot advance at all unless it belongs to
a release — see [`LIFECYCLE.md`](./LIFECYCLE.md).)

When the work is done, the human approves the feature/release as compliant, or
sends it back for re-refinement; the release is then moved to `complete`.

## CLI summary

| Command | Effect |
|---------|--------|
| `tk release list` | List releases for the active project |
| `tk release create -title "..." [-purpose "..."] [-target-date ...]` | Create a release (`in_design`) |
| `tk release update <id> -title "..."` | Update title / purpose / target date |
| `tk release status <id> <in_design\|in_progress\|complete>` | Transition status (seal / complete) |
| `tk release add-feature <release-id> <feature-id>` | Add a feature (and subtree) to a release |
| `tk release remove <release-id> <feature-id>` | Remove a feature from a release |
| `tk release delete <id>` | Delete a release |
| `tk feature clone <feature-id>` | Deep-clone a feature and its subtree |

## REST API summary

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/projects/{projectRef}/releases` | List releases |
| POST | `/api/projects/{projectRef}/releases` | Create a release (201) |
| PUT | `/api/releases/{id}` | Update title / purpose / target_date |
| POST | `/api/releases/{id}/status` | Set status (`in_design`/`in_progress`/`complete`) |
| DELETE | `/api/releases/{id}` | Delete a release (`{ok:true}`) |
| PUT | `/api/tickets/{id}/release` | Add feature to / remove from a release (`{release_id}` or `null`) |
| POST | `/api/tickets/{id}/clone` | Deep-clone a feature (201) |

See the full definitions in [`api/openapi.yaml`](./api/openapi.yaml).
