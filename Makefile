MIGRATE_IMAGE      := migrate/migrate:v4.17.1
MIGRATIONS_DIR     := apps/api/internal/db/migrations
DB_URL_LOCAL       := postgres://sorolens:sorolens@localhost:5432/sorolens?sslmode=disable
DB_URL_DOCKER      := postgres://sorolens:sorolens@postgres:5432/sorolens?sslmode=disable
OAPI_CODEGEN_VERSION := v2.4.1
# oapi-codegen embeds a gzipped copy of the spec in the generated client, and
# gzip output is not stable across Go releases. The generator must therefore be
# built with the same toolchain the committed client was produced with, or CI's
# "Go client up-to-date" job reports a spurious diff.
OAPI_CODEGEN_GO := go1.26.0
OAPI_CODEGEN := $(CURDIR)/.bin/oapi-codegen
OAPI_SPEC    := docs/openapi.yaml
CLIENT_DIR   := packages/go-client

.PHONY: up down logs psql migrate-up migrate-down migrate-new test lint dev build client-go openapi lint-openapi

## up: start all Docker services in the background
up:
	docker compose up -d

## down: stop and remove all Docker services
down:
	docker compose down

## logs: tail logs from all running services
logs:
	docker compose logs -f

## psql: open a psql shell against the local Postgres container
psql:
	docker compose exec postgres psql -U sorolens -d sorolens

## migrate-up: apply all pending migrations (runs golang-migrate in Docker)
migrate-up:
	MSYS_NO_PATHCONV=1 docker run --rm \
		--network sorolens_net \
		-v "$(CURDIR)/$(MIGRATIONS_DIR):/migrations" \
		$(MIGRATE_IMAGE) \
		-path /migrations \
		-database "$(DB_URL_DOCKER)" \
		up

## migrate-down: roll back the most recent migration
migrate-down:
	MSYS_NO_PATHCONV=1 docker run --rm \
		--network sorolens_net \
		-v "$(CURDIR)/$(MIGRATIONS_DIR):/migrations" \
		$(MIGRATE_IMAGE) \
		-path /migrations \
		-database "$(DB_URL_DOCKER)" \
		down 1

## migrate-new NAME=<name>: create a new up/down migration pair
migrate-new:
	@test -n "$(NAME)" || (echo "Usage: make migrate-new NAME=add_some_table" && exit 1)
	@next=$$(ls $(MIGRATIONS_DIR)/*.up.sql 2>/dev/null | wc -l); \
	next=$$((next + 1)); \
	seq=$$(printf "%06d" $$next); \
	touch "$(MIGRATIONS_DIR)/$${seq}_$(NAME).up.sql" \
	      "$(MIGRATIONS_DIR)/$${seq}_$(NAME).down.sql"; \
	echo "Created $(MIGRATIONS_DIR)/$${seq}_$(NAME).{up,down}.sql"

## test: run all Go and TypeScript tests
test:
	cd apps/api && go test -race ./...
	cd services/indexer && go test -race ./...
	cd cli && go test -race ./...
	pnpm test

## openapi: check docs/openapi.yaml covers every live API route, lint it,
## and regenerate the Go client from it. The spec is hand-maintained; the
## route check (apps/api/internal/router/openapi_test.go) fails when a route
## is added without documenting it, or documented without existing.
openapi:
	cd apps/api && go test -count=1 -run TestOpenAPICoversEveryRoute ./internal/router
	$(MAKE) lint-openapi
	$(MAKE) client-go

## client-go: regenerate the Go client from the OpenAPI spec
client-go:
	GOBIN=$(CURDIR)/.bin GOTOOLCHAIN=$(OAPI_CODEGEN_GO) go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION)
	cd $(CLIENT_DIR) && $(OAPI_CODEGEN) --config oapi-codegen.yaml ../../$(OAPI_SPEC)
	cd $(CLIENT_DIR) && go mod tidy

## lint-openapi: validate the OpenAPI spec with redocly (skipped without npx)
lint-openapi:
	@if command -v npx >/dev/null 2>&1; then \
		npx --yes @redocly/cli lint $(OAPI_SPEC); \
	else \
		echo "npx unavailable; skipped redocly lint"; \
	fi

## lint: run golangci-lint and pnpm lint across the monorepo
lint:
	cd apps/api && golangci-lint run ./... || true
	cd services/indexer && golangci-lint run ./... || true
	cd cli && golangci-lint run ./... || true
	pnpm lint

## dev: start the Next.js dev server (and any other parallel dev scripts)
dev:
	pnpm dev

## build: build all Go binaries and TypeScript packages
build:
	cd apps/api && go build ./...
	cd services/indexer && go build ./...
	cd cli && go build ./...
	cd $(CLIENT_DIR) && go build ./...
	pnpm build
