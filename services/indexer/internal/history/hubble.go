package history

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

// Hubble is SDF's public BigQuery copy of the Stellar ledger. Its contract
// events table holds every event since Soroban launched, with the full
// ContractEvent XDR, which is what lets an imported event be stored exactly
// like one read from getEvents.
const (
	// DefaultHubbleProject is the public project the dataset lives in.
	DefaultHubbleProject = "crypto-stellar"
	// DefaultHubbleDataset is the mainnet (pubnet) dataset. Hubble publishes
	// no testnet history, so other networks need HubbleConfig.Dataset set to
	// a dataset of the same shape.
	DefaultHubbleDataset = "crypto_stellar"
	// HubbleEventsTable is the contract events table in that dataset.
	HubbleEventsTable = "history_contract_events"
)

// HubbleConfig configures the BigQuery source.
type HubbleConfig struct {
	// BillingProject is the Google Cloud project queries are billed to. It
	// must be one you own: the public dataset's project only stores data.
	BillingProject string
	// Project and Dataset locate the events table. Empty means the public
	// mainnet dataset.
	Project string
	Dataset string
	// MaxBytesBilled caps what one query may scan; BigQuery refuses a query
	// that would scan more, so a mistake cannot run up a bill. 0 means
	// DefaultMaxBytesBilled.
	MaxBytesBilled int64
}

// DefaultMaxBytesBilled caps one batch query at 10 GiB.
const DefaultMaxBytesBilled int64 = 10 << 30

// Table returns the fully qualified events table.
func (c HubbleConfig) Table() string {
	project, dataset := c.Project, c.Dataset
	if project == "" {
		project = DefaultHubbleProject
	}
	if dataset == "" {
		dataset = DefaultHubbleDataset
	}
	return fmt.Sprintf("`%s.%s.%s`", project, dataset, HubbleEventsTable)
}

// Validate checks the configuration before any query runs.
func (c HubbleConfig) Validate(network string) error {
	if c.BillingProject == "" {
		return errors.New("hubble: a billing project is required (-hubble-billing-project or HUBBLE_BILLING_PROJECT)")
	}
	for _, part := range []string{c.Project, c.Dataset} {
		if strings.ContainsAny(part, "`;\\ \t\n") {
			return fmt.Errorf("hubble: invalid project or dataset name %q", part)
		}
	}
	if c.Dataset == "" && network != "" && network != "mainnet" && network != "public" && network != "pubnet" {
		return fmt.Errorf("hubble: the public dataset only holds mainnet history; set a dataset for %s", network)
	}
	return nil
}

// HubbleQuery is the SQL one batch runs. The ledger range is partition
// pruned through closed_at's partitioning, so each batch scans only its own
// days. Only events from successful transactions are imported, matching
// getEvents. event_index numbers a transaction's events in a stable order so
// re-runs produce the same IDs.
func HubbleQuery(cfg HubbleConfig) string {
	return `SELECT
  ledger_sequence,
  closed_at,
  transaction_hash,
  transaction_id,
  ROW_NUMBER() OVER (PARTITION BY transaction_id ORDER BY contract_event_xdr) - 1 AS event_index,
  contract_id,
  contract_event_xdr,
  in_successful_contract_call
FROM ` + cfg.Table() + `
WHERE contract_id = @contract_id
  AND ledger_sequence BETWEEN @from_ledger AND @to_ledger
  AND closed_at BETWEEN @from_time AND @to_time
  AND successful
ORDER BY ledger_sequence, transaction_id, event_index`
}

// HubbleRow is one row of HubbleQuery's result.
type HubbleRow struct {
	LedgerSequence           int64
	ClosedAt                 time.Time
	TransactionHash          string
	TransactionID            int64
	EventIndex               int64
	ContractID               string
	ContractEventXDR         string
	InSuccessfulContractCall bool
}

// EventID builds the ID getEvents gives an event: the TOID of its
// operation, zero padded to 19 digits, and its index in the transaction.
// Hubble's transaction_id is the transaction's TOID, which is also the TOID
// of a Soroban transaction's single operation.
func EventID(transactionID, eventIndex int64) string {
	return fmt.Sprintf("%019d-%010d", transactionID, eventIndex)
}

// eventTypes maps XDR event types to the names getEvents uses.
var eventTypes = map[xdr.ContractEventType]string{
	xdr.ContractEventTypeContract:   "contract",
	xdr.ContractEventTypeSystem:     "system",
	xdr.ContractEventTypeDiagnostic: "diagnostic",
}

// ToEvent turns a Hubble row into the event live indexing would store.
func (r HubbleRow) ToEvent() (poller.Event, error) {
	if r.LedgerSequence <= 0 || r.LedgerSequence > int64(^uint32(0)) {
		return poller.Event{}, fmt.Errorf("hubble row has ledger %d", r.LedgerSequence)
	}
	if r.TransactionID <= 0 || r.EventIndex < 0 {
		return poller.Event{}, fmt.Errorf("hubble row in ledger %d has no transaction id or event index", r.LedgerSequence)
	}
	var ev xdr.ContractEvent
	if err := xdr.SafeUnmarshalBase64(r.ContractEventXDR, &ev); err != nil {
		return poller.Event{}, fmt.Errorf("decode contract event in tx %s: %w", r.TransactionHash, err)
	}
	body, ok := ev.Body.GetV0()
	if !ok {
		return poller.Event{}, fmt.Errorf("contract event in tx %s has an unknown body version", r.TransactionHash)
	}
	topics := make([]string, 0, len(body.Topics))
	for _, t := range body.Topics {
		b64, err := xdr.MarshalBase64(t)
		if err != nil {
			return poller.Event{}, fmt.Errorf("encode topic: %w", err)
		}
		topics = append(topics, b64)
	}
	value, err := xdr.MarshalBase64(body.Data)
	if err != nil {
		return poller.Event{}, fmt.Errorf("encode value: %w", err)
	}
	typ, ok := eventTypes[ev.Type]
	if !ok {
		typ = strings.ToLower(ev.Type.String())
	}
	return poller.Event{
		ID:               EventID(r.TransactionID, r.EventIndex),
		ContractID:       r.ContractID,
		Ledger:           uint32(r.LedgerSequence),
		LedgerClosedAt:   r.ClosedAt.UTC(),
		TxHash:           r.TransactionHash,
		Type:             typ,
		TopicXDR:         topics,
		ValueXDR:         value,
		InSuccessfulCall: r.InSuccessfulContractCall,
	}, nil
}

// Bounds on how long one mainnet ledger takes to close. The target is 5 to 6
// seconds; the margin keeps an estimated time range from cutting off real
// rows, while the ledger filter still decides exactly which rows match.
const (
	fastestLedger = 3 * time.Second
	slowestLedger = 8 * time.Second
)

// LedgerTimeBounds estimates a close-time range covering ledgers from..to,
// given one ledger whose close time is known (the anchor). It exists for
// partition pruning: Hubble's tables are partitioned by close time, and a
// query without a time filter would scan the whole table. The estimate is
// widest far from the anchor, so the source moves its anchor to the newest
// row it has seen.
func LedgerTimeBounds(from, to, anchorLedger uint32, anchorTime time.Time) (time.Time, time.Time) {
	offset := func(l uint32, per time.Duration) time.Duration {
		if l >= anchorLedger {
			return time.Duration(l-anchorLedger) * per
		}
		return -time.Duration(anchorLedger-l) * per
	}
	// The earliest from can have closed: slow ledgers if it is before the
	// anchor, fast ones if after. The latest to can have closed is the
	// reverse.
	earliestPer, latestPer := fastestLedger, slowestLedger
	if from < anchorLedger {
		earliestPer = slowestLedger
	}
	if to < anchorLedger {
		latestPer = fastestLedger
	}
	earliest := anchorTime.Add(offset(from, earliestPer)).Add(-time.Hour)
	latest := anchorTime.Add(offset(to, latestPer)).Add(time.Hour)
	return earliest, latest
}

// RowReader runs a Hubble query. BigQueryReader is the real one; tests use a
// fake.
type RowReader interface {
	Rows(ctx context.Context, sql string, params map[string]any) ([]HubbleRow, error)
}

// HubbleSource reads a contract's archived events from Hubble.
//
// Each batch needs a close-time range to prune partitions by. The source
// starts from an anchor, a ledger with a known close time (the RPC's latest
// ledger, for instance), and moves the anchor to the newest row each batch
// returns. Batches run oldest first and contiguously, so after the first one
// the estimate is tight.
type HubbleSource struct {
	Config HubbleConfig
	Reader RowReader

	anchorLedger uint32
	anchorTime   time.Time
}

// NewHubbleSource builds a source with a starting anchor.
func NewHubbleSource(cfg HubbleConfig, reader RowReader, anchorLedger uint32, anchorTime time.Time) *HubbleSource {
	return &HubbleSource{Config: cfg, Reader: reader, anchorLedger: anchorLedger, anchorTime: anchorTime}
}

// Events implements Source.
func (s *HubbleSource) Events(ctx context.Context, q Query) ([]poller.Event, error) {
	if err := s.Config.Validate(q.Network); err != nil {
		return nil, err
	}
	if s.anchorLedger == 0 || s.anchorTime.IsZero() {
		return nil, errors.New("hubble: no anchor ledger to estimate close times from")
	}
	fromTime, toTime := LedgerTimeBounds(q.FromLedger, q.ToLedger, s.anchorLedger, s.anchorTime)
	rows, err := s.Reader.Rows(ctx, HubbleQuery(s.Config), map[string]any{
		"contract_id": q.ContractID,
		"from_ledger": int64(q.FromLedger),
		"to_ledger":   int64(q.ToLedger),
		"from_time":   fromTime,
		"to_time":     toTime,
	})
	if err != nil {
		return nil, err
	}
	events := make([]poller.Event, 0, len(rows))
	for _, row := range rows {
		ev, err := row.ToEvent()
		if err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	// Rows come oldest first, so the last one is the nearest known close
	// time to the next batch.
	if n := len(events); n > 0 {
		s.anchorLedger, s.anchorTime = events[n-1].Ledger, events[n-1].LedgerClosedAt
	}
	return events, nil
}
