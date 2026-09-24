package store

import (
	"context"
	"sort"
	"time"

	"github.com/sorolens/sorolens/packages/rules"
)

// RuleStore in-memory implementation for MockStore.

// AddRule is a test helper that seeds a rule directly.
func (m *MockStore) AddRule(r AlertRule) {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	if r.UpdatedAt.IsZero() {
		r.UpdatedAt = r.CreatedAt
	}
	if r.Severity == "" {
		r.Severity = "Warning"
	}
	m.rules = append(m.rules, r)
}

func (m *MockStore) CreateRule(_ context.Context, r AlertRule) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	if r.UpdatedAt.IsZero() {
		r.UpdatedAt = r.CreatedAt
	}
	m.rules = append(m.rules, r)
	return nil
}

func (m *MockStore) UpdateRule(_ context.Context, r AlertRule) error {
	for i := range m.rules {
		if m.rules[i].ID == r.ID {
			r.CreatedAt = m.rules[i].CreatedAt
			m.rules[i] = r
			return nil
		}
	}
	return ErrNotFound
}

func (m *MockStore) DeleteRule(_ context.Context, id string) error {
	for i := range m.rules {
		if m.rules[i].ID == id {
			m.rules = append(m.rules[:i], m.rules[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (m *MockStore) GetRule(_ context.Context, id string) (AlertRule, error) {
	for _, r := range m.rules {
		if r.ID == id {
			return r, nil
		}
	}
	return AlertRule{}, ErrNotFound
}

func (m *MockStore) ListRules(_ context.Context, cursor string, limit int, enabled *bool, contractID string) ([]AlertRule, string, error) {
	if limit <= 0 {
		limit = 50
	}
	var out []AlertRule
	for _, r := range m.rules {
		if enabled != nil && r.Enabled != *enabled {
			continue
		}
		if contractID != "" && r.ContractID != contractID {
			continue
		}
		if cursor != "" && r.ID <= cursor {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	var next string
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}

// RuleWindowStats aggregates the in-memory invocations/events for the window,
// mirroring the postgres implementation.
func (m *MockStore) RuleWindowStats(_ context.Context, contractID string, window time.Duration) (rules.WindowStats, error) {
	stats := rules.WindowStats{Duration: window}
	cutoff := time.Now().UTC().Add(-window)
	for _, inv := range m.invocations {
		if inv.ContractID != contractID || inv.LedgerClosedAt.Before(cutoff) {
			continue
		}
		stats.Invocations = append(stats.Invocations, rules.InvocationSample{
			Status:      inv.Status,
			Timestamp:   inv.LedgerClosedAt,
			FeeStroops:  inv.ResourceFeeCharged,
			CPUInsn:     inv.CPUInsn,
			MemBytes:    inv.MemByte,
			LedgerBytes: inv.LedgerReadByte + inv.LedgerWriteByte,
		})
	}
	for _, e := range m.events {
		if e.ContractID == contractID && !e.LedgerClosedAt.Before(cutoff) {
			stats.Events++
		}
	}
	return stats, nil
}