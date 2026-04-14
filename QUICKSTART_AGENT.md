# Agent Quickstart

This guide shows how to run `tk` as an autonomous agent worker, plus dry-run patterns that let you simulate behavior without changing ticket state.

## 1. Talk to a server

As an `admin`, ensure you have access to a `tk server` instance:

```bash
# terminal 2
export TICKET_URL=http://ticket.localhost
export TICKET_USERNAME=admin
export TICKET_PASSWORD=password
tk status
```

## 2. Create agent credentials

Using your `admin` user, create some agents

```bash
tk agent create
# prints:
# agent_id: <uuid>
# password: <password>

tk agent create 
# prints:
# agent_id: <uuid>
# password: <password>

```

```bash
export AGENT_ID=<uuid>
export AGENT_PASSWORD=<password>
```

## 3. Dry-run (no ticket mutation)

Use `tk agent request -dryrun` to simulate assignment only.

```bash
# single simulated request
tk agent request -dryrun

# simulate repeatedly (every 2s, 10 loops)
tk agent request -dryrun -loop 10 -sleep 2

# target one specific ticket, still no mutation
tk agent request -dryrun -id TK-123
```

`-dryrun` is the safest way to test routing/eligibility without the agent claiming or updating tickets.

## 4. Real agent run (does work)

```bash
tk agent run -v
```

- `-v` streams LLM command/input-output logs.
- `-poll-seconds` controls idle polling interval (default `5`).
- `-project-id` limits work to one project.

## 5. LLM-specific invocation examples

### Claude CLI

```bash
export TICKET_AGENT_LLM=claude
tk agent run -v
# or:
tk agent run -llm claude -v
```

### GitHub Copilot (Codex CLI)

```bash
export TICKET_AGENT_LLM=codex
tk agent run -v
# or:
tk agent run -llm codex -v
```

### Local OpenCode + Ollama

`tk` allows only approved LLM binary names. Add `opencode` to the allow-list, then run it as the agent LLM.

```bash
export TICKET_AGENT_ALLOWED_LLM_BINARIES=opencode
export TICKET_AGENT_LLM=opencode
export OLLAMA_HOST=http://127.0.0.1:11434
tk agent run -v
```

Equivalent flag form:

```bash
TICKET_AGENT_ALLOWED_LLM_BINARIES=opencode tk agent run -llm opencode -v
```

## 6. Practical safe workflow

Use this sequence when validating a new setup:

```bash
# 1) verify connectivity/auth
tk status

# 2) dry-run assignment behavior only
tk agent request -dryrun -loop 3 -sleep 1

# 3) then enable real execution
tk agent run -llm claude -v
```

## 7. Common errors

- `agent run requires remote mode`: set `TICKET_URL` to `http(s)://...`
- `missing required values`: set `AGENT_ID` and `AGENT_PASSWORD`
- `llm binary ... is not in the allow-list`: add it to `TICKET_AGENT_ALLOWED_LLM_BINARIES`
