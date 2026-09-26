FROM golang:1.21-alpine
WORKDIR /app
COPY main.go .
RUN go build -o mock-rpc main.go
CMD ["./mock-rpc"]
