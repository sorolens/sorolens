# syntax=docker/dockerfile:1
# Build from the repository root. MinIO publishes this release as source.
FROM golang:1.26.11-alpine@sha256:ecb67f987aa1a57794c91d19ade7ae0209f2f0f5496aaef9e6ae85ddf92d02ab AS builder
ENV CGO_ENABLED=0 GOBIN=/out GOTOOLCHAIN=local
RUN go install -trimpath -ldflags="-s -w \
    -X github.com/minio/minio/cmd.Version=2025-10-15T17:29:55Z \
    -X github.com/minio/minio/cmd.ReleaseTag=RELEASE.2025-10-15T17-29-55Z \
    -X github.com/minio/minio/cmd.CommitID=9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a \
    -X github.com/minio/minio/cmd.ShortCommitID=9e49d5e7a648" \
    github.com/minio/minio@9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a
RUN go install -trimpath -ldflags="-s -w \
    -X github.com/minio/mc/cmd.Version=2025-08-13T08:35:41Z \
    -X github.com/minio/mc/cmd.ReleaseTag=RELEASE.2025-08-13T08-35-41Z \
    -X github.com/minio/mc/cmd.CommitID=7394ce0dd2a80935aded936b09fa12cbb3cb8096 \
    -X github.com/minio/mc/cmd.ShortCommitID=7394ce0dd2a8" \
    github.com/minio/mc@7394ce0dd2a80935aded936b09fa12cbb3cb8096
COPY scripts/container-healthcheck/main.go /tmp/healthcheck.go
RUN go build -trimpath -ldflags="-s -w" -o /out/healthcheck /tmp/healthcheck.go && mkdir /out/data

FROM gcr.io/distroless/base-debian12:nonroot@sha256:7f0c72cd138b442ae0deeb69c08b1acf5525439ba251a49ad93c320a061567e5
COPY --from=builder /out/minio /out/mc /out/healthcheck /usr/local/bin/
COPY --from=builder --chown=nonroot:nonroot /out/data/ /data/
USER nonroot:nonroot
EXPOSE 9000 9001
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
  CMD ["/usr/local/bin/healthcheck", "MINIO_PORT", "9000", "/minio/health/live"]
ENTRYPOINT ["/usr/local/bin/minio"]
