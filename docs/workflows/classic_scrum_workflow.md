# Classic Scrum Workflow

Import file: `docs/workflows/classic_scrum_workflow.json`

## Create required roles

```bash
tk role create -title "Product Owner"
tk role create -title "Business Analyst"
tk role create -title "Solution Architect"
tk role create -title "Software Engineer"
tk role create -title "Senior Engineer"
tk role create -title "QA Engineer"
tk role create -title "Scrum Master"
```

## Import the workflow

```bash
tk workflow import -f docs/workflows/classic_scrum_workflow.json
tk workflow list
```

## Attach it to your project

```bash
tk project use TODO
WF_ID=$(tk workflow list -json | jq -r '.[] | select(.name=="Classic Scrum Delivery") | .workflow_id')
tk project workflow "$WF_ID"
```
