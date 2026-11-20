// Command retention runs one pass of the database events retention and partition maintenance job.
//
// It archives events older than EVENTS_RETENTION_DAYS from the events table into
// events_archive, ensuring events outside the retention window are preserved and
// reachable while preventing unbounded table growth. It also pre-creates future
// monthly partitions so inserts never fail.
//
// Usage:
//
//	DATABASE_URL=postgres://... EVENTS_RETENTION_DAYS=90 go run ./cmd/retention
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

	retentionDays := 90
	if v := os.Getenv("EVENTS_RETENTION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retentionDays = n
		} else {
			logger.Warn("invalid EVENTS_RETENTION_DAYS, using default", "raw", v, "default", retentionDays)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	st := store.NewFullStore(pool)

	// 1. Maintain future partitions
	logger.Info("running partition maintenance")
	if err := st.MaintainPartitions(ctx, 6); err != nil {
		logger.Error("partition maintenance failed", "err", err)
		os.Exit(1)
	}

	// 2. Archive old events
	logger.Info("running events retention archive", "retention_days", retentionDays)
	start := time.Now()
	archived, err := st.ArchiveOldEvents(ctx, retentionDays)
	if err != nil {
		logger.Error("events retention archive failed", "err", err)
		os.Exit(1)
	}

	logger.Info("retention run complete",
		"archived_rows", archived,
		"retention_days", retentionDays,
		"duration", time.Since(start).String(),
	)
}
