package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

func main() {
	mode := flag.String("mode", "once", "Run mode: once or continuous")
	maxDuration := flag.Duration("max-duration", 270*time.Second, "Maximum duration for a single pass (once mode)")
	pollInterval := flag.Duration("poll-interval", 5*time.Minute, "Sleep between passes (continuous mode)")
	ledgerWindow := flag.Uint("ledger-window", 120960, "Ledger window per getEvents call")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := poller.Config{
		LedgerWindow:         uint32(*ledgerWindow),
		PollInterval:         *pollInterval,
		MaxDuration:          *maxDuration,
		AnomalyEnabled:       envBool("INDEXER_ANOMALY_ENABLED", false),
		AnomalyLookbackHours: envInt("INDEXER_ANOMALY_LOOKBACK_HOURS", 168),
		AnomalySigma:         envFloat("INDEXER_ANOMALY_SIGMA", 3),
		AnomalyMinHistory:    envInt("INDEXER_ANOMALY_MIN_HISTORY", 12),
	}

	// Wire up real dependencies.
	// In production, substitute the real RPC client, store, and Redis client here.
	// See apps/api/internal/soroban and apps/api/internal/store for implementations.
	clients := make(map[string]poller.RPCClient)
	for _, network := range []string{"testnet", "mainnet", "futurenet"} {
		url := os.Getenv("SOROBAN_RPC_URL_" + strings.ToUpper(network))
		if url == "" {
			continue
		}
		clients[network] = &stubRPC{endpoint: url}
	}
	if len(clients) == 0 {
		clients[""] = &stubRPC{}
	}
	store := &stubStore{}
	redis := &stubRedis{}

	p := poller.NewWithRPCClients(clients, store, redis, cfg, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("sorolens/indexer starting", "mode", *mode)
	if err := p.Run(ctx, *mode); err != nil {
		log.Error("indexer error", "err", err)
		os.Exit(1)
	}
	log.Info("sorolens/indexer done")
}

// ---- stub adapters (replaced in a future session when apps/api is wired) --

type stubRPC struct {
	endpoint string
}

func (s *stubRPC) GetLatestLedger(ctx context.Context) (*poller.LatestLedger, error) {
	if s.endpoint == "" {
		return nil, fmt.Errorf("stub: RPC not wired; set up apps/api soroban.Client")
	}
	return &poller.LatestLedger{Sequence: 1, ProtocolVersion: 22}, nil
}
func (s *stubRPC) GetEvents(ctx context.Context, start, end uint32, filters []poller.EventFilter) (*poller.GetEventsResult, error) {
	if s.endpoint == "" {
		return nil, fmt.Errorf("stub: RPC not wired")
	}
	return &poller.GetEventsResult{}, nil
}
func (s *stubRPC) GetTransaction(ctx context.Context, hash string) (*poller.TransactionResult, error) {
	if s.endpoint == "" {
		return nil, fmt.Errorf("stub: RPC not wired")
	}
	return &poller.TransactionResult{}, nil
}
func (s *stubRPC) GetLedgerEntries(ctx context.Context, keys []string) (*poller.GetLedgerEntriesResult, error) {
	if s.endpoint == "" {
		return nil, fmt.Errorf("stub: RPC not wired")
	}
	return &poller.GetLedgerEntriesResult{}, nil
}

type stubStore struct{}

func (s *stubStore) ListContracts(ctx context.Context, cursor string, limit int) ([]poller.Contract, string, error) {
	return nil, "", nil
}
func (s *stubStore) BatchInsertEvents(ctx context.Context, events []poller.Event) error {
	return nil
}
func (s *stubStore) BatchInsertInvocations(ctx context.Context, invocations []poller.Invocation) error {
	return nil
}
func (s *stubStore) GetSyncState(ctx context.Context, contractID string) (poller.SyncState, error) {
	return poller.SyncState{ContractID: contractID}, nil
}
func (s *stubStore) UpsertSyncState(ctx context.Context, state poller.SyncState) error {
	return nil
}
func (s *stubStore) CreateNextMonthPartition(_ context.Context) error { return nil }
func (s *stubStore) CreateMonthlyPartitionIfNotExists(_ context.Context, _ int, _ int) error {
	return nil
}
func (s *stubStore) RecentHourlyActivity(ctx context.Context, contractID string, hours int) ([]poller.HourlyActivity, error) {
	return nil, nil
}
func (s *stubStore) InsertAlert(ctx context.Context, a poller.Alert) error { return nil }
func (s *stubStore) InsertContractUpgrade(_ context.Context, _ poller.ContractUpgrade) error {
	return nil
}
func (s *stubStore) UpdateContractWasmHash(_ context.Context, _ string, _ string) error {
	return nil
}
func (s *stubStore) ContractHealthInputs(_ context.Context, _ string) (poller.HealthInputs, error) {
	return poller.HealthInputs{}, nil
}
func (s *stubStore) UpsertContractHealthScore(_ context.Context, _ poller.ContractHealthScore) error {
	return nil
}

type stubRedis struct{}

func (r *stubRedis) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return true, nil
}
func (r *stubRedis) Del(ctx context.Context, key string) error { return nil }

// envBool reads a boolean env var with a default.
func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// envInt reads an integer env var with a default.
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// envFloat reads a float env var with a default.
func envFloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}
