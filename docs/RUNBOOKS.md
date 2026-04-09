# Runbooks

Operational playbooks for common `ticket` server scenarios. Each runbook
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
# 1. Pull the image (or build from source)
docker pull ghcr.io/simonski/ticket:latest

# 2. Create a data volume
docker volume create ticket-data

# 3. Start the server
docker compose up -d

# 4. Verify health
curl http://localhost:8080/api/healthz
# Expected: {"status":"ok","version":"0.1.x"}

# 5. Create the first admin user (via CLI connecting to the running server)
ticket register --server http://localhost:8080 --username admin --password <secret>

# 6. Create the first project
ticket project new --server http://localhost:8080 --title "My Project" --prefix MP
```

**Checklist before going live:**
- [ ] `TICKET_ENCRYPTION_KEY` is set (required for email encryption at rest)
- [ ] TLS termination is configured at the reverse proxy
- [ ] `TICKET_SESSION_EXPIRY_DAYS` is set to an appropriate value (default: 30)
- [ ] Daily backup cron is scheduled (see [Backup and restore](#backup-and-restore))
- [ ] Docker resource limits are set in `compose.yaml`

---

## Restart after crash

**Symptoms:** Container exited unexpectedly; `docker compose ps` shows `Exited`.

```bash
# Check exit code and recent logs
docker compose logs --tail 50 ticket

# Check for OOM kill
dmesg | grep -i "killed process"

# Restart
docker compose up -d

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
docker cp ticket:/home/ticket/.ticket/ticket.db /tmp/ticket-recovery.db
docker cp ticket:/home/ticket/.ticket/ticket.db-wal /tmp/ticket-recovery.db-wal 2>/dev/null || true

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
tk export > /tmp/ticket-export-$(date +%Y%m%d).json
# Then restore into a fresh DB:
tk initdb
tk import /tmp/ticket-export-$(date +%Y%m%d).json
```

---

## Backup and restore

### Automated daily backup (recommended)

Add this cron job on the host (or as a sidecar container):

```bash
# /etc/cron.daily/ticket-backup
#!/bin/bash
BACKUP_DIR=/var/backups/ticket
mkdir -p "$BACKUP_DIR"
docker exec ticket tk export | gzip > "$BACKUP_DIR/ticket-$(date +%Y%m%d-%H%M%S).json.gz"
# Keep last 30 days
find "$BACKUP_DIR" -name "*.json.gz" -mtime +30 -delete
```

### Manual backup

```bash
tk export | gzip > ticket-backup-$(date +%Y%m%d).json.gz
```

### Restore from backup

```bash
# Stop the server
docker compose stop ticket

# Restore
gunzip -c ticket-backup-20250718.json.gz | tk import --overwrite

# Restart
docker compose start ticket

# Verify
curl http://localhost:8080/api/healthz
ticket project ls
```

> **Note:** `tk import` uses upsert semantics — it does not delete existing
> data. Use `--overwrite` to replace records that already exist.

---

## User lockout

**Symptoms:** A user cannot log in; IP rate limit or account disabled.

### Check account status

```bash
ticket user get --username <username>
# Look for: enabled: false
```

### Re-enable a disabled account

```bash
ticket user enable --username <username>
```

### Reset a password

```bash
ticket user password --username <username> --password <new-password>
```

### Clear IP rate limit (if triggered by brute-force protection)

The rate limiter resets after 1 minute. If a legitimate user is blocked:
1. Wait 60 seconds and retry.
2. If using a reverse proxy, check that `X-Forwarded-For` is being set
   correctly — a shared proxy IP can cause false positives.
3. Configure `TRUSTED_PROXIES` env var to only trust forwarded IPs from your
   known proxy.

---

## Agent reaper issues

**Symptoms:** Agents stuck in `active` state after their process has exited;
new agent runs fail because the limit is reached.

The agent reaper runs every 10 minutes and marks agents `idle` if they haven't
sent a heartbeat (`TouchAgent`) within the threshold.

### Force reaper run (restart server)

```bash
docker compose restart ticket
# Reaper runs immediately on startup
```

### Manually reset a stuck agent

```bash
ticket agent update --id <agent-id> --status idle
```

### Increase reaper threshold

The threshold is 10 minutes (hardcoded in `internal/server/server.go:44`).
If agents legitimately run longer, increase the threshold and rebuild.

---

## High latency / slow queries

**Symptoms:** API responses are slow (>500ms); web UI feels sluggish.

### Check ticket count

```bash
ticket ticket ls --json | jq length
```

Large projects (>10,000 tickets) will be slow on unbounded list queries. Apply
pagination: `--limit 100 --offset 0`.

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
docker compose logs --tail 100 ticket | grep -i "websocket\|ws\|realtime"
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
du -sh /var/lib/docker/volumes/ticket-data/_data/

# Prune old backups
find /var/backups/ticket -name "*.json.gz" -mtime +30 -delete

# Prune Docker
docker system prune -f

# If the WAL file is large, checkpoint it
docker exec ticket sqlite3 /home/ticket/.ticket/ticket.db "PRAGMA wal_checkpoint(TRUNCATE);"

# Enable history retention to prevent unbounded growth
# Set TICKET_HISTORY_RETENTION_DAYS=180 in your compose.yaml environment
```
