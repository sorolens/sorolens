package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// MockStore implements TokenTransferStore for unit tests.

func (m *MockStore) UpsertTokenTransfer(_ context.Context, t TokenTransfer) error {
	for _, existing := range m.tokenTransfers {
		if existing.EventID == t.EventID {
			return nil // idempotent
		}
	}
	t.ID = int64(len(m.tokenTransfers) + 1)
	m.tokenTransfers = append(m.tokenTransfers, t)
	return nil
}

func (m *MockStore) UpsertTokenHolderBalance(_ context.Context, contractID, network, holder, delta string) error {
	key := contractID + ":" + holder
	cur := m.tokenHolderBalances[key]
	if cur == nil {
		cur = &TokenHolder{ContractID: contractID, Network: network, Holder: holder, Balance: "0"}
		if m.tokenHolderBalances == nil {
			m.tokenHolderBalances = make(map[string]*TokenHolder)
		}
		m.tokenHolderBalances[key] = cur
	}
	// Simplified: just replace with delta for unit tests.
	cur.Balance = delta
	return nil
}

func (m *MockStore) ListTokenTransfers(_ context.Context, contractID, cursor string, limit int, f TokenTransferFilters) ([]TokenTransfer, string, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	var out []TokenTransfer
	for _, t := range m.tokenTransfers {
		if t.ContractID != contractID {
			continue
		}
		if f.Network != "" && t.Network != f.Network {
			continue
		}
		if f.TransferType != "" && !strings.EqualFold(t.TransferType, f.TransferType) {
			continue
		}
		if f.From != 0 && t.Ledger < f.From {
			continue
		}
		if f.To != 0 && t.Ledger > f.To {
			continue
		}
		if cursor != "" && fmt.Sprintf("%d", t.ID) >= cursor {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ledger != out[j].Ledger {
			return out[i].Ledger > out[j].Ledger
		}
		return out[i].ID > out[j].ID
	})
	var next string
	if len(out) > limit {
		next = fmt.Sprintf("%d", out[limit-1].ID)
		out = out[:limit]
	}
	return out, next, nil
}

func (m *MockStore) ListTokenHolders(_ context.Context, contractID, network, cursor string, limit int) ([]TokenHolder, string, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	var out []TokenHolder
	for _, h := range m.tokenHolderBalances {
		if h.ContractID != contractID {
			continue
		}
		if network != "" && h.Network != network {
			continue
		}
		if cursor != "" && h.Holder <= cursor {
			continue
		}
		out = append(out, *h)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Balance != out[j].Balance {
			return out[i].Balance > out[j].Balance
		}
		return out[i].Holder < out[j].Holder
	})
	var next string
	if len(out) > limit {
		next = out[limit-1].Holder
		out = out[:limit]
	}
	return out, next, nil
}
