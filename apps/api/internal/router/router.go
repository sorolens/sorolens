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
	r.Use(middleware.RequestID)
	r.Use(middleware.CORS)
	r.Use(middleware.Recoverer(h.Logger))
	r.Use(middleware.Logger(h.Logger))
	r.Use(middleware.RateLimit())
	r.Use(chiMiddleware.StripSlashes)

	// Health
	r.Get("/health", h.Health)
	r.Get("/readyz", h.Readyz)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.ContentTypeJSON)

		// Stats
		r.Get("/stats/global", h.GlobalStats)

		// Contracts
		r.Post("/contracts", h.RegisterContract)
		r.Get("/contracts", h.ListContracts)
		r.Route("/contracts/{id}", func(r chi.Router) {
			r.Get("/", h.GetContract)
			r.Get("/events", h.ListEvents)
			r.Get("/invocations", h.ListInvocations)
			r.Get("/storage", h.ListStorageEntries)
			r.Get("/stats", h.ContractStats)
			r.Get("/stream", h.StreamEvents)
		})
	})

	return r
}
