package router

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sorolens/sorolens/apps/api/internal/graph"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/metrics"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
)

// corsMiddleware returns an http.Handler middleware that sets CORS headers.
//
// When CORS_ALLOWED_ORIGINS is unset or contains "*" all origins are allowed
// (Access-Control-Allow-Origin: *). When it is a comma-separated list of
// origins, only requests whose Origin header is in the list receive an echoed
// origin back; all others receive the first entry in the list (the browser
// will still block the mismatch). This lets a self-hosted API restrict
// cross-origin access to the Vercel dashboard domain without any code change.
func corsMiddleware() func(http.Handler) http.Handler {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")

	// Build the allow-set. nil means wildcard (*).
	var allowed map[string]struct{}
	if raw != "" {
		allowed = make(map[string]struct{})
		for _, o := range strings.Split(raw, ",") {
			o = strings.TrimSpace(o)
			if o == "" {
				continue
			}
			if o == "*" {
				allowed = nil // wildcard overrides any explicit list
				break
			}
			allowed[o] = struct{}{}
		}
		if len(allowed) == 0 {
			allowed = nil
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			var allowOrigin string
			if allowed == nil {
				allowOrigin = "*"
			} else if _, ok := allowed[origin]; ok {
				allowOrigin = origin
			} else {
				// Fall back to the first configured origin; the browser will
				// reject the mismatch, but the response header is always set.
				for o := range allowed {
					allowOrigin = o
					break
				}
			}

			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, Authorization, X-API-Key")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			if allowOrigin != "*" {
				w.Header().Set("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// New builds and returns the HTTP router with all middleware and routes wired.
// maxBodyBytes caps the request body size in bytes; values of zero or less
// disable the limit. Callers normally pass config.MaxBodyBytesFromEnv().
func New(h *handler.Handler, maxBodyBytes int64) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(OTelMiddleware)

	r.Use(middleware.RequestID)
	r.Use(corsMiddleware())
	// Sentry must run before Recoverer: it reports a panic and re-panics so
	// Recoverer still produces the standard 500 response.
	r.Use(middleware.Sentry)
	r.Use(middleware.Recoverer(h.Logger))
	r.Use(middleware.BodyLimit(maxBodyBytes))
	r.Use(middleware.Logger(h.Logger))
	// Verify the caller's credential once and attach the resulting identity
	// to the request context (issue #518). Must run before Audit, which
	// attributes writes to the verified caller, and before the per-route
	// scope/role middleware. Bare identity headers are never trusted.
	r.Use(middleware.Identity(h.Store, middleware.IdentityOptions{
		Sessions:    h.SessionSigner,
		ProxySecret: h.TrustedProxySecret,
	}, h.Logger))
	// Audit trail for every mutating /api/ request (issue #122). It sits
	// outside rate limiting, content-type and auth checks so their
	// rejections are audited too, and inside Recoverer so a panic is
	// recorded as 500 before being recovered.
	r.Use(middleware.Audit(h.Store, h.Logger, "/api/"))
	r.Use(chiMiddleware.StripSlashes)
	r.Use(middleware.Metrics)

	// Emit ETags on cacheable GET/HEAD responses and answer If-None-Match
	// matches with an empty 304, short-circuiting the body before it
	// crosses the wire (issue #152).
	r.Use(middleware.ETag)

	// Compress responses (gzip/br) for clients that ask for it. Must run
	// before the rate limiter so compressed and uncompressed variants of a
	// route share one rate-limit bucket (responses are buffered until the
	// compression decision, so limiter headers written later are unaffected).
	r.Use(middleware.Compression)

	r.Use(middleware.RateLimit(h.RedisClient, h.Store))

	// Health (not rate-limited)
	r.Get("/health", h.Health)
	r.Get("/readyz", h.Readyz)

	// Version (public, not rate-limited, no DH access). Mounted at /api/version
	// rather than under /api/v1 so it stays reachable without a versioned client
	// and without the v1 scope/role middleware.
	r.Get("/api/version", h.Version)

	// Prometheus metrics. It sits outside /api/v1 so it needs no API key, and
	// the rate limiter skips it explicitly (see middleware.RateLimit) so a
	// scrape is never throttled. A registry per router keeps tests that build
	// many routers independent; it carries the request collectors from the
	// metrics package (updated by middleware.Metrics), the response-cache
	// counters (issue #143), the rate-limiter decision counters, the Postgres
	// pool collector (metrics.SetPool wires the live pool), and the default Go
	// and process collectors.
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(middleware.CacheCollectors()...)
	reg.MustRegister(middleware.RateLimitCollectors()...)
	reg.MustRegister(metrics.HTTPRequestsInFlight, metrics.HTTPRequestsTotal, metrics.HTTPRequestDuration)
	reg.MustRegister(metrics.PoolCollector())
	r.Get("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP)
	pprofAdmin := middleware.RequireRoleOrForbidden(h.Logger, middleware.RoleAdmin)
	r.With(pprofAdmin).Get("/debug/pprof", pprof.Index)
	r.With(pprofAdmin).Get("/debug/pprof/", pprof.Index)
	r.With(pprofAdmin).Get("/debug/pprof/cmdline", pprof.Cmdline)
	r.With(pprofAdmin).Get("/debug/pprof/profile", pprof.Profile)
	r.With(pprofAdmin).Get("/debug/pprof/symbol", pprof.Symbol)
	r.With(pprofAdmin).Post("/debug/pprof/symbol", pprof.Symbol)
	r.With(pprofAdmin).Get("/debug/pprof/trace", pprof.Trace)
	for _, profile := range []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"} {
		r.With(pprofAdmin).Get("/debug/pprof/"+profile, pprof.Handler(profile).ServeHTTP)
	}

	// Slack slash commands (issue #127). Outside /api/v1 because Slack posts
	// form-encoded bodies, which the JSON content-type guard would reject;
	// every request is authenticated by its Slack signature instead.
	r.Post("/integrations/slack/commands", h.SlackCommand)

	// pprof (issue #157). Gated behind admin role so probing always gets 403
	// rather than 401, avoiding path enumeration by unauthenticated callers.
	adminOnly := middleware.RequireRoleOrForbidden(h.Logger, middleware.RoleAdmin)
	r.With(adminOnly).HandleFunc("/debug/pprof", pprof.Index)
	r.With(adminOnly).HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.With(adminOnly).HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.With(adminOnly).HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.With(adminOnly).HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.With(adminOnly).HandleFunc("/debug/pprof/{name}", pprof.Index)

	// GraphQL (issue #125): read-only, POST only. Same credential rules as
	// the REST read routes: anonymous is allowed, an API key needs
	// read:contracts.
	gql, err := graph.NewHandler(h.Store, h.GraphQL)
	if err != nil {
		// Only fails if the embedded persisted queries are unreadable,
		// which is a build defect.
		panic(fmt.Sprintf("graphql handler: %v", err))
	}
	r.With(middleware.RequireScopes(h.Logger)).Post("/graphql", gql.ServeHTTP)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.ContentTypeJSON)

		// Liveness probe with dependency reachability. It always returns 200
		// and carries no scope rule, so uptime monitors can poll it without a
		// credential.
		r.Get("/health", h.HealthCheck)

		// Workspace sessions (issue #518). Unauthenticated and unscoped by
		// design: the server mints an unguessable identity and returns an
		// HMAC-signed token the dashboard sends as a bearer credential.
		// Global rate limiting still applies.
		r.Post("/auth/session", h.CreateSession)

		// Scoped API key auth. It is applied per route with r.With so chi has
		// already resolved the leaf route pattern when the middleware runs; the
		// required scope is looked up from the metadata table in
		// middleware/scopes.go keyed by that pattern. Requests without a
		// credential still pass on read/write routes (public v0.1 surface),
		// while API key management always requires a credential.
		scope := middleware.RequireScopes(h.Logger)
		// RBAC: role enforcement on top of scope. Route wiring uses r.With
		// (same pattern as scope) so the role middleware denies a request
		// that lacks the minimum role regardless of API key scopes.
		contributor := middleware.RequireRole(h.Logger, middleware.RoleContributor)
		admin := middleware.RequireRole(h.Logger, middleware.RoleAdmin)

		// Request timeouts (issue #154). The SSE stream is registered first
		// with its own long deadline; r is then rebound so every route
		// registered below inherits the default cap and returns 503 past it.
		r.With(scope, middleware.StreamTimeout(h.StreamTimeoutOrDefault())).Get("/stream/events", h.StreamEventsSSE)
		// WebSocket subscriptions (issue #126): real-time push with live filter
		// changes, complementary to the SSE stream above. Registered before the
		// default Timeout rebind (it buffers responses, which breaks the upgrade
		// handshake) and without StreamTimeout (its write deadline would cut
		// hijacked connections); the handler enforces its own liveness via
		// protocol heartbeats.
		r.With(scope).Get("/subscribe", h.SubscribeEventsWS)
		r = r.With(middleware.Timeout(h.RequestTimeoutOrDefault()))

		get := func(pattern string, fn http.HandlerFunc) { r.With(scope).Get(pattern, fn) }

		// Stats
		// /stats/global is cached briefly and backed by row-count estimates
		// (issue #530). The handler serves from a short TTL cache and reports
		// an `approximate` indicator when counts come from pg_class.reltuples.
		get("/stats/global", h.GlobalStats)

		// Live dashboard feeds (#139): the newest events across all tracked
		// contracts, and per-contract events-per-minute buckets for the
		// sparklines and hot-contracts leaderboard.
		get("/events/recent", h.RecentEvents)
		get("/stats/activity", h.LiveActivity)

		// Cross-contract events explorer feed (issue #97).
		get("/events", h.ListAllEvents)

		// Alert deduplication and grouping engine (issue #269).
		// GET /api/v1/alerts          — grouped view (default)
		// GET /api/v1/alerts?flat=true — raw ContractAlert feed
		get("/alerts", h.ListAlerts)
		// User-defined alert rules (rule language). Reads are public like the
		// rest of the v0.1 surface; authoring mutates shared state, so it needs
		// the contributor role like contract registration.
		get("/rules", h.ListRules)
		get("/rules/metrics", h.ListRuleMetrics)
		get("/rules/library", h.ListRuleLibrary)
		r.With(scope).Post("/rules/validate", h.ValidateRule)
		r.With(scope).Post("/rules/preview", h.PreviewRule)
		r.With(scope, contributor).Post("/rules", h.CreateRule)
		r.With(scope, contributor).Patch("/rules/{id}", h.SetRuleEnabled)
		r.With(scope, contributor).Delete("/rules/{id}", h.DeleteRule)

		// Search contracts (issue #181)
		get("/search", h.SearchContracts)

		// Cross-contract comparison (issue #324): one round-trip that fans
		// out to the per-contract stats/health lookups in parallel.
		get("/compare", h.CompareContracts)

		// Response cache (issue #143): hot reads are cached for CacheTTL and
		// a successful write purges the namespace it changes.
		cacheContracts := middleware.Cache(h.Cache, middleware.CacheNamespace
	})
}
