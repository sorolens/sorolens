package router_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// metricsRedisClient is a permissive RedisClient stub: the rate limiter calls
// Incr/Expire on every request, and these tests must never be throttled.
type metricsRedisClient struct{}

func (metricsRedisClient) Incr(context.Context, string) (int64, error) { return 1, nil }

func (metricsRedisClient) Expire(context.Context, string, time.Duration) (bool, error) {
	return true, nil
}

func newMetricsTestRouter() http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	h := &handler.Handler{
		Store:       store.NewMockStore(),
		DB:          &store.MockPinger{Healthy: true},
		Redis:       &store.MockPinger{Healthy: true},
		RedisClient: metricsRedisClient{},
		Logger:      logger,
	}
	return router.New(h, 0)
}

// requiredMetrics are the metric names the endpoint must expose: the three
// API-owned collectors plus the default Go and process collectors.
var requiredMetrics = []string{
	"sorolens_http_requests_total",
	"sorolens_http_request_duration_seconds",
	"sorolens_http_requests_in_flight",
	"go_goroutines",
	"process_resident_memory_bytes",
}

// TestMetricsEndpointExposesPrometheusExposition scrapes GET /metrics and
// asserts the response is a valid Prometheus exposition carrying the expected
// metric names.
func TestMetricsEndpointExposesPrometheusExposition(t *testing.T) {
	srv := newMetricsTestRouter()

	// Drive one request first: a CounterVec/HistogramVec emits nothing until it
	// has at least one observed series.
	warm := httptest.NewRecorder()
	srv.ServeHTTP(warm, httptest.NewRequest(http.MethodGet, "/health", nil))
	if warm.Code != http.StatusOK {
		t.Fatalf("warm-up GET /health: got status %d, want 200", warm.Code)
	}

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /metrics: got status %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("GET /metrics: got Content-Type %q, want a text/plain exposition", ct)
	}

	body := rr.Body.String()
	for _, name := range requiredMetrics {
		if !strings.Contains(body, name) {
			t.Errorf("GET /metrics: exposition is missing metric %q", name)
		}
	}

	// The route label must be the resolved chi pattern, not the raw path.
	if !strings.Contains(body, `route="/health"`) {
		t.Errorf("GET /metrics: expected a series labelled route=\"/health\"; body:\n%s", body)
	}
	if !strings.Contains(body, `method="GET"`) {
		t.Errorf("GET /metrics: expected a series labelled method=\"GET\"")
	}
}

// TestMetricsEndpointIsUnauthenticated pins the acceptance criterion that the
// endpoint needs no API key: a bare request (no Authorization, no X-API-Key)
// must still return the exposition.
func TestMetricsEndpointIsUnauthenticated(t *testing.T) {
	srv := newMetricsTestRouter()

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /metrics without credentials: got status %d, want 200", rr.Code)
	}
}
