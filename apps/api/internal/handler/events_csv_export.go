package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Contract events CSV export.
//
// The export is the flat, spreadsheet-shaped counterpart of
// GET /api/v1/contracts/{id}/events: same rows, same filters, no cursor
// pagination. Rows are streamed straight from the query into the response
// writer, so exporting a contract with millions of events costs a constant
// amount of memory in the API process.
//
// Two consequences of streaming are deliberate and documented here rather than
// worked around:
//
//   - The response is committed as soon as the first byte is written, so a
//     failure part-way through cannot be turned into an error status. It is
//     logged instead. A caller that receives a CSV should check the row count
//     against the contract's event count when completeness matters.
//   - The export is unbounded. That is the point of an export, and the route
//     sits behind the same rate limiter as every other read.
func (h *Handler) ExportEventsCSV(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	from, to, ok := ledgerRangeQuery(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "from and to must be positive integers and from must not exceed to")
		return
	}
	f := store.EventFilters{
		Type:    r.URL.Query().Get("type"),
		Network: network,
		From:    from,
		To:      to,
	}

	// Content-Disposition carries the contract id, which is attacker-controlled
	// on a read: eventsCSVFilename keeps the header well-formed.
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", eventsCSVFilename(contractID)))
	w.Header().Set("Cache-Control", "no-store")

	cw := &committingWriter{w: w}
	if err := h.Store.StreamEventsCSV(r.Context(), contractID, f, cw); err != nil {
		h.Logger.Error("export events csv", "err", err, "contract_id", contractID)
		// Nothing has reached the client yet, so the status code is still ours
		// to choose and a JSON error is a better answer than a header-only
		// file that looks like a contract with no events.
		if !cw.written {
			w.Header().Del("Content-Disposition")
			writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to export events")
		}
	}
}

// committingWriter tracks whether the response has been committed, which is
// what decides if a streaming failure can still be reported as an error status.
// Counting bytes rather than tracking WriteHeader calls is deliberate: the
// handler does not write a status up front, and a zero-length Write is
// ambiguous, so "has any byte reached the client" is the condition that
// actually matters.
//
// Note the CSV writer itself buffers, so rows reach the client in ~4 KiB
// chunks rather than one row at a time. That is the right granularity: memory
// stays constant without paying a write syscall per event.
type committingWriter struct {
	w       http.ResponseWriter
	written bool
}

func (c *committingWriter) Write(p []byte) (int, error) {
	if len(p) > 0 {
		c.written = true
	}
	return c.w.Write(p)
}

// ledgerRangeQuery reads the optional ?from=/?to= ledger bounds. It reports
// ok=false when either bound is present but not a positive integer, or when
// from is greater than to — a range that can never match is a caller mistake
// worth surfacing rather than silently answering with an empty file.
func ledgerRangeQuery(r *http.Request) (from, to uint32, ok bool) {
	var err error
	if from, err = optionalLedgerQuery(r, "from"); err != nil {
		return 0, 0, false
	}
	if to, err = optionalLedgerQuery(r, "to"); err != nil {
		return 0, 0, false
	}
	if from != 0 && to != 0 && from > to {
		return 0, 0, false
	}
	return from, to, true
}

// optionalLedgerQuery parses one ledger bound. An absent or empty value means
// "unbounded" and is reported as 0, matching the store's filter convention.
func optionalLedgerQuery(r *http.Request, key string) (uint32, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return uint32(n), nil
}

// eventsCSVFilename builds the download name for an event export. The contract
// ID embedded in it is attacker-controlled on reads, so anything outside the
// base64-ish contract alphabet is replaced to keep the Content-Disposition
// header well-formed: a quote would otherwise terminate the filename early and
// a path separator would let a caller pick where the file lands. The mapping
// mirrors snapshotFilename in snapshot_export.go rather than sharing a helper,
// so this change stays inside the CSV-export issue's file scope.
func eventsCSVFilename(contractID string) string {
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
	return safe + "-events.csv"
}
