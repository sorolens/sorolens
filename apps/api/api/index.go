// Package handler is the Vercel Go serverless entrypoint for the Sorolens API.
// Vercel serverless functions do not support long-lived connections; the
// /stream endpoint therefore uses polling rather than SSE.
package handler

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	sorohandler "github.com/sorolens/sorolens/apps/api/internal/handler"
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

		pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
		if err != nil {
			logger.Error("postgres connect", "err", err)
			return
		}

		redisOpts, err := redis.ParseURL(os.Getenv("REDIS_URL"))
		if err != nil {
			logger.Error("redis parse url", "err", err)
			return
		}
		redisClient := redis.NewClient(redisOpts)

		h := &sorohandler.Handler{
			Store:  store.NewFullStore(pool),
			DB:     &dbPinger{pool: pool},
			Redis:  &redisPinger{client: redisClient},
			Logger: logger,
		}
		handler = router.New(h)
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
