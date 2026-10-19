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

# Build metadata for GET /api/version. Override on the command line, e.g.
#   make build VERSION=1.2.3
# VERSION defaults to "dev" (matching cli/Makefile) until a release tag sets it.
VERSION   ?= dev
GIT_SHA   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILT_AT  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
API_BUILDINFO := github.com/sorolens/sorolens/apps/api/internal/buildinfo
API_LDFLAGS   := -ldflags "-X $(API_BUILDINFO).Version=$(VERSION) -X $(API_BUILDINFO).GitSHA=$(GIT_SHA) -X $(API_BUILDINFO).BuiltAt=$(BUILT_AT)"

.PHONY: up down logs psql migrate-up migrate-down migrate-version migrate-force migrate-new migrate-check test lint dev api build client-go openapi lint-openapi

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

## migrate-version: print the version the target database is at
migrate-version:
	MSYS_NO_PATHCONV=1 docker run --rm \
		--network sorolens_net \
		-v "$(CURDIR)/$(MIGRATIONS_DIR):/migrations" \
		$(MIGRATE_IMAGE) \
		-path /migrations \
		-database "$(DB_URL_DOCKER)" \
		version

## migrate-force VERSION=<n>: record <n> as applied without running anything.
## A database whose schema was built by the psql loops (every deployment path
## in this repo does that, and docker-compose skips it once `contracts` exists)
## has no schema_migrations row, so golang-migrate believes it is at version 0
## and would replay 000001 onto a populated schema. Baseline such a database
## once, at the highest version it already has applied, before using
## migrate-up on it:
##
##   make migrate-force VERSION=$$(ls $(MIGRATIONS_DIR)/*.up.sql | wc -l)
migrate-force:
	@test -n "$(VERSION)" || (echo "Usage: make migrate-force VERSION=36" && exit 1)
	MSYS_NO_PATHCONV=1 docker run --rm \
		--network sorolens_net \
		-v "$(CURDIR)/$(MIGRATIONS_DIR):/migrations" \
		$(MIGRATE_IMAGE) \
		-path /migrations \
		-database "$(DB_URL_DOCKER)" \
		force $(VERSION)

## migrate-new NAME=<name>: create a new up/down migration pair.
## The next version is the highest one present plus one — not the file count,
## which disagreed with it whenever two migrations shared a version and handed
## out a number that was already taken (issue #516). Any existing file at that
## version is refused, whatever its name: golang-migrate keys on the number, so
## two files at one version break the whole directory.
migrate-new:
	@test -n "$(NAME)" || (echo "Usage: make migrate-new NAME=add_some_table" && exit 1)
	@next=$$(for f in $(MIGRATIONS_DIR)/*.up.sql; do basename "$$f" | cut -d_ -f1; done 2>/dev/null | sort -n | tail -1 | sed 's/^0*//'); \
	next=$$(( $${next:-0} + 1 )); \
	seq=$$(printf "%06d" $$next); \
	taken=$$(ls $(MIGRATIONS_DIR)/$${seq}_*.sql 2>/dev/null); \
	if [ -n "$$taken" ]; then \
		echo "version $${seq} is already taken by: $$taken" >&2; exit 1; \
	fi; \
	touch "$(MIGRATIONS_DIR)/$${seq}_$(NAME).up.sql" \
	      "$(MIGRATIONS_DIR)/$${seq}_$(NAME).down.sql"; \
	echo "Created $(MIGRATIONS_DIR)/$${seq}_$(NAME).{up,down}.sql"

## migrate-check: fail if the migrations directory is not a single valid
## sequence. Same assertion the Go guard makes in
## apps/api/internal/db/migrations_test.go, without needing a database.
migrate-check:
	cd apps/api && go test -count=1 -run TestMigration ./internal/db

## test: run all Go and TypeScript tests
test:
	cd apps/api && go test -race ./...
	cd services/indexer && go test -race ./...
	cd cli && go test -race ./...
	cd $(CLIENT_DIR) && go test -race ./...
	pnpm test
	cargo test --workspace

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
	cd apps/api && golangci-lint run ./...
	cd services/indexer && golangci-lint run ./...
	cd cli && golangci-lint run ./...
	cd $(CLIENT_DIR) && golangci-lint run ./...
	pnpm lint

## dev: start the Next.js dev server (and any other parallel dev scripts)
dev:
	pnpm dev

## api: run the Go API against the Docker services, using the .env.example defaults
api:
	cd apps/api && \
		DATABASE_URL="$(DB_URL_LOCAL)" \
		DIRECT_DATABASE_URL="$(DB_URL_LOCAL)" \
		REDIS_URL="redis://localhost:6379" \
		go run .

## build: build all Go binaries and TypeScript packages
build:
	cd apps/api && go build $(API_LDFLAGS) ./...
	cd services/indexer && go build ./...
	cd cli && go build ./...
	cd $(CLIENT_DIR) && go build ./...
	pnpm build
