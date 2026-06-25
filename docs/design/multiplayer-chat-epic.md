# Design: Multiplayer chat — power UX & ephemeral agent tasking

Epic: **TK-166**. Stories: **TK-167** (A), **TK-168** (B), **TK-169** (C), **TK-170** (D).

This document is committed to the epic branch **before** implementation, per the
project convention. Each story branches from the epic branch with its own PR; the
epic merges to `main` only once all stories are complete.

## Context: what already exists

The original `SLACK_MULTIPLAYER_CHAT` brief is largely shipped. Do **not** rebuild:

| Capability | Where |
|---|---|
| Rooms (create/join/leave, public/private, scopes: global/project/breakout/DM) | `internal/store/room.go`, `internal/server/api_rooms.go` |
| Humans **and** agents as room members | room members + user `type="agent"` |
| Agent tasking → permanent ticket | `/task [@assignee] <desc>` (`api_rooms.go` `handleRoomCommand`) |
| Double-shift command palette + slash aliases (`/c /chat /backlog` …) | `web/shared/app.js` (~9715) |
| `@name` / `#label` highlighting, mention parsing | `highlightRoomText`, `renderChatMessage` |
| `/tk-NNN` ticket lookup → shallow action menu | palette ticket lookup |
| Per-room realtime delivery | `internal/server/realtime.go` (`liveHub`, `/api/ws`) |
| Agent replies (CLI/API command + model resolution) | `internal/server/room_agent.go` |

The epic covers the four genuine gaps below.

---

## Story A (TK-167): Composer power transforms

Sed-style substitution in the chat composer: `s/old/new/` (first match),
`s/old/new/g` (global); consider an `i` (case-insensitive) flag.

- **Scope (starting):** composer text only. Parse into a `{pattern, replacement,
  flags, target}` shape so a future `target` (selected ticket field / message) can be
  added without rework. The target question is recorded as resolved-to-composer-only.
- **Safety:** invalid regex → visible inline error, composer text left **unchanged**.
  Never silently mangle input.
- **Surface:** pure front-end in `web/shared/app.js` (composer input + keydown near
  `app.js:2596`). No API change for composer-only scope.

**Resolved scope (TK-167):** the substitution targets your **previous composer
message** (the last entry in the composer history) and loads the corrected text
back into the composer for review — it is **not** auto-sent and does **not** edit a
server-side message. This is the recognisable "sed correction" chat idiom while
staying composer-only (no message-edit API). Invalid regex or no prior message →
visible inline notice, composer left unchanged. The parser returns `{pattern,
replacement, flags}` so a future explicit `target` (a selected ticket field) can be
layered on without rework. Delimiter is any punctuation after `s` (so `s|/|-|g`
works); flags limited to `g`/`i`.

---

## Story B (TK-168): Ephemeral agent backlog

The middle lane between a one-shot `@agent` reply and a permanent `/task` ticket.

### Trigger rule (deterministic, no NLP)
- `@agent do X` / `@agent queue X` → **enqueue** an ephemeral work-item.
- any other `@agent X` → existing **one-shot reply**.

### Data model (new table)
Work-item: `{id, room_id, agent_id, instruction, state, result, ephemeral,
created_at, updated_at}` where `state ∈ {queued, running, done, failed}`.
- `ephemeral` flag + auto-purge of `done`/expired items → never appears in the
  permanent ticket backlog.
- Persisted (not in-memory) so an in-flight queue survives a server restart.

### Worker
- Serial, **concurrency 1 per `(room, agent)`**, FIFO.
- Reuses `room_agent.go` command/model resolution to execute each instruction.
- Posts progress/result back into the room; updates item state.

### UI — live queue panel
- Small panel in the chat room: **pending / running / done**.
- Updated over the existing `liveHub` WebSocket fanout (new event type), no reload.

### Promotion
- `/promote <item>` converts a temp item → real ticket via the existing `/task`
  ticket-creation path. One-way escape hatch.

```
@agent do X ─▶ enqueue (state=queued, ephemeral=1)
                   │
                   ▼  serial worker, 1 per (room,agent)
              state=running ──▶ execute via room_agent resolution
                   │                     │
                   ▼                     ▼
          live panel updates       post result to room
          over WebSocket                │
                   ▼                     ▼
              state=done ──▶ auto-purge (expired)
                   │
                   └─▶ /promote ──▶ real ticket via /task path
```

Reuses: room membership, WS hub, `room_agent.go`, `/task` path.
New: queue table + worker loop + panel + enqueue rule + `/promote`.

---

## Story C (TK-169): Deeper `/tk-NNN` palette action stack

Expand the shallow palette ticket-action menu into a richer numbered stack.

- `/tk-NNN` → ticket summary + **numbered** action list; selectable by number keys
  and arrow keys + Enter.
- **ESC pops one frame** (back one level) in the conceptual UX stack; the palette
  only closes at the root frame. (Frame-popping already exists in the palette.)
- Action set to finalise in design: open detail, comment, change stage/state, claim,
  assign, copy key, open in chat/breakout room — wired to existing ticket API
  endpoints. Cancel is a no-op (no data loss).
- Front-end focused (`app.js` ~9715), reusing existing endpoints.

**Resolved (TK-169):** the palette already had the frame stack, ESC frame-pop, and
number/arrow selection (TK-127/TK-130). This story made action menus support nested
sub-frames (`{label, submenu}`) and expanded the ticket actions to: Open detail, Add
comment…, **Lifecycle…** (a pushed sub-frame: next / previous / ready / complete /
reopen / close), Claim, Open in chat (breakout), Copy key — all wired to existing
`POST /api/tickets/{key}/{action}` and `/api/tickets/claim`. ESC pops one level
(submenu → ticket actions → command list → close).

---

## Story D (TK-170): Room presence & typing indicators

Ephemeral, derived purely from live WebSocket connections — nothing persisted.

- **Presence:** `liveHub` already tracks per-client `roomID` (`realtime.go`). Surface
  a per-room online list to subscribers; update on join/leave/disconnect.
- **Typing:** composer emits a debounced `typing` event over WS; server fans it to
  room subscribers; UI shows a transient "X is typing…" that auto-expires.
- New: presence tracking on the hub, a typing event type, presence/typing UI in the
  room header/footer.

---

## Sequencing

D and A are low-risk and independent — good warm-ups. C builds on existing palette
internals. B is the largest (schema + worker + UI + commands) and should land last so
presence/panel patterns from D are available to reuse. Each story is independently
shippable into the epic branch.
