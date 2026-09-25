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
// INDEXER_LEDGER_WINDOW (120960 ledgers ≈ 7 days), INDEXER_MAX_DURATION (270s),
// VERIFY_BUILD_COMMAND (stellar contract build), VERIFY_TIMEOUT (10m),
// VERIFY_WORKSPACE_DIR (OS temp directory).
// REQUEST_MAX_BODY_BYTES (1048576 bytes = 1 MiB).
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

// DefaultRequestMaxBodyBytes is the default largest request body the API will
// read: 1 MiB. It keeps an unbounded body read from exhausting process memory.
const DefaultRequestMaxBodyBytes int64 = 1 << 20

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
	// VerifyBuildCommand is the deterministic build invocation used for
	// contract source verification (issue #263), parsed from a space-separated
	// string. It is executed directly, never through a shell.
	VerifyBuildCommand []string
	// VerifyTimeout bounds a single verification build.
	VerifyTimeout time.Duration
	// VerifyWorkspaceDir is the parent directory for verification workspaces.
	// Empty means the OS temporary directory.
	VerifyWorkspaceDir string
	// InitialAdminGitHubID, when set, seeds a user with the admin role on
	// startup. The user is keyed by this value as both its ID and GitHub ID so
	// requests authenticated with X-User-ID or X-GitHub-ID resolve to it.
	InitialAdminGitHubID string
	// CacheTTL is the lifetime of cached GET responses (API_CACHE_TTL,
	// default 30s). Zero disables the response cache.
	CacheTTL time.Duration
	// SlackSigningSecret verifies Slack slash command requests
	// (SLACK_SIGNING_SECRET). Empty disables the Slack command endpoint.
	SlackSigningSecret string
}

// Load reads configuration from environment variables and returns an error
// that lists every missing required variable so the process fails fast with a
// single, actionable message.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		DirectDatabaseURL:    os.Getenv("DIRECT_DATABASE_URL"),
		RedisURL:             os.Getenv("REDIS_URL"),
		SorobanRPCURL:        getEnvDefault("SOROBAN_RPC_URL", "https://soroban-testnet.stellar.org"),
		StellarNetwork:       getEnvDefault("STELLAR_NETWORK", "testnet"),
		Port:                 getEnvDefault("PORT", "8080"),
		LogLevel:             getEnvDefault("LOG_LEVEL", "info"),
		InitialAdminGitHubID: os.Getenv("INITIAL_ADMIN_GITHUB_ID"),
		SlackSigningSecret:   os.Getenv("SLACK_SIGNING_SECRET"),
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

	cfg.VerifyBuildCommand = strings.Fields(getEnvDefault("VERIFY_BUILD_COMMAND", "stellar contract build"))
	if len(cfg.VerifyBuildCommand) == 0 {
		return nil, fmt.Errorf("VERIFY_BUILD_COMMAND: must contain at least one argument")
	}

	verifyTimeoutStr := getEnvDefault("VERIFY_TIMEOUT", "10m")
	verifyTimeout, err := time.ParseDuration(verifyTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("VERIFY_TIMEOUT: invalid duration %q: %w", verifyTimeoutStr, err)
	}
	cfg.VerifyTimeout = verifyTimeout

	cfg.VerifyWorkspaceDir = os.Getenv("VERIFY_WORKSPACE_DIR")

	cacheTTLStr := getEnvDefault("API_CACHE_TTL", "30s")
	cacheTTL, err := time.ParseDuration(cacheTTLStr)
	if err != nil || cacheTTL < 0 {
		return nil, fmt.Errorf("API_CACHE_TTL: invalid duration %q", cacheTTLStr)
	}
	cfg.CacheTTL = cacheTTL

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

// MaxBodyBytesFromEnv resolves the request body size limit from
// REQUEST_MAX_BODY_BYTES, falling back to DefaultRequestMaxBodyBytes when the
// variable is unset. The value must be a positive integer: a non-numeric or
// non-positive value is rejected so a bad deployment fails fast instead of
// silently disabling the guard.
func MaxBodyBytesFromEnv() (int64, error) {
	raw := getEnvDefault("REQUEST_MAX_BODY_BYTES", strconv.FormatInt(DefaultRequestMaxBodyBytes, 10))
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("REQUEST_MAX_BODY_BYTES: invalid size %q: must be a positive integer", raw)
	}
	return n, nil
}

// getEnvDefault returns the value of the environment variable named by the key.
// If the variable is not present, it returns the provided default value.
func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
