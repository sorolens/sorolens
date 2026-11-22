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
	// alert_subscriptions and alert_groups both reference
	// monitored_contracts ON DELETE CASCADE, so mirror that here.
	filteredSubs := m.alertSubscriptions[:0]
	for _, sub := range m.alertSubscriptions {
		if sub.ContractID != contractID {
			filteredSubs = append(filteredSubs, sub)
		}
	}
	m.alertSubscriptions = filteredSubs
	filteredGroups := m.alertGroups[:0]
	for _, g := range m.alertGroups {
		if g.ContractID != contractID {
			filteredGroups = append(filteredGroups, g)
		}
	}
	m.alertGroups = filteredGroups
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
	if a.ID == 0 {
		a.ID = int64(len(m.alerts) + 1)
	}
	if a.State == "" {
		a.State = AlertStateOpen
	}
	m.alerts = append(m.alerts, a)
	return nil
}

func (m *MockStore) ListMonitoredContracts(_ context.Context, cursor string, limit int, network string) ([]MonitoredContract, string, error) {
	m.ensureWatchdog()
	// Same page-size bounds as the postgres implementation.
	if limit <= 0 || limit > 200 {
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
	// Same page-size bounds as the postgres implementation.
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]HealthCheck, 0)
	for _, h := range m.healthChecks {
		if h.ContractID == contractID {
			out = append(out, h)
		}
	}
	// ORDER BY timestamp DESC, matching the postgres query.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Timestamp.Equal(out[j].Timestamp) {
			return out[i].TxHash > out[j].TxHash
		}
		return out[i].Timestamp.After(out[j].Timestamp)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MockStore) ListAlerts(ctx context.Context, contractID, severity, network, cursor string, limit int) ([]ContractAlert, string, error) {
	return m.ListAlertsWithState(ctx, contractID, severity, network, "", cursor, limit)
}

func (m *MockStore) ListAlertsWithState(_ context.Context, contractID, severity, network, state, cursor string, limit int) ([]ContractAlert, string, error) {
	// Same page-size bounds as the postgres implementation.
	if limit <= 0 || limit > 500 {
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
	now := time.Now().UTC()
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
		// Handle snooze expiry in mock
		effectiveState := a.State
		if effectiveState == "" {
			effectiveState = AlertStateOpen
		}
		if effectiveState == AlertStateSnoozed && a.SnoozedUntil != nil && a.SnoozedUntil.Before(now) {
			effectiveState = AlertStateOpen
		}
		if state != "" && effectiveState != state {
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
		a.State = effectiveState
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

func (m *MockStore) AcknowledgeAlert(_ context.Context, id int64, user string) (ContractAlert, error) {
	m.ensureWatchdog()
	for i, a := range m.alerts {
		if a.ID == id {
			if a.State == AlertStateResolved {
				return ContractAlert{}, ErrInvalidTransition
			}
			now := time.Now().UTC()
			m.alerts[i].State = AlertStateAcknowledged
			m.alerts[i].AcknowledgedAt = &now
			m.alerts[i].AcknowledgedBy = user
			return m.alerts[i], nil
		}
	}
	return ContractAlert{}, ErrAlertNotFound
}

func (m *MockStore) SnoozeAlert(_ context.Context, id int64, duration time.Duration, user string) (ContractAlert, error) {
	if duration <= 0 {
		return ContractAlert{}, ErrInvalidTransition
	}
	m.ensureWatchdog()
	for i, a := range m.alerts {
		if a.ID == id {
			if a.State == AlertStateResolved {
				return ContractAlert{}, ErrInvalidTransition
			}
			now := time.Now().UTC()
			until := now.Add(duration)
			m.alerts[i].State = AlertStateSnoozed
			m.alerts[i].SnoozedAt = &now
			m.alerts[i].SnoozedBy = user
			m.alerts[i].SnoozedUntil = &until
			return m.alerts[i], nil
		}
	}
	return ContractAlert{}, ErrAlertNotFound
}

func (m *MockStore) ResolveAlert(_ context.Context, id int64, user string) (ContractAlert, error) {
	m.ensureWatchdog()
	for i, a := range m.alerts {
		if a.ID == id {
			if a.State == AlertStateResolved {
				return ContractAlert{}, ErrInvalidTransition
			}
			now := time.Now().UTC()
			m.alerts[i].State = AlertStateResolved
			m.alerts[i].ResolvedAt = &now
			m.alerts[i].ResolvedBy = user
			return m.alerts[i], nil
		}
	}
	return ContractAlert{}, ErrAlertNotFound
}

func (m *MockStore) GetAlertCountsByState(_ context.Context, contractID, network string) (map[string]int64, error) {
	m.ensureWatchdog()
	counts := map[string]int64{
		AlertStateOpen:         0,
		AlertStateAcknowledged: 0,
		AlertStateSnoozed:      0,
		AlertStateResolved:     0,
	}
	now := time.Now().UTC()
	for _, a := range m.alerts {
		if contractID != "" && a.ContractID != contractID {
			continue
		}
		if network != "" && m.monitored[a.ContractID].Network != network {
			continue
		}
		st := a.State
		if st == "" {
			st = AlertStateOpen
		}
		if st == AlertStateSnoozed && a.SnoozedUntil != nil && a.SnoozedUntil.Before(now) {
			st = AlertStateOpen
		}
		counts[st]++
	}
	return counts, nil
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
