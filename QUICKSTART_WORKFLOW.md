# Workflow Quickstart (Minimal Journey)

This is the smallest practical workflow journey.

## 1. Bootstrap

```bash
tk initdb
tk init
```

## 2. CRUD the core entities

```bash
# Project (Create + Read)
tk project create -prefix CUS -title "Customer Portal"
tk project get CUS

# Workflow (Create + Read)
WF_ID=$(tk workflow create -name "Delivery" -json | jq -r '.workflow_id')
tk workflow list
tk workflow add-stage -id "$WF_ID" -name design
```

## 3. Users and agents

```bash
tk user create -username alice -password secret12
tk agent create
tk user list
tk agent list
```

## 4. Bind workflow and create ticket

```bash
tk project use CUS
tk project workflow "$WF_ID"
tk add "Build login page"
```

## 5. Request and execute ticket work

```bash
TK_ID=$(tk request -json | jq -r '.ticket.id')
tk active "$TK_ID"
tk prompt "$TK_ID"
tk success "$TK_ID" -m "done"
```
