package handler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// APIStore is the combined read/write interface required by the HTTP handlers.
type APIStore interface {
	store.Store
	store.QueryStore
	store.WatchdogStore
	store.ContractUpgradeStore
	store.HealthScoreStore
	store.APIKeyStore
	store.AlertSubscriptionStore
	store.AlertGroupStore
	store.WatchlistStore
	store.UserStore
	store.PerformanceStore
	store.ContractNoteStore
	store.FailedEventStore
	store.GlobalEventStore
	store.LabelStore
}

// Pinger is implemented by both the postgres pool and the Redis client.
type Pinger interface {
	Ping(ctx context.Context) error
}

type RedisClient interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
}

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	Store       APIStore
	DB          Pinger
	Redis       Pinger
	RedisClient RedisClient
	Logger      *slog.Logger
	StreamHub   *StreamHub

	// Cache stores hot GET responses (issue #143). Nil disables caching.
	Cache middleware.ResponseCache
	// CacheTTL is how long a cached response lives. Zero disables caching.
	CacheTTL time.Duration
	// SlackSigningSecret verifies Slack slash command requests (issue #127).
	// Empty disables the Slack command endpoint.
	SlackSigningSecret string

	// ReportSigningKey signs exported SLA reports (issue #266). When empty the
	// reporting handlers fall back to REPORT_SIGNING_KEY; with neither set the
	// reports are emitted unsigned and every response says so.
	ReportSigningKey string

	// summaryCacheOnce guards lazy construction of summaryCache, the
	// process-wide memo for composite per-contract dashboard summaries.
	summaryCacheOnce sync.Once
	summaryCacheVal  *SummaryCache
}
