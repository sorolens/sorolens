package store

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"time"
)

// Contract events are exported as CSV by StreamEventsCSV. The rendering lives
// here, shared by the Postgres and in-memory backends, so both produce
// byte-identical files for the same event and the export stays a stable
// contract for consumers instead of an accident of each backend's code.
//
// Two properties matter:
//
//   - Determinism: rows are ordered by (ledger, id) ascending and every field
//     is rendered the same way, so exporting an unchanged store twice yields
//     identical bytes and a diff means the data changed.
//   - No spreadsheet formula execution: see csvText and csvJSON.
var eventsCSVColumns = []string{
	"id",
	"contract_id",
	"network",
	"ledger",
	"ledger_closed_at",
	"tx_hash",
	"type",
	"topic_xdr",
	"value_xdr",
	"topic_decoded",
	"value_decoded",
	"in_successful_call",
}

// newEventsCSVWriter returns a CSV writer over w with the column header
// already written, so a caller only has to emit rows.
func newEventsCSVWriter(w io.Writer) (*csv.Writer, error) {
	cw := csv.NewWriter(w)
	if err := cw.Write(eventsCSVColumns); err != nil {
		return nil, err
	}
	return cw, nil
}

// flushEventsCSV drains the CSV writer and records a failure in err when the
// caller has not already reported one. csv.Writer buffers, so a failing
// destination usually only surfaces during the final Flush: reading
// csv.Writer.Error() before that Flush returns nil and would report a
// truncated download as a success.
func flushEventsCSV(cw *csv.Writer, err *error) {
	cw.Flush()
	if *err == nil {
		*err = cw.Error()
	}
}

// eventJSON encodes a decoded event field the way the Postgres backend stores
// it, so an in-memory export matches one taken from the database.
func eventJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

// eventCSVRecord renders one event as a CSV row of len(eventsCSVColumns)
// fields. topicXDR, topicDecoded and valueDecoded are the JSON encodings of
// those fields: the Postgres path hands over the stored JSON text verbatim
// while the in-memory path re-encodes the decoded values it holds.
//
// The slice is freshly allocated, so callers may keep or reuse their own
// reference; csv.Writer copies every field into its own buffer regardless.
func eventCSVRecord(e Event, topicXDR, topicDecoded, valueDecoded string) []string {
	row := make([]string, len(eventsCSVColumns))
	row[0] = e.ID
	row[1] = e.ContractID
	row[2] = e.Network
	row[3] = strconv.FormatUint(uint64(e.Ledger), 10)
	row[4] = formatCSVTime(e.LedgerClosedAt)
	row[5] = e.TxHash
	row[6] = csvText(e.Type)
	row[7] = topicXDR
	row[8] = csvText(e.ValueXDR)
	row[9] = topicDecoded
	row[10] = valueDecoded
	row[11] = strconv.FormatBool(e.InSuccessfulCall)
	return row
}

func formatCSVTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// csvText neutralizes spreadsheet formula injection in a free-text column.
// Excel and LibreOffice evaluate a cell that starts with =, +, - or @ as a
// formula, so a contract that emits an event named "=cmd|'/c calc'!A1" would
// otherwise run on the machine of whoever opens the export. A leading
// apostrophe forces the cell to be read as text; spreadsheets drop it on CSV
// import, so the cell still displays the original value.
//
// Only the free-text columns need this. The remaining ones cannot open with a
// formula lead: id, contract_id and tx_hash are strkeys and hashes, network is
// a fixed enum, and the three JSON columns are always a document or a scalar
// that begins with [ { " a digit or -, none of which a spreadsheet evaluates
// as a formula. Guarding those would only risk corrupting a negative number
// into text.
func csvText(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	default:
		return s
	}
}

// matchesEventFilters reports whether an event passes the export filters. It is
// the in-memory counterpart of the StreamEventsCSV SQL predicate and must stay
// in step with it.
func matchesEventFilters(e Event, f EventFilters) bool {
	if f.Network != "" && e.Network != f.Network {
		return false
	}
	if f.Type != "" && e.Type != f.Type {
		return false
	}
	if f.From != 0 && e.Ledger < f.From {
		return false
	}
	if f.To != 0 && e.Ledger > f.To {
		return false
	}
	return true
}

// lessEventOrder mirrors the export query's "ORDER BY ledger ASC, id ASC".
func lessEventOrder(a, b Event) bool {
	if a.Ledger != b.Ledger {
		return a.Ledger < b.Ledger
	}
	return a.ID < b.ID
}
