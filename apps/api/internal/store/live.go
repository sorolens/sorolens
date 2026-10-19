package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// ContractEventRate is one contract's event activity over the live window. It
// backs both the per-contract sparklines and the "hot contracts" leaderboard
// on the /live dashboard.
type ContractEventRate struct {
	ContractID string
	Label      string
	Network    string
	// Total is the number of events observed in the window across all
	// buckets.
	Total int64
	// PerMinute holds one bucket per minute of the requested window, oldest
	// first. Its length always equals the requested minute count, including
	// trailing and leading zero buckets, so the series is contiguous and the
	// sparkline never shifts under the reader.
	PerMinute []int64
}

// LiveStore provides the read queries that back the real-time /live dashboard
// (issue #139). None of these are cursor-paginated: the dashboard always wants
// the newest slice of a bounded window.
type LiveStore interface {
	// RecentEventsAll returns the most recent events across every tracked
	// contract, newest first. It is the data source for the event ticker.
	RecentEventsAll(ctx context.Context, limit int) ([]Event, error)

	// ContractEventRates returns one row per contract that emitted at least
	// one event in the last `minutes` minutes, ordered by total event count
	// descending (the "hot contracts" ordering).
	ContractEventRates(ctx context.Context, minutes int) ([]ContractEventRate, error)
}

// LiveWindowMinute is the default sparkline window: 30 one-minute buckets.
const LiveWindowMinute = 30

// MaxLiveWindowMinute caps a live window at 24 hours of one-minute buckets.
const MaxLiveWindowMinute = 24 * 60

// LiveWindowStart returns the aligned start of a live window of `minutes`
// one-minute buckets ending at the current minute. Bucket boundaries are
// aligned to the wall-clock minute so that two callers a few seconds apart
// agree on the x-axis, which keeps the sparkline from jittering between
// refreshes.
func LiveWindowStart(minutes int, now time.Time) time.Time {
	if minutes <= 0 {
		minutes = LiveWindowMinute
	}
	if minutes > MaxLiveWindowMinute {
		minutes = MaxLiveWindowMinute
	}
	return now.UTC().Truncate(time.Minute).Add(-time.Duration(minutes-1) * time.Minute)
}

// eventColumns is the shared SELECT list for event rows. Every event query in
// this package scans the columns in this order so scanEvents can be reused.
const eventColumns = `id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
	       topic_xdr, value_xdr, topic_decoded, value_decoded,
	       in_successful_call, inserted_at`

// scanEvents drains rows into a slice of Event, decoding the JSONB topic and
// value columns exactly as the hand-written scanners elsewhere in the package
// do. The caller must not close rows separately.
func scanEvents(rows pgx.Rows) ([]Event, error) {
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var topicXDR, topicDec, valDec []byte
		if err := rows.Scan(
			&e.ID, &e.ContractID, &e.Network, &e.Ledger, &e.LedgerClosedAt, &e.TxHash, &e.Type,
			&topicXDR, &e.ValueXDR, &topicDec, &valDec,
			&e.InSuccessfulCall, &e.InsertedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(topicXDR, &e.TopicXDR)
		_ = json.Unmarshal(topicDec, &e.TopicDecoded)
		_ = json.Unmarshal(valDec, &e.ValueDecoded)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---- RecentEventsAll --------------------------------------------------------

func (s *postgresStore) RecentEventsAll(ctx context.Context, limit int) ([]Event, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+eventColumns+`
		FROM events
		ORDER BY ledger_closed_at DESC, ledger DESC, id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent events (all contracts): %w", err)
	}
	return scanEvents(rows)
}

// ---- ContractEventRates -----------------------------------------------------

func (s *postgresStore) ContractEventRates(ctx context.Context, minutes int) ([]ContractEventRate, error) {
	if minutes <= 0 {
		minutes = LiveWindowMinute
	}
	if minutes > MaxLiveWindowMinute {
		minutes = MaxLiveWindowMinute
	}

	start := LiveWindowStart(minutes, time.Now())

	rows, err := s.pool.Query(ctx, `
		SELECT e.contract_id,
		       COALESCE(c.label, '')   AS label,
		       COALESCE(c.network, '') AS network,
		       date_trunc('minute', e.ledger_closed_at) AS minute,
		       COUNT(*) AS event_count
		FROM events e
		LEFT JOIN contracts c ON c.id = e.contract_id
		WHERE e.ledger_closed_at >= $1
		GROUP BY 1, 2, 3, 4
		ORDER BY 1, 4`, start)
	if err != nil {
		return nil, fmt.Errorf("contract event rates: %w", err)
	}
	defer rows.Close()

	byContract := make(map[string]*ContractEventRate)
	var order []string
	for rows.Next() {
		var contractID, label, network string
		var minute time.Time
		var count int64
		if err := rows.Scan(&contractID, &label, &network, &minute, &count); err != nil {
			return nil, err
		}
		rate, ok := byContract[contractID]
		if !ok {
			rate = &ContractEventRate{
				ContractID: contractID,
				Label:      label,
				Network:    network,
				PerMinute:  make([]int64, minutes),
			}
			byContract[contractID] = rate
			order = append(order, contractID)
		}
		if rate.Label == "" {
			rate.Label = label
		}
		if idx := int(minute.UTC().Sub(start).Minutes()); idx >= 0 && idx < minutes {
			rate.PerMinute[idx] = count
		}
		rate.Total += count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]ContractEventRate, 0, len(order))
	for _, id := range order {
		out = append(out, *byContract[id])
	}
	// Hot contracts first. Ties break on contract id so the leaderboard
	// order is stable between refreshes.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].ContractID < out[j].ContractID
	})
	return out, nil
}
