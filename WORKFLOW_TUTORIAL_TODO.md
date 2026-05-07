# Workflow Tutorial: Todo Requirement Through Plan → Implement → Review

This tutorial shows a single requirement moving through a linear workflow with one role in each phase.

Scenario: **“Users can create, complete, and reopen todo items.”**

## 1. Bootstrap

```bash
tk initdb
tk init
```

## 2. Create project + workflow phases

```bash
tk project create -prefix TODO -title "Todo App"
tk project use TODO

WF_ID=$(tk workflow create -name "Todo Delivery" -json | jq -r '.workflow_id')
tk workflow add-stage -id "$WF_ID" -name plan
tk workflow add-stage -id "$WF_ID" -name implement
tk workflow add-stage -id "$WF_ID" -name review
```

Capture stage IDs so we can assign roles:

```bash
PLAN_STAGE_ID=$(tk workflow get -id "$WF_ID" -json | jq -r '.stages[] | select(.stage_name=="plan") | .workflow_stage_id')
IMPLEMENT_STAGE_ID=$(tk workflow get -id "$WF_ID" -json | jq -r '.stages[] | select(.stage_name=="implement") | .workflow_stage_id')
REVIEW_STAGE_ID=$(tk workflow get -id "$WF_ID" -json | jq -r '.stages[] | select(.stage_name=="review") | .workflow_stage_id')
```

## 3. Create one role per phase

```bash
PLAN_ROLE_ID=$(tk role create -title "Planner" -description "Clarifies scope and acceptance criteria" -json | jq -r '.role_id')
IMPLEMENT_ROLE_ID=$(tk role create -title "Implementer" -description "Builds the todo behavior" -json | jq -r '.role_id')
REVIEW_ROLE_ID=$(tk role create -title "Reviewer" -description "Validates behavior and quality" -json | jq -r '.role_id')
```

Assign roles to stages:

```bash
tk workflow stage-role-add -workflow_id "$WF_ID" -stage_id "$PLAN_STAGE_ID" -role_id "$PLAN_ROLE_ID"
tk workflow stage-role-add -workflow_id "$WF_ID" -stage_id "$IMPLEMENT_STAGE_ID" -role_id "$IMPLEMENT_ROLE_ID"
tk workflow stage-role-add -workflow_id "$WF_ID" -stage_id "$REVIEW_STAGE_ID" -role_id "$REVIEW_ROLE_ID"
```

Bind workflow to project:

```bash
tk project workflow "$WF_ID"
```

## 4. Create the todo requirement ticket

```bash
tk add -t requirement "Todo list supports create, complete, and reopen"
TK_ID=$(tk ls -json | jq -r '.[0].ticket_id')
tk get -id "$TK_ID"
```

## 5. Work the ticket through each phase

### Plan

```bash
tk active "$TK_ID"
tk prompt "$TK_ID"
tk success "$TK_ID" -m "Plan approved: clear AC and scope"
tk get -id "$TK_ID"
```

### Implement

```bash
tk active "$TK_ID"
tk prompt "$TK_ID"
tk success "$TK_ID" -m "Implementation complete"
tk get -id "$TK_ID"
```

### Review

```bash
tk active "$TK_ID"
tk prompt "$TK_ID"
tk success "$TK_ID" -m "Review approved and ready to close"
tk get -id "$TK_ID"
```

## 6. Request-based usage loop (agent/user picks work)

Instead of manually selecting a ticket, use request mode:

```bash
tk request -explain
REQ_ID=$(tk request -json | jq -r '.ticket.ticket_id')
tk prompt "$REQ_ID"
```

If no work is available or a prior ticket was rejected, `tk request -explain` gives the reason.

## 7. What this demonstrates

1. Workflow phases are linear: **plan → implement → review**.
2. Each phase has at least one explicit role.
3. A requirement ticket advances by `success` at each phase.
4. The request loop can pull the next ticket to work on without hardcoding IDs.

## 8. Intervention decision when work fails

When work fails, stop automation and record a human decision:

```bash
tk fail -id "$TK_ID"
tk intervene -id "$TK_ID" -outcome split-work -m "Split backend validation into a follow-up ticket"
tk history "$TK_ID"
```

Valid outcomes are: `retry-role`, `retry-stage`, `split-work`, `cancel`.
