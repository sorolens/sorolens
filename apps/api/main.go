package main

import (
	"context"
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
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
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

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Error("redis parse url", "err", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	h := &handler.Handler{
		Store:       store.NewFullStore(pool),
		DB:          &dbPinger{pool: pool},
		Redis:       &redisPinger{client: redisClient},
		RedisClient: &realRedisClient{client: redisClient},
		Logger:      logger,

		Cache:              &middleware.RedisCache{Client: redisClient},
		CacheTTL:           cfg.CacheTTL,
		SlackSigningSecret: cfg.SlackSigningSecret,
		RequestTimeout:     cfg.RequestTimeout,
		StreamTimeout:      cfg.StreamTimeout,
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
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:     router.New(h, maxBodyBytes),
		ReadTimeout: 15 * time.Second,
		// WriteTimeout starts before the handler's own timer, so it must
		// outlast API_REQUEST_TIMEOUT or the 503 is cut off mid-write and the
		// client sees a dropped connection. The SSE route extends its own
		// write deadline per request (middleware.StreamTimeout).
		WriteTimeout: cfg.RequestTimeout + 5*time.Second,
		IdleTimeout:  60 * time.Second,
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

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
	logger.Info("shutdown complete")
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
