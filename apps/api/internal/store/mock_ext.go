package store

import (
	"context"
	"sort"
	"time"
)

// This file holds the in-memory implementation of the LiveStore (#139) and
// ArchiveStore (#146) surfaces on MockStore. They are kept out of mock.go so
// the two features stay reviewable as a unit.

// ---- store.LiveStore --------------------------------------------------------

func (m *MockStore) RecentEventsAll(_ context.Context, limit int) ([]Event, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	out := make([]Event, len(m.events))
	copy(out, m.events)
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].LedgerClosedAt.Equal(out[j].LedgerClosedAt) {
			return out[i].LedgerClosedAt.After(out[j].LedgerClosedAt)
		}
		if out[i].Ledger != out[j].Ledger {
			return out[i].Ledger > out[j].Ledger
		}
		return out[i].ID > out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MockStore) ContractEventRates(_ context.Context, minutes int) ([]ContractEventRate, error) {
	if minutes <= 0 {
		minutes = LiveWindowMinute
	}
	if minutes > MaxLiveWindowMinute {
		minutes = MaxLiveWindowMinute
	}
	start := LiveWindowStart(minutes, time.Now())

	byContract := make(map[string]*ContractEventRate)
	var order []string
	for _, e := range m.events {
		if e.LedgerClosedAt.Before(start) {
			continue
		}
		rate, ok := byContract[e.ContractID]
		if !ok {
			rate = &ContractEventRate{
				ContractID: e.ContractID,
				PerMinute:  make([]int64, minutes),
			}
			if c, ok := m.contracts[e.ContractID]; ok {
				rate.Label = c.Label
				rate.Network = c.Network
			}
			byContract[e.ContractID] = rate
			order = append(order, e.ContractID)
		}
		if idx := int(e.LedgerClosedAt.UTC().Truncate(time.Minute).Sub(start).Minutes()); idx >= 0 && idx < minutes {
			rate.PerMinute[idx]++
		}
		rate.Total++
	}

	out := make([]ContractEventRate, 0, len(order))
	for _, id := range order {
		out = append(out, *byContract[id])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].ContractID < out[j].ContractID
	})
	return out, nil
}

// ---- store.ArchiveStore -----------------------------------------------------

func (m *MockStore) ContractsWithEventsBefore(_ context.Context, before time.Time) ([]string, error) {
	seen := make(map[string]bool)
	var out []string
	for _, e := range m.events {
		if e.LedgerClosedAt.Before(before) && !seen[e.ContractID] {
			seen[e.ContractID] = true
			out = append(out, e.ContractID)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (m *MockStore) EventsBefore(_ context.Context, contractID string, before time.Time, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 10_000
	}
	var out []Event
	for _, e := range m.events {
		if e.ContractID == contractID && e.LedgerClosedAt.Before(before) {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Ledger != out[j].Ledger {
			return out[i].Ledger < out[j].Ledger
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MockStore) DeleteEventsByID(_ context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	drop := make(map[string]bool, len(ids))
	for _, id := range ids {
		drop[id] = true
	}
	kept := m.events[:0]
	var deleted int64
	for _, e := range m.events {
		if drop[e.ID] {
			deleted++
			continue
		}
		kept = append(kept, e)
	}
	m.events = kept
	return deleted, nil
}
