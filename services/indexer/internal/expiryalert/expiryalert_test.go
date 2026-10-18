package expiryalert

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

// The runner must stay plug-compatible with the poller's end-of-pass hook.
var _ poller.ExpiryAlerter = (*Runner)(nil)

// ---- mock Store (issue #510 acceptance criteria: "Tests use mock store ...")

// fakeStore is an in-memory stand-in for pgstore: contract paging, per-contract
// near-expiry entry sets, and the (tx_hash, contract_id) de-dup table the real
// store enforces with ux_contract_alerts_tx_contract. It records the arguments
// the runner passes so tests can assert the configured threshold reaches the
// query, and can fail any single call to exercise the best-effort paths.
//
// The contract list is kept sorted by ID, as the real keyset query returns it.
type fakeStore struct {
	contracts []poller.Contract
	entries   map[string][]poller.ExpiringStorageEntry
	// recorded mirrors contract_alerts' dedup keys. Seed it before a pass to
	// simulate an alert raised by an earlier run.
	recorded map[string]bool
	inserted []poller.Alert

	listErr    error
	entriesErr map[string]error
	existsErr  error
	insertErr  error

	listCalls     int
	scanCalls     int
	seenThreshold int64
	seenLimit     int
	existsCalls   int
}

func newFakeStore(contracts ...poller.Contract) *fakeStore {
	return &fakeStore{
		contracts: contracts,
		entries:   map[string][]poller.ExpiringStorageEntry{},
		recorded:  map[string]bool{},
	}
}

func (f *fakeStore) ListContracts(_ context.Context, cursor string, limit int) ([]poller.Contract, string, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	if limit <= 0 {
		limit = 50
	}
	out := make([]poller.Contract, 0, limit)
	for _, c := range f.contracts {
		if cursor != "" && c.ID <= cursor {
			continue
		}
		out = append(out, c)
		if len(out) == limit {
			break
		}
	}
	next := ""
	if len(out) == limit {
		last := out[len(out)-1].ID
		for _, c := range f.contracts {
			if c.ID > last {
				next = last
				break
			}
		}
	}
	return out, next, nil
}

func (f *fakeStore) ExpiringStorageEntries(_ context.Context, contractID string, thresholdLedgers int64, limit int) ([]poller.ExpiringStorageEntry, error) {
	f.scanCalls++
	f.seenThreshold = thresholdLedgers
	f.seenLimit = limit
	if err := f.entriesErr[contractID]; err != nil {
		return nil, err
	}
	entries := f.entries[contractID]
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func (f *fakeStore) AlertExists(_ context.Context, txHash, contractID string) (bool, error) {
	f.existsCalls++
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.recorded[contractID+"|"+txHash], nil
}

func (f *fakeStore) InsertAlert(_ context.Context, a poller.Alert) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, a)
	f.recorded[a.ContractID+"|"+a.TxHash] = true
	return nil
}

// ---- mock Notifier ("... and mock notifier") -------------------------------

// fakeNotifier records every alert handed to the existing watchdog dispatch
// path, so tests can assert both the count and the ordering relative to the
// store insert.
type fakeNotifier struct {
	alerts []poller.Alert
}

func (n *fakeNotifier) Notify(_ context.Context, a poller.Alert) {
	n.alerts = append(n.alerts, a)
}

// ---- helpers --------------------------------------------------------------

var testNow = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

func newRunner(store Store, notify Notifier, cfg Config) *Runner {
	r := New(store, notify, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.now = func() time.Time { return testNow }
	return r
}

func active(id string) poller.Contract {
	return poller.Contract{ID: id, Status: "active", Network: "public"}
}

func entry(key string, durability string, liveUntil int64) poller.ExpiringStorageEntry {
	return poller.ExpiringStorageEntry{KeyXDR: key, Durability: durability, LiveUntilLedger: liveUntil}
}

// ---- default threshold ----------------------------------------------------

func TestDefaultThresholdIs5000Ledgers(t *testing.T) {
	if DefaultThresholdLedgers != 5_000 {
		t.Fatalf("DefaultThresholdLedgers = %d, want 5000 (issue #510)", DefaultThresholdLedgers)
	}
	cfg := Config{}.withDefaults()
	if cfg.ThresholdLedgers != DefaultThresholdLedgers {
		t.Fatalf("zero threshold = %d, want default %d", cfg.ThresholdLedgers, DefaultThresholdLedgers)
	}
	if cfg.MaxEntries != DefaultMaxEntries || cfg.ContractPageSize != defaultContractPageSize {
		t.Fatalf("withDefaults() = %+v, want bounded defaults", cfg)
	}
	// A negative threshold must not degrade into "everything is expiring".
	if got := (Config{ThresholdLedgers: -1}).withDefaults().ThresholdLedgers; got != DefaultThresholdLedgers {
		t.Fatalf("negative threshold = %d, want %d", got, DefaultThresholdLedgers)
	}
}

// ---- env configuration (acceptance criterion: "Threshold configurable via env")

func TestConfigFromEnvReadsThresholdAndMaxEntries(t *testing.T) {
	env := map[string]string{
		EnvThresholdLedgers: "250",
		EnvMaxEntries:       "10",
	}
	cfg := ConfigFromEnv(func(k string) string { return env[k] })
	if cfg.ThresholdLedgers != 250 {
		t.Fatalf("ThresholdLedgers = %d, want 250", cfg.ThresholdLedgers)
	}
	if cfg.MaxEntries != 10 {
		t.Fatalf("MaxEntries = %d, want 10", cfg.MaxEntries)
	}
}

func TestConfigFromEnvFallsBackToDefaults(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
	}{
		{"unset", nil},
		{"blank", map[string]string{EnvThresholdLedgers: "   ", EnvMaxEntries: ""}},
		{"non numeric", map[string]string{EnvThresholdLedgers: "soon", EnvMaxEntries: "many"}},
		{"non positive", map[string]string{EnvThresholdLedgers: "0", EnvMaxEntries: "-5"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(k string) string { return tc.env[k] }
			cfg := ConfigFromEnv(getenv)
			if cfg.ThresholdLedgers != DefaultThresholdLedgers {
				t.Fatalf("ThresholdLedgers = %d, want default %d", cfg.ThresholdLedgers, DefaultThresholdLedgers)
			}
			if cfg.MaxEntries != DefaultMaxEntries {
				t.Fatalf("MaxEntries = %d, want default %d", cfg.MaxEntries, DefaultMaxEntries)
			}
		})
	}
	// A nil lookup means "no environment consulted", never a crash.
	if got := ConfigFromEnv(nil).ThresholdLedgers; got != DefaultThresholdLedgers {
		t.Fatalf("ConfigFromEnv(nil) threshold = %d, want default", got)
	}
}

func TestConfigFromEnvThresholdReachesTheStoreQuery(t *testing.T) {
	store := newFakeStore(active("CA"))
	store.entries["CA"] = []poller.ExpiringStorageEntry{entry("k1", "persistent", 100)}
	notifier := &fakeNotifier{}

	env := map[string]string{EnvThresholdLedgers: "144", EnvMaxEntries: "7"}
	r := newRunner(store, notifier, ConfigFromEnv(func(k string) string { return env[k] }))
	if err := r.AlertExpiry(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.seenThreshold != 144 {
		t.Fatalf("store saw threshold %d, want 144 from env", store.seenThreshold)
	}
	if store.seenLimit != 7 {
		t.Fatalf("store saw limit %d, want 7 from env", store.seenLimit)
	}
}

func TestEnabledDefaultsToOnAndHonoursExplicitFalse(t *testing.T) {
	env := map[string]string{}
	getenv := func(k string) string { return env[k] }
	if !Enabled(getenv) {
		t.Fatal("expiry alerting must default to enabled")
	}
	for _, v := range []string{"false", "0", "FALSE", "False"} {
		env[EnvEnabled] = v
		if Enabled(getenv) {
			t.Fatalf("Enabled(%q) = true, want false", v)
		}
	}
	for _, v := range []string{"true", "1", "TRUE"} {
		env[EnvEnabled] = v
		if !Enabled(getenv) {
			t.Fatalf("Enabled(%q) = false, want true", v)
		}
	}
	// Unparseable values keep the feature on rather than silently disabling it.
	env[EnvEnabled] = "maybe"
	if !Enabled(getenv) {
		t.Fatal("unparseable value must fall back to enabled")
	}
	if !Enabled(nil) {
		t.Fatal("Enabled(nil getenv) must default to true")
	}
}

// ---- alert fires once when an entry crosses the threshold -----------------

func TestAlertFiresOnceWhenEntriesCrossThreshold(t *testing.T) {
	store := newFakeStore(active("CA"))
	store.entries["CA"] = []poller.ExpiringStorageEntry{
		entry("k-early", "persistent", 1_000_100),
		entry("k-late", "temporary", 1_000_400),
	}
	notifier := &fakeNotifier{}

	r := newRunner(store, notifier, Config{ThresholdLedgers: 5_000})
	if err := r.AlertExpiry(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 1 {
		t.Fatalf("inserted alerts = %d, want exactly one summary per contract", len(store.inserted))
	}
	if len(notifier.alerts) != 1 {
		t.Fatalf("notified alerts = %d, want 1", len(notifier.alerts))
	}
	a := store.inserted[0]
	if a.ContractID != "CA" || a.Severity != "Critical" {
		t.Fatalf("alert = %+v, want Critical summary for CA", a)
	}
	if a.TxHash == "" {
		t.Fatal("alert must carry a deterministic dedup key")
	}
	if a.Ledger != 1_000_100 {
		t.Fatalf("alert ledger = %d, want the earliest expiry 1000100", a.Ledger)
	}
	if a.Timestamp != testNow {
		t.Fatalf("alert timestamp = %v, want %v", a.Timestamp, testNow)
	}
	for _, want := range []string{"2 storage entries", "5000 ledgers", "1 persistent", "1 temporary", "1000100"} {
		if !strings.Contains(a.Message, want) {
			t.Fatalf("message %q missing %q", a.Message, want)
		}
	}
	// The configured threshold and the bounded scan size are handed to the store.
	if store.seenThreshold != 5_000 || store.seenLimit != DefaultMaxEntries {
		t.Fatalf("store saw threshold=%d limit=%d, want 5000/%d", store.seenThreshold, store.seenLimit, DefaultMaxEntries)
	}
}

func TestEmptyEntriesRaiseNoAlert(t *testing.T) {
	store := newFakeStore(active("CA"), active("CB"))
	notifier := &fakeNotifier{}
	if err := newRunner(store, notifier, Config{}).AlertExpiry(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 0 || len(notifier.alerts) != 0 {
		t.Fatalf("expected no alerts, got inserted=%d notified=%d", len(store.inserted), len(notifier.alerts))
	}
	if store.scanCalls != 2 {
		t.Fatalf("scan calls = %d, want one per active contract", store.scanCalls)
	}
}

func TestOnlyActiveAndBackfillingContractsAreScanned(t *testing.T) {
	store := newFakeStore(
		poller.Contract{ID: "CA", Status: "pending"},
		poller.Contract{ID: "CB", Status: "paused"},
		poller.Contract{ID: "CC", Status: "backfilling"},
	)
	store.entries["CC"] = []poller.ExpiringStorageEntry{entry("k", "instance", 42)}
	// A pending contract that somehow has near-expiry rows must never fire.
	store.entries["CA"] = []poller.ExpiringStorageEntry{entry("k", "instance", 42)}
	notifier := &fakeNotifier{}

	if err := newRunner(store, notifier, Config{}).AlertExpiry(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 1 || store.inserted[0].ContractID != "CC" {
		t.Fatalf("alerts = %+v, want only CC", store.inserted)
	}
}

// ---- no duplicate alerts until state changes (acceptance criterion) -------

func TestNoDuplicateAlertWhileStateUnchanged(t *testing.T) {
	store := newFakeStore(active("CA"))
	entries := []poller.ExpiringStorageEntry{entry("k1", "persistent", 100), entry("k2", "temporary", 200)}
	store.entries["CA"] = entries
	notifier := &fakeNotifier{}
	r := newRunner(store, notifier, Config{})

	for pass := 1; pass <= 3; pass++ {
		if err := r.AlertExpiry(context.Background()); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
	}
	if len(store.inserted) != 1 {
		t.Fatalf("inserted alerts = %d after 3 identical passes, want 1", len(store.inserted))
	}
	if len(notifier.alerts) != 1 {
		t.Fatalf("webhook dispatches = %d after 3 identical passes, want 1", len(notifier.alerts))
	}
	// The dedup probe runs every pass, so a repeat is skipped before any insert.
	if store.existsCalls != 3 {
		t.Fatalf("dedup probes = %d, want 3", store.existsCalls)
	}
}

func TestNewAlertWhenEntrySetChanges(t *testing.T) {
	store := newFakeStore(active("CA"))
	store.entries["CA"] = []poller.ExpiringStorageEntry{entry("k1", "persistent", 100)}
	notifier := &fakeNotifier{}
	r := newRunner(store, notifier, Config{})

	if err := r.AlertExpiry(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Another entry crosses the threshold: that is new state and must alert.
	store.entries["CA"] = append(store.entries["CA"], entry("k2", "persistent", 150))
	if err := r.AlertExpiry(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 2 || len(notifier.alerts) != 2 {
		t.Fatalf("alerts inserted=%d notified=%d, want 2/2 after a new entry crossed", len(store.inserted), len(notifier.alerts))
	}
	if store.inserted[0].TxHash == store.inserted[1].TxHash {
		t.Fatal("changed entry set must yield a new dedup key")
	}

	// Extending an entry (state change) also re-alerts; a reverted set does not
	// collide with the earlier key because the hash covers the TTL values.
	store.entries["CA"] = []poller.ExpiringStorageEntry{entry("k1", "persistent", 900)}
	if err := r.AlertExpiry(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 3 {
		t.Fatalf("alerts = %d, want a third after an entry was extended", len(store.inserted))
	}
}

// ---- one summary per contract, best-effort --------------------------------

func TestOneSummaryPerContract(t *testing.T) {
	store := newFakeStore(active("CA"), active("CB"))
	store.entries["CA"] = []poller.ExpiringStorageEntry{entry("a1", "persistent", 10), entry("a2", "temporary", 20)}
	store.entries["CB"] = []poller.ExpiringStorageEntry{entry("b1", "instance", 30)}
	notifier := &fakeNotifier{}

	if err := newRunner(store, notifier, Config{}).AlertExpiry(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 2 {
		t.Fatalf("alerts = %d, want one per contract", len(store.inserted))
	}
	for _, a := range store.inserted {
		if a.Severity != "Critical" {
			t.Fatalf("contract %s: severity = %q, want Critical so the existing notifier dispatches it", a.ContractID, a.Severity)
		}
		if !strings.Contains(a.Message, "storage entries expire within") {
			t.Fatalf("contract %s: message %q must be a near-expiry summary", a.ContractID, a.Message)
		}
	}
	byContract := map[string]string{}
	for _, a := range store.inserted {
		byContract[a.ContractID] = a.Message
	}
	if !strings.HasPrefix(byContract["CA"], "2 ") || !strings.Contains(byContract["CA"], "10") {
		t.Fatalf("CA summary = %q, want its own 2 entries and earliest ledger 10", byContract["CA"])
	}
	if !strings.HasPrefix(byContract["CB"], "1 ") || !strings.Contains(byContract["CB"], "30") {
		t.Fatalf("CB summary = %q, want its own 1 entry and earliest ledger 30", byContract["CB"])
	}
}

func TestContractScanErrorIsSkippedNotFatal(t *testing.T) {
	store := newFakeStore(active("CA"), active("CB"))
	store.entries["CB"] = []poller.ExpiringStorageEntry{entry("b1", "instance", 30)}
	store.entriesErr = map[string]error{"CA": errors.New("boom")}
	notifier := &fakeNotifier{}

	r := newRunner(store, notifier, Config{})
	if err := r.AlertExpiry(context.Background()); err != nil {
		t.Fatalf("a bad contract must not abort the pass: %v", err)
	}
	if len(store.inserted) != 1 || store.inserted[0].ContractID != "CB" {
		t.Fatalf("alerts = %+v, want only CB", store.inserted)
	}
}

func TestListContractsErrorAbortsPass(t *testing.T) {
	store := newFakeStore(active("CA"))
	store.listErr = errors.New("db down")
	err := newRunner(store, &fakeNotifier{}, Config{}).AlertExpiry(context.Background())
	if err == nil || !strings.Contains(err.Error(), "list contracts") {
		t.Fatalf("err = %v, want wrapped list-contracts failure", err)
	}
}

func TestInsertFailureSuppressesDispatch(t *testing.T) {
	store := newFakeStore(active("CA"))
	store.entries["CA"] = []poller.ExpiringStorageEntry{entry("k1", "persistent", 100)}
	store.insertErr = errors.New("insert failed")
	notifier := &fakeNotifier{}

	if err := newRunner(store, notifier, Config{}).AlertExpiry(context.Background()); err != nil {
		t.Fatalf("insert failure must be logged and skipped, not fatal: %v", err)
	}
	if len(notifier.alerts) != 0 {
		t.Fatal("nothing may be dispatched before the dedup row is durable")
	}
}

func TestDedupProbeErrorSkipsContract(t *testing.T) {
	store := newFakeStore(active("CA"), active("CB"))
	store.entries["CA"] = []poller.ExpiringStorageEntry{entry("k1", "persistent", 1)}
	store.entries["CB"] = []poller.ExpiringStorageEntry{entry("k2", "persistent", 2)}
	store.existsErr = errors.New("probe failed")

	r := newRunner(store, &fakeNotifier{}, Config{})
	if err := r.AlertExpiry(context.Background()); err != nil {
		t.Fatalf("per-contract probe failure must be skipped: %v", err)
	}
	if len(store.inserted) != 0 {
		t.Fatalf("alerts = %d, want 0 when the dedup probe fails", len(store.inserted))
	}
}

// ---- pagination and cancellation ------------------------------------------

func TestWalksEveryContractPage(t *testing.T) {
	store := newFakeStore(active("CA"), active("CB"), active("CC"))
	for _, id := range []string{"CA", "CB", "CC"} {
		store.entries[id] = []poller.ExpiringStorageEntry{entry("k-"+id, "persistent", 5)}
	}
	notifier := &fakeNotifier{}

	r := newRunner(store, notifier, Config{ContractPageSize: 1})
	if err := r.AlertExpiry(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 3 {
		t.Fatalf("alerts = %d, want one per contract across pages", len(store.inserted))
	}
	if store.listCalls < 3 {
		t.Fatalf("list calls = %d, want the cursor to advance page by page", store.listCalls)
	}
}

func TestPassStopsOnCancelledContext(t *testing.T) {
	store := newFakeStore(active("CA"), active("CB"))
	store.entries["CA"] = []poller.ExpiringStorageEntry{entry("k1", "persistent", 5)}
	store.entries["CB"] = []poller.ExpiringStorageEntry{entry("k2", "persistent", 5)}
	notifier := &fakeNotifier{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := newRunner(store, notifier, Config{}).AlertExpiry(ctx); err != nil {
		t.Fatalf("cancellation is a clean stop, not an error: %v", err)
	}
	if len(notifier.alerts) != 0 {
		t.Fatalf("dispatches = %d, want 0 after cancellation", len(notifier.alerts))
	}
}

// ---- dedup key -----------------------------------------------------------

func TestDedupKeyIsDeterministicAndStateSensitive(t *testing.T) {
	set := []poller.ExpiringStorageEntry{entry("k1", "persistent", 100), entry("k2", "temporary", 200)}
	key := DedupKey("CA", set)
	if key != DedupKey("CA", set) {
		t.Fatal("identical state must collapse to one dedup key")
	}
	if !strings.HasPrefix(key, "expiry:CA:") {
		t.Fatalf("key %q must be namespaced like the anomaly job's keys", key)
	}
	if key == DedupKey("CB", set) {
		t.Fatal("the key must be scoped to its contract")
	}
	changed := []poller.ExpiringStorageEntry{entry("k1", "persistent", 101), entry("k2", "temporary", 200)}
	if key == DedupKey("CA", changed) {
		t.Fatal("an extended entry must yield a new key")
	}
	if key == DedupKey("CA", set[:1]) {
		t.Fatal("a shrunk set must yield a new key")
	}
	// Order matters: the store returns entries ordered by live_until_ledger.
	reversed := []poller.ExpiringStorageEntry{set[1], set[0]}
	if key == DedupKey("CA", reversed) {
		t.Fatal("a different entry order must yield a different key")
	}
	if len(DedupKey("CA", nil)) != len(key) {
		t.Fatal("dedup keys must be fixed width for the tx_hash column")
	}
}

func TestSummaryReportsTruncationHonestly(t *testing.T) {
	store := newFakeStore(active("CA"))
	var many []poller.ExpiringStorageEntry
	for i := 0; i < 3; i++ {
		many = append(many, entry(string(rune('a'+i)), "persistent", int64(100+i)))
	}
	store.entries["CA"] = many
	notifier := &fakeNotifier{}

	r := newRunner(store, notifier, Config{MaxEntries: 3})
	if err := r.AlertExpiry(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 1 {
		t.Fatalf("alerts = %d, want 1", len(store.inserted))
	}
	msg := store.inserted[0].Message
	if !strings.Contains(msg, "first 3 by expiry; more may exist") {
		t.Fatalf("a full page must be flagged as possibly truncated, got %q", msg)
	}
	if store.seenLimit != 3 {
		t.Fatalf("scan limit = %d, want the configured 3", store.seenLimit)
	}
}
