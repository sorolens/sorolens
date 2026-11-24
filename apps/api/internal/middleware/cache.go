package middleware

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

// ResponseCache stores rendered JSON response bodies (issue #143).
type ResponseCache interface {
	// Get returns the cached body for key; ok is false on a miss.
	Get(ctx context.Context, key string) (body []byte, ok bool, err error)
	// Set stores body under key for ttl.
	Set(ctx context.Context, key string, body []byte, ttl time.Duration) error
	// Purge deletes every key in namespace.
	Purge(ctx context.Context, namespace string) error
}

// Cache namespaces. A write to a resource purges its namespace.
const (
	CacheNamespaceContracts = "contracts"
	CacheNamespaceWatchdog  = "watchdog"
	CacheNamespaceLabels    = "labels"
	CacheNamespaceStats     = "stats"
)

// cacheKeyPrefix is the Redis key prefix for cached responses; keys are
// "<prefix><namespace>:<method> <path>?<sorted query>".
const cacheKeyPrefix = "sorolens:cache:"

// cacheRequests counts cache lookups by namespace and result (hit | miss).
var cacheRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "sorolens_api_cache_requests_total",
	Help: "Response cache lookups by namespace and result (hit or miss).",
}, []string{"namespace", "result"})

// cachePurges counts namespace purges triggered by writes.
var cachePurges = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "sorolens_api_cache_purges_total",
	Help: "Response cache namespace purges triggered by writes.",
}, []string{"namespace"})

// CacheCollectors returns the cache metrics for registration on /metrics.
func CacheCollectors() []prometheus.Collector {
	return []prometheus.Collector{cacheRequests, cachePurges, cacheStale}
}

// cacheKey identifies a response by method, path and canonical (sorted)
// query string, so ?a=1&b=2 and ?b=2&a=1 share an entry. Per-user namespaces
// include the identity verified by the Identity middleware, never a raw
// header (issue #518).
func cacheKey(namespace string, r *http.Request) string {
	identity := ""
	if namespace == CacheNamespaceLabels {
		identity = VerifiedUserID(r)
	}
	key := cacheKeyPrefix + namespace + ":"
	if identity != "" {
		key += identity + ":"
	}
	return key + r.Method + " " + r.URL.Path + "?" + r.URL.Query().Encode()
}

// staleKeySuffix is appended to a cache key to store the stale fallback copy.
const staleKeySuffix = ":stale"

// cacheStale counts stale responses served when the upstream is unavailable.
var cacheStale = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "sorolens_api_cache_stale_total",
	Help: "Stale cache responses served due to upstream unavailability, by namespace.",
}, []string{"namespace"})

// Cache returns middleware that serves GET responses for the namespace from
// c, caching successful (200) responses for ttl. A nil cache or zero ttl
// disables it. Cache errors fail open: the request is served by the handler.
// Responses carry X-Cache: HIT or MISS.
func Cache(c ResponseCache, namespace string, ttl time.Duration, logger *slog.Logger) func(http.Handler) http.Handler {
	return CacheWithStale(c, namespace, ttl, 0, logger)
}

// bufferedResponseWriter captures the status code, headers, and body written by
// a handler so the middleware can inspect the result before forwarding it.
type bufferedResponseWriter struct {
	code   int
	header http.Header
	body   bytes.Buffer
}

func newBufferedResponseWriter() *bufferedResponseWriter {
	return &bufferedResponseWriter{code: http.StatusOK, header: make(http.Header)}
}

func (b *bufferedResponseWriter) Header() http.Header         { return b.header }
func (b *bufferedResponseWriter) WriteHeader(code int)        { b.code = code }
func (b *bufferedResponseWriter) Write(p []byte) (int, error) { return b.body.Write(p) }

// flush copies all buffered headers, status, and body to dst.
func (b *bufferedResponseWriter) flush(dst http.ResponseWriter) {
	for k, vv := range b.header {
		for _, v := range vv {
			dst.Header().Add(k, v)
		}
	}
	dst.WriteHeader(b.code)
	_, _ = dst.Write(b.body.Bytes())
}

// CacheWithStale is like Cache but also keeps a stale copy of each fresh entry
// for staleTTL. When the upstream handler returns a 5xx response and a stale
// copy exists, the stale body is served instead with:
//
//	Warning: 110 - "Response is Stale"
//	X-Cache: STALE
//
// Stale serving is never applied to the CacheNamespaceLabels namespace because
// it carries per-user data that must never bleed across identities.
// A zero or negative staleTTL disables stale serving.
func CacheWithStale(c ResponseCache, namespace string, ttl, staleTTL time.Duration, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if c == nil || ttl <= 0 {
			return next
		}
		allowStale := staleTTL > 0 && namespace != CacheNamespaceLabels
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}
			key := cacheKey(namespace, r)
			body, ok, err := c.Get(r.Context(), key)
			if err != nil && logger != nil {
				logger.Warn("cache get failed", "err", err, "namespace", namespace)
			}
			if ok {
				cacheRequests.WithLabelValues(namespace, "hit").Inc()
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(body)
				return
			}
			cacheRequests.WithLabelValues(namespace, "miss").Inc()

			// Buffer the handler's response so we can inspect the status code
			// before deciding whether to forward it or serve a stale copy.
			buf := newBufferedResponseWriter()
			buf.header.Set("X-Cache", "MISS")
			next.ServeHTTP(buf, r)

			if buf.code >= 500 && allowStale {
				// Upstream is unavailable; try the stale copy.
				staleBody, staleOK, staleErr := c.Get(r.Context(), key+staleKeySuffix)
				if staleErr != nil && logger != nil {
					logger.Warn("stale cache get failed", "err", staleErr, "namespace", namespace)
				}
				if staleOK && len(staleBody) > 0 {
					cacheStale.WithLabelValues(namespace).Inc()
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-Cache", "STALE")
					w.Header().Set("Warning", `110 - "Response is Stale"`)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(staleBody)
					return
				}
			}

			// Forward the buffered response verbatim (X-Cache: MISS is in buf.header).
			buf.flush(w)

			if buf.code != http.StatusOK || buf.body.Len() == 0 {
				return
			}
			// Detach from the request context: the client may already be gone.
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), time.Second)
			defer cancel()
			bodyBytes := buf.body.Bytes()
			if err := c.Set(ctx, key, bodyBytes, ttl); err != nil && logger != nil {
				logger.Warn("cache set failed", "err", err, "namespace", namespace)
			}
			if allowStale {
				if err := c.Set(ctx, key+staleKeySuffix, bodyBytes, staleTTL); err != nil && logger != nil {
					logger.Warn("stale cache set failed", "err", err, "namespace", namespace)
				}
			}
		})
	}
}

// InvalidateOnWrite returns middleware that purges the given namespaces after
// a successful (2xx) non-GET request, before the response is flushed to the
// client, so a follow-up read never sees the pre-write body.
func InvalidateOnWrite(c ResponseCache, logger *slog.Logger, namespaces ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if c == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := chiMiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			if r.Method == http.MethodGet || ww.Status() < 200 || ww.Status() >= 300 {
				return
			}
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
			defer cancel()
			for _, ns := range namespaces {
				if err := c.Purge(ctx, ns); err != nil {
					if logger != nil {
						logger.Error("cache purge failed", "err", err, "namespace", ns)
					}
					continue
				}
				cachePurges.WithLabelValues(ns).Inc()
			}
		})
	}
}

// RedisCache is the Redis-backed ResponseCache.
type RedisCache struct {
	Client *redis.Client
}

func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := c.Client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, body []byte, ttl time.Duration) error {
	return c.Client.Set(ctx, key, body, ttl).Err()
}

// Purge deletes every key in the namespace (SCAN + UNLINK, so it never
// blocks Redis the way KEYS would).
func (c *RedisCache) Purge(ctx context.Context, namespace string) error {
	iter := c.Client.Scan(ctx, 0, cacheKeyPrefix+namespace+":*", 500).Iterator()
	var batch []string
	for iter.Next(ctx) {
		batch = append(batch, iter.Val())
		if len(batch) == 500 {
			if err := c.Client.Unlink(ctx, batch...).Err(); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(batch) > 0 {
		return c.Client.Unlink(ctx, batch...).Err()
	}
	return nil
}
