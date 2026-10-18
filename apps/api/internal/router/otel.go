package router

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporter/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	
// InitTracer configures the global OTel tracer provider. When
// OTEL_EXPORTER_OTLP_ENDPOINT is unset the provider has no exporter and
// spans are effectively no-op. The returned provider must be shut down
	// on exit to flush any batched spans.
	//
	// The endpoint may be provided with or without a scheme; the gRPC
	// exporter expects a host:port pair, so an http(s):// prefix is stripped.
	func InitTracer(serviceName string) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()
	var exp sdktrace.SpanExporter
	var err error

	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		endpoint = strings.TrimPrefix(endpoint, "https://")
		endpoint = strings.TrimPrefix(endpoint, "http://")
		exp, err = otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint))
		if err != nil {
			return nil, err
		}
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	}

	if exp != nil {
		opts = append(opts, sdktrace.WithBatcher(exp))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	return tp, nil
	}

func OTelMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("api")
		// Use the chi route pattern (set by the router before
		// handlers run) so span names are low-cardinality, e.g.
		// /api/v1/contracts/{id} instead of the raw URL path. Fall
		// back to the raw path for unmatched requests (e.g. 404).
		spanName := chi.RouteContext(r.Context()).RoutePattern
		if spanName == "" {
			spanName = r.URL.Path
		}
		ctx, span := tracer.Start(r.Context(), spanName)
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
