// Package handler is the Vercel Go serverless entrypoint for the Sorolens API.
// Vercel serverless functions do not support long-lived connections; the
// /stream endpoint therefore uses polling rather than SSE.
package handler

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	apiconfig "github.com/sorolens/sorolens/apps/api/internal/config"
	sorohandler "github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/metrics"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

var (
	once    sync.Once
	handler http.Handler
)

// Handler is the Vercel Go serverless function entrypoint.
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		var pool *pgxpool.Pool
		config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
		if err == nil {
			poolSettings, settingsErr := apiconfig.PoolSettingsFromEnv()
			if settingsErr != nil {
				logger.Error("pool config", "err", settingsErr)
				return
			}
			poolSettings.Apply(config)
			config.ConnConfig.Tracer = otelpgx.NewTracer()
			var p *pgxpool.Pool
			p, err = pgxpool.NewWithConfig(context.Background(), config)
			pool = p
		}
		if err != nil {
			logger.Error("postgres connect", "err", err)
			return
		}
		// Export the live pool on GET /metrics (read lazily per scrape).
		metrics.SetPool(func() metrics.PoolStat { return pool.Stat() })

		var redisClient *redis.Client
		if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
			redisOpts, redisErr := redis.ParseURL(redisURL)
			if redisErr != nil {
				logger.Error("redis parse url; rate limiter will fail open", "err", redisErr)
			} else {
				redisClient = redis.NewClient(redisOpts)
			}
		}

		var redisHealth interface{ Ping(context.Context) error }
		var cache middleware.ResponseCache
		var rateLimitRedis sorohandler.RedisClient
		if redisClient != nil {
			redisHealth = &redisPinger{client: redisClient}
			cache = &middleware.RedisCache{Client: redisClient}
			rateLimitRedis = &vercelRedisClient{client: redisClient}
		}

		h := &sorohandler.Handler{
			Store:       store.NewFullStore(pool),
			DB:          &dbPinger{pool: pool},
			Redis:       redisHealth,
			RedisClient: rateLimitRedis,
			Logger:      logger,

			Cache:              cache,
			CacheTTL:           30 * time.Second,
			SlackSigningSecret: os.Getenv("SLACK_SIGNING_SECRET"),
			SessionSigner:      middleware.NewSessionSigner(os.Getenv("IDENTITY_SIGNING_KEY"), 0),
			TrustedProxySecret: os.Getenv("TRUSTED_IDENTITY_PROXY_SECRET"),
			TrustedProxies:     splitCSV(os.Getenv("TRUSTED_PROXIES")),
			RateLimitPerMinute: positiveIntEnv("RATE_LIMIT_PER_MINUTE", 60),
			RateLimitWindow:    positiveDurationEnv("RATE_LIMIT_WINDOW", time.Minute),
		}
		maxBodyBytes := apiconfig.DefaultRequestMaxBodyBytes
		if n, err := apiconfig.MaxBodyBytesFromEnv(); err != nil {
			logger.Error("request body limit", "err", err)
		} else {
			maxBodyBytes = n
		}
		handler = router.New(h, maxBodyBytes)
	})

	if handler == nil {
		http.Error(w, `{"error":{"code":"INTERNAL","message":"server not initialized"}}`, http.StatusInternalServerError)
		return
	}
	handler.ServeHTTP(w, r)
}

type dbPinger struct{ pool *pgxpool.Pool }

func (p *dbPinger) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

type redisPinger struct{ client *redis.Client }

func (p *redisPinger) Ping(ctx context.Context) error { return p.client.Ping(ctx).Err() }

type vercelRedisClient struct{ client *redis.Client }

func (r *vercelRedisClient) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}
func (r *vercelRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return r.client.Expire(ctx, key, expiration).Result()
}
func (r *vercelRedisClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return r.client.Eval(ctx, script, keys, args...).Result()
}

func splitCSV(value string) []string {
	var entries []string
	for _, entry := range strings.Split(value, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

func positiveIntEnv(key string, fallback int) int {
	n, err := strconv.Atoi(os.Getenv(key))
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func positiveDurationEnv(key string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(os.Getenv(key))
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
