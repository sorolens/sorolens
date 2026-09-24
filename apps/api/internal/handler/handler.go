package handler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/graph"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/simulator"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// APIStore is the combined read/write interface required by the HTTP handlers.
type APIStore interface {
	store.Store
	store.ContractBulkStore
	store.QueryStore
	store.LiveStore
	store.ArchiveStore
	store.WatchdogStore
	store.ContractUpgradeStore
	store.ContractSpecStore
	store.HealthScoreStore
	store.APIKeyStore
	store.AlertSubscriptionStore
	store.AlertGroupStore
	store.WatchlistStore
	store.UserStore
	store.PerformanceStore
	store.ContractNoteStore
	store.GlobalEventStore
	store.LabelStore
	store.RuleStore
	store.WatchedAccountStore
	store.AuditStore
	store.GroupStore
	store.ContractVerificationStore
	store.ContractWasmStore
	store.FailedEventStore
}

// Pinger is implemented by both the postgres pool and the Redis client.
type Pinger interface {
	Ping(ctx context.Context) error
}

type RedisClient interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
}

// ColdEventReader serves events that have been archived out of Postgres into
// cold storage (issue #146). It is satisfied by *coldstorage.Reader. A nil
// Cold disables the fallback, which is the default for local development and
// for deployments that have not configured a cold bucket.
type ColdEventReader interface {
	Events(ctx context.Context, contractID string, from, to uint32, limit int) ([]store.Event, error)
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

	// GraphQL configures the /graphql endpoint (issue #125).
	GraphQL graph.Options

	// Verifier rebuilds submitted contract source and compares the resulting
	// Wasm hash against the on-chain hash (issue #263). It is nil on
	// deployments without a build sandbox (for example the Vercel serverless
	// entrypoint), in which case POST /contracts/{id}/verify answers 503.
	Verifier ContractVerifier

	// Simulator runs dry-run invocations for POST /simulate. When nil, the
	// handler falls back to a default service that caches results for 30s.
	Simulator *simulator.Service

	// Cold is optional; when set, event queries fall back to object storage for
	// ledger ranges that are no longer in Postgres.
	Cold ColdEventReader

	// summaryCacheOnce guards lazy construction of summaryCache, the
	// process-wide memo for composite per-contract dashboard summaries.
	summaryCacheOnce sync.Once
	summaryCacheVal  *SummaryCache
}
