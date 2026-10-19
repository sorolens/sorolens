package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sorolens/sorolens/apps/api/internal/config"
	"github.com/sorolens/sorolens/apps/api/internal/graph"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/metrics"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
	"github.com/sorolens/sorolens/apps/api/internal/verify"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "err", err)
		os.Exit(1)
	}

	if cfg.SentryDSN != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:         cfg.SentryDSN,
			Environment: cfg.SentryEnvironment,
		}); err != nil {
			logger.Error("sentry init", "err", err)
		} else {
			defer sentry.Flush(2 * time.Second)
		}
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logger.Error("parse config", "err", err)
		os.Exit(1)
	}
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		logger.Error("postgres connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Export the live pool on every /metrics endpoint: the snapshot is read
	// lazily on each scrape, so no background sampler is needed.
	metrics.SetPool(func() metrics.PoolStat { return pool.Stat() })

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Error("redis parse url", "err", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close() //nolint:errcheck // best-effort close on shutdown

	// Source verification runs builds on the API host, so it is only wired in
	// the long-running server (not the Vercel serverless entrypoint) and is
	// bounded by VERIFY_TIMEOUT. See internal/verify for the isolation caveats.
	verifier := verify.NewService(verify.Options{
		BuildCommand: cfg.VerifyBuildCommand,
		Timeout:      cfg.VerifyTimeout,
		WorkspaceDir: cfg.VerifyWorkspaceDir,
	})

	h := &handler.Handler{
		Store:       store.NewFullStore(pool),
		DB:          &dbPinger{pool: pool},
		Redis:       &redisPinger{client: redisClient},
		RedisClient: &realRedisClient{client: redisClient},
		Logger:      logger,
		Verifier:    verifier,

		Cache:              &middleware.RedisCache{Client: redisClient},
		CacheTTL:           cfg.CacheTTL,
		SlackSigningSecret: cfg.SlackSigningSecret,
		SessionSigner:      middleware.NewSessionSigner(identitySigningKey(cfg, logger), cfg.IdentitySessionTTL),
		TrustedProxySecret: cfg.TrustedIdentityProxySecret,
		RequestTimeout:     cfg.RequestTimeout,
		StreamTimeout:      cfg.StreamTimeout,
		GraphQL: graph.Options{
			ComplexityLimit: cfg.GraphQLComplexityLimit,
			PersistedOnly:   cfg.GraphQLPersistedOnly,
		},
	}

	if err := seedInitialAdmin(context.Background(), h.Store, cfg.InitialAdminGitHubID, logger); err != nil {
		logger.Error("seed initial admin", "err", err)
		os.Exit(1)
	}

	maxBodyBytes, err := config.MaxBodyBytesFromEnv()
	if err != nil {
		logger.Error("config", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:        fmt.Sprintf(":%s", cfg.Port),
		Handler:     router.New(h, maxBodyBytes),
		ReadTimeout: 15 * time.Second,
		// WriteTimeout starts before the handler's own timer, so it must
		// outlast API_REQUEST_TIMEOUT or the 503 is cut off mid-write and the
		// client sees a dropped connection. The SSE route extends its own
		// write deadline per request (middleware.StreamTimeout).
		WriteTimeout: cfg.RequestTimeout + 5*time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Optional dedicated metrics listener. When METRICS_PORT is set, GET
	// /metrics is served there without authentication so the exposition can be
	// scraped on a private interface rather than the public API port.
	//
	// The admin listener serves the package-level registry, which owns the
	// HTTP and pool collectors. The cache and rate-limiter counters live in
	// the middleware package and are registered by the router on its own
	// registry, so register them here too — METRICS_PORT then exposes the
	// same series as GET /metrics on the main port.
	metrics.Registry.MustRegister(middleware.CacheCollectors()...)
	metrics.Registry.MustRegister(middleware.RateLimitCollectors()...)
	var metricsSrv *http.Server
	if cfg.MetricsPort != "" {
		metricsSrv = &http.Server{
			Addr:         fmt.Sprintf(":%s", cfg.MetricsPort),
			Handler:      metrics.NewAdminMux(),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("sorolens/api listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	if metricsSrv != nil {
		go func() {
			logger.Info("sorolens/api metrics listening", "port", cfg.MetricsPort)
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("metrics listen", "err", err)
				os.Exit(1)
			}
		}()
	}

	<-ctx.Done()
	logger.Info("shutting down on SIGTERM/SIGINT, draining in-flight requests")

	// Drain in-flight requests with a 30s timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
	if metricsSrv != nil {
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("metrics shutdown", "err", err)
		}
	}
	logger.Info("shutdown complete")
}

// identitySigningKey returns the configured session signing key, or a fresh
// ephemeral key when the operator did not set IDENTITY_SIGNING_KEY. The
// ephemeral fallback keeps local development working without shipping a
// hard-coded secret; sessions then reset on restart and do not work across
// replicas, so production must configure the variable.
func identitySigningKey(cfg *config.Config, logger *slog.Logger) string {
	if cfg.IdentitySigningKey != "" {
		return cfg.IdentitySigningKey
	}
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		logger.Error("generate ephemeral identity signing key", "err", err)
		os.Exit(1)
	}
	logger.Warn("IDENTITY_SIGNING_KEY not set; generated an ephemeral key (sessions reset on restart)")
	return hex.EncodeToString(buf[:])
}

// seedInitialAdmin creates the bootstrap admin user when the
// INITIAL_ADMIN_GITHUB_ID environment variable is set. It is idempotent:
// the user's row is keyed by its GitHub ID (also used as the row ID) so the
// same bootstrap value always resolves to the same admin on restarts.
func seedInitialAdmin(ctx context.Context, s store.FullStore, githubID string, logger *slog.Logger) error {
	if githubID == "" {
		logger.Info("INITIAL_ADMIN_GITHUB_ID not set; skipping admin seed")
		return nil
	}
	id := githubID
	if err := s.UpsertUser(ctx, store.User{
		ID:       id,
		GitHubID: &githubID,
		Role:     store.RoleAdmin,
	}); err != nil {
		return fmt.Errorf("seed admin user %q: %w", githubID, err)
	}
	logger.Info("seeded initial admin", "github_id", githubID)
	return nil
}

type dbPinger struct{ pool *pgxpool.Pool }

func (p *dbPinger) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

type redisPinger struct{ client *redis.Client }

func (p *redisPinger) Ping(ctx context.Context) error { return p.client.Ping(ctx).Err() }

type realRedisClient struct{ client *redis.Client }

func (r *realRedisClient) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}
func (r *realRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return r.client.Expire(ctx, key, expiration).Result()
}
