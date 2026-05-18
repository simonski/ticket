# Runbooks

Operational playbooks for common `tk` server scenarios. Each runbook
describes symptoms, diagnosis steps, and resolution.

---

## Contents

1. [Cold start / fresh deployment](#cold-start--fresh-deployment)
2. [Restart after crash](#restart-after-crash)
3. [Database recovery](#database-recovery)
4. [Backup and restore](#backup-and-restore)
5. [User lockout](#user-lockout)
6. [Agent reaper issues](#agent-reaper-issues)
7. [High latency / slow queries](#high-latency--slow-queries)
8. [WebSocket disconnections](#websocket-disconnections)
9. [Disk full](#disk-full)

---

## Cold start / fresh deployment

**When to use:** First-time deployment on a new host.

```bash
# 1. Pull the image
docker pull ghcr.io/simonski/ticket:latest

# 2. Find the runtime UID/GID used by the image
docker run --rm --entrypoint sh ghcr.io/simonski/ticket:latest -c 'id -u ticket && id -g ticket'

# 3. Create and pre-own the persistent data directory
mkdir -p ./data
sudo chown -R <uid>:<gid> ./data
sudo chmod 750 ./data

# 4. Create .env from the deployment template and edit the values
cp env.template .env
$EDITOR .env

# 5. Start the server
docker compose --env-file ./.env -f ./compose.yaml up -d

# 6. Verify health
curl http://localhost:8080/api/healthz
# Expected: {"status":"ok","version":"0.1.x"}

# 7. Configure CLI environment for the running server
export TICKET_URL=http://localhost:8080
export TICKET_USERNAME=admin
export TICKET_PASSWORD=<secret>
export TICKET_PROJECT=1

# 8. Create the first project
tk project new -title "My Project" -prefix MP
```

On first boot the container creates `/data/ticket.db` and bootstraps the
`admin` account. Set `TICKET_ADMIN_PASSWORD` in `.env` before the first boot and
store that secret outside source control.

**Checklist before going live:**
- [ ] `TICKET_ADMIN_PASSWORD` was set before the first boot and recorded in your secret store
- [ ] TLS termination is configured at the reverse proxy
- [ ] Daily backup cron is scheduled (see [Backup and restore](#backup-and-restore))
- [ ] Docker resource limits are set in `compose.yaml`

---

## Restart after crash

**Symptoms:** Container exited unexpectedly; `docker compose ps` shows `Exited`.

```bash
# Check exit code and recent logs
docker compose --env-file ./.env -f ./compose.yaml logs --tail 50 ticket

# Check for OOM kill
dmesg | grep -i "killed process"

# Restart
docker compose --env-file ./.env -f ./compose.yaml up -d

# Verify recovery
curl http://localhost:8080/api/healthz
```

**If the container keeps crashing:**
1. Check disk space: `df -h /var/lib/docker`
2. Check SQLite WAL file: if `ticket.db-wal` is much larger than `ticket.db`,
   the WAL may be corrupted — see [Database recovery](#database-recovery)
3. Check for port conflict: `ss -tlnp | grep 8080`

---

## Database recovery

**Symptoms:** Server fails to start with a SQLite error; data appears missing.

### Step 1 — Check WAL integrity

```bash
# Copy the DB out of the container
docker cp ticket:/data/ticket.db /tmp/ticket-recovery.db
docker cp ticket:/data/ticket.db-wal /tmp/ticket-recovery.db-wal 2>/dev/null || true

# Run integrity check
sqlite3 /tmp/ticket-recovery.db "PRAGMA integrity_check;"
# Expected: ok
```

### Step 2 — Checkpoint the WAL

```bash
sqlite3 /tmp/ticket-recovery.db "PRAGMA wal_checkpoint(TRUNCATE);"
```

### Step 3 — If integrity check fails, restore from backup

See [Backup and restore](#backup-and-restore).

### Step 4 — Recover from export

If no backup exists but the server can start:

```bash
tk export -o /tmp/ticket-export-$(date +%Y%m%d).json
# Then restore into a fresh DB:
tk initdb
tk import -i /tmp/ticket-export-$(date +%Y%m%d).json
```

---

## Backup and restore

### Automated daily backup (recommended)

The repo ships an automation script:

```bash
make backup-db
```

This calls `scripts/backup_ticket_db.sh`, which:

1. Exports a snapshot with `tk export -o`.
2. Compresses it to `ticket-YYYYMMDD-HHMMSS.json.gz`.
3. Prunes backups older than `KEEP_DAYS` (default: 30).

Optional environment overrides:

```bash
BACKUP_DIR=/var/backups/ticket KEEP_DAYS=45 make backup-db
```

Cron example:

```bash
# /etc/cron.daily/ticket-backup
#!/bin/bash
cd /srv/ticket
BACKUP_DIR=/var/backups/ticket KEEP_DAYS=30 make backup-db
```

### Manual backup

```bash
tk export | gzip > ticket-backup-$(date +%Y%m%d).json.gz
```

### Restore from backup

```bash
# Stop the server
docker compose --env-file ./.env -f ./compose.yaml stop ticket

# Restore the snapshot payload to a temporary file
gunzip -c ticket-backup-20250718.json.gz > /tmp/ticket-restore.json

# Re-import into the local database used by the current TICKET_HOME
tk import -i /tmp/ticket-restore.json

# Restart
docker compose --env-file ./.env -f ./compose.yaml start ticket

# Verify
curl http://localhost:8080/api/healthz
tk project ls
```

> **Note:** `tk import` replaces the local database contents with the snapshot
> in the file you pass via `-i`.

### Client/server restore checklist

1. Stop any running `tk server` process pointed at the same local database.
2. Confirm `TICKET_HOME` points at the target global home and that you are in the repo or directory whose `.ticket/config.json` should route to that database.
3. Run `tk import -i /path/to/snapshot.json`.
4. Run `tk status` and `tk project ls` to confirm the workspace reopened cleanly.

### Server mode restore checklist

1. Stop the container or service so no writers are active.
2. Copy or extract the backup payload onto the host.
3. Run `tk import -i /path/to/snapshot.json` in the same `TICKET_HOME` used by the server container or process.
4. Restart the server and confirm `/api/healthz`, `/api/status`, and a basic `tk project ls` call succeed.

---

## User lockout

**Symptoms:** A user cannot log in; IP rate limit or account disabled.

### Check account status

```bash
tk admin user ls | grep "<username>"
# Confirm the user exists and whether it needs re-enabling.
```

### Re-enable a disabled account

```bash
tk admin user enable -username <username>
```

### Reset a password

```bash
tk admin user reset-password -username <username> -password <new-password>
```

### Clear IP rate limit (if triggered by brute-force protection)

The rate limiter resets after 1 minute. If a legitimate user is blocked:
1. Wait 60 seconds and retry.
2. If using a reverse proxy, check that `X-Forwarded-For` is being set
   correctly — a shared proxy IP can cause false positives.
3. The current limiter still keys from the observed remote address, so a shared
   proxy IP can affect multiple users until the limiter window expires.

---

## Agent reaper issues

**Symptoms:** Agents stuck in `active` state after their process has exited;
new agent runs fail because the limit is reached.

The agent reaper runs every 10 minutes and marks agents `idle` if they haven't
sent a heartbeat (`TouchAgent`) within the threshold.

### Force reaper run (restart server)

```bash
docker compose --env-file ./.env -f ./compose.yaml restart ticket
# Reaper runs immediately on startup
```

### Manually reset a stuck agent

```bash
tk agent ls
```

There is currently no dedicated CLI command to force an agent back to `idle`.
Use the restart path above to trigger the reaper immediately, or wait for the
next reaper cycle.

### Increase reaper threshold

The threshold is 10 minutes (hardcoded in `internal/server/server.go:44`).
If agents legitimately run longer, increase the threshold and rebuild.

---

## High latency / slow queries

**Symptoms:** API responses are slow (>500ms); web UI feels sluggish.

### Check ticket count

```bash
tk ls -json | jq length
```

Large projects (>10,000 tickets) will be slow on unbounded list queries. Apply
a server-side limit with `-n 100` while investigating.

### Check disk I/O

```bash
iostat -x 1 10
```

SQLite is I/O-bound. If the disk is saturated, consider:
- Moving the data volume to faster storage
- Enabling WAL mode (already the default)
- Enabling `TICKET_HISTORY_RETENTION_DAYS` to prune old history events

### Check index coverage

The following indexes should exist (verify with `.schema` in sqlite3):

```
idx_tickets_open, idx_tickets_archived, idx_tickets_status, idx_tickets_type
idx_project_members_user_id, idx_team_members_user_id, idx_team_agents_user_id
idx_ticket_labels_ticket_id, idx_users_username
```

If any are missing, the migration was not applied. Restart the server — it
runs migrations on every startup.

---

## WebSocket disconnections

**Symptoms:** Web UI shows "reconnecting"; live ticket updates stop arriving.

### Check server logs

```bash
docker compose --env-file ./.env -f ./compose.yaml logs --tail 100 ticket | grep -i "websocket\|ws\|realtime"
```

### Common causes

| Cause | Fix |
|-------|-----|
| Nginx/proxy timeout | Set `proxy_read_timeout 3600s` and `proxy_send_timeout 3600s` |
| Load balancer idle timeout | Set ALB/NLB idle timeout to 3600s |
| Client network interruption | Web UI auto-reconnects; no action needed |
| Server crash | See [Restart after crash](#restart-after-crash) |

### Validate Origin header config

WebSocket upgrades are rejected if the `Origin` header doesn't match the
server `Host`. If behind a reverse proxy, ensure the proxy forwards the
correct `Host` header.

---

## Disk full

**Symptoms:** Server crashes or returns 500 errors; Docker logs show "no space left on device".

```bash
# Check disk usage
df -h
du -sh ./data

# Prune old backups
find /var/backups/ticket -name "*.json.gz" -mtime +30 -delete

# Prune Docker
docker system prune -f

# If the WAL file is large, checkpoint it
docker exec ticket sqlite3 /data/ticket.db "PRAGMA wal_checkpoint(TRUNCATE);"

# Enable history retention to prevent unbounded growth
# Set TICKET_HISTORY_RETENTION_DAYS=180 in your compose.yaml environment
```
