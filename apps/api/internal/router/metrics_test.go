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
	"github.com/sorolens/sorolens/apps/api/internal/metrics"
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
// API-owned HTTP collectors, the rate-limiter decision counter, the default
// Go and process collectors, and (once wired) the Postgres pool collector.
var requiredMetrics = []string{
	"sorolens_http_requests_total",
	"sorolens_http_request_duration_seconds",
	"sorolens_http_requests_in_flight",
	"sorolens_api_ratelimit_requests_total",
	"go_goroutines",
	"process_resident_memory_bytes",
}

// scrapeMetrics GETs /metrics from the router and returns the exposition
// body, failing the test on a non-200 or a non-Prometheus content type.
func scrapeMetrics(t *testing.T, srv http.Handler) string {
	t.Helper()
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /metrics: got status %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("GET /metrics: got Content-Type %q, want a text/plain exposition", ct)
	}
	return rr.Body.String()
}

// fakePoolStat is a fixed pgx pool snapshot wired into tests that need the
// pool collector to emit series. *pgxpool.Stat satisfies the same interface.
type fakePoolStat struct{}

func (fakePoolStat) AcquireCount() int64                 { return 7 }
func (fakePoolStat) AcquireDuration() time.Duration      { return time.Second }
func (fakePoolStat) AcquiredConns() int32                { return 2 }
func (fakePoolStat) CanceledAcquireCount() int64         { return 1 }
func (fakePoolStat) ConstructingConns() int32            { return 0 }
func (fakePoolStat) EmptyAcquireCount() int64            { return 3 }
func (fakePoolStat) EmptyAcquireWaitTime() time.Duration { return 50 * time.Millisecond }
func (fakePoolStat) IdleConns() int32                    { return 5 }
func (fakePoolStat) MaxConns() int32                     { return 10 }
func (fakePoolStat) TotalConns() int32                   { return 7 }
func (fakePoolStat) NewConnsCount() int64                { return 4 }
func (fakePoolStat) MaxLifetimeDestroyCount() int64      { return 1 }
func (fakePoolStat) MaxIdleDestroyCount() int64          { return 2 }

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

// TestMetricsRouteLabelIsChiPatternNotRawPath drives a parameterized route
// with a high-entropy path segment and asserts the exposition labels the
// series with the chi route pattern rather than the raw path — the
// acceptance criterion that keeps label cardinality bounded however many
// distinct contract IDs are requested.
func TestMetricsRouteLabelIsChiPatternNotRawPath(t *testing.T) {
	srv := newMetricsTestRouter()

	const contractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractID, nil))

	body := scrapeMetrics(t, srv)
	if !strings.Contains(body, "route=\"/api/v1/contracts/{id}\"") {
		t.Errorf("GET /metrics: expected the chi pattern route=/api/v1/contracts/{id}; body:\n%s", body)
	}
	if strings.Contains(body, contractID) {
		t.Errorf("GET /metrics: raw contract ID leaked into a route label, cardinality would grow with traffic; body:\n%s", body)
	}
}

// TestMetricsUnmatchedRoutesFoldIntoSingleSeries asserts paths that match no
// route cannot mint unbounded label values: a root-level 404 folds into the
// single route="unmatched" series, and a 404 beneath a mounted subtree folds
// into that mount's wildcard pattern (chi resolves /api/v1/* for it).
func TestMetricsUnmatchedRoutesFoldIntoSingleSeries(t *testing.T) {
	srv := newMetricsTestRouter()

	const rootProbe = "/definitely-not-a-route-9f2c1e"
	const subtreeProbe = "/api/v1/definitely-not-a-route/9f2c1e"
	probes := []string{rootProbe, subtreeProbe}

	for _, probe := range probes {
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, probe, nil))
	}

	body := scrapeMetrics(t, srv)
	if !strings.Contains(body, "route=\"unmatched\"") {
		t.Errorf("GET /metrics: expected the folded route=unmatched series; body:\n%s", body)
	}
	if !strings.Contains(body, "route=\"/api/v1/*\"") {
		t.Errorf("GET /metrics: expected the mounted subtree wildcard route=/api/v1/*; body:\n%s", body)
	}
	for _, probe := range probes {
		if strings.Contains(body, probe) {
			t.Errorf("GET /metrics: raw unmatched path %q leaked into a route label; body:\n%s", probe, body)
		}
	}
}

// TestMetricsExposesRateLimitAndPoolCounters asserts the two collectors this
// change adds appear on /metrics: the rate-limiter decision counter (fed by
// the request that just passed the limiter) and the Postgres pool collector
// (fed by the wired snapshot).
func TestMetricsExposesRateLimitAndPoolCounters(t *testing.T) {
	metrics.SetPool(func() metrics.PoolStat { return fakePoolStat{} })
	t.Cleanup(func() { metrics.SetPool(nil) })

	srv := newMetricsTestRouter()

	// A non-exempt route, so the limiter records a decision for this scrape.
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/stats/global", nil))

	body := scrapeMetrics(t, srv)
	for _, want := range []string{
		"sorolens_api_ratelimit_requests_total",
		"result=\"allowed\"",
		"sorolens_db_pool_acquires_total",
		"sorolens_db_pool_empty_acquires_total",
		"sorolens_db_pool_max_connections",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("GET /metrics: exposition is missing %q; body:\n%s", want, body)
		}
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
