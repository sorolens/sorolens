package handler

// v2 API (issue #144).
//
// v1 grew organically and is inconsistent: some list endpoints return
// `{events: [...]}`, others `{storage: [...]}`; some paginate, some do not;
// some expose a bare `cursor`, others `next_cursor`; and several numeric
// fields carry inconsistent names (`cpu_insn`, `mem_byte`,
// `ledger_read_byte`). v2 fixes all of that without touching v1.
//
// # v2 conventions
//
//  1. Every list endpoint returns the same envelope:
//     {"data": [...], "pagination": {"next_cursor": string|null, "has_more": bool}}
//     The item key is always `data`, never a resource-specific name.
//  2. Every list endpoint accepts `cursor` and `limit` and always returns
//     `pagination`, even when the result is empty.
//  3. Every timestamp is an RFC 3339 UTC string. v2 never returns unix epoch.
//  4. Optional fields are returned explicitly as null rather than omitted, so
//     clients can rely on the key always being present.
//  5. Nested arrays inside a single-resource document keep their semantic name
//     (`storage`, `series`, `events`) because they are not the page itself.
//  6. Errors keep the v1 shape: {"error": {"code", "message", "request_id"}}.
//
// The field-level v1 → v2 mapping is documented in docs/api-v2.md.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- envelope ---------------------------------------------------------------

type v2Pagination struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

type v2List[T any] struct {
	Data       []T          `json:"data"`
	Pagination v2Pagination `json:"pagination"`
}

// v2Data guarantees an empty list serialises as [] rather than null.
func v2Data[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// v2Page builds the uniform list envelope. nextRaw is the raw (unencoded)
// cursor returned by the store; an empty value means the page is the last one.
func v2Page[T any](items []T, nextRaw string) v2List[T] {
	pg := v2Pagination{HasMore: nextRaw != ""}
	if nextRaw != "" {
		encoded := encodeCursor(nextRaw)
		pg.NextCursor = &encoded
	}
	return v2List[T]{Data: v2Data(items), Pagination: pg}
}

// ---- shared coercion helpers ------------------------------------------------

// v2Time renders every timestamp in v2 as RFC 3339 in UTC.
func v2Time(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// v2TimePtr renders a nullable timestamp, keeping the key present as null.
func v2TimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := v2Time(*t)
	return &s
}

// v2Str maps v1's empty-string-means-absent convention onto an explicit null.
func v2Str(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// v2Cursor decodes the opaque cursor, writing a 422 and returning ok=false
// when it is malformed.
func v2Cursor(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return "", false
	}
	return raw, true
}

// ---- DTOs -------------------------------------------------------------------

type v2Contract struct {
	ID                 string  `json:"id"`
	Network            string  `json:"network"`
	Label              *string `json:"label"`
	WasmHash           *string `json:"wasm_hash"`
	CreatedAtLedger    int64   `json:"created_at_ledger"`
	BackfillCompleteAt *string `json:"backfill_complete_at"`
	Status             string  `json:"status"`
	AddedAt            string  `json:"added_at"`
}

func v2ContractFromStore(c store.Contract) v2Contract {
	return v2Contract{
		ID:                 c.ID,
		Network:            c.Network,
		Label:              v2Str(c.Label),
		WasmHash:           v2Str(c.WasmHash),
		CreatedAtLedger:    c.CreatedAtLedger,
		BackfillCompleteAt: v2TimePtr(c.BackfillCompleteAt),
		Status:             c.Status,
		AddedAt:            v2Time(c.AddedAt),
	}
}

type v2Event struct {
	ID               string   `json:"id"`
	ContractID       string   `json:"contract_id"`
	Network          string   `json:"network"`
	Ledger           uint32   `json:"ledger"`
	LedgerClosedAt   string   `json:"ledger_closed_at"`
	TxHash           string   `json:"tx_hash"`
	Type             string   `json:"type"`
	TopicXDR         []string `json:"topic_xdr"`
	ValueXDR         string   `json:"value_xdr"`
	TopicDecoded     []any    `json:"topic_decoded"`
	ValueDecoded     any      `json:"value_decoded"`
	InSuccessfulCall bool     `json:"in_successful_call"`
}

func v2EventFromStore(e store.Event) v2Event {
	return v2Event{
		ID:               e.ID,
		ContractID:       e.ContractID,
		Network:          e.Network,
		Ledger:           e.Ledger,
		LedgerClosedAt:   v2Time(e.LedgerClosedAt),
		TxHash:           e.TxHash,
		Type:             e.Type,
		TopicXDR:         v2Data(e.TopicXDR),
		ValueXDR:         e.ValueXDR,
		TopicDecoded:     v2Data(e.TopicDecoded),
		ValueDecoded:     e.ValueDecoded,
		InSuccessfulCall: e.InSuccessfulCall,
	}
}

type v2Invocation struct {
	TxHash                    string         `json:"tx_hash"`
	ContractID                string         `json:"contract_id"`
	Network                   string         `json:"network"`
	Ledger                    uint32         `json:"ledger"`
	LedgerClosedAt            string         `json:"ledger_closed_at"`
	Status                    string         `json:"status"`
	FunctionName              *string        `json:"function_name"`
	ArgsDecoded               map[string]any `json:"args_decoded"`
	ResultDecoded             any            `json:"result_decoded"`
	ResultXDR                 string         `json:"result_xdr"`
	ResourceFeeChargedStroops int64          `json:"resource_fee_charged_stroops"`
	CPUInstructions           int64          `json:"cpu_instructions"`
	MemoryBytes               int64          `json:"memory_bytes"`
	LedgerReadBytes           int64          `json:"ledger_read_bytes"`
	LedgerWriteBytes          int64          `json:"ledger_write_bytes"`
	ApplicationOrder          int            `json:"application_order"`
}

func v2InvocationFromStore(inv store.Invocation) v2Invocation {
	return v2Invocation{
		TxHash:                    inv.TxHash,
		ContractID:                inv.ContractID,
		Network:                   inv.Network,
		Ledger:                    inv.Ledger,
		LedgerClosedAt:            v2Time(inv.LedgerClosedAt),
		Status:                    inv.Status,
		FunctionName:              v2Str(inv.FunctionName),
		ArgsDecoded:               inv.ArgsDecoded,
		ResultDecoded:             inv.ResultDecoded,
		ResultXDR:                 inv.ResultXDR,
		ResourceFeeChargedStroops: inv.ResourceFeeCharged,
		CPUInstructions:           inv.CPUInsn,
		MemoryBytes:               inv.MemByte,
		LedgerReadBytes:           inv.LedgerReadByte,
		LedgerWriteBytes:          inv.LedgerWriteByte,
		ApplicationOrder:          inv.ApplicationOrder,
	}
}

type v2StorageEntry struct {
	ContractID         string `json:"contract_id"`
	Network            string `json:"network"`
	KeyXDR             string `json:"key_xdr"`
	KeyDecoded         any    `json:"key_decoded"`
	ValueXDR           string `json:"value_xdr"`
	ValueDecoded       any    `json:"value_decoded"`
	Durability         string `json:"durability"`
	LiveUntilLedger    int64  `json:"live_until_ledger"`
	LastModifiedLedger int64  `json:"last_modified_ledger"`
	Status             string `json:"status"`
	LastSeenAt         string `json:"last_seen_at"`
}

func v2StorageEntryFromStore(se store.StorageEntry) v2StorageEntry {
	return v2StorageEntry{
		ContractID:         se.ContractID,
		Network:            se.Network,
		KeyXDR:             se.KeyXDR,
		KeyDecoded:         se.KeyDecoded,
		ValueXDR:           se.ValueXDR,
		ValueDecoded:       se.ValueDecoded,
		Durability:         se.Durability,
		LiveUntilLedger:    se.LiveUntilLedger,
		LastModifiedLedger: se.LastModifiedLedger,
		Status:             se.Status,
		LastSeenAt:         v2Time(se.LastSeenAt),
	}
}

type v2ContractStats struct {
	EventCount            int64  `json:"event_count"`
	InvocationCount       int64  `json:"invocation_count"`
	StorageCount          int64  `json:"storage_count"`
	LastSyncedLedger      uint32 `json:"last_synced_ledger"`
	WindowEventCount      int64  `json:"window_event_count"`
	WindowInvocationCount int64  `json:"window_invocation_count"`
	WindowDuration        string `json:"window_duration"`
}

type v2GlobalStats struct {
	TrackedContracts    int64 `json:"tracked_contracts"`
	TotalEvents         int64 `json:"total_events"`
	TotalInvocations    int64 `json:"total_invocations"`
	TotalStorageEntries int64 `json:"total_storage_entries"`
}

type v2ContractRate struct {
	ContractID string  `json:"contract_id"`
	Label      *string `json:"label"`
	Network    string  `json:"network"`
	Total      int64   `json:"total"`
	PerMinute  []int64 `json:"per_minute"`
}

type v2Activity struct {
	Minutes     int              `json:"minutes"`
	WindowStart string           `json:"window_start"`
	Data        []v2ContractRate `json:"data"`
}

type v2Upgrade struct {
	ContractID string  `json:"contract_id"`
	FromHash   string  `json:"from_hash"`
	ToHash     string  `json:"to_hash"`
	Ledger     int64   `json:"ledger"`
	TxHash     *string `json:"tx_hash"`
	At         string  `json:"at"`
}

func v2UpgradeFromStore(u store.ContractUpgrade) v2Upgrade {
	return v2Upgrade{
		ContractID: u.ContractID,
		FromHash:   u.FromHash,
		ToHash:     u.ToHash,
		Ledger:     u.Ledger,
		TxHash:     v2Str(u.TxHash),
		At:         v2Time(u.At),
	}
}

type v2MonitoredContract struct {
	ContractID    string  `json:"contract_id"`
	Network       string  `json:"network"`
	Name          string  `json:"name"`
	Owner         string  `json:"owner"`
	Status        string  `json:"status"`
	LastCheck     *string `json:"last_check"`
	CheckInterval int64   `json:"check_interval"`
	RegisteredAt  string  `json:"registered_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type v2HealthCheck struct {
	ContractID string `json:"contract_id"`
	Status     string `json:"status"`
	Metadata   string `json:"metadata"`
	Ledger     int64  `json:"ledger"`
	TxHash     string `json:"tx_hash"`
	Timestamp  string `json:"timestamp"`
}

type v2Alert struct {
	ContractID string `json:"contract_id"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Ledger     int64  `json:"ledger"`
	TxHash     string `json:"tx_hash"`
	Timestamp  string `json:"timestamp"`
}

type v2WatchdogStats struct {
	TotalMonitored int64 `json:"total_monitored"`
	Healthy        int64 `json:"healthy"`
	Degraded       int64 `json:"degraded"`
	Unresponsive   int64 `json:"unresponsive"`
	TotalAlerts    int64 `json:"total_alerts"`
	CriticalAlerts int64 `json:"critical_alerts"`
}

type v2WatchlistItem struct {
	ContractID string `json:"contract_id"`
	AddedAt    string `json:"added_at"`
}

type v2WatchlistStatus struct {
	InWatchlist bool `json:"in_watchlist"`
}

// ---- stats ------------------------------------------------------------------

// V2GlobalStats handles GET /api/v2/stats/global.
func (h *Handler) V2GlobalStats(w http.ResponseWriter, r *http.Request) {
	gs, err := h.Store.GetGlobalStats(r.Context())
	if err != nil {
		h.Logger.Error("v2 get global stats", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch stats")
		return
	}
	writeJSON(w, http.StatusOK, v2GlobalStats{
		TrackedContracts:    gs.TrackedContracts,
		TotalEvents:         gs.TotalEvents,
		TotalInvocations:    gs.TotalInvocations,
		TotalStorageEntries: gs.TotalStorageEntries,
	})
}

// ---- contracts --------------------------------------------------------------

// V2ListContracts handles GET /api/v2/contracts.
func (h *Handler) V2ListContracts(w http.ResponseWriter, r *http.Request) {
	rawCursor, ok := v2Cursor(w, r)
	if !ok {
		return
	}
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	contracts, nextRaw, err := h.Store.ListContracts(r.Context(), rawCursor, intQuery(r, "limit", 50), store.ContractFilters{
		Network: network,
		Status:  r.URL.Query().Get("status"),
	})
	if err != nil {
		h.Logger.Error("v2 list contracts", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list contracts")
		return
	}
	resp := make([]v2Contract, len(contracts))
	for i, c := range contracts {
		resp[i] = v2ContractFromStore(c)
	}
	writeJSON(w, http.StatusOK, v2Page(resp, nextRaw))
}

// V2RegisterContract handles POST /api/v2/contracts. Same semantics as v1
// (including the 422 validation codes), but returns the v2 contract document.
func (h *Handler) V2RegisterContract(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if !validateContractID(req.ID) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "id must be a 56-character string starting with 'C'")
		return
	}
	if !validNetworks[req.Network] {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}

	c := store.Contract{
		ID:      req.ID,
		Network: req.Network,
		Label:   req.Label,
		Status:  "pending",
		AddedAt: time.Now().UTC(),
	}
	if err := h.Store.UpsertContract(r.Context(), c); err != nil {
		h.Logger.Error("v2 upsert contract", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to register contract")
		return
	}
	writeJSON(w, http.StatusCreated, v2ContractFromStore(c))
}

// V2GetContract handles GET /api/v2/contracts/{id}.
func (h *Handler) V2GetContract(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c, err := h.Store.GetContract(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
		return
	}
	if err != nil {
		h.Logger.Error("v2 get contract", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch contract")
		return
	}
	writeJSON(w, http.StatusOK, v2ContractFromStore(c))
}

// ---- events -----------------------------------------------------------------

// V2ListEvents handles GET /api/v2/contracts/{id}/events.
func (h *Handler) V2ListEvents(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	rawCursor, ok := v2Cursor(w, r)
	if !ok {
		return
	}
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	limit := intQuery(r, "limit", 50)
	f := store.EventFilters{
		Type:    r.URL.Query().Get("type"),
		Network: network,
		From:    uint32Query(r, "from"),
		To:      uint32Query(r, "to"),
	}
	events, nextRaw, err := h.Store.ListEvents(r.Context(), contractID, rawCursor, limit, f)
	if err != nil {
		h.Logger.Error("v2 list events", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list events")
		return
	}
	// Same cold-storage fallback as v1 (#146) so archived ledgers stay
	// queryable through both namespaces.
	if len(events) == 0 && h.Cold != nil && f.From > 0 && rawCursor == "" {
		archived, coldErr := h.Cold.Events(r.Context(), contractID, f.From, f.To, limit)
		if coldErr != nil {
			h.Logger.Error("v2 list archived events", "err", coldErr, "contract_id", contractID)
		} else if len(archived) > 0 {
			events = archived
			nextRaw = ""
		}
	}
	resp := make([]v2Event, len(events))
	for i, e := range events {
		resp[i] = v2EventFromStore(e)
	}
	writeJSON(w, http.StatusOK, v2Page(resp, nextRaw))
}

// V2RecentEvents handles GET /api/v2/events/recent.
func (h *Handler) V2RecentEvents(w http.ResponseWriter, r *http.Request) {
	limit := intQuery(r, "limit", 50)
	if limit > 200 {
		limit = 200
	}
	events, err := h.Store.RecentEventsAll(r.Context(), limit)
	if err != nil {
		h.Logger.Error("v2 recent events", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch recent events")
		return
	}
	resp := make([]v2Event, len(events))
	for i, e := range events {
		resp[i] = v2EventFromStore(e)
	}
	writeJSON(w, http.StatusOK, v2Page(resp, ""))
}

// V2LiveActivity handles GET /api/v2/stats/activity.
func (h *Handler) V2LiveActivity(w http.ResponseWriter, r *http.Request) {
	minutes := intQuery(r, "minutes", store.LiveWindowMinute)
	if minutes > store.MaxLiveWindowMinute {
		minutes = store.MaxLiveWindowMinute
	}
	rates, err := h.Store.ContractEventRates(r.Context(), minutes)
	if err != nil {
		h.Logger.Error("v2 live activity", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch live activity")
		return
	}
	resp := make([]v2ContractRate, len(rates))
	for i, rate := range rates {
		resp[i] = v2ContractRate{
			ContractID: rate.ContractID,
			Label:      v2Str(rate.Label),
			Network:    rate.Network,
			Total:      rate.Total,
			PerMinute:  v2Data(rate.PerMinute),
		}
	}
	writeJSON(w, http.StatusOK, v2Activity{
		Minutes:     minutes,
		WindowStart: v2Time(store.LiveWindowStart(minutes, time.Now())),
		Data:        resp,
	})
}

// ---- invocations / storage / stats -----------------------------------------

// V2ListInvocations handles GET /api/v2/contracts/{id}/invocations.
func (h *Handler) V2ListInvocations(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	rawCursor, ok := v2Cursor(w, r)
	if !ok {
		return
	}
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	// v2 renames the v1 `fn` query parameter to `function_name`, matching the
	// response field, and still accepts `fn` for a migration window.
	fn := r.URL.Query().Get("function_name")
	if fn == "" {
		fn = r.URL.Query().Get("fn")
	}
	f := store.InvocationFilters{
		Status:       r.URL.Query().Get("status"),
		FunctionName: fn,
		Network:      network,
		From:         uint32Query(r, "from"),
		To:           uint32Query(r, "to"),
	}
	invs, nextRaw, err := h.Store.ListInvocations(r.Context(), contractID, rawCursor, intQuery(r, "limit", 50), f)
	if err != nil {
		h.Logger.Error("v2 list invocations", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list invocations")
		return
	}
	resp := make([]v2Invocation, len(invs))
	for i, inv := range invs {
		resp[i] = v2InvocationFromStore(inv)
	}
	writeJSON(w, http.StatusOK, v2Page(resp, nextRaw))
}

// V2ListStorageEntries handles GET /api/v2/contracts/{id}/storage.
func (h *Handler) V2ListStorageEntries(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	rawCursor, ok := v2Cursor(w, r)
	if !ok {
		return
	}
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	f := store.StorageFilters{
		Durability: r.URL.Query().Get("durability"),
		Status:     r.URL.Query().Get("status"),
		Network:    network,
	}
	entries, nextRaw, err := h.Store.ListStorageEntries(r.Context(), contractID, rawCursor, intQuery(r, "limit", 50), f)
	if err != nil {
		h.Logger.Error("v2 list storage entries", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list storage entries")
		return
	}
	resp := make([]v2StorageEntry, len(entries))
	for i, se := range entries {
		resp[i] = v2StorageEntryFromStore(se)
	}
	writeJSON(w, http.StatusOK, v2Page(resp, nextRaw))
}

// V2ContractStats handles GET /api/v2/contracts/{id}/stats.
func (h *Handler) V2ContractStats(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	window := r.URL.Query().Get("window")
	if window == "" {
		window = "24h"
	}
	cs, err := h.Store.GetContractStats(r.Context(), contractID, window)
	if err != nil {
		h.Logger.Error("v2 get contract stats", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch stats")
		return
	}
	writeJSON(w, http.StatusOK, v2ContractStats{
		EventCount:            cs.EventCount,
		InvocationCount:       cs.InvocationCount,
		StorageCount:          cs.StorageCount,
		LastSyncedLedger:      cs.LastSyncedLedger,
		WindowEventCount:      cs.WindowEventCount,
		WindowInvocationCount: cs.WindowInvocationCount,
		WindowDuration:        cs.WindowDuration,
	})
}

// ---- upgrades / health score ------------------------------------------------

// V2ListContractUpgrades handles GET /api/v2/contracts/{id}/upgrades.
func (h *Handler) V2ListContractUpgrades(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	if _, err := h.Store.GetContract(r.Context(), contractID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
			return
		}
		h.Logger.Error("v2 get contract", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load contract")
		return
	}
	upgrades, err := h.Store.ListContractUpgrades(r.Context(), contractID, intQuery(r, "limit", 50))
	if err != nil {
		h.Logger.Error("v2 list contract upgrades", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list contract upgrades")
		return
	}
	resp := make([]v2Upgrade, len(upgrades))
	for i, u := range upgrades {
		resp[i] = v2UpgradeFromStore(u)
	}
	writeJSON(w, http.StatusOK, v2Page(resp, ""))
}

// V2GetContractHealthScore handles GET /api/v2/contracts/{id}/health-score.
func (h *Handler) V2GetContractHealthScore(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	if _, err := h.Store.GetContract(r.Context(), contractID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
			return
		}
		h.Logger.Error("v2 get contract for health score", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load contract")
		return
	}
	health, err := h.Store.GetContractHealthScore(r.Context(), contractID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "health score not yet computed")
			return
		}
		h.Logger.Error("v2 get contract health score", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load health score")
		return
	}
	// The v1 shape already uses RFC 3339 for computed_at and numeric
	// components, so v2 serves the same document.
	writeJSON(w, http.StatusOK, contractHealthScoreFromStore(health))
}

// ---- watchdog ---------------------------------------------------------------

// V2WatchdogStats handles GET /api/v2/watchdog/stats.
func (h *Handler) V2WatchdogStats(w http.ResponseWriter, r *http.Request) {
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	s, err := h.Store.GetWatchdogStats(r.Context(), network)
	if err != nil {
		h.Logger.Error("v2 watchdog stats", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch watchdog stats")
		return
	}
	writeJSON(w, http.StatusOK, v2WatchdogStats{
		TotalMonitored: s.TotalMonitored,
		Healthy:        s.Healthy,
		Degraded:       s.Degraded,
		Unresponsive:   s.Unresponsive,
		TotalAlerts:    s.TotalAlerts,
		CriticalAlerts: s.CriticalAlerts,
	})
}

// V2ListMonitoredContracts handles GET /api/v2/watchdog/contracts.
func (h *Handler) V2ListMonitoredContracts(w http.ResponseWriter, r *http.Request) {
	rawCursor, ok := v2Cursor(w, r)
	if !ok {
		return
	}
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	items, nextRaw, err := h.Store.ListMonitoredContracts(r.Context(), rawCursor, intQuery(r, "limit", 50), network)
	if err != nil {
		h.Logger.Error("v2 list monitored contracts", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list monitored contracts")
		return
	}
	resp := make([]v2MonitoredContract, len(items))
	for i, m := range items {
		resp[i] = v2MonitoredContract{
			ContractID:    m.ContractID,
			Network:       m.Network,
			Name:          m.Name,
			Owner:         m.Owner,
			Status:        m.Status,
			LastCheck:     v2TimePtr(m.LastCheck),
			CheckInterval: m.CheckInterval,
			RegisteredAt:  v2Time(m.RegisteredAt),
			UpdatedAt:     v2Time(m.UpdatedAt),
		}
	}
	writeJSON(w, http.StatusOK, v2Page(resp, nextRaw))
}

// V2GetMonitoredContract handles GET /api/v2/watchdog/contracts/{id}.
func (h *Handler) V2GetMonitoredContract(w http.ResponseWriter, r *http.Request) {
	m, err := h.Store.GetMonitoredContract(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "monitored contract not found")
		return
	}
	if err != nil {
		h.Logger.Error("v2 get monitored contract", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch monitored contract")
		return
	}
	writeJSON(w, http.StatusOK, v2MonitoredContract{
		ContractID:    m.ContractID,
		Network:       m.Network,
		Name:          m.Name,
		Owner:         m.Owner,
		Status:        m.Status,
		LastCheck:     v2TimePtr(m.LastCheck),
		CheckInterval: m.CheckInterval,
		RegisteredAt:  v2Time(m.RegisteredAt),
		UpdatedAt:     v2Time(m.UpdatedAt),
	})
}

// V2ListHealthChecks handles GET /api/v2/watchdog/contracts/{id}/health.
func (h *Handler) V2ListHealthChecks(w http.ResponseWriter, r *http.Request) {
	checks, err := h.Store.ListHealthChecks(r.Context(), chi.URLParam(r, "id"), intQuery(r, "limit", 100))
	if err != nil {
		h.Logger.Error("v2 list health checks", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list health checks")
		return
	}
	resp := make([]v2HealthCheck, len(checks))
	for i, c := range checks {
		resp[i] = v2HealthCheck{
			ContractID: c.ContractID,
			Status:     c.Status,
			Metadata:   c.Metadata,
			Ledger:     c.Ledger,
			TxHash:     c.TxHash,
			Timestamp:  v2Time(c.Timestamp),
		}
	}
	writeJSON(w, http.StatusOK, v2Page(resp, ""))
}

// V2ListWatchdogAlerts handles GET /api/v2/watchdog/alerts and
// GET /api/v2/watchdog/contracts/{id}/alerts.
func (h *Handler) V2ListWatchdogAlerts(w http.ResponseWriter, r *http.Request) {
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	alerts, next, err := h.Store.ListAlerts(
		r.Context(),
		chi.URLParam(r, "id"),
		r.URL.Query().Get("severity"),
		network,
		r.URL.Query().Get("cursor"),
		intQuery(r, "limit", 100),
	)
	if err != nil {
		h.Logger.Error("v2 list alerts", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list alerts")
		return
	}
	resp := make([]v2Alert, len(alerts))
	for i, a := range alerts {
		resp[i] = v2Alert{
			ContractID: a.ContractID,
			Severity:   a.Severity,
			Message:    a.Message,
			Ledger:     a.Ledger,
			TxHash:     a.TxHash,
			Timestamp:  v2Time(a.Timestamp),
		}
	}
	writeJSON(w, http.StatusOK, v2Page(resp, next))
}

// ---- watchlist --------------------------------------------------------------

// V2ListWatchlist handles GET /api/v2/watchlist.
func (h *Handler) V2ListWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeInvalidInput, "X-User-ID header is required")
		return
	}
	contractIDs, err := h.Store.ListWatchlist(r.Context(), userID)
	if err != nil {
		h.Logger.Error("v2 list watchlist", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list watchlist")
		return
	}
	resp := make([]v2WatchlistItem, len(contractIDs))
	for i, cid := range contractIDs {
		resp[i] = v2WatchlistItem{ContractID: cid, AddedAt: v2Time(time.Now())}
	}
	writeJSON(w, http.StatusOK, v2Page(resp, ""))
}

// V2AddToWatchlist handles POST /api/v2/watchlist.
func (h *Handler) V2AddToWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeInvalidInput, "X-User-ID header is required")
		return
	}
	var req struct {
		ContractID string `json:"contract_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.ContractID) == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "contract_id is required")
		return
	}
	if err := h.Store.AddToWatchlist(r.Context(), userID, req.ContractID); err != nil {
		h.Logger.Error("v2 add to watchlist", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to add to watchlist")
		return
	}
	writeJSON(w, http.StatusCreated, v2WatchlistStatus{InWatchlist: true})
}

// V2RemoveFromWatchlist handles DELETE /api/v2/watchlist/{contractId}.
func (h *Handler) V2RemoveFromWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeInvalidInput, "X-User-ID header is required")
		return
	}
	if err := h.Store.RemoveFromWatchlist(r.Context(), userID, chi.URLParam(r, "contractId")); err != nil {
		h.Logger.Error("v2 remove from watchlist", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to remove from watchlist")
		return
	}
	writeJSON(w, http.StatusOK, v2WatchlistStatus{InWatchlist: false})
}

// V2WatchlistStatus handles GET /api/v2/watchlist/{contractId}/status.
func (h *Handler) V2WatchlistStatus(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeInvalidInput, "X-User-ID header is required")
		return
	}
	inWatchlist, err := h.Store.IsInWatchlist(r.Context(), userID, chi.URLParam(r, "contractId"))
	if err != nil {
		h.Logger.Error("v2 watchlist status", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to check watchlist status")
		return
	}
	writeJSON(w, http.StatusOK, v2WatchlistStatus{InWatchlist: inWatchlist})
}

// ---- passthrough ------------------------------------------------------------

// V2ContractForecast, V2ContractSnapshot, V2StreamEvents, V2ContractGraph, and
// the api-keys handlers delegate to their v1 implementations. Those documents
// already satisfy the v2 conventions: RFC 3339 timestamps, explicit nulls,
// stable field names, and (for stream) a bounded non-paginated list. They are
// listed as passthrough in docs/api-v2.md so the mapping is explicit rather
// than implied.

// V2ValidateContract handles POST /api/v2/contracts/validate. The validation
// document is already version-neutral (booleans, nulls, echoed input), so v2
// serves the same response.
func (h *Handler) V2ValidateContract(w http.ResponseWriter, r *http.Request) {
	h.ValidateContract(w, r)
}

// V2ContractForecast handles GET /api/v2/contracts/{id}/forecast.
func (h *Handler) V2ContractForecast(w http.ResponseWriter, r *http.Request) {
	h.ContractForecast(w, r)
}

// V2ContractSnapshot handles GET /api/v2/contracts/{id}/snapshot.
func (h *Handler) V2ContractSnapshot(w http.ResponseWriter, r *http.Request) {
	h.ContractSnapshot(w, r)
}

// V2StreamEvents handles GET /api/v2/contracts/{id}/stream.
func (h *Handler) V2StreamEvents(w http.ResponseWriter, r *http.Request) {
	h.StreamEvents(w, r)
}

// V2ContractGraph handles GET /api/v2/contracts/{id}/graph.
func (h *Handler) V2ContractGraph(w http.ResponseWriter, r *http.Request) {
	h.ContractGraph(w, r)
}
