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
// SENTRY_ENVIRONMENT (production), REQUEST_MAX_BODY_BYTES (1048576 bytes = 1 MiB).
// COLD_STORAGE_THRESHOLD_DAYS (90), COLD_STORAGE_REGION (us-east-1).
// COLD_STORAGE_BUCKET is optional: leaving it empty disables the cold tier.
// SENTRY_ENVIRONMENT (production), REQUEST_MAX_BODY_BYTES (1048576 bytes = 1 MiB),
// API_CACHE_TTL (30s), API_REQUEST_TIMEOUT (30s), API_STREAM_TIMEOUT (5m).
// Optional with no default: SENTRY_DSN. Error reporting is disabled entirely
// when it is unset.
//
// METRICS_PORT is optional with no default: when set, GET /metrics is also
// served on that port (a dedicated admin listener); when empty the endpoint is
// only available on PORT.
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
	// MetricsPort, when non-empty, starts a second HTTP listener that serves
	// only GET /metrics. It lets operators scrape metrics on a private
	// interface instead of exposing the endpoint on the public API port.
	MetricsPort string
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
	// ColdStorageBucket is the S3-compatible bucket holding archived events.
	// Empty disables the cold-storage tier entirely: the archive job refuses
	// to run and the API never falls back to object storage.
	ColdStorageBucket string
	// ColdStorageThresholdDays is how old an event must be before the nightly
	// archive job moves it out of Postgres.
	ColdStorageThresholdDays int
	// ColdStorageEndpoint overrides the S3 endpoint for MinIO, Backblaze B2,
	// and other S3-compatible services. Empty uses the AWS endpoint.
	ColdStorageEndpoint string
	// ColdStorageRegion is the signing region for the archive bucket.
	ColdStorageRegion string
	// ColdStorageAccessKeyID and ColdStorageSecretAccessKey are optional
	// explicit credentials. Empty falls back to the default AWS credential
	// chain.
	ColdStorageAccessKeyID     string
	ColdStorageSecretAccessKey string
	// InitialAdminGitHubID, when set, seeds a user with the admin role on
	// startup. The user is keyed by this value as both its ID and GitHub ID so
	// requests authenticated with X-User-ID or X-GitHub-ID resolve to it.
	InitialAdminGitHubID string
	// SentryDSN is the Sentry project DSN. Error reporting is disabled
	// entirely when this is empty.
	SentryDSN string
	// SentryEnvironment tags reported events (e.g. production, staging).
	SentryEnvironment string
	// CacheTTL is the lifetime of cached GET responses (API_CACHE_TTL,
	// default 30s). Zero disables the response cache.
	CacheTTL time.Duration
	// RequestTimeout caps handling of every /api/v1 request except the SSE
	// stream; past it the client gets a 503 (API_REQUEST_TIMEOUT, default 30s).
	RequestTimeout time.Duration
	// StreamTimeout bounds one SSE connection on /api/v1/stream/events
	// (API_STREAM_TIMEOUT, default 5m).
	StreamTimeout time.Duration
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
		MetricsPort:          os.Getenv("METRICS_PORT"),
		InitialAdminGitHubID: os.Getenv("INITIAL_ADMIN_GITHUB_ID"),
		SentryDSN:            os.Getenv("SENTRY_DSN"),
		SentryEnvironment:    getEnvDefault("SENTRY_ENVIRONMENT", "production"),
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

	// Cold storage (issue #146). Optional: an empty bucket disables the tier.
	cfg.ColdStorageBucket = os.Getenv("COLD_STORAGE_BUCKET")
	cfg.ColdStorageEndpoint = os.Getenv("COLD_STORAGE_ENDPOINT")
	cfg.ColdStorageRegion = getEnvDefault("COLD_STORAGE_REGION", "us-east-1")
	cfg.ColdStorageAccessKeyID = os.Getenv("COLD_STORAGE_ACCESS_KEY_ID")
	cfg.ColdStorageSecretAccessKey = os.Getenv("COLD_STORAGE_SECRET_ACCESS_KEY")

	thresholdStr := getEnvDefault("COLD_STORAGE_THRESHOLD_DAYS", "90")
	threshold, err := strconv.Atoi(thresholdStr)
	if err != nil || threshold < 1 {
		return nil, fmt.Errorf("COLD_STORAGE_THRESHOLD_DAYS: invalid integer %q: must be a positive number of days", thresholdStr)
	}
	cfg.ColdStorageThresholdDays = threshold

	cacheTTLStr := getEnvDefault("API_CACHE_TTL", "30s")
	cacheTTL, err := time.ParseDuration(cacheTTLStr)
	if err != nil || cacheTTL < 0 {
		return nil, fmt.Errorf("API_CACHE_TTL: invalid duration %q", cacheTTLStr)
	}
	cfg.CacheTTL = cacheTTL

	reqTimeoutStr := getEnvDefault("API_REQUEST_TIMEOUT", "30s")
	reqTimeout, err := time.ParseDuration(reqTimeoutStr)
	if err != nil || reqTimeout <= 0 {
		return nil, fmt.Errorf("API_REQUEST_TIMEOUT: invalid duration %q (must be > 0)", reqTimeoutStr)
	}
	cfg.RequestTimeout = reqTimeout

	streamTimeoutStr := getEnvDefault("API_STREAM_TIMEOUT", "5m")
	streamTimeout, err := time.ParseDuration(streamTimeoutStr)
	if err != nil || streamTimeout <= 0 {
		return nil, fmt.Errorf("API_STREAM_TIMEOUT: invalid duration %q (must be > 0)", streamTimeoutStr)
	}
	cfg.StreamTimeout = streamTimeout

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
