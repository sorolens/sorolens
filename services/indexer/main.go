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
	"github.com/sorolens/sorolens/services/indexer/internal/watchdog"
)

func main() {
	tp, _ := poller.InitTracer()
	if tp != nil {
		defer tp.Shutdown(context.Background())
	}

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
	st := &stubStore{}
	redis := &stubRedis{}

	// Wire watchdog interceptor. The stub store implements no watchdog
	// surface yet, so wdStore stays nil and the interceptor no-ops; the
	// any-assertion lights up once the real FullStore is wired here.
	var storeAny any = st
	var wdStore watchdogStore
	if ws, ok := storeAny.(watchdogStore); ok {
		wdStore = ws
	}
	watchdogEnabled := os.Getenv("WATCHDOG_ENABLED") == "true"
	watchdogContractID := os.Getenv("WATCHDOG_CONTRACT_ID")

	for k, c := range clients {
		clients[k] = &watchdogInterceptor{
			RPCClient: c,
			store:     wdStore,
			enabled:   watchdogEnabled,
			contract:  watchdogContractID,
			log:       log,
		}
	}

	p := poller.NewWithRPCClients(clients, st, redis, cfg, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start nightly performance job
	go func() {
		type perfStore interface {
			ComputeAndStoreBaselines(ctx context.Context, snapshotDate time.Time) error
			CheckAndEmitRegressions(ctx context.Context, snapshotDate time.Time) (int, error)
		}

		if ps, ok := storeAny.(perfStore); ok {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(24 * time.Hour):
					now := time.Now()
					if err := ps.ComputeAndStoreBaselines(ctx, now); err != nil {
						log.Error("failed to compute baselines", "err", err)
					} else {
						alerts, err := ps.CheckAndEmitRegressions(ctx, now)
						if err != nil {
							log.Error("failed to check regressions", "err", err)
						} else if alerts > 0 {
							log.Info("emitted performance regression alerts", "count", alerts)
						}
					}
				}
			}
		}
	}()

	log.Info("sorolens/indexer starting", "mode", *mode)
	if err := p.Run(ctx, *mode); err != nil {
		log.Error("indexer error", "err", err)
		os.Exit(1)
	}
	log.Info("sorolens/indexer done")
}

// decodeWatchdogValue converts an event's base64 XDR value into the decoded
// map form RawEvent expects. Nested ScVal types (maps, vecs) are not decoded
// yet by apps/api/internal/soroban, so the map carries the type/human summary;
// classifier field lookups stay dormant until nested decoding lands.
func decodeWatchdogValue(valueXDR string) map[string]any {
	out := map[string]any{}
	if valueXDR == "" {
		return out
	}
	sc, err := decodeScVal(valueXDR)
	if err != nil {
		return out
	}
	out["type"] = sc.Type
	out["human"] = sc.Human
	if m, ok := sc.Value.(map[string]any); ok {
		return m
	}
	return out
}

// watchdogInterceptor intercepts getEvents and routes watchdog events to the classifier
type watchdogInterceptor struct {
	poller.RPCClient
	store    watchdogStore
	enabled  bool
	contract string
	log      *slog.Logger
}

func (w *watchdogInterceptor) GetEvents(ctx context.Context, start, end uint32, filters []poller.EventFilter) (*poller.GetEventsResult, error) {
	res, err := w.RPCClient.GetEvents(ctx, start, end, filters)
	if err != nil || res == nil || !w.enabled || w.contract == "" || w.store == nil {
		return res, err
	}

	for _, e := range res.Events {
		if e.ContractID == w.contract {
			t, _ := time.Parse(time.RFC3339, e.LedgerClosedAt)
			raw := watchdog.RawEvent{
				ContractID:     e.ContractID,
				Ledger:         int64(e.Ledger),
				LedgerClosedAt: t,
				TxHash:         e.TxHash,
				Topics:         e.Topic,
				Value:          decodeWatchdogValue(e.Value),
			}

			switch watchdog.ClassifyKind(raw) {
			case watchdog.KindContractRegistered:
				if reg, err := watchdog.ProjectRegistration(raw); err == nil {
					_ = w.store.UpsertMonitoredContract(ctx, wdMonitoredContract{
						ContractID:    reg.ContractID,
						Name:          reg.Name,
						Owner:         reg.Owner,
						CheckInterval: reg.CheckInterval,
						RegisteredAt:  reg.Timestamp,
					})
					w.log.Info("watchdog: registered contract", "target", reg.ContractID)
				}
			case watchdog.KindContractDeregistered:
				if dereg, err := watchdog.ProjectDeregistration(raw); err == nil {
					_ = w.store.DeleteMonitoredContract(ctx, dereg.ContractID)
					w.log.Info("watchdog: deregistered contract", "target", dereg.ContractID)
				}
			case watchdog.KindHealthCheck:
				if h, err := watchdog.ProjectHealth(raw); err == nil {
					_ = w.store.InsertHealthCheck(ctx, wdHealthCheck{
						ContractID: h.ContractID,
						Status:     h.Status,
						Metadata:   h.Metadata,
						Ledger:     h.Ledger,
						TxHash:     h.TxHash,
						Timestamp:  h.Timestamp,
					})
					w.log.Info("watchdog: health check", "target", h.ContractID, "status", h.Status)
				}
			case watchdog.KindContractAlert:
				if a, err := watchdog.ProjectAlert(raw); err == nil {
					_ = w.store.InsertContractAlert(ctx, wdContractAlert{
						ContractID: a.ContractID,
						Severity:   a.Severity,
						Message:    a.Message,
						Ledger:     a.Ledger,
						TxHash:     a.TxHash,
						Timestamp:  a.Timestamp,
					})
					w.log.Info("watchdog: alert", "target", a.ContractID, "severity", a.Severity)
				}
			}
		}
	}
	return res, nil
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
func (s *stubStore) GetIndexerCursor(_ context.Context, _ string) (uint32, error) { return 0, nil }
func (s *stubStore) SetIndexerCursor(_ context.Context, _ string, _ uint32) error  { return nil }
func (s *stubStore) BatchInsertWithCursor(_ context.Context, _ string, _ uint32, _ []poller.Event, _ []poller.Invocation, _ poller.SyncState) error {
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
