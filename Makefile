MIGRATE_IMAGE      := migrate/migrate:v4.17.1
MIGRATIONS_DIR     := apps/api/internal/db/migrations
DB_URL_LOCAL       := postgres://sorolens:sorolens@localhost:5432/sorolens?sslmode=disable
DB_URL_DOCKER      := postgres://sorolens:sorolens@postgres:5432/sorolens?sslmode=disable

.PHONY: up down logs psql migrate-up migrate-down migrate-new test lint dev build

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
	pnpm build
