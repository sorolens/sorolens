package handler

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// summaryCacheTTL is how long a composite dashboard summary is memoized.
// A dashboard page load fans out into several aggregate queries; a 5s TTL
// absorbs that burst while keeping the counts visibly fresh.
const summaryCacheTTL = 5 * time.Second

// summaryCacheKey is the memo key for one contract's summary.
func summaryCacheKey(contractID string) string { return "contract_summary:" + contractID }

// SummaryCache is a small in-process TTL cache. It intentionally lives in the
// handler layer: the summary is derived purely from indexed data, so a stale
// read costs nothing except up to ttl seconds of lag.
type SummaryCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	entries map[string]summaryCacheEntry
}

type summaryCacheEntry struct {
	value   any
	expires time.Time
}

// NewSummaryCache returns a cache whose entries expire after ttl.
func NewSummaryCache(ttl time.Duration) *SummaryCache {
	return &SummaryCache{
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]summaryCacheEntry),
	}
}

// Get returns the cached value for key if it has not expired yet.
func (c *SummaryCache) Get(key string) (any, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !c.now().Before(e.expires) {
		delete(c.entries, key)
		return nil, false
	}
	return e.value, true
}

// Set stores value for key, replacing any previous entry.
func (c *SummaryCache) Set(key string, value any) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = summaryCacheEntry{value: value, expires: c.now().Add(c.ttl)}
}

// summaryCache returns the process-wide summary cache, creating it on first
// use so the zero-value Handler used in tests needs no constructor.
func (h *Handler) summaryCache() *SummaryCache {
	h.summaryCacheOnce.Do(func() {
		h.summaryCacheVal = NewSummaryCache(summaryCacheTTL)
	})
	return h.summaryCacheVal
}

// contractSummaryResponse is the composite document a dashboard needs in one
// call: totals, the newest event, the newest invocation, and the cached health
// score. The optional sub-documents are null until the indexer has produced
// the corresponding data, so the schema is stable across contracts.
type contractSummaryResponse struct {
	ContractID       string                       `json:"contract_id"`
	Network          string                       `json:"network"`
	Label            string                       `json:"label"`
	Status           string                       `json:"status"`
	GeneratedAt      time.Time                    `json:"generated_at"`
	Stats            contractStatsResponse        `json:"stats"`
	LatestEvent      *eventResponse               `json:"latest_event"`
	LatestInvocation *invocationResponse          `json:"latest_invocation"`
	HealthScore      *contractHealthScoreResponse `json:"health_score"`
}

func contractStatsFromStore(cs store.ContractStats) contractStatsResponse {
	return contractStatsResponse{
		EventCount:            cs.EventCount,
		InvocationCount:       cs.InvocationCount,
		StorageCount:          cs.StorageCount,
		LastSyncedLedger:      cs.LastSyncedLedger,
		WindowEventCount:      cs.WindowEventCount,
		WindowInvocationCount: cs.WindowInvocationCount,
		WindowDuration:        cs.WindowDuration,
	}
}

// ContractSummary handles GET /api/v1/contracts/{id}/summary.
//
// It composes the existing per-contract queries (counts, recent events, recent
// invocations, cached health score) into one document so a dashboard page load
// costs a single request. Every lookup is either an aggregate or a
// limit-1 "newest row" query, so the work is constant per request — no per-row
// (N+1) queries — and the composed result is memoized for summaryCacheTTL.
func (h *Handler) ContractSummary(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	cache := h.summaryCache()
	if cached, ok := cache.Get(summaryCacheKey(contractID)); ok {
		if resp, ok := cached.(contractSummaryResponse); ok {
			writeJSON(w, http.StatusOK, resp)
			return
		}
	}

	contract, err := h.Store.GetContract(r.Context(), contractID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
		return
	}
	if err != nil {
		h.Logger.Error("summary get contract", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch contract")
		return
	}

	stats, err := h.Store.GetContractStats(r.Context(), contractID, "24h")
	if err != nil {
		h.Logger.Error("summary contract stats", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch contract stats")
		return
	}

	resp := contractSummaryResponse{
		ContractID:  contract.ID,
		Network:     contract.Network,
		Label:       contract.Label,
		Status:      contract.Status,
		GeneratedAt: time.Now().UTC(),
		Stats:       contractStatsFromStore(stats),
	}

	events, err := h.Store.RecentEvents(r.Context(), contractID, 1)
	if err != nil {
		h.Logger.Error("summary recent events", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch latest event")
		return
	}
	if len(events) > 0 {
		e := eventFromStore(events[0])
		resp.LatestEvent = &e
	}

	invs, err := h.Store.RecentInvocations(r.Context(), contractID, 1)
	if err != nil {
		h.Logger.Error("summary recent invocations", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch latest invocation")
		return
	}
	if len(invs) > 0 {
		inv := invocationFromStore(invs[0])
		resp.LatestInvocation = &inv
	}

	health, err := h.Store.GetContractHealthScore(r.Context(), contractID)
	switch {
	case err == nil:
		hs := contractHealthScoreFromStore(health)
		resp.HealthScore = &hs
	case errors.Is(err, store.ErrNotFound):
		// The indexer has not scored this contract yet; health_score stays null.
	default:
		h.Logger.Error("summary health score", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch health score")
		return
	}

	cache.Set(summaryCacheKey(contractID), resp)
	writeJSON(w, http.StatusOK, resp)
}
