package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// MockEscalationPolicyStore is an in-memory EscalationPolicyStore for tests.
type MockEscalationPolicyStore struct {
	mu       sync.Mutex
	policies map[string]EscalationPolicy
	seq      int
}

func newMockEscalationPolicyStore() *MockEscalationPolicyStore {
	return &MockEscalationPolicyStore{policies: make(map[string]EscalationPolicy)}
}

func (m *MockEscalationPolicyStore) CreateEscalationPolicy(_ context.Context, p EscalationPolicy) (EscalationPolicy, error) {
	if p.Name == "" {
		return EscalationPolicy{}, fmt.Errorf("%w: name is required", ErrInvalidEscalationPolicy)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	now := time.Now().UTC()
	p.ID = fmt.Sprintf("ep-%d", m.seq)
	p.CreatedAt = now
	p.UpdatedAt = now
	m.policies[p.ID] = p
	return p, nil
}

func (m *MockEscalationPolicyStore) GetEscalationPolicy(_ context.Context, id string) (EscalationPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.policies[id]
	if !ok {
		return EscalationPolicy{}, ErrEscalationPolicyNotFound
	}
	return p, nil
}

func (m *MockEscalationPolicyStore) ListEscalationPolicies(_ context.Context) ([]EscalationPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]EscalationPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *MockEscalationPolicyStore) UpdateEscalationPolicy(_ context.Context, p EscalationPolicy) (EscalationPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.policies[p.ID]
	if !ok {
		return EscalationPolicy{}, ErrEscalationPolicyNotFound
	}
	existing.Name = p.Name
	existing.Steps = p.Steps
	existing.UpdatedAt = time.Now().UTC()
	m.policies[p.ID] = existing
	return existing, nil
}

func (m *MockEscalationPolicyStore) DeleteEscalationPolicy(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.policies, id)
	return nil
}

// embed into MockStore
func (m *MockStore) CreateEscalationPolicy(ctx context.Context, p EscalationPolicy) (EscalationPolicy, error) {
	return m.escalation().CreateEscalationPolicy(ctx, p)
}
func (m *MockStore) GetEscalationPolicy(ctx context.Context, id string) (EscalationPolicy, error) {
	return m.escalation().GetEscalationPolicy(ctx, id)
}
func (m *MockStore) ListEscalationPolicies(ctx context.Context) ([]EscalationPolicy, error) {
	return m.escalation().ListEscalationPolicies(ctx)
}
func (m *MockStore) UpdateEscalationPolicy(ctx context.Context, p EscalationPolicy) (EscalationPolicy, error) {
	return m.escalation().UpdateEscalationPolicy(ctx, p)
}
func (m *MockStore) DeleteEscalationPolicy(ctx context.Context, id string) error {
	return m.escalation().DeleteEscalationPolicy(ctx, id)
}

func (m *MockStore) escalation() *MockEscalationPolicyStore {
	if m.escalationStore == nil {
		m.escalationStore = newMockEscalationPolicyStore()
	}
	return m.escalationStore
}
