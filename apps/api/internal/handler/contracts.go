package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- response types ---------------------------------------------------------

type contractResponse struct {
	ID                 string     `json:"id"`
	Network            string     `json:"network"`
	Label              string     `json:"label"`
	WasmHash           string     `json:"wasm_hash"`
	CreatedAtLedger    int64      `json:"created_at_ledger"`
	BackfillCompleteAt *time.Time `json:"backfill_complete_at"`
	Status             string     `json:"status"`
	AddedAt            time.Time  `json:"added_at"`
}

type eventResponse struct {
	ID               string    `json:"id"`
	ContractID       string    `json:"contract_id"`
	Ledger           uint32    `json:"ledger"`
	LedgerClosedAt   time.Time `json:"ledger_closed_at"`
	TxHash           string    `json:"tx_hash"`
	Type             string    `json:"type"`
	TopicXDR         []string  `json:"topic_xdr"`
	ValueXDR         string    `json:"value_xdr"`
	TopicDecoded     []any     `json:"topic_decoded"`
	ValueDecoded     any       `json:"value_decoded"`
	InSuccessfulCall bool      `json:"in_successful_call"`
}

type invocationResponse struct {
	TxHash             string         `json:"tx_hash"`
	ContractID         string         `json:"contract_id"`
	Ledger             uint32         `json:"ledger"`
	LedgerClosedAt     time.Time      `json:"ledger_closed_at"`
	Status             string         `json:"status"`
	FunctionName       string         `json:"function_name"`
	ArgsDecoded        map[string]any `json:"args_decoded"`
	ResultDecoded      any            `json:"result_decoded"`
	ResultXDR          string         `json:"result_xdr"`
	ResourceFeeCharged int64          `json:"resource_fee_charged"`
	CPUInsn            int64          `json:"cpu_insn"`
	MemByte            int64          `json:"mem_byte"`
	LedgerReadByte     int64          `json:"ledger_read_byte"`
	LedgerWriteByte    int64          `json:"ledger_write_byte"`
	ApplicationOrder   int            `json:"application_order"`
}

type storageEntryResponse struct {
	ContractID         string    `json:"contract_id"`
	KeyXDR             string    `json:"key_xdr"`
	KeyDecoded         any       `json:"key_decoded"`
	ValueXDR           string    `json:"value_xdr"`
	ValueDecoded       any       `json:"value_decoded"`
	Durability         string    `json:"durability"`
	LiveUntilLedger    int64     `json:"live_until_ledger"`
	LastModifiedLedger int64     `json:"last_modified_ledger"`
	Status             string    `json:"status"`
	LastSeenAt         time.Time `json:"last_seen_at"`
}

type contractStatsResponse struct {
	EventCount            int64  `json:"event_count"`
	InvocationCount       int64  `json:"invocation_count"`
	StorageCount          int64  `json:"storage_count"`
	LastSyncedLedger      uint32 `json:"last_synced_ledger"`
	WindowEventCount      int64  `json:"window_event_count"`
	WindowInvocationCount int64  `json:"window_invocation_count"`
	WindowDuration        string `json:"window_duration"`
}

// ---- helpers ----------------------------------------------------------------

func encodeCursor(raw string) string {
	if raw == "" {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(encoded string) (string, bool) {
	if encoded == "" {
		return "", true
	}
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func intQuery(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func uint32Query(r *http.Request, key string) uint32 {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0
	}
	n, _ := strconv.ParseUint(v, 10, 32)
	return uint32(n)
}

func contractFromStore(c store.Contract) contractResponse {
	return contractResponse{
		ID:                 c.ID,
		Network:            c.Network,
		Label:              c.Label,
		WasmHash:           c.WasmHash,
		CreatedAtLedger:    c.CreatedAtLedger,
		BackfillCompleteAt: c.BackfillCompleteAt,
		Status:             c.Status,
		AddedAt:            c.AddedAt,
	}
}

func eventFromStore(e store.Event) eventResponse {
	return eventResponse{
		ID:               e.ID,
		ContractID:       e.ContractID,
		Ledger:           e.Ledger,
		LedgerClosedAt:   e.LedgerClosedAt,
		TxHash:           e.TxHash,
		Type:             e.Type,
		TopicXDR:         e.TopicXDR,
		ValueXDR:         e.ValueXDR,
		TopicDecoded:     e.TopicDecoded,
		ValueDecoded:     e.ValueDecoded,
		InSuccessfulCall: e.InSuccessfulCall,
	}
}

func invocationFromStore(inv store.Invocation) invocationResponse {
	return invocationResponse{
		TxHash:             inv.TxHash,
		ContractID:         inv.ContractID,
		Ledger:             inv.Ledger,
		LedgerClosedAt:     inv.LedgerClosedAt,
		Status:             inv.Status,
		FunctionName:       inv.FunctionName,
		ArgsDecoded:        inv.ArgsDecoded,
		ResultDecoded:      inv.ResultDecoded,
		ResultXDR:          inv.ResultXDR,
		ResourceFeeCharged: inv.ResourceFeeCharged,
		CPUInsn:            inv.CPUInsn,
		MemByte:            inv.MemByte,
		LedgerReadByte:     inv.LedgerReadByte,
		LedgerWriteByte:    inv.LedgerWriteByte,
		ApplicationOrder:   inv.ApplicationOrder,
	}
}

func storageEntryFromStore(se store.StorageEntry) storageEntryResponse {
	return storageEntryResponse{
		ContractID:         se.ContractID,
		KeyXDR:             se.KeyXDR,
		KeyDecoded:         se.KeyDecoded,
		ValueXDR:           se.ValueXDR,
		ValueDecoded:       se.ValueDecoded,
		Durability:         se.Durability,
		LiveUntilLedger:    se.LiveUntilLedger,
		LastModifiedLedger: se.LastModifiedLedger,
		Status:             se.Status,
		LastSeenAt:         se.LastSeenAt,
	}
}

var validNetworks = map[string]bool{
	"testnet":    true,
	"mainnet":    true,
	"futurenet":  true,
	"standalone": true,
}

func validateContractID(id string) bool {
	return len(id) == 56 && strings.HasPrefix(id, "C")
}

// ---- handlers ---------------------------------------------------------------

type registerRequest struct {
	ID      string `json:"id"`
	Network string `json:"network"`
	Label   string `json:"label"`
}

// RegisterContract handles POST /api/v1/contracts.
func (h *Handler) RegisterContract(w http.ResponseWriter, r *http.Request) {
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
		h.Logger.Error("upsert contract", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to register contract")
		return
	}
	writeJSON(w, http.StatusCreated, contractFromStore(c))
}

// ListContracts handles GET /api/v1/contracts.
func (h *Handler) ListContracts(w http.ResponseWriter, r *http.Request) {
	rawCursor, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	limit := intQuery(r, "limit", 50)

	contracts, nextRaw, err := h.Store.ListContracts(r.Context(), rawCursor, limit)
	if err != nil {
		h.Logger.Error("list contracts", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list contracts")
		return
	}

	resp := make([]contractResponse, len(contracts))
	for i, c := range contracts {
		resp[i] = contractFromStore(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"contracts":   resp,
		"next_cursor": encodeCursor(nextRaw),
	})
}

// GetContract handles GET /api/v1/contracts/{id}.
func (h *Handler) GetContract(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c, err := h.Store.GetContract(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
		return
	}
	if err != nil {
		h.Logger.Error("get contract", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch contract")
		return
	}
	writeJSON(w, http.StatusOK, contractFromStore(c))
}

// ListEvents handles GET /api/v1/contracts/{id}/events.
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	rawCursor, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	f := store.EventFilters{
		Type: r.URL.Query().Get("type"),
		From: uint32Query(r, "from"),
		To:   uint32Query(r, "to"),
	}
	events, nextRaw, err := h.Store.ListEvents(r.Context(), contractID, rawCursor, intQuery(r, "limit", 50), f)
	if err != nil {
		h.Logger.Error("list events", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list events")
		return
	}
	resp := make([]eventResponse, len(events))
	for i, e := range events {
		resp[i] = eventFromStore(e)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events":      resp,
		"next_cursor": encodeCursor(nextRaw),
	})
}

// ListInvocations handles GET /api/v1/contracts/{id}/invocations.
func (h *Handler) ListInvocations(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	rawCursor, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	f := store.InvocationFilters{
		Status:       r.URL.Query().Get("status"),
		FunctionName: r.URL.Query().Get("fn"),
		From:         uint32Query(r, "from"),
		To:           uint32Query(r, "to"),
	}
	invs, nextRaw, err := h.Store.ListInvocations(r.Context(), contractID, rawCursor, intQuery(r, "limit", 50), f)
	if err != nil {
		h.Logger.Error("list invocations", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list invocations")
		return
	}
	resp := make([]invocationResponse, len(invs))
	for i, inv := range invs {
		resp[i] = invocationFromStore(inv)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"invocations": resp,
		"next_cursor": encodeCursor(nextRaw),
	})
}

// ListStorageEntries handles GET /api/v1/contracts/{id}/storage.
func (h *Handler) ListStorageEntries(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	rawCursor, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	f := store.StorageFilters{
		Durability: r.URL.Query().Get("durability"),
		Status:     r.URL.Query().Get("status"),
	}
	entries, nextRaw, err := h.Store.ListStorageEntries(r.Context(), contractID, rawCursor, intQuery(r, "limit", 50), f)
	if err != nil {
		h.Logger.Error("list storage entries", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list storage entries")
		return
	}
	resp := make([]storageEntryResponse, len(entries))
	for i, se := range entries {
		resp[i] = storageEntryFromStore(se)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"storage":     resp,
		"next_cursor": encodeCursor(nextRaw),
	})
}

// ContractStats handles GET /api/v1/contracts/{id}/stats.
func (h *Handler) ContractStats(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	window := r.URL.Query().Get("window")
	if window == "" {
		window = "24h"
	}

	cs, err := h.Store.GetContractStats(r.Context(), contractID, window)
	if err != nil {
		h.Logger.Error("get contract stats", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch stats")
		return
	}
	writeJSON(w, http.StatusOK, contractStatsResponse{
		EventCount:            cs.EventCount,
		InvocationCount:       cs.InvocationCount,
		StorageCount:          cs.StorageCount,
		LastSyncedLedger:      cs.LastSyncedLedger,
		WindowEventCount:      cs.WindowEventCount,
		WindowInvocationCount: cs.WindowInvocationCount,
		WindowDuration:        cs.WindowDuration,
	})
}

// StreamEvents handles GET /api/v1/contracts/{id}/stream.
// Returns the 20 most recent events. Vercel serverless functions do not support
// long-lived connections, so this endpoint uses polling instead of SSE.
func (h *Handler) StreamEvents(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	events, err := h.Store.RecentEvents(r.Context(), contractID, 20)
	if err != nil {
		h.Logger.Error("stream events", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch events")
		return
	}
	resp := make([]eventResponse, len(events))
	for i, e := range events {
		resp[i] = eventFromStore(e)
	}
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	writeJSON(w, http.StatusOK, map[string]any{"events": resp})
}
