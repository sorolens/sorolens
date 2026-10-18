# syntax=docker/dockerfile:1.7
#
# Build context: the monorepo root (.).
#
# Usage (from repo root):
#   docker build \
#     --build-arg VERSION=$(git describe --tags --always) \
#     --build-arg GIT_SHA=$(git rev-parse --short HEAD) \
#     --build-arg BUILT_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
#     -t sorolens/api:latest \
#     -f apps/api/Dockerfile \
#     .

# ---------------------------------------------------------------------------
# Stage 1 – builder
# Builds a statically-linked binary from the Go source.
# ---------------------------------------------------------------------------
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Copy the API module manifests first so the layer is cached until they change.
COPY apps/api/go.mod apps/api/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy only the API source tree (not the whole monorepo).
COPY apps/api/ .

# Inject build metadata into the /api/version endpoint via ldflags.
ARG VERSION=dev
ARG GIT_SHA=unknown
ARG BUILT_AT=unknown

# Build for the container's own platform: the binary runs in the image built in
# this same platform, so no GOARCH override is needed (and hardcoding amd64
# breaks arm64 hosts).
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 \
    go build \
      -ldflags="-s -w \
        -X github.com/sorolens/sorolens/apps/api/internal/buildinfo.Version=${VERSION} \
        -X github.com/sorolens/sorolens/apps/api/internal/buildinfo.GitSHA=${GIT_SHA} \
        -X github.com/sorolens/sorolens/apps/api/internal/buildinfo.BuiltAt=${BUILT_AT}" \
      -o /out/sorolens-api \
      .

# ---------------------------------------------------------------------------
# Stage 2 – runtime
# Minimal distroless image: no shell, no package manager, non-root user.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

# CA certificates and timezone data from the builder stage.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# API binary.
COPY --from=builder /out/sorolens-api /usr/local/bin/sorolens-api

# SQL migrations — used by the prod-compose migrate service or a Kubernetes
# init-container to apply the schema before the API starts.
COPY --from=builder /src/internal/db/migrations /migrations

# distroless/base-debian12 nonroot UID = 65532.
USER nonroot:nonroot

# Default port; override with PORT env var.
ENV PORT=8080
EXPOSE 8080

# Healthcheck against the /healthz endpoint using the distroless-friendly
# built-in health probe (no shell/curl available in the image).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/usr/local/bin/sorolens-api", "healthcheck"]

ENTRYPOINT ["/usr/local/bin/sorolens-api"]
