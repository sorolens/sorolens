# syntax=docker/dockerfile:1
#
# Sorolens API image.
#
# Build from the repository root:
#   docker build -f apps/api/Dockerfile -t sorolens-api .
#
# The container applies pending database migrations on startup (see
# apps/api/internal/db/migrate.go, which embeds the SQL files) and then serves
# the HTTP API on $PORT (default 8080).

# ---------------------------------------------------------------------------
# Build stage
# ---------------------------------------------------------------------------
# The API is a self-contained Go module under apps/api, so the build disables
# the repository go.work (GOWORK=off) and compiles from apps/api/go.mod only.
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Copy the module files first so dependency downloads are cached independently
# of the source tree.
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

COPY apps/api/ ./

# CGO is disabled: pgx, go-redis, and golang-migrate are pure Go, which lets the
# runtime stage stay on a minimal base image.
RUN GOWORK=off CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/sorolens-api \
        .

# ---------------------------------------------------------------------------
# Runtime stage
# ---------------------------------------------------------------------------
FROM alpine:3.20 AS runtime

# ca-certificates is required for TLS to Postgres, Redis, and Soroban RPC
# endpoints; tzdata keeps TIMESTAMPTZ formatting predictable.
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S sorolens \
    && adduser -S -G sorolens sorolens

COPY --from=builder /out/sorolens-api /usr/local/bin/sorolens-api

USER sorolens
WORKDIR /app

ENV PORT=8080
EXPOSE 8080

# The health route is dependency-free (no Postgres or Redis required), so it is
# safe to use as a liveness probe.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -q -O- "http://127.0.0.1:${PORT}/api/v1/health" || exit 1

ENTRYPOINT ["/usr/local/bin/sorolens-api"]
