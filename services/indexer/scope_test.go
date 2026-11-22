package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

type mockPollerStore struct {
	poller.Store
	mu        sync.Mutex
	contracts []poller.Contract
	callCount int
	err       error
}

func (m *mockPollerStore) ListContracts(_ context.Context, cursor string, limit int) ([]poller.Contract, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.err != nil {
		return nil, "", m.err
	}

	startIdx := 0
	if cursor != "" {
		for i, c := range m.contracts {
			if c.ID > cursor {
				startIdx = i
				break
			}
			if i == len(m.contracts)-1 {
				startIdx = len(m.contracts)
			}
		}
	}

	if startIdx >= len(m.contracts) {
		return nil, "", nil
	}

	endIdx := startIdx + limit
	var nextCursor string
	if endIdx >= len(m.contracts) {
		endIdx = len(m.contracts)
		nextCursor = ""
	} else {
		nextCursor = m.contracts[endIdx-1].ID
	}

	return m.contracts[startIdx:endIdx], nextCursor, nil
}

func TestScopedStore_DefaultEmpty(t *testing.T) {
	mock := &mockPollerStore{}
	s := newScopedStore(mock)

	if s.ScopeSize() != 0 {
		t.Errorf("ScopeSize() = %d, want 0", s.ScopeSize())
	}
	if s.InScope("c1") {
		t.Errorf("InScope(\"c1\") = true, want false")
	}

	contracts, nextCursor, err := s.ListContracts(context.Background(), "", 10)
	if err != nil {
		t.Fatalf("ListContracts error: %v", err)
	}
	if len(contracts) != 0 || nextCursor != "" {
		t.Errorf("ListContracts returned (%v, %q), want (nil, \"\")", contracts, nextCursor)
	}
	if mock.callCount != 0 {
		t.Errorf("mock was called %d times, want 0", mock.callCount)
	}
}

func TestScopedStore_SetScope(t *testing.T) {
	mock := &mockPollerStore{}
	s := newScopedStore(mock)

	s.SetScope([]string{"c1", "", "c2", "c1", ""})

	if s.ScopeSize() != 2 {
		t.Errorf("ScopeSize() = %d, want 2", s.ScopeSize())
	}
	if !s.InScope("c1") {
		t.Errorf("InScope(\"c1\") = false, want true")
	}
	if !s.InScope("c2") {
		t.Errorf("InScope(\"c2\") = false, want true")
	}
	if s.InScope("c3") {
		t.Errorf("InScope(\"c3\") = true, want false")
	}
	if s.InScope("") {
		t.Errorf("InScope(\"\") = true, want false")
	}

	// Update scope to empty
	s.SetScope([]string{})
	if s.ScopeSize() != 0 {
		t.Errorf("ScopeSize() = %d, want 0", s.ScopeSize())
	}
	if s.InScope("c1") {
		t.Errorf("InScope(\"c1\") after reset = true, want false")
	}
}

func TestScopedStore_ConcurrentAccess(t *testing.T) {
	mock := &mockPollerStore{}
	s := newScopedStore(mock)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cid := fmt.Sprintf("c%d", id)
			s.SetScope([]string{cid, "common"})
			_ = s.InScope(cid)
			_ = s.InScope("common")
			_ = s.ScopeSize()
		}(i)
	}
	wg.Wait()
}

func TestScopedStore_ListContracts(t *testing.T) {
	ctx := context.Background()

	t.Run("propagates inner store error", func(t *testing.T) {
		mockErr := errors.New("db failure")
		mock := &mockPollerStore{err: mockErr}
		s := newScopedStore(mock)
		s.SetScope([]string{"c1"})

		_, _, err := s.ListContracts(ctx, "", 10)
		if !errors.Is(err, mockErr) {
			t.Fatalf("ListContracts error = %v, want %v", err, mockErr)
		}
	})

	t.Run("filters only contracts in scope across pages", func(t *testing.T) {
		allContracts := []poller.Contract{
			{ID: "c1", Network: "testnet"},
			{ID: "c2", Network: "testnet"},
			{ID: "c3", Network: "testnet"},
			{ID: "c4", Network: "testnet"},
			{ID: "c5", Network: "testnet"},
		}
		mock := &mockPollerStore{contracts: allContracts}
		s := newScopedStore(mock)
		s.SetScope([]string{"c2", "c4"})

		// Limit 1 to force pagination
		c1, next1, err := s.ListContracts(ctx, "", 1)
		if err != nil {
			t.Fatalf("page 1 error: %v", err)
		}
		if len(c1) != 1 || c1[0].ID != "c2" {
			t.Errorf("page 1 = %v, want [c2]", c1)
		}
		if next1 != "c2" {
			t.Errorf("page 1 next cursor = %q, want c2", next1)
		}

		// Page 2 resuming from next1
		c2, next2, err := s.ListContracts(ctx, next1, 1)
		if err != nil {
			t.Fatalf("page 2 error: %v", err)
		}
		if len(c2) != 1 || c2[0].ID != "c4" {
			t.Errorf("page 2 = %v, want [c4]", c2)
		}
		if next2 != "c4" {
			t.Errorf("page 2 next cursor = %q, want c4", next2)
		}

		// Page 3 resuming from next2 - should be empty
		c3, next3, err := s.ListContracts(ctx, next2, 1)
		if err != nil {
			t.Fatalf("page 3 error: %v", err)
		}
		if len(c3) != 0 || next3 != "" {
			t.Errorf("page 3 = (%v, %q), want (nil, \"\")", c3, next3)
		}
	})

	t.Run("default limit when limit <= 0", func(t *testing.T) {
		allContracts := []poller.Contract{
			{ID: "c1", Network: "testnet"},
			{ID: "c2", Network: "testnet"},
		}
		mock := &mockPollerStore{contracts: allContracts}
		s := newScopedStore(mock)
		s.SetScope([]string{"c1", "c2"})

		res, _, err := s.ListContracts(ctx, "", 0)
		if err != nil {
			t.Fatalf("ListContracts error: %v", err)
		}
		if len(res) != 2 {
			t.Errorf("len(res) = %d, want 2", len(res))
		}
	})
}

type emptyBatchStore struct {
	poller.Store
	calls int
}

func (e *emptyBatchStore) ListContracts(_ context.Context, _ string, _ int) ([]poller.Contract, string, error) {
	e.calls++
	return nil, "next-cursor", nil
}

func TestScopedStore_DefensiveEmptyBatch(t *testing.T) {
	// Guards against infinite loops when the inner store returns an empty batch with a successor
	mock := &emptyBatchStore{}
	s := newScopedStore(mock)
	s.SetScope([]string{"c1"})

	out, cursor, err := s.ListContracts(context.Background(), "", 10)
	if err != nil {
		t.Fatalf("ListContracts error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("len(out) = %d, want 0", len(out))
	}
	if cursor != "next-cursor" {
		t.Errorf("cursor = %q, want next-cursor", cursor)
	}
	if mock.calls != 1 {
		t.Errorf("mock.calls = %d, want 1", mock.calls)
	}
}
