# syntax=docker/dockerfile:1
#
# Build context: the monorepo root (.).
#
# Usage (from repo root):
#   docker build -t sorolens/indexer -f services/indexer/Dockerfile .

# ---------------------------------------------------------------------------
# Stage 1 – builder
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# The indexer's rules engine imports apps/api/rulelang, so the two modules are
# resolved together through a trimmed workspace. Shipping the whole monorepo
# would copy the dashboard and every other module into the build context.
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY services/indexer/go.mod services/indexer/go.sum ./services/indexer/
RUN printf 'go 1.26\n\nuse (\n\t./apps/api\n\t./services/indexer\n)\n' > go.work

RUN cd services/indexer && go mod download

COPY apps/api/rulelang ./apps/api/rulelang
COPY services/indexer/ ./services/indexer/

# Build for the container's own platform: the binary runs in the image built in
# this same platform, so no GOARCH override is needed (and hardcoding amd64
# breaks arm64 hosts).
RUN cd services/indexer && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/sorolens-indexer .

# ---------------------------------------------------------------------------
# Stage 2 – runtime
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/sorolens-indexer /usr/local/bin/sorolens-indexer

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/sorolens-indexer"]
