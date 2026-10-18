package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// scriptedRedis answers Incr with a scripted count or error and records how
// often it was touched, so an exempt path can prove it never reached Redis.
type scriptedRedis struct {
	count int64
	err   error
	incrs int
}

func (s *scriptedRedis) Incr(context.Context, string) (int64, error) {
	s.incrs++
	return s.count, s.err
}

func (s *scriptedRedis) Expire(context.Context, string, time.Duration) (bool, error) {
	return true, nil
}

// stubKeyLookup is an APIKeyLookup with a scripted answer: the zero value
// rejects every key, which keeps a request on the anonymous tier.
type stubKeyLookup struct {
	key store.APIKey
	err error
}

func (s stubKeyLookup) GetAPIKeyByHash(context.Context, string) (store.APIKey, error) {
	return s.key, s.err
}

func (s stubKeyLookup) TouchAPIKey(context.Context, string) error { return nil }

// rateLimitValue reads one series of the limiter counter. WithLabelValues
// creates the series on first use, so the value is zero before any request.
func rateLimitValue(result, tier string) float64 {
	return testutil.ToFloat64(rateLimitRequests.WithLabelValues(result, tier))
}

// serveThroughRateLimit runs one request through the limiter and reports
// whether the request was passed downstream.
func serveThroughRateLimit(path string, rc RedisClient, lookup APIKeyLookup) bool {
	return serveThroughRateLimitWithAPIKey(path, "", rc, lookup)
}

// serveThroughRateLimitWithAPIKey is serveThroughRateLimit with an optional
// bearer token presented on the request.
func serveThroughRateLimitWithAPIKey(path, apiKey string, rc RedisClient, lookup APIKeyLookup) bool {
	called := false
	h := RateLimit(rc, lookup)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
	return called
}

// TestRateLimitCountsAllowedRequests asserts a request under the limit is
// recorded with result="allowed" and reaches the handler.
func TestRateLimitCountsAllowedRequests(t *testing.T) {
	before := rateLimitValue("allowed", "anonymous")

	called := serveThroughRateLimit("/api/v1/contracts", &scriptedRedis{count: 1}, stubKeyLookup{})

	if !called {
		t.Error("request under the limit did not reach the handler")
	}
	if got := rateLimitValue("allowed", "anonymous") - before; got != 1 {
		t.Errorf(`allowed counter delta = %v, want 1`, got)
	}
}

// TestRateLimitCountsRejections asserts a throttled request is recorded with
// result="limited" and answered with 429 without reaching the handler.
func TestRateLimitCountsRejections(t *testing.T) {
	before := rateLimitValue("limited", "anonymous")

	called := false
	h := RateLimit(&scriptedRedis{count: 101}, stubKeyLookup{})(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil))

	if called {
		t.Error("throttled request reached the handler")
	}
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rr.Code)
	}
	if got := rateLimitValue("limited", "anonymous") - before; got != 1 {
		t.Errorf(`limited counter delta = %v, want 1`, got)
	}
}

// TestRateLimitCountsExemptScrape asserts the Prometheus scrape path is
// counted as exempt and never touches Redis — a scrape must not consume a
// caller's budget.
func TestRateLimitCountsExemptScrape(t *testing.T) {
	before := rateLimitValue("exempt", "anonymous")
	redis := &scriptedRedis{count: 1}

	called := serveThroughRateLimit("/metrics", redis, stubKeyLookup{})

	if !called {
		t.Error("exempt request did not reach the handler")
	}
	if redis.incrs != 0 {
		t.Errorf("exempt request hit Redis %d times, want 0", redis.incrs)
	}
	if got := rateLimitValue("exempt", "anonymous") - before; got != 1 {
		t.Errorf(`exempt counter delta = %v, want 1`, got)
	}
}

// TestRateLimitCountsRedisErrors asserts a Redis outage is counted with
// result="error" while the request still fails open.
func TestRateLimitCountsRedisErrors(t *testing.T) {
	before := rateLimitValue("error", "anonymous")

	called := serveThroughRateLimit("/api/v1/contracts",
		&scriptedRedis{err: errors.New("redis down")}, stubKeyLookup{})

	if !called {
		t.Error("request during a Redis error did not fail open")
	}
	if got := rateLimitValue("error", "anonymous") - before; got != 1 {
		t.Errorf(`error counter delta = %v, want 1`, got)
	}
}

// TestRateLimitCountsAuthenticatedTier asserts a request carrying a valid API
// key lands on tier="authenticated".
func TestRateLimitCountsAuthenticatedTier(t *testing.T) {
	before := rateLimitValue("allowed", "authenticated")

	lookup := stubKeyLookup{key: store.APIKey{ID: "key-1"}}
	called := serveThroughRateLimitWithAPIKey("/api/v1/contracts", "sk_live_1", &scriptedRedis{count: 1}, lookup)

	if !called {
		t.Error("authenticated request did not reach the handler")
	}
	if got := rateLimitValue("allowed", "authenticated") - before; got != 1 {
		t.Errorf(`authenticated counter delta = %v, want 1`, got)
	}
}

// TestRateLimitLabelsAreBounded exercises every decision the limiter can
// make and asserts every resulting series carries exactly the two documented
// labels with values from their closed sets — the acceptance criterion that
// label cardinality stays bounded.
func TestRateLimitLabelsAreBounded(t *testing.T) {
	// exempt, error, allowed and limited, plus the authenticated tier, in
	// one pass.
	serveThroughRateLimit("/metrics", &scriptedRedis{count: 1}, stubKeyLookup{})
	serveThroughRateLimit("/api/v1/contracts", &scriptedRedis{err: errors.New("down")}, stubKeyLookup{})
	serveThroughRateLimit("/api/v1/contracts", &scriptedRedis{count: 1}, stubKeyLookup{})
	serveThroughRateLimit("/api/v1/contracts", &scriptedRedis{count: 101}, stubKeyLookup{})
	serveThroughRateLimitWithAPIKey("/api/v1/contracts", "sk_live_1",
		&scriptedRedis{count: 1}, stubKeyLookup{key: store.APIKey{ID: "key-1"}})

	reg := prometheus.NewRegistry()
	reg.MustRegister(rateLimitRequests)
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if len(families) != 1 {
		t.Fatalf("gathered %d metric families, want 1", len(families))
	}

	results := map[string]bool{"allowed": true, "limited": true, "exempt": true, "error": true}
	tiers := map[string]bool{"anonymous": true, "authenticated": true}

	for _, m := range families[0].Metric {
		if len(m.Label) != 2 {
			t.Errorf("series has %d labels, want exactly 2 (result, tier): %v", len(m.Label), m.Label)
			continue
		}
		labels := map[string]string{}
		for _, l := range m.Label {
			labels[l.GetName()] = l.GetValue()
		}
		if !results[labels["result"]] {
			t.Errorf("unexpected result label value %q", labels["result"])
		}
		if !tiers[labels["tier"]] {
			t.Errorf("unexpected tier label value %q", labels["tier"])
		}
	}

	// 4 decisions x 2 tiers is the hard ceiling on this metric.
	if got := len(families[0].Metric); got > 8 {
		t.Errorf("series count = %d, want <= 8", got)
	}
}
