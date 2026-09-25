package store

import (
	"context"
	"fmt"
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

func (m *MockStore) ListMonitoredContracts(_ context.Context, cursor string, limit int, network string) ([]MonitoredContract, string, error) {
	m.ensureWatchdog()
	if limit <= 0 {
		limit = 50
	}
	ids := make([]string, 0, len(m.monitored))
	for id, mc := range m.monitored {
		if network != "" && mc.Network != network {
			continue
		}
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

// ListAlerts mirrors the postgres keyset pagination: rows are ordered
// newest first by (timestamp desc, contract_id desc) and the optional
// cursor selects strictly older rows. The returned cursor points at the
// last row of the page and is empty when the feed is exhausted.
func (m *MockStore) ListAlerts(_ context.Context, contractID, severity, network, cursor string, limit int) ([]ContractAlert, string, error) {
	if limit <= 0 {
		limit = 100
	}
	var cursorTS time.Time
	var cursorID string
	if cursor != "" {
		ts, id, err := decodeAlertsCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		cursorTS, cursorID = ts, id
	}
	m.ensureWatchdog()
	out := make([]ContractAlert, 0)
	for i := len(m.alerts) - 1; i >= 0; i-- {
		a := m.alerts[i]
		if contractID != "" && a.ContractID != contractID {
			continue
		}
		if severity != "" && a.Severity != severity {
			continue
		}
		if network != "" && m.monitored[a.ContractID].Network != network {
			continue
		}
		if cursor != "" {
			// Keyset: keep rows strictly older than the (timestamp, id) pair
			// under (timestamp DESC, contract_id DESC) ordering.
			if a.Timestamp.After(cursorTS) {
				continue
			}
			if a.Timestamp.Equal(cursorTS) && a.ContractID >= cursorID {
				continue
			}
		}
		out = append(out, a)
		if len(out) > limit {
			break
		}
	}
	var next string
	if len(out) > limit {
		last := out[limit-1]
		next = encodeAlertsCursor(last.Timestamp, last.ContractID)
		out = out[:limit]
	}
	return out, next, nil
}

func (m *MockStore) GetWatchdogStats(_ context.Context, network string) (WatchdogStats, error) {
	m.ensureWatchdog()
	var s WatchdogStats
	for _, mc := range m.monitored {
		if network != "" && mc.Network != network {
			continue
		}
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
	for _, a := range m.alerts {
		if network != "" && m.monitored[a.ContractID].Network != network {
			continue
		}
		s.TotalAlerts++
		if a.Severity == "Critical" {
			s.CriticalAlerts++
		}
	}
	return s, nil
}

// GetContractUptime computes uptime percentage for the mock store by scanning
// the in-memory health checks for the requested window.
func (m *MockStore) GetContractUptime(_ context.Context, contractID string, window string) (UptimeResult, error) {
	var dur time.Duration
	switch window {
	case "24h":
		dur = 24 * time.Hour
	case "7d":
		dur = 7 * 24 * time.Hour
	case "30d":
		dur = 30 * 24 * time.Hour
	default:
		return UptimeResult{}, fmt.Errorf("invalid window %q: must be 24h, 7d, or 30d", window)
	}

	since := time.Now().UTC().Add(-dur)
	var total, healthy int64
	for _, h := range m.healthChecks {
		if h.ContractID != contractID {
			continue
		}
		if h.Timestamp.Before(since) {
			continue
		}
		total++
		if h.Status == "Healthy" {
			healthy++
		}
	}

	var uptimePct float64
	if total > 0 {
		uptimePct = float64(healthy) / float64(total) * 100.0
	}
	return UptimeResult{
		ContractID: contractID,
		Window:     window,
		Uptime:     uptimePct,
	}, nil
}
