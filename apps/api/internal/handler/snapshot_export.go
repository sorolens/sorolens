package handler

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Snapshot export contract.
//
// The export is a self-describing, versioned JSON document. Its shape is
// frozen by snapshotSchemaVersion; consumers should branch on that field
// rather than assuming the current field set.
//
// Determinism guarantees (so two exports of an unchanged store are
// byte-identical and can be diffed):
//   - fields are emitted in a fixed order (struct declaration order);
//   - the storage and events collections are explicitly sorted;
//   - no wall-clock timestamps are included: the document is keyed by the
//     observability ledger, not by "now".
//
// Omission rules:
//   - collections are always emitted, as [] when empty;
//   - nullable contract timestamps are emitted as null when unset;
//   - summary fields that have no value (no events, no truncation) are
//     omitted rather than emitted as zero values.
const (
	// snapshotSchemaVersion identifies the JSON snapshot export shape.
	snapshotSchemaVersion = 1

	// snapshotEventLimit caps the "recent events" section.
	snapshotEventLimit = 50
	// snapshotStorageLimit caps the storage section so a runaway contract
	// cannot produce an unbounded export.
	snapshotStorageLimit = 1000
	// snapshotPageSize is the page size used to walk storage entries.
	snapshotPageSize = 200
)

// snapshotSummary is the fourth section: counts and the identity of the most
// recent event, derived from the other three sections.
type snapshotSummary struct {
	StorageCount       int    `json:"storage_count"`
	EventCount         int    `json:"event_count"`
	FirstTrackedLedger uint32 `json:"first_tracked_ledger"`
	// LastEventID and LastEventLedger describe the newest event in the
	// events section and are omitted when the contract has no events.
	LastEventID     string `json:"last_event_id,omitempty"`
	LastEventLedger uint32 `json:"last_event_ledger,omitempty"`
	// StorageTruncated is present (true) only when the storage section hit
	// snapshotStorageLimit and does not represent the full key set.
	StorageTruncated bool `json:"storage_truncated,omitempty"`
}

// contractSnapshotExport is the top-level JSON snapshot export document.
//
// The four content sections are metadata, storage, events, and summary.
type contractSnapshotExport struct {
	SchemaVersion int                    `json:"schema_version"`
	ContractID    string                 `json:"contract_id"`
	Network       string                 `json:"network"`
	Ledger        uint32                 `json:"ledger"`
	Metadata      contractResponse       `json:"metadata"`
	Storage       []storageEntryResponse `json:"storage"`
	Events        []eventResponse        `json:"events"`
	Summary       snapshotSummary        `json:"summary"`
}

// ContractSnapshotExport handles GET /api/v1/contracts/{id}/snapshot.json.
//
// It returns the contract's current state as one portable JSON blob: contract
// metadata, live storage entries, the most recent events, and a summary. The
// response is served as a downloadable attachment and is gzip-compressed when
// the client sends Accept-Encoding: gzip.
func (h *Handler) ContractSnapshotExport(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	contract, err := h.Store.GetContract(r.Context(), contractID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
		return
	}
	if err != nil {
		h.Logger.Error("snapshot export get contract", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch contract")
		return
	}

	storage, truncated, err := h.snapshotStorage(r.Context(), contractID)
	if err != nil {
		h.Logger.Error("snapshot export storage", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to build snapshot")
		return
	}

	events, err := h.Store.RecentEvents(r.Context(), contractID, snapshotEventLimit)
	if err != nil {
		h.Logger.Error("snapshot export events", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to build snapshot")
		return
	}
	// RecentEvents is newest-first in SQL, but sort explicitly so the export
	// is deterministic regardless of backing store ordering.
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Ledger != events[j].Ledger {
			return events[i].Ledger > events[j].Ledger
		}
		return events[i].ID > events[j].ID
	})

	first, err := h.Store.ContractFirstLedger(r.Context(), contractID)
	if err != nil {
		h.Logger.Error("snapshot export first ledger", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to build snapshot")
		return
	}
	ledger := snapshotLedger(r.Context(), h, contract, first, events)

	storageResp := make([]storageEntryResponse, len(storage))
	for i, se := range storage {
		storageResp[i] = storageEntryFromStore(se)
	}
	eventsResp := make([]eventResponse, len(events))
	for i, e := range events {
		eventsResp[i] = eventFromStore(e)
	}

	summary := snapshotSummary{
		StorageCount:       len(storageResp),
		EventCount:         len(eventsResp),
		FirstTrackedLedger: first,
		StorageTruncated:   truncated,
	}
	if len(eventsResp) > 0 {
		summary.LastEventID = eventsResp[0].ID
		summary.LastEventLedger = eventsResp[0].Ledger
	}

	doc := contractSnapshotExport{
		SchemaVersion: snapshotSchemaVersion,
		ContractID:    contract.ID,
		Network:       contract.Network,
		Ledger:        ledger,
		Metadata:      contractFromStore(contract),
		Storage:       storageResp,
		Events:        eventsResp,
		Summary:       summary,
	}

	body, err := json.Marshal(doc)
	if err != nil {
		h.Logger.Error("snapshot export marshal", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to encode snapshot")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+snapshotFilename(contractID)+`"`)
	w.Header().Set("Vary", "Accept-Encoding")

	if acceptsGzip(r.Header.Get("Accept-Encoding")) {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		if _, err := gz.Write(body); err != nil {
			return
		}
		_ = gz.Close()
		return
	}
	_, _ = w.Write(body)
}

// snapshotLedger resolves the ledger the export is keyed to: the greatest of
// the first tracked ledger, the contract's recorded creation ledger, the
// indexer's sync cursor, and the newest exported event.
func snapshotLedger(ctx context.Context, h *Handler, contract store.Contract, first uint32, events []store.Event) uint32 {
	ledger := first
	if ledger == 0 && contract.CreatedAtLedger > 0 && contract.CreatedAtLedger <= math.MaxUint32 {
		ledger = uint32(contract.CreatedAtLedger)
	}
	if ss, err := h.Store.GetSyncState(ctx, contract.ID); err == nil && ss.LastLedger > ledger {
		ledger = ss.LastLedger
	}
	if len(events) > 0 && events[0].Ledger > ledger {
		ledger = events[0].Ledger
	}
	return ledger
}

// snapshotStorage returns the current storage state: one entry per key, the
// version with the highest last_modified_ledger winning. Entries are read in
// pages and sorted by key so the section is deterministic. The boolean is true
// when the section was truncated at snapshotStorageLimit.
func (h *Handler) snapshotStorage(ctx context.Context, contractID string) ([]store.StorageEntry, bool, error) {
	var (
		out       []store.StorageEntry
		truncated bool
		cursor    string
	)
	for {
		entries, next, err := h.Store.ListStorageEntries(ctx, contractID, cursor, snapshotPageSize, store.StorageFilters{})
		if err != nil {
			return nil, false, err
		}
		out = append(out, entries...)
		if len(out) >= snapshotStorageLimit {
			out = out[:snapshotStorageLimit]
			truncated = next != ""
			break
		}
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}

	// Collapse history to the live version per key. The mock store keeps every
	// write, and the history table can too, so dedupe here rather than
	// assuming the backing store returns one row per key.
	latest := make(map[string]store.StorageEntry, len(out))
	order := make([]string, 0, len(out))
	for _, se := range out {
		prev, ok := latest[se.KeyXDR]
		if !ok {
			order = append(order, se.KeyXDR)
			latest[se.KeyXDR] = se
			continue
		}
		if se.LastModifiedLedger > prev.LastModifiedLedger {
			latest[se.KeyXDR] = se
		}
	}
	keys := order
	sort.Strings(keys)

	deduped := make([]store.StorageEntry, 0, len(keys))
	for _, k := range keys {
		deduped = append(deduped, latest[k])
	}
	return deduped, truncated, nil
}

// snapshotFilename builds a safe Content-Disposition filename from the
// contract ID. The ID is attacker-controlled on reads, so anything outside
// the base64-ish contract alphabet is replaced to keep the header well-formed.
func snapshotFilename(contractID string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, contractID)
	if safe == "" {
		safe = "contract"
	}
	return safe + "-snapshot.json"
}

// acceptsGzip reports whether the Accept-Encoding header advertises gzip with
// a non-zero quality value.
func acceptsGzip(header string) bool {
	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(part, ";")
		if !strings.EqualFold(strings.TrimSpace(fields[0]), "gzip") {
			continue
		}
		for _, param := range fields[1:] {
			kv := strings.SplitN(strings.TrimSpace(param), "=", 2)
			if len(kv) != 2 || !strings.EqualFold(strings.TrimSpace(kv[0]), "q") {
				continue
			}
			if q, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64); err == nil && q == 0 {
				return false
			}
		}
		return true
	}
	return false
}
