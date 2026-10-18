# Accept both the Docker Compose v2 plugin and the standalone binary.
COMPOSE ?= $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; else echo "docker-compose"; fi)

.PHONY: up down integration-test

# Build and start everything the suite needs. The indexer is built but not
# started: the Go tests run it explicitly after registering their fixture.
up:
	$(COMPOSE) up -d --build postgres redis mock-rpc migrate api
	$(COMPOSE) build indexer

down:
	$(COMPOSE) down -v --remove-orphans

# End-to-end flow: mock RPC -> indexer -> Postgres -> API, with exact
# assertions. Always tears the stack down, including on failure.
integration-test:
	@set -e; \
	trap '$(COMPOSE) down -v --remove-orphans' EXIT; \
	$(MAKE) up; \
	echo "Waiting for the API to become ready..."; \
	until curl -fsS http://localhost:8080/readyz >/dev/null; do sleep 1; done; \
	go test ./... -v -count=1
