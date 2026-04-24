# Deploying ticket

Use a GitHub Personal Access Token with package access to log Docker into GHCR:

```bash
echo "<YOUR_GITHUB_PAT>" | docker login ghcr.io -u simonski --password-stdin
```

This compose file stores the database in `./data` on the host.

Start:

```bash
docker compose -f ./compose.yaml up -d
```

Stop:

```bash
docker compose -f ./compose.yaml down
```

Follow logs:

```bash
docker compose -f ./compose.yaml logs -f
```
