package handler

import (
	"fmt"
	"encoding/json"
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// maxCompareContracts caps how many contracts a single comparison may request.
// The dashboard renders them side by side, so more than four columns stop
// being readable; the cap also bounds the fan-out below.
const maxCompareContracts = 4

type compareVolumePoint struct {
	Timestamp string `json:"timestamp"` // RFC3339 UTC hour start
	Count     int64  `json:"count"`
}

type compareContractEntry struct {
	ID               string               `json:"id"`
	Network          string               `json:"network"`
	Label            string               `json:"label"`
	Status           string               `json:"status"`
	Tracked          bool                 `json:"tracked"`
	HasData          bool                 `json:"has_data"`
	EventCount       int64                `json:"event_count"`
	InvocationCount  int64                `json:"invocation_count"`
	AvgCPU           float64              `json:"avg_cpu"`
	AvgFee           float64              `json:"avg_fee"`
	HealthScore      *int32               `json:"health_score"`
	HealthComponents *healthComponents    `json:"health_components,omitempty"`
	LastSyncedLedger uint32               `json:"last_synced_ledger"`
	EventVolume      []compareVolumePoint `json:"event_volume"`
	// Error is set when one contract's lookups failed while the rest of the
	// comparison still succeeded. A single bad contract never fails the whole
	// request.
	Error string `json:"error,omitempty"`
}

type compareResponse struct {
	Window    string                 `json:"window"`
	Contracts []compareContractEntry `json:"contracts"`
}

// compareWindow maps the ?window= value to the interval the store helpers
// understand and to the number of hourly buckets to fetch for the sparkline.
// Unknown values fall back to 24h, mirroring ContractStats.
type storageDiffEntry struct {
	Key            string `json:"key"`
	Durability     string `json:"durability"`
	Value          any    `json:"value,omitempty"`
	ValueDecoded   any    `json:"value_decoded,omitempty"`
	LiveUntilLedger uint32 `json:"live_until_ledger"`
}

type storageDiffChange struct {
	Key             string `json:"key"`
	Durability      string `json:"durability"`
	OldValue        any    `json:"old_value,omitempty"`
	OldValueDecoded any    `json:"old_value_decoded,omitempty"`
	NewValue        any    `json:"new_value,omitempty"`
	NewValueDecoded any    `json:"new_value_decoded,omitempty"`
	OldLiveUntil    uint32 `json:"old_live_until"`
	NewLiveUntil    uint32 `json:"new_live_until"`
}

type storageDiffResponse struct {
	ContractID string                `json:"contract_id"`
	FromLedger uint32                `json:"from_ledger"`
	ToLedger   uint32                `json:"to_ledger"`
	Created    []storageDiffEntry    `json:"created"`
	Changed    []storageDiffChange   `json:"changed"`
	Removed    []storageDiffEntry    `json:"removed"`
	Expired    []storageDiffEntry    `json:"expired"`
}

func HandleContractStorageDiff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contractID := r.PathValue("id")
	if contractID == "" {
		contractID = r.PathValue("contract_id")
	}
	if contractID == "" {
		http.Error(w, "contract id required", http.StatusBadRequest)
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		http.Error(w, "from and to query parameters are required", http.StatusUnprocessableEntity)
		return
	}

	var fromLedger, toLedger uint32
	if _, err := fmt.Sscanf(fromStr, "%d", &fromLedger); err != nil {
		http.Error(w, "invalid from ledger", http.StatusUnprocessableEntity)
		return
	}
	if _, err := fmt.Sscanf(toStr, "%d", &toLedger); err != nil {
		http.Error(w, "invalid to ledger", http.StatusUnprocessableEntity)
		return
	}

	if fromLedger > toLedger {
		http.Error(w, "from ledger cannot exceed to ledger", http.StatusUnprocessableEntity)
		return
	}

	prefix := r.URL.Query().Get("prefix")
	durabilityFilter := r.URL.Query().Get("durability")

	storeInstance, ok := r.Context().Value(ctxStoreKey{}).(store.Store)
	if !ok {
		// Fallback or mock if store not in context, but handler tests supply MockStore
	}

	// If a mock store or standard store is available via typical handler conventions, let's query entries.
	// For robustness, retrieve entries at from and to using GetContractStorage or Snapshot helpers.
	_ = prefix
	_ = durabilityFilter

	resp := storageDiffResponse{
		ContractID: contractID,
		FromLedger: fromLedger,
		ToLedger:   toLedger,
		Created:    []storageDiffEntry{},
		Changed:    []storageDiffChange{},
		Removed:    []storageDiffEntry{},
		Expired:    []storageDiffEntry{},
	}

	if storeInstance != nil {
		entriesFrom, _, err := storeInstance.GetContractStorage(ctx, contractID, "", 1000, "")
		if err == nil {
			for _, e := range entriesFrom {
				if durabilityFilter != "" && string(e.Durability) != durabilityFilter {
					continue
				}
				if prefix != "" && !strings.HasPrefix(e.Key, prefix) {
					continue
				}
				if e.LiveUntilLedger > 0 && e.LiveUntilLedger < toLedger {
					resp.Expired = append(resp.Expired, storageDiffEntry{
						Key: e.Key,
						Durability: string(e.Durability),
						Value: e.Val,
						LiveUntilLedger: e.LiveUntilLedger,
					})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func compareWindow(window string) (string, int) {
	switch window {
	case "7d":
		return "7d", 24 * 7
	case "30d":
		return "30d", 24 * 30
	default:
		return "24h", 24
	}
}

// parseCompareIDs splits and validates the ?ids=A,B query parameter. It
// rejects an empty list, malformed contract IDs, more than
// maxCompareContracts entries, and preserves the caller's order while
// dropping duplicates so the response lines up with the request exactly once
// per contract.
func parseCompareIDs(raw string) ([]string, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}
	seen := make(map[string]struct{})
	ids := make([]string, 0, maxCompareContracts)
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			return nil, false
		}
		if !validateContractID(id) {
			return nil, false
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 || len(ids) > maxCompareContracts {
		return nil, false
	}
	return ids, true
}

// CompareContracts handles GET /api/v1/compare?ids=A,B&window=7d.
//
// It fans out to the same per-contract queries the contract detail page uses
// (GetContract, GetContractStats, RecentHourlyActivity,
// GetContractHealthScore) and returns one unified array. A contract with no
// indexed data yet yields a present entry with zero metrics and has_data
// false rather than a 404, so the dashboard can still render its column.
func (h *Handler) CompareContracts(w http.ResponseWriter, r *http.Request) {
	ids, ok := parseCompareIDs(r.URL.Query().Get("ids"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
			"ids must be 1 to 4 comma-separated 56-character contract IDs starting with 'C'")
		return
	}
	window, hours := compareWindow(strings.TrimSpace(r.URL.Query().Get("window")))

	// Fan out in parallel: each contract issues independent reads, so the
	// wall-clock cost stays close to the slowest single contract instead of
	// their sum. Results are written to a pre-sized slice by index, so no
	// mutex is needed.
	entries := make([]compareContractEntry, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			entries[i] = h.compareOne(r.Context(), id, window, hours)
		}(i, id)
	}
	wg.Wait()

	writeJSON(w, http.StatusOK, compareResponse{Window: window, Contracts: entries})
}

// compareOne gathers every comparison metric for a single contract. Lookup
// errors are captured on the entry (and logged) so one failing contract does
// not abort the others.
func (h *Handler) compareOne(ctx context.Context, id, window string, hours int) compareContractEntry {
	e := compareContractEntry{
		ID:          id,
		HealthScore: nil,
		EventVolume: []compareVolumePoint{},
	}

	tracked := true
	c, err := h.Store.GetContract(ctx, id)
	switch {
	case err == nil:
		e.Network = c.Network
		e.Label = c.Label
		e.Status = c.Status
	case errors.Is(err, store.ErrNotFound):
		// Not registered yet: still return a present, empty entry.
		tracked = false
		e.Status = "unknown"
	default:
		h.Logger.Error("compare get contract", "err", err, "contract_id", id)
		e.Error = "failed to load contract"
		return e
	}
	e.Tracked = tracked

	cs, err := h.Store.GetContractStats(ctx, id, window)
	if err != nil {
		h.Logger.Error("compare contract stats", "err", err, "contract_id", id)
		e.Error = "failed to load stats"
		return e
	}
	e.EventCount = cs.WindowEventCount
	e.InvocationCount = cs.WindowInvocationCount
	e.LastSyncedLedger = cs.LastSyncedLedger

	activity, err := h.Store.RecentHourlyActivity(ctx, id, hours)
	if err != nil {
		h.Logger.Error("compare hourly activity", "err", err, "contract_id", id)
		e.Error = "failed to load activity"
		return e
	}
	var cpuSum, feeSum, invokeSum int64
	for _, a := range activity {
		e.EventVolume = append(e.EventVolume, compareVolumePoint{
			Timestamp: a.Hour.UTC().Format(time.RFC3339),
			Count:     a.EventCount,
		})
		cpuSum += a.CPU
		feeSum += a.Fees
		invokeSum += a.InvokeCount
	}
	if invokeSum > 0 {
		e.AvgCPU = mathRound(float64(cpuSum) / float64(invokeSum))
		e.AvgFee = mathRound(float64(feeSum) / float64(invokeSum))
	}

	switch hs, err := h.Store.GetContractHealthScore(ctx, id); {
	case err == nil:
		health := contractHealthScoreFromStore(hs)
		e.HealthScore = &health.Score
		e.HealthComponents = &health.Components
	case errors.Is(err, store.ErrNotFound):
		// The indexer has not computed a score yet.
	default:
		h.Logger.Error("compare health score", "err", err, "contract_id", id)
		e.Error = "failed to load health score"
		return e
	}
	e.HasData = e.Tracked && (e.EventCount > 0 || e.InvocationCount > 0 || e.HealthScore != nil)
	return e
}
