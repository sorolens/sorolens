package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/coordinator"
	"github.com/sorolens/sorolens/services/indexer/internal/metrics"
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
	metricsAddr := flag.String("metrics-addr", envString("INDEXER_METRICS_ADDR", ":9100"), "Address for the Prometheus /metrics HTTP server (empty disables it)")
	// Sharded topology (issue #272). The default role preserves the original
	// single-process behaviour: index every contract in this process.
	role := flag.String("role", envString("INDEXER_ROLE", "all"), "Role: all, coordinator, or worker")
	shardCount := flag.Int("shard-count", envInt("INDEXER_SHARD_COUNT", coordinator.DefaultShardCount), "Number of shards contracts are spread across")
	workerID := flag.String("worker-id", envString("INDEXER_WORKER_ID", ""), "Unique id for this process (required for coordinator and worker roles)")
	shardStoreKind := flag.String("shard-store", envString("INDEXER_SHARD_STORE", ""), "Shard store backend: memory (single node only)")
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

	// Prometheus metrics (issue #198): the indexer exposes per-network lag on
	// /metrics. The server is best-effort — a bind failure is logged but does
	// not stop indexing.
	recorder := metrics.New()
	p.SetMetrics(recorder)
	metricsSrv := startMetricsServer(*metricsAddr, recorder.Handler(), log)
	defer func() {
		if metricsSrv == nil {
			return
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			log.Warn("indexer metrics shutdown", "err", err)
		}
	}()

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

	switch *role {
	case "", "all":
		log.Info("sorolens/indexer starting", "mode", *mode, "role", "all")
		if err := p.Run(ctx, *mode); err != nil {
			log.Error("indexer error", "err", err)
			os.Exit(1)
		}

	case "coordinator", "worker":
		shardStore, err := buildShardStore(*shardStoreKind)
		if err != nil {
			log.Error("indexer: shard store unavailable", "role", *role, "err", err)
			os.Exit(1)
		}
		if *workerID == "" {
			log.Error("indexer: --worker-id is required for the coordinator and worker roles")
			os.Exit(1)
		}

		shardCfg := coordinator.Config{
			ShardCount:        *shardCount,
			WorkerID:          *workerID,
			HeartbeatInterval: envDuration("INDEXER_HEARTBEAT_INTERVAL", coordinator.DefaultHeartbeatInterval),
			StaleAfter:        envDuration("INDEXER_WORKER_STALE_AFTER", coordinator.DefaultStaleAfter),
			RebalanceInterval: envDuration("INDEXER_REBALANCE_INTERVAL", coordinator.DefaultRebalanceInterval),
		}

		if *role == "coordinator" {
			log.Info("sorolens/indexer starting", "role", "coordinator", "worker_id", *workerID, "shards", *shardCount)
			if err := coordinator.New(shardStore, log, shardCfg).Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("indexer coordinator error", "err", err)
				os.Exit(1)
			}
		} else {
			// The poller is wrapped so it only ever sees the contracts this
			// worker's shards cover; every other store call passes through.
			scoped := newScopedStore(st)
			wp := poller.NewWithRPCClients(clients, scoped, redis, cfg, log)
			wp.SetMetrics(recorder)
			run := func(passCtx context.Context, contractIDs []string) error {
				scoped.SetScope(contractIDs)
				return wp.Run(passCtx, "once")
			}
			log.Info("sorolens/indexer starting", "role", "worker", "worker_id", *workerID, "shards", *shardCount)
			if err := coordinator.NewWorker(shardStore, log, shardCfg).Run(ctx, run); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("indexer worker error", "err", err)
				os.Exit(1)
			}
		}

	default:
		log.Error("indexer: unknown --role", "role", *role, "want", "all, coordinator, or worker")
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
func (s *stubRPC) GetContractWasmHash(_ context.Context, _ string) (string, error) {
	return "", nil
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
func (s *stubStore) RecordContractVersion(_ context.Context, _ poller.ContractVersion) error {
	return nil
}
func (s *stubStore) GetLatestContractVersion(_ context.Context, _ string) (poller.ContractVersion, error) {
	return poller.ContractVersion{}, poller.ErrVersionNotFound
}

type stubRedis struct{}

func (r *stubRedis) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return true, nil
}
func (r *stubRedis) Del(ctx context.Context, key string) error { return nil }

// startMetricsServer serves the Prometheus /metrics endpoint on addr and
// returns the server so the caller can shut it down. It returns nil when addr
// is empty, which disables the endpoint. Startup errors are logged rather than
// fatal so a port clash never takes the indexer down.
func startMetricsServer(addr string, h http.Handler, log *slog.Logger) *http.Server {
	if addr == "" {
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", h)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("indexer metrics listening", "addr", addr, "path", "/metrics")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("indexer metrics server", "addr", addr, "err", err)
		}
	}()
	return srv
}

// envString reads a string env var with a default.
func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

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

// envDuration reads a duration env var with a default. Values use Go duration
// syntax, e.g. "30s" or "2m".
func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// buildShardStore resolves the store the coordinator and workers share.
//
// Only the in-memory backend is wired today. It keeps the shard table inside
// one process, which is enough to exercise the topology on a single node but is
// NOT safe for replicas: each process would hold its own table and believe it
// owned every shard. Real horizontal scaling needs a coordinator.Store backed
// by Postgres; the schema it must write is
// apps/api/internal/db/migrations/000011_indexer_shards.up.sql. It fails fast
// rather than silently degrading to per-process ownership.
func buildShardStore(kind string) (coordinator.Store, error) {
	switch kind {
	case "memory":
		return coordinator.NewMemStore(), nil
	default:
		return nil, fmt.Errorf(
			"set --shard-store=memory for a single-node run; multi-replica sharding requires a shared coordinator.Store (see docs/indexer-sharding.md), got %q",
			kind,
		)
	}
}
