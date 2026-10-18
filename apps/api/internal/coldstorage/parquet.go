// Package coldstorage implements the cold-storage tier for old events (issue
// #146).
//
// Events older than a configurable threshold are exported from Postgres to
// Parquet files in an S3-compatible bucket (AWS S3, MinIO, Backblaze B2, ...)
// and only then deleted from Postgres. The API reads archived events back
// transparently, so a query for an old ledger range still returns data after
// the rows have left the hot store.
//
// # Object layout
//
//	events/<contract-id>/<YYYY-MM>.parquet
//
// One Parquet file per contract per calendar month, keyed off
// ledger_closed_at. Re-archiving the same month merges into the existing file
// and de-duplicates by event id, so the job is safe to run repeatedly.
//
// # Why not embedded DuckDB
//
// The issue suggested loading the Parquet with DuckDB embedded. That requires
// cgo and a multi-minute C++ build in every environment that compiles the API
// (including CI), so this implementation reads the files with the pure-Go
// Parquet reader instead. The on-disk format is identical, so a future
// DuckDB-backed reader can consume the same objects.
package coldstorage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/parquet-go/parquet-go"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// eventRow is the Parquet schema of an archived event. Decoded XDR values are
// stored as JSON strings so the file stays readable by non-Go tools (DuckDB
// sees plain VARCHAR columns rather than an opaque blob).
type eventRow struct {
	ID               string `parquet:"id"`
	ContractID       string `parquet:"contract_id"`
	Network          string `parquet:"network"`
	Ledger           uint32 `parquet:"ledger"`
	LedgerClosedAt   int64  `parquet:"ledger_closed_at"` // unix milliseconds, UTC
	TxHash           string `parquet:"tx_hash"`
	Type             string `parquet:"type"`
	TopicXDR         string `parquet:"topic_xdr"` // JSON array of base64 ScVal
	ValueXDR         string `parquet:"value_xdr"`
	TopicDecoded     string `parquet:"topic_decoded"` // JSON
	ValueDecoded     string `parquet:"value_decoded"` // JSON
	InSuccessfulCall bool   `parquet:"in_successful_call"`
	InsertedAt       int64  `parquet:"inserted_at"` // unix milliseconds, UTC
}

func toRow(e store.Event) (eventRow, error) {
	topicXDR, err := json.Marshal(e.TopicXDR)
	if err != nil {
		return eventRow{}, fmt.Errorf("archive marshal topic_xdr: %w", err)
	}
	topicDec, err := json.Marshal(e.TopicDecoded)
	if err != nil {
		return eventRow{}, fmt.Errorf("archive marshal topic_decoded: %w", err)
	}
	valueDec, err := json.Marshal(e.ValueDecoded)
	if err != nil {
		return eventRow{}, fmt.Errorf("archive marshal value_decoded: %w", err)
	}
	return eventRow{
		ID:               e.ID,
		ContractID:       e.ContractID,
		Network:          e.Network,
		Ledger:           e.Ledger,
		LedgerClosedAt:   e.LedgerClosedAt.UTC().UnixMilli(),
		TxHash:           e.TxHash,
		Type:             e.Type,
		TopicXDR:         string(topicXDR),
		ValueXDR:         e.ValueXDR,
		TopicDecoded:     string(topicDec),
		ValueDecoded:     string(valueDec),
		InSuccessfulCall: e.InSuccessfulCall,
		InsertedAt:       e.InsertedAt.UTC().UnixMilli(),
	}, nil
}

func fromRow(r eventRow) store.Event {
	e := store.Event{
		ID:               r.ID,
		ContractID:       r.ContractID,
		Network:          r.Network,
		Ledger:           r.Ledger,
		LedgerClosedAt:   time.UnixMilli(r.LedgerClosedAt).UTC(),
		TxHash:           r.TxHash,
		Type:             r.Type,
		ValueXDR:         r.ValueXDR,
		InSuccessfulCall: r.InSuccessfulCall,
		InsertedAt:       time.UnixMilli(r.InsertedAt).UTC(),
	}
	// A JSON "null" (or malformed) payload leaves the field at its zero value,
	// matching how a NULL JSONB column reads back from Postgres.
	_ = json.Unmarshal([]byte(r.TopicXDR), &e.TopicXDR)
	_ = json.Unmarshal([]byte(r.TopicDecoded), &e.TopicDecoded)
	_ = json.Unmarshal([]byte(r.ValueDecoded), &e.ValueDecoded)
	return e
}

// EncodeEvents serialises events into an in-memory Parquet file.
func EncodeEvents(events []store.Event) ([]byte, error) {
	rows := make([]eventRow, 0, len(events))
	for _, e := range events {
		r, err := toRow(e)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r)
	}
	var buf bytes.Buffer
	if err := parquet.Write[eventRow](&buf, rows); err != nil {
		return nil, fmt.Errorf("encode parquet: %w", err)
	}
	return buf.Bytes(), nil
}

// DecodeEvents parses a Parquet file produced by EncodeEvents. An empty
// payload decodes to an empty slice rather than an error so a zero-length
// object cannot fail a query.
func DecodeEvents(data []byte) ([]store.Event, error) {
	if len(data) == 0 {
		return nil, nil
	}
	rows, err := parquet.Read[eventRow](bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("decode parquet: %w", err)
	}
	out := make([]store.Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(r))
	}
	return out, nil
}

// MergeEvents combines an existing archived set with newly exported events.
// Rows are de-duplicated by event id (incoming wins), then ordered by ledger
// and id so the Parquet file has a stable, deterministic layout across runs.
func MergeEvents(existing, incoming []store.Event) []store.Event {
	byID := make(map[string]store.Event, len(existing)+len(incoming))
	for _, e := range existing {
		byID[e.ID] = e
	}
	for _, e := range incoming {
		byID[e.ID] = e
	}
	out := make([]store.Event, 0, len(byID))
	for _, e := range byID {
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Ledger != out[j].Ledger {
			return out[i].Ledger < out[j].Ledger
		}
		return out[i].ID < out[j].ID
	})
	return out
}
