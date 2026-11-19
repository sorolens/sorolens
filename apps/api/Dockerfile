# syntax=docker/dockerfile:1
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

# Build a statically linked binary from the API module.
FROM golang:1.26.11-alpine@sha256:ecb67f987aa1a57794c91d19ade7ae0209f2f0f5496aaef9e6ae85ddf92d02ab AS builder

RUN apk add --no-cache git ca-certificates tzdata

ENV GOTOOLCHAIN=local
WORKDIR /src

COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ .

ARG VERSION=dev
ARG GIT_SHA=dev
ARG BUILT_AT=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w \
      -X github.com/sorolens/sorolens/apps/api/internal/buildinfo.Version=${VERSION} \
      -X github.com/sorolens/sorolens/apps/api/internal/buildinfo.GitSHA=${GIT_SHA} \
      -X github.com/sorolens/sorolens/apps/api/internal/buildinfo.BuiltAt=${BUILT_AT}" \
    -o /out/sorolens-api .

COPY scripts/container-healthcheck/main.go /tmp/healthcheck.go
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/healthcheck /tmp/healthcheck.go

FROM gcr.io/distroless/base-debian12:nonroot@sha256:7f0c72cd138b442ae0deeb69c08b1acf5525439ba251a49ad93c320a061567e5

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/sorolens-api /usr/local/bin/sorolens-api
COPY --from=builder /src/internal/db/migrations /migrations
COPY --from=builder /out/healthcheck /usr/local/bin/healthcheck

USER nonroot:nonroot
ENV PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s --start-period=15s --retries=3 \
  CMD ["/usr/local/bin/healthcheck", "PORT", "8080", "/readyz"]

ENTRYPOINT ["/usr/local/bin/sorolens-api"]
