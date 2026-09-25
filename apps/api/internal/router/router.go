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
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
)

// New builds and returns the HTTP router with all middleware and routes wired.
// maxBodyBytes caps the request body size in bytes; values of zero or less
// disable the limit. Callers normally pass cfg.RequestMaxBodyBytes.
func New(h *handler.Handler, maxBodyBytes int64) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(OTelMiddleware)

	r.Use(middleware.RequestID)
	r.Use(middleware.CORS)
	r.Use(middleware.Recoverer(h.Logger))
	r.Use(middleware.BodyLimit(maxBodyBytes))
	r.Use(middleware.Logger(h.Logger))
	r.Use(chiMiddleware.StripSlashes)

	// Emit ETags on cacheable GET/HEAD responses and answer If-None-Match
	// matches with an empty 304, short-circuiting the body before it
	// crosses the wire (issue #152).
	r.Use(middleware.ETag)

	r.Use(middleware.RateLimit(h.RedisClient, h.Store))

	// Health (not rate-limited)
	r.Get("/health", h.Health)
	r.Get("/readyz", h.Readyz)

	// Prometheus metrics (issue #143: response cache hit/miss counters). A
	// registry per router keeps tests that build many routers independent.
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(middleware.CacheCollectors()...)
	r.Get("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP)

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

		get := func(pattern string, fn http.HandlerFunc) { r.With(scope).Get(pattern, fn) }

		// Stats
		get("/stats/global", h.GlobalStats)

		// Response cache (issue #143): hot reads are cached for CacheTTL and
		// a successful write purges the namespace it changes.
		cacheContracts := middleware.Cache(h.Cache, middleware.CacheNamespaceContracts, h.CacheTTL, h.Logger)
		cacheWatchdog := middleware.Cache(h.Cache, middleware.CacheNamespaceWatchdog, h.CacheTTL, h.Logger)
		purgeContracts := middleware.InvalidateOnWrite(h.Cache, h.Logger, middleware.CacheNamespaceContracts)

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

		// Contracts. Registration mutates shared state, so it requires at
		// least contributor role. Reads stay open.
		r.With(scope, contributor, purgeContracts).Post("/contracts", h.RegisterContract)
		r.With(scope, cacheContracts).Get("/contracts", h.ListContracts)
		r.With(scope, cacheContracts).Get("/contracts/{id}", h.GetContract)
		get("/contracts/{id}/events", h.ListEvents)
		get("/contracts/{id}/invocations", h.ListInvocations)
		// Global invocation explorer: resource usage across every contract.
		get("/invocations", h.ListAllInvocations)
		get("/contracts/{id}/storage", h.ListStorageEntries)
		get("/contracts/{id}/stats", h.ContractStats)
		get("/contracts/{id}/forecast", h.ContractForecast)
		get("/contracts/{id}/snapshot", h.ContractSnapshot)
		get("/contracts/{id}/upgrades", h.ListContractUpgrades)
		get("/contracts/{id}/health-score", h.GetContractHealthScore)
		get("/contracts/{id}/summary", h.ContractSummary)
		get("/contracts/{id}/stream", h.StreamEvents)
		get("/contracts/{id}/graph", h.ContractGraph)

		// Source verification (issue #263). Submitting source mutates the
		// verification record, so it requires at least contributor role;
		// reading the cached verdict stays open.
		r.With(scope, contributor).Post("/contracts/{id}/verify", h.VerifyContract)
		get("/contracts/{id}/verification", h.GetContractVerification)
		get("/stream/events", h.StreamEventsSSE)

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
		get("/watchdog/contracts/{id}/uptime", h.GetContractUptime)

		// Monthly SLA and uptime reporting (issue #266). Reports are derived
		// from the watchdog health checks and alerts already stored, so there
		// is no new ingestion path. The badge is plain SVG so it can be
		// embedded in a README without a client library.
		get("/reports/{contract_id}", h.GetContractReport)
		get("/reports/{contract_id}/history", h.GetContractReportHistory)
		get("/reports/{contract_id}/badge.svg", h.GetContractSLABadge)

		// Alert notification subscriptions (issue #127). They hold
		// integration secrets, so reading them also needs contributor.
		r.With(scope, contributor).Post("/watchdog/subscriptions", h.CreateSubscription)
		r.With(scope, contributor).Get("/watchdog/subscriptions", h.ListSubscriptions)
		r.With(scope, contributor).Delete("/watchdog/subscriptions/{id}", h.DeleteSubscription)
	})

	return r
}
