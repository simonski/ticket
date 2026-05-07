# Workflow User Guide (Minimal)

This guide is intentionally small: CRUD + users/agents + workflow ticket request.

## Entity model (minimal)

1. Project
2. Workflow
3. Stage
4. Ticket
5. User
6. Agent

## Minimal command set

### Create / Read

```bash
tk project create -prefix CUS -title "Customer Portal"
tk project get CUS

WF_ID=$(tk workflow create -name "Delivery" -json | jq -r '.workflow_id')
tk workflow list
tk workflow add-stage -id "$WF_ID" -name design

tk add "Build login page"
TK_ID=$(tk ls -json | jq -r '.[0].id')
tk get "$TK_ID"
```

### Update

```bash
tk project workflow "$WF_ID"
tk update -id "$TK_ID" -stage design -state active
```

### Delete

```bash
tk workflow rm -id "$WF_ID" -check
tk rm -id "$TK_ID"
```

### Users and agents

```bash
tk user create -username alice -password secret12
tk user list

tk agent create
tk agent list
```

### Request-ticket workflow loop

```bash
tk request
tk request -explain
TK_ID=$(tk request -json | jq -r '.ticket.id')
tk active "$TK_ID"
tk prompt "$TK_ID"
tk success "$TK_ID" -m "completed"
```

## Review focus

1. Is this the smallest usable journey?
2. Are command names and order clear?
3. Is any command unnecessary for day-1 usage?
