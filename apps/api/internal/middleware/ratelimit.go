package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type RedisClient interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
}

// rateLimitRequests counts every request the limiter evaluates, by decision
// and by the credential tier the caller presented. Both labels are closed
// sets — result is one of allowed/limited/exempt/error and tier is one of
// anonymous/authenticated — so cardinality is bounded at 8 series no matter
// how many distinct callers or paths show up.
var rateLimitRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "sorolens_api_ratelimit_requests_total",
	Help: "Requests evaluated by the rate limiter by decision (allowed, limited, exempt, error) and credential tier (anonymous, authenticated).",
}, []string{"result", "tier"})

// RateLimitCollectors returns the rate-limiter metrics for registration on
// /metrics.
func RateLimitCollectors() []prometheus.Collector {
	return []prometheus.Collector{rateLimitRequests}
}

// RateLimit enforces a per-IP rate limit of 100 req/min for unauthenticated
// and 1000 req/min for authenticated requests, backed by Redis.
func RateLimit(rc RedisClient, lookup APIKeyLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The tier is whether the request presented an API key at all —
			// a header read only, so the exempt paths below never touch the
			// store. A key that later fails validation still applies the
			// anonymous limit; the label records what was presented.
			tier := "anonymous"
			token := extractAPIKey(r)
			if token != "" {
				tier = "authenticated"
			}

			// Health probes, the version endpoint, and the Prometheus scrape
			// endpoint are exempt: a scrape must not be throttled (it would
			// otherwise create gaps in the series during incident response)
			// and must never consume another caller's budget.
			if r.URL.Path == "/health" || r.URL.Path == "/readyz" ||
				r.URL.Path == "/api/version" || r.URL.Path == "/metrics" {
				rateLimitRequests.WithLabelValues("exempt", tier).Inc()
				next.ServeHTTP(w, r)
				return
			}

			ip := clientIP(r)
			limit := 100

			// Check if authenticated. The Identity middleware has already
			// verified the credential when it ran; fall back to a lookup for
			// direct callers/tests.
			if _, ok := IdentityFromRequest(r); ok {
				limit = 1000
			} else if token != "" {
				key, err := lookup.GetAPIKeyByHash(r.Context(), store.HashKey(token))
				if err == nil && !key.Revoked() {
					limit = 1000
				}
			}

			redisKey := "rate_limit:" + ip
			minuteWindow := time.Now().Minute()
			redisKey = redisKey + ":" + strconv.Itoa(minuteWindow)

			count, err := rc.Incr(r.Context(), redisKey)
			if err != nil {
				// On Redis error, fail open
				rateLimitRequests.WithLabelValues("error", tier).Inc()
				next.ServeHTTP(w, r)
				return
			}
			if count == 1 {
				_, _ = rc.Expire(r.Context(), redisKey, time.Minute)
			}

			if count > int64(limit) {
				rateLimitRequests.WithLabelValues("limited", tier).Inc()
				w.Header().Set("Retry-After", "60")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]string{
						"code":       "RATE_LIMITED",
						"message":    "too many requests",
						"request_id": GetRequestID(r.Context()),
					},
				})
				return
			}
			rateLimitRequests.WithLabelValues("allowed", tier).Inc()
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if ip, _, err := net.SplitHostPort(fwd); err == nil {
			return ip
		}
		return fwd
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		return r.RemoteAddr
	}
	return ip
}
