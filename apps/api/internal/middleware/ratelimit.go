package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const (
	rateLimit  = 100.0
	rateBurst  = 100
	refillRate = rateLimit / 60.0 // tokens per second
)

type RedisClient interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
}

// RateLimit enforces a per-IP rate limit of 100 req/min for unauthenticated
// and 1000 req/min for authenticated requests, backed by Redis.
func RateLimit(rc RedisClient, lookup APIKeyLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" || r.URL.Path == "/readyz" || r.URL.Path == "/api/version" {
				next.ServeHTTP(w, r)
				return
			}

			ip := clientIP(r)
			limit := 100

			// Check if authenticated
			token := extractAPIKey(r)
			if token != "" {
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
				next.ServeHTTP(w, r)
				return
			}
			if count == 1 {
				rc.Expire(r.Context(), redisKey, time.Minute)
			}

			if count > int64(limit) {
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
