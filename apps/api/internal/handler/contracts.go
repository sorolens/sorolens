package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/forecast"
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

type contractListResponse struct {
	ID             string     `json:"id"`
	Network        string     `json:"network"`
	Label          string     `json:"label"`
	WasmHash       string     `json:"wasm_hash"`
	Status         string     `json:"status"`
	AddedAt        time.Time  `json:"added_at"`
	LastActivityAt *time.Time `json:"last_activity_at"`
}

type eventResponse struct {
	ID               string    `json:"id"`
	ContractID       string    `json:"contract_id"`
	Network          string    `json:"network"`
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
	Network            string         `json:"network"`
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
	Network            string    `json:"network"`
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

type storageChangeResponse struct {
	KeyXDR             string                `json:"key_xdr"`
	KeyDecoded         any                   `json:"key_decoded"`
	Kind               string                `json:"kind"`
	Durability         string                `json:"durability"`
	ChangedFields      []string              `json:"changed_fields,omitempty"`
	Before             *storageEntryResponse `json:"before"`
	After              *storageEntryResponse `json:"after"`
	LastModifiedLedger int64                 `json:"last_modified_ledger"`
}

type storageDiffResponse struct {
	ContractID string                  `json:"contract_id"`
	FromLedger uint32                  `json:"from_ledger"`
	ToLedger   uint32                  `json:"to_ledger"`
	Counts     map[string]int          `json:"counts"`
	Changes    []storageChangeResponse `json:"changes"`
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

// networkParam reads the optional ?network= filter. It returns ("", true) when
// the parameter is absent or empty, and ok=false when the value is not a
// recognized network.
func networkParam(r *http.Request) (string, bool) {
	v := strings.TrimSpace(r.URL.Query().Get("network"))
	if v == "" || v == "all" {
		return "", true
	}
	if !validNetworks[v] {
		return "", false
	}
	return v, true
}

// contractSortParam reads the optional ?sort= and ?dir= filters for the
// contracts list. Unknown sort columns and directions are rejected so the
// handler can respond 422 instead of silently falling back to the default.
func contractSortParam(r *http.Request) (sort, dir string, ok bool) {
	sort = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	dir = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("dir")))
	if sort != "" && !store.ValidContractSort(sort) {
		return "", "", false
	}
	if dir != "" && dir != "asc" && dir != "desc" {
		return "", "", false
	}
	return sort, dir, true
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

func contractListFromStore(c store.Contract) contractListResponse {
	return contractListResponse{
		ID:             c.ID,
		Network:        c.Network,
		Label:          c.Label,
		WasmHash:       c.WasmHash,
		Status:         c.Status,
		AddedAt:        c.AddedAt,
		LastActivityAt: c.LastActivityAt,
	}
}

func eventFromStore(e store.Event) eventResponse {
	return eventResponse{
		ID:               e.ID,
		ContractID:       e.ContractID,
		Network:          e.Network,
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
		Network:            inv.Network,
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
		Network:            se.Network,
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

func storageDiffFromStore(d store.StorageDiff) storageDiffResponse {
	resp := storageDiffResponse{
		ContractID: d.ContractID,
		FromLedger: d.From,
		ToLedger:   d.To,
		Counts:     d.Counts,
		Changes:    make([]storageChangeResponse, 0, len(d.Changes)),
	}
	if resp.Counts == nil {
		resp.Counts = map[string]int{}
	}
	for _, c := range d.Changes {
		change := storageChangeResponse{
			KeyXDR:             c.KeyXDR,
			KeyDecoded:         c.KeyDecoded,
			Kind:               string(c.Kind),
			Durability:         c.Durability,
			ChangedFields:      c.ChangedFields,
			LastModifiedLedger: c.LastModifiedLedger,
		}
		if c.Before != nil {
			before := storageEntryFromStore(*c.Before)
			change.Before = &before
		}
		if c.After != nil {
			after := storageEntryFromStore(*c.After)
			change.After = &after
		}
		resp.Changes = append(resp.Changes, change)
	}
	return resp
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
//
// Query params: cursor, limit, network (testnet|mainnet|futurenet), status,
// sort (id|label|network|status|added_at), dir (asc|desc).
func (h *Handler) ListContracts(w http.ResponseWriter, r *http.Request) {
	rawCursor, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	sort, dir, ok := contractSortParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "sort must be one of: id, label, network, status, added_at; dir must be asc or desc")
		return
	}
	limit := intQuery(r, "limit", 50)
	f := store.ContractFilters{
		Network: network,
		Status:  r.URL.Query().Get("status"),
		Sort:    sort,
		SortDir: dir,
	}

	contracts, nextRaw, err := h.Store.ListContracts(r.Context(), rawCursor, limit, f)
	if err != nil {
		h.Logger.Error("list contracts", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list contracts")
		return
	}

	resp := make([]contractListResponse, len(contracts))
	for i, c := range contracts {
		resp[i] = contractListFromStore(c)
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
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	var inSuccess *bool
	if param := r.URL.Query().Get("in_successful_call"); param != "" {
		if param == "true" {
			v := true
			inSuccess = &v
		} else if param == "false" {
			v := false
			inSuccess = &v
		}
	}
	f := store.EventFilters{
		Type:             r.URL.Query().Get("type"),
		Network:          network,
		Topic:            strings.TrimSpace(r.URL.Query().Get("topic")),
		From:             uint32Query(r, "from"),
		To:               uint32Query(r, "to"),
		InSuccessfulCall: inSuccess,
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

// ExportEventsCSV handles GET /api/v1/contracts/{id}/events.csv.
// Streams events as CSV without buffering all rows in memory.
func (h *Handler) ExportEventsCSV(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	f := store.EventFilters{
		Type:    r.URL.Query().Get("type"),
		Network: network,
		From:    uint32Query(r, "from"),
		To:      uint32Query(r, "to"),
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-events.csv\"", contractID))
	w.Header().Set("Cache-Control", "no-store")

	if err := h.Store.StreamEventsCSV(r.Context(), contractID, f, w); err != nil {
		h.Logger.Error("export events csv", "err", err)
	}
}

// ListInvocations handles GET /api/v1/contracts/{id}/invocations.
func (h *Handler) ListInvocations(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	rawCursor, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	f := store.InvocationFilters{
		Status:       r.URL.Query().Get("status"),
		FunctionName: r.URL.Query().Get("function_name"),
		Network:      network,
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

// ---- global invocations -----------------------------------------------------

// parseInvocationCursor decodes the raw (already base64-decoded) global
// invocation cursor "<ledger>:<tx_hash>" into its keyset components. An empty
// cursor means "start at the newest invocation".
func parseInvocationCursor(raw string) (uint32, string, bool) {
	if raw == "" {
		return 0, "", true
	}
	ledgerStr, txHash, found := strings.Cut(raw, ":")
	if !found || txHash == "" {
		return 0, "", false
	}
	n, err := strconv.ParseUint(ledgerStr, 10, 32)
	if err != nil || n == 0 {
		return 0, "", false
	}
	return uint32(n), txHash, true
}

// timeParam parses an inclusive date or RFC3339 bound. A plain date is expanded
// to the start (or, when endOfDay is set, the end) of that UTC day so that
// ?since=2026-09-01&until=2026-09-30 covers the whole range.
func timeParam(r *http.Request, key string, endOfDay bool) (*time.Time, bool) {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return nil, true
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		t = t.UTC()
		return &t, true
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		if endOfDay {
			t = t.Add(24*time.Hour - time.Nanosecond)
		}
		return &t, true
	}
	return nil, false
}

// ListAllInvocations handles GET /api/v1/invocations: the global invocation
// explorer. It returns invocations across every tracked contract, newest
// first (ledger DESC, tx_hash DESC).
//
// Query params: cursor, limit, contract_id, fn, status, network, from/to
// (ledger bounds), and since/until (inclusive date or RFC3339 bounds on
// ledger_closed_at).
func (h *Handler) ListAllInvocations(w http.ResponseWriter, r *http.Request) {
	rawCursor, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	cursorLedger, cursorTxHash, ok := parseInvocationCursor(rawCursor)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	since, ok := timeParam(r, "since", false)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "since must be an RFC3339 timestamp or YYYY-MM-DD date")
		return
	}
	until, ok := timeParam(r, "until", true)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "until must be an RFC3339 timestamp or YYYY-MM-DD date")
		return
	}

	f := store.InvocationFilters{
		ContractID:   strings.TrimSpace(r.URL.Query().Get("contract_id")),
		Status:       r.URL.Query().Get("status"),
		FunctionName: r.URL.Query().Get("fn"),
		Network:      network,
		From:         uint32Query(r, "from"),
		To:           uint32Query(r, "to"),
		Since:        since,
		Until:        until,
	}
	invs, nextLedger, nextTxHash, err := h.Store.ListAllInvocations(
		r.Context(), cursorLedger, cursorTxHash, intQuery(r, "limit", 50), f,
	)
	if err != nil {
		h.Logger.Error("list all invocations", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list invocations")
		return
	}

	resp := make([]invocationResponse, len(invs))
	for i, inv := range invs {
		resp[i] = invocationFromStore(inv)
	}
	nextRaw := ""
	if nextTxHash != "" {
		nextRaw = fmt.Sprintf("%d:%s", nextLedger, nextTxHash)
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

// ContractStorageDiff handles GET /api/v1/contracts/{id}/storage/diff?from=N&to=M.
//
// It compares the contract's storage state at two ledgers and returns every
// key that was created, updated, deleted, or expired in between — the storage
// equivalent of `git diff`. Temporary, persistent, and instance entries are
// all covered because the diff is computed over full per-key snapshots.
func (h *Handler) ContractStorageDiff(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	fromStr := strings.TrimSpace(r.URL.Query().Get("from"))
	toStr := strings.TrimSpace(r.URL.Query().Get("to"))
	if fromStr == "" || toStr == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "from and to query parameters are required")
		return
	}
	fromU64, err := strconv.ParseUint(fromStr, 10, 32)
	if err != nil || fromU64 == 0 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "from must be a positive integer")
		return
	}
	toU64, err := strconv.ParseUint(toStr, 10, 32)
	if err != nil || toU64 == 0 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "to must be a positive integer")
		return
	}
	if fromU64 > toU64 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "from must be less than or equal to to")
		return
	}

	if _, err := h.Store.GetContract(r.Context(), contractID); errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
		return
	} else if err != nil {
		h.Logger.Error("storage diff get contract", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch contract")
		return
	}

	diff, err := h.Store.GetStorageDiff(r.Context(), contractID, uint32(fromU64), uint32(toU64))
	if err != nil {
		h.Logger.Error("storage diff", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to build storage diff")
		return
	}
	writeJSON(w, http.StatusOK, storageDiffFromStore(diff))
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

// ---- cost forecasting -------------------------------------------------------

type forecastPointResponse struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
	Lower float64 `json:"lower"`
	Upper float64 `json:"upper"`
}

type forecastSeriesResponse struct {
	Metric     string                  `json:"metric"`
	Horizon    int                     `json:"horizon"`
	DailyCount int                     `json:"daily_count"`
	Points     []forecastPointResponse `json:"points"`
}

type forecastResponse struct {
	ContractID   string                   `json:"contract_id"`
	LookbackDays int                      `json:"lookback_days"`
	Series       []forecastSeriesResponse `json:"series"`
}

// ContractForecast returns projected fees, invocations, and event volume for
// the next N days (horizon, default 30d) by fitting a linear trend + weekly
// seasonality model over the last 90 days of daily aggregates.
func (h *Handler) ContractForecast(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	horizon := 30
	if hv := r.URL.Query().Get("horizon"); hv != "" {
		n, err := strconv.Atoi(strings.TrimSuffix(hv, "d"))
		if err != nil || n <= 0 {
			writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "horizon must be a positive number of days, e.g. horizon=30d")
			return
		}
		horizon = n
	}
	if horizon > 365 {
		horizon = 365
	}

	const lookback = 90
	aggs, err := h.Store.DailyAggregates(r.Context(), contractID, lookback)
	if err != nil {
		h.Logger.Error("get daily aggregates", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch daily aggregates")
		return
	}

	serieses := []struct {
		metric string
		pick   func(store.DailyAggregate) float64
	}{
		{"fees", func(a store.DailyAggregate) float64 { return a.Fee }},
		{"invocations", func(a store.DailyAggregate) float64 { return a.Invocations }},
		{"events", func(a store.DailyAggregate) float64 { return a.Events }},
	}

	resp := forecastResponse{
		ContractID:   contractID,
		LookbackDays: lookback,
		Series:       make([]forecastSeriesResponse, 0, len(serieses)),
	}
	for _, s := range serieses {
		history := make([]forecast.DayValue, 0, len(aggs))
		for _, a := range aggs {
			history = append(history, forecast.DayValue{Date: a.Day, Value: s.pick(a)})
		}
		preds := forecast.Fit(history, horizon)
		pts := make([]forecastPointResponse, 0, len(preds))
		for _, p := range preds {
			pts = append(pts, forecastPointResponse{
				Date:  p.Date.Format("2006-01-02"),
				Value: mathRound(p.Value),
				Lower: mathRound(p.Lower),
				Upper: mathRound(p.Upper),
			})
		}
		resp.Series = append(resp.Series, forecastSeriesResponse{
			Metric:     s.metric,
			Horizon:    horizon,
			DailyCount: len(pts),
			Points:     pts,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func mathRound(v float64) float64 {
	if v > 1e-5 {
		return math.Round(v*100) / 100
	}
	return 0
}

// ---- snapshot / replay ------------------------------------------------------

type snapshotResponse struct {
	ContractID         string                 `json:"contract_id"`
	Network            string                 `json:"network"`
	Ledger             uint32                 `json:"ledger"`
	FirstTrackedLedger uint32                 `json:"first_tracked_ledger"`
	Storage            []storageEntryResponse `json:"storage"`
	LastEvent          *eventResponse         `json:"last_event"`
}

// ContractSnapshot handles GET /api/v1/contracts/{id}/snapshot?ledger=N.
//
// It replays the contract's storage state and last known event as they were
// at ledger N, so an operator can inspect "what did the contract look like
// immediately before?" without reading raw ledger data.
func (h *Handler) ContractSnapshot(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	ledgerStr := strings.TrimSpace(r.URL.Query().Get("ledger"))
	if ledgerStr == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "ledger query parameter is required")
		return
	}
	ledgerU64, err := strconv.ParseUint(ledgerStr, 10, 32)
	if err != nil || ledgerU64 == 0 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "ledger must be a positive integer")
		return
	}
	ledger := uint32(ledgerU64)

	contract, err := h.Store.GetContract(r.Context(), contractID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
		return
	}
	if err != nil {
		h.Logger.Error("snapshot get contract", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch contract")
		return
	}

	first, err := h.Store.ContractFirstLedger(r.Context(), contractID)
	if err != nil {
		h.Logger.Error("snapshot first ledger", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to compute snapshot range")
		return
	}
	if first == 0 && contract.CreatedAtLedger > 0 && contract.CreatedAtLedger <= int64(^uint32(0)) {
		first = uint32(contract.CreatedAtLedger)
	}
	if first > 0 && ledger < first {
		writeError(w, r, http.StatusNotFound, CodeNotFound, fmt.Sprintf(
			"no snapshot for ledger %d: contract %s was first tracked at ledger %d",
			ledger, contractID, first))
		return
	}

	entries, err := h.Store.GetStorageSnapshot(r.Context(), contractID, ledger)
	if err != nil {
		h.Logger.Error("snapshot storage", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to build snapshot")
		return
	}

	var lastEvent *eventResponse
	if e, err := h.Store.LastEventAtOrBefore(r.Context(), contractID, ledger); err == nil {
		ev := eventFromStore(e)
		lastEvent = &ev
	} else if !errors.Is(err, store.ErrNotFound) {
		h.Logger.Error("snapshot last event", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to build snapshot")
		return
	}

	resp := make([]storageEntryResponse, len(entries))
	for i, se := range entries {
		resp[i] = storageEntryFromStore(se)
	}
	writeJSON(w, http.StatusOK, snapshotResponse{
		ContractID:         contractID,
		Network:            contract.Network,
		Ledger:             ledger,
		FirstTrackedLedger: first,
		Storage:            resp,
		LastEvent:          lastEvent,
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

func (h *Handler) SearchContracts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := intQuery(r, "limit", 10)

	contracts, err := h.Store.SearchContracts(r.Context(), q, limit)
	if err != nil {
		h.Logger.Error("search contracts", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to search contracts")
		return
	}

	if contracts == nil {
		contracts = []store.Contract{}
	}

	var res []contractResponse
	for _, c := range contracts {
		res = append(res, contractResponse{
			ID:                 c.ID,
			Network:            c.Network,
			Label:              c.Label,
			WasmHash:           c.WasmHash,
			CreatedAtLedger:    c.CreatedAtLedger,
			BackfillCompleteAt: c.BackfillCompleteAt,
			Status:             c.Status,
			AddedAt:            c.AddedAt,
		})
	}

	// API typically returns a list of results wrapped or just the array.
	// We'll return an array directly for simplicity or wrapped in { results: ... } if preferred.
	// Looking at other routes, List returns { items: [...] }. But a simple array is fine too.
	// Wait, List returns { items: [...], next_cursor: ... }. Let's return { items: [...] } for consistency.
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": res,
	})
}
