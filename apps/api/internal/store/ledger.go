package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetEventsByTxHash returns all events for the given tx_hash, ordered by
// ledger ASC then id ASC.
func (s *postgresStore) GetEventsByTxHash(ctx context.Context, txHash string) ([]Event, error) {
	rows, err := s.readDB(ctx).Query(ctx, `
		SELECT id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
		       topic_xdr, value_xdr, topic_decoded, value_decoded,
		       in_successful_call, inserted_at
		FROM events
		WHERE tx_hash = $1
		ORDER BY ledger ASC, id ASC`,
		txHash,
	)
	if err != nil {
		return nil, fmt.Errorf("get events by tx hash: %w", err)
	}
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

// GetLedgerSummary returns aggregated statistics for a single ledger sequence.
// Returns ErrNotFound when neither invocations nor events have been indexed for
// that ledger.
func (s *postgresStore) GetLedgerSummary(ctx context.Context, sequence uint32, network string) (LedgerSummary, error) {
	var sum LedgerSummary
	sum.Sequence = sequence

	// Aggregate invocation counts.
	err := s.readDB(ctx).QueryRow(ctx, `
		SELECT
			MAX(ledger_closed_at),
			COUNT(DISTINCT tx_hash),
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'FAILED' THEN 1 ELSE 0 END), 0)
		FROM invocations
		WHERE ledger = $1
		  AND ($2 = '' OR network = $2)`,
		sequence, network,
	).Scan(&sum.ClosedAt, &sum.TxCount, &sum.InvocationCount, &sum.FailedCount)
	if err != nil {
		return LedgerSummary{}, fmt.Errorf("get ledger summary invocations: %w", err)
	}

	// Event count across all contracts for this ledger.
	if err := s.readDB(ctx).QueryRow(ctx, `
		SELECT COUNT(*)
		FROM events
		WHERE ledger = $1
		  AND ($2 = '' OR network = $2)`,
		sequence, network,
	).Scan(&sum.EventCount); err != nil {
		return LedgerSummary{}, fmt.Errorf("get ledger summary events: %w", err)
	}

	if sum.TxCount == 0 && sum.EventCount == 0 {
		return LedgerSummary{}, ErrNotFound
	}

	// Top 10 contracts by event count.
	rows, err := s.readDB(ctx).Query(ctx, `
		SELECT e.contract_id,
		       COUNT(*) AS event_count,
		       COALESCE(inv_counts.invocation_count, 0)
		FROM events e
		LEFT JOIN (
			SELECT contract_id, COUNT(*) AS invocation_count
			FROM invocations
			WHERE ledger = $1 AND ($2 = '' OR network = $2)
			GROUP BY contract_id
		) inv_counts USING (contract_id)
		WHERE e.ledger = $1
		  AND ($2 = '' OR e.network = $2)
		GROUP BY e.contract_id, inv_counts.invocation_count
		ORDER BY event_count DESC
		LIMIT 10`,
		sequence, network,
	)
	if err != nil {
		return LedgerSummary{}, fmt.Errorf("get ledger summary top contracts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c ContractActivitySummary
		if err := rows.Scan(&c.ContractID, &c.EventCount, &c.InvokeCount); err != nil {
			return LedgerSummary{}, err
		}
		sum.TopContracts = append(sum.TopContracts, c)
	}
	if rows.Err() != nil {
		return LedgerSummary{}, rows.Err()
	}
	if sum.TopContracts == nil {
		sum.TopContracts = []ContractActivitySummary{}
	}
	return sum, nil
}
