package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sorolens/sorolens/apps/api/internal/metrics"
)

const (
	defaultAnonymousLimit     = 100
	defaultAuthenticatedLimit = 1000
	defaultRateLimitWindow    = time.Minute
)

// RedisClient is the legacy counter subset used by API usage metering.
type RedisClient interface {
	Incr(context.Context, string) (int64, error)
	Expire(context.Context, string, time.Duration) (bool, error)
}

type rateLimitRedis interface {
	Eval(context.Context, string, []string, ...interface{}) (interface{}, error)
}

type RateLimitConfig struct {
	TrustedProxies     []string
	AnonymousLimit     int
	AuthenticatedLimit int
	Window             time.Duration
}

var rateLimitRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "sorolens_api_ratelimit_requests_total",
	Help: "Requests evaluated by the rate limiter by decision and credential tier.",
}, []string{"result", "tier"})

var requestSequence atomic.Uint64

func RateLimitCollectors() []prometheus.Collector { return []prometheus.Collector{rateLimitRequests} }

const rateLimitLuaScript = `
local now = redis.call('TIME')
local timestamp = now[1] * 1000 + math.floor(now[2] / 1000)
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', timestamp - window)
local count = redis.call('ZCARD', KEYS[1])
local oldest = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
local reset = window
if oldest[2] then reset = math.max(1, tonumber(oldest[2]) + window - timestamp) end
if count >= limit then return {count, reset, 0} end
redis.call('ZADD', KEYS[1], timestamp, ARGV[3])
redis.call('PEXPIRE', KEYS[1], window)
return {count + 1, reset, 1}
`

// RateLimit enforces per-key budgets and uses the client IP only for requests
// without a verified API key. The optional config preserves default callers.
func RateLimit(rc interface{}, _ APIKeyLookup, configs ...RateLimitConfig) func(http.Handler) http.Handler {
	cfg := RateLimitConfig{AnonymousLimit: defaultAnonymousLimit, AuthenticatedLimit: defaultAuthenticatedLimit, Window: defaultRateLimitWindow}
	if len(configs) > 0 {
		provided := configs[0]
		cfg.TrustedProxies = provided.TrustedProxies
		if provided.AnonymousLimit > 0 {
			cfg.AnonymousLimit = provided.AnonymousLimit
		}
		if provided.AuthenticatedLimit > 0 {
			cfg.AuthenticatedLimit = provided.AuthenticatedLimit
		}
		if provided.Window > 0 {
			cfg.Window = provided.Window
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isRateLimitExempt(r.URL.Path) {
				rateLimitRequests.WithLabelValues("exempt", "anonymous").Inc()
				next.ServeHTTP(w, r)
				return
			}
			bucket, tier, limit := "", "anonymous", cfg.AnonymousLimit
			if id, ok := IdentityFromRequest(r); ok && id.APIKey != nil {
				bucket, tier, limit = "key:"+id.APIKey.ID, "authenticated", cfg.AuthenticatedLimit
			} else {
				bucket = "ip:" + clientIPWithTrustedProxies(r, cfg.TrustedProxies)
			}
			client, ok := rc.(rateLimitRedis)
			if !ok || client == nil {
				metrics.RateLimitFailOpenTotal.WithLabelValues("redis_unavailable").Inc()
				rateLimitRequests.WithLabelValues("error", tier).Inc()
				next.ServeHTTP(w, r)
				return
			}
			windowMS := cfg.Window.Milliseconds()
			member := strconv.FormatInt(time.Now().UnixNano(), 10) + ":" + strconv.FormatUint(requestSequence.Add(1), 10)
			result, err := client.Eval(r.Context(), rateLimitLuaScript, []string{"sorolens:rate-limit:" + bucket}, windowMS, limit, member)
			if err != nil {
				metrics.RateLimitFailOpenTotal.WithLabelValues("redis_error").Inc()
				slog.Default().Warn("rate limiter unavailable; allowing request", "err", err)
				rateLimitRequests.WithLabelValues("error", tier).Inc()
				next.ServeHTTP(w, r)
				return
			}
			count, resetMS, allowed, err := parseRateLimitResult(result)
			if err != nil {
				metrics.RateLimitFailOpenTotal.WithLabelValues("invalid_response").Inc()
				slog.Default().Warn("invalid rate limiter response; allowing request", "err", err)
				rateLimitRequests.WithLabelValues("error", tier).Inc()
				next.ServeHTTP(w, r)
				return
			}
			remaining := int64(limit) - count
			if remaining < 0 {
				remaining = 0
			}
			resetSeconds := int64((time.Duration(resetMS) * time.Millisecond).Seconds())
			if resetSeconds < 1 {
				resetSeconds = 1
			}
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Unix()+resetSeconds, 10))
			if !allowed {
				rateLimitRequests.WithLabelValues("limited", tier).Inc()
				w.Header().Set("Retry-After", strconv.FormatInt(resetSeconds, 10))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "RATE_LIMITED", "message": "too many requests", "request_id": GetRequestID(r.Context())}})
				return
			}
			rateLimitRequests.WithLabelValues("allowed", tier).Inc()
			next.ServeHTTP(w, r)
		})
	}
}

func isRateLimitExempt(path string) bool {
	return path == "/health" || path == "/healthz" || path == "/readyz" || path == "/api/version" || path == "/metrics"
}

func parseRateLimitResult(value interface{}) (count, reset int64, allowed bool, err error) {
	items, ok := value.([]interface{})
	if !ok || len(items) != 3 {
		return 0, 0, false, fmt.Errorf("unexpected rate limit result")
	}
	toInt := func(v interface{}) (int64, bool) {
		switch n := v.(type) {
		case int64:
			return n, true
		case int:
			return int64(n), true
		case string:
			parsed, e := strconv.ParseInt(n, 10, 64)
			return parsed, e == nil
		case []byte:
			parsed, e := strconv.ParseInt(string(n), 10, 64)
			return parsed, e == nil
		default:
			return 0, false
		}
	}
	count, ok = toInt(items[0])
	if !ok {
		return 0, 0, false, fmt.Errorf("invalid request count")
	}
	reset, ok = toInt(items[1])
	if !ok {
		return 0, 0, false, fmt.Errorf("invalid reset time")
	}
	decision, ok := toInt(items[2])
	if !ok {
		return 0, 0, false, fmt.Errorf("invalid decision")
	}
	return count, reset, decision == 1, nil
}

func clientIP(r *http.Request) string {
	return clientIPWithTrustedProxies(r, nil)
}

func clientIPWithTrustedProxies(r *http.Request, trusted []string) string {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	peerIP := net.ParseIP(strings.Trim(peer, "[]"))
	if peerIP == nil {
		return "unknown"
	}
	if !isTrustedProxy(peerIP, trusted) {
		return peerIP.String()
	}
	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(forwarded) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(forwarded[i]))
		if ip != nil && !isTrustedProxy(ip, trusted) {
			return ip.String()
		}
	}
	return peerIP.String()
}

func isTrustedProxy(ip net.IP, entries []string) bool {
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if parsed := net.ParseIP(entry); parsed != nil && parsed.Equal(ip) {
			return true
		}
		_, subnet, err := net.ParseCIDR(entry)
		if err == nil && subnet.Contains(ip) {
			return true
		}
	}
	return false
}
