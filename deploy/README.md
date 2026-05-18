# Deploying ticket

Use a GitHub Personal Access Token with package access to log Docker into GHCR:

```bash
echo "<YOUR_GITHUB_PAT>" | docker login ghcr.io -u simonski --password-stdin
```

This deployment bundle stores the database in `./data` on the host and runs the
container as an explicit non-root UID/GID from a local `.env` file.

Before the first start:

1. Find the image UID/GID:

```bash
docker run --rm --entrypoint sh ghcr.io/simonski/ticket:latest -c 'id -u ticket && id -g ticket'
```

2. Create and pre-own the bind mount:

```bash
mkdir -p ./data
sudo chown -R <uid>:<gid> ./data
sudo chmod 750 ./data
```

3. Copy the environment template and set `TICKET_UID`, `TICKET_GID`, and
   `TICKET_ADMIN_PASSWORD`:

```bash
cp env.template .env
$EDITOR .env
```

Start:

```bash
docker compose --env-file ./.env -f ./compose.yaml up -d
```

Stop:

```bash
docker compose --env-file ./.env -f ./compose.yaml down
```

Follow logs:

```bash
docker compose --env-file ./.env -f ./compose.yaml logs -f
```
