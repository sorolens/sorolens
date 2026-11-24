package store

import (
	"context"
	"fmt"
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

// ensureMaintenance lazily creates the map; callers must hold maintenanceMu.
func (m *MockStore) ensureMaintenance() {
	if m.maintenanceWindows == nil {
		m.maintenanceWindows = make(map[int64]MaintenanceWindow)
	}
}

func (m *MockStore) CreateMaintenanceWindow(_ context.Context, w MaintenanceWindow) (MaintenanceWindow, error) {
	if w.Name == "" || w.StartTime.IsZero() || w.EndTime.IsZero() || !w.EndTime.After(w.StartTime) {
		return MaintenanceWindow{}, ErrInvalidMaintenanceWindow
	}
	if w.Scope != "global" && w.Scope != "contract" && w.Scope != "group" {
		return MaintenanceWindow{}, ErrInvalidMaintenanceWindow
	}
	if (w.Scope == "contract" || w.Scope == "group") && w.TargetID == "" {
		return MaintenanceWindow{}, ErrInvalidMaintenanceWindow
	}
	if w.Timezone == "" {
		w.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(w.Timezone); err != nil {
		return MaintenanceWindow{}, fmt.Errorf("%w: invalid timezone %s", ErrInvalidMaintenanceWindow, w.Timezone)
	}
	if w.Recurrence == "" {
		w.Recurrence = "none"
	}

	m.maintenanceMu.Lock()
	defer m.maintenanceMu.Unlock()
	m.ensureMaintenance()
	m.maintenanceWindowSeq++
	w.ID = m.maintenanceWindowSeq
	w.CreatedAt = time.Now().UTC()
	w.UpdatedAt = w.CreatedAt
	m.maintenanceWindows[w.ID] = w
	return w, nil
}

func (m *MockStore) GetMaintenanceWindow(_ context.Context, id int64) (MaintenanceWindow, error) {
	m.maintenanceMu.Lock()
	defer m.maintenanceMu.Unlock()
	m.ensureMaintenance()
	w, ok := m.maintenanceWindows[id]
	if !ok {
		return MaintenanceWindow{}, ErrMaintenanceWindowNotFound
	}
	return w, nil
}

func (m *MockStore) UpdateMaintenanceWindow(_ context.Context, w MaintenanceWindow) (MaintenanceWindow, error) {
	if w.ID == 0 || w.Name == "" || !w.EndTime.After(w.StartTime) {
		return MaintenanceWindow{}, ErrInvalidMaintenanceWindow
	}
	if w.Timezone == "" {
		w.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(w.Timezone); err != nil {
		return MaintenanceWindow{}, fmt.Errorf("%w: invalid timezone %s", ErrInvalidMaintenanceWindow, w.Timezone)
	}
	m.maintenanceMu.Lock()
	defer m.maintenanceMu.Unlock()
	m.ensureMaintenance()
	existing, ok := m.maintenanceWindows[w.ID]
	if !ok {
		return MaintenanceWindow{}, ErrMaintenanceWindowNotFound
	}
	existing.Name = w.Name
	existing.StartTime = w.StartTime
	existing.EndTime = w.EndTime
	existing.Timezone = w.Timezone
	existing.Recurrence = w.Recurrence
	existing.SuppressNotifications = w.SuppressNotifications
	existing.UpdatedAt = time.Now().UTC()
	m.maintenanceWindows[w.ID] = existing
	return existing, nil
}

func (m *MockStore) DeleteMaintenanceWindow(_ context.Context, id int64) error {
	m.maintenanceMu.Lock()
	defer m.maintenanceMu.Unlock()
	m.ensureMaintenance()
	if _, ok := m.maintenanceWindows[id]; !ok {
		return ErrMaintenanceWindowNotFound
	}
	delete(m.maintenanceWindows, id)
	return nil
}

func (m *MockStore) ListMaintenanceWindows(_ context.Context, scope, targetID string) ([]MaintenanceWindow, error) {
	m.maintenanceMu.Lock()
	defer m.maintenanceMu.Unlock()
	m.ensureMaintenance()
	var out []MaintenanceWindow
	for _, w := range m.maintenanceWindows {
		if scope != "" && w.Scope != scope {
			continue
		}
		if targetID != "" && w.TargetID != targetID {
			continue
		}
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartTime.After(out[j].StartTime)
	})
	return out, nil
}

func (m *MockStore) IsUnderMaintenance(ctx context.Context, contractID string, groupID string, t time.Time) (*MaintenanceWindow, bool, error) {
	windows, err := m.ListMaintenanceWindows(ctx, "", "")
	if err != nil {
		return nil, false, err
	}
	for _, w := range windows {
		matchesScope := false
		switch w.Scope {
		case "global":
			matchesScope = true
		case "contract":
			if contractID != "" && w.TargetID == contractID {
				matchesScope = true
			}
		case "group":
			if groupID != "" && w.TargetID == groupID {
				matchesScope = true
			}
		}

		if matchesScope && w.IsActive(t) {
			win := w
			return &win, win.SuppressNotifications, nil
		}
	}
	return nil, false, nil
}
