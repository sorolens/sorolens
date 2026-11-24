# syntax=docker/dockerfile:1
#
# Build context: the monorepo root (.).
#
# Usage (from repo root):
#   docker build -t sorolens/indexer -f services/indexer/Dockerfile .

# ---------------------------------------------------------------------------
# Stage 1 – builder
# ---------------------------------------------------------------------------
FROM golang:1.26.11-alpine@sha256:ecb67f987aa1a57794c91d19ade7ae0209f2f0f5496aaef9e6ae85ddf92d02ab AS builder

RUN apk add --no-cache git ca-certificates tzdata

ENV GOTOOLCHAIN=local
WORKDIR /src

# The indexer's rules engine imports apps/api/rulelang and its failure
# classifier imports apps/api/failurecategory, so the two modules are resolved
# together through a trimmed workspace. Shipping the whole monorepo would copy
# the dashboard and every other module into the build context.
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY services/indexer/go.mod services/indexer/go.sum ./services/indexer/
RUN printf 'go 1.26\n\nuse (\n\t./apps/api\n\t./services/indexer\n)\n' > go.work

COPY go.work.sum ./
RUN cd services/indexer && go mod download

COPY apps/api/rulelang ./apps/api/rulelang
COPY apps/api/failurecategory ./apps/api/failurecategory
COPY services/indexer/ ./services/indexer/

# Build for the container's own platform: the binary runs in the image built in
# this same platform, so no GOARCH override is needed (and hardcoding amd64
# breaks arm64 hosts).
ARG VERSION=dev
ARG GIT_SHA=dev
ARG BUILT_AT=dev
RUN cd services/indexer && CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.gitSHA=${GIT_SHA} -X main.builtAt=${BUILT_AT}" \
    -o /out/sorolens-indexer .

COPY scripts/container-healthcheck/main.go /tmp/healthcheck.go
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/healthcheck /tmp/healthcheck.go

# ---------------------------------------------------------------------------
# Stage 2 – runtime
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/base-debian12:nonroot@sha256:7f0c72cd138b442ae0deeb69c08b1acf5525439ba251a49ad93c320a061567e5

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/sorolens-indexer /usr/local/bin/sorolens-indexer

USER nonroot:nonroot

COPY --from=builder /out/healthcheck /usr/local/bin/healthcheck
HEALTHCHECK --interval=15s --timeout=5s --start-period=15s --retries=3 \
  CMD ["/usr/local/bin/healthcheck", "INDEXER_METRICS_ADDR", "9100", "/metrics"]

ENTRYPOINT ["/usr/local/bin/sorolens-indexer"]
CMD ["-mode", "continuous"]
