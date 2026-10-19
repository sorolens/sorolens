package store

import (
	"context"
	"sort"
	"time"
)

// Contract health-score in-memory implementation for MockStore.

func (m *MockStore) ContractHealthInputs(_ context.Context, contractID string) (HealthScoreInputs, error) {
	return condenseMockHealthInputs(m, contractID), nil
}

func (m *MockStore) UpsertContractHealthScore(_ context.Context, h ContractHealthScore) error {
	if h.ComputedAt.IsZero() {
		h.ComputedAt = time.Now().UTC()
	}
	if m.healthScores == nil {
		m.healthScores = make(map[string]ContractHealthScore)
	}
	m.healthScores[h.ContractID] = h
	return nil
}

func (m *MockStore) GetContractHealthScore(_ context.Context, contractID string) (ContractHealthScore, error) {
	if m.GetHealthScoreErr != nil {
		return ContractHealthScore{}, m.GetHealthScoreErr
	}
	h, ok := m.healthScores[contractID]
	if !ok {
		return ContractHealthScore{}, ErrNotFound
	}
	return h, nil
}

// condenseMockHealthInputs derives HealthScoreInputs from whatever the mock
// already holds, mirroring the postgres aggregation so handler tests can seed
// contracts/invocations/storage and assert on realistic inputs.
func condenseMockHealthInputs(m *MockStore, contractID string) HealthScoreInputs {
	var in HealthScoreInputs

	if mc, ok := m.monitored[contractID]; ok {
		in.WatchdogStatus = mc.Status
	}

	// The postgres query reads only the last 100 checks, newest first.
	recent := make([]HealthCheck, 0, len(m.healthChecks))
	for _, hc := range m.healthChecks {
		if hc.ContractID == contractID {
			recent = append(recent, hc)
		}
	}
	sort.SliceStable(recent, func(i, j int) bool {
		return recent[i].Timestamp.After(recent[j].Timestamp)
	})
	if len(recent) > 100 {
		recent = recent[:100]
	}
	for _, hc := range recent {
		in.TotalChecks++
		if hc.Status == "Healthy" {
			in.HealthyChecks++
		}
	}

	// Invocation error rate over the trailing 24 hours.
	since := time.Now().UTC().Add(-24 * time.Hour)
	for _, inv := range m.invocations {
		if inv.ContractID != contractID || inv.LedgerClosedAt.Before(since) {
			continue
		}
		in.TotalInvocations++
		if inv.Status != "SUCCESS" {
			in.FailedInvocations++
		}
	}

	// TTL headroom is measured against the contract's last known ledger,
	// matching the postgres storage query.
	threshold := int64(m.syncStates[contractID].LastLedger) + storageHealthyHorizonLedgers
	for _, se := range m.storageEntries {
		if se.ContractID != contractID || se.Status != "live" {
			continue
		}
		in.TotalStorage++
		if se.LiveUntilLedger > 0 && se.LiveUntilLedger <= threshold {
			in.ExpiringStorage++
		}
	}

	if activity, err := m.RecentHourlyActivity(context.Background(), contractID, 24); err == nil {
		in.Activity = activity
	}
	return in
}
