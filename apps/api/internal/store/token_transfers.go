package store

import (
	"context"
	"fmt"
	"time"
)

// TokenTransfer is one SEP-41 token event (transfer, mint, burn, clawback, or
// approve) extracted from the events table by the indexer.
type TokenTransfer struct {
	ID             int64
	ContractID     string
	Network        string
	TxHash         string
	Ledger         uint32
	LedgerClosedAt time.Time
	EventID        string
	TransferType   string // transfer | mint | burn | clawback | approve
	FromAddress    string // empty for mint
	ToAddress      string // empty for burn/clawback
	Amount         string // decimal string; numeric(39,0) in Postgres
	InsertedAt     time.Time
}

// TokenHolder holds a snapshot of one address's balance for a token contract.
type TokenHolder struct {
	ContractID string
	Network    string
	Holder     string
	Balance    string // decimal string
	UpdatedAt  time.Time
}

// TokenTransferFilters holds optional filters for ListTokenTransfers.
type TokenTransferFilters struct {
	Network      string
	TransferType string
	From         uint32
	To           uint32
}

// TokenTransferStore is the read/write surface for SEP-41 token tracking
// (issue #801).
type TokenTransferStore interface {
	// UpsertTokenTransfer inserts or silently ignores a duplicate token
	// transfer row (idempotent via the unique event_id constraint).
	UpsertTokenTransfer(ctx context.Context, t TokenTransfer) error

	// UpsertTokenHolderBalance applies a signed balance delta to a holder row,
	// clamping the result to zero on underflow. The delta is a decimal string
	// that may be negative (e.g. "-1000000" for a burn).
	UpsertTokenHolderBalance(ctx context.Context, contractID, network, holder, delta string) error

	// ListTokenTransfers returns token transfers for a contract, newest first.
	// cursor is the string form of the last row's id ("" for the first page);
	// next is "" when the page is the last one.
	ListTokenTransfers(ctx context.Context, contractID, cursor string, limit int, f TokenTransferFilters) ([]TokenTransfer, string, error)

	// ListTokenHolders returns holders sorted by balance descending.
	// cursor is the holder address of the last row ("" for the first page).
	ListTokenHolders(ctx context.Context, contractID, network, cursor string, limit int) ([]TokenHolder, string, error)
}

// ---- postgres implementation ------------------------------------------------

func (s *postgresStore) UpsertTokenTransfer(ctx context.Context, t TokenTransfer) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO token_transfers
		    (contract_id, network, tx_hash, ledger, ledger_closed_at, event_id,
		     transfer_type, from_address, to_address, amount)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (event_id) DO NOTHING`,
		t.ContractID, t.Network, t.TxHash, t.Ledger, t.LedgerClosedAt, t.EventID,
		t.TransferType, nullStr(t.FromAddress), nullStr(t.ToAddress), t.Amount,
	)
	if err != nil {
		return fmt.Errorf("upsert token transfer: %w", err)
	}
	return nil
}

func (s *postgresStore) UpsertTokenHolderBalance(ctx context.Context, contractID, network, holder, delta string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO token_holder_balances (contract_id, network, holder, balance, updated_at)
		VALUES ($1, $2, $3,
		    GREATEST(0, $4::numeric(39,0)),
		    NOW()
		)
		ON CONFLICT (contract_id, holder) DO UPDATE
		    SET balance    = GREATEST(0, token_holder_balances.balance + $4::numeric(39,0)),
		        updated_at = NOW()`,
		contractID, network, holder, delta,
	)
	if err != nil {
		return fmt.Errorf("upsert token holder balance: %w", err)
	}
	return nil
}

func (s *postgresStore) ListTokenTransfers(ctx context.Context, contractID, cursor string, limit int, f TokenTransferFilters) ([]TokenTransfer, string, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	rows, err := s.readDB(ctx).Query(ctx, `
		SELECT id, contract_id, network, tx_hash, ledger, ledger_closed_at, event_id,
		       transfer_type,
		       COALESCE(from_address, ''),
		       COALESCE(to_address, ''),
		       amount::text, inserted_at
		FROM token_transfers
		WHERE contract_id = $1
		  AND ($2 = '' OR network = $2)
		  AND ($3 = '' OR transfer_type = $3)
		  AND ($4 = 0   OR ledger >= $4)
		  AND ($5 = 0   OR ledger <= $5)
		  AND ($6 = ''  OR id < $6::bigint)
		ORDER BY ledger DESC, id DESC
		LIMIT $7`,
		contractID, f.Network, f.TransferType, f.From, f.To, cursor, limit+1,
	)
	if err != nil {
		return nil, "", fmt.Errorf("list token transfers: %w", err)
	}
	defer rows.Close()

	var out []TokenTransfer
	for rows.Next() {
		var t TokenTransfer
		if err := rows.Scan(
			&t.ID, &t.ContractID, &t.Network, &t.TxHash, &t.Ledger, &t.LedgerClosedAt, &t.EventID,
			&t.TransferType, &t.FromAddress, &t.ToAddress, &t.Amount, &t.InsertedAt,
		); err != nil {
			return nil, "", err
		}
		out = append(out, t)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}

	var next string
	if len(out) > limit {
		next = fmt.Sprintf("%d", out[limit-1].ID)
		out = out[:limit]
	}
	return out, next, nil
}

func (s *postgresStore) ListTokenHolders(ctx context.Context, contractID, network, cursor string, limit int) ([]TokenHolder, string, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	rows, err := s.readDB(ctx).Query(ctx, `
		SELECT contract_id, network, holder, balance::text, updated_at
		FROM token_holder_balances
		WHERE contract_id = $1
		  AND ($2 = '' OR network = $2)
		  AND ($3 = '' OR holder > $3)
		ORDER BY balance DESC, holder ASC
		LIMIT $4`,
		contractID, network, cursor, limit+1,
	)
	if err != nil {
		return nil, "", fmt.Errorf("list token holders: %w", err)
	}
	defer rows.Close()

	var out []TokenHolder
	for rows.Next() {
		var h TokenHolder
		if err := rows.Scan(&h.ContractID, &h.Network, &h.Holder, &h.Balance, &h.UpdatedAt); err != nil {
			return nil, "", err
		}
		out = append(out, h)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}

	var next string
	if len(out) > limit {
		next = out[limit-1].Holder
		out = out[:limit]
	}
	return out, next, nil
}

// nullStr returns nil when s is empty, so empty optional columns are stored as
// NULL rather than empty string.
func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
