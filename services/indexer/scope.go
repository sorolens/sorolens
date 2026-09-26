package main

import (
	"context"
	"sync"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

// scopedStore narrows the poller's view of the world to the contracts this
// worker owns (issue #272).
//
// It embeds poller.Store so every other method — writes, sync state, cursor
// updates — passes straight through to the real store untouched; only
// ListContracts is overridden. That keeps the sharding change out of the
// poller entirely: a worker given a contract scope simply sees a smaller
// contract table, and the existing indexing loop does the rest.
type scopedStore struct {
	poller.Store

	mu    sync.RWMutex
	scope map[string]struct{}
}

// newScopedStore wraps a store. With no scope set it reports zero contracts,
// which is the safe default: a worker that has not been assigned shards yet
// must index nothing rather than everything, or every replica would duplicate
// the same work.
func newScopedStore(inner poller.Store) *scopedStore {
	return &scopedStore{Store: inner, scope: map[string]struct{}{}}
}

// SetScope replaces the assigned contract set. Called whenever the worker
// refreshes its shard assignment, including when a shard is taken away.
func (s *scopedStore) SetScope(contractIDs []string) {
	next := make(map[string]struct{}, len(contractIDs))
	for _, id := range contractIDs {
		if id == "" {
			continue
		}
		next[id] = struct{}{}
	}
	s.mu.Lock()
	s.scope = next
	s.mu.Unlock()
}

// InScope reports whether a contract is currently assigned to this worker.
func (s *scopedStore) InScope(contractID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.scope) == 0 {
		return false
	}
	_, ok := s.scope[contractID]
	return ok
}

// ScopeSize is the number of contracts currently assigned to this worker.
func (s *scopedStore) ScopeSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.scope)
}

// ListContracts pages over the assigned contracts only.
//
// The inner store is still paged by contract-id keyset so the underlying query
// stays indexed. The cursor handed back is the id of the last *selected*
// contract, which the inner keyset comparison (contract_id > cursor) resumes
// from correctly even when most of a page was filtered out.
func (s *scopedStore) ListContracts(ctx context.Context, cursor string, limit int) ([]poller.Contract, string, error) {
	if limit <= 0 {
		limit = 50
	}

	s.mu.RLock()
	scoped := len(s.scope) > 0
	scope := make(map[string]struct{}, len(s.scope))
	for id := range s.scope {
		scope[id] = struct{}{}
	}
	s.mu.RUnlock()

	if !scoped {
		return nil, "", nil
	}

	out := make([]poller.Contract, 0, limit)
	inner := cursor
	for {
		batch, nextInner, err := s.Store.ListContracts(ctx, inner, limit)
		if err != nil {
			return nil, "", err
		}

		for _, c := range batch {
			if _, ok := scope[c.ID]; !ok {
				continue
			}
			out = append(out, c)
			if len(out) == limit {
				// More may remain; resume after the last contract returned.
				return out, c.ID, nil
			}
		}

		if nextInner == "" {
			return out, "", nil
		}
		if len(batch) == 0 {
			// Defensive: an empty page that still claims a successor would
			// otherwise spin forever.
			return out, nextInner, nil
		}
		inner = nextInner
	}
}
