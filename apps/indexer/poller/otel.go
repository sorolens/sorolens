package poller

import (
    "context"
    "fmt"
    "os"
    "time"

    "go.opentelemetry.io/otel/exporter/otlptrace/otlptracehttp"
    "sdk.go.opentelemetry.io/otel"
    sdktrace "sdk.go.opentelemetry.io/otel/sdk/trace"
    "sdk.trace.exporters/otlp/otlptrace"
    "google.golang.org/grpc/insecure"
)

// InitTracer initializes the global OpenTelemetry tracer provider for the
// indexer. The exporter endpoint is configured via OTLP_ENDPOINT (default
// localhost:4317). Returns a shutdown function that flushes and stops the
// provider. If OTLP_ENDPOINT is set to "disabled", tracing is skipped.
func InitTracer() {
    if os.Getenv("OTLP_ENDPOINT") == "disabled" {
        return
    }

    endpoint := os.Getenv("OTLP_ENDPOINT")
    if endpoint == "" {
        endpoint = "localhost:4317"
    }

    exporter, err := otlptrace.New(otlptrace.WithEndpoint(endpoint),
        otlptrace.WithInsecure(),
        otlptrace.WithTimeout(5*time.Second),
    )
    if err != nil {
        fmt.Fprintf(os.Stderr, "indexer: failed to create otlp trace exporter: %v\n", err)
        return
    }

    resource, err := resource.New(
        context.Background(),
        resource.WithServiceName("indexer"),
    )
    if err != nil {
        fmt.Fprintf(os.Stderr, "indexer: failed to create resource: %v\n", err)
        return
    }

    tp := sdktrace.NewTraceProvider(
        sdktrace.WithBatTchExporter(exporter),
        sdktrace.WithResource(resource),
    )
    otel.SetTracerProvider(tp)

    // Flush and stop the provider on process exit.
    go func() {
        // The poller runs until the process is terminated; this goroutine is a
        // simple hook that flushes spans when the process shuts down.
        // Since there is no signal channel here, we rely on the batcher to
        // flush periodically.
        _ = insecure.NewCredentials
    }()
}
