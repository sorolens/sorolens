package store

import (
	"context"
	"sort"
	"time"
)

// Watchdog in-memory implementation for MockStore.

func (m *MockStore) ensureWatchdog() {
	if m.monitored == nil {
		m.monitored = make(map[string]MonitoredContract)
	}
}

func (m *MockStore) UpsertMonitoredContract(_ context.Context, mc MonitoredContract) error {
	m.ensureWatchdog()
	if existing, ok := m.monitored[mc.ContractID]; ok && mc.LastCheck == nil {
		mc.LastCheck = existing.LastCheck
	}
	mc.UpdatedAt = time.Now().UTC()
	m.monitored[mc.ContractID] = mc
	return nil
}

func (m *MockStore) DeleteMonitoredContract(_ context.Context, contractID string) error {
	m.ensureWatchdog()
	delete(m.monitored, contractID)
	filteredChecks := m.healthChecks[:0]
	for _, h := range m.healthChecks {
		if h.ContractID != contractID {
			filteredChecks = append(filteredChecks, h)
		}
	}
	m.healthChecks = filteredChecks
	filteredAlerts := m.alerts[:0]
	for _, a := range m.alerts {
		if a.ContractID != contractID {
			filteredAlerts = append(filteredAlerts, a)
		}
	}
	m.alerts = filteredAlerts
	return nil
}

func (m *MockStore) InsertHealthCheck(_ context.Context, h HealthCheck) error {
	for _, existing := range m.healthChecks {
		if existing.TxHash == h.TxHash && existing.ContractID == h.ContractID {
			return nil
		}
	}
	m.healthChecks = append(m.healthChecks, h)
	return nil
}

func (m *MockStore) InsertContractAlert(_ context.Context, a ContractAlert) error {
	for _, existing := range m.alerts {
		if existing.TxHash == a.TxHash && existing.ContractID == a.ContractID {
			return nil
		}
	}
	m.alerts = append(m.alerts, a)
	return nil
}

func (m *MockStore) ListMonitoredContracts(_ context.Context, cursor string, limit int) ([]MonitoredContract, string, error) {
	m.ensureWatchdog()
	if limit <= 0 {
		limit = 50
	}
	ids := make([]string, 0, len(m.monitored))
	for id := range m.monitored {
		if cursor == "" || id > cursor {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := make([]MonitoredContract, 0, len(ids))
	for _, id := range ids {
		out = append(out, m.monitored[id])
	}
	var next string
	if len(out) > limit {
		next = out[limit-1].ContractID
		out = out[:limit]
	}
	return out, next, nil
}

func (m *MockStore) GetMonitoredContract(_ context.Context, contractID string) (MonitoredContract, error) {
	m.ensureWatchdog()
	mc, ok := m.monitored[contractID]
	if !ok {
		return MonitoredContract{}, ErrNotFound
	}
	return mc, nil
}

func (m *MockStore) ListHealthChecks(_ context.Context, contractID string, limit int) ([]HealthCheck, error) {
	if limit <= 0 {
		limit = 100
	}
	out := make([]HealthCheck, 0)
	for i := len(m.healthChecks) - 1; i >= 0 && len(out) < limit; i-- {
		if m.healthChecks[i].ContractID == contractID {
			out = append(out, m.healthChecks[i])
		}
	}
	return out, nil
}

func (m *MockStore) ListAlerts(_ context.Context, contractID, severity string, limit int) ([]ContractAlert, error) {
	if limit <= 0 {
		limit = 100
	}
	out := make([]ContractAlert, 0)
	for i := len(m.alerts) - 1; i >= 0 && len(out) < limit; i-- {
		a := m.alerts[i]
		if contractID != "" && a.ContractID != contractID {
			continue
		}
		if severity != "" && a.Severity != severity {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

func (m *MockStore) GetWatchdogStats(_ context.Context) (WatchdogStats, error) {
	m.ensureWatchdog()
	var s WatchdogStats
	for _, mc := range m.monitored {
		s.TotalMonitored++
		switch mc.Status {
		case "Healthy":
			s.Healthy++
		case "Degraded":
			s.Degraded++
		case "Unresponsive":
			s.Unresponsive++
		}
	}
	s.TotalAlerts = int64(len(m.alerts))
	for _, a := range m.alerts {
		if a.Severity == "Critical" {
			s.CriticalAlerts++
		}
	}
	return s, nil
}
