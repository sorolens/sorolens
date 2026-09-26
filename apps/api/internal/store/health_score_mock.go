package store

import (
	"context"
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
	for _, hc := range m.healthChecks {
		if hc.ContractID != contractID {
			continue
		}
		in.TotalChecks++
		if hc.Status == "Healthy" {
			in.HealthyChecks++
		}
	}
	for _, inv := range m.invocations {
		if inv.ContractID != contractID {
			continue
		}
		in.TotalInvocations++
		if inv.Status != "SUCCESS" {
			in.FailedInvocations++
		}
	}
	for _, se := range m.storageEntries {
		if se.ContractID != contractID {
			continue
		}
		if se.Status != "live" {
			continue
		}
		in.TotalStorage++
		if se.LiveUntilLedger > 0 && se.LiveUntilLedger <= storageHealthyHorizonLedgers {
			in.ExpiringStorage++
		}
	}
	return in
}
