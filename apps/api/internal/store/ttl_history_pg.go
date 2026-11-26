package store

import (
	"context"
	"fmt"
	"time"
)

func (s *postgresStore) RecordTTLOperation(ctx context.Context, op TTLOperation) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ttl_history
		    (contract_id, network, tx_hash, ledger, ledger_closed_at,
		     op_type, key_xdr, old_live_ledger, new_live_ledger, inserted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (contract_id, tx_hash, key_xdr) DO NOTHING`,
		op.ContractID, op.Network, op.TxHash, op.Ledger, op.LedgerClosedAt,
		op.OpType, op.KeyXDR, op.OldLiveLedger, op.NewLiveLedger,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("record ttl operation: %w", err)
	}
	return nil
}

func (s *postgresStore) ListTTLHistory(ctx context.Context, contractID, cursor string, limit int) ([]TTLOperation, string, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, network, tx_hash, ledger, ledger_closed_at,
		       op_type, key_xdr, old_live_ledger, new_live_ledger, inserted_at
		FROM ttl_history
		WHERE contract_id = $1
		  AND ($2 = '' OR id < $2)
		ORDER BY id DESC
		LIMIT $3`,
		contractID, cursor, limit+1,
	)
	if err != nil {
		return nil, "", fmt.Errorf("list ttl history: %w", err)
	}
	defer rows.Close()

	var out []TTLOperation
	for rows.Next() {
		var op TTLOperation
		if err := rows.Scan(
			&op.ID, &op.ContractID, &op.Network, &op.TxHash, &op.Ledger,
			&op.LedgerClosedAt, &op.OpType, &op.KeyXDR,
			&op.OldLiveLedger, &op.NewLiveLedger, &op.InsertedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scan ttl history: %w", err)
		}
		out = append(out, op)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}

	var nextCursor string
	if len(out) > limit {
		nextCursor = out[limit-1].ID
		out = out[:limit]
	}
	return out, nextCursor, nil
}
