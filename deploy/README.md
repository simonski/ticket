# Deploying ticket

## Local testing

Build and run locally before pushing to production:

```bash
make docker-build
docker compose -f deploy/compose.yaml up
```

On first run the container initialises the database and prints the admin password:

```
No database found — initialising...
admin user: admin
admin password: <generated-uuid>
```

Copy the password — you need it to log in. If running detached (`-d`), retrieve it with:

```bash
docker logs ticket-ticket-1 2>&1 | grep "admin password"
```

Verify the server is running:

```bash
curl http://localhost:8080/api/healthz
```

Stop with:

```bash
docker compose -f deploy/compose.yaml down
```

Data persists in the `ticket-data` Docker volume across restarts.

## Publishing to ghcr.io

Images are pushed to `ghcr.io/simonski/ticket`.

### One-time: authenticate Docker to ghcr.io

Create a GitHub Personal Access Token (classic) with `write:packages` scope at
https://github.com/settings/tokens, then:

```bash
echo "<YOUR_TOKEN>" | docker login ghcr.io -u simonski --password-stdin
```

### Push a new version

```bash
make docker-push
```

This builds the image, tags it as both `:<version>` and `:latest`, and pushes both to ghcr.io.

## Deploying to ticket.exe.xyz

### First-time server setup

SSH into the server and install Docker if not already present:

```bash
curl -fsSL https://get.docker.com | sh
```

Authenticate Docker to ghcr.io (same PAT as above, but with `read:packages` scope is sufficient):

```bash
echo "<YOUR_TOKEN>" | docker login ghcr.io -u simonski --password-stdin
```

Copy `deploy/compose.yaml` to the server:

```bash
scp deploy/compose.yaml ticket.exe.xyz:~/compose.yaml
```

Start the stack:

```bash
ssh ticket.exe.xyz "docker compose -f ~/compose.yaml up -d"
```

Check the admin password:

```bash
ssh ticket.exe.xyz "docker logs ticket-ticket-1 2>&1 | grep 'admin password'"
```

### How updates work

Watchtower runs alongside the ticket container and polls ghcr.io every 5 minutes.
When a new `:latest` image is available, it automatically pulls it and restarts the
ticket container. The SQLite database is on a Docker volume, so data survives restarts.

The sdlc is:

1. Make changes locally
2. `make docker-push` — builds and pushes to ghcr.io
3. Within 5 minutes, watchtower pulls the new image and restarts the container

### Reverse proxy / TLS

The compose file exposes port 8080. To serve on `https://ticket.exe.xyz`, put a
reverse proxy (nginx, caddy, traefik) in front that terminates TLS and forwards
to `localhost:8080`.

Example with Caddy (add to a Caddyfile on the server):

```
ticket.exe.xyz {
    reverse_proxy localhost:8080
}
```

## Data management

### Backup

```bash
# From the server
docker cp ticket-ticket-1:/home/ticket/.ticket/ticket.db ./ticket-backup.db
```

### Restore

```bash
docker compose -f ~/compose.yaml down
docker cp ./ticket-backup.db ticket-ticket-1:/home/ticket/.ticket/ticket.db
docker compose -f ~/compose.yaml up -d
```

### Volume location

The named volume `ticket-data` is managed by Docker. Find its path with:

```bash
docker volume inspect ticket-data
```
