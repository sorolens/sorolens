FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /out/mock-rpc .

FROM alpine:3.21
COPY --from=builder /out/mock-rpc /usr/local/bin/mock-rpc
ENV PORT=8000
EXPOSE 8000
ENTRYPOINT ["/usr/local/bin/mock-rpc"]
