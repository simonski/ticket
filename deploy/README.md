# Deploying ticket

This bundle is deployed to the EXE_DEV target via `make deploy` from the repo root. Once copied, use the included `Makefile` to manage the service.

## First-time setup

Log Docker into GHCR with a GitHub Personal Access Token that has package read access:

```bash
echo "<YOUR_GITHUB_PAT>" | docker login ghcr.io -u simonski --password-stdin
```

Then run setup — it creates `.env` from the template and pre-owns the data directory with the correct UID/GID from the container image:

```bash
make setup
```

Edit `.env` to set `TICKET_UID`, `TICKET_GID`, and `TICKET_ADMIN_PASSWORD`:

```bash
$EDITOR .env
```

## Running

```bash
make up      # start in background
make down    # stop
make logs    # follow logs
```

## Upgrading the database

The `tk` binary (deployed to `~/tk`) can run database migrations directly on the host:

```bash
~/tk migrate
```

Run this before restarting the container after an upgrade that includes schema changes.
