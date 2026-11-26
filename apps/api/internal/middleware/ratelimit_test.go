package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type fakeRateLimitRedis struct {
	result interface{}
	err    error
	keys   []string
	called int
}

func (f *fakeRateLimitRedis) Eval(_ context.Context, _ string, keys []string, _ ...interface{}) (interface{}, error) {
	f.called++
	f.keys = keys
	return f.result, f.err
}

func TestRateLimitUsesVerifiedAPIKeyAndWritesHeaders(t *testing.T) {
	redis := &fakeRateLimitRedis{result: []interface{}{int64(3), int64(10000), int64(1)}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	request.RemoteAddr = "203.0.113.2:5000"
	request = request.WithContext(WithIdentity(request.Context(), VerifiedIdentity{APIKey: &store.APIKey{ID: "key-42"}}))
	response := httptest.NewRecorder()
	called := false
	RateLimit(redis, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(response, request)
	if !called {
		t.Fatal("allowed request did not reach handler")
	}
	if got := strings.Join(redis.keys, ","); got != "sorolens:rate-limit:key:key-42" {
		t.Fatalf("Redis key = %q", got)
	}
	if response.Header().Get("X-RateLimit-Limit") != "1000" || response.Header().Get("X-RateLimit-Remaining") != "997" {
		t.Fatalf("unexpected headers: %v", response.Header())
	}
}

func TestRateLimitReturnsRetryAfterWhenLimited(t *testing.T) {
	redis := &fakeRateLimitRedis{result: []interface{}{int64(101), int64(1500), int64(0)}}
	response := httptest.NewRecorder()
	called := false
	RateLimit(redis, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil))
	if called || response.Code != http.StatusTooManyRequests {
		t.Fatalf("called=%v status=%d", called, response.Code)
	}
	if response.Header().Get("Retry-After") == "" || response.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("missing limit headers: %v", response.Header())
	}
}

func TestRateLimitExemptsHealthAndMetrics(t *testing.T) {
	for _, path := range []string{"/health", "/readyz", "/healthz", "/metrics"} {
		redis := &fakeRateLimitRedis{}
		called := false
		RateLimit(redis, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
		if !called || redis.called != 0 {
			t.Errorf("path %s called=%v Redis calls=%d", path, called, redis.called)
		}
	}
}

func TestRateLimitFailsOpenOnRedisErrorOrNil(t *testing.T) {
	for _, client := range []interface{}{nil, &fakeRateLimitRedis{err: errors.New("redis unavailable")}} {
		called := false
		RateLimit(client, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil))
		if !called {
			t.Fatal("request did not fail open")
		}
	}
}

func TestClientIPHonorsForwardedHeaderOnlyFromTrustedPeer(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "198.51.100.9:5000"
	request.Header.Set("X-Forwarded-For", "203.0.113.4")
	if got := clientIPWithTrustedProxies(request, []string{"10.0.0.0/8"}); got != "198.51.100.9" {
		t.Fatalf("untrusted peer client IP = %q", got)
	}
	request.RemoteAddr = "10.0.0.5:5000"
	if got := clientIPWithTrustedProxies(request, []string{"10.0.0.0/8"}); got != "203.0.113.4" {
		t.Fatalf("trusted peer client IP = %q", got)
	}
}

func TestClientIPWalksForwardedChainFromRightToLeft(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.10:5000"
	request.Header.Set("X-Forwarded-For", "203.0.113.200, 198.51.100.4, 10.0.0.8")
	if got := clientIPWithTrustedProxies(request, []string{"10.0.0.0/8"}); got != "198.51.100.4" {
		t.Fatalf("client IP = %q, want nearest untrusted hop", got)
	}
}

func TestParseRateLimitResultRejectsInvalidValues(t *testing.T) {
	if _, _, _, err := parseRateLimitResult("bad"); err == nil {
		t.Fatal("expected malformed result error")
	}
	if _, _, _, err := parseRateLimitResult([]interface{}{int64(1)}); err == nil {
		t.Fatal("expected short result error")
	}
}

func TestRateLimitMetricsLabelsRemainBounded(t *testing.T) {
	before := testutil.ToFloat64(rateLimitRequests.WithLabelValues("allowed", "authenticated"))
	redis := &fakeRateLimitRedis{result: []interface{}{int64(1), int64(time.Minute.Milliseconds()), int64(1)}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	request = request.WithContext(WithIdentity(request.Context(), VerifiedIdentity{APIKey: &store.APIKey{ID: "metric-key"}}))
	RateLimit(redis, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(httptest.NewRecorder(), request)
	if delta := testutil.ToFloat64(rateLimitRequests.WithLabelValues("allowed", "authenticated")) - before; delta != 1 {
		t.Fatalf("counter delta = %v", delta)
	}
}
