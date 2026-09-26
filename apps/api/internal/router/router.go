package router

import (
	"net/http"
	"net/http/pprof"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/metrics"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
)

// New builds and returns the HTTP router with all middleware and routes wired.
// maxBodyBytes caps the request body size in bytes; values of zero or less
// disable the limit. Callers normally pass config.MaxBodyBytesFromEnv().
func New(h *handler.Handler, maxBodyBytes int64) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(OTelMiddleware)

	r.Use(middleware.RequestID)
	r.Use(middleware.CORS)
	// Sentry must run before Recoverer: it reports a panic and re-panics so
	// Recoverer still produces the standard 500 response.
	r.Use(middleware.Sentry)
	r.Use(middleware.Recoverer(h.Logger))
	r.Use(middleware.BodyLimit(maxBodyBytes))
	r.Use(middleware.Logger(h.Logger))
	r.Use(chiMiddleware.StripSlashes)
	r.Use(middleware.Metrics)

	// Emit ETags on cacheable GET/HEAD responses and answer If-None-Match
	// matches with an empty 304, short-circuiting the body before it
	// crosses the wire (issue #152).
	r.Use(middleware.ETag)

	r.Use(middleware.RateLimit(h.RedisClient, h.Store))

	// Health (not rate-limited)
	r.Get("/health", h.Health)
	r.Get("/readyz", h.Readyz)

	// Version (public, not rate-limited, no DB access). Mounted at /api/version
	// rather than under /api/v1 so it stays reachable without a versioned client
	// and without the v1 scope/role middleware.
	r.Get("/api/version", h.Version)

	// Prometheus metrics. It sits outside /api/v1 so it needs no API key, and
	// the rate limiter skips it explicitly (see middleware.RateLimit) so a
	// scrape is never throttled. A registry per router keeps tests that build
	// many routers independent; it carries the request collectors from the
	// metrics package (updated by middleware.Metrics), the response-cache
	// counters (issue #143), and the default Go and process collectors.
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(middleware.CacheCollectors()...)
	reg.MustRegister(metrics.HTTPRequestsInFlight, metrics.HTTPRequestsTotal, metrics.HTTPRequestDuration)
	r.Get("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP)
	pprofAdmin := middleware.RequireRoleOrForbidden(h.Store, h.Logger, middleware.RoleAdmin)
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
	adminOnly := middleware.RequireRoleOrForbidden(h.Store, h.Logger, middleware.RoleAdmin)
	r.With(adminOnly).HandleFunc("/debug/pprof", pprof.Index)
	r.With(adminOnly).HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.With(adminOnly).HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.With(adminOnly).HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.With(adminOnly).HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.With(adminOnly).HandleFunc("/debug/pprof/{name}", pprof.Index)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.ContentTypeJSON)

		// Liveness probe with dependency reachability. It always returns 200
		// and carries no scope rule, so uptime monitors can poll it without a
		// credential.
		r.Get("/health", h.HealthCheck)

		// Scoped API key auth. It is applied per route with r.With so chi has
		// already resolved the leaf route pattern when the middleware runs; the
		// required scope is looked up from the metadata table in
		// middleware/scopes.go keyed by that pattern. Requests without a
		// credential still pass on read/write routes (public v0.1 surface),
		// while API key management always requires a credential.
		scope := middleware.RequireScopes(h.Store, h.Logger)
		// RBAC: role enforcement on top of scope. Route wiring uses r.With
		// (same pattern as scope) so the role middleware denies a request
		// that lacks the minimum role regardless of API key scopes.
		contributor := middleware.RequireRole(h.Store, h.Logger, middleware.RoleContributor)
		admin := middleware.RequireRole(h.Store, h.Logger, middleware.RoleAdmin)

		// Request timeouts (issue #154). The SSE stream is registered first
		// with its own long deadline; r is then rebound so every route
		// registered below inherits the default cap and returns 503 past it.
		r.With(scope, middleware.StreamTimeout(h.StreamTimeoutOrDefault())).Get("/stream/events", h.StreamEventsSSE)
		r = r.With(middleware.Timeout(h.RequestTimeoutOrDefault()))

		get := func(pattern string, fn http.HandlerFunc) { r.With(scope).Get(pattern, fn) }

		// Stats
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
		// Search contracts (issue #181)
		get("/search", h.SearchContracts)

		// Cross-contract comparison (issue #324): one round-trip that fans
		// out to the per-contract stats/health lookups in parallel.
		get("/compare", h.CompareContracts)

		// Response cache (issue #143): hot reads are cached for CacheTTL and
		// a successful write purges the namespace it changes.
		cacheContracts := middleware.Cache(h.Cache, middleware.CacheNamespaceContracts, h.CacheTTL, h.Logger)
		cacheWatchdog := middleware.Cache(h.Cache, middleware.CacheNamespaceWatchdog, h.CacheTTL, h.Logger)
		purgeContracts := middleware.InvalidateOnWrite(h.Cache, h.Logger, middleware.CacheNamespaceContracts)
		cacheLabels := middleware.Cache(h.Cache, middleware.CacheNamespaceLabels, h.CacheTTL, h.Logger)
		purgeLabels := middleware.InvalidateOnWrite(h.Cache, h.Logger, middleware.CacheNamespaceLabels)

		// Contracts. Registration mutates shared state, so it requires at
		// least contributor role. Reads stay open.
		//
		// /contracts/validate is the tracking wizard's read-only pre-flight
		// check (issue #140). It is a POST purely to keep the path
		// unambiguous against /contracts/{id}; it never writes.
		r.With(scope).Post("/contracts/validate", h.ValidateContract)
		r.With(scope, contributor, purgeContracts).Post("/contracts", h.RegisterContract)
		r.With(scope, contributor, purgeLabels).Post("/labels", h.CreateLabel)
		r.With(scope, cacheLabels).Get("/labels", h.ListLabels)
		r.With(scope, cacheLabels).Get("/resolve", h.ResolveLabel)
		r.With(scope, cacheContracts).Get("/contracts", h.ListContracts)
		r.With(scope, cacheContracts).Get("/contracts/{id}", h.GetContract)
		get("/contracts/{id}/events", h.ListEvents)
		get("/contracts/{id}/events.csv", h.ExportEventsCSV)
		get("/contracts/{id}/invocations", h.ListInvocations)
		// Global invocation explorer: resource usage across every contract.
		get("/invocations", h.ListAllInvocations)
		get("/contracts/{id}/storage", h.ListStorageEntries)
		get("/contracts/{id}/stats", h.ContractStats)
		get("/contracts/{id}/forecast", h.ContractForecast)
		get("/contracts/{id}/snapshot", h.ContractSnapshot)
		get("/contracts/{id}/snapshot.json", h.ContractSnapshotExport)
		get("/contracts/{id}/upgrades", h.ListContractUpgrades)
		get("/contracts/{id}/health-score", h.GetContractHealthScore)
		get("/contracts/{id}/summary", h.ContractSummary)
		get("/contracts/{id}/stream", h.StreamEvents)
		get("/contracts/{id}/graph", h.ContractGraph)
		get("/contracts/{id}/spec", h.GetContractSpec)

		// User-defined contract tags (issue #459). Wrapped by the contributor
		// role so an anonymous caller cannot label a contract even though the
		// write scope passes on the public surface.
		r.With(scope, contributor).Post("/contracts/{id}/tags", h.AddContractTag)
		r.With(scope, contributor).Delete("/contracts/{id}/tags/{tag}", h.RemoveContractTag)
		// Dead-letter queue for events that failed processing (issue #202).
		get("/dlq", h.ListFailedEvents)
		r.With(scope, contributor).Post("/dlq/{id}/requeue", h.RequeueFailedEvent)

		// API keys (admin scope + admin role).
		r.With(scope, admin).Get("/api-keys", h.ListAPIKeys)
		r.With(scope, admin).Post("/api-keys", h.CreateAPIKey)
		r.With(scope, admin).Delete("/api-keys/{id}", h.RevokeAPIKey)

		// Admin surface. Wrapped by role admin so contributors cannot reach
		// these endpoints even when the API key carries admin scope.
		r.With(admin).Route("/admin", func(r chi.Router) {
			r.Get("/keys", h.ListAPIKeys)
			r.Post("/keys", h.CreateAPIKey)
			r.Delete("/keys/{id}", h.RevokeAPIKey)
		})

		// Watchlist
		r.Route("/watchlist", func(r chi.Router) {
			r.Post("/", h.AddToWatchlist)
			r.Delete("/{contractId}", h.RemoveFromWatchlist)
			r.Get("/", h.ListWatchlist)
			r.Get("/{contractId}/status", h.WatchlistStatus)
		})

		// Watchdog: data from the on-chain sorolens-watchdog contract.
		// Watchdog stats are written by the indexer, not the API, so they
		// rely on the TTL rather than write invalidation.
		r.With(scope, cacheWatchdog).Get("/watchdog/stats", h.WatchdogStats)
		get("/watchdog/alerts", h.ListWatchdogAlerts)
		get("/watchdog/contracts", h.ListMonitoredContracts)
		get("/watchdog/contracts/{id}", h.GetMonitoredContract)
		get("/watchdog/contracts/{id}/health", h.ListHealthChecks)
		get("/watchdog/contracts/{id}/alerts", h.ListWatchdogAlerts)

		// User-defined alert rules (DSL). Reads are open; writes require a
		// contributor. The samples/preview sub-routes are registered before the
		// {id} route so "samples" and "preview" are never captured as an id.
		r.With(scope, contributor).Post("/rules", h.CreateRule)
		get("/rules", h.ListRules)
		get("/rules/samples", h.ListRuleSamples)
		r.With(scope, contributor).Post("/rules/preview", h.PreviewRule)
		get("/rules/{id}", h.GetRule)
		r.With(scope, contributor).Put("/rules/{id}", h.UpdateRule)
		r.With(scope, contributor).Delete("/rules/{id}", h.DeleteRule)
	})

	return r
}
