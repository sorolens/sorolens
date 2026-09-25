package handler

import (
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
		score := hs.Score
		e.HealthScore = &score
	case errors.Is(err, store.ErrNotFound):
		// Indexer has not computed a score yet; leave it null.
	default:
		h.Logger.Error("compare health score", "err", err, "contract_id", id)
		e.Error = "failed to load health score"
		return e
	}

	e.HasData = e.Tracked && (e.EventCount > 0 || e.InvocationCount > 0 || e.HealthScore != nil)
	return e
}
