package store

import (
	"context"
	"sort"
	"time"
)

// Contract upgrade in-memory implementation for MockStore.

func (m *MockStore) InsertContractUpgrade(_ context.Context, u ContractUpgrade) error {
	if u.ID == 0 {
		u.ID = int64(len(m.contractUpgrades) + 1)
	}
	if u.At.IsZero() {
		u.At = time.Now().UTC()
	}
	for _, existing := range m.contractUpgrades {
		if existing.ContractID == u.ContractID && existing.TxHash == u.TxHash {
			return nil // idempotent
		}
	}
	m.contractUpgrades = append(m.contractUpgrades, u)
	return nil
}

func (m *MockStore) ListContractUpgrades(_ context.Context, contractID string, limit int) ([]ContractUpgrade, error) {
	if m.ListUpgradesErr != nil {
		return nil, m.ListUpgradesErr
	}
	if limit <= 0 {
		limit = 100
	}
	out := make([]ContractUpgrade, 0)
	for _, u := range m.contractUpgrades {
		if u.ContractID == contractID {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ledger == out[j].Ledger {
			return out[i].ID > out[j].ID
		}
		return out[i].Ledger > out[j].Ledger
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MockStore) LatestContractUpgrade(ctx context.Context, contractID string) (ContractUpgrade, error) {
	upgrades, err := m.ListContractUpgrades(ctx, contractID, 1)
	if err != nil {
		return ContractUpgrade{}, err
	}
	if len(upgrades) == 0 {
		return ContractUpgrade{}, ErrNotFound
	}
	return upgrades[0], nil
}
