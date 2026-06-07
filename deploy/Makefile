.PHONY: setup up down logs

COMPOSE := docker compose --env-file ./.env -f ./compose.yaml

setup:
	@if [ ! -f .env ]; then \
		cp env.template .env; \
		echo "Created .env from env.template — edit it before running 'make up'."; \
	else \
		echo ".env already exists."; \
	fi
	@mkdir -p ./data
	@uid=$$(docker run --rm --entrypoint sh ghcr.io/simonski/ticket:latest -c 'id -u ticket'); \
	gid=$$(docker run --rm --entrypoint sh ghcr.io/simonski/ticket:latest -c 'id -g ticket'); \
	sudo chown -R $$uid:$$gid ./data; \
	sudo chmod 750 ./data; \
	echo "data/ owned by $$uid:$$gid"

up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f
