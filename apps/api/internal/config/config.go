// Package config loads runtime configuration for the API from environment
// variables. It provides a Config struct with database, Redis, Soroban RPC,
// and indexer settings, and a Load function that validates required variables
// and applies defaults.
//
// # Environment variables
//
// Required: DATABASE_URL, REDIS_URL.
// Optional with defaults: SOROBAN_RPC_URL (testnet), STELLAR_NETWORK (testnet),
// PORT (8080), LOG_LEVEL (info), INDEXER_POLL_INTERVAL (5m),
// INDEXER_LEDGER_WINDOW (120960 ledgers ≈ 7 days), INDEXER_MAX_DURATION (270s).
//
// Load collects every missing required variable into a single error message
// so the process fails fast with actionable output.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the API.
type Config struct {
	// DatabaseURL is the pooled Postgres connection string used by the API.
	DatabaseURL string
	// DirectDatabaseURL is the non-pooled connection string used for migrations.
	DirectDatabaseURL string
	// RedisURL is the Redis connection string used for caching and advisory locks.
	RedisURL string
	// SorobanRPCURL is the Soroban RPC endpoint to query on cache misses.
	SorobanRPCURL string
	// StellarNetwork is the network identifier: testnet, mainnet, or futurenet.
	StellarNetwork string
	// Port is the HTTP port the API listens on.
	Port string
	// LogLevel controls log verbosity: debug, info, warn, or error.
	LogLevel string
	// IndexerPollInterval is how often the indexer polls for new events.
	IndexerPollInterval time.Duration
	// IndexerLedgerWindow is the number of past ledgers included in a backfill.
	IndexerLedgerWindow int
	// IndexerMaxDuration is the wall-clock budget for a single indexer run.
	IndexerMaxDuration time.Duration
}

// Load reads configuration from environment variables and returns an error
// that lists every missing required variable so the process fails fast with a
// single, actionable message.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		DirectDatabaseURL: os.Getenv("DIRECT_DATABASE_URL"),
		RedisURL:          os.Getenv("REDIS_URL"),
		SorobanRPCURL:     getEnvDefault("SOROBAN_RPC_URL", "https://soroban-testnet.stellar.org"),
		StellarNetwork:    getEnvDefault("STELLAR_NETWORK", "testnet"),
		Port:              getEnvDefault("PORT", "8080"),
		LogLevel:          getEnvDefault("LOG_LEVEL", "info"),
	}

	pollStr := getEnvDefault("INDEXER_POLL_INTERVAL", "5m")
	poll, err := time.ParseDuration(pollStr)
	if err != nil {
		return nil, fmt.Errorf("INDEXER_POLL_INTERVAL: invalid duration %q: %w", pollStr, err)
	}
	cfg.IndexerPollInterval = poll

	windowStr := getEnvDefault("INDEXER_LEDGER_WINDOW", "120960")
	window, err := strconv.Atoi(windowStr)
	if err != nil {
		return nil, fmt.Errorf("INDEXER_LEDGER_WINDOW: invalid integer %q: %w", windowStr, err)
	}
	cfg.IndexerLedgerWindow = window

	maxDurStr := getEnvDefault("INDEXER_MAX_DURATION", "270s")
	maxDur, err := time.ParseDuration(maxDurStr)
	if err != nil {
		return nil, fmt.Errorf("INDEXER_MAX_DURATION: invalid duration %q: %w", maxDurStr, err)
	}
	cfg.IndexerMaxDuration = maxDur

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.RedisURL == "" {
		missing = append(missing, "REDIS_URL")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

// getEnvDefault returns the value of the environment variable named by the key.
// If the variable is not present, it returns the provided default value.
func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
