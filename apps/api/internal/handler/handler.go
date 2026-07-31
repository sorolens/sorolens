package handler

import (
	"context"
	"log/slog"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// APIStore is the combined read/write interface required by the HTTP handlers.
type APIStore interface {
	store.Store
	store.QueryStore
}

// Pinger is implemented by both the postgres pool and the Redis client.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	Store  APIStore
	DB     Pinger
	Redis  Pinger
	Logger *slog.Logger
}
