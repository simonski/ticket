#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$ROOT_DIR"

ACTION="${1:-up}"
shift || true

case "$ACTION" in
  up)
    docker compose up -d --build "$@"
    echo ""
    echo "ticket is starting. On first boot, fetch the printed admin password with:"
    echo "  docker compose logs --no-color ticket | grep 'ADMIN PASSWORD:'"
    echo ""
    docker compose logs --no-color --tail=50 ticket
    ;;
  down)
    docker compose down "$@"
    ;;
  logs)
    docker compose logs --no-color -f ticket "$@"
    ;;
  restart)
    docker compose up -d --build --force-recreate "$@"
    ;;
  *)
    echo "usage: deploy/deploy.sh [up|down|logs|restart] [docker-compose-args...]" >&2
    exit 1
    ;;
esac
