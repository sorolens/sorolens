package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
)

// New builds and returns the HTTP router with all middleware and routes wired.
func New(h *handler.Handler) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(OTelMiddleware)

	r.Use(middleware.RequestID)
	r.Use(middleware.CORS)
	r.Use(middleware.Recoverer(h.Logger))
	r.Use(middleware.Logger(h.Logger))
	r.Use(chiMiddleware.StripSlashes)

	r.Use(middleware.RateLimit(h.RedisClient, h.Store))

	// Health (not rate-limited)
	r.Get("/health", h.Health)
	r.Get("/readyz", h.Readyz)

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

		// Cross-contract comparison (issue #324): one round-trip that fans
		// out to the per-contract stats/health lookups in parallel.
		get("/compare", h.CompareContracts)

		// Contracts. Registration mutates shared state, so it requires at
		// least contributor role. Reads stay open.
		r.With(scope, contributor).Post("/contracts", h.RegisterContract)
		get("/contracts", h.ListContracts)
		get("/contracts/{id}", h.GetContract)
		get("/contracts/{id}/events", h.ListEvents)
		get("/contracts/{id}/invocations", h.ListInvocations)
		get("/contracts/{id}/storage", h.ListStorageEntries)
		get("/contracts/{id}/stats", h.ContractStats)
		get("/contracts/{id}/forecast", h.ContractForecast)
		get("/contracts/{id}/snapshot", h.ContractSnapshot)
		get("/contracts/{id}/upgrades", h.ListContractUpgrades)
		get("/contracts/{id}/health-score", h.GetContractHealthScore)
		get("/contracts/{id}/summary", h.ContractSummary)
		get("/contracts/{id}/stream", h.StreamEvents)
		get("/contracts/{id}/graph", h.ContractGraph)
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
		get("/watchdog/stats", h.WatchdogStats)
		get("/watchdog/alerts", h.ListWatchdogAlerts)
		get("/watchdog/contracts", h.ListMonitoredContracts)
		get("/watchdog/contracts/{id}", h.GetMonitoredContract)
		get("/watchdog/contracts/{id}/health", h.ListHealthChecks)
		get("/watchdog/contracts/{id}/alerts", h.ListWatchdogAlerts)
	})

	return r
}
