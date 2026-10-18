// Command coldarchive runs one pass of the cold-storage tier (issue #146).
//
// It exports every event older than COLD_STORAGE_THRESHOLD_DAYS from Postgres
// into Parquet objects in COLD_STORAGE_BUCKET, then deletes the exported rows.
// It is meant to be scheduled (a nightly cron / GitHub Actions workflow) and
// exits non-zero if the pass fails, so the scheduler surfaces the error.
//
// Usage:
//
//	COLD_STORAGE_BUCKET=sorolens-archive \
//	COLD_STORAGE_ENDPOINT=http://localhost:9000 \
//	COLD_STORAGE_ACCESS_KEY_ID=minioadmin \
//	COLD_STORAGE_SECRET_ACCESS_KEY=minioadmin \
//	DATABASE_URL=postgres://... REDIS_URL=redis://... \
//	go run ./cmd/coldarchive
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sorolens/sorolens/apps/api/internal/coldstorage"
	"github.com/sorolens/sorolens/apps/api/internal/config"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "err", err)
		os.Exit(1)
	}
	if cfg.ColdStorageBucket == "" {
		logger.Error("COLD_STORAGE_BUCKET is not set; the cold-storage tier is disabled")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	bucket, err := coldstorage.NewS3Store(ctx, coldstorage.S3Config{
		Bucket:          cfg.ColdStorageBucket,
		Region:          cfg.ColdStorageRegion,
		Endpoint:        cfg.ColdStorageEndpoint,
		AccessKeyID:     cfg.ColdStorageAccessKeyID,
		SecretAccessKey: cfg.ColdStorageSecretAccessKey,
	})
	if err != nil {
		logger.Error("cold storage client", "err", err)
		os.Exit(1)
	}

	archiver := coldstorage.NewArchiver(
		store.NewFullStore(pool),
		bucket,
		time.Duration(cfg.ColdStorageThresholdDays)*24*time.Hour,
		logger,
	)

	start := time.Now()
	stats, err := archiver.Run(ctx)
	if err != nil {
		logger.Error("archive run failed", "err", err)
		os.Exit(1)
	}

	logger.Info("archive run complete",
		"contracts", stats.Contracts,
		"events", stats.Events,
		"deleted", stats.Deleted,
		"objects", stats.Objects,
		"threshold_days", cfg.ColdStorageThresholdDays,
		"duration", time.Since(start).String(),
	)
}
