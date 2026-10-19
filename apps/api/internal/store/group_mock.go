package store

import (
	"context"
	"sort"
	"time"
)

// ---- store.GroupStore (in-memory) -------------------------------------------

// groupMembers returns the contract IDs in a group, sorted for determinism.
func (m *MockStore) groupMembers(groupID string) []string {
	members := m.groupContracts[groupID]
	out := make([]string, 0, len(members))
	for contractID := range members {
		out = append(out, contractID)
	}
	sort.Strings(out)
	return out
}

// groupOwned reports whether the group exists and belongs to ownerID.
func (m *MockStore) groupOwned(ownerID, groupID string) bool {
	g, ok := m.groups[groupID]
	return ok && g.OwnerID == ownerID
}

// groupStatsFor mirrors the postgres aggregation: events, invocations, and
// storage entries are counted per member contract, and the average health
// score is taken over members that have a cached score.
func (m *MockStore) groupStatsFor(groupID string) (contractCount, events, invocations, storage int64, avgHealth float64) {
	members := m.groupMembers(groupID)
	contractCount = int64(len(members))

	for _, contractID := range members {
		for _, e := range m.events {
			if e.ContractID == contractID {
				events++
			}
		}
		for _, inv := range m.invocations {
			if inv.ContractID == contractID {
				invocations++
			}
		}
		for _, se := range m.storageEntries {
			if se.ContractID == contractID {
				storage++
			}
		}
	}

	var scoreSum float64
	var scoreCount int64
	for _, contractID := range members {
		if hs, ok := m.healthScores[contractID]; ok {
			scoreSum += float64(hs.Score)
			scoreCount++
		}
	}
	if scoreCount > 0 {
		avgHealth = scoreSum / float64(scoreCount)
	}
	return contractCount, events, invocations, storage, avgHealth
}

func (m *MockStore) CreateGroup(_ context.Context, ownerID, name string) (Group, error) {
	trimmed, ok := validGroupName(name)
	if !ok {
		return Group{}, ErrInvalidGroupName
	}
	if m.groups == nil {
		m.groups = make(map[string]Group)
	}
	g := Group{
		ID:        newGroupID(),
		OwnerID:   ownerID,
		Name:      trimmed,
		CreatedAt: time.Now().UTC(),
	}
	m.groups[g.ID] = g
	return g, nil
}

func (m *MockStore) ListGroups(_ context.Context, ownerID string) ([]GroupSummary, error) {
	out := make([]GroupSummary, 0)
	for id, g := range m.groups {
		if g.OwnerID != ownerID {
			continue
		}
		cc, ev, inv, st, avg := m.groupStatsFor(id)
		out = append(out, GroupSummary{
			Group:              g,
			ContractCount:      cc,
			EventCount:         ev,
			InvocationCount:    inv,
			StorageEntryCount:  st,
			AverageHealthScore: avg,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (m *MockStore) GetGroup(_ context.Context, ownerID, groupID string) (Group, error) {
	if !m.groupOwned(ownerID, groupID) {
		return Group{}, ErrNotFound
	}
	return m.groups[groupID], nil
}

func (m *MockStore) UpdateGroup(_ context.Context, ownerID, groupID, name string) (Group, error) {
	if !m.groupOwned(ownerID, groupID) {
		return Group{}, ErrNotFound
	}
	trimmed, ok := validGroupName(name)
	if !ok {
		return Group{}, ErrInvalidGroupName
	}
	g := m.groups[groupID]
	g.Name = trimmed
	m.groups[groupID] = g
	return g, nil
}

func (m *MockStore) DeleteGroup(_ context.Context, ownerID, groupID string) error {
	if !m.groupOwned(ownerID, groupID) {
		return ErrNotFound
	}
	delete(m.groups, groupID)
	delete(m.groupContracts, groupID)
	return nil
}

func (m *MockStore) AddContractToGroup(_ context.Context, ownerID, groupID, contractID string) error {
	if !m.groupOwned(ownerID, groupID) {
		return ErrNotFound
	}
	if m.groupContracts == nil {
		m.groupContracts = make(map[string]map[string]bool)
	}
	if m.groupContracts[groupID] == nil {
		m.groupContracts[groupID] = make(map[string]bool)
	}
	m.groupContracts[groupID][contractID] = true
	return nil
}

func (m *MockStore) RemoveContractFromGroup(_ context.Context, ownerID, groupID, contractID string) error {
	if !m.groupOwned(ownerID, groupID) {
		return ErrNotFound
	}
	if m.groupContracts[groupID] != nil {
		delete(m.groupContracts[groupID], contractID)
	}
	return nil
}

func (m *MockStore) ListGroupContracts(_ context.Context, ownerID, groupID string) ([]GroupContract, error) {
	if !m.groupOwned(ownerID, groupID) {
		return nil, ErrNotFound
	}
	out := make([]GroupContract, 0)
	for _, contractID := range m.groupMembers(groupID) {
		c, ok := m.contracts[contractID]
		if !ok {
			// Mirrors the postgres JOIN on contracts: memberships that point
			// at an unknown contract are not rendered.
			continue
		}
		gc := GroupContract{
			ContractID: c.ID,
			Network:    c.Network,
			Label:      c.Label,
			Status:     c.Status,
		}
		if hs, ok := m.healthScores[c.ID]; ok {
			score := hs.Score
			gc.HealthScore = &score
		}
		var last time.Time
		found := false
		for _, e := range m.events {
			if e.ContractID == c.ID && (!found || e.LedgerClosedAt.After(last)) {
				last = e.LedgerClosedAt
				found = true
			}
		}
		for _, inv := range m.invocations {
			if inv.ContractID == c.ID && (!found || inv.LedgerClosedAt.After(last)) {
				last = inv.LedgerClosedAt
				found = true
			}
		}
		if found {
			t := last
			gc.LastActivityAt = &t
		}
		out = append(out, gc)
	}
	return out, nil
}

func (m *MockStore) GetGroupStats(_ context.Context, ownerID, groupID string) (GroupStats, error) {
	if !m.groupOwned(ownerID, groupID) {
		return GroupStats{}, ErrNotFound
	}
	cc, ev, inv, st, avg := m.groupStatsFor(groupID)
	return GroupStats{
		GroupID:            groupID,
		ContractCount:      cc,
		EventCount:         ev,
		InvocationCount:    inv,
		StorageEntryCount:  st,
		AverageHealthScore: avg,
	}, nil
}
